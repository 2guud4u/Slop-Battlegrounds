package api

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"slop-battlegrounds/internal/game"
)

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
