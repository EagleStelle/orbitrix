package hub

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/time/rate"

	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
)

// Conn is one player's connection, whatever carries it.
type Conn interface {
	// Send delivers one whole message. It may block; the hub calls it from one goroutine per Conn.
	Send(msg []byte) error
	// Receive returns the next whole message. An error ends the connection.
	Receive() ([]byte, error)
	// Close ends the connection and unblocks Receive. It is safe to call more than once.
	Close() error
}

// Limits on every Conn. Players send only small messages (a Hello is the largest), and at most a few a second.
const (
	MaxMessage   = 128
	msgRate      = 20 // messages a second, sustained
	msgBurst     = 40
	helloTimeout = 5 * time.Second
	outQueue     = 64 // messages waiting to be sent before a slow player is dropped
)

// DefaultGrace is how long a dropped player's seat is held.
const DefaultGrace = 60 * time.Second

var (
	ErrState = errors.New("hub: not allowed now")
	ErrSeat  = errors.New("hub: no such seat")
)

// Seat is a seat as the host sets it up: a human or an AI.
type Seat struct {
	Human bool
	AI    ai.Strategy
}

// Config sets up a room.
type Config struct {
	Code       string // room code a player's Hello must carry; empty accepts any
	Cols, Rows int
	Seats      []Seat // in turn order
	TurnTimer  time.Duration
	Grace      time.Duration // seat hold after a drop; 0 means DefaultGrace
	AIDelay    time.Duration // pause before each AI move or pass, so the stream of moves isn't a burst
	Seed       uint64        // AI seed; 0 picks a random one
	// Budget is an AI's work per move; nil means Strategy.Budget(). A browser host passes a smaller one.
	Budget func(s ai.Strategy, cells int) int
	// OnClose is called once for every Conn the room has finished with, after its last message was sent. The host
	// uses it to revoke the connection's TURN credential.
	OnClose func(Conn)
}

// Room is one room: its lobby, its seats and the game. The host's process runs it; every player, the host's own
// included, talks to it only through a Conn. All methods are safe for concurrent use.
type Room struct {
	mu    sync.Mutex
	cfg   Config
	seats []seat
	peers map[*peer]struct{}
	game  *core.Game // nil in the lobby
	seq   uint32     // moves applied this game
	bot   ai.Bot
	timer *time.Timer // the pending AI move, pass or turn timeout
	shut  bool
	// epochs numbers the seats' holds across the room, so a grace timer whose seat was dropped, released or moved
	// matches no seat and does nothing.
	epochs int
}

type seat struct {
	Seat
	name  string
	token proto.Token // zero while nobody holds the seat
	peer  *peer
	ready bool
	epoch int // this hold's number from Room.epochs; a stale grace timer's never matches
	away  *time.Timer
	gone  bool // its player left mid-game; waits for Replace or Skip
	skip  bool
}

type peer struct {
	conn   Conn
	out    chan []byte
	seat   int // -1 until Hello
	lim    *rate.Limiter
	hello  *time.Timer
	closed bool
}

// New checks cfg and returns a room in the lobby.
func New(cfg Config) (*Room, error) {
	if _, err := core.NewGame(cfg.Cols, cfg.Rows, len(cfg.Seats)); err != nil {
		return nil, fmt.Errorf("hub: %w", err)
	}
	for i, s := range cfg.Seats {
		if !s.Human && s.AI >= ai.NumStrategies {
			return nil, fmt.Errorf("hub: seat %d: unknown strategy %d", i, s.AI)
		}
	}
	if cfg.TurnTimer/time.Second > 0xffff {
		return nil, fmt.Errorf("hub: turn timer %v too long", cfg.TurnTimer)
	}
	if cfg.Grace <= 0 {
		cfg.Grace = DefaultGrace
	}
	if cfg.Seed == 0 {
		var b [8]byte
		_, _ = rand.Read(b[:]) // never fails
		for _, v := range b {
			cfg.Seed = cfg.Seed<<8 | uint64(v)
		}
	}
	r := &Room{cfg: cfg, seats: make([]seat, len(cfg.Seats), core.MaxPlayers), peers: map[*peer]struct{}{}}
	for i, s := range cfg.Seats {
		r.seats[i].Seat = s
	}
	return r, nil
}

// Add runs c as a new player connection. It returns at once; the room reads and writes c on its own goroutines.
func (r *Room) Add(c Conn) {
	p := &peer{conn: c, out: make(chan []byte, outQueue), seat: -1, lim: rate.NewLimiter(msgRate, msgBurst)}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut {
		close(p.out)
	} else {
		r.peers[p] = struct{}{}
		p.hello = time.AfterFunc(helloTimeout, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if p.seat < 0 {
				r.bye(p, proto.ReasonTimeout)
			}
		})
		go r.read(p)
	}
	go r.write(p)
}

// write sends p's queue, then closes the connection once drop has closed the queue.
func (r *Room) write(p *peer) {
	for m := range p.out {
		if p.conn.Send(m) != nil {
			p.conn.Close()
			for range p.out { // keep draining until drop closes the queue
			}
		}
	}
	p.conn.Close()
	if r.cfg.OnClose != nil {
		r.cfg.OnClose(p.conn)
	}
}

func (r *Room) read(p *peer) {
	for {
		b, err := p.conn.Receive()
		r.mu.Lock()
		switch {
		case err != nil:
			r.drop(p)
		case len(b) > MaxMessage:
			r.bye(p, proto.ReasonBadMessage)
		case !p.lim.Allow():
			r.bye(p, proto.ReasonRateLimit)
		default:
			r.handle(p, b)
		}
		done := p.closed
		r.mu.Unlock()
		if done {
			return
		}
	}
}

func (r *Room) handle(p *peer, b []byte) {
	m, err := proto.Decode(b)
	if errors.Is(err, proto.ErrVersion) {
		r.bye(p, proto.ReasonVersion)
		return
	}
	if err != nil {
		r.bye(p, proto.ReasonBadMessage)
		return
	}
	if h, ok := m.(proto.Hello); ok != (p.seat < 0) {
		r.bye(p, proto.ReasonBadMessage) // Hello first, and only once
		return
	} else if ok {
		r.hello(p, h)
		return
	}
	s := &r.seats[p.seat]
	switch m := m.(type) {
	case proto.Ready:
		if r.game == nil && s.ready != m.On {
			s.ready = m.On
			r.broadcastLobby()
		}
	case proto.Move:
		// A late or illegal move is ignored, not punished: it can race a turn timeout.
		g := r.game
		if g != nil && g.Turn == p.seat && int(m.C) < g.Geo.Cols && int(m.R) < g.Geo.Rows {
			if cell := g.Geo.Index(int(m.C), int(m.R)); g.Legal(cell) {
				r.apply(cell)
			}
		}
	case proto.Resync:
		if r.game != nil {
			r.send(p, proto.Snapshot{Seq: r.seq, Game: r.game.Snapshot()})
		}
	case proto.Leave:
		s.peer = nil // detach first, so drop doesn't hold the seat
		r.release(p.seat)
		r.drop(p)
		r.broadcastLobby()
	default:
		r.bye(p, proto.ReasonBadMessage)
	}
}

func (r *Room) hello(p *peer, h proto.Hello) {
	if r.cfg.Code != "" && !strings.EqualFold(h.Code, r.cfg.Code) {
		r.bye(p, proto.ReasonBadCode)
		return
	}
	i := r.seatFor(h.Token)
	if i < 0 {
		r.bye(p, proto.ReasonFull)
		return
	}
	s := &r.seats[i]
	if old := s.peer; old != nil {
		s.peer = nil
		r.bye(old, proto.ReasonReplaced)
	}
	if s.token == (proto.Token{}) {
		_, _ = rand.Read(s.token[:]) // never fails
		s.name = cleanName(h.Name)
	}
	if s.away != nil {
		s.away.Stop()
	}
	s.peer, p.seat = p, i
	p.hello.Stop()
	r.send(p, proto.Welcome{Seat: uint8(i), Token: s.token})
	r.broadcastLobby()
	if r.game != nil {
		r.send(p, proto.Snapshot{Seq: r.seq, Game: r.game.Snapshot()})
	}
}

// seatFor is the seat token rejoins, or the first open human seat in the lobby, or -1.
func (r *Room) seatFor(t proto.Token) int {
	for i, s := range r.seats {
		if t != (proto.Token{}) && s.token == t {
			return i
		}
	}
	for i, s := range r.seats {
		if r.game == nil && s.Human && s.token == (proto.Token{}) {
			return i
		}
	}
	return -1
}

// cleanName keeps a name to one line of printable text.
func cleanName(s string) string {
	return strings.TrimSpace(strings.Map(func(c rune) rune {
		if !unicode.IsPrint(c) {
			return -1
		}
		return c
	}, s))
}

// bye tells p why, then drops it.
func (r *Room) bye(p *peer, reason proto.Reason) {
	r.send(p, proto.Bye{Reason: reason})
	r.drop(p)
}

// drop closes p once its queue is sent. A seated player who drops keeps their seat for the grace period.
func (r *Room) drop(p *peer) {
	if p.closed {
		return
	}
	p.closed = true
	if p.hello != nil {
		p.hello.Stop()
	}
	delete(r.peers, p)
	close(p.out)
	if p.seat < 0 || r.seats[p.seat].peer != p {
		return
	}
	s := &r.seats[p.seat]
	s.peer, s.ready = nil, false
	if r.shut {
		return
	}
	r.hold(p.seat)
	r.broadcastLobby()
}

// hold keeps seat i for its away player through the grace period, then frees it.
func (r *Room) hold(i int) {
	s := &r.seats[i]
	r.epochs++
	s.epoch = r.epochs
	epoch := s.epoch
	s.away = time.AfterFunc(r.cfg.Grace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if s.epoch == epoch && s.peer == nil {
			r.release(i)
			r.broadcastLobby()
		}
	})
}

// release frees seat i for good: open again in the lobby, gone mid-game until the host replaces or skips it.
func (r *Room) release(i int) {
	s := &r.seats[i]
	s.token, s.ready = proto.Token{}, false
	r.epochs++
	s.epoch = r.epochs
	if s.away != nil {
		s.away.Stop()
	}
	if r.game == nil {
		s.name = ""
	} else if s.Human && !s.skip {
		s.gone = true
	}
}

// send queues m for p. A player whose queue is full isn't reading, so they are dropped.
func (r *Room) send(p *peer, m proto.Msg) { r.queue(p, proto.Append(nil, m)) }

func (r *Room) queue(p *peer, b []byte) {
	if p.closed {
		return
	}
	select {
	case p.out <- b:
	default:
		r.drop(p)
	}
}

func (r *Room) broadcast(m proto.Msg) {
	b := proto.Append(nil, m)
	for p := range r.peers {
		if p.seat >= 0 {
			r.queue(p, b)
		}
	}
}

func (r *Room) broadcastLobby() { r.broadcast(r.lobby()) }

// Lobby is the room as its players see it.
func (r *Room) Lobby() proto.Lobby {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lobby()
}

func (r *Room) lobby() proto.Lobby {
	l := proto.Lobby{Cols: uint8(r.cfg.Cols), Rows: uint8(r.cfg.Rows), TurnTimer: uint16(r.cfg.TurnTimer / time.Second),
		Playing: r.game != nil, Seats: make([]proto.Seat, len(r.seats))}
	for i, s := range r.seats {
		st := proto.SeatOpen
		switch {
		case !s.Human:
			st = proto.SeatAI
		case s.skip:
			st = proto.SeatSkipped
		case s.gone:
			st = proto.SeatGone
		case s.peer != nil:
			st = proto.SeatHere
		case s.token != (proto.Token{}):
			st = proto.SeatAway
		}
		l.Seats[i] = proto.Seat{State: st, AI: uint8(s.AI), Ready: s.ready, Name: s.name}
	}
	return l
}

// Start begins a game once every human seat is held by a connected player who is ready.
func (r *Room) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut || r.game != nil {
		return ErrState
	}
	for i, s := range r.seats {
		if s.Human && (s.peer == nil || !s.ready) {
			return fmt.Errorf("%w: seat %d isn't ready", ErrState, i)
		}
	}
	r.game, _ = core.NewGame(r.cfg.Cols, r.cfg.Rows, len(r.seats)) // checked by New
	r.seq = 0
	r.broadcastLobby()
	r.broadcast(proto.Snapshot{Game: r.game.Snapshot()})
	r.turn()
	return nil
}

// Rematch returns a finished game's room to the lobby. Everyone has to be ready again, and seats whose players left
// open up.
func (r *Room) Rematch() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut || r.game == nil || !r.game.Over() {
		return ErrState
	}
	r.stopGame()
	return nil
}

// Kick removes the player in seat i. Their token stops working, so they can't rejoin.
func (r *Room) Kick(i int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i < 0 || i >= len(r.seats) || !r.seats[i].Human || r.seats[i].token == (proto.Token{}) {
		return ErrSeat
	}
	if p := r.seats[i].peer; p != nil {
		r.seats[i].peer = nil
		r.bye(p, proto.ReasonKicked)
	}
	r.release(i)
	r.broadcastLobby()
	return nil
}

// Configure changes the board and the seats while the room is in its lobby. A player whose seat goes away or turns
// into an AI is let go as if kicked; every other player keeps their seat.
func (r *Room) Configure(cols, rows int, seats []Seat) error {
	if _, err := core.NewGame(cols, rows, len(seats)); err != nil {
		return fmt.Errorf("hub: %w", err)
	}
	for i, s := range seats {
		if !s.Human && s.AI >= ai.NumStrategies {
			return fmt.Errorf("hub: seat %d: unknown strategy %d", i, s.AI)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut || r.game != nil {
		return ErrState
	}
	for i := range r.seats {
		if i < len(seats) && seats[i].Human && r.seats[i].Human {
			continue
		}
		if p := r.seats[i].peer; p != nil {
			r.seats[i].peer = nil
			r.bye(p, proto.ReasonKicked)
		}
		r.release(i)
	}
	held := len(r.seats)
	r.seats = r.seats[:len(seats)] // New gave the slice room for every seat, so grace timers keep valid pointers
	for i, s := range seats {
		if i >= held {
			r.seats[i] = seat{epoch: r.seats[i].epoch} // a timer from a seat removed earlier must stay stale
		}
		r.seats[i].Seat = s
	}
	r.cfg.Cols, r.cfg.Rows = cols, rows
	r.broadcastLobby()
	return nil
}

// RemoveSeat takes seat i out of the room in its lobby. Its player is let go as if kicked; the seats after it move up
// one, keeping their players, who are told their new seat (a Welcome with the same token).
func (r *Room) RemoveSeat(i int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut || r.game != nil {
		return ErrState
	}
	if i < 0 || i >= len(r.seats) || len(r.seats) <= core.MinPlayers {
		return ErrSeat
	}
	if p := r.seats[i].peer; p != nil {
		r.seats[i].peer = nil
		r.bye(p, proto.ReasonKicked)
	}
	r.release(i)
	copy(r.seats[i:], r.seats[i+1:])
	r.seats = r.seats[:len(r.seats)-1]
	for j := i; j < len(r.seats); j++ {
		s := &r.seats[j]
		if s.peer != nil {
			s.peer.seat = j
			r.send(s.peer, proto.Welcome{Seat: uint8(j), Token: s.token})
		} else if s.token != (proto.Token{}) && s.away != nil { // away: its grace starts again at its new place
			s.away.Stop()
			r.hold(j)
		}
	}
	r.broadcastLobby()
	return nil
}

// SetCode changes the code a new player's Hello must carry; "" lets anyone in. Seated players keep their seats.
func (r *Room) SetCode(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.Code = code
}

// Replace hands human seat i to an AI. Its player must be gone: an open seat in the lobby, or one whose player left
// a game (after the grace period, a kick or Leave).
func (r *Room) Replace(i int, s ai.Strategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i < 0 || i >= len(r.seats) || s >= ai.NumStrategies {
		return ErrSeat
	}
	st := &r.seats[i]
	if !st.Human || st.token != (proto.Token{}) {
		return ErrState
	}
	st.Seat, st.gone, st.skip, st.name = Seat{AI: s}, false, false, ""
	r.broadcastLobby()
	if r.game != nil && r.game.Turn == i {
		r.turn()
	}
	return nil
}

// Skip passes seat i's turns for the rest of the game. Its player must have left it.
func (r *Room) Skip(i int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i < 0 || i >= len(r.seats) {
		return ErrSeat
	}
	st := &r.seats[i]
	if r.game == nil || !st.gone {
		return ErrState
	}
	st.gone, st.skip = false, true
	r.broadcastLobby()
	if r.game.Turn == i {
		r.turn()
	}
	return nil
}

// Rejoins reports whether t is a seat's rejoin token, so the host can approve that player without asking.
func (r *Room) Rejoins(t proto.Token) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.seats {
		if t != (proto.Token{}) && s.token == t {
			return true
		}
	}
	return false
}

// Close ends the room: every player gets Bye{ReasonClosed}.
func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shut {
		return
	}
	r.shut = true
	if r.timer != nil {
		r.timer.Stop()
	}
	for p := range r.peers {
		r.bye(p, proto.ReasonClosed)
	}
	for i := range r.seats {
		if s := &r.seats[i]; s.away != nil {
			s.away.Stop()
		}
	}
}

// stopGame returns to the lobby.
func (r *Room) stopGame() {
	if r.timer != nil {
		r.timer.Stop()
	}
	r.game = nil
	for i := range r.seats {
		s := &r.seats[i]
		s.ready = false
		if s.gone || s.skip {
			s.gone, s.skip, s.name = false, false, ""
		}
	}
	r.broadcastLobby()
}

// turn schedules what the seat to move does on its own: an AI moves, a skipped seat passes and a human's timer
// starts.
func (r *Room) turn() {
	if r.timer != nil {
		r.timer.Stop()
	}
	g := r.game
	if g.Over() {
		return
	}
	if !r.anyoneActive() {
		r.stopGame() // only skipped seats are left, so nothing could ever change
		return
	}
	s := &r.seats[g.Turn]
	switch {
	case !s.Human:
		r.after(r.cfg.AIDelay, func() { r.think(s.AI) })
	case s.skip:
		r.after(r.cfg.AIDelay, r.pass)
	case r.cfg.TurnTimer > 0:
		r.after(r.cfg.TurnTimer, func() { r.think(ai.Jester) }) // out of time: a random legal move
	}
}

// after runs f in d unless the game has moved on by then.
func (r *Room) after(d time.Duration, f func()) {
	g, seq := r.game, r.seq
	r.timer = time.AfterFunc(d, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.shut && r.game == g && r.seq == seq {
			f()
		}
	})
}

func (r *Room) anyoneActive() bool {
	for i, s := range r.seats {
		if !s.skip && r.game.Alive(i) {
			return true
		}
	}
	return false
}

// think plays strategy s's move for the seat to move. Like local play, the seed is the room's plus the move number.
func (r *Room) think(s ai.Strategy) {
	g := r.game
	budget := s.Budget()
	if r.cfg.Budget != nil {
		budget = r.cfg.Budget(s, g.Geo.Cells())
	}
	rng := core.NewRNG(r.cfg.Seed + uint64(g.Moves))
	r.apply(r.bot.Choose(g, s, rng.Uint64(), budget))
}

func (r *Room) apply(cell int) {
	g := r.game
	player := g.Turn
	if g.Simulate(cell) != nil {
		return // callers only pass legal cells
	}
	r.seq++
	c, row := g.Geo.Coords(cell)
	r.broadcast(proto.MoveApplied{Seq: r.seq, Player: uint8(player), C: uint8(c), R: uint8(row), Hash: g.Hash()})
	r.turn()
}

func (r *Room) pass() {
	g := r.game
	player := g.Turn
	if g.Pass() != nil {
		return
	}
	r.seq++
	r.broadcast(proto.MoveApplied{Seq: r.seq, Player: uint8(player), C: proto.Pass, R: proto.Pass, Hash: g.Hash()})
	r.turn()
}
