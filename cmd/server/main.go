package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"slop-battlegrounds/internal/server"
)

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

func main() {
	loadDotEnv(".env")

	genDir := os.Getenv("GEN_DIR")
	if genDir == "" {
		genDir = filepath.Join(os.TempDir(), "slop-gen")
	}
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		log.Fatal(err)
	}

	hub := server.NewHub(server.Defaults{
		GroqKey:         os.Getenv("GROQ_API_KEY"),
		GeminiKey:       os.Getenv("GEMINI_API_KEY"),
		CFAccountID:     os.Getenv("CF_ACCOUNT_ID"),
		CFToken:         os.Getenv("CF_API_TOKEN"),
		PollinationsKey: os.Getenv("POLLINATION_API_KEY"),
		GenDir:          genDir,
	})

	// Serve the built frontend in prod if it exists.
	var static http.Handler
	if fi, err := os.Stat("web/dist/index.html"); err == nil && !fi.IsDir() {
		static = http.FileServer(http.Dir("web/dist"))
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("slop-battlegrounds listening on %s (gen dir: %s)", addr, genDir)
	log.Fatal(http.ListenAndServe(addr, hub.Handler(static)))
}
