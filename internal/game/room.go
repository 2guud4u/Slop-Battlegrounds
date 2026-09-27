package game

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	draftTimeout   = 60 * time.Second
	betTimeout     = 30 * time.Second
	combatTimeout  = 90 * time.Second
	verdictTimeout = 20 * time.Second

	verbsPerFighter = 2
	handAdjs        = 4
	handNouns       = 4
	handVerbs       = 3
	startCoins      = 100
	maxEventImages  = 2 // event art beyond the 2 champion images; free tiers are slow

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
	Type    string `json:"type"`
	Adj     string `json:"adj"`     // card id
	Noun    string `json:"noun"`    // card id
	On      string `json:"on"`      // bet target fighter id
	Amount  int    `json:"amount"`  // bet amount
	Verb    string `json:"verb"`    // card id
	Avatar  string `json:"avatar"`  // avatar key
	Message string `json:"message"` // error payloads outbound
}

type Room struct {
	mu  sync.Mutex
	gen int // bumped on every phase transition; timers carry their gen

	Code    string
	Phase   Phase
	Round   int
	hostID  string
	players []*Player
	conns   map[string]Client // playerID -> conn
	bets    []Bet
	fight   *Fight
	deckSeq int

	judge  Judge
	imager Imager
}

func New(code string, j Judge, img Imager) *Room {
	return &Room{
		Code:   code,
		Phase:  PhaseLobby,
		conns:  map[string]Client{},
		judge:  j,
		imager: img,
	}
}

// ---------- join / leave ----------

// avatarLocked changes a player's avatar (lobby only).
func (r *Room) avatarLocked(playerID, key string) {
	if r.Phase != PhaseLobby {
		return
	}
	if p := r.findLocked(playerID); p != nil && AvatarURL(key) != "" {
		p.Avatar = key
	}
}

// Join adds (or reconnects) a player. Returns the player ID.
func (r *Room) Join(name, avatar string, c Client) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.players {
		if p.Name == name {
			p.Connected = true
			r.conns[p.ID] = c
			r.broadcastLocked()
			return p.ID, nil
		}
	}
	if r.Phase != PhaseLobby {
		return "", fmt.Errorf("game already in progress")
	}
	if len(r.players) >= 8 {
		return "", fmt.Errorf("room is full")
	}
	p := &Player{
		ID:        fmt.Sprintf("p%d", len(r.players)+1),
		Name:      name,
		Avatar:    avatar,
		Coins:     startCoins,
		Connected: true,
		Hand:      r.dealHandLocked(),
	}
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
		r.draftLocked(playerID, m.Adj, m.Noun)
	case "bet":
		r.betLocked(playerID, m.On, m.Amount)
	case "verb":
		r.verbLocked(playerID, m.Verb)
	case "avatar":
		r.avatarLocked(playerID, m.Avatar)
	case "next":
		r.nextLocked(playerID)
	case "pass":
		r.passLocked(playerID)
	}
	r.broadcastLocked()
}

func (r *Room) sendErr(playerID, msg string) {
	if c := r.conns[playerID]; c != nil {
		_ = c.SendJSON(ClientMsg{Type: "error", Message: msg})
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

func (r *Room) draftLocked(playerID, adjID, nounID string) {
	if r.Phase != PhaseDraft {
		return
	}
	p := r.findLocked(playerID)
	if p == nil || p.Champion != nil {
		return
	}
	adj := takeCard(p, adjID, KindAdj)
	noun := takeCard(p, nounID, KindNoun)
	if adj == "" || noun == "" {
		r.sendErr(playerID, "invalid card choice")
		return
	}
	p.Champion = &Champion{Adj: adj, Noun: noun}
	champ := p.Champion
	prompt := adj + " " + noun + ", fantasy battle champion, dramatic digital art"
	go r.genImage(prompt, func(url string) {
		if p.Champion == champ && champ.ImageURL == "" {
			champ.ImageURL = url
			r.broadcastLocked()
		}
	})
	r.maybeAdvanceLocked()
	r.broadcastLocked()
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

func (r *Room) verbLocked(playerID, cardID string) {
	if r.Phase != PhaseCombat || r.fight == nil {
		return
	}
	if playerID != r.fight.A && playerID != r.fight.B {
		return
	}
	p := r.findLocked(playerID)
	if p == nil {
		return
	}
	if countMoves(r.fight.Moves, playerID) >= verbsPerFighter {
		r.sendErr(playerID, "out of moves")
		return
	}
	verb := takeCard(p, cardID, KindVerb)
	if verb == "" {
		r.sendErr(playerID, "invalid verb card")
		return
	}
	r.fight.Moves = append(r.fight.Moves, Move{PlayerID: playerID, Verb: verb})
	r.maybeAdvanceLocked()
	r.broadcastLocked()
}

func (r *Room) nextLocked(playerID string) {
	if r.Phase != PhaseVerdict || playerID != r.hostID {
		return
	}
	r.toDraftLocked()
	r.broadcastLocked()
}

func (r *Room) passLocked(playerID string) {
	p := r.findLocked(playerID)
	if p == nil {
		return
	}
	switch r.Phase {
	case PhaseBetting:
		r.betLocked(playerID, "", 0)
	case PhaseCombat:
		if playerID == r.fight.A || playerID == r.fight.B {
			p.Passed = true
			r.maybeAdvanceLocked()
			r.broadcastLocked()
		}
	}
}

// ---------- phase machine ----------

func (r *Room) toDraftLocked() {
	r.Round++
	r.bets = nil
	r.fight = nil
	r.Phase = PhaseDraft
	for _, p := range r.players {
		p.Champion = nil
		p.Passed = false
		r.topUpHandLocked(p)
	}
	r.armTimerLocked(draftTimeout)
}

func (r *Room) toBettingLocked() {
	elig := r.championsLocked()
	if len(elig) < 2 {
		// not enough champions; everyone re-drafts next round
		r.toDraftLocked()
		return
	}
	i := (r.Round - 1) % len(elig)
	j := (i + 1) % len(elig)
	r.fight = &Fight{A: elig[i].ID, B: elig[j].ID}
	for _, p := range r.players {
		p.Passed = false
	}
	if len(r.players) <= 2 {
		// nobody to bet; skip to combat
		r.toCombatLocked()
		return
	}
	r.Phase = PhaseBetting
	r.armTimerLocked(betTimeout)
}

func (r *Room) toCombatLocked() {
	for _, p := range r.players {
		p.Passed = false
	}
	r.Phase = PhaseCombat
	r.armTimerLocked(combatTimeout)
}

func (r *Room) toVerdictLocked() {
	r.Phase = PhaseVerdict
	r.armTimerLocked(verdictTimeout)
	gen := r.gen
	f := r.fight
	a := r.findLocked(f.A)
	b := r.findLocked(f.B)
	go func() {
		req := JudgeRequest{
			A:      *a.Champion,
			B:      *b.Champion,
			AID:    f.A,
			BID:    f.B,
			MovesA: verbsOf(f.Moves, f.A),
			MovesB: verbsOf(f.Moves, f.B),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		v, err := r.judge.Judge(ctx, req)
		cancel()
		if err != nil || v == nil || (v.WinnerID != f.A && v.WinnerID != f.B) {
			log.Printf("room %s: judge failed (%v), using fallback verdict", r.Code, err)
			v = fallbackVerdict(f, a.Champion, b.Champion)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen != gen {
			return // phase moved on (e.g. host skipped)
		}
		f.Verdict = v
		r.payoutLocked(v.WinnerID)
		// event art, async
		for i := 0; i < len(v.Events) && i < maxEventImages; i++ {
			go r.genImage(v.Events[i].ImagePrompt+", fantasy battle scene, dramatic digital art", func(url string) {
				if r.fight == f && f.Verdict != nil && i < len(f.Verdict.Events) {
					f.Verdict.Events[i].ImageURL = url
					r.broadcastLocked()
				}
			})
		}
		r.broadcastLocked()
	}()
}

// maybeAdvanceLocked auto-progresses phases once all required inputs are in.
func (r *Room) maybeAdvanceLocked() {
	switch r.Phase {
	case PhaseDraft:
		for _, p := range r.players {
			if p.Connected && p.Champion == nil {
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
	case PhaseCombat:
		if r.fight == nil {
			return
		}
		for _, fid := range []string{r.fight.A, r.fight.B} {
			p := r.findLocked(fid)
			if p == nil || !p.Connected {
				continue
			}
			if !p.Passed && countMoves(r.fight.Moves, fid) < verbsPerFighter {
				return
			}
		}
		r.toVerdictLocked()
	}
}

// armTimerLocked schedules a phase timeout. Call with r.mu held.
func (r *Room) armTimerLocked(d time.Duration) {
	r.gen++
	gen, phase := r.gen, r.Phase
	log.Printf("room %s -> %s (round %d)", r.Code, phase, r.Round)
	time.AfterFunc(d, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.gen != gen || r.Phase != phase {
			return
		}
		r.timeoutLocked()
	})
}

func (r *Room) timeoutLocked() {
	switch r.Phase {
	case PhaseDraft:
		// players who never drafted get a random champion from their hand
		for _, p := range r.players {
			if p.Connected && p.Champion == nil {
				p.Champion = randomChampion(p)
			}
		}
		r.toBettingLocked()
	case PhaseBetting:
		r.toCombatLocked()
	case PhaseCombat:
		r.toVerdictLocked()
	case PhaseVerdict:
		r.toDraftLocked()
	}
	r.broadcastLocked()
}

func (r *Room) payoutLocked(winnerID string) {
	for _, b := range r.bets {
		if b.Amount == 0 {
			continue
		}
		if b.On == winnerID {
			if p := r.findLocked(b.BettorID); p != nil {
				p.Coins += b.Amount * betOdds
			}
		}
	}
	if w := r.findLocked(winnerID); w != nil {
		w.Coins += winBonus
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

func (r *Room) dealHandLocked() []Card {
	var h []Card
	deal := func(pool []string, kind string, n int) {
		for i := 0; i < n; i++ {
			r.deckSeq++
			h = append(h, Card{ID: fmt.Sprintf("c%d", r.deckSeq), Text: pool[rand.IntN(len(pool))], Kind: kind})
		}
	}
	deal(adjectivePool, KindAdj, handAdjs)
	deal(nounPool, KindNoun, handNouns)
	deal(verbPool, KindVerb, handVerbs)
	return h
}

func (r *Room) topUpHandLocked(p *Player) {
	counts := map[string]int{KindAdj: 0, KindNoun: 0, KindVerb: 0}
	for _, c := range p.Hand {
		counts[c.Kind]++
	}
	targets := map[string]int{KindAdj: handAdjs, KindNoun: handNouns, KindVerb: handVerbs}
	pools := map[string][]string{KindAdj: adjectivePool, KindNoun: nounPool, KindVerb: verbPool}
	for kind, want := range targets {
		for counts[kind] < want {
			r.deckSeq++
			p.Hand = append(p.Hand, Card{ID: fmt.Sprintf("c%d", r.deckSeq), Text: pools[kind][rand.IntN(len(pools[kind]))], Kind: kind})
			counts[kind]++
		}
	}
}

func takeCard(p *Player, id, kind string) string {
	for i, c := range p.Hand {
		if c.ID == id && c.Kind == kind {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return c.Text
		}
	}
	return ""
}

func countMoves(moves []Move, pid string) int {
	n := 0
	for _, m := range moves {
		if m.PlayerID == pid {
			n++
		}
	}
	return n
}

func verbsOf(moves []Move, pid string) []string {
	var out []string
	for _, m := range moves {
		if m.PlayerID == pid {
			out = append(out, m.Verb)
		}
	}
	return out
}

func randomChampion(p *Player) *Champion {
	adj, noun := "", ""
	for _, c := range p.Hand {
		if c.Kind == KindAdj && adj == "" {
			adj = c.Text
		}
		if c.Kind == KindNoun && noun == "" {
			noun = c.Text
		}
	}
	if adj == "" {
		adj = adjectivePool[rand.IntN(len(adjectivePool))]
	}
	if noun == "" {
		noun = nounPool[rand.IntN(len(nounPool))]
	}
	return &Champion{Adj: adj, Noun: noun}
}

func fallbackVerdict(f *Fight, a, b *Champion) *Verdict {
	winner := f.A
	if rand.IntN(2) == 1 {
		winner = f.B
	}
	loser := b.Name()
	if winner == f.B {
		loser = a.Name()
	}
	return &Verdict{
		WinnerID: winner,
		Reason:   "The judges were unreachable, so fate decided.",
		Events: []Event{
			{Text: fmt.Sprintf("%s and %s collide in a blinding flash of slop.", a.Name(), b.Name()), ImagePrompt: "two fantasy monsters colliding"},
			{Text: fmt.Sprintf("%s stands triumphant over the fallen %s.", winnerName(winner, f, a, b), loser), ImagePrompt: "victorious fantasy champion"},
		},
	}
}

func winnerName(id string, f *Fight, a, b *Champion) string {
	if id == f.A {
		return a.Name()
	}
	return b.Name()
}

// genImage calls the imager off-lock, then applies the result under r.mu.
// The apply closure is responsible for staleness checks.
func (r *Room) genImage(prompt string, apply func(string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	url, err := r.imager.Generate(ctx, prompt)
	cancel()
	if err != nil {
		log.Printf("room %s: image gen failed: %v", r.Code, err)
		return
	}
	r.mu.Lock()
	apply(url)
	r.mu.Unlock()
}
