package proto

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"

	"orbitrix/core"
)

// Version is the first byte of every message. Peers on different versions can't talk; a host answers a Hello from
// another version with Bye{ReasonVersion}, which every version decodes alike (version, type, reason).
const Version = 3

// Limits checked when decoding.
const (
	MaxName = 24 // bytes in a player name
	MaxCode = 8  // bytes in a room code
)

// Room codes, online and on the LAN, are CodeLen characters of CodeAlphabet, which leaves out 0/O/1/I as they read
// alike.
const (
	CodeLen      = 6
	CodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// NewCode returns a random room code.
func NewCode() string {
	var b [CodeLen]byte
	_, _ = rand.Read(b[:]) // never fails
	for i := range b {
		b[i] = CodeAlphabet[b[i]%byte(len(CodeAlphabet))] // 256 is a multiple of 32: no bias
	}
	return string(b[:])
}

// Pass in MoveApplied's C and R means the player's turn was passed instead of played.
const Pass = 0xFF

var (
	ErrVersion = errors.New("proto: unsupported version")
	ErrInvalid = errors.New("proto: invalid message")
)

// Type identifies a message. Values are part of the wire format: never reorder them.
type Type uint8

const (
	// Player to host.
	TypeHello Type = iota + 1
	TypeReady
	TypeMove
	TypeResync
	TypeLeave
	// Host to player.
	TypeWelcome
	TypeLobby
	TypeMoveApplied
	TypeSnapshot
	TypeBye
)

// Msg is one message. Every type below implements it.
type Msg interface{ Type() Type }

// Token is a seat's rejoin secret, issued by the host in Welcome.
type Token [16]byte

// Hello is a player's first message: a new player sends a zero Token, a returning one the Token from Welcome.
type Hello struct {
	Name, Code string
	Token      Token
}

// Ready marks the player ready (or not) in the lobby.
type Ready struct{ On bool }

// Move places the player's orb at column C, row R.
type Move struct{ C, R uint8 }

// Resync asks the host for a Snapshot after a state hash mismatch.
type Resync struct{}

// Leave gives the seat up for good: no grace period, no rejoin.
type Leave struct{}

// Welcome tells a player their seat and the token to rejoin it with.
type Welcome struct {
	Seat  uint8
	Token Token
}

// SeatState is what a seat holds right now.
type SeatState uint8

const (
	SeatAI      SeatState = iota // an AI plays it; Seat.AI is the strategy
	SeatOpen                     // a human seat nobody has taken
	SeatHere                     // a connected human
	SeatAway                     // a human who dropped, inside the grace period
	SeatGone                     // a human who left mid-game; the host may replace or skip the seat
	SeatSkipped                  // the host passes this seat's turns
	numSeatStates
)

// Seat describes one seat in a Lobby.
type Seat struct {
	State SeatState
	AI    uint8 // ai.Strategy when State is SeatAI
	Ready bool
	Name  string
}

// Lobby is the room as it stands. The host sends it on every seat change, in the lobby and during a game.
type Lobby struct {
	Cols, Rows uint8
	TurnTimer  uint16 // seconds a human has to move; 0 means no limit
	Playing    bool
	Seats      []Seat
	Map        *core.MapDefinition // nil selects the ordinary rectangular board
}

// MoveApplied reports a move the host accepted, numbered by Seq from 1 each game, with the state hash after it.
// C and R are Pass when the turn was passed.
type MoveApplied struct {
	Seq          uint32
	Player, C, R uint8
	Hash         uint64
}

// Snapshot is the whole game after move Seq: sent when a game starts (Seq 0), to a player who rejoins mid-game and
// in answer to Resync. Game comes from the network: rebuild it with core.Restore, which validates it.
type Snapshot struct {
	Seq  uint32
	Game core.Snapshot
}

// Reason says why the host closed a connection.
type Reason uint8

const (
	ReasonClosed     Reason = iota // the host closed the room
	ReasonVersion                  // the player's protocol version differs
	ReasonBadCode                  // wrong room code
	ReasonFull                     // no free seat
	ReasonKicked                   // the host removed the player
	ReasonReplaced                 // the same seat connected again
	ReasonTimeout                  // no Hello in time
	ReasonBadMessage               // a message that didn't decode or wasn't allowed
	ReasonRateLimit                // too many messages
	numReasons
)

// Bye is the host's last message before it closes a connection.
type Bye struct{ Reason Reason }

func (Hello) Type() Type       { return TypeHello }
func (Ready) Type() Type       { return TypeReady }
func (Move) Type() Type        { return TypeMove }
func (Resync) Type() Type      { return TypeResync }
func (Leave) Type() Type       { return TypeLeave }
func (Welcome) Type() Type     { return TypeWelcome }
func (Lobby) Type() Type       { return TypeLobby }
func (MoveApplied) Type() Type { return TypeMoveApplied }
func (Snapshot) Type() Type    { return TypeSnapshot }
func (Bye) Type() Type         { return TypeBye }

// Append appends m's encoding to dst and returns it.
func Append(dst []byte, m Msg) []byte {
	dst = append(dst, Version, byte(m.Type()))
	switch m := m.(type) {
	case Hello:
		dst = appendStr(appendStr(dst, m.Name), m.Code)
		dst = append(dst, m.Token[:]...)
	case Ready:
		dst = appendBool(dst, m.On)
	case Move:
		dst = append(dst, m.C, m.R)
	case Welcome:
		dst = append(append(dst, m.Seat), m.Token[:]...)
	case Lobby:
		dst = binary.BigEndian.AppendUint16(append(dst, m.Cols, m.Rows), m.TurnTimer)
		dst = append(appendBool(dst, m.Playing), byte(len(m.Seats)))
		for _, s := range m.Seats {
			dst = appendStr(appendBool(append(dst, byte(s.State), s.AI), s.Ready), s.Name)
		}
		dst = appendMap(dst, m.Map)
	case MoveApplied:
		dst = binary.BigEndian.AppendUint32(dst, m.Seq)
		dst = binary.BigEndian.AppendUint64(append(dst, m.Player, m.C, m.R), m.Hash)
	case Snapshot:
		g := m.Game
		dst = binary.BigEndian.AppendUint32(dst, m.Seq)
		dst = append(dst, byte(g.Cols), byte(g.Rows), byte(g.Players), byte(g.Turn))
		dst = binary.BigEndian.AppendUint32(dst, uint32(g.Moves))
		dst = append(binary.BigEndian.AppendUint16(dst, g.Played), byte(g.Winner))
		dst = append(dst, g.Count...)
		for _, o := range g.Owner {
			dst = append(dst, byte(o))
		}
		dst = appendMap(dst, g.Map)
		dst = appendBytes(dst, g.Durability)
		dst = appendBytes(dst, g.OpenGates)
		dst = appendBytes(dst, g.Pressed)
	case Bye:
		dst = append(dst, byte(m.Reason))
	}
	return dst
}

// Decode parses one message. It checks structure and ranges, not game rules: a Snapshot still needs core.Restore.
// Encodings are canonical, so Append(nil, m) gives back b exactly.
func Decode(b []byte) (Msg, error) {
	if len(b) < 2 {
		return nil, ErrInvalid
	}
	if b[0] != Version {
		return nil, ErrVersion
	}
	r := reader{b: b[2:]}
	var m Msg
	switch Type(b[1]) {
	case TypeHello:
		h := Hello{Name: r.str(MaxName), Code: r.str(MaxCode)}
		r.token(&h.Token)
		m = h
	case TypeReady:
		m = Ready{On: r.bool()}
	case TypeMove:
		m = Move{C: r.u8(), R: r.u8()}
	case TypeResync:
		m = Resync{}
	case TypeLeave:
		m = Leave{}
	case TypeWelcome:
		w := Welcome{Seat: r.u8()}
		r.token(&w.Token)
		m = w
	case TypeLobby:
		m = r.lobby()
	case TypeMoveApplied:
		m = MoveApplied{Seq: r.u32(), Player: r.u8(), C: r.u8(), R: r.u8(), Hash: r.u64()}
	case TypeSnapshot:
		m = r.snapshot()
	case TypeBye:
		m = Bye{Reason: Reason(r.u8())}
		if m.(Bye).Reason >= numReasons {
			r.fail()
		}
	default:
		return nil, fmt.Errorf("%w: type %d", ErrInvalid, b[1])
	}
	if r.bad || len(r.b) != 0 {
		return nil, fmt.Errorf("%w: type %d", ErrInvalid, b[1])
	}
	return m, nil
}

func appendStr(dst []byte, s string) []byte { return append(append(dst, byte(len(s))), s...) }

func appendBytes(dst, b []byte) []byte {
	return append(binary.BigEndian.AppendUint16(dst, uint16(len(b))), b...)
}

func appendMap(dst []byte, m *core.MapDefinition) []byte {
	if m == nil {
		return binary.BigEndian.AppendUint32(dst, 0)
	}
	b, err := core.EncodeMap(*m)
	if err != nil {
		// Append's callers send validated game state. If that contract is broken, send an invalid frame rather than
		// silently changing a custom map to a rectangle.
		return binary.BigEndian.AppendUint32(dst, ^uint32(0))
	}
	return append(binary.BigEndian.AppendUint32(dst, uint32(len(b))), b...)
}

func appendBool(dst []byte, v bool) []byte {
	if v {
		return append(dst, 1)
	}
	return append(dst, 0)
}

// reader consumes b; after any short or invalid read it returns zeros and sets bad.
type reader struct {
	b   []byte
	bad bool
}

func (r *reader) fail() { r.bad, r.b = true, nil }

func (r *reader) next(n int) []byte {
	if len(r.b) < n {
		r.fail()
		return make([]byte, min(n, 8)) // enough for the fixed-size reads; longer reads of bad input get nothing
	}
	v := r.b[:n:n]
	r.b = r.b[n:]
	return v
}

func (r *reader) u8() uint8   { return r.next(1)[0] }
func (r *reader) u16() uint16 { return binary.BigEndian.Uint16(r.next(2)) }
func (r *reader) u32() uint32 { return binary.BigEndian.Uint32(r.next(4)) }
func (r *reader) u64() uint64 { return binary.BigEndian.Uint64(r.next(8)) }

func (r *reader) bool() bool {
	v := r.u8()
	if v > 1 {
		r.fail()
	}
	return v == 1
}

func (r *reader) str(max int) string {
	n := int(r.u8())
	if n > max {
		r.fail()
		return ""
	}
	s := r.next(n)
	if !utf8.Valid(s) {
		r.fail()
		return ""
	}
	return string(s)
}

func (r *reader) token(t *Token) { copy(t[:], r.next(len(t))) }

func (r *reader) bytes(max int) []byte {
	n := int(r.u16())
	if n > max {
		r.fail()
		return nil
	}
	return append([]byte(nil), r.next(n)...)
}

func (r *reader) mapDefinition() *core.MapDefinition {
	n := r.u32()
	if n == 0 {
		return nil
	}
	if n > core.MaxMapBytes {
		r.fail()
		return nil
	}
	b := r.next(int(n))
	if r.bad {
		return nil
	}
	m, err := core.DecodeMap(b)
	if err != nil {
		r.fail()
		return nil
	}
	// Map imports may accept equivalent representations; the game protocol remains canonical.
	canonical, err := core.EncodeMap(m)
	if err != nil || !bytes.Equal(b, canonical) {
		r.fail()
		return nil
	}
	return &m
}

func (r *reader) lobby() Lobby {
	l := Lobby{Cols: r.u8(), Rows: r.u8(), TurnTimer: r.u16(), Playing: r.bool()}
	n := int(r.u8())
	if n > core.MaxPlayers {
		r.fail()
		return l
	}
	for i := 0; i < n && !r.bad; i++ {
		s := Seat{State: SeatState(r.u8()), AI: r.u8(), Ready: r.bool(), Name: r.str(MaxName)}
		if s.State >= numSeatStates {
			r.fail()
		}
		l.Seats = append(l.Seats, s)
	}
	l.Map = r.mapDefinition()
	if l.Map != nil && (l.Map.Cols != int(l.Cols) || l.Map.Rows != int(l.Rows)) {
		r.fail()
	}
	return l
}

func (r *reader) snapshot() Snapshot {
	s := Snapshot{Seq: r.u32()}
	g := &s.Game
	g.Cols, g.Rows, g.Players, g.Turn = int(r.u8()), int(r.u8()), int(r.u8()), int(r.u8())
	g.Moves, g.Played, g.Winner = int(r.u32()), r.u16(), int8(r.u8())
	n := g.Cols * g.Rows
	g.Count = append([]uint8(nil), r.next(n)...)
	for _, o := range r.next(n) {
		g.Owner = append(g.Owner, int8(o))
	}
	g.Map = r.mapDefinition()
	g.Durability = r.bytes(core.MaxSize * core.MaxSize)
	g.OpenGates = r.bytes(255)
	g.Pressed = r.bytes(core.MaxSize * core.MaxSize)
	return s
}
