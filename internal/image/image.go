package image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cloudflare Workers AI running FLUX.1-schnell. Writes generated PNGs into
// Dir and returns paths under URLPrefix (served by the HTTP server).
type Cloudflare struct {
	AccountID string
	Token     string
	Model     string // default "@cf/black-forest-labs/flux-1-schnell"
	Dir       string
	URLPrefix string // e.g. "/gen/"
	HTTP      *http.Client
}

func NewCloudflare(account, token, dir, prefix string) *Cloudflare {
	return &Cloudflare{
		AccountID: account,
		Token:     token,
		Model:     "@cf/black-forest-labs/flux-1-schnell",
		Dir:       dir,
		URLPrefix: prefix,
		HTTP:      &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Cloudflare) Generate(ctx context.Context, prompt string) (string, error) {
	ep := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s",
		c.AccountID, url.PathEscape(c.Model))
	body := fmt.Sprintf(`{"prompt":%q,"steps":4}`, prompt)
	req, err := http.NewRequestWithContext(ctx, "POST", ep, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("cloudflare %d: %s", resp.StatusCode, trunc(data, 300))
	}
	var out struct {
		Result struct {
			Image string `json:"image"` // base64
		} `json:"result"`
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if !out.Success {
		msg := "unknown error"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return "", fmt.Errorf("cloudflare: %s", msg)
	}
	png, err := base64.StdEncoding.DecodeString(out.Result.Image)
	if err != nil {
		return "", fmt.Errorf("cloudflare: bad image payload: %w", err)
	}
	name := fmt.Sprintf("%d-%d.png", time.Now().UnixNano(), rand.IntN(1<<20))
	if err := os.WriteFile(filepath.Join(c.Dir, name), png, 0o644); err != nil {
		return "", err
	}
	return c.URLPrefix + name, nil
}

// Gemini image generation via Google AI Studio ("nano banana",
// gemini-2.5-flash-image). Single API key; writes PNGs into Dir.
type Gemini struct {
	Key       string
	Model     string // default "gemini-2.5-flash-image"
	Dir       string
	URLPrefix string
	HTTP      *http.Client
}

func NewGemini(key, dir, prefix string) *Gemini {
	return &Gemini{
		Key:       key,
		Model:     "gemini-2.5-flash-image",
		Dir:       dir,
		URLPrefix: prefix,
		HTTP:      &http.Client{Timeout: 90 * time.Second},
	}
}

func (g *Gemini) Generate(ctx context.Context, prompt string) (string, error) {
	ep := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		url.PathEscape(g.Model))
	body := fmt.Sprintf(`{"contents":[{"parts":[{"text":%q}]}],"generationConfig":{"responseModalities":["IMAGE"]}}`, prompt)
	req, err := http.NewRequestWithContext(ctx, "POST", ep, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", g.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, trunc(data, 300))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("gemini: %s", out.Error.Message)
	}
	for _, c := range out.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData.Data == "" {
				continue
			}
			png, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return "", fmt.Errorf("gemini: bad image payload: %w", err)
			}
			name := fmt.Sprintf("%d-%d.png", time.Now().UnixNano(), rand.IntN(1<<20))
			if err := os.WriteFile(filepath.Join(g.Dir, name), png, 0o644); err != nil {
				return "", err
			}
			return g.URLPrefix + name, nil
		}
	}
	return "", fmt.Errorf("gemini: no image in response")
}

// Pollinations: zero-key URL API. Kept as an opt-in/fallback provider.
type Pollinations struct{}

func (Pollinations) Generate(_ context.Context, prompt string) (string, error) {
	return fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=768&height=512&nologo=true&seed=%d",
		url.PathEscape(prompt), rand.IntN(1<<30)), nil
}

// Mock: picsum placeholders so the loop works with zero keys.
type Mock struct{}

func (Mock) Generate(_ context.Context, prompt string) (string, error) {
	return fmt.Sprintf("https://picsum.photos/seed/%d/768/512", rand.IntN(1<<30)), nil
}

func trunc(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
