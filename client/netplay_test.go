//go:build !js

package client

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/proto"
	"orbitrix/server/signal"
)

// TestMain keeps the tests' settings out of the player's own settings file.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "orbitrix-client-test")
	if err != nil {
		panic(err)
	}
	platform.SetDataDir(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestOnlineRoom drives the real scenes of a host and a joiner through an online room: the join request, Ready and
// Start in the lobby, a whole game played move by move through the host (with an AI seat), Play again back to the
// lobby, and the host leaving, which the joiner hears about.
func TestOnlineRoom(t *testing.T) {
	srv := httptest.NewServer(signal.New(signal.Config{STUN: []string{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ha, ja := setupTestApp(1040, 640, 1, 3), setupTestApp(390, 844, 1, 3)
	ha.set.Cols, ha.set.Rows, ha.set.seats = 5, 4, []int{seatHuman, seatHuman, 1 + 1}
	h, err := online.Open(ctx, url, "", false, hubConfig(ha))
	if err != nil {
		t.Fatal(err)
	}
	host := hostSession(ha, h)
	var hs, js scene = newLobby(host), nil

	joined := make(chan joinResult, 1)
	go func() {
		c, err := online.Join(ctx, url, h.Code(), onlineName, proto.Token{}, false, func() {})
		joined <- joinResult{c: c, err: err}
	}()
	step := func(a *app, s *scene) {
		t.Helper()
		if *s == nil {
			return
		}
		a.in = input{x: offScreen, y: offScreen}
		if err := (*s).update(a); err != nil {
			t.Fatal(err)
		}
		if a.next != nil {
			*s, a.next = a.next, nil
		}
	}
	var joiner *session
	for i := 0; ; i++ {
		if i > 100000 || ctx.Err() != nil {
			t.Fatal("the room never started")
		}
		step(ha, &hs)
		step(ja, &js)
		if r := h.Requests(nil); len(r) > 0 {
			h.Approve(r[0].Peer)
		}
		select {
		case r := <-joined:
			if r.err != nil {
				t.Fatal(r.err)
			}
			joiner = newSession(online.NewLink(r.c))
			joiner.code = h.Code()
			joiner.link.Send(proto.Hello{Name: onlineName})
			js = newLobby(joiner)
		default:
		}
		if l, ok := js.(*lobbyScene); ok && joiner.seat >= 0 && joiner.got && !joiner.lobby.Seats[joiner.seat].Ready {
			l.act() // Ready
		}
		if l, ok := hs.(*lobbyScene); ok && l.canAct() && joiner != nil && joiner.got {
			l.act() // Start
		}
		_, hp := hs.(*playScene)
		_, jp := js.(*playScene)
		if hp && jp {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if ja.set.Rejoin == nil || ja.set.Rejoin.Code != h.Code() {
		t.Fatal("the joiner must remember its seat to rejoin")
	}

	for i := 0; ; i++ {
		if i > 200000 || ctx.Err() != nil {
			t.Fatal("the game never ended")
		}
		for _, s := range []struct {
			a *app
			s *scene
		}{{ha, &hs}, {ja, &js}} {
			p := (*s.s).(*playScene)
			p.b.fastForward()
			if !p.b.playing && !p.over && p.mine() {
				p.play(s.a, p.game.LegalMoves(nil)[0])
			}
			step(s.a, s.s)
		}
		hp, jp := hs.(*playScene), js.(*playScene)
		if hp.resync || jp.resync {
			t.Fatal("the boards went apart")
		}
		if hp.over && jp.over {
			if hp.game.Hash() != jp.game.Hash() || hp.seq != jp.seq {
				t.Fatal("the game ended differently for the host and the joiner")
			}
			break
		}
		time.Sleep(50 * time.Microsecond)
	}

	hs.(*playScene).restartGame(ha) // Play again
	for i := 0; ; i++ {
		if i > 100000 {
			t.Fatal("Play again must take both players back to the lobby")
		}
		step(ha, &hs)
		step(ja, &js)
		_, hl := hs.(*lobbyScene)
		_, jl := js.(*lobbyScene)
		if hl && jl && host.lobby.Seats[host.seat].Ready { // the host readies itself again
			break
		}
		time.Sleep(time.Millisecond)
	}

	host.leave(ha)
	for i := 0; ; i++ {
		if i > 100000 {
			t.Fatal("the joiner must hear the host leave")
		}
		step(ja, &js)
		if g, ok := js.(*goneScene); ok {
			if g.why != byeText[proto.ReasonClosed] || ja.set.Rejoin != nil {
				t.Fatalf("joiner ended with %q, rejoin %v", g.why, ja.set.Rejoin)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
}
