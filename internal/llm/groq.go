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
	Model    string // e.g. "llama-3.3-70b-versatile"
	HTTP     *http.Client
	Endpoint string
}

func NewGroq(key string) *Groq {
	return &Groq{
		Key:      key,
		Model:    "llama-3.3-70b-versatile",
		HTTP:     &http.Client{},
		Endpoint: "https://api.groq.com/openai/v1/chat/completions",
	}
}

const judgePrompt = `You are the unhinged ringside announcer of a ridiculous monster fighting game.
Two champions fight. Each fighter may use verb-moves during combat to influence the outcome.
Pick the winner based on how awesome/funny/effective their moves are — be creative, not fair.

Champion A: %q
Champion B: %q
A's moves: %s
B's moves: %s

Respond with strict JSON only:
{"winner":"a" or "b","reason":"one punchy sentence summarizing why the winner deserved it","events":[{"text":"narration","image_prompt":"vivid image prompt for the scene"}]}

Write 4-6 events narrating the fight like a dramatic play-by-play, in chronological order:
- Open with the champions entering/sizing each other up.
- Each event = 1-2 sentences of vivid, specific narration. Reference the actual moves played and weaponize the absurdity of the champions' names (e.g. what does a Soggy Tax Auditor physically DO?).
- Escalate: mid-fight twists, crowd reactions, property damage.
- Final event = the decisive finishing blow and the victor standing triumphant.
- Keep it PG-13, theatrical, and absurd. No bullet points in the text fields.`

type groqReq struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
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

type judgeJSON struct {
	Winner string `json:"winner"`
	Reason string `json:"reason"`
	Events []struct {
		Text        string `json:"text"`
		ImagePrompt string `json:"image_prompt"`
	} `json:"events"`
}

func orNone(m []string) string {
	if len(m) == 0 {
		return "(none — they just stood there menacingly)"
	}
	return strings.Join(m, "; ")
}

func (g *Groq) Judge(ctx context.Context, req game.JudgeRequest) (*game.Verdict, error) {
	prompt := fmt.Sprintf(judgePrompt,
		req.A.Name(), req.B.Name(),
		orNone(req.MovesA), orNone(req.MovesB))

	var body groqReq
	body.Model = g.Model
	body.ResponseFormat.Type = "json_object"
	body.Messages = append(body.Messages,
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "user", Content: prompt})

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.Key)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("groq %d: %s", resp.StatusCode, string(data))
	}
	var gr groqResp
	if err := json.Unmarshal(data, &gr); err != nil {
		return nil, err
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("groq: %s", gr.Error.Message)
	}
	if len(gr.Choices) == 0 {
		return nil, fmt.Errorf("groq: empty choices")
	}
	var out judgeJSON
	if err := json.Unmarshal([]byte(gr.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("groq: bad verdict JSON: %w", err)
	}
	v := &game.Verdict{Reason: out.Reason}
	switch strings.ToLower(strings.TrimSpace(out.Winner)) {
	case "a":
		v.WinnerID = req.AID
	case "b":
		v.WinnerID = req.BID
	default:
		return nil, fmt.Errorf("groq: bad winner %q", out.Winner)
	}
	for _, e := range out.Events {
		v.Events = append(v.Events, game.Event{Text: e.Text, ImagePrompt: e.ImagePrompt})
	}
	if len(v.Events) == 0 {
		v.Events = []game.Event{{Text: out.Reason, ImagePrompt: "epic fantasy battle"}}
	}
	return v, nil
}
