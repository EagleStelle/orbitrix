package proto

import (
	"bytes"
	"orbitrix/core"
	"reflect"
	"testing"
)

func TestMapMessagesRoundTrip(t *testing.T) {
	for _, m := range core.StarterMaps() {
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range []Msg{Lobby{Cols: uint8(m.Cols), Rows: uint8(m.Rows), Map: &m}, Snapshot{Seq: 9, Game: g.Snapshot()}} {
			b := Append(nil, msg)
			decoded, err := Decode(b)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(msg, decoded) {
				t.Fatalf("map message differs: %T", msg)
			}
			if !bytes.Equal(Append(nil, decoded), b) {
				t.Fatal("noncanonical encoding")
			}
			if s, ok := decoded.(Snapshot); ok {
				restored, err := core.Restore(s.Game)
				if err != nil {
					t.Fatal(err)
				}
				if restored.Hash() != g.Hash() {
					t.Fatal("map snapshot lost state")
				}
			}
			for i := 0; i < len(b); i++ {
				if _, err := Decode(b[:i]); err == nil {
					t.Fatalf("accepted truncated map message at %d", i)
				}
			}
		}
	}
}
