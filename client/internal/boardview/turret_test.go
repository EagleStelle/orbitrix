package boardview

import (
	"slices"
	"testing"

	"orbitrix/core"
)

// TestTurretPlaybackMatchesGame plays random games on a map with Turrets and Vaults and checks that the board's
// playback, shots and turning barrels included, ends exactly on the game's state.
func TestTurretPlaybackMatchesGame(t *testing.T) {
	m := core.NewMap(5, 5)
	for _, i := range []int{6, 12, 18} {
		m.Cells[i].Feature = core.Turret
	}
	m.Cells[8].Feature, m.Cells[16].Feature = core.Vault, core.Vault
	orbs := func(g *core.Game) (n int) {
		for _, c := range g.Count {
			n += int(c)
		}
		return
	}
	fired := false
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
			cell, mover, before := moves[rng.Intn(len(moves))], int8(g.Turn), orbs(g)
			waves, err := g.Apply(cell)
			if err != nil {
				t.Fatal(err)
			}
			fired = fired || orbs(g) > before+1 // only a shot adds to the placed orb here
			b.State.Events = g.TerrainEvents
			b.Start(cell, mover, waves, g.ProducedAt, nil)
			for b.Playing {
				b.Tick(nil)
			}
			if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) || !slices.Equal(b.State.Replay.Pressed, g.Pressed) || !slices.Equal(b.State.Replay.Durability, g.Durability) {
				t.Fatalf("seed %d turn %d: playback differs from the resolved game", seed, turn)
			}
		}
	}
	if !fired {
		t.Fatal("random play never fired a Turret")
	}
}
