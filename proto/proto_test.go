package proto

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"orbitrix/core"
)

func samples(t testing.TB) []Msg {
	g, err := core.NewGame(4, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(5); err != nil {
		t.Fatal(err)
	}
	return []Msg{
		Hello{Name: "Ana ✨", Code: "K7M2QX", Token: Token{1, 2, 3, 15: 9}},
		Hello{},
		Ready{On: true},
		Ready{},
		Move{C: 39, R: 0},
		Resync{},
		Leave{},
		Welcome{Seat: 11, Token: Token{0: 0xff, 15: 1}},
		Lobby{Cols: 40, Rows: 13, TurnTimer: 90, Playing: true, Seats: []Seat{
			{State: SeatHere, Ready: true, Name: "Host"},
			{State: SeatAI, AI: 11},
			{State: SeatSkipped, Name: "Bo"},
		}},
		Lobby{Cols: 3, Rows: 3},
		MoveApplied{Seq: 1 << 30, Player: 2, C: 4, R: 7, Hash: 0xdeadbeefcafef00d},
		MoveApplied{Seq: 3, Player: 1, C: Pass, R: Pass},
		Snapshot{Seq: 12, Game: g.Snapshot()},
		Bye{Reason: ReasonKicked},
	}
}

func TestRoundTrip(t *testing.T) {
	for _, m := range samples(t) {
		b := Append(nil, m)
		got, err := Decode(b)
		if err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("round trip:\n got %#v\nwant %#v", got, m)
		}
		if !bytes.Equal(Append(nil, got), b) {
			t.Fatalf("%T: re-encoding differs", m)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	long := Append(nil, Hello{Name: string(bytes.Repeat([]byte("x"), MaxName))})
	long[2]++ // name length one over the limit
	for name, b := range map[string][]byte{
		"empty":         nil,
		"type only":     {Version},
		"unknown type":  {Version, 200},
		"short move":    {Version, byte(TypeMove), 1},
		"trailing byte": {Version, byte(TypeLeave), 0},
		"bad bool":      {Version, byte(TypeReady), 2},
		"bad reason":    {Version, byte(TypeBye), byte(numReasons)},
		"long name":     long,
		"bad utf-8":     append(Append(nil, Hello{})[:2], 1, 0xff, 0),
		"13 seats":      {Version, byte(TypeLobby), 9, 9, 0, 0, 0, 13},
		"bad seat":      {Version, byte(TypeLobby), 9, 9, 0, 0, 0, 1, byte(numSeatStates), 0, 0, 0},
		"short board":   Append(nil, Snapshot{Game: core.Snapshot{Cols: 3, Rows: 3}})[:20],
	} {
		if _, err := Decode(b); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	if _, err := Decode([]byte{Version + 1, byte(TypeBye), 0}); !errors.Is(err, ErrVersion) {
		t.Errorf("other version: %v", err)
	}
}

// FuzzDecode checks that Decode never panics and that whatever it accepts re-encodes to the same bytes.
func FuzzDecode(f *testing.F) {
	for _, m := range samples(f) {
		f.Add(Append(nil, m))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := Decode(b)
		if err != nil {
			return
		}
		if again := Append(nil, m); !bytes.Equal(again, b) {
			t.Fatalf("%T re-encodes to %x, decoded from %x", m, again, b)
		}
	})
}
