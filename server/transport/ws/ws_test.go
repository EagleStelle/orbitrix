//go:build !js

package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/transport/ws"
)

// TestHubOverWebSocket joins a room over a real WebSocket and checks both directions and the closing Bye.
func TestHubOverWebSocket(t *testing.T) {
	room, err := hub.New(hub.Config{Cols: 3, Rows: 3, Seats: []hub.Seat{{Human: true}, {}}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r)
		if err != nil {
			t.Error(err)
			return
		}
		room.Add(c)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := ws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Send(proto.Append(nil, proto.Hello{Name: "Ana"})); err != nil {
		t.Fatal(err)
	}
	want := []proto.Type{proto.TypeWelcome, proto.TypeLobby, proto.TypeBye}
	for i, typ := range want {
		if i == 2 {
			room.Close()
		}
		b, err := c.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if m, err := proto.Decode(b); err != nil || m.Type() != typ {
			t.Fatalf("message %d: %v %v, want type %d", i, m, err, typ)
		}
	}
	if _, err := c.Receive(); err == nil {
		t.Fatal("still open after Bye")
	}
}
