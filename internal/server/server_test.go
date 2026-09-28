package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"slop-battlegrounds/internal/game"
)

type testClient struct {
	t    *testing.T
	conn *websocket.Conn
	ctx  context.Context
	ch   chan stateMsg
	you  string // filled on first state
}

// mirror of the server stateMsg wire shape
type stateMsg struct {
	Type    string     `json:"type"`
	Message string     `json:"message"`
	Room    string     `json:"room"`
	Phase   game.Phase `json:"phase"`
	Round   int        `json:"round"`
	You     string     `json:"you"`
	Players []struct {
		ID       string         `json:"id"`
		Coins    int            `json:"coins"`
		Champion *game.Champion `json:"champion"`
	} `json:"players"`
	Bets []struct {
		BettorID string `json:"bettorId"`
		On       string `json:"on"`
		Amount   int    `json:"amount"`
	} `json:"bets"`
	Fight *struct {
		A struct {
			PlayerID string `json:"playerId"`
			Moves    []struct {
				Verb string `json:"verb"`
			} `json:"moves"`
		} `json:"a"`
		B struct {
			PlayerID string `json:"playerId"`
			Moves    []struct {
				Verb string `json:"verb"`
			} `json:"moves"`
		} `json:"b"`
		Location  string            `json:"location"`
		Scene     string            `json:"scene"`
		Round     int               `json:"round"`
		Resolving bool              `json:"resolving"`
		Events    []game.RoundEvent `json:"events"`
		Draw      bool              `json:"draw"`
		MyOptions []string          `json:"myOptions"`
		Verdict   *game.Verdict     `json:"verdict"`
	} `json:"fight"`
	LocOptions []string             `json:"locOptions"`
	Hand       []game.Card          `json:"hand"`
	Templates  []game.ForgeTemplate `json:"templates"`
	Shop       []game.ForgeTemplate `json:"shop"`
}

func dial(t *testing.T, url string, join JoinMsg) *testClient {
	t.Helper()
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &testClient{t: t, conn: conn, ctx: context.Background(), ch: make(chan stateMsg, 256)}
	join.Type = "join"
	c.send(join)
	go func() {
		for {
			var m stateMsg
			if err := wsjson.Read(c.ctx, c.conn, &m); err != nil {
				close(c.ch)
				return
			}
			c.ch <- m
		}
	}()
	return c
}

func (c *testClient) send(v any) {
	c.t.Helper()
	if err := wsjson.Write(c.ctx, c.conn, v); err != nil {
		c.t.Fatalf("send: %v", err)
	}
}

func (c *testClient) close() {
	c.conn.Close(websocket.StatusNormalClosure, "")
}

// wait pulls states until pred matches.
func (c *testClient) wait(pred func(stateMsg) bool) stateMsg {
	c.t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case m, ok := <-c.ch:
			if !ok {
				c.t.Fatalf("connection closed")
			}
			if m.Type == "error" {
				c.t.Fatalf("server error: %s", m.Message)
			}
			if m.You != "" {
				c.you = m.You
			}
			if pred(m) {
				return m
			}
		case <-timer.C:
			c.t.Fatalf("timed out waiting for state")
		}
	}
}

func pickCards(hand []game.Card, kind string) []string {
	var ids []string
	for _, c := range hand {
		if c.Kind == kind {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func TestFullGameLoop(t *testing.T) {
	hub := NewHub(Defaults{GenDir: t.TempDir()})
	srv := httptest.NewServer(hub.Handler(nil))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	// Player 1 creates a room (becomes host).
	c1 := dial(t, wsURL, JoinMsg{Name: "alice"})
	defer c1.close()
	s := c1.wait(func(m stateMsg) bool { return m.You != "" })
	room := s.Room
	if room == "" {
		t.Fatal("no room code")
	}
	if len(pickCards(s.Hand, game.KindAdj)) == 0 || len(pickCards(s.Hand, game.KindNoun)) == 0 {
		t.Fatal("no hand dealt")
	}

	// Player 2 joins it.
	c2 := dial(t, wsURL, JoinMsg{Name: "bob", Room: room})
	defer c2.close()
	c2.wait(func(m stateMsg) bool { return m.You != "" && len(m.Players) == 2 })

	// Host starts → draft.
	c1.send(game.ClientMsg{Type: "start"})
	s1 := c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft })
	s2 := c2.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft })

	// Both draft champions.
	c1.send(game.ClientMsg{Type: "draft", Template: "adj_noun", Cards: []string{pickCards(s1.Hand, game.KindAdj)[0], pickCards(s1.Hand, game.KindNoun)[0]}})
	c2.send(game.ClientMsg{Type: "draft", Template: "adj_noun", Cards: []string{pickCards(s2.Hand, game.KindAdj)[0], pickCards(s2.Hand, game.KindNoun)[0]}})

	// 2 players → betting skipped → combat.
	s1 = c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseCombat })
	s2 = c2.wait(func(m stateMsg) bool { return m.Phase == game.PhaseCombat })
	if s1.Fight == nil {
		t.Fatal("no fight in combat phase")
	}

	// Combat: wait for LLM options, then P1 picks one and P2 lets fate
	// decide — mock judge calls it on round 1 (lopsided) → early verdict.
	s1 = c1.wait(func(m stateMsg) bool {
		return m.Phase == game.PhaseCombat && m.Fight != nil && len(m.Fight.MyOptions) > 0
	})
	c1.send(game.ClientMsg{Type: "verb", Verb: s1.Fight.MyOptions[0]})
	c2.send(game.ClientMsg{Type: "pass"})

	// Mock judge resolves → verdict.
	s1 = c1.wait(func(m stateMsg) bool {
		return m.Phase == game.PhaseVerdict && m.Fight != nil && m.Fight.Verdict != nil
	})
	v := s1.Fight.Verdict
	if v.WinnerID != s1.Fight.A.PlayerID {
		t.Fatalf("winner = %q, want %q (the fighter who acted)", v.WinnerID, s1.Fight.A.PlayerID)
	}
	if len(s1.Fight.Events) == 0 {
		t.Fatal("no round events narrated")
	}

	// Payout: winner +25, loser +20.
	coins := map[string]int{}
	for _, p := range s1.Players {
		coins[p.ID] = p.Coins
	}
	if coins[v.WinnerID] != 125 {
		t.Fatalf("winner coins = %d, want 125", coins[v.WinnerID])
	}
	loser := s1.Fight.A.PlayerID
	if v.WinnerID == loser {
		loser = s1.Fight.B.PlayerID
	}
	if coins[loser] != 120 {
		t.Fatalf("loser coins = %d, want 120", coins[loser])
	}

	// Everyone must pass the verdict before the shop opens.
	c1.send(game.ClientMsg{Type: "pass"})
	c2.send(game.ClientMsg{Type: "pass"})
	s1 = c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseShop })
	if len(s1.Shop) == 0 {
		t.Fatal("no shop catalog in shop phase")
	}
	if len(s1.Templates) != 3 {
		t.Fatalf("starter templates = %d, want 3", len(s1.Templates))
	}
	s2shop := c2.wait(func(m stateMsg) bool { return m.Phase == game.PhaseShop })
	// Simultaneous shop: anyone may buy; templates are single-stock.
	shopC, otherC := c1, c2
	shopS := s1
	if s1.You == v.WinnerID {
		shopC, otherC = c2, c1
		shopS = s2shop
	}
	// The verdict loser (105 coins) buys the 45-coin noun_with_adj_noun template.
	shopC.send(game.ClientMsg{Type: "buy", Template: "noun_with_adj_noun"})
	bs := shopC.wait(func(m stateMsg) bool { return len(m.Templates) == 4 })
	var me struct {
		ID       string         `json:"id"`
		Coins    int            `json:"coins"`
		Champion *game.Champion `json:"champion"`
	}
	for _, p := range bs.Players {
		if p.ID == shopS.You {
			me = p
		}
	}
	if me.Coins != 75 {
		t.Fatalf("loser coins after buy = %d, want 75 (120-45)", me.Coins)
	}
	// Both pass → +2 parting cards each → next draft.
	shopC.send(game.ClientMsg{Type: "pass"})
	otherC.send(game.ClientMsg{Type: "pass"})
	sd1 := c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft })
	if sd1.Round != 2 {
		t.Fatalf("round = %d, want 2", sd1.Round)
	}
	// Hands are finite: no refill — the forged cards are gone for good.
	if len(sd1.Hand) == 0 {
		t.Fatal("hand should persist across rounds, minus spent cards")
	}
}

func TestBettingWithThreePlayers(t *testing.T) {
	hub := NewHub(Defaults{GenDir: t.TempDir()})
	srv := httptest.NewServer(hub.Handler(nil))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	c1 := dial(t, wsURL, JoinMsg{Name: "alice"})
	defer c1.close()
	s := c1.wait(func(m stateMsg) bool { return m.You != "" })
	room := s.Room

	c2 := dial(t, wsURL, JoinMsg{Name: "bob", Room: room})
	defer c2.close()
	c3 := dial(t, wsURL, JoinMsg{Name: "carol", Room: room})
	defer c3.close()
	c3.wait(func(m stateMsg) bool { return len(m.Players) == 3 })

	c1.send(game.ClientMsg{Type: "start"})

	// Everyone drafts.
	// Draft: the fight is already announced — the two fighters forge, the
	// spectator's forge is rejected.
	states := map[string]stateMsg{}
	for _, c := range []*testClient{c1, c2, c3} {
		st := c.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft && m.Fight != nil })
		states[st.You] = st
	}
	fighterIDs := map[string]bool{}
	var spectator *testClient
	for _, c := range []*testClient{c1, c2, c3} {
		st := states[c.you]
		if st.Fight.A.PlayerID == c.you || st.Fight.B.PlayerID == c.you {
			fighterIDs[c.you] = true
			c.send(game.ClientMsg{
				Type: "draft", Template: "adj_noun",
				Cards: []string{pickCards(st.Hand, game.KindAdj)[0], pickCards(st.Hand, game.KindNoun)[0]},
			})
		} else {
			spectator = c
		}
	}
	if spectator == nil {
		t.Fatal("no spectator found")
	}

	// Betting phase expected once both fighters forge.
	s = c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseBetting })
	if s.Fight == nil {
		t.Fatal("no fight in betting phase")
	}
	fighterIDs = map[string]bool{s.Fight.A.PlayerID: true, s.Fight.B.PlayerID: true}

	// The spectator bets 50 on fighter A.
	spectator.send(game.ClientMsg{Type: "bet", On: s.Fight.A.PlayerID, Amount: 50})

	// Combat begins once all spectators have bet.
	s = spectator.wait(func(m stateMsg) bool { return m.Phase == game.PhaseCombat })
	for _, p := range s.Players {
		if p.ID == spectator.you && p.Coins != 50 {
			t.Fatalf("spectator coins = %d, want 50 after staking 50", p.Coins)
		}
	}
}
func TestDrawAfterThreeRounds(t *testing.T) {
	hub := NewHub(Defaults{GenDir: t.TempDir()})
	srv := httptest.NewServer(hub.Handler(nil))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	c1 := dial(t, wsURL, JoinMsg{Name: "alice"})
	defer c1.close()
	s := c1.wait(func(m stateMsg) bool { return m.You != "" })

	c2 := dial(t, wsURL, JoinMsg{Name: "bob", Room: s.Room})
	defer c2.close()
	c2.wait(func(m stateMsg) bool { return len(m.Players) == 2 })

	c1.send(game.ClientMsg{Type: "start"})
	s1 := c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft })
	s2 := c2.wait(func(m stateMsg) bool { return m.Phase == game.PhaseDraft })
	c1.send(game.ClientMsg{Type: "draft", Template: "adj_noun", Cards: []string{pickCards(s1.Hand, game.KindAdj)[0], pickCards(s1.Hand, game.KindNoun)[0]}})
	c2.send(game.ClientMsg{Type: "draft", Template: "adj_noun", Cards: []string{pickCards(s2.Hand, game.KindAdj)[0], pickCards(s2.Hand, game.KindNoun)[0]}})

	c1.wait(func(m stateMsg) bool { return m.Phase == game.PhaseCombat })

	// Both fighters pick real options every round — mock scores a draw.
	for round := 1; round <= 3; round++ {
		st1 := c1.wait(func(m stateMsg) bool {
			return m.Fight != nil && m.Fight.Round == round && !m.Fight.Resolving && len(m.Fight.MyOptions) > 0
		})
		st2 := c2.wait(func(m stateMsg) bool {
			return m.Fight != nil && m.Fight.Round == round && !m.Fight.Resolving && len(m.Fight.MyOptions) > 0
		})
		c1.send(game.ClientMsg{Type: "verb", Verb: st1.Fight.MyOptions[0]})
		c2.send(game.ClientMsg{Type: "verb", Verb: st2.Fight.MyOptions[0]})
	}

	s1 = c1.wait(func(m stateMsg) bool {
		return m.Phase == game.PhaseVerdict && m.Fight != nil && m.Fight.Verdict != nil
	})
	if !s1.Fight.Draw {
		t.Fatal("expected draw flag after 3 undecided rounds")
	}
	if s1.Fight.Verdict.WinnerID != "" {
		t.Fatalf("draw verdict has winner %q", s1.Fight.Verdict.WinnerID)
	}
	if len(s1.Fight.Events) != 3 {
		t.Fatalf("expected 3 round events, got %d", len(s1.Fight.Events))
	}
	for _, p := range s1.Players {
		if p.Coins != 120 {
			t.Fatalf("player %s coins = %d, want 120 on a draw", p.ID, p.Coins)
		}
	}
}
