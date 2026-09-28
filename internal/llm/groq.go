package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"slop-battlegrounds/internal/game"
)

// Groq judges fights via the Groq chat-completions API (OpenAI-compatible).
type Groq struct {
	Key      string
	Model    string // e.g. "openai/gpt-oss-120b"
	HTTP     *http.Client
	Endpoint string
}

func NewGroq(key string) *Groq {
	return &Groq{
		Key:      key,
		Model:    "openai/gpt-oss-120b",
		HTTP:     &http.Client{},
		Endpoint: "https://api.groq.com/openai/v1/chat/completions",
	}
}

const roundPrompt = `You are a funny, slightly unhinged ringside announcer who speaks like a real
person — a BATTLE TO THE DEATH. This is combat round %d of 3. Two champions
traded lethal actions.

Battleground: %q
Champion A: %q — actions this round: %s
Champion B: %q — actions this round: %s

Earlier rounds:
%s

Write a punchy 2-3 sentence play-by-play of this round only. Build on what
already happened (wounds, terrain destroyed, grudges). Reference the actual
moves and make the battleground matter when relevant.

IMPORTANT WRITING STYLE:
- Write like a funny human game announcer, not a fantasy novelist.
- Use simple, conversational language.
- Describe what happens first; add humor second.
- Keep sentences short and easy to scan.
- Use concrete verbs and nouns.
- Avoid adjective chains such as "glittering, neon-lit, sugar-crazed..."
- Use at most 1-2 descriptive adjectives per sentence.
- Avoid excessive metaphors, similes, and poetic descriptions.
- Do not make every action sound epic or catastrophic.
- Do not use phrases like "tearing through", "roar-like chorus",
  "shrouded in", "under a sky of", "makes the crowd howl", or similar
  trailer/fantasy-novel language unless the joke specifically calls for it.
- Let the ridiculous character names and actions provide the humor.
- Vary sentence structure naturally.
- Do not force an exclamation mark into every sentence.
- NEVER use flowery language just to make the scene sound exciting.
- The narration should feel spontaneous and readable in a game UI.

Then decide: is this fight DECIDED — one champion definitively killed,
knocked out cold, or banished? Be stingy; a tie or close call is not decided.%s

Writing level: %s

Respond with strict JSON only:
{"text":"the narration","over":true|false,"winner":"a"|"b"|""}
- "over":true only when the fight is definitively over; then "winner" is the
  victor. Otherwise over=false and winner="".`

const lastRoundRule = `
This is the FINAL round. If both champions somehow still stand, the judges
score it a draw — over=false, winner="". A draw means both survive, barely.`

const optionsPrompt = `You run the battle menu for a ridiculous monster fight to the death.

Battleground: %q
Champion A: %q — Champion B: %q
This is round %d of 3.

Earlier rounds:
%s

Invent 3 absurd, funny, deadly actions each champion could perform NEXT.
Tailor them to their ridiculous names, the battleground, and what already
happened (wounds, broken terrain, grudges).

ACTION STYLE:
- 3-9 words each.
- Start with a strong verb.
- Use simple, readable language.
- Make the action itself funny; don't rely on fancy adjectives.
- Specific and visual beats vague and dramatic.
- Avoid adjective chains.
- Avoid words like "devastating", "unstoppable", "legendary",
  "cataclysmic", "infernal", "mystical", and "ultimate" unless essential
  to the joke.
- No fantasy-novel prose.
- No narration or explanation.
- Each action should feel like something a player would actually choose
  from a game menu.
- PG-13.

Writing level: %s

Respond with strict JSON only:
{"a":["action","action","action"],"b":["action","action","action"]}`

const locationsPrompt = `Invent %d absurd, funny battlegrounds for a ridiculous monster fighting game.

Short noun phrases, 3-7 words each. Places where a fight would be funny
and dangerous.

STYLE:
- Use concrete, recognizable objects and places.
- Keep them easy to visualize.
- Prefer one funny idea over several stacked ideas.
- Avoid elaborate fantasy descriptions.
- Avoid adjective chains.
- No poetic language.
- No numbers, no quotes.

Writing level: %s

Respond with strict JSON only: {"locs":["place","place","place","place"]}`

type groqReq struct {
	Model               string `json:"model"`
	ReasoningEffort     string `json:"reasoning_effort,omitempty"` // gpt-oss: "low" keeps latency sane
	MaxCompletionTokens int    `json:"max_completion_tokens,omitempty"`
	Messages            []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type groqResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type roundJSON struct {
	Text   string `json:"text"`
	Over   bool   `json:"over"`
	Winner string `json:"winner"`
}

func orLoc(l string) string {
	if l == "" {
		return "a mysterious void"
	}
	return l
}

func orNone(m []string) string {
	if len(m) == 0 {
		return "(none — they just stood there menacingly)"
	}
	return strings.Join(m, "; ")
}

func historyOf(h []string) string {
	if len(h) == 0 {
		return "(none yet — this is the opening exchange)"
	}
	return strings.Join(h, "\n")
}

// levelGuide translates the room's complexity setting into prompt wording.
func levelGuide(level string) string {
	switch level {
	case game.LevelMiddle:
		return "1 = very simple, conversational game writing. Shortest sentences."
	case game.LevelCollege:
		return "4 = highly expressive but still concise and readable — dry wit, no purple prose."
	default: // high
		return "3 = punchy announcer style with occasional colorful language."
	}
}

func (g *Groq) Round(ctx context.Context, req game.JudgeRequest) (*game.RoundEvent, error) {
	tail := ""
	if req.LastRound {
		tail = lastRoundRule
	}
	prompt := fmt.Sprintf(roundPrompt,
		req.Round, orLoc(req.Location), req.A.Name(), orNone(req.MovesA),
		req.B.Name(), orNone(req.MovesB), historyOf(req.History), tail, levelGuide(req.Level))
	content, err := g.call(ctx, prompt, true)
	if err != nil {
		return nil, err
	}
	var out roundJSON
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("groq: bad round JSON: %w", err)
	}
	ev := &game.RoundEvent{Text: strings.TrimSpace(out.Text)}
	if ev.Text == "" {
		return nil, fmt.Errorf("groq: empty narration")
	}
	if out.Over {
		switch strings.ToLower(strings.TrimSpace(out.Winner)) {
		case "a":
			ev.Decided, ev.WinnerID = true, req.AID
		case "b":
			ev.Decided, ev.WinnerID = true, req.BID
		}
	}
	return ev, nil
}

// Options invents three per-fighter actions for the round.
func (g *Groq) Options(ctx context.Context, req game.JudgeRequest) (game.RoundOptions, error) {
	prompt := fmt.Sprintf(optionsPrompt, orLoc(req.Location), req.A.Name(), req.B.Name(), req.Round, historyOf(req.History), levelGuide(req.Level))
	content, err := g.call(ctx, prompt, true)
	if err != nil {
		return game.RoundOptions{}, err
	}
	var out game.RoundOptions
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return game.RoundOptions{}, fmt.Errorf("groq: bad options JSON: %w", err)
	}
	return out, nil
}

// Locations invents battlegrounds for the draft-phase vote.
func (g *Groq) Locations(ctx context.Context, n int, level string) ([]string, error) {
	prompt := fmt.Sprintf(locationsPrompt, n, levelGuide(level))
	content, err := g.call(ctx, prompt, true)
	if err != nil {
		return nil, err
	}
	var out struct {
		Locs []string `json:"locs"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("groq: bad locations JSON: %w", err)
	}
	var locs []string
	for _, l := range out.Locs {
		if s := strings.TrimSpace(l); s != "" {
			locs = append(locs, s)
		}
	}
	if len(locs) == 0 {
		return nil, fmt.Errorf("groq: empty locations")
	}
	return locs, nil
}

// call posts a single user message; jsonMode toggles response_format.
func (g *Groq) call(ctx context.Context, prompt string, jsonMode bool) (string, error) {
	var body groqReq
	body.Model = g.Model
	body.ReasoningEffort = "low" // don't burn the round on chains of thought
	body.MaxCompletionTokens = 800
	body.Messages = append(body.Messages, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: prompt})
	if jsonMode {
		body.ResponseFormat = &struct {
			Type string `json:"type"`
		}{Type: "json_object"}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.Key)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq %d: %s", resp.StatusCode, string(data))
	}
	var gr groqResp
	if err := json.Unmarshal(data, &gr); err != nil {
		return "", err
	}
	if gr.Error != nil {
		return "", fmt.Errorf("groq: %s", gr.Error.Message)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("groq: empty choices")
	}
	return gr.Choices[0].Message.Content, nil
}

// Scene asks the model for a one-paragraph intro: the battleground, the
// fighters entering, the stakes. Plain text, no JSON.
func (g *Groq) Scene(ctx context.Context, req game.JudgeRequest) (string, error) {
	prompt := fmt.Sprintf(`You are a funny, slightly unhinged ringside announcer who speaks like a real
person. The fight takes place in %q — a battle to the death. Champion A is %q;
champion B is %q.
Write exactly 2 punchy sentences setting the scene: the arena, the crowd, the
champions' entrances. Simple conversational language, concrete images, short
sentences — the absurd names carry the humor. No adjective chains, no
fantasy-novel prose, PG-13. Writing level: %s
Plain text only.`,
		orLoc(req.Location), req.A.Name(), req.B.Name(), levelGuide(req.Level))
	return g.call(ctx, prompt, false)
}
