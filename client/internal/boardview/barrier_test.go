package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestBreakableBarrierReplayAndSounds(t *testing.T) {
	for _, durability := range []uint8{1, 2} {
		m := core.NewMap(7, 7)
		m.Edges = []core.MapEdge{{A: 24, B: 25, Kind: core.BreakableBarrier}}
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.EdgeDurability[0] = durability
		g.Owner[24], g.Count[24] = 0, 2
		wantSound, wantWaves := sound.WallHit, 1
		if durability == 1 {
			// The remaining four orbs explode again across the newly opened edge.
			g.Count[24] = 6
			wantSound, wantWaves = sound.WallBreak, 2
		}
		g.Orbs[0] = int32(g.Count[24])
		v := NewView(g)
		waves, err := g.Apply(24)
		if err != nil || len(waves) != wantWaves {
			t.Fatalf("waves=%v err=%v", waves, err)
		}
		v.Events = g.TerrainEvents
		if got := v.terrainSounds(v.Place(24, 0)); got != 0 {
			t.Fatal("barrier sounded before the wave landed")
		}
		v.Launch(waves[0])
		if v.Replay.EdgeDurability[0] != durability || v.Geo.Cap[24] != 3 {
			t.Fatal("barrier changed before the wave landed")
		}
		if got := v.terrainSounds(v.Land(waves[0])); got != wantSound || v.Count[25] != 0 {
			t.Fatal("destroying wave must sound on landing without crossing the barrier")
		}
		for _, w := range waves[1:] {
			v.Launch(w)
			if got := v.terrainSounds(v.Land(w)); got != 0 {
				t.Fatal("destroyed barrier sounded again")
			}
		}
		if durability == 1 && v.Count[25] != 1 {
			t.Fatal("next wave did not cross the destroyed barrier")
		}
		if len(v.Events) != 0 || !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) || !slices.Equal(v.Geo.Cap, g.Geo.Cap) || !slices.Equal(v.Replay.EdgeDurability, g.EdgeDurability) {
			t.Fatal("barrier replay differs from resolved game")
		}
	}
}
