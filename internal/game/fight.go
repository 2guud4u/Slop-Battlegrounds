package game

import (
	"fmt"
	"math/rand/v2"
)

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
