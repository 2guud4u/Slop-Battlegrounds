// Package api hosts the HTTP + WebSocket surface: room registry, provider
// wiring, and the client send pump. The game rules live in internal/game.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"slop-battlegrounds/internal/config"
	"slop-battlegrounds/internal/game"
)

// JoinMsg is the first message a client must send on the socket.
type JoinMsg struct {
	Type          string `json:"type"` // "join"
	Room          string `json:"room"` // empty = create a new room
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`        // avatar key from /api/avatars
	LLMProvider   string `json:"llmProvider"`   // host only
	LLMKey        string `json:"llmKey"`        // host only
	ImageProvider string `json:"imageProvider"` // host only: "auto"|"cloudflare"|"gemini"|"pollinations"|"mock"
	ImageAccount  string `json:"imageAccount"`  // host only (cloudflare)
	ImageToken    string `json:"imageToken"`    // host only (cloudflare)
	ImageKey      string `json:"imageKey"`      // host only (gemini)
}

// Hub tracks live rooms and serves the socket endpoint.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*game.Room
	def   config.Defaults
}

func NewHub(def config.Defaults) *Hub {
	return &Hub{rooms: map[string]*game.Room{}, def: def}
}

// Handler builds the mux: /ws for the game socket, /gen for locally rendered
// images, /api/avatars for the join screen, and static (built frontend) if given.
func (h *Hub) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.serveWS)
	mux.Handle("/gen/", http.StripPrefix("/gen/", http.FileServer(http.Dir(h.def.GenDir))))
	mux.HandleFunc("/api/avatars", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(game.Avatars)
	})
	if static != nil {
		mux.Handle("/", static)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "slop-battlegrounds server — run the Vite dev server for the UI")
		})
	}
	return mux
}

const wsReadLimit = 1 << 16

func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,                          // dev scaffold; origin check goes here for prod
		CompressionMode:    websocket.CompressionDisabled, // keep wire simple; plain JSON frames
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)
	ctx := r.Context()

	t0 := time.Now()
	var jm JoinMsg
	if err := wsjson.Read(ctx, conn, &jm); err != nil || jm.Type != "join" {
		_ = wsjson.Write(ctx, conn, game.ClientMsg{Type: "error", Message: "expected join {room?}"})
		conn.Close(websocket.StatusProtocolError, "bad join")
		return
	}
	room, err := h.room(jm)
	if err != nil {
		_ = wsjson.Write(ctx, conn, game.ClientMsg{Type: "error", Message: err.Error()})
		conn.Close(websocket.StatusProtocolError, err.Error())
		return
	}
	c := newWSClient(conn, ctx)
	go c.pump()
	playerID, err := room.Join(jm.Name, jm.Avatar, c)
	log.Printf("room %s: %s joined in %v", room.Code, jm.Name, time.Since(t0))
	if err != nil {
		_ = wsjson.Write(ctx, conn, game.ClientMsg{Type: "error", Message: err.Error()})
		conn.Close(websocket.StatusProtocolError, err.Error())
		return
	}
	defer func() {
		room.Leave(playerID)
		c.close()
		conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	for {
		var m game.ClientMsg
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			return // disconnect; defer cleans up
		}
		room.Handle(playerID, m)
	}
}

// room returns an existing room or creates one (first joiner = host, whose
// provider settings/keys apply to the room).
func (h *Hub) room(jm JoinMsg) (*game.Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if jm.Room != "" {
		if rm, ok := h.rooms[jm.Room]; ok {
			return rm, nil
		}
		return nil, fmt.Errorf("room %q not found", jm.Room)
	}
	code := newCode(h.rooms)
	log.Printf("creating room %s for host %q…", code, jm.Name)
	judge := resolveJudge(jm, h.def)
	imager := resolveImager(jm, h.def)
	rm := game.New(code, judge, imager)
	h.rooms[code] = rm
	log.Printf("room %s created (llm=%T image=%T)", code, judge, imager)
	return rm, nil
}
