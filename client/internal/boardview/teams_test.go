package boardview

import (
	"slices"
	"testing"

	"orbitrix/core"
)

// TestTeamPlaybackMatchesGame plays random team games on a classic board and on rolled maps, checking that the
// board's playback, with each orb in its own colour, ends exactly on the game's state.
func TestTeamPlaybackMatchesGame(t *testing.T) {
	shared := false
	for seed := uint64(1); seed <= 24; seed++ {
		g, err := core.NewGame(6, 7, 5)
		if seed%3 != 0 {
			g, err = core.NewGameWithMap(core.RandomMap(8, 9, seed), 5)
		}
		if err == nil {
			err = g.SetTeams([]uint8{0, 1, 0, 1, 2})
		}
		if err != nil {
			t.Fatal(err)
		}
		b := New(g)
		rng := core.NewRNG(seed)
		var moves []int
		for turn := 0; turn < 300 && !g.Over(); turn++ {
			if moves = g.LegalMoves(moves[:0]); len(moves) == 0 {
				if err := g.Pass(); err != nil {
					t.Fatal(err)
				}
				continue
			}
			cell, mover := moves[rng.Intn(len(moves))], int8(g.Turn)
			waves, err := g.Apply(cell)
			if err != nil {
				t.Fatal(err)
			}
			b.State.Events = g.TerrainEvents
			b.Start(cell, mover, waves, g.ProducedAt, nil)
			for b.Playing {
				b.Tick(nil)
			}
			if !slices.Equal(b.State.Mix, g.Mix) || !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) {
				t.Fatalf("seed %d turn %d: playback differs from the resolved game", seed, turn)
			}
			if g.Map != nil && (!slices.Equal(b.State.Replay.Pressed, g.Pressed) || !slices.Equal(b.State.Replay.Boxes, g.Boxes)) {
				t.Fatalf("seed %d turn %d: playback's map state differs from the resolved game", seed, turn)
			}
			for i, o := range g.Owner {
				shared = shared || o != core.NoOwner && g.Mix[i*g.Players+int(o)] != g.Count[i]
			}
		}
	}
	if !shared {
		t.Fatal("random play never put two colours in one cell")
	}
}
