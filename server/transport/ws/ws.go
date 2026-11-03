// Package ws carries hub connections over WebSocket, for LAN games. Dial works in every build, the browser
// included; Accept (accept.go) only where the platform can listen.
package ws

import (
	"context"
	"fmt"
	"time"

	"github.com/coder/websocket"
)

// readLimit covers the largest message either side sends: a Snapshot of a 40×40 board is about 3.2 KB.
const readLimit = 16 << 10

const writeTimeout = 10 * time.Second

// Conn is a hub connection over one WebSocket. Messages travel as binary frames.
type Conn struct{ c *websocket.Conn }

// New wraps an open WebSocket.
func New(c *websocket.Conn) *Conn {
	c.SetReadLimit(readLimit)
	return &Conn{c}
}

// Dial connects to a host's hub at url (ws://host:port/...).
func Dial(ctx context.Context, url string) (*Conn, error) {
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ws: dial: %w", err)
	}
	return New(c), nil
}

func (c *Conn) Send(msg []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return c.c.Write(ctx, websocket.MessageBinary, msg)
}

func (c *Conn) Receive() ([]byte, error) {
	typ, b, err := c.c.Read(context.Background())
	if err == nil && typ != websocket.MessageBinary {
		err = fmt.Errorf("ws: %v frame", typ)
		c.c.CloseNow()
	}
	return b, err
}

func (c *Conn) Close() error { return c.c.Close(websocket.StatusNormalClosure, "") }
