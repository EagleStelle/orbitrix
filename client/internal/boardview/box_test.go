package boardview

import (
	"slices"
	"testing"

	"orbitrix/core"
)

// TestBoxPlaybackMatchesGame plays random games among Boxes, Holes, a Landfill, a Spring and a Hold, checking that
// the board's playback, with its held, returned and shoved orbs, ends exactly on the game's state.
func TestBoxPlaybackMatchesGame(t *testing.T) {
	m := core.NewMap(7, 7)
	for _, i := range []int{16, 24, 32, 18, 30} {
		m.Cells[i].Feature = core.Box
	}
	m.Cells[23].Terrain, m.Cells[38].Terrain = core.Hole, core.Landfill
	m.Cells[0].Terrain, m.Cells[1].Feature = core.Wall, core.Spring
	m.Cells[26].Circuit, m.Cells[26].Group = core.Hold, 1
	m.Edges = []core.MapEdge{{A: 40, B: 41, Kind: core.Gate, Group: 1}}
	moved, filled, slid := false, false, false
	for seed := uint64(1); seed <= 8; seed++ {
		g, err := core.NewGameWithMap(m, 2+int(seed%3))
		if err != nil {
			t.Fatal(err)
		}
		b := New(g)
		start := slices.Clone(g.Boxes)
		rng := core.NewRNG(seed)
		var moves []int
		for turn := 0; turn < 200 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			cell, mover := moves[rng.Intn(len(moves))], int8(g.Turn)
			waves, err := g.Apply(cell)
			if err != nil {
				t.Fatal(err)
			}
			b.State.Events = g.TerrainEvents
			b.Start(cell, mover, waves, g.ProducedAt, nil)
			b.falls = b.falls[:0]
			for b.Playing {
				slid = slid || len(b.slides) != 0
				b.Tick(nil)
			}
			if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) || !slices.Equal(b.State.Replay.Boxes, g.Boxes) ||
				!slices.Equal(b.State.Replay.Durability, g.Durability) || !slices.Equal(b.State.Replay.Pressed, g.Pressed) {
				t.Fatalf("seed %d turn %d: playback differs from the resolved game", seed, turn)
			}
			for i, v := range g.Boxes {
				if b.shownBoxes[i]&core.CrateFloor != v&core.CrateFloor {
					t.Fatalf("seed %d turn %d: pit %d still drawn open after its Box sank", seed, turn, i)
				}
			}
			moved = moved || !slices.Equal(g.Boxes, start)
			filled = filled || slices.Contains(g.Boxes, core.CrateFloor)
		}
	}
	if !moved || !filled || !slid {
		t.Fatalf("random play never moved a Box (%v), filled a pit (%v) or animated a slide (%v)", moved, filled, slid)
	}
}
