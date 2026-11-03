//go:build !js

package ws

import (
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

// Accept upgrades a request to a hub connection. A browser's Origin must match the host it connected to, which holds
// for the web build a LAN host serves itself.
func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("ws: accept: %w", err)
	}
	return New(c), nil
}
