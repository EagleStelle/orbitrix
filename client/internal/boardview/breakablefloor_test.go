package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestBreakableFloorReplayDropsItsOrbs(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Terrain = core.BreakableFloor
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.Durability[12], g.Played = 1, 3
	// Its own explosion breaks it, and the orb it keeps falls in.
	g.Owner[12], g.Count[12], g.Orbs[0] = 0, 4, 4
	g.Owner[24], g.Count[24], g.Orbs[1] = 1, 1, 1
	state := MapState{Map: g.Map, Durability: g.Durability}
	if mapArtTerrain(&state, 12) != core.BreakableFloor || mapArtJoin(core.BreakableFloor) != 0 {
		t.Fatal("an intact Breakable Floor must draw as a cracked floor tile")
	}
	v, b := NewView(g), New(g)
	waves, err := g.Apply(12)
	if err != nil || len(waves) != 1 || g.TerrainAt(12) != core.Void {
		t.Fatalf("waves=%v err=%v", waves, err)
	}
	v.Events = g.TerrainEvents
	v.Place(12, 0)
	v.Launch(waves[0])
	if got := v.terrainSounds(v.Land(waves[0])); got != sound.WallBreak {
		t.Fatal("a breaking floor must sound like a breaking wall")
	}
	b.State.Events = g.TerrainEvents
	b.Start(12, 0, waves, g.ProducedAt, nil)
	for b.Playing {
		b.Tick(nil)
	}
	if !slices.ContainsFunc(b.falls, func(o fallingOrb) bool { return o.hole == 12 && o.owner == 0 && !o.landfill }) {
		t.Fatalf("the broken floor's orbs must fall into its Void: %v", b.falls)
	}
	for _, replay := range []*view{v, b.State} {
		if !slices.Equal(replay.Count, g.Count) || !slices.Equal(replay.Owner, g.Owner) || !slices.Equal(replay.Geo.Cap, g.Geo.Cap) {
			t.Fatal("Breakable Floor replay differs from the resolved game")
		}
	}
	state.Durability = b.State.Replay.Durability
	if mapArtTerrain(&state, 12) != core.Void {
		t.Fatal("a broken floor must draw as Void")
	}
}
