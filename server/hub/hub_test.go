package hub_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/transport/loopback"
)

// player is a test client on a loopback connection.
type player struct {
	t     *testing.T
	conn  *loopback.Conn
	msgs  chan proto.Msg
	seat  int
	token proto.Token
	game  *core.Game
}

func connect(t *testing.T, room *hub.Room) *player {
	t.Helper()
	a, b := loopback.Pipe()
	room.Add(b)
	p := &player{t: t, conn: a, msgs: make(chan proto.Msg, 256), seat: -1}
	go func() {
		defer close(p.msgs)
		for {
			raw, err := a.Receive()
			if err != nil {
				return
			}
			m, err := proto.Decode(raw)
			if err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			p.msgs <- m
		}
	}()
	return p
}

// join connects and says Hello, returning once the player has a seat.
func join(t *testing.T, room *hub.Room, name string, token proto.Token) *player {
	t.Helper()
	p := connect(t, room)
	p.send(proto.Hello{Name: name, Code: "ABC234", Token: token})
	w := waitFor[proto.Welcome](p)
	p.seat, p.token = int(w.Seat), w.Token
	return p
}

func (p *player) send(m proto.Msg) {
	if err := p.conn.Send(proto.Append(nil, m)); err != nil {
		p.t.Errorf("send %T: %v", m, err)
	}
}

func (p *player) next() proto.Msg {
	select {
	case m, ok := <-p.msgs:
		if !ok {
			return nil
		}
		return m
	case <-time.After(5 * time.Second):
		p.t.Errorf("seat %d: no message", p.seat)
		return nil
	}
}

// waitFor skips messages until one of type T arrives.
func waitFor[T proto.Msg](p *player) T {
	p.t.Helper()
	for {
		m := p.next()
		if m == nil {
			var zero T
			p.t.Fatalf("seat %d: closed while waiting for %T", p.seat, zero)
		}
		if v, ok := m.(T); ok {
			return v
		}
	}
}

// lobbyWhere waits for a Lobby that satisfies ok.
func (p *player) lobbyWhere(ok func(proto.Lobby) bool) proto.Lobby {
	p.t.Helper()
	for {
		if l := waitFor[proto.Lobby](p); ok(l) {
			return l
		}
	}
}

// track applies a game message to the player's copy and checks the host's hash.
func (p *player) track(m proto.Msg) {
	switch m := m.(type) {
	case proto.Snapshot:
		g, err := core.Restore(m.Game)
		if err != nil {
			p.t.Fatalf("restore: %v", err)
		}
		p.game = g
	case proto.MoveApplied:
		var err error
		if m.C == proto.Pass {
			err = p.game.Pass()
		} else {
			err = p.game.Simulate(p.game.Geo.Index(int(m.C), int(m.R)))
		}
		if err != nil || p.game.Hash() != m.Hash {
			p.t.Fatalf("seat %d move %d: err %v, hash %x, host %x", p.seat, m.Seq, err, p.game.Hash(), m.Hash)
		}
	}
}

// autoplay plays the player's turns with its first legal move until the game ends, and returns the winner.
func (p *player) autoplay() int {
	for {
		m := p.next()
		if m == nil {
			return -1
		}
		p.track(m)
		if p.game == nil {
			continue
		}
		if p.game.Over() {
			return int(p.game.Winner)
		}
		if _, ok := m.(proto.Lobby); !ok && p.game.Turn == p.seat {
			cell := p.game.LegalMoves(nil)[0]
			c, r := p.game.Geo.Coords(cell)
			p.send(proto.Move{C: uint8(c), R: uint8(r)})
		}
	}
}

func newRoom(t *testing.T, cfg hub.Config) *hub.Room {
	t.Helper()
	cfg.Code = "ABC234"
	if cfg.Cols == 0 {
		cfg.Cols, cfg.Rows = 4, 4
	}
	room, err := hub.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(room.Close)
	return room
}

func humans(n int, extra ...hub.Seat) []hub.Seat {
	s := make([]hub.Seat, n)
	for i := range s {
		s[i].Human = true
	}
	return append(s, extra...)
}

func start(t *testing.T, room *hub.Room, ps ...*player) {
	t.Helper()
	for _, p := range ps {
		p.send(proto.Ready{On: true})
	}
	ps[0].lobbyWhere(func(l proto.Lobby) bool {
		for _, s := range l.Seats {
			if s.State == proto.SeatHere && !s.Ready {
				return false
			}
		}
		return true
	})
	if err := room.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestFullGame(t *testing.T) {
	var closed atomic.Int32
	room := newRoom(t, hub.Config{Cols: 5, Rows: 5, Seats: humans(2, hub.Seat{AI: ai.Mason}),
		OnClose: func(hub.Conn) { closed.Add(1) }})
	a, b := join(t, room, "Ana", proto.Token{}), join(t, room, " Bo\x00 ", proto.Token{})
	if a.seat != 0 || b.seat != 1 || a.token == b.token {
		t.Fatalf("seats %d %d", a.seat, b.seat)
	}
	if err := room.Start(); !errors.Is(err, hub.ErrState) {
		t.Fatalf("start before ready: %v", err)
	}
	start(t, room, a, b)

	winners := make(chan int, 2)
	go func() { winners <- a.autoplay() }()
	go func() { winners <- b.autoplay() }()
	if w1, w2 := <-winners, <-winners; w1 != w2 || w1 < 0 {
		t.Fatalf("winners %d and %d", w1, w2)
	}

	if err := room.Rematch(); err != nil {
		t.Fatal(err)
	}
	l := a.lobbyWhere(func(l proto.Lobby) bool { return !l.Playing })
	if l.Seats[1].Name != "Bo" || l.Seats[0].Ready || l.Seats[2].State != proto.SeatAI {
		t.Fatalf("lobby after rematch: %+v", l.Seats)
	}
	room.Close()
	if bye := waitFor[proto.Bye](a); bye.Reason != proto.ReasonClosed {
		t.Fatalf("bye %v", bye.Reason)
	}
	deadline := time.Now().Add(5 * time.Second)
	for closed.Load() != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if closed.Load() != 2 {
		t.Fatalf("OnClose ran %d times, want 2", closed.Load())
	}
}

func TestRejoin(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(2)})
	a, b := join(t, room, "Ana", proto.Token{}), join(t, room, "Bo", proto.Token{})
	start(t, room, a, b)
	waitFor[proto.Snapshot](b)

	b.conn.Close() // a crash, not a Leave
	a.lobbyWhere(func(l proto.Lobby) bool { return l.Seats[1].State == proto.SeatAway })
	if !room.Rejoins(b.token) || room.Rejoins(proto.Token{}) {
		t.Fatal("Rejoins")
	}
	if p := connect(t, room); true {
		p.send(proto.Hello{Code: "abc234"}) // a newcomer mid-game finds no seat
		if bye := waitFor[proto.Bye](p); bye.Reason != proto.ReasonFull {
			t.Fatalf("newcomer: %v", bye.Reason)
		}
	}
	back := join(t, room, "", b.token)
	if back.seat != 1 || back.token != b.token {
		t.Fatalf("rejoined seat %d", back.seat)
	}
	if s := waitFor[proto.Snapshot](back); s.Game.Players != 2 {
		t.Fatalf("snapshot %+v", s)
	}
	// The same token again replaces the connection.
	again := join(t, room, "", b.token)
	if bye := waitFor[proto.Bye](back); bye.Reason != proto.ReasonReplaced || again.seat != 1 {
		t.Fatalf("second rejoin: %v, seat %d", bye.Reason, again.seat)
	}
}

func TestGraceSkipAndReplace(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(3), Grace: 20 * time.Millisecond})
	a, b, c := join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{})
	start(t, room, a, b, c)
	a.track(waitFor[proto.Snapshot](a))
	if err := room.Skip(1); !errors.Is(err, hub.ErrState) {
		t.Fatalf("skip a connected seat: %v", err)
	}
	b.send(proto.Leave{})
	c.conn.Close()
	a.lobbyWhere(func(l proto.Lobby) bool {
		return l.Seats[1].State == proto.SeatGone && l.Seats[2].State == proto.SeatGone
	})
	if err := room.Skip(1); err != nil {
		t.Fatal(err)
	}
	if err := room.Replace(2, ai.Jester); err != nil {
		t.Fatal(err)
	}
	// a plays on; seat 1 passes and seat 2's AI moves, until a wins.
	passed := false
	for !a.game.Over() {
		if a.game.Turn == 0 {
			cell := a.game.LegalMoves(nil)[0]
			col, row := a.game.Geo.Coords(cell)
			a.send(proto.Move{C: uint8(col), R: uint8(row)})
		}
		mv := waitFor[proto.MoveApplied](a)
		a.track(mv)
		passed = passed || mv.Player == 1 && mv.C == proto.Pass
	}
	if !passed {
		t.Fatal("seat 1 never passed")
	}
}

func TestKickAndTurnTimer(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(3), TurnTimer: 10 * time.Millisecond})
	a, b, c := join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{})
	start(t, room, a, b, c)
	if err := room.Kick(2); err != nil {
		t.Fatal(err)
	}
	if bye := waitFor[proto.Bye](c); bye.Reason != proto.ReasonKicked || room.Rejoins(c.token) {
		t.Fatalf("kick: %v", bye.Reason)
	}
	// Nobody moves: the timer plays for each seat in turn, the kicked one included.
	seen := map[uint8]bool{}
	for len(seen) < 3 {
		seen[waitFor[proto.MoveApplied](a).Player] = true
	}
}

// TestConfigure changes a lobby's board and seats under its players: a seat that turns into an AI lets its player go,
// the others keep theirs, a new seat opens, and a new code applies to new players only.
func TestConfigure(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(3)})
	a, b := join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{})
	if err := room.Configure(5, 6, []hub.Seat{{Human: true}, {AI: ai.Jester}, {Human: true}, {Human: true}}); err != nil {
		t.Fatal(err)
	}
	if bye := waitFor[proto.Bye](b); bye.Reason != proto.ReasonKicked || room.Rejoins(b.token) {
		t.Fatalf("a seat turned AI: %v", bye.Reason)
	}
	l := a.lobbyWhere(func(l proto.Lobby) bool { return len(l.Seats) == 4 })
	if l.Cols != 5 || l.Rows != 6 || l.Seats[0].State != proto.SeatHere || l.Seats[1].State != proto.SeatAI ||
		l.Seats[3].State != proto.SeatOpen {
		t.Fatalf("lobby: %+v", l)
	}
	room.SetCode("XYZ789")
	c := connect(t, room)
	c.send(proto.Hello{Code: "ABC234"})
	if bye := waitFor[proto.Bye](c); bye.Reason != proto.ReasonBadCode {
		t.Fatalf("the old code: %v", bye.Reason)
	}
	if err := room.Configure(2, 2, humans(5)); err == nil {
		t.Fatal("a board too small for its players")
	}
	if err := room.Configure(5, 6, []hub.Seat{{Human: true}, {AI: ai.Jester}}); err != nil {
		t.Fatal(err)
	}
	start(t, room, a)
	if room.Configure(5, 5, humans(2)) != hub.ErrState {
		t.Fatal("configured mid-game")
	}
}

// TestRemoveSeat takes a middle seat out of a lobby: its player is let go, and the one behind it moves up and is told
// its new seat.
func TestRemoveSeat(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(3)})
	a, b, c := join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{})
	if err := room.RemoveSeat(1); err != nil {
		t.Fatal(err)
	}
	if bye := waitFor[proto.Bye](b); bye.Reason != proto.ReasonKicked {
		t.Fatalf("the removed seat's player: %v", bye.Reason)
	}
	if w := waitFor[proto.Welcome](c); w.Seat != 1 || w.Token != c.token {
		t.Fatalf("the moved player: seat %d", w.Seat)
	}
	l := a.lobbyWhere(func(l proto.Lobby) bool { return len(l.Seats) == 2 })
	if l.Seats[0].State != proto.SeatHere || l.Seats[1].State != proto.SeatHere {
		t.Fatalf("lobby: %+v", l.Seats)
	}
	if room.RemoveSeat(0) != hub.ErrSeat {
		t.Fatal("a room keeps its two seats")
	}
}

// TestSwapSeats trades a player's seat with a bot's in a lobby: the player is told their new seat and the bot moves.
// Two players can't trade.
func TestSwapSeats(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(2, hub.Seat{AI: ai.Spy})})
	a, b := join(t, room, "", proto.Token{}), join(t, room, "", proto.Token{})
	if err := room.SwapSeats(1, 2); err != nil {
		t.Fatal(err)
	}
	if w := waitFor[proto.Welcome](b); w.Seat != 2 || w.Token != b.token {
		t.Fatalf("the moved player: seat %d", w.Seat)
	}
	l := a.lobbyWhere(func(l proto.Lobby) bool { return l.Seats[1].State == proto.SeatAI })
	if l.Seats[1].AI != uint8(ai.Spy) || l.Seats[2].State != proto.SeatHere {
		t.Fatalf("lobby: %+v", l.Seats)
	}
	if room.SwapSeats(1, 1) != hub.ErrSeat || room.SwapSeats(0, 3) != hub.ErrSeat || room.SwapSeats(0, 2) != hub.ErrSeat {
		t.Fatal("swapped a seat with itself, one past the end, or two players")
	}
}

func TestRejects(t *testing.T) {
	room := newRoom(t, hub.Config{Seats: humans(1, hub.Seat{AI: ai.Jester})})
	for name, tc := range map[string]struct {
		raw  []byte
		want proto.Reason
	}{
		"move first": {proto.Append(nil, proto.Move{}), proto.ReasonBadMessage},
		"wrong code": {proto.Append(nil, proto.Hello{Code: "ZZZ222"}), proto.ReasonBadCode},
		"version":    {[]byte{proto.Version + 1, byte(proto.TypeHello)}, proto.ReasonVersion},
		"garbage":    {[]byte{proto.Version, 99}, proto.ReasonBadMessage},
		"too big":    {make([]byte, hub.MaxMessage+1), proto.ReasonBadMessage},
	} {
		p := connect(t, room)
		if err := p.conn.Send(tc.raw); err != nil {
			t.Fatal(err)
		}
		if bye := waitFor[proto.Bye](p); bye.Reason != tc.want {
			t.Errorf("%s: %v, want %v", name, bye.Reason, tc.want)
		}
	}

	p := join(t, room, "", proto.Token{})
	if full := connect(t, room); true {
		full.send(proto.Hello{Code: "ABC234"})
		if bye := waitFor[proto.Bye](full); bye.Reason != proto.ReasonFull {
			t.Fatalf("second human: %v", bye.Reason)
		}
	}
	for i := 0; i < 100; i++ { // far over the burst
		_ = p.conn.Send(proto.Append(nil, proto.Resync{}))
	}
	if bye := waitFor[proto.Bye](p); bye.Reason != proto.ReasonRateLimit {
		t.Fatalf("flood: %v", bye.Reason)
	}
}

func TestNewRejects(t *testing.T) {
	for name, cfg := range map[string]hub.Config{
		"one seat":   {Cols: 3, Rows: 3, Seats: humans(1)},
		"tiny board": {Cols: 2, Rows: 3, Seats: humans(2)},
		"bad AI":     {Cols: 3, Rows: 3, Seats: humans(1, hub.Seat{AI: ai.NumStrategies})},
	} {
		if _, err := hub.New(cfg); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
