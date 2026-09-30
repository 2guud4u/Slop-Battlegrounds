package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"slop-battlegrounds/internal/game"
)

// Chat is the only thing a provider implements: send one user message, get
// the reply text. jsonMode asks the provider to force a JSON object reply.
type Chat interface {
	Complete(ctx context.Context, prompt string, jsonMode bool) (string, error)
}

// Judge implements game.Judge on top of any Chat — shared prompts, shared
// parsing, provider-specific transport.
type Judge struct {
	Chat Chat
}

func (j Judge) Scene(ctx context.Context, req game.JudgeRequest) (string, error) {
	return j.Chat.Complete(ctx, ScenePrompt(req), false)
}

func (j Judge) Round(ctx context.Context, req game.JudgeRequest) (*game.RoundEvent, error) {
	content, err := j.Chat.Complete(ctx, RoundPrompt(req), true)
	if err != nil {
		return nil, err
	}
	var out struct {
		Text   string `json:"text"`
		Over   bool   `json:"over"`
		Winner string `json:"winner"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("llm: bad round JSON: %w", err)
	}
	ev := &game.RoundEvent{Text: strings.TrimSpace(out.Text)}
	if ev.Text == "" {
		return nil, fmt.Errorf("llm: empty narration")
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

func (j Judge) Options(ctx context.Context, req game.JudgeRequest) (game.RoundOptions, error) {
	content, err := j.Chat.Complete(ctx, OptionsPrompt(req), true)
	if err != nil {
		return game.RoundOptions{}, err
	}
	var out game.RoundOptions
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return game.RoundOptions{}, fmt.Errorf("llm: bad options JSON: %w", err)
	}
	return out, nil
}

func (j Judge) Locations(ctx context.Context, n int, level string) ([]string, error) {
	content, err := j.Chat.Complete(ctx, LocationsPrompt(n, level), true)
	if err != nil {
		return nil, err
	}
	var out struct {
		Locs []string `json:"locs"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("llm: bad locations JSON: %w", err)
	}
	var locs []string
	for _, l := range out.Locs {
		if s := strings.TrimSpace(l); s != "" {
			locs = append(locs, s)
		}
	}
	if len(locs) == 0 {
		return nil, fmt.Errorf("llm: empty locations")
	}
	return locs, nil
}
