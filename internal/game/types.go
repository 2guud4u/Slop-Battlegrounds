package game

import "context"

type Phase string

const (
	PhaseLobby   Phase = "lobby"
	PhaseDraft   Phase = "draft"
	PhaseBetting Phase = "betting"
	PhaseCombat  Phase = "combat"
	PhaseVerdict Phase = "verdict"
	PhaseShop    Phase = "shop" // forge-template store, after everyone's fought
	PhaseOver    Phase = "over" // nobody left can field two champions
)

// Card kinds.
// Card kinds — all eight parts of speech minus interjections.
const (
	KindAdj   = "adj"
	KindNoun  = "noun"
	KindVerb  = "verb"
	KindAdv   = "adv"   // adverb
	KindPron  = "pron"  // pronoun
	KindPrep  = "prep"  // preposition
	KindConj  = "conj"  // conjunction
	KindInter = "inter" // interjection — drawn from no pool, reserved
)

// Complexity levels for judge narration and action options.
const (
	LevelMiddle  = "middle"
	LevelHigh    = "high" // default
	LevelCollege = "college"
)

type Card struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type Champion struct {
	Text  string `json:"name"`  // forged name, literals included
	Cards []Card `json:"cards"` // the cards played, in order — rendered on the table
}

func (c Champion) Name() string { return c.Text }

type Player struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"` // avatar key, e.g. "tralalero"
	Coins     int       `json:"coins"`
	Wins      int       `json:"wins"`
	Connected bool      `json:"connected"`
	Champion  *Champion `json:"champion,omitempty"`

	Hand      []Card          `json:"-"` // delivered only to the owner via personalized snapshot
	Passed    bool            `json:"-"`
	Fought    bool            `json:"-"` // has fought at least once — gates the shop
	Templates map[string]bool `json:"-"` // owned forge-template ids
	Token     string          `json:"-"` // secret seat key — reconnects by token, sent only to the owner
}

type Bet struct {
	BettorID string `json:"bettorId"`
	On       string `json:"on"` // fighter player ID
	Amount   int    `json:"amount"`
}

// Move is one committed action in a combat round.
type Move struct {
	Round    int    `json:"round"`
	PlayerID string `json:"playerId"`
	Verb     string `json:"verb"`
	Fate     bool   `json:"fate"` // player passed — fate invented the action
}

// RoundEvent is the judged beat for one combat round: narration + art.
type RoundEvent struct {
	Round    int    `json:"round"`
	Text     string `json:"text"`
	ImageURL string `json:"imageUrl"`
	// Decided indicates the judge called a winner this round.
	Decided  bool   `json:"decided"`
	WinnerID string `json:"winnerId"`
}

type Verdict struct {
	WinnerID string `json:"winnerId"` // "" means draw
	Reason   string `json:"reason"`
}

// Fight is internal room state; the wire shape is built in snapshot().
type Fight struct {
	A, B       string         // player IDs
	Location   string         // battleground voted by the table
	Scene      string         // judge-narrated setup, set async when betting opens
	SceneImage string         // art for the setup, chained off the scene text
	SceneGen   bool           // scene gen kicked off — guards double dispatch
	Round      int            // 1..3, which action round is live
	Fate       map[string]int // playerID -> round they let fate decide
	Moves      []Move
	Events     []RoundEvent        // judged narration + art, one per completed round
	Options    map[string][]string // playerID -> 3 LLM-authored actions, refreshed per round
	OptRound   int                 // which round Options belong to
	Resolving  bool                // a round verdict is in flight
	Draw       bool                // nobody won after round 3
	Verdict    *Verdict
}
type JudgeRequest struct {
	A, B           Champion
	AID, BID       string
	MovesA, MovesB []string
	History        []string // narration of earlier rounds, oldest first
	FateA, FateB   bool     // that fighter let fate decide this round
	Location       string
	Round          int    // which action round (1..3) is being judged
	LastRound      bool   // true on round 3 — a draw becomes legal
	Level          string // narration complexity: middle|high|college
}

// Judge narrates the fight.
type Judge interface {
	// Scene writes the arena intro when combat begins.
	Scene(ctx context.Context, req JudgeRequest) (string, error)
	// Options proposes 3 actions per fighter for the round about to start.
	Options(ctx context.Context, req JudgeRequest) (RoundOptions, error)
	// Round resolves one combat round: narration text + whether someone won.
	Round(ctx context.Context, req JudgeRequest) (*RoundEvent, error)
	// Locations invents n battlegrounds for the draft vote.
	Locations(ctx context.Context, n int, level string) ([]string, error)
}

// RoundOptions: three action strings per fighter, tailored to them.
type RoundOptions struct {
	A []string `json:"a"`
	B []string `json:"b"`
}

// Imager returns a URL the client can load directly. Implementations either
// return a remote URL or write a file under the server's /gen/ route.
type Imager interface {
	Generate(ctx context.Context, prompt, ref string) (string, error)
}
