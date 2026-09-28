package game

import (
	"fmt"
	"math/rand/v2"
)

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
