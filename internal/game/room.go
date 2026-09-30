package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
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

	// reconnectGrace: a dropped seat stays "connected" this long so a page
	// reload doesn't forfeit the fighter's turn or skip their shop.
	reconnectGrace = 8 * time.Second
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
	Complexity    string              // narration level: one of the Level* constants
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

// Join adds (or reconnects) a player. A matching seat token reclaims that
// seat in any phase; otherwise a same-name match does (legacy/manual rejoin).
// Returns the player ID; the seat token reaches the client in its snapshot.
func (r *Room) Join(name, avatar, token string, c Client) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var seat *Player
	if token != "" {
		for _, p := range r.players {
			if p.Token == token {
				seat = p
				break
			}
		}
	}
	if seat == nil && name != "" {
		for _, p := range r.players {
			if strings.EqualFold(p.Name, name) {
				seat = p
				break
			}
		}
	}
	if seat != nil {
		seat.Connected = true
		r.conns[seat.ID] = c
		r.broadcastLocked()
		return seat.ID, nil
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
		Token:     newToken(),
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

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Leave detaches c from the player's seat. The seat stays live for
// reconnectGrace; if nobody reclaims it by then, the player is marked
// disconnected and the phase machine skips them. A stale socket closing
// after a reload (c no longer the seat's conn) is ignored.
func (r *Room) Leave(playerID string, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[playerID] != c {
		return // already replaced by a newer connection
	}
	delete(r.conns, playerID)
	time.AfterFunc(reconnectGrace, func() { r.expireSeat(playerID) })
}

// expireSeat drops a player who didn't come back within the grace period.
func (r *Room) expireSeat(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, back := r.conns[playerID]; back {
		return
	}
	p := r.findLocked(playerID)
	if p == nil || !p.Connected {
		return
	}
	p.Connected = false
	log.Printf("room %s: %s dropped (no reconnect within %v)", r.Code, p.Name, reconnectGrace)
	// A shopper who left: mark done, deal+trim so the phase can close.
	if r.Phase == PhaseShop {
		p.Passed = true
		r.shopCloseLocked(p)
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
