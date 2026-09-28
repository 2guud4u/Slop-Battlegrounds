package game

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	verbsPerFighter = 3
	handMax         = 7 // hard cap — packs can't push past it
	startCoins      = 100
	packCost        = 15 // coins per 3-card pack in the shop
	packSize        = 3
	nounSafety      = 3 // draw this many nouns when a hand runs dry

	winBonus  = 25
	joinBonus = 20
	betOdds   = 2 // winner bettors get stake*betOdds back
	minBet    = 10
)

// Client is one websocket connection owned by a player.
type Client interface {
	SendJSON(v any) error
}

// ClientMsg is the union of all inbound ws messages (post-join).
type ClientMsg struct {
	Type     string   `json:"type"`
	Cards    []string `json:"cards"`    // draft: ordered card ids filling the template's slots
	Template string   `json:"template"` // draft: forge template id / shop: buy id
	Pack     string   `json:"pack"`     // shop: card kind to buy a 3-pack of
	On       string   `json:"on"`       // bet target fighter id
	Amount   int      `json:"amount"`   // bet amount
	Verb     string   `json:"verb"`     // combat: the option text the fighter picked
	Avatar   string   `json:"avatar"`   // avatar key
	Name2    string   `json:"name"`     // rename in lobby
	Loc      string   `json:"loc"`      // battleground vote (draft phase)
	Card     string   `json:"card"`     // shop: discard this card id
	Message  string   `json:"message"`  // error payloads outbound
}

type Room struct {
	mu  sync.Mutex
	gen int // bumped on every phase transition; timers carry their gen

	Code          string
	Phase         Phase
	Round         int
	WinnerID      string // set when the game ends (PhaseOver)
	hostID        string
	players       []*Player
	conns         map[string]Client // playerID -> conn
	bets          []Bet
	fight         *Fight
	deckSeq       int
	decks         map[string][]string // shuffled draw piles per kind
	locOptions    []string            // battleground candidates this round
	locVotes      map[string]string   // playerID -> chosen option
	Complexity    string              // narration level: LevelMiddle/LevelHigh/LevelCollege
	revealAt      time.Time           // both champions forged — draft lingers for the reveal
	soldTemplates map[string]bool     // one copy per template — gone once bought

	judge  Judge
	imager Imager
}

func New(code string, j Judge, img Imager) *Room {
	return &Room{
		Code:          code,
		Phase:         PhaseLobby,
		conns:         map[string]Client{},
		decks:         buildDecks(),
		judge:         j,
		imager:        img,
		Complexity:    LevelHigh,
		soldTemplates: map[string]bool{},
	}
}

// ---------- join / leave ----------

// Join adds (or reconnects) a player. Returns the player ID.
func (r *Room) Join(name, avatar string, c Client) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reconnect: same (non-empty) name reclaims the seat.
	if name != "" {
		for _, p := range r.players {
			if strings.EqualFold(p.Name, name) {
				p.Connected = true
				r.conns[p.ID] = c
				r.broadcastLocked()
				return p.ID, nil
			}
		}
	}
	if r.Phase != PhaseLobby {
		return "", fmt.Errorf("game already in progress")
	}
	if len(r.players) >= 8 {
		return "", fmt.Errorf("room is full")
	}
	if name == "" {
		name = fmt.Sprintf("Player %d", len(r.players)+1)
	}
	// Avatars are unique; a pre-picked key that's already taken is dropped —
	// the player picks another in the lobby.
	for _, q := range r.players {
		if avatar != "" && q.Avatar == avatar {
			avatar = ""
			break
		}
	}
	p := &Player{
		ID:        fmt.Sprintf("p%d", len(r.players)+1),
		Name:      name,
		Avatar:    avatar,
		Coins:     startCoins,
		Connected: true,
		Templates: map[string]bool{},
	}
	for _, id := range starterTemplateIDs {
		p.Templates[id] = true
	}
	p.Hand = r.dealHandLocked(p)
	r.players = append(r.players, p)
	r.conns[p.ID] = c
	if r.hostID == "" {
		r.hostID = p.ID
	}
	r.broadcastLocked()
	return p.ID, nil
}

func (r *Room) Leave(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p.ID == playerID {
			p.Connected = false
			delete(r.conns, p.ID)
		}
	}
	// A shopper who left: mark done, deal+trim so the phase can close.
	if r.Phase == PhaseShop {
		for _, p := range r.players {
			if !p.Connected {
				p.Passed = true
				r.shopCloseLocked(p)
			}
		}
	}
	r.maybeAdvanceLocked()
	r.broadcastLocked()
}

func (r *Room) Players() []*Player {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Player, len(r.players))
	copy(out, r.players)
	return out
}

// ---------- inbound messages ----------

func (r *Room) Handle(playerID string, m ClientMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch m.Type {
	case "start":
		r.startLocked(playerID)
	case "draft":
		r.draftLocked(playerID, m.Template, m.Cards)
	case "bet":
		r.betLocked(playerID, m.On, m.Amount)
	case "verb":
		r.verbLocked(playerID, m.Verb)
	case "avatar":
		r.avatarLocked(playerID, m.Avatar)
	case "name":
		r.nameLocked(playerID, m.Name2)
	case "loc":
		r.locVoteLocked(playerID, m.Loc)
	case "level":
		r.levelLocked(playerID, m.Message)
	case "pass":
		r.passLocked(playerID)
	case "buy":
		if m.Pack != "" {
			r.buyPackLocked(playerID, m.Pack)
		} else {
			r.shopLocked(playerID, m.Template)
		}
	case "discard":
		r.discardLocked(playerID, m.Card)
	}
	r.broadcastLocked()
}

func (r *Room) sendErr(playerID, msg string) {
	if c := r.conns[playerID]; c != nil {
		_ = c.SendJSON(ClientMsg{Type: "error", Message: msg})
	}
}

func (r *Room) findLocked(id string) *Player {
	for _, p := range r.players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (r *Room) championsLocked() []*Player {
	var out []*Player
	for _, p := range r.players {
		if p.Champion != nil {
			out = append(out, p)
		}
	}
	return out
}
