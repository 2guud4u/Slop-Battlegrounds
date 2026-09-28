package game

import (
	"fmt"
	"log"
	"math/rand/v2"
	"strings"
)

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
	p := r.findLocked(playerID)
	t := TemplateByID(tmplID)
	if p == nil || t == nil || t.Cost == 0 {
		return
	}
	if p.Templates[tmplID] {
		r.sendErr(playerID, "already owned")
		return
	}
	if r.soldTemplates[tmplID] {
		r.sendErr(playerID, "sold out — someone grabbed it first")
		return
	}
	if p.Coins < t.Cost {
		r.sendErr(playerID, "not enough coins")
		return
	}
	p.Coins -= t.Cost
	p.Templates[tmplID] = true
	r.soldTemplates[tmplID] = true // one copy only — off the shelf
	log.Printf("room %s: %s bought forge template %s", r.Code, p.Name, tmplID)
}

// buyPackLocked: a 3-card pack of one kind. Buying past handMax is fine —
// the player trims back down when they finish shopping.
func (r *Room) buyPackLocked(playerID, kind string) {
	p := r.findLocked(playerID)
	if p == nil || cardPools[kind] == nil {
		return
	}
	if p.Coins < packCost {
		r.sendErr(playerID, "not enough coins")
		return
	}
	p.Coins -= packCost
	for i := 0; i < packSize; i++ {
		p.Hand = append(p.Hand, r.drawLocked(kind))
	}
	log.Printf("room %s: %s bought a %s pack (hand %d)", r.Code, p.Name, kind, len(p.Hand))
}

// discardLocked: only legal while over the cap — trimming after a pack binge.
func (r *Room) discardLocked(playerID, cardID string) {
	if r.Phase != PhaseShop {
		return
	}
	p := r.findLocked(playerID)
	if p == nil || len(p.Hand) <= handMax {
		return
	}
	takeCard(p, cardID, "")
	r.maybeAdvanceLocked()
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
		if p.Passed {
			return // already cashed out
		}
		p.Passed = true // done shopping — collect the parting cards
		r.shopCloseLocked(p)
		r.maybeAdvanceLocked()
	case PhaseCombat:
		f := r.fight
		if f == nil {
			return
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
