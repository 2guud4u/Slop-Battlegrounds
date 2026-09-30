package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Groq is a Chat over the Groq chat-completions API (OpenAI-compatible).
type Groq struct {
	Key      string
	Model    string // e.g. "openai/gpt-oss-120b"
	HTTP     *http.Client
	Endpoint string
}

// NewGroq returns a Judge backed by Groq.
func NewGroq(key string) Judge {
	return Judge{Chat: &Groq{
		Key:      key,
		Model:    "openai/gpt-oss-120b",
		HTTP:     &http.Client{},
		Endpoint: "https://api.groq.com/openai/v1/chat/completions",
	}}
}

type groqMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqFormat struct {
	Type string `json:"type"`
}

type groqReq struct {
	Model               string      `json:"model"`
	ReasoningEffort     string      `json:"reasoning_effort,omitempty"` // gpt-oss: "low" keeps latency sane
	MaxCompletionTokens int         `json:"max_completion_tokens,omitempty"`
	Messages            []groqMsg   `json:"messages"`
	ResponseFormat      *groqFormat `json:"response_format,omitempty"`
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

// Complete posts a single user message; jsonMode toggles response_format.
func (g *Groq) Complete(ctx context.Context, prompt string, jsonMode bool) (string, error) {
	body := groqReq{
		Model:               g.Model,
		ReasoningEffort:     "low", // don't burn the round on chains of thought
		MaxCompletionTokens: 800,
		Messages:            []groqMsg{{Role: "user", Content: prompt}},
	}
	if jsonMode {
		body.ResponseFormat = &groqFormat{Type: "json_object"}
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
