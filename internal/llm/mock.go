package llm

import (
	"context"
	"fmt"
	"math/rand/v2"

	"slop-battlegrounds/internal/game"
)

// Mock judge: deterministic-free, picks the fighter with more moves (ties random)
// and narrates a full play-by-play using the actual moves played.
type Mock struct{}

func (Mock) Judge(_ context.Context, req game.JudgeRequest) (*game.Verdict, error) {
	aWins := len(req.MovesA) > len(req.MovesB)
	if len(req.MovesA) == len(req.MovesB) {
		aWins = rand.IntN(2) == 0
	}
	winner, loser, wid := req.A, req.B, req.AID
	wMoves, lMoves := req.MovesA, req.MovesB
	if !aWins {
		winner, loser, wid = req.B, req.A, req.BID
		wMoves, lMoves = req.MovesB, req.MovesA
	}

	events := []game.Event{
		{
			Text:        fmt.Sprintf("The arena lights die. A single spotlight finds %s — and across the pit, %s cracks its knuckles. The crowd holds its breath.", req.A.Name(), req.B.Name()),
			ImagePrompt: fmt.Sprintf("%s facing %s in a dim arena, dramatic spotlight", req.A.Name(), req.B.Name()),
		},
	}

	// Narrate each move with connective tissue (moves are gerunds).
	for i := 0; i < len(wMoves) || i < len(lMoves); i++ {
		if i < len(wMoves) {
			events = append(events, game.Event{
				Text:        fmt.Sprintf("%s takes the initiative — %s! The crowd goes feral.", winner.Name(), wMoves[i]),
				ImagePrompt: fmt.Sprintf("%s %s", winner.Name(), wMoves[i]),
			})
		}
		if i < len(lMoves) {
			events = append(events, game.Event{
				Text:        fmt.Sprintf("%s answers back — %s! The arena shakes.", loser.Name(), lMoves[i]),
				ImagePrompt: fmt.Sprintf("%s %s", loser.Name(), lMoves[i]),
			})
		}
	}
	if len(wMoves)+len(lMoves) == 0 {
		events = append(events, game.Event{
			Text:        "Neither champion lifts a finger. It's a psychological staredown for the ages — trainers are weeping, bookies are baffled.",
			ImagePrompt: "two monsters locked in an intense staredown",
		})
	}

	finisher := "delivers a final, devastating blow"
	if len(wMoves) > 0 {
		finisher = "ends it by " + wMoves[len(wMoves)-1]
	}
	events = append(events, game.Event{
		Text:        fmt.Sprintf("It's over. %s %s — %s crumples. %s stands alone under the falling confetti, victorious and barely winded.", winner.Name(), finisher, loser.Name(), winner.Name()),
		ImagePrompt: fmt.Sprintf("%s standing victorious over defeated %s, confetti falling", winner.Name(), loser.Name()),
	})

	return &game.Verdict{
		WinnerID: wid,
		Reason: fmt.Sprintf("%s overwhelmed %s%s.",
			winner.Name(), loser.Name(),
			map[bool]string{true: " with a relentless barrage", false: " by sheer force of presence"}[len(wMoves) > 0]),
		Events: events,
	}, nil
}
