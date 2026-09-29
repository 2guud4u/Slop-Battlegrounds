package game

import (
	"context"
	"log"
	"math/rand/v2"
	"time"
)

// allFoughtLocked: every connected player has been a fighter at least once.
func (r *Room) allFoughtLocked() bool {
	for _, p := range r.players {
		if p.Connected && !p.Fought {
			return false
		}
	}
	return true
}

// toShopLocked: everyone shops at once — no turn order. The phase ends when
// all connected players have taken their +2 cards and trimmed back to handMax.
func (r *Room) toShopLocked() {
	for _, p := range r.players {
		p.Passed = false // "done shopping" flag
		p.Fought = false // shop ends the cycle — rotation restarts after it
		if !p.Connected {
			r.shopCloseLocked(p) // ghosts skip straight to done
		}
	}
	r.Phase = PhaseShop
	r.enterLocked()
}

// shopCloseLocked deals the +2 parting cards — the first is a guaranteed noun
// when the hand is dry — then trims a disconnected hand down to handMax.
func (r *Room) shopCloseLocked(p *Player) {
	needNoun := true
	for _, c := range p.Hand {
		if c.Kind == KindNoun {
			needNoun = false
			break
		}
	}
	for i := 0; i < 2; i++ {
		kind := r.anyKindLocked()
		if needNoun && i == 0 {
			kind = KindNoun
		}
		p.Hand = append(p.Hand, r.drawLocked(kind))
	}
	for !p.Connected && len(p.Hand) > handMax { // ghosts can't pick a discard
		p.Hand = p.Hand[:len(p.Hand)-1]
	}
}

// anyKindLocked picks a random drawable kind.
func (r *Room) anyKindLocked() string {
	kinds := []string{KindNoun, KindAdj, KindVerb, KindAdv, KindPron, KindPrep, KindConj}
	return kinds[rand.IntN(len(kinds))]
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

func (r *Room) toDraftLocked() {
	r.Round++
	r.bets = nil
	r.fight = nil
	r.locVotes = map[string]string{}
	r.locOptions = nil       // filled async by the judge; falls back to the pool
	r.revealAt = time.Time{} // reveal timer only arms once both fighters forge
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

func (r *Room) toBettingLocked() {
	a, b := r.findLocked(r.fight.A), r.findLocked(r.fight.B)
	if a == nil || b == nil || a.Champion == nil || b.Champion == nil {
		// A fighter left or couldn't forge — re-draft for a fresh bout.
		r.toDraftLocked()
		return
	}
	if r.fight.Location == "" { // set at reveal arm; late votes can't repaint the scene
		r.fight.Location = r.winningLocationLocked()
	}
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
	r.enterLocked()
	r.genSceneLocked()   // no-op if betting already kicked it off
	r.genOptionsLocked() // first round's move set
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
		// Both champions are in — hold the draft screen for the reveal,
		// then the table moves to betting.
		if r.revealAt.IsZero() {
			r.revealAt = time.Now().Add(5 * time.Second)
			r.fight.Location = r.winningLocationLocked() // votes so far — scene gen can start
			r.genSceneLocked()                           // paint during the reveal, not after
			gen := r.gen
			time.AfterFunc(time.Until(r.revealAt), func() {
				r.mu.Lock()
				defer r.mu.Unlock()
				if r.gen != gen || r.Phase != PhaseDraft {
					return
				}
				r.maybeAdvanceLocked()
				r.broadcastLocked()
			})
			return
		}
		if time.Now().Before(r.revealAt) {
			return
		}
		r.revealAt = time.Time{}
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
		for _, p := range r.players {
			if p.Connected && (!p.Passed || len(p.Hand) > handMax) {
				return // still shopping, or owes discards
			}
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

// enterLocked bumps gen (stale-check token for async work) and logs the phase.
// Call with r.mu held, after Phase is set.
func (r *Room) enterLocked() {
	r.gen++
	log.Printf("room %s -> %s (round %d)", r.Code, r.Phase, r.Round)
}
