package signal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/transport/loopback"
	"orbitrix/server/transport/rtc"
)

// TestOnlineGame plays a whole game the way an online room runs: the host creates a room through signaling and runs
// the hub, a joiner is approved and connects over a pion data channel, and both play against an AI seat until
// someone wins, each checking the host's state hash after every move.
func TestOnlineGame(t *testing.T) {
	s := New(Config{Password: testPassword, STUN: []string{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	srv := httptest.NewServer(s)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hc, err := Dial(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer hc.Close()
	if err := hc.Send(ctx, Msg{T: "host.create", Password: testPassword, Humans: 2}); err != nil {
		t.Fatal(err)
	}
	rm, err := hc.Receive(ctx)
	if err != nil || rm.T != "room" {
		t.Fatalf("room: %+v %v", rm, err)
	}
	room, err := hub.New(hub.Config{Code: rm.Code, Cols: 6, Rows: 5,
		Seats: []hub.Seat{{Human: true}, {Human: true}, {AI: ai.Mason}}})
	if err != nil {
		t.Fatal(err)
	}
	defer room.Close()
	go hostSignaling(ctx, t, hc, room, rm.ICE)

	results := make(chan error, 2)
	mine, theirs := loopback.Pipe()
	room.Add(theirs)
	go func() { results <- play(mine, rm.Code, "Host") }()
	go func() {
		c, err := joinOnline(ctx, url, rm.Code)
		if err != nil {
			results <- err
			return
		}
		results <- play(c, rm.Code, "Joiner")
	}()
	go func() { // the host presses Start once both players are ready
		for ctx.Err() == nil && room.Start() != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}()
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

// hostSignaling approves every joiner and offers each a data channel; the open channel joins the room.
func hostSignaling(ctx context.Context, t *testing.T, hc *Client, room *hub.Room, ice []byte) {
	peers := map[string]*rtc.Peer{}
	for {
		m, err := hc.Receive(ctx)
		if err != nil {
			return
		}
		switch m.T {
		case "request":
			_ = hc.Send(ctx, Msg{T: "approve", Peer: m.Peer})
		case "approved":
			p, offer, err := rtc.Offer(ctx, ice)
			if err != nil {
				t.Error(err)
				return
			}
			peers[m.Peer] = p
			_ = hc.Send(ctx, Msg{T: "signal", Peer: m.Peer, SDP: offer})
		case "signal":
			p := peers[m.Peer]
			if p == nil || p.Accept(m.SDP) != nil {
				t.Errorf("answer from %q", m.Peer)
				return
			}
			go func(id string) {
				c, err := p.Conn(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				c.Peer = id
				room.Add(c)
			}(m.Peer)
		}
	}
}

// joinOnline goes through signaling to an open data channel to the host, then leaves signaling.
func joinOnline(ctx context.Context, url, code string) (hub.Conn, error) {
	jc, err := Dial(ctx, url)
	if err != nil {
		return nil, err
	}
	defer jc.Close()
	if err := jc.Send(ctx, Msg{T: "join", Code: code, Name: "Bo"}); err != nil {
		return nil, err
	}
	var ice []byte
	for {
		m, err := jc.Receive(ctx)
		switch {
		case err != nil:
			return nil, err
		case m.T == "approved":
			ice = m.ICE
		case m.T == "signal":
			p, answer, err := rtc.Answer(ctx, ice, m.SDP)
			if err != nil {
				return nil, err
			}
			if err := jc.Send(ctx, Msg{T: "signal", SDP: answer}); err != nil {
				return nil, err
			}
			return p.Conn(ctx)
		case m.T != "wait":
			return nil, fmt.Errorf("joiner got %+v", m)
		}
	}
}

// play is a scripted player: ready at once, then its first legal move every turn until the game ends.
func play(c hub.Conn, code, name string) error {
	send := func(m proto.Msg) error { return c.Send(proto.Append(nil, m)) }
	if err := send(proto.Hello{Name: name, Code: code}); err != nil {
		return err
	}
	seat, moves := -1, 0
	var g *core.Game
	for {
		b, err := c.Receive()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		m, err := proto.Decode(b)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		switch m := m.(type) {
		case proto.Welcome:
			seat = int(m.Seat)
			if err := send(proto.Ready{On: true}); err != nil {
				return err
			}
		case proto.Snapshot:
			if g, err = core.Restore(m.Game); err != nil {
				return err
			}
		case proto.MoveApplied:
			if err := g.Simulate(g.Geo.Index(int(m.C), int(m.R))); err != nil || g.Hash() != m.Hash {
				return fmt.Errorf("%s: move %d: %v, hash %x, host %x", name, m.Seq, err, g.Hash(), m.Hash)
			}
			moves++
		case proto.Bye:
			return fmt.Errorf("%s: bye %d", name, m.Reason)
		default:
			continue
		}
		if g == nil {
			continue
		}
		if g.Over() {
			if moves < 3 {
				return errors.New("game over too soon")
			}
			return nil
		}
		if g.Turn == seat {
			c, r := g.Geo.Coords(g.LegalMoves(nil)[0])
			if err := send(proto.Move{C: uint8(c), R: uint8(r)}); err != nil {
				return err
			}
		}
	}
}
