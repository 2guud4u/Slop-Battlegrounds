package game

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
)

// Resilient wraps a Judge: live calls pass through; failures degrade to the
// canned local narration instead of erroring. The room never sees a judge
// outage — degraded beats carry a flag the wire shows as "judges unreachable".
//
// Every Judge given to New is wrapped, so call sites treat the seam as
// always-answering. The degradation policy lives here, not in the phase
// machine: three canned implementations used to be spread across the room
// (fallbackRound, sampleVerbs, PickBattlegrounds) — they're all behind the
// seam now.
type Resilient struct {
	Inner Judge
	Room  string // for degrade logs
}

func (r Resilient) Round(ctx context.Context, req JudgeRequest) (*RoundEvent, error) {
	ev, err := r.Inner.Round(ctx, req)
	if err != nil || ev == nil {
		log.Printf("room %s: round %d judge failed (%v), using fallback", r.Room, req.Round, err)
		ev = fallbackRound(req.AID, req.BID, req.A, req.B, req.MovesA, req.MovesB, req.Round)
		ev.Degraded = true
	}
	ev.Round = req.Round // the seam pins the round, not the provider
	return ev, nil
}

func (r Resilient) Options(ctx context.Context, req JudgeRequest) (RoundOptions, error) {
	opts, err := r.Inner.Options(ctx, req)
	if err != nil || len(opts.A) == 0 || len(opts.B) == 0 {
		log.Printf("room %s: options gen failed (%v), using fallback moves", r.Room, err)
		return RoundOptions{A: sampleVerbs(verbsPerFighter), B: sampleVerbs(verbsPerFighter)}, nil
	}
	return opts, nil
}

func (r Resilient) Locations(ctx context.Context, n int, level string) ([]string, error) {
	locs, err := r.Inner.Locations(ctx, n, level)
	if err != nil || len(locs) == 0 {
		return PickBattlegrounds(n), nil
	}
	return locs, nil
}

// Scene degrades to one canned welcome line rather than failing — the theater
// still gets its intro when the judge is down.
func (r Resilient) Scene(ctx context.Context, req JudgeRequest) (string, error) {
	s, err := r.Inner.Scene(ctx, req)
	if err != nil || s == "" {
		log.Printf("room %s: scene gen failed: %v", r.Room, err)
		loc := req.Location
		if loc == "" {
			loc = "a mysterious void"
		}
		return fmt.Sprintf("Welcome to %s! Tonight %s squares off against %s — the crowd is already insufferable.", loc, req.A.Name(), req.B.Name()), nil
	}
	return s, nil
}

// fallbackRound narrates a round when the judge is unreachable. Winner is
// picked at random (never on the last round — that yields a draw).
func fallbackRound(aID, bID string, a, b Champion, movesA, movesB []string, round int) *RoundEvent {
	ev := &RoundEvent{
		Round: round,
		Text: fmt.Sprintf("%s %s while %s %s — the judges are unreachable, but the crowd is roaring.",
			a.Name(), joinVerbs(movesA, "flails"), b.Name(), joinVerbs(movesB, "flails")),
	}
	if round < 3 && rand.IntN(4) == 0 {
		winner := aID
		if rand.IntN(2) == 1 {
			winner = bID
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
