//go:build !js

package client

import (
	"context"
	"errors"
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
	"orbitrix/server/lan"
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

// testRoom is a hosted room and a joiner on their way in, for playRoom.
type testRoom struct {
	host    *session
	code    string
	joined  func() *session // the joiner's session once the host has let them in, else nil; on the game loop
	approve func()          // lets a waiting joiner in, where the host must
}

// TestOnlineRoom plays a room through signaling and WebRTC.
func TestOnlineRoom(t *testing.T) {
	srv := httptest.NewServer(signal.New(signal.Config{STUN: []string{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ha, ja := testRoomApps()
	h, err := online.Open(ctx, url, "", false, false, hubConfig(ha))
	if err != nil {
		t.Fatal(err)
	}
	r := testRoom{host: hostSession(h.Room, h.Me, "Human"), code: h.Code()}
	r.host.host = h
	joined := make(chan joinResult, 1)
	go func() {
		c, err := online.Join(ctx, url, h.Code(), "Human", proto.Token{}, false, func() {})
		joined <- joinResult{c: c, err: err}
	}()
	r.joined = func() *session {
		select {
		case jr := <-joined:
			if jr.err != nil {
				t.Fatal(jr.err)
			}
			n := newSession(online.NewLink(jr.c))
			n.code = h.Code()
			n.link.Send(proto.Hello{Name: "Human"})
			return n
		default:
			return nil
		}
	}
	r.approve = func() {
		if q := h.Requests(nil); len(q) > 0 {
			h.Approve(q[0].Peer)
		}
	}
	playRoom(t, ctx, ha, ja, r, false)
}

// TestLANRoom plays a room on this device's LAN address, with a dropped connection the joiner wins back on its own.
func TestLANRoom(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ha, ja := testRoomApps()
	l, err := lan.Open(hubConfig(ha), "test")
	if errors.Is(err, lan.ErrNoNetwork) {
		t.Skip(err)
	} else if err != nil {
		t.Fatal(err)
	}
	r := testRoom{host: hostSession(l.Room, online.NewLink(l.Me), "Human"), approve: func() {}}
	r.host.lan, r.host.addr = l, l.Addr
	joined := make(chan lanResult, 1)
	go joinLAN(ctx, []string{l.Addr}, "Human", "", proto.Token{}, joined)
	r.joined = func() *session {
		select {
		case jr := <-joined:
			if jr.err != nil {
				t.Fatal(jr.err)
			}
			return lanSession(ja, jr.c, jr.w, jr.addr, jr.code)
		default:
			return nil
		}
	}
	playRoom(t, ctx, ha, ja, r, true)
	if ja.set.Rejoin != nil {
		t.Fatal("a closed room must forget its rejoin")
	}
}

func testRoomApps() (host, joiner *app) {
	host, joiner = setupTestApp(1040, 640, 1, 3), setupTestApp(390, 844, 1, 3)
	host.set.Cols, host.set.Rows, host.set.seats = 5, 4, []int{seatHuman, seatHuman, 1 + 1}
	return host, joiner
}

// playRoom drives the real scenes of a host and a joiner through a room: Ready and Start in the lobby, a whole game
// played move by move through the host (with an AI seat), Play again back to the lobby, a dropped connection where
// drop says (which a LAN joiner wins back), and the host leaving, which the joiner hears about.
func playRoom(t *testing.T, ctx context.Context, ha, ja *app, r testRoom, drop bool) {
	t.Helper()
	host := r.host
	var hs, js scene = newRoom(host), nil
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
		r.approve()
		if joiner == nil {
			if joiner = r.joined(); joiner != nil {
				js = newRoom(joiner)
			}
		}
		if _, ok := js.(*roomScene); ok && joiner.seat >= 0 && joiner.got && !joiner.lobby.Seats[joiner.seat].Ready {
			joiner.link.Send(proto.Ready{On: true})
		}
		if l, ok := hs.(*setupScene); ok && l.room.canAct(ha) && joiner != nil && joiner.got {
			l.room.act(ha) // Start
		}
		_, hp := hs.(*playScene)
		_, jp := js.(*playScene)
		if hp && jp {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if ja.set.Rejoin == nil || ja.set.Rejoin.Code != r.code || ja.set.Rejoin.Addr != host.addr {
		t.Fatalf("the joiner must remember its seat to rejoin: %+v", ja.set.Rejoin)
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
			p.paused = !p.over // an online game goes on behind the pause menu
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
		_, hl := hs.(*setupScene)
		_, jl := js.(*roomScene)
		if hl && jl && host.lobby.Seats[host.seat].Ready { // the host readies itself again
			break
		}
		time.Sleep(time.Millisecond)
	}

	if drop {
		seat := ja.net.seat
		ja.net.link.Close() // as if the host had gone quiet
		var disconnected bool
		for i := 0; ; i++ {
			if i > 100000 || ctx.Err() != nil {
				t.Fatal("the joiner must get its seat back")
			}
			step(ha, &hs)
			step(ja, &js)
			if g, ok := js.(*goneScene); ok && g.why == "Host disconnected" {
				disconnected = true
			}
			if _, ok := js.(*roomScene); ok && disconnected && ja.net.seat >= 0 {
				if ja.net.seat != seat {
					t.Fatalf("rejoined seat %d, held %d", ja.net.seat, seat)
				}
				break
			}
			time.Sleep(time.Millisecond)
		}
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

// TestRoomSetup opens a LAN room from Setup and changes it while it waits, through clicks on Setup's own controls and
// the Room dialog: the players' plus opens a seat in the hub, Private gives the room a code that joining then needs,
// and handing a held seat to a bot from its menu lets its player go.
func TestRoomSetup(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	a.set.Cols, a.set.Rows, a.set.seats = 5, 5, []int{seatHuman, seatHuman}
	s := newRoomSetup(a, true)
	s.room.private = false
	click := func(at rect) {
		t.Helper()
		a.in = input{x: at.cx(), y: at.cy(), clicked: true}
		if err := s.update(a); err != nil {
			t.Fatal(err)
		}
		a.in = input{x: offScreen, y: offScreen}
	}
	wait := func(what string, ok func() bool) {
		t.Helper()
		for start := time.Now(); !ok(); time.Sleep(time.Millisecond) {
			if s.room.note == "No local network" {
				t.Skip(s.room.note)
			}
			if time.Since(start) > 10*time.Second {
				t.Fatal(what)
			}
			if err := s.update(a); err != nil {
				t.Fatal(err)
			}
		}
	}
	s.layout(a)
	click(s.start) // Open room
	n := func() *session { return s.room.net }
	wait("the room never opened", func() bool { return n() != nil && n().got && n().seat == 0 })
	if label := s.room.actionLabel(a); label != "Waiting for players" && label != "Start with bots" {
		t.Fatalf("an open room's action reads %q", label)
	}

	s.layout(a)
	click(s.plusP)
	wait("the hub never grew a seat", func() bool { return len(n().lobby.Seats) == 3 })

	s.layout(a)
	click(s.invite)
	if !s.room.open {
		t.Fatal("the invite button must open the Room dialog")
	}
	s.room.layoutDialog(a)
	click(s.room.opts[1]) // Private, a draft
	if len(s.room.code()) != proto.CodeLen {
		t.Fatal("choosing Private must prepare its room code before Confirm")
	}
	click(rect{1, 1, 1, 1})
	if s.room.open || s.room.private {
		t.Fatal("a tap outside closes the dialog and drops its draft")
	}
	s.layout(a)
	click(s.invite)
	s.room.layoutDialog(a)
	click(s.room.opts[1])
	s.room.layoutDialog(a)
	click(s.room.confirmB)
	if s.room.open || s.room.code() == "" {
		t.Fatal("Confirm makes the room private, with a code")
	}
	code := s.room.code()
	s = newRoom(n()).(*setupScene) // returning from a game rebuilds the host's room scene
	for _, access := range []int{0, 1} {
		s.layout(a)
		click(s.invite)
		s.room.layoutDialog(a)
		click(s.room.opts[access])
		s.room.layoutDialog(a)
		if access == 1 && s.room.code() != code {
			t.Fatalf("choosing Private after a game displays %q, want the saved code %q", s.room.code(), code)
		}
		click(s.room.confirmB)
	}
	if s.room.shareText() != code || n().code != code {
		t.Fatal("the displayed, shared and accepted LAN codes must agree after switching Public then Private")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var refused lan.Refused
	if _, _, err := lan.Join(ctx, n().addr, proto.Hello{}); !errors.As(err, &refused) ||
		refused.Reason != proto.ReasonBadCode {
		t.Fatalf("joining a private room without its code: %v", err)
	}
	c, w, err := lan.Join(ctx, n().addr, proto.Hello{Code: s.room.code()})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	wait("the joiner never showed", func() bool { return s.room.held(int(w.Seat)) })
	s.layout(a)
	click(s.seatHit[w.Seat])
	if s.menu != int(w.Seat) || s.menuCount(a) != len(seatLabels)+2 {
		t.Fatal("a seat's menu must offer Swap position and Remove player")
	}
	panel, first := s.menuRects(a)
	click(setupMenuOption(panel, first, 1, a.u)) // Jester
	wait("a seat handed to a bot must let its player go", func() bool {
		return n().lobby.Seats[w.Seat].State == proto.SeatAI
	})

	// Removing a bot's seat before another player's moves that player up.
	s.layout(a)
	click(s.seatHit[2])
	panel, first = s.menuRects(a)
	click(setupMenuOption(panel, first, seatHuman, a.u))
	wait("seat 2 never opened for a player", func() bool { return n().lobby.Seats[2].State == proto.SeatOpen })
	c2, w2, err := lan.Join(ctx, n().addr, proto.Hello{Code: s.room.code()})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	wait("the second joiner never showed", func() bool { return s.room.held(int(w2.Seat)) })
	s.layout(a)
	click(s.seatHit[w.Seat])
	panel, first = s.menuRects(a)
	click(setupMenuOption(panel, first, len(seatLabels)+1, a.u)) // Remove player
	wait("the seat never went", func() bool { return len(n().lobby.Seats) == 2 && a.set.Players == 2 })
	if !s.room.held(int(w2.Seat) - 1) {
		t.Fatal("the player behind a removed seat must move up with it")
	}

	// That player can't trade with the host, only with a bot, which the player then swaps places with.
	s.layout(a)
	click(s.seatHit[1])
	if s.swaps(a) != 0 || s.menuCount(a) != len(seatLabels)+2 {
		t.Fatal("two players must find Swap position listed with nobody to trade with")
	}
	panel, first = s.menuRects(a)
	click(setupMenuOption(panel, first, len(seatLabels), a.u))
	if s.swap {
		t.Fatal("two players must never swap")
	}
	click(rect{1, 1, 1, 1})
	s.layout(a)
	click(setupStepHit(s.plusP, a.u))
	wait("the room never got its bot", func() bool { return len(n().lobby.Seats) == 3 })
	s.layout(a)
	click(s.seatHit[1])
	panel, first = s.menuRects(a)
	click(setupMenuOption(panel, first, len(seatLabels), a.u)) // Swap position
	if s.swaps(a) != 1 || s.swapTarget(a, 0) != 2 {
		t.Fatal("a player's seat must list only the bot")
	}
	panel, first = s.menuRects(a)
	click(setupMenuOption(panel, first, 0, a.u))
	wait("the swap never reached the room", func() bool {
		return s.room.held(2) && n().lobby.Seats[1].State == proto.SeatAI
	})
	s.leave(a)
}
