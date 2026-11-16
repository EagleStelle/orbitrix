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

// A shot fired at a Landfill falls into it on the board, which then shows the fill the game holds.
func TestTurretShotFillsALandfillOnTheBoard(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Feature, m.Cells[11].Terrain = core.Turret, core.Landfill // up, where a Turret faces first
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 3
	s.Count[12], s.Owner[12] = 1, 0
	s.Count[24], s.Owner[24] = 1, 1
	if g, err = core.Restore(s); err != nil {
		t.Fatal(err)
	}
	b, fill, fell := New(g), g.Durability[11], false
	for _, cell := range []int{0, 24, 1, 24, 2} { // the Turret skips a move, fires up into the Landfill, then right
		mover := int8(g.Turn)
		waves, err := g.Apply(cell)
		if err != nil {
			t.Fatal(err)
		}
		b.State.Events = g.TerrainEvents
		b.Start(cell, mover, waves, g.ProducedAt, nil)
		for n := 0; (b.Playing || len(b.falls) > 0) && n < 1000; n++ {
			b.Tick(nil)
			fell = fell || len(b.falls) == 1 && b.falls[0].landfill && b.falls[0].hole == 11 && b.falls[0].owner == 0
		}
		if b.landfillDurability[11] != g.Durability[11] || !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Replay.Durability, g.Durability) {
			t.Fatalf("after %d: board differs from the game", cell)
		}
	}
	if !fell || g.Durability[11] != fill-1 {
		t.Fatalf("fell %v, Landfill needs %d of %d: one shot falls in", fell, g.Durability[11], fill)
	}
}
