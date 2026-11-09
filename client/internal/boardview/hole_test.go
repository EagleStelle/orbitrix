package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestSpillwayReplayDropsItsExtraOrb(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Feature = core.Spillway
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.Owner[12], g.Count[12], g.Orbs[0] = 0, 4, 4
	v := NewView(g)
	revision := v.revision
	v.Count[12]++
	v.Launch(core.Wave{12})
	if v.revision == revision {
		t.Fatal("emptying a Spillway must return its ink to neutral")
	}
	b := New(g)
	waves, err := g.Apply(12)
	if err != nil || len(waves) != 1 {
		t.Fatalf("waves=%v err=%v", waves, err)
	}
	b.Start(12, 0, waves, g.ProducedAt, nil)
	b.falls = b.falls[:0]
	for b.Playing {
		b.Tick(nil)
	}
	if len(b.falls) != 1 || b.falls[0].hole != 12 || b.falls[0].owner != 0 {
		t.Fatalf("the extra orb must fall into the Spillway's own pit: %v", b.falls)
	}
	if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) {
		t.Fatal("Spillway replay differs from resolved game")
	}
}

func TestHoleReplayDropsWastedOrbs(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Terrain = core.Hole
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.Owner[7], g.Count[7], g.Orbs[0] = 0, 3, 3
	b := New(g)
	waves, err := g.Apply(7)
	if err != nil || len(waves) != 1 {
		t.Fatalf("waves=%v err=%v", waves, err)
	}
	b.State.Events = g.TerrainEvents
	b.Start(7, 0, waves, g.ProducedAt, nil)
	if got := b.dropIntoHoles(); got != sound.Fall || len(b.falls) != 1 {
		t.Fatal("an orb sent into a Hole must fall with its cue")
	}
	b.falls = b.falls[:0]
	for b.Playing {
		b.Tick(nil)
	}
	if len(b.falls) != 1 || b.falls[0].hole != 12 || b.falls[0].owner != 0 {
		t.Fatalf("falls %v", b.falls)
	}
	if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) {
		t.Fatal("Hole replay differs from resolved game")
	}
	for range fallTicks {
		b.Tick(nil)
	}
	if len(b.falls) != 0 {
		t.Fatal("wasted orbs must fade away")
	}
}

func TestHolesJoinOnlyHoles(t *testing.T) {
	m := core.NewMap(3, 3)
	m.Cells[1].Terrain, m.Cells[4].Terrain, m.Cells[7].Terrain = core.Hole, core.Hole, core.Wall
	state := MapState{Map: &m}
	if got := mapArtWallMask(&state, 4); got != mapWest {
		t.Fatalf("Hole mask %b: must join the Hole to its west only", got)
	}
	if got := mapArtWallMask(&state, 7); got != 0 {
		t.Fatalf("Wall mask %b: must not join a Hole", got)
	}
}
