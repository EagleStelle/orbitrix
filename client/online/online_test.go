//go:build !js

package online

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/signal"
)

// Approval starts a bounded connection attempt; waiting for the host's decision remains separate.
func TestJoinProgressAndTimeout(t *testing.T) {
	for _, approved := range []bool{false, true} {
		t.Run(fmt.Sprint(approved), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer c.CloseNow()
				ctx := context.Background()
				send := func(m signal.Msg) {
					b, _ := json.Marshal(m)
					c.Write(ctx, websocket.MessageText, b)
				}
				send(signal.Msg{T: "challenge", V: signal.Version, C: "test"})
				for range 3 { // hello, proof of work, join
					if _, _, err := c.Read(ctx); err != nil {
						return
					}
				}
				send(signal.Msg{T: "wait"})
				if approved {
					send(signal.Msg{T: "approved", ICE: json.RawMessage(`[]`)})
				}
				c.Read(ctx) // the host never sends an offer
			}))
			defer srv.Close()
			limit := 200 * time.Millisecond
			if approved {
				limit = connectWait + 5*time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), limit)
			defer cancel()
			var progress []bool
			_, err := Join(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), "ABC234", "Guest", proto.Token{}, false,
				func(waiting bool) { progress = append(progress, waiting) })
			if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrConnect) != approved {
				t.Fatalf("approval %v: %v", approved, err)
			}
			if approved && ctx.Err() != nil {
				t.Fatal("join waited for the caller's deadline instead of timing out its connection attempt")
			}
			want := []bool{true}
			if approved {
				want = append(want, false)
			}
			if !slices.Equal(progress, want) {
				t.Fatalf("progress %v, want %v", progress, want)
			}
		})
	}
}

// TestHostJoinRejoinClose walks an online room through its life: a joiner waits for the host's approval and takes a
// seat, drops, comes back with its token without being asked about, and hears the room close when the host leaves.
func TestHostJoinRejoinClose(t *testing.T) {
	srv := httptest.NewServer(signal.New(signal.Config{STUN: []string{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h, err := Open(ctx, url, "", false, false, hub.Config{Cols: 5, Rows: 5,
		Seats: []hub.Seat{{Human: true}, {Human: true}, {AI: ai.Mason}}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.Me.Send(proto.Hello{Name: "Host"})

	// Made public, the room shows in the list anyone can ask for.
	h.Update(true, 2, signal.Listed{Cols: 5, Rows: 5, Open: 1})
	rooms := ListRooms(url)
	defer rooms.Close()
	for l := rooms.Get(nil); len(l) != 1 || l[0].Code != h.Code(); l = rooms.Get(nil) {
		if ctx.Err() != nil {
			t.Fatal("the public room was never listed")
		}
		time.Sleep(10 * time.Millisecond)
	}

	joined := make(chan hub.Conn, 1)
	join := func(token proto.Token) {
		c, err := Join(ctx, url, h.Code(), "Bo", token, false, func(bool) {})
		if err != nil {
			t.Error(err)
		}
		joined <- c
	}
	go join(proto.Token{})
	for len(h.Requests(nil)) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	h.Approve(h.Requests(nil)[0].Peer)
	l := NewLink(<-joined)
	l.Send(proto.Hello{Name: "Bo"})
	w := wait[proto.Welcome](t, l)
	if w.Seat != 1 {
		t.Fatalf("joiner got seat %d", w.Seat)
	}

	l.Close() // the joiner's app is killed
	go join(w.Token)
	l = NewLink(<-joined)
	if r := h.Requests(nil); len(r) != 0 {
		t.Fatalf("a rejoin asked the host: %v", r)
	}
	l.Send(proto.Hello{Name: "Bo", Token: w.Token})
	if w2 := wait[proto.Welcome](t, l); w2.Seat != w.Seat {
		t.Fatalf("rejoined seat %d, want %d", w2.Seat, w.Seat)
	}

	h.Close()
	if b := wait[proto.Bye](t, l); b.Reason != proto.ReasonClosed {
		t.Fatalf("bye %d", b.Reason)
	}
}

// TestListSurvivesRedial: the server ends a listing socket after a while, as it ends every socket that never hosts or
// joins. The list stays up while Rooms dials again, instead of blinking empty.
func TestListSurvivesRedial(t *testing.T) {
	var dials atomic.Int32
	redialled := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := context.Background()
		if dials.Add(1) > 1 {
			close(redialled)
			c.Read(ctx) // until Rooms closes
			return
		}
		ch, _ := json.Marshal(signal.Msg{T: "challenge", V: signal.Version, C: "x"})
		c.Write(ctx, websocket.MessageText, ch)
		for range 3 { // hello, the proof of work, list
			c.Read(ctx)
		}
		rooms, _ := json.Marshal(signal.Msg{T: "rooms", Rooms: []signal.Listed{{Code: "ABC234", Cols: 5, Rows: 5, Open: 1}}})
		c.Write(ctx, websocket.MessageText, rooms) // then the socket ends
	}))
	defer srv.Close()
	rooms := ListRooms("ws" + strings.TrimPrefix(srv.URL, "http"))
	defer rooms.Close()
	select {
	case <-redialled:
	case <-time.After(10 * time.Second):
		t.Fatal("Rooms never dialled again")
	}
	if l := rooms.Get(nil); len(l) != 1 {
		t.Fatalf("listed %v while dialling again", l)
	}
}

// wait polls l until a message of type M arrives.
func wait[M proto.Msg](t *testing.T, l *Link) M {
	t.Helper()
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end); {
		m, ok := l.Poll()
		if mm, is := m.(M); ok && is {
			return mm
		}
		if !ok {
			if l.Closed() {
				t.Fatal("connection closed")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("timed out")
	panic("unreachable")
}
