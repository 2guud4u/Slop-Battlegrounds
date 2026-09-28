package game

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sort"
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
	Message  string   `json:"message"`  // error payloads outbound
}

type Room struct {
	mu  sync.Mutex
	gen int // bumped on every phase transition; timers carry their gen

	Code       string
	Phase      Phase
	Round      int
	WinnerID   string // set when the game ends (PhaseOver)
	hostID     string
	players    []*Player
	conns      map[string]Client // playerID -> conn
	bets       []Bet
	fight      *Fight
	deckSeq    int
	decks      map[string][]string // shuffled draw piles per kind
	locOptions []string            // battleground candidates this round
	locVotes   map[string]string   // playerID -> chosen option
	Complexity string              // narration level: LevelMiddle/LevelHigh/LevelCollege
	shopQueue  []string            // shop turn order: fewest wins, ties by fewest coins
	turnAt     int                 // index into shopQueue — whose buy turn it is

	judge  Judge
	imager Imager
}

func New(code string, j Judge, img Imager) *Room {
	return &Room{
		Code:       code,
		Phase:      PhaseLobby,
		conns:      map[string]Client{},
		decks:      buildDecks(),
		judge:      j,
		imager:     img,
		Complexity: LevelHigh,
	}
}

// ---------- join / leave ----------

// avatarLocked changes a player's avatar (lobby only). Managers are unique —
// a key already claimed by another player is rejected.
func (r *Room) avatarLocked(playerID, key string) {
	if r.Phase != PhaseLobby {
		return
	}
	if AvatarURL(key) == "" {
		return
	}
	p := r.findLocked(playerID)
	if p == nil {
		return
	}
	for _, q := range r.players {
		if q != p && q.Avatar == key {
			r.sendErr(playerID, "manager already taken")
			return
		}
	}
	p.Avatar = key
}

// nameLocked renames a player (lobby only). Names must be unique.
func (r *Room) nameLocked(playerID, name string) {
	if r.Phase != PhaseLobby {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 24 {
		return
	}
	p := r.findLocked(playerID)
	if p == nil {
		return
	}
	for _, q := range r.players {
		if q != p && strings.EqualFold(q.Name, name) {
			r.sendErr(playerID, "name already taken")
			return
		}
	}
	p.Name = name
}

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
	// A shopper who left forfeits the rest of the queue turn.
	for r.Phase == PhaseShop {
		if cur := r.shopTurnLocked(); cur != "" {
			p := r.findLocked(cur)
			if p == nil || p.Connected {
				break
			}
			p.Passed = true
			r.nextShopTurnLocked()
		} else {
			break
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
	}
	r.broadcastLocked()
}

func (r *Room) sendErr(playerID, msg string) {
	if c := r.conns[playerID]; c != nil {
		_ = c.SendJSON(ClientMsg{Type: "error", Message: msg})
	}
}

// levelLocked: host-only, lobby-only complexity setting.
func (r *Room) levelLocked(playerID, level string) {
	if r.Phase != PhaseLobby || playerID != r.hostID {
		return
	}
	switch level {
	case LevelMiddle, LevelHigh, LevelCollege:
		r.Complexity = level
	}
}

func (r *Room) startLocked(playerID string) {
	if r.Phase != PhaseLobby {
		r.sendErr(playerID, "game already started")
		return
	}
	if playerID != r.hostID {
		r.sendErr(playerID, "only the host can start")
		return
	}
	if len(r.players) < 2 {
		r.sendErr(playerID, "need at least 2 players")
		return
	}
	r.toDraftLocked()
}

// draftLocked validates the ordered cards against an owned forge template.
// Literal slots (with/of/the…) join the name verbatim; card ids fill the
// kind slots in order.
func (r *Room) draftLocked(playerID, tmplID string, cardIDs []string) {
	if r.Phase != PhaseDraft {
		return
	}
	p := r.findLocked(playerID)
	if p == nil || p.Champion != nil {
		return
	}
	if r.fight == nil || (playerID != r.fight.A && playerID != r.fight.B) {
		r.sendErr(playerID, "spectators don't forge — you're the audience this round")
		return
	}
	t := TemplateByID(tmplID)
	if t == nil || !p.Templates[tmplID] {
		r.sendErr(playerID, "you don't own that forge template")
		return
	}
	need := 0
	for _, s := range t.Slots {
		if !slotIsLiteral(s) {
			need++
		}
	}
	if len(cardIDs) != need {
		r.sendErr(playerID, fmt.Sprintf("template needs %d cards", need))
		return
	}
	// Build the name slot by slot; each card must match its slot's kind.
	var words []string
	ci := 0
	for _, s := range t.Slots {
		if slotIsLiteral(s) {
			words = append(words, s)
			continue
		}
		c := peekCard(p, cardIDs[ci])
		if c == nil || c.Kind != s {
			r.sendErr(playerID, fmt.Sprintf("slot %d needs a %s", ci+1, s))
			return
		}
		words = append(words, c.Text)
		ci++
	}
	var cards []Card
	for _, id := range cardIDs { // all valid — collect + remove from hand
		for i, c := range p.Hand {
			if c.ID == id {
				cards = append(cards, c)
				p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
				break
			}
		}
	}
	p.Champion = &Champion{Text: strings.Join(words, " "), Cards: cards}
	r.maybeAdvanceLocked()
	r.broadcastLocked()
}

// shopLocked buys a forge template with coins.
func (r *Room) shopLocked(playerID, tmplID string) {
	if r.Phase != PhaseShop {
		return
	}
	if r.shopTurnLocked() != playerID {
		r.sendErr(playerID, "not your turn to shop")
		return
	}
	p := r.findLocked(playerID)
	t := TemplateByID(tmplID)
	if p == nil || t == nil || t.Cost == 0 {
		return
	}
	if p.Templates[tmplID] {
		r.sendErr(playerID, "already owned")
		return
	}
	if p.Coins < t.Cost {
		r.sendErr(playerID, "not enough coins")
		return
	}
	p.Coins -= t.Cost
	p.Templates[tmplID] = true
	log.Printf("room %s: %s bought forge template %s", r.Code, p.Name, tmplID)
}

func (r *Room) betLocked(playerID, on string, amount int) {
	if r.Phase != PhaseBetting || r.fight == nil {
		return
	}
	if playerID == r.fight.A || playerID == r.fight.B {
		r.sendErr(playerID, "fighters cannot bet")
		return
	}
	p := r.findLocked(playerID)
	if p == nil || r.findLocked(on) == nil || (on != r.fight.A && on != r.fight.B) {
		return
	}
	for _, b := range r.bets {
		if b.BettorID == playerID {
			r.sendErr(playerID, "already bet this round")
			return
		}
	}
	if amount == 0 { // pass
		p.Passed = true
		r.bets = append(r.bets, Bet{BettorID: playerID, On: on, Amount: 0})
		r.maybeAdvanceLocked()
		r.broadcastLocked()
		return
	}
	if amount < minBet {
		r.sendErr(playerID, fmt.Sprintf("minimum bet is %d", minBet))
		return
	}
	if amount > p.Coins {
		r.sendErr(playerID, "not enough coins")
		return
	}
	p.Coins -= amount
	r.bets = append(r.bets, Bet{BettorID: playerID, On: on, Amount: amount})
	r.maybeAdvanceLocked()
	r.broadcastLocked()
}

func (r *Room) verbLocked(playerID, opt string) {
	if r.Phase != PhaseCombat || r.fight == nil {
		return
	}
	f := r.fight
	if f.Round == 1 && time.Now().Before(f.ReadyAt) {
		return // bell hasn't rung yet
	}
	if playerID != f.A && playerID != f.B {
		return
	}
	if r.actedLocked(f, playerID) {
		r.sendErr(playerID, "already acted this round")
		return
	}
	opt = strings.TrimSpace(opt)
	valid := false
	for _, o := range f.Options[playerID] {
		if o == opt {
			valid = true
			break
		}
	}
	if !valid {
		r.sendErr(playerID, "invalid action option")
		return
	}
	f.Moves = append(f.Moves, Move{Round: f.Round, PlayerID: playerID, Verb: opt})
	r.maybeAdvanceLocked()
	r.broadcastLocked()
}

// actedLocked reports whether a fighter has committed an action in the
// current round (picked an option or let fate decide).
func (r *Room) actedLocked(f *Fight, pid string) bool {
	for _, m := range f.Moves {
		if m.PlayerID == pid && m.Round == f.Round {
			return true
		}
	}
	return f.Fate[pid] == f.Round
}

// locVoteLocked records a battleground vote (draft phase only). Fighters
// (predicted by the same round-robin that picks the pairing) can't vote.
func (r *Room) locVoteLocked(playerID, opt string) {
	if r.Phase != PhaseDraft {
		return
	}
	if r.findLocked(playerID) == nil {
		return
	}
	if r.fight == nil || (playerID == r.fight.A || playerID == r.fight.B) {
		r.sendErr(playerID, "fighters can't vote on the battleground")
		return
	}
	for _, o := range r.locOptions {
		if o == opt {
			r.locVotes[playerID] = opt
			return
		}
	}
}

// allFoughtLocked: every connected player has been a fighter at least once.
func (r *Room) allFoughtLocked() bool {
	for _, p := range r.players {
		if p.Connected && !p.Fought {
			return false
		}
	}
	return true
}

// toShopLocked: serial buying — fewest wins shops first, ties broken by
// fewest coins (catch-up balancing).
func (r *Room) toShopLocked() {
	sorted := append([]*Player{}, r.players...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Wins != b.Wins {
			return a.Wins < b.Wins
		}
		if a.Coins != b.Coins {
			return a.Coins < b.Coins
		}
		return a.ID < b.ID
	})
	r.shopQueue = nil
	for _, p := range sorted {
		if p.Connected {
			r.shopQueue = append(r.shopQueue, p.ID)
		}
	}
	for _, p := range r.players {
		p.Passed = false
		p.Fought = false // shop ends the cycle — rotation restarts after it
	}
	r.turnAt = 0
	r.Phase = PhaseShop
	r.enterLocked()
}

// toOverLocked ends the game: richest connected player takes it.
func (r *Room) toOverLocked() {
	best := ""
	coins := -1
	for _, p := range r.players {
		if p.Connected && p.Coins > coins {
			best, coins = p.ID, p.Coins
		}
	}
	r.WinnerID = best
	r.Phase = PhaseOver
	r.enterLocked()
}

// canForgeLocked: does this hand still cover at least one owned template?
func (r *Room) canForgeLocked(p *Player) bool {
	counts := map[string]int{}
	for _, c := range p.Hand {
		counts[c.Kind]++
	}
	for _, t := range forgeTemplates {
		if !p.Templates[t.ID] {
			continue
		}
		need := map[string]int{}
		for _, s := range t.Slots {
			if !slotIsLiteral(s) {
				need[s]++
			}
		}
		ok := true
		for k, n := range need {
			if counts[k] < n {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// shopTurnLocked: the player whose buy turn is live, or "".
func (r *Room) shopTurnLocked() string {
	if r.turnAt < 0 || r.turnAt >= len(r.shopQueue) {
		return ""
	}
	return r.shopQueue[r.turnAt]
}

// nextShopTurnLocked moves the turn pointer to the next shopper.
func (r *Room) nextShopTurnLocked() {
	r.turnAt++
}

func (r *Room) passLocked(playerID string) {
	p := r.findLocked(playerID)
	if p == nil {
		return
	}
	switch r.Phase {
	case PhaseBetting:
		r.betLocked(playerID, "", 0)
	case PhaseVerdict:
		p.Passed = true // ready for the next round
		r.maybeAdvanceLocked()
	case PhaseShop:
		if r.shopTurnLocked() != playerID {
			return // not your turn
		}
		p.Passed = true // done shopping
		r.nextShopTurnLocked()
		r.maybeAdvanceLocked()
	case PhaseCombat:
		f := r.fight
		if f == nil || (f.Round == 1 && time.Now().Before(f.ReadyAt)) {
			return // still counting down
		}
		if playerID != f.A && playerID != f.B {
			return
		}
		if r.actedLocked(f, playerID) {
			return
		}
		f.Fate[playerID] = f.Round // fate picks one of the fighter's options
		opts := f.Options[playerID]
		pick := sampleVerbs(1)[0]
		if len(opts) > 0 {
			pick = opts[rand.IntN(len(opts))]
		}
		f.Moves = append(f.Moves, Move{Round: f.Round, PlayerID: playerID, Verb: pick, Fate: true})
		r.maybeAdvanceLocked()
		r.broadcastLocked()
	}
}

// genOptionsLocked asks the judge for fresh action options for the current
// round; applied only if the round hasn't moved on. Call with r.mu held.
func (r *Room) genOptionsLocked() {
	f := r.fight
	if f == nil {
		return
	}
	f.OptRound = f.Round
	f.Options = map[string][]string{} // clear last round's options until new ones land
	a, b := r.findLocked(f.A), r.findLocked(f.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		return
	}
	req := JudgeRequest{A: *a.Champion, B: *b.Champion, Location: f.Location, Round: f.Round, History: eventHistory(f), Level: r.Complexity}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		opts, err := r.judge.Options(ctx, req)
		cancel()
		if err != nil || len(opts.A) == 0 || len(opts.B) == 0 {
			log.Printf("room %s: options gen failed (%v), using fallback moves", r.Code, err)
			opts = RoundOptions{A: sampleVerbs(verbsPerFighter), B: sampleVerbs(verbsPerFighter)}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.fight != f || f.OptRound != req.Round {
			return // stale
		}
		f.Options[f.A] = opts.A
		f.Options[f.B] = opts.B
		r.broadcastLocked()
	}()
}

// ---------- phase machine ----------

func (r *Room) toDraftLocked() {
	r.Round++
	r.bets = nil
	r.fight = nil
	r.locVotes = map[string]string{}
	r.locOptions = nil // filled async by the judge; falls back to the pool
	for _, p := range r.players {
		p.Champion = nil
		p.Passed = false
		r.nounSafetyLocked(p)
	}
	// Pair up players who haven't fought this cycle. An odd leftover gets a
	// rematch against a random past winner — the bout everyone wants to see.
	var unfought []*Player
	for _, p := range r.players {
		if p.Connected && r.canForgeLocked(p) && !p.Fought {
			unfought = append(unfought, p)
		}
	}
	var a, b *Player
	switch {
	case len(unfought) >= 2:
		a, b = unfought[0], unfought[1]
	case len(unfought) == 1:
		a = unfought[0]
		b = r.pastWinnerLocked(a.ID)
		if b == nil { // no prior winner — any forgeable spectator does
			for _, p := range r.players {
				if p.Connected && p.ID != a.ID && r.canForgeLocked(p) {
					b = p
					break
				}
			}
		}
	default: // all fought but shop reset missed — keep the room alive
		var elig []*Player
		for _, p := range r.players {
			if p.Connected && r.canForgeLocked(p) {
				elig = append(elig, p)
			}
		}
		if len(elig) >= 2 {
			a, b = elig[0], elig[1]
		}
	}
	if a == nil || b == nil {
		r.toOverLocked()
		return
	}
	r.fight = &Fight{A: a.ID, B: b.ID, Round: 1, Fate: map[string]int{}, Options: map[string][]string{}}
	log.Printf("room %s: bout %q vs %q", r.Code, a.Name, b.Name)
	r.Phase = PhaseDraft
	r.enterLocked()
	r.genLocationsLocked()
}

// pastWinnerLocked: a random forgeable fighter who already won this cycle —
// the odd-count rematch opponent.
func (r *Room) pastWinnerLocked(exceptID string) *Player {
	var prev []*Player
	for _, p := range r.players {
		if p.Connected && p.ID != exceptID && p.Wins > 0 && r.canForgeLocked(p) {
			prev = append(prev, p)
		}
	}
	if len(prev) == 0 {
		return nil
	}
	return prev[rand.IntN(len(prev))]
}

// genLocationsLocked asks the judge for this round's battlegrounds; applied
// only while still in the same draft phase. Call with r.mu held.
func (r *Room) genLocationsLocked() {
	gen := r.gen
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		locs, err := r.judge.Locations(ctx, 4, r.Complexity)
		cancel()
		if err != nil || len(locs) == 0 {
			locs = PickBattlegrounds(4)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen == gen && len(r.locOptions) == 0 {
			r.locOptions = locs
			r.broadcastLocked()
		}
	}()
}

func (r *Room) toBettingLocked() {
	a, b := r.findLocked(r.fight.A), r.findLocked(r.fight.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		// A fighter left or couldn't forge — re-draft for a fresh bout.
		r.toDraftLocked()
		return
	}
	r.fight.Location = r.winningLocationLocked() // spectator votes are in
	for _, p := range r.players {
		p.Passed = false
	}
	r.genSceneLocked() // scene text + art land during betting
	if len(r.connectedLocked()) <= 2 {
		// nobody left to bet; skip to combat
		r.toCombatLocked()
		return
	}
	r.Phase = PhaseBetting
	r.enterLocked()
}

func (r *Room) connectedLocked() []*Player {
	var out []*Player
	for _, p := range r.players {
		if p.Connected {
			out = append(out, p)
		}
	}
	return out
}

func (r *Room) toCombatLocked() {
	for _, p := range r.players {
		p.Passed = false
	}
	r.Phase = PhaseCombat
	if r.fight != nil {
		r.fight.ReadyAt = time.Now().Add(6 * time.Second) // card-toss + smoke + "vs" reveal
	}
	r.enterLocked()
	r.genSceneLocked()   // no-op if betting already kicked it off
	r.genOptionsLocked() // first round's move set
}

// genSceneLocked narrates the arena setup (async) then paints a wide shot of
// the scene. Safe to call more than once — SceneGen guards the dispatch.
func (r *Room) genSceneLocked() {
	f := r.fight
	if f == nil || f.SceneGen {
		return
	}
	f.SceneGen = true
	a, b := r.findLocked(f.A), r.findLocked(f.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		s, err := r.judge.Scene(ctx, JudgeRequest{A: *a.Champion, B: *b.Champion, Location: f.Location, Level: r.Complexity})
		cancel()
		if err != nil || s == "" {
			log.Printf("room %s: scene gen failed: %v", r.Code, err)
			return
		}
		r.mu.Lock()
		if r.fight != f || f.Scene != "" {
			r.mu.Unlock()
			return
		}
		f.Scene = s
		r.broadcastLocked()
		r.mu.Unlock()
		// paint the narration — before the first bell if the net is kind
		prompt := sceneImagePrompt(f.Location, s, a.Champion.Name(), b.Champion.Name())
		go r.genImage(prompt, "", func(url string) {
			if r.fight == f && f.SceneImage == "" {
				f.SceneImage = url
				r.broadcastLocked()
			}
		})
	}()
}

// winningLocationLocked: plurality vote among spectators (fighters don't get a
// say — they're busy forging). Tie or no spectator votes → first option.
func (r *Room) winningLocationLocked() string {
	counts := map[string]int{}
	for pid, v := range r.locVotes {
		if r.fight != nil && (pid == r.fight.A || pid == r.fight.B) {
			continue
		}
		counts[v]++
	}
	best, bestN := "", -1
	for _, o := range r.locOptions {
		if counts[o] > bestN {
			best, bestN = o, counts[o]
		}
	}
	if best == "" {
		best = "a mysterious void"
	}
	return best
}

// drawStrings copies n unique entries from pool (shuffled). n may exceed
// len(pool): pool cycles.
func drawStrings(pool []string, n int) []string {
	perm := rand.Perm(len(pool))
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, pool[perm[i%len(perm)]])
	}
	return out
}

// PickBattlegrounds samples n canned locations — the mock judge's Locations
// and the fallback when the LLM is down.
func PickBattlegrounds(n int) []string {
	return drawStrings(battlegroundPool, n)
}

func (r *Room) toVerdictLocked() {
	for _, p := range r.players {
		p.Passed = false
	}
	r.Phase = PhaseVerdict
	r.enterLocked()
}

// maybeAdvanceLocked auto-progresses phases once all required inputs are in.
func (r *Room) maybeAdvanceLocked() {
	switch r.Phase {
	case PhaseDraft:
		if r.fight == nil {
			r.toDraftLocked() // malformed draft — repick a bout (or game over)
			return
		}
		for _, fid := range []string{r.fight.A, r.fight.B} {
			p := r.findLocked(fid)
			if p != nil && p.Connected && p.Champion == nil && r.canForgeLocked(p) {
				return
			}
		}
		r.toBettingLocked()
	case PhaseBetting:
		if r.fight == nil {
			return
		}
		for _, p := range r.players {
			if !p.Connected || p.ID == r.fight.A || p.ID == r.fight.B {
				continue
			}
			done := p.Passed
			for _, b := range r.bets {
				if b.BettorID == p.ID {
					done = true
				}
			}
			if !done {
				return
			}
		}
		r.toCombatLocked()
	case PhaseVerdict:
		for _, p := range r.players {
			if p.Connected && !p.Passed {
				return
			}
		}
		if r.allFoughtLocked() {
			r.toShopLocked()
		} else {
			r.toDraftLocked()
		}
	case PhaseShop:
		if r.turnAt < len(r.shopQueue) {
			return
		}
		r.toDraftLocked()
	case PhaseCombat:
		f := r.fight
		if f == nil || f.Resolving {
			return
		}
		for _, fid := range []string{f.A, f.B} {
			p := r.findLocked(fid)
			if p == nil || !p.Connected {
				continue
			}
			if !r.actedLocked(f, fid) {
				return
			}
		}
		r.resolveRoundLocked()
	}
}

// resolveRoundLocked judges the just-finished combat round: narration + art,
// then either next round or the verdict. Call with r.mu held.
func (r *Room) resolveRoundLocked() {
	f := r.fight
	f.Resolving = true
	gen := r.gen
	round := f.Round
	a, b := r.findLocked(f.A), r.findLocked(f.B)
	movesA := movesInRound(f, f.A, round)
	movesB := movesInRound(f, f.B, round)
	fateA := f.Fate[f.A] == round
	fateB := f.Fate[f.B] == round
	go func() {
		req := JudgeRequest{
			A: *a.Champion, B: *b.Champion, AID: f.A, BID: f.B,
			MovesA: movesA, MovesB: movesB, FateA: fateA, FateB: fateB, Location: f.Location,
			History: eventHistory(f), Level: r.Complexity,
			Round: round, LastRound: round == 3,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		ev, err := r.judge.Round(ctx, req)
		cancel()
		if err != nil || ev == nil {
			log.Printf("room %s: round %d judge failed (%v), using fallback", r.Code, round, err)
			ev = fallbackRound(f, a.Champion, b.Champion, movesA, movesB, round)
		}
		ev.Round = round
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen != gen || r.fight != f {
			return
		}
		f.Events = append(f.Events, *ev)
		idx := len(f.Events) - 1
		if ev.Text != "" {
			go r.genImage(eventPrompt(ev.Text), f.SceneImage, func(url string) {
				if r.fight == f && idx < len(f.Events) && f.Events[idx].ImageURL == "" {
					f.Events[idx].ImageURL = url
					r.broadcastLocked()
				}
			})
		}
		f.Resolving = false
		if ev.Decided && ev.WinnerID != "" {
			f.Verdict = &Verdict{WinnerID: ev.WinnerID, Reason: ev.Text}
			r.payoutLocked(ev.WinnerID)
			r.toVerdictLocked()
		} else if round >= 3 {
			f.Draw = true
			f.Verdict = &Verdict{Reason: ev.Text}
			r.payoutLocked("") // draw: stakes returned
			r.toVerdictLocked()
		} else {
			f.Round++
			log.Printf("room %s -> combat round %d (round %d)", r.Code, f.Round, r.Round)
			r.genOptionsLocked() // fresh options for the new round
		}
		r.broadcastLocked()
	}()
}

func movesInRound(f *Fight, pid string, round int) []string {
	var out []string
	for _, m := range f.Moves {
		if m.PlayerID == pid && m.Round == round {
			out = append(out, m.Verb)
		}
	}
	return out
}

// eventHistory: "Round N: <narration>" lines for every resolved round —
// fed to the judge so later prompts remember what already happened.
func eventHistory(f *Fight) []string {
	var out []string
	for _, e := range f.Events {
		out = append(out, fmt.Sprintf("Round %d: %s", e.Round, e.Text))
	}
	return out
}

// enterLocked bumps gen (stale-check token for async work) and logs the phase.
// Call with r.mu held, after Phase is set.
func (r *Room) enterLocked() {
	r.gen++
	log.Printf("room %s -> %s (round %d)", r.Code, r.Phase, r.Round)
}

func (r *Room) payoutLocked(winnerID string) {
	// Both fighters have now fought — gates the shop.
	if r.fight != nil {
		if a := r.findLocked(r.fight.A); a != nil {
			a.Fought = true
		}
		if b := r.findLocked(r.fight.B); b != nil {
			b.Fought = true
		}
	}
	for _, b := range r.bets {
		if b.Amount == 0 {
			continue
		}
		if winnerID == "" {
			// draw: stakes returned
			if p := r.findLocked(b.BettorID); p != nil {
				p.Coins += b.Amount
			}
		} else if b.On == winnerID {
			if p := r.findLocked(b.BettorID); p != nil {
				p.Coins += b.Amount * betOdds
			}
		}
	}
	if w := r.findLocked(winnerID); w != nil {
		w.Coins += winBonus
		w.Wins++
	}
	for _, p := range r.players {
		if p.Connected && p.ID != winnerID {
			p.Coins += joinBonus
		}
	}
}

// ---------- helpers ----------

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

// buildDecks shuffles each kind's pool into a fresh deck. Cards are unique
// within a room until the deck cycles.
func buildDecks() map[string][]string {
	decks := map[string][]string{}
	for kind, pool := range cardPools {
		decks[kind] = append([]string{}, pool...)
	}
	for _, d := range decks {
		rand.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	}
	return decks
}

// drawLocked pops a card of the given kind, reshuffling the pool when the
// deck runs dry. Call with r.mu held.
func (r *Room) drawLocked(kind string) Card {
	d := r.decks[kind]
	if len(d) == 0 {
		d = append([]string{}, cardPools[kind]...)
		rand.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	}
	text := d[len(d)-1]
	// Noun cards sometimes arrive pre-modified; verb cards pre-adverbed —
	// the card's Kind stays what it is, the affix is just flavor.
	switch kind {
	case KindNoun:
		if rand.IntN(5) == 0 { // ~20%
			text = adjectivePool[rand.IntN(len(adjectivePool))] + " " + text
		}
	case KindVerb:
		if rand.IntN(5) == 0 {
			text = adverbPool[rand.IntN(len(adverbPool))] + " " + text
		}
	}
	r.decks[kind] = d[:len(d)-1]
	r.deckSeq++
	return Card{ID: fmt.Sprintf("c%d", r.deckSeq), Text: text, Kind: kind}
}

// dealHandLocked: the opening hand — every template's slot kinds plus one
// spare each, topped off with nouns up to handMax.
func (r *Room) dealHandLocked(p *Player) []Card {
	targets := map[string]int{}
	for _, t := range forgeTemplates {
		if !p.Templates[t.ID] {
			continue
		}
		for _, s := range t.Slots {
			if !slotIsLiteral(s) {
				targets[s]++
			}
		}
	}
	var h []Card
	for kind, n := range targets {
		for i := 0; i < n+1 && len(h) < handMax; i++ {
			h = append(h, r.drawLocked(kind))
		}
	}
	for len(h) < handMax {
		h = append(h, r.drawLocked(KindNoun))
	}
	return h
}

// nounSafetyLocked: a hand with no nouns draws back up — the noun safety
// valve so players can almost always keep forging.
func (r *Room) nounSafetyLocked(p *Player) {
	if len(p.Hand) >= handMax {
		return
	}
	for _, c := range p.Hand {
		if c.Kind == KindNoun {
			return
		}
	}
	for i := 0; i < nounSafety && len(p.Hand) < handMax; i++ {
		p.Hand = append(p.Hand, r.drawLocked(KindNoun))
	}
}

// buyPackLocked: a 3-card pack of one kind, if it fits under handMax.
func (r *Room) buyPackLocked(playerID, kind string) {
	p := r.findLocked(playerID)
	if p == nil || cardPools[kind] == nil {
		return
	}
	if r.shopTurnLocked() != playerID {
		r.sendErr(playerID, "not your turn to shop")
		return
	}
	if p.Coins < packCost {
		r.sendErr(playerID, "not enough coins")
		return
	}
	if len(p.Hand)+packSize > handMax {
		r.sendErr(playerID, "hand is full — 7 max")
		return
	}
	p.Coins -= packCost
	for i := 0; i < packSize; i++ {
		p.Hand = append(p.Hand, r.drawLocked(kind))
	}
	log.Printf("room %s: %s bought a %s pack", r.Code, p.Name, kind)
}

func takeCard(p *Player, id, kind string) string {
	for i, c := range p.Hand {
		if c.ID == id && (kind == "" || c.Kind == kind) {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return c.Text
		}
	}
	return ""
}

// peekCard finds a hand card without removing it.
func peekCard(p *Player, id string) *Card {
	for i := range p.Hand {
		if p.Hand[i].ID == id {
			return &p.Hand[i]
		}
	}
	return nil
}

// sampleVerbs draws n random pool actions — fallback when the judge can't
// propose options, and the fate-decide draw when options haven't loaded.
func sampleVerbs(n int) []string {
	pool := append([]string{}, verbPool...)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	return pool[:n]
}

// fallbackRound narrates a round when the judge is unreachable. Winner is
// picked at random (never on the last round — that yields a draw).
func fallbackRound(f *Fight, a, b *Champion, movesA, movesB []string, round int) *RoundEvent {
	ev := &RoundEvent{
		Round: round,
		Text: fmt.Sprintf("%s %s while %s %s — the judges are unreachable, but the crowd is roaring.",
			a.Name(), joinVerbs(movesA, "flails"), b.Name(), joinVerbs(movesB, "flails")),
	}
	if round < 3 && rand.IntN(4) == 0 {
		winner := f.A
		if rand.IntN(2) == 1 {
			winner = f.B
		}
		ev.Decided, ev.WinnerID = true, winner
	}
	return ev
}

func joinVerbs(v []string, fallback string) string {
	if len(v) == 0 {
		return fallback
	}
	return v[0]
}

// genImage calls the imager off-lock, then applies the result under r.mu.
func (r *Room) genImage(prompt, ref string, apply func(string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	url, err := r.imager.Generate(ctx, prompt, ref)
	cancel()
	if err != nil {
		log.Printf("room %s: image gen failed: %v", r.Code, err)
		return
	}
	r.mu.Lock()
	apply(url)
	r.mu.Unlock()
}
