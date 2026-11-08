package boardview

import (
	"slices"
	"testing"

	"orbitrix/core"
)

func TestMixedCircuitReplayMatchesResolvedGame(t *testing.T) {
	m := core.NewMap(9, 9)
	m.Cells[20] = core.MapCell{Circuit: core.Latch, Group: 1}
	m.Cells[40] = core.MapCell{Circuit: core.Toggle, Group: 1}
	m.Cells[50] = core.MapCell{Circuit: core.Hold, Group: 1}
	m.Cells[30] = core.MapCell{Circuit: core.Door, Group: 1}
	m.Cells[31] = core.MapCell{Circuit: core.Door, Group: 1, InitiallyOpen: true}
	m.Edges = []core.MapEdge{{A: 10, B: 11, Kind: core.Gate, Group: 1, InitiallyOpen: true}}
	g, err := core.NewGameWithMap(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	rng := core.NewRNG(246)
	var moves []int
	for turn := 0; turn < 250 && !g.Over(); turn++ {
		moves = g.LegalMoves(moves[:0])
		if len(moves) == 0 {
			t.Fatal("no legal move")
		}
		cell := moves[rng.Intn(len(moves))]
		player := int8(g.Turn)
		waves, err := g.Apply(cell)
		if err != nil {
			t.Fatal(err)
		}
		v.Events = g.TerrainEvents
		v.Place(cell, player)
		for _, wave := range waves {
			v.Launch(wave)
			v.Land(wave)
		}
		if !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) || !slices.Equal(v.Geo.Cap, g.Geo.Cap) || !slices.Equal(v.Replay.Pressed, g.Pressed) || v.Replay.CircuitOn != g.CircuitOn {
			t.Fatalf("circuit playback differs at turn %d", turn)
		}
	}
}
