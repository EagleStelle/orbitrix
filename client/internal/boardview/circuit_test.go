package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/ui"
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

func TestTrappedOrbsFreezeAndResume(t *testing.T) {
	geo, err := core.NewGeometry(3, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{1, 2, 3, 4, 8, 12} {
		for _, initiallyLocked := range []bool{false, true} {
			f := newOrbField(geo)
			capacity := max(2, count+1)
			if initiallyLocked {
				capacity = 0
			}
			before := slices.Clone(f.update(0, count, capacity, 1000, 60))
			for _, tms := range []float64{1017, 2000, 100000} {
				if got := f.update(0, count, 0, tms, 60); !slices.Equal(got, before) {
					t.Fatalf("count %d, initially locked %v: trapped orbs moved at %v ms", count, initiallyLocked, tms)
				}
			}
			if got := f.update(0, count, max(2, count+1), 100017, 60); slices.Equal(got, before) {
				t.Fatalf("count %d: reopened orbs did not resume motion", count)
			}
			if n := testing.AllocsPerRun(100, func() { f.update(0, count, 0, 100034, 60) }); n != 0 {
				t.Fatalf("frozen motion allocates %v times", n)
			}
		}
	}
}

func TestTrappedOrbsUseFloorInkWithoutMarks(t *testing.T) {
	for mode := range ui.Palettes {
		for _, marks := range []bool{false, true} {
			b := &Board{Palette: &ui.Palettes[mode], marks: marks, OrbSize: 32, OrbDrawSize: 32}
			b.Orb(20, 20, int(core.NoOwner))
			if len(b.Verts) != 8 {
				t.Fatalf("mode %d, marks %v: trapped orb included a player mark", mode, marks)
			}
			for _, vertex := range b.Verts[4:] {
				ink := b.Palette.Tile
				if vertex.ColorR != float32(ink.R)/255 || vertex.ColorG != float32(ink.G)/255 || vertex.ColorB != float32(ink.B)/255 {
					t.Fatalf("mode %d: trapped orb retained player ink", mode)
				}
			}
			b.Verts, b.idx = b.Verts[:0], b.idx[:0]
			b.Orb(20, 20, 0)
			want := 8
			if marks {
				want = 12
			}
			if len(b.Verts) != want || b.Verts[4].ColorR != float32(b.Palette.Players[0].R)/255 {
				t.Fatalf("mode %d, marks %v: ordinary orb lost its player styling", mode, marks)
			}
		}
	}
}
