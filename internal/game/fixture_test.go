package game

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var updateFixture = flag.Bool("update-fixture", false, "regenerate web/src/fixtures/state.json")

// fixturePath is the client-side copy of the wire contract: types.ts is kept
// honest against this JSON by tsc (state.fixture.ts casts it to State).
const fixturePath = "../../web/src/fixtures/state.json"

// TestWireFixture emits a populated draft-phase StateMsg and compares it to
// the golden JSON the TypeScript side type-checks against. Volatile fields
// (seat token, hand contents) are sanitized — the fixture proves the schema,
// not the data.
func TestWireFixture(t *testing.T) {
	r, ids := testRoom(t, 3)

	// Deterministic hands so every player can forge (canForge gates revealAt).
	hand := []Card{
		{ID: "c1", Text: "Fiery", Kind: KindAdj},
		{ID: "c2", Text: "Dragon", Kind: KindNoun},
		{ID: "c3", Text: "Suplexes", Kind: KindVerb},
	}
	for _, p := range r.players {
		p.Hand = append([]Card{}, hand...)
	}

	r.mu.Lock()
	r.startLocked(ids[0]) // → draft: fight picked, locOptions filling async
	r.mu.Unlock()

	// Wait for the (stub) judge's locations to land.
	deadline := time.Now().Add(2 * time.Second)
	for {
		r.mu.Lock()
		n := len(r.locOptions)
		r.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("locOptions never populated")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Spectator votes for a battleground — exercises locVotes/myVote.
	r.mu.Lock()
	r.locVoteLocked(ids[2], r.locOptions[0])
	msg := r.snapshotLocked(ids[2]) // spectator's view — includes Hand + Templates
	r.mu.Unlock()

	msg.Token = "" // volatile

	got, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	if *updateFixture {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixturePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v (run `go test ./internal/game -update-fixture`)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire schema drifted from %s — regenerate with `go test ./internal/game -update-fixture` and re-check web types", fixturePath)
	}
}
