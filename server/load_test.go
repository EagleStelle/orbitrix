package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"orbitrix/server/signal"
	"orbitrix/server/signal/pow"
)

// TestFloodFromOneIP floods /signal with wrong passwords from one IP (every test socket comes from 127.0.0.1) at the
// real one-second failure delay: 50 bots, each guessing on its socket until the server closes it after three
// failures, then reconnecting at once. The correct password from that same IP must still create a room, after
// solving the raised proof of work, and /play must keep answering quickly.
//
// The bots stay under the per-IP socket cap (100). A flood that keeps the cap full sheds every new socket from that
// IP, the honest one too; that is the cap's job, and the edge rule in front of it works the same way.
func TestFloodFromOneIP(t *testing.T) {
	if testing.Short() {
		t.Skip("floods for a few seconds")
	}
	const password = "correct horse battery staple 42"
	sig := signal.New(signal.Config{Password: password, STUN: []string{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	h, err := Site{Web: build, Signal: sig}.Handler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/signal"

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var mu sync.Mutex
	attempts := 0
	for w := 0; w < 50; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				c, err := signal.Dial(ctx, url)
				if err != nil {
					continue
				}
				for ctx.Err() == nil {
					if c.Send(ctx, signal.Msg{T: "host.create", Password: "wrong guess", Humans: 2}) != nil {
						break
					}
					mu.Lock()
					attempts++
					mu.Unlock()
					if m, err := c.Receive(ctx); err != nil || m.T != "error" {
						break
					}
				}
				c.Close()
			}
		}()
	}
	defer func() { cancel(); wg.Wait() }()
	time.Sleep(3500 * time.Millisecond) // past the first round of closes, so the bots are reconnecting

	start := time.Now()
	resp, err := http.Get(srv.URL + "/play")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("/play under flood: %v %v", resp, err)
	}
	resp.Body.Close()
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("/play took %v under flood", d)
	}

	start = time.Now()
	rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer rcancel()
	c, _, err := websocket.Dial(rctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	var ch, room signal.Msg
	read := func(m *signal.Msg) {
		_, b, err := c.Read(rctx)
		if err != nil || json.Unmarshal(b, m) != nil {
			t.Fatalf("read: %s %v", b, err)
		}
	}
	write := func(m signal.Msg) {
		b, _ := json.Marshal(m)
		if err := c.Write(rctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	read(&ch)
	if ch.Bits == 0 {
		t.Fatal("the flood didn't raise the proof of work")
	}
	write(signal.Msg{T: "hello", V: signal.Version})
	write(signal.Msg{T: "pow", C: ch.C, N: pow.Solve(ch.C, ch.Bits)})
	write(signal.Msg{T: "host.create", Password: password, Humans: 2})
	read(&room)
	if room.T != "room" || room.Code == "" {
		t.Fatalf("correct password under flood: %+v", room)
	}
	mu.Lock()
	t.Logf("%d wrong attempts flooded in; the host's challenge had %d bits; room in %v", attempts, ch.Bits,
		time.Since(start))
	mu.Unlock()
}
