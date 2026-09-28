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

func TestShopOverbuyThenDiscard(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1 := r.findLocked(ids[0])
	r.toShopLocked()
	// Full hand can still buy — the pack lands on top.
	for len(p1.Hand) < handMax {
		p1.Hand = append(p1.Hand, r.drawLocked(KindAdj))
	}
	r.buyPackLocked(ids[0], KindNoun)
	if len(p1.Hand) != handMax+packSize {
		t.Fatalf("hand = %d, want %d after overbuy", len(p1.Hand), handMax+packSize)
	}
	// Discards only legal while over the cap.
	r.discardLocked(ids[0], p1.Hand[0].ID)
	if len(p1.Hand) != handMax+packSize-1 {
		t.Fatalf("hand = %d after discard", len(p1.Hand))
	}
}

func TestShopDoneDealsTwoCardsAndEnds(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1, p2 := r.findLocked(ids[0]), r.findLocked(ids[1])
	p1.Hand, p2.Hand = p1.Hand[:3], p2.Hand[:4]
	r.toShopLocked()
	r.passLocked(ids[0]) // p1 done → +2 cards
	if len(p1.Hand) != 5 {
		t.Fatalf("hand = %d, want 3+2 parting cards", len(p1.Hand))
	}
	if r.Phase == PhaseShop {
		// still waiting on p2
		r.passLocked(ids[1])
	}
	if r.Phase == PhaseShop {
		t.Fatal("shop did not close after everyone passed")
	}
	if len(p2.Hand) != 6 {
		t.Fatalf("p2 hand = %d, want 4+2", len(p2.Hand))
	}
}

func TestShopDoneGuaranteesNoun(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1 := r.findLocked(ids[0])
	// Strip all nouns — the parting cards must include one.
	var keep []Card
	for _, c := range p1.Hand {
		if c.Kind != KindNoun {
			keep = append(keep, c)
		}
	}
	p1.Hand = keep
	r.toShopLocked()
	r.passLocked(ids[0])
	hasNoun := false
	for _, c := range p1.Hand[len(p1.Hand)-2:] {
		if c.Kind == KindNoun {
			hasNoun = true
		}
	}
	if !hasNoun {
		t.Fatal("parting cards dealt no noun to a noun-dry hand")
	}
}

func TestShopOvercapMustDiscardBeforeEnd(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1, p2 := r.findLocked(ids[0]), r.findLocked(ids[1])
	for len(p1.Hand) < handMax+2 {
		p1.Hand = append(p1.Hand, r.drawLocked(KindAdj))
	}
	p2.Hand = p2.Hand[:4] // 4 + 2 parting = 6 — under the cap, no discards owed
	r.toShopLocked()
	r.passLocked(ids[0])
	r.passLocked(ids[1])
	if r.Phase != PhaseShop {
		t.Fatalf("shop closed while %s is %d over the cap", ids[0], len(p1.Hand)-handMax)
	}
	for len(p1.Hand) > handMax { // 9 + 2 parting = 11 → 4 discards
		r.discardLocked(ids[0], p1.Hand[0].ID)
	}
	if r.Phase == PhaseShop {
		t.Fatal("shop still open after discards")
	}
}

func TestShopTemplateSoldOut(t *testing.T) {
	r, ids := testRoom(t, 2)
	p1, p2 := r.findLocked(ids[0]), r.findLocked(ids[1])
	p1.Coins, p2.Coins = 100, 100
	r.toShopLocked()
	r.shopLocked(ids[0], "noun_of_noun")
	if !r.soldTemplates["noun_of_noun"] {
		t.Fatal("template not marked sold")
	}
	r.shopLocked(ids[1], "noun_of_noun") // second buyer loses the race
	if p2.Templates["noun_of_noun"] {
		t.Fatal("sold-out template sold twice")
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
