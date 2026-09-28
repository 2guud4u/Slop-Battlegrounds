package llm

import (
	"context"
	"fmt"

	"slop-battlegrounds/internal/game"
)

// Mock judge: narrates each round from the picked options. Decides when
// exactly one fighter ceded to fate — the fighter who acted wins.
type Mock struct{}

var mockOpts = [][]string{
	{"unleashes a devastating haymaker", "summons a spectral dodongo", "does the worm, aggressively"},
	{"hurls a tankard of dragon grog", "deploys a rabid goose", "charges a haymaker while humming"},
	{"strikes a pose so hard it hurts", "flexes until the floor cracks", "sneezes acid"},
	{"roundhouse-kicks the moon", "weaponizes a firm handshake", "breathes on a microphone"},
}

func (Mock) Options(_ context.Context, req game.JudgeRequest) (game.RoundOptions, error) {
	i := (req.Round - 1) % len(mockOpts)
	j := (i + 2) % len(mockOpts)
	return game.RoundOptions{
		A: append([]string{}, mockOpts[i]...),
		B: append([]string{}, mockOpts[j]...),
	}, nil
}

func (Mock) Round(_ context.Context, req game.JudgeRequest) (*game.RoundEvent, error) {
	aAct := joinOr(req.MovesA, "does literally nothing")
	bAct := joinOr(req.MovesB, "does literally nothing")

	ev := &game.RoundEvent{Round: req.Round}
	switch {
	case req.FateA && req.FateB:
		ev.Text = fmt.Sprintf("Both champions refuse to lift a finger — fate drags them forward anyway. %s and %s circle %s while the crowd boos, then falls weirdly quiet.",
			req.A.Name(), req.B.Name(), req.Location)
	case req.FateA:
		ev.Text = fmt.Sprintf("%s leaves it to fate and somehow %s. Meanwhile %s %s! The arena loses its mind.",
			req.A.Name(), aAct, req.B.Name(), bAct)
	case req.FateB:
		ev.Text = fmt.Sprintf("%s wastes no time — %s! %s trusts the gods and ends up %s.",
			req.A.Name(), aAct, req.B.Name(), bAct)
	default:
		ev.Text = fmt.Sprintf("%s goes first — %s! Not to be outdone, %s answers by %s! The crowd is feral.",
			req.A.Name(), aAct, req.B.Name(), bAct)
	}

	// A fighter who ceded to fate loses when the other acted.
	if req.FateA != req.FateB {
		ev.Decided = true
		if req.FateB {
			ev.WinnerID = req.AID
		} else {
			ev.WinnerID = req.BID
		}
		ev.Text += " And just like that — it's a knockout!"
	}
	return ev, nil
}

func joinOr(moves []string, fallback string) string {
	if len(moves) == 0 {
		return fallback
	}
	return moves[0]
}

// Scene for the mock: one canned line referencing the champions and location.
func (Mock) Scene(_ context.Context, req game.JudgeRequest) (string, error) {
	loc := req.Location
	if loc == "" {
		loc = "a mysterious void"
	}
	return fmt.Sprintf("Welcome to %s! Tonight %s squares off against %s — the crowd is already insufferable.", loc, req.A.Name(), req.B.Name()), nil
}

// Locations for the mock: a shuffled slice of the canned battleground pool.
func (Mock) Locations(_ context.Context, n int, _ string) ([]string, error) {
	return game.PickBattlegrounds(n), nil
}
