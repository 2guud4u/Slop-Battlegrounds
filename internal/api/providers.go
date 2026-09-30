package api

import (
	"fmt"
	"math/rand/v2"
	"time"

	"slop-battlegrounds/internal/config"
	"slop-battlegrounds/internal/game"
	imagegen "slop-battlegrounds/internal/image"
	"slop-battlegrounds/internal/llm"
)

// resolveJudge picks the room's LLM judge: host-supplied provider+key first,
// then the server's env fallback, then mock.
func resolveJudge(jm JoinMsg, def config.Defaults) game.Judge {
	switch jm.LLMProvider {
	case "mock":
		return llm.Mock{}
	case "groq":
		if jm.LLMKey != "" {
			return llm.NewGroq(jm.LLMKey)
		}
		if def.GroqKey != "" {
			return llm.NewGroq(def.GroqKey)
		}
		return llm.Mock{}
	default: // auto
		key := jm.LLMKey
		if key == "" {
			key = def.GroqKey
		}
		if key != "" {
			return llm.NewGroq(key)
		}
		return llm.Mock{}
	}
}

// resolveImager picks the room's image generator the same way. "auto" is
// pollinations — free tier, needs POLLINATION_API_KEY (fine without one).
func resolveImager(jm JoinMsg, def config.Defaults) game.Imager {
	account, token, gkey := jm.ImageAccount, jm.ImageToken, jm.ImageKey
	if account == "" {
		account = def.CFAccountID
	}
	if token == "" {
		token = def.CFToken
	}
	if gkey == "" {
		gkey = def.GeminiKey
	}
	switch jm.ImageProvider {
	case "mock":
		return imagegen.Mock{}
	case "pollinations":
		return imagegen.Pollinations{Key: def.PollinationsKey}
	case "cloudflare":
		if account != "" && token != "" {
			return imagegen.NewCloudflare(account, token, def.GenDir, "/gen/")
		}
		return imagegen.Mock{}
	case "gemini":
		if gkey != "" {
			return imagegen.NewGemini(gkey, def.GenDir, "/gen/")
		}
		return imagegen.Mock{}
	default: // auto: pollinations — free tier, needs POLLINATION_API_KEY
		return imagegen.Pollinations{Key: def.PollinationsKey}
	}
}

// newCode mints a 4-letter room code not already in use.
func newCode(rooms map[string]*game.Room) string {
	const letters = "abcdefghjkmnpqrstuvwxyz"
	for i := 0; i < 100; i++ {
		b := make([]byte, 4)
		for i := range b {
			b[i] = letters[rand.IntN(len(letters))]
		}
		if _, taken := rooms[string(b)]; !taken {
			return string(b)
		}
	}
	return fmt.Sprintf("r%d", time.Now().UnixNano())
}
