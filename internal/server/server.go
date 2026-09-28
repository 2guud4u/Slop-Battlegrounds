package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"slop-battlegrounds/internal/game"
	imagegen "slop-battlegrounds/internal/image"
	"slop-battlegrounds/internal/llm"
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
}

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

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*game.Room
	def   Defaults
}

func NewHub(def Defaults) *Hub {
	return &Hub{rooms: map[string]*game.Room{}, def: def}
}

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

func resolveJudge(jm JoinMsg, def Defaults) game.Judge {
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

func resolveImager(jm JoinMsg, def Defaults) game.Imager {
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

// wsClient adapts a websocket conn to game.Client. Sends go through a
// buffered queue + pump goroutine so a stalled client can never hold the
// room's mutex mid-broadcast. wsjson.Write is not safe for concurrent use —
// only the pump writes.
type queuedMsg struct {
	v         any
	droppable bool // state snapshots can be skipped; errors cannot
}

type wsClient struct {
	conn *websocket.Conn
	ctx  context.Context
	ch   chan queuedMsg
	done chan struct{}
}

const sendQueue = 32

func newWSClient(conn *websocket.Conn, ctx context.Context) *wsClient {
	return &wsClient{conn: conn, ctx: ctx, ch: make(chan queuedMsg, sendQueue), done: make(chan struct{})}
}

// SendJSON never blocks: it enqueues, or drops a stale state if the client
// is behind. Errors always attempt delivery first.
func (c *wsClient) SendJSON(v any) error {
	q := queuedMsg{v: v, droppable: true}
	if cm, ok := v.(game.ClientMsg); ok && cm.Type == "error" {
		q.droppable = false // errors must reach the client
	}
	select {
	case c.ch <- q:
		return nil
	default:
	}
	// Queue full: drop the oldest droppable snapshot and retry once.
	var keep []queuedMsg
	for {
		select {
		case old := <-c.ch:
			if !old.droppable {
				keep = append(keep, old)
				continue
			}
			goto evicted
		default:
			goto full
		}
	}
evicted:
	for _, m := range keep {
		c.ch <- m
	}
	c.ch <- q
	return nil
full:
	for _, m := range keep {
		c.ch <- m
	}
	c.close() // client can't keep up — cut it loose
	return nil
}

// pump writes queued messages until the client closes.
func (c *wsClient) pump() {
	for {
		select {
		case <-c.done:
			return
		case q := <-c.ch:
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := wsjson.Write(ctx, c.conn, q.v)
			cancel()
			if err != nil {
				log.Printf("send to client failed: %v", err)
				c.close()
				return
			}
		}
	}
}

func (c *wsClient) close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}
