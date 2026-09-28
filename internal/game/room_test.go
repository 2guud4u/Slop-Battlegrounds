package game

import (
	"context"
	"testing"
)

// stubJudge: silent judge — tests drive transitions directly.
type stubJudge struct{}

func (stubJudge) Scene(context.Context, JudgeRequest) (string, error) { return "arena", nil }
func (stubJudge) Options(context.Context, JudgeRequest) (RoundOptions, error) {
	return RoundOptions{A: []string{"x", "y", "z"}, B: []string{"x", "y", "z"}}, nil
}
func (stubJudge) Round(context.Context, JudgeRequest) (*RoundEvent, error) {
	return &RoundEvent{Text: "they clashed"}, nil
}
func (stubJudge) Locations(context.Context, int, string) ([]string, error) {
	return []string{"a", "b", "c", "d"}, nil
}

type stubImager struct{}

func (stubImager) Generate(context.Context, string, string) (string, error) { return "", nil }

type stubClient struct{}

func (stubClient) SendJSON(any) error { return nil }

func testRoom(t *testing.T, n int) (*Room, []string) {
	t.Helper()
	r := New("test", stubJudge{}, stubImager{})
	var ids []string
	for i := 0; i < n; i++ {
		id, err := r.Join("p"+string(rune('a'+i)), "av", stubClient{})
		if err != nil {
			t.Fatalf("join: %v", err)
		}
		ids = append(ids, id)
	}
	return r, ids
}

func TestHandDealtToSeven(t *testing.T) {
	r, ids := testRoom(t, 2)
	p := r.findLocked(ids[0])
	if len(p.Hand) > handMax {
		t.Fatalf("hand = %d cards, want <= %d", len(p.Hand), handMax)
	}
	nouns := 0
	for _, c := range p.Hand {
		if c.Kind == KindNoun {
			nouns++
		}
	}
	if nouns == 0 {
		t.Fatal("opening hand must contain nouns")
	}
}

func TestNoAutoRefillBetweenRounds(t *testing.T) {
	r, ids := testRoom(t, 2)
	p := r.findLocked(ids[0])
	p.Hand = p.Hand[:4] // spent cards stay spent
	before := len(p.Hand)
	r.toDraftLocked()
	if len(p.Hand) < before {
		t.Fatalf("hand shrank to %d, want >= %d", len(p.Hand), before)
	}
}

func TestNounSafetyReplenishes(t *testing.T) {
	r, ids := testRoom(t, 2)
	p := r.findLocked(ids[0])
	// drain all nouns
	var kept []Card
	for _, c := range p.Hand {
		if c.Kind != KindNoun {
			kept = append(kept, c)
		}
	}
	p.Hand = kept
	r.toDraftLocked()
	nouns := 0
	for _, c := range p.Hand {
		if c.Kind == KindNoun {
			nouns++
		}
	}
	if nouns != nounSafety {
		t.Fatalf("noun safety drew %d, want %d", nouns, nounSafety)
	}
	if len(p.Hand) > handMax {
		t.Fatalf("hand = %d, exceeds cap %d", len(p.Hand), handMax)
	}
}

func TestBuyPackRespectsTurnAndCap(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1, p2 := r.findLocked(ids[0]), r.findLocked(ids[1])
	p1.Wins, p2.Wins = 0, 2 // p1 shops first
	p1.Hand = p1.Hand[:2]
	r.toShopLocked()
	if got := r.shopTurnLocked(); got != ids[0] {
		t.Fatalf("first shop turn = %q, want %q (fewest wins)", got, ids[0])
	}
	// Out of turn buy is ignored.
	before := len(p2.Hand)
	r.buyPackLocked(ids[1], KindNoun)
	if len(p2.Hand) != before {
		t.Fatal("out-of-turn pack buy applied")
	}
	// In-turn buy works.
	r.buyPackLocked(ids[0], KindNoun)
	if len(p1.Hand) != 2+packSize {
		t.Fatalf("hand = %d, want %d", len(p1.Hand), 2+packSize)
	}
	// Fill to 7 then cap blocks the next pack.
	for len(p1.Hand) < handMax {
		p1.Hand = append(p1.Hand, r.drawLocked(KindAdj))
	}
	r.buyPackLocked(ids[0], KindAdj)
	if len(p1.Hand) != handMax {
		t.Fatalf("hand = %d, cap %d ignored", len(p1.Hand), handMax)
	}
}

func TestShopOrderFewestWinsThenCoins(t *testing.T) {
	r, ids := testRoom(t, 3)
	a, b, c := r.findLocked(ids[0]), r.findLocked(ids[1]), r.findLocked(ids[2])
	a.Wins, b.Wins, c.Wins = 2, 1, 1 // b/c tie on wins → coins decide
	b.Coins, c.Coins = 50, 30        // c is poorer → shops before b
	r.toShopLocked()
	want := []string{ids[2], ids[1], ids[0]}
	if len(r.shopQueue) != 3 {
		t.Fatalf("queue = %v", r.shopQueue)
	}
	for i, id := range want {
		if r.shopQueue[i] != id {
			t.Fatalf("queue[%d] = %q, want %q (order %v)", i, r.shopQueue[i], id, want)
		}
	}
}
func TestGameOverWhenNobodyCanForge(t *testing.T) {
	r, ids := testRoom(t, 2)
	// Only one player can field a champion: the other owns no templates.
	r.findLocked(ids[1]).Templates = map[string]bool{}
	r.findLocked(ids[1]).Coins = 200 // richest still takes the room
	r.Phase = PhaseDraft
	r.maybeAdvanceLocked() // → repick → no valid pair → over
	if r.Phase != PhaseOver {
		t.Fatalf("phase = %s, want over", r.Phase)
	}
	if r.WinnerID != ids[1] {
		t.Fatalf("winner = %q, want %q (richest)", r.WinnerID, ids[1])
	}
}

func TestRotationUnfoughtFirst(t *testing.T) {
	r, ids := testRoom(t, 3)
	r.findLocked(ids[0]).Fought = true // p1 fought last round
	r.toDraftLocked()
	got := map[string]bool{r.fight.A: true, r.fight.B: true}
	if !got[ids[1]] || !got[ids[2]] {
		t.Fatalf("bout = %v, want the two unfought players %s,%s", got, ids[1], ids[2])
	}
}

func TestOddPlayerGetsPastWinner(t *testing.T) {
	r, ids := testRoom(t, 3)
	// p1 and p2 already fought; p2 won a prior bout.
	r.findLocked(ids[0]).Fought = true
	r.findLocked(ids[1]).Fought = true
	r.findLocked(ids[1]).Wins = 1
	r.toDraftLocked()
	got := map[string]bool{r.fight.A: true, r.fight.B: true}
	if !got[ids[2]] {
		t.Fatalf("bout = %v, want the unfought player %s in it", got, ids[2])
	}
	if !got[ids[1]] {
		t.Fatalf("bout = %v, want past winner %s as the rematch opponent", got, ids[1])
	}
}
