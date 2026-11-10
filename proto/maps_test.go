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
	circuit.Edges = []core.MapEdge{{A: 8, B: 9, Kind: core.Gate, Group: 1, InitiallyOpen: true}, {A: 24, B: 25, Kind: core.BreakableBarrier}}
	portals := core.NewMap(7, 7)
	portals.Cells[16], portals.Cells[32] = core.MapCell{Feature: core.Portal, Group: 1}, core.MapCell{Feature: core.Portal, Group: 1}
	portals.Cells[16].PortalMode, portals.Cells[32].PortalMode = core.PortalVertical, core.PortalVertical
	landfills := core.NewMap(7, 7)
	landfills.Cells[24].Terrain = core.Landfill
	landfills.Cells[0].Terrain = core.Sinkhole
	for _, m := range append(core.StarterMaps(), circuit, portals, landfills) {
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
			g.EdgeDurability[1] = 1
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

func TestPartialLandfillSurvivesResync(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Terrain = core.Landfill
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []uint8{4, 3, 2, 1, 0} { // its fill target is 4, its capacity as Floor
		s := g.Snapshot()
		s.Durability[12] = remaining
		g, err = core.Restore(s)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := Decode(Append(nil, Snapshot{Seq: 9, Game: g.Snapshot()}))
		if err != nil {
			t.Fatal(err)
		}
		restored, err := core.Restore(decoded.(Snapshot).Game)
		if err != nil || restored.Hash() != g.Hash() || restored.Geo.Playable(12) != (remaining == 0) {
			t.Fatalf("Landfill with %d left to fill lost during resync: %v", remaining, err)
		}
	}
}
