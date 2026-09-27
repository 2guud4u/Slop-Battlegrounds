package game

// Public/wire views. Player secrets (hand) only go to their owner; host API
// keys never leave the Config struct.

type playerView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"` // avatar key
	AvatarURL string    `json:"avatarUrl"`
	Coins     int       `json:"coins"`
	Connected bool      `json:"connected"`
	Host      bool      `json:"host"`
	Champion  *Champion `json:"champion"`
	Drafted   bool      `json:"drafted"`
	HandSize  int       `json:"handSize"`
}
type betView struct {
	BettorID string `json:"bettorId"`
	On       string `json:"on"`
	Amount   int    `json:"amount"`
	Pass     bool   `json:"pass"`
}

type fighterView struct {
	PlayerID  string   `json:"playerId"`
	Name      string   `json:"name"`
	AvatarURL string   `json:"avatarUrl"`
	Champion  Champion `json:"champion"`
	Verbs     []string `json:"verbs"`
}

type fightView struct {
	A       fighterView `json:"a"`
	B       fighterView `json:"b"`
	Verdict *Verdict    `json:"verdict"`
}

type stateMsg struct {
	Type    string       `json:"type"` // "state"
	Room    string       `json:"room"`
	Phase   Phase        `json:"phase"`
	Round   int          `json:"round"`
	You     string       `json:"you"`
	HostID  string       `json:"hostId"`
	Players []playerView `json:"players"`
	Bets    []betView    `json:"bets"`
	Fight   *fightView   `json:"fight"`
	Hand    []Card       `json:"hand"`
}

// broadcastLocked pushes a personalized snapshot to every connected player.
// Call with r.mu held.
func (r *Room) broadcastLocked() {
	for pid, c := range r.conns {
		_ = c.SendJSON(r.snapshotLocked(pid))
	}
}

func (r *Room) snapshotLocked(youID string) stateMsg {
	msg := stateMsg{
		Type:    "state",
		Room:    r.Code,
		Phase:   r.Phase,
		Round:   r.Round,
		You:     youID,
		HostID:  r.hostID,
		Players: []playerView{},
		Bets:    []betView{},
		Hand:    []Card{},
	}
	for _, p := range r.players {
		msg.Players = append(msg.Players, playerView{
			ID:        p.ID,
			Name:      p.Name,
			Avatar:    p.Avatar,
			AvatarURL: AvatarURL(p.Avatar),
			Coins:     p.Coins,
			Connected: p.Connected,
			Host:      p.ID == r.hostID,
			Champion:  p.Champion,
			Drafted:   p.Champion != nil,
			HandSize:  len(p.Hand),
		})
		if p.ID == youID {
			msg.Hand = p.Hand
		}
	}
	for _, b := range r.bets {
		msg.Bets = append(msg.Bets, betView{
			BettorID: b.BettorID,
			On:       b.On,
			Amount:   b.Amount,
			Pass:     b.Amount == 0,
		})
	}
	if r.fight != nil {
		fv := &fightView{Verdict: r.fight.Verdict}
		verbs := func(pid string) []string {
			if v := verbsOf(r.fight.Moves, pid); v != nil {
				return v
			}
			return []string{}
		}
		if a := r.findLocked(r.fight.A); a != nil && a.Champion != nil {
			fv.A = fighterView{PlayerID: a.ID, Name: a.Name, AvatarURL: AvatarURL(a.Avatar), Champion: *a.Champion, Verbs: verbs(a.ID)}
		}
		if b := r.findLocked(r.fight.B); b != nil && b.Champion != nil {
			fv.B = fighterView{PlayerID: b.ID, Name: b.Name, AvatarURL: AvatarURL(b.Avatar), Champion: *b.Champion, Verbs: verbs(b.ID)}
		}
		msg.Fight = fv
	}
	return msg
}
