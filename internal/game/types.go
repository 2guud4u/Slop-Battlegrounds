package game

import "context"

type Phase string

const (
	PhaseLobby   Phase = "lobby"
	PhaseDraft   Phase = "draft"
	PhaseBetting Phase = "betting"
	PhaseCombat  Phase = "combat"
	PhaseVerdict Phase = "verdict"
)

// Card kinds.
const (
	KindAdj  = "adj"
	KindNoun = "noun"
	KindVerb = "verb"
)

type Card struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type Champion struct {
	Adj      string `json:"adj"`
	Noun     string `json:"noun"`
	ImageURL string `json:"imageUrl"`
}

func (c Champion) Name() string { return c.Adj + " " + c.Noun }

type Player struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"` // avatar key, e.g. "tralalero"
	Coins     int       `json:"coins"`
	Connected bool      `json:"connected"`
	Champion  *Champion `json:"champion,omitempty"`

	Hand   []Card `json:"-"` // delivered only to the owner via personalized snapshot
	Passed bool   `json:"-"`
}

type Bet struct {
	BettorID string `json:"bettorId"`
	On       string `json:"on"` // fighter player ID
	Amount   int    `json:"amount"`
}

type Move struct {
	PlayerID string `json:"playerId"`
	Verb     string `json:"verb"`
}

type Event struct {
	Text        string `json:"text"`
	ImagePrompt string `json:"imagePrompt"`
	ImageURL    string `json:"imageUrl"`
}

type Verdict struct {
	WinnerID string  `json:"winnerId"`
	Reason   string  `json:"reason"`
	Events   []Event `json:"events"`
}

// Fight is internal room state; the wire shape is built in snapshot().
type Fight struct {
	A, B    string // player IDs
	Moves   []Move
	Verdict *Verdict
}
type JudgeRequest struct {
	A, B           Champion
	AID, BID       string
	MovesA, MovesB []string
}

type Judge interface {
	Judge(ctx context.Context, req JudgeRequest) (*Verdict, error)
}

// Imager returns a URL the client can load directly. Implementations either
// return a remote URL or write a file under the server's /gen/ route.
type Imager interface {
	Generate(ctx context.Context, prompt string) (string, error)
}
