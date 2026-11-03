package ws

import (
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

// Accept upgrades a request to a hub connection. Native clients send no Origin; a browser's would have to match the
// host it connected to. In the browser build it always fails: a page can't listen.
func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("ws: accept: %w", err)
	}
	return New(c), nil
}
