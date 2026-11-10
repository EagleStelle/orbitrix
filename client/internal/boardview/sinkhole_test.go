package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestSinkholeReplayDropsItsOrbsIntoTheHole(t *testing.T) {
	for _, placed := range []bool{true, false} {
		m := core.NewMap(5, 5)
		m.Cells[12].Terrain, m.Cells[13].Terrain = core.Sinkhole, core.Hole
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.Played = 3
		g.Owner[24], g.Count[24], g.Orbs[1] = 1, 1, 1
		g.Owner[12], g.Count[12] = 0, 3 // its capacity is 4, a route into the Hole included
		g.Owner[7], g.Count[7] = 0, 3
		g.Orbs[0] = 6
		state := MapState{Map: g.Map, Durability: g.Durability}
		if mapArtTerrain(&state, 12) != core.Sinkhole || mapArtJoin(core.Sinkhole) != 0 {
			t.Fatal("a standing Sinkhole must draw as a floor tile")
		}
		// Placing the fourth orb collapses it at once; an orb from 7 does it after its wave.
		cell := 7
		if placed {
			cell = 12
		}
		v, b := NewView(g), New(g)
		waves, err := g.Apply(cell)
		if err != nil || g.TerrainAt(12) != core.Hole {
			t.Fatalf("placed=%v: waves=%v err=%v", placed, waves, err)
		}
		v.Events = g.TerrainEvents
		cue := v.terrainSounds(v.Place(cell, 0))
		for _, w := range waves {
			v.Launch(w)
			cue |= v.terrainSounds(v.Land(w))
		}
		if cue&sound.Fall == 0 || cue&(sound.WallBreak|sound.WallHit) != 0 {
			t.Fatalf("placed=%v: a collapsing Sinkhole must sound like a fall into a Hole, got %v", placed, cue)
		}
		b.State.Events = g.TerrainEvents
		b.Start(cell, 0, waves, g.ProducedAt, nil)
		if placed && !slices.ContainsFunc(b.falls, func(o fallingOrb) bool { return o.hole == 12 && o.owner == 0 && !o.landfill }) {
			t.Fatalf("the placement's collapse must drop its orbs before the first wave: %v", b.falls)
		}
		for b.Playing {
			b.Tick(nil)
		}
		if !placed && !slices.ContainsFunc(b.falls, func(o fallingOrb) bool { return o.hole == 12 && o.owner == 0 && !o.landfill }) {
			t.Fatalf("the filled Sinkhole's orbs must fall into it: %v", b.falls)
		}
		for _, replay := range []*view{v, b.State} {
			if !slices.Equal(replay.Count, g.Count) || !slices.Equal(replay.Owner, g.Owner) || !slices.Equal(replay.Geo.Cap, g.Geo.Cap) || !slices.Equal(replay.Replay.Durability, g.Durability) {
				t.Fatalf("placed=%v: Sinkhole replay differs from the resolved game", placed)
			}
		}
		state.Durability = b.State.Replay.Durability
		if mapArtTerrain(&state, 12) != core.Hole || mapArtWallMask(&state, 12)&mapSouth == 0 {
			t.Fatal("a collapsed Sinkhole must draw as a Hole joined to the Hole beside it")
		}
	}
}
