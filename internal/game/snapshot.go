package game

// Public/wire views. Player secrets (hand) only go to their owner; host API
// keys never leave the Config struct.

type playerView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"` // avatar key
	AvatarURL string    `json:"avatarUrl"`
	Coins     int       `json:"coins"`
	Wins      int       `json:"wins"`
	Connected bool      `json:"connected"`
	Host      bool      `json:"host"`
	Champion  *Champion `json:"champion"`
	Drafted   bool      `json:"drafted"`
	HandSize  int       `json:"handSize"`
	Ready     bool      `json:"ready"` // passed/ready in verdict+shop phases
}
type betView struct {
	BettorID string `json:"bettorId"`
	On       string `json:"on"`
	Amount   int    `json:"amount"`
	Pass     bool   `json:"pass"`
}

type fighterView struct {
	PlayerID  string     `json:"playerId"`
	Name      string     `json:"name"`
	AvatarURL string     `json:"avatarUrl"`
	Champion  Champion   `json:"champion"`
	Moves     []moveView `json:"moves"`
}

type moveView struct {
	Round int    `json:"round"`
	Verb  string `json:"verb"`
	Fate  bool   `json:"fate"`
}

type fightView struct {
	A          fighterView    `json:"a"`
	B          fighterView    `json:"b"`
	Location   string         `json:"location"`
	Scene      string         `json:"scene"`
	SceneImage string         `json:"sceneImage"`
	Round      int            `json:"round"`
	Fate       map[string]int `json:"fate"` // playerID -> round fate decided
	Events     []RoundEvent   `json:"events"`
	Resolving  bool           `json:"resolving"`
	Draw       bool           `json:"draw"`
	MyOptions  []string       `json:"myOptions"` // only set for the viewing fighter
	Verdict    *Verdict       `json:"verdict"`
}

type stateMsg struct {
	Type      string          `json:"type"` // "state"
	Room      string          `json:"room"`
	Phase     Phase           `json:"phase"`
	Round     int             `json:"round"`
	You       string          `json:"you"`
	Token     string          `json:"token"` // viewer's own seat key — stored client-side for reload
	HostID    string          `json:"hostId"`
	Players   []playerView    `json:"players"`
	WinnerID  string          `json:"winnerId"` // set when phase == "over"
	Bets      []betView       `json:"bets"`
	Fight     *fightView      `json:"fight"`
	Hand      []Card          `json:"hand"`
	Templates []ForgeTemplate `json:"templates"` // owned forge templates
	Shop      []ForgeTemplate `json:"shop"`      // buyable forge templates
	RevealAt  int64           `json:"revealAt"`  // unix ms — draft reveal ends, betting opens
	PackCost  int             `json:"packCost"`  // coins per 3-card pack

	LocOptions []string       `json:"locOptions"`
	LocVotes   map[string]int `json:"locVotes"` // option -> tally
	MyVote     string         `json:"myVote"`
	Level      string         `json:"level"` // narration complexity
}

// broadcastLocked pushes a personalized snapshot to every connected player.
// Call with r.mu held.
func (r *Room) broadcastLocked() {
	for pid, c := range r.conns {
		_ = c.SendJSON(r.snapshotLocked(pid))
	}
}

func (r *Room) tokenLocked(pid string) string {
	if p := r.findLocked(pid); p != nil {
		return p.Token
	}
	return ""
}

func (r *Room) snapshotLocked(youID string) stateMsg {
	msg := stateMsg{
		Level:      r.Complexity,
		Type:       "state",
		Room:       r.Code,
		Phase:      r.Phase,
		Round:      r.Round,
		You:        youID,
		Token:      r.tokenLocked(youID),
		HostID:     r.hostID,
		WinnerID:   r.WinnerID,
		RevealAt:   r.revealAt.UnixMilli(),
		PackCost:   packCost,
		Players:    []playerView{},
		Bets:       []betView{},
		Hand:       []Card{},
		LocOptions: r.locOptions,
		LocVotes:   map[string]int{},
		MyVote:     r.locVotes[youID],
	}
	for _, p := range r.players {
		msg.Players = append(msg.Players, playerView{
			ID:        p.ID,
			Name:      p.Name,
			Avatar:    p.Avatar,
			AvatarURL: AvatarURL(p.Avatar),
			Coins:     p.Coins,
			Wins:      p.Wins,
			Connected: p.Connected,
			Host:      p.ID == r.hostID,
			HandSize:  len(p.Hand),
			Ready:     p.Passed,
		})
		if p.ID == youID {
			msg.Hand = p.Hand
			for _, t := range forgeTemplates {
				if p.Templates[t.ID] {
					msg.Templates = append(msg.Templates, t)
				}
			}
		}
	}
	if r.Phase == PhaseShop {
		stock := make([]ForgeTemplate, len(forgeTemplates))
		copy(stock, forgeTemplates)
		for i := range stock {
			stock[i].Sold = r.soldTemplates[stock[i].ID]
		}
		msg.Shop = stock
	}
	for _, b := range r.bets {
		msg.Bets = append(msg.Bets, betView{
			BettorID: b.BettorID,
			On:       b.On,
			Amount:   b.Amount,
			Pass:     b.Amount == 0,
		})
	}
	for _, v := range r.locVotes {
		msg.LocVotes[v]++
	}
	if r.fight != nil {
		moves := func(pid string) []moveView {
			out := []moveView{}
			for _, m := range r.fight.Moves {
				if m.PlayerID == pid {
					out = append(out, moveView{Round: m.Round, Verb: m.Verb, Fate: m.Fate})
				}
			}
			return out
		}
		fv := &fightView{Verdict: r.fight.Verdict, Location: r.fight.Location, Scene: r.fight.Scene, SceneImage: r.fight.SceneImage, Round: r.fight.Round, Fate: r.fight.Fate, Resolving: r.fight.Resolving, Events: append([]RoundEvent{}, r.fight.Events...), Draw: r.fight.Draw, MyOptions: r.fight.Options[youID]}
		if a := r.findLocked(r.fight.A); a != nil {
			fv.A = fighterView{PlayerID: a.ID, Name: a.Name, AvatarURL: AvatarURL(a.Avatar), Moves: moves(a.ID)}
			if a.Champion != nil {
				fv.A.Champion = *a.Champion
			}
		}
		if b := r.findLocked(r.fight.B); b != nil {
			fv.B = fighterView{PlayerID: b.ID, Name: b.Name, AvatarURL: AvatarURL(b.Avatar), Moves: moves(b.ID)}
			if b.Champion != nil {
				fv.B.Champion = *b.Champion
			}
		}
		msg.Fight = fv
	}
	return msg
}
