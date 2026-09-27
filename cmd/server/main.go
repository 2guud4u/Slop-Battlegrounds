package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"slop-battlegrounds/internal/server"
)

func main() {
	genDir := os.Getenv("GEN_DIR")
	if genDir == "" {
		genDir = filepath.Join(os.TempDir(), "slop-gen")
	}
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		log.Fatal(err)
	}

	hub := server.NewHub(server.Defaults{
		GroqKey:     os.Getenv("GROQ_API_KEY"),
		GeminiKey:   os.Getenv("GEMINI_API_KEY"),
		CFAccountID: os.Getenv("CF_ACCOUNT_ID"),
		CFToken:     os.Getenv("CF_API_TOKEN"),
		GenDir:      genDir,
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
