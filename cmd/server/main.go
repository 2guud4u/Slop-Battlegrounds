// Slop Battlegrounds server: WS game hub + generated-file host + static build.
package main

import (
	"log"
	"net/http"
	"os"

	"slop-battlegrounds/internal/api"
	"slop-battlegrounds/internal/config"
)

func main() {
	def := config.Load()
	hub := api.NewHub(def)

	// Serve the built frontend in prod if it exists.
	var static http.Handler
	if fi, err := os.Stat("web/dist/index.html"); err == nil && !fi.IsDir() {
		static = http.FileServer(http.Dir("web/dist"))
	}

	log.Printf("slop-battlegrounds listening on %s (gen dir: %s)", def.Addr, def.GenDir)
	log.Fatal(http.ListenAndServe(def.Addr, hub.Handler(static)))
}
