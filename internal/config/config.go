// Package config loads process-level settings: .env into the environment,
// then typed env vars into Defaults.
package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Defaults holds server-level provider config (from env). Per-room host keys
// override these.
type Defaults struct {
	GroqKey         string
	GeminiKey       string
	CFAccountID     string
	CFToken         string
	PollinationsKey string
	GenDir          string // where generated images are written
	Addr            string
}

// Load reads .env (real env vars win) into the process environment and
// returns server defaults.
func Load() Defaults {
	loadDotEnv(".env")

	genDir := os.Getenv("GEN_DIR")
	if genDir == "" {
		genDir = filepath.Join(os.TempDir(), "slop-gen")
	}
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		log.Fatal(err)
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Defaults{
		GroqKey:         os.Getenv("GROQ_API_KEY"),
		GeminiKey:       os.Getenv("GEMINI_API_KEY"),
		CFAccountID:     os.Getenv("CF_ACCOUNT_ID"),
		CFToken:         os.Getenv("CF_API_TOKEN"),
		PollinationsKey: os.Getenv("POLLINATION_API_KEY"),
		GenDir:          genDir,
		Addr:            addr,
	}
}

// loadDotEnv reads KEY=value lines from a .env file into the environment.
// Real env vars take precedence; missing file is fine.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}
