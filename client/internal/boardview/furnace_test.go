package boardview

import (
	"slices"
	"testing"

	"orbitrix/core"
)

// TestFurnacePlaybackMatchesGame plays random games on a map with Furnaces, Crushers and Vaults and checks that the
// board's playback, including the step where the mover's makers run and shield hits, ends exactly on the game's state.
func TestFurnacePlaybackMatchesGame(t *testing.T) {
	m := core.NewMap(5, 5)
	for _, i := range []int{2, 12, 18} {
		m.Cells[i].Feature = core.Furnace
	}
	m.Cells[22].Feature, m.Cells[6].Feature = core.Crusher, core.Crusher
	m.Cells[8].Feature, m.Cells[16].Feature = core.Vault, core.Vault
	produced, crushed, shielded := false, false, false
	for seed := uint64(1); seed <= 10; seed++ {
		g, err := core.NewGameWithMap(m, 3)
		if err != nil {
			t.Fatal(err)
		}
		b := New(g)
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
				b.Tick(nil)
			}
			if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) || !slices.Equal(b.State.Replay.Pressed, g.Pressed) {
				t.Fatalf("seed %d turn %d: playback differs from the resolved game", seed, turn)
			}
			produced = produced || g.ProducedAt >= 0
			for _, f := range b.falls {
				crushed = crushed || m.Cells[f.hole].Feature == core.Crusher
				shielded = shielded || f.shield
			}
		}
	}
	if !produced || !crushed || !shielded {
		t.Fatal("random play never ran a Furnace, crushed an orb or hit a shield")
	}
}

func TestCrusherEmptiesToNeutralInk(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[22].Feature, m.Cells[12].Feature = core.Crusher, core.Furnace
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	v.Owner[22], v.Count[22] = 0, 1
	v.Owner[12], v.Count[12] = 0, 1
	if made, crushed := v.Produce(0, nil); made || len(crushed) != 0 || v.Count[22] != 1 || v.Count[12] != 1 {
		t.Fatal("Furnaces and Crushers must skip their first turn")
	}
	v.Owner[12], v.Count[12] = core.NoOwner, 0 // leave only the Crusher
	revision := v.revision
	if made, crushed := v.Produce(0, nil); made || !slices.Equal(crushed, []int16{22}) {
		t.Fatalf("made %v crushed %v", made, crushed)
	}
	v.settle()
	if v.Count[22] != 0 || v.Owner[22] != core.NoOwner || v.Replay.Pressed[22] != 0 || v.revision == revision {
		t.Fatal("an emptied Crusher must turn neutral, start over and redraw its ink")
	}
	if _, crushed := v.Produce(0, nil); len(crushed) != 0 {
		t.Fatal("a neutral Crusher takes nothing")
	}
}
