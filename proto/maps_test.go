package proto

import (
	"bytes"
	"orbitrix/core"
	"reflect"
	"testing"
)

func TestMapMessagesRoundTrip(t *testing.T) {
	circuit := core.NewMap(7, 7)
	circuit.Cells[16] = core.MapCell{Circuit: core.Latch, Group: 1}
	circuit.Cells[24] = core.MapCell{Circuit: core.Toggle, Group: 1}
	circuit.Cells[32] = core.MapCell{Circuit: core.Door, Group: 1, InitiallyOpen: true}
	circuit.Edges = []core.MapEdge{{A: 8, B: 9, Kind: core.Gate, Group: 1, InitiallyOpen: true}}
	for _, m := range append(core.StarterMaps(), circuit) {
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		if m.Cells[24].Circuit == core.Toggle {
			if _, err := g.Apply(16); err != nil {
				t.Fatal(err)
			}
			if _, err := g.Apply(24); err != nil {
				t.Fatal(err)
			}
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
