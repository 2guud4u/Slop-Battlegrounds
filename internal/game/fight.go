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
