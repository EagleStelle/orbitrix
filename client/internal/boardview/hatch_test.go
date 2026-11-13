package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestHatchPlaybackDropsOrbsAndSettlesBoxes(t *testing.T) {
	for _, box := range []bool{false, true} {
		for _, placement := range []bool{false, true} {
			m := core.NewMap(7, 7)
			m.Cells[8] = core.MapCell{Circuit: core.Toggle, Group: 1}
			m.Cells[24] = core.MapCell{Circuit: core.Hatch, Group: 1}
			if box {
				m.Cells[40].Feature = core.Box
			}
			g, err := core.NewGameWithMap(m, 2)
			if err != nil {
				t.Fatal(err)
			}
			g.Owner[48], g.Count[48], g.Orbs[1] = 1, 1, 1
			if box {
				g.Boxes[40], g.Boxes[24] = 0, core.BoxHere
			} else {
				g.Owner[24], g.Count[24], g.Orbs[1] = 1, 3, 4
			}
			cell := 8
			if !placement {
				cell = 7
				g.Owner[7], g.Count[7], g.Orbs[0] = 0, 2, 2
			}
			b := New(g)
			waves, err := g.Apply(cell)
			if err != nil {
				t.Fatal(err)
			}
			b.State.Events = g.TerrainEvents
			b.Start(cell, 0, waves, g.ProducedAt, nil)
			if placement && box && (!b.Playing || !b.settling || b.Wave != -1) {
				t.Fatal("the placement must hold playback until its Box has fallen")
			}
			dropped := slices.ContainsFunc(b.falls, func(o fallingOrb) bool { return o.hole == 24 && o.box == box })
			for tick := 0; tick < 200 && b.Playing; tick++ {
				b.Tick(nil)
				dropped = dropped || slices.ContainsFunc(b.falls, func(o fallingOrb) bool { return o.hole == 24 && o.box == box })
			}
			if b.Playing || !dropped || !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) ||
				!slices.Equal(b.State.Replay.Boxes, g.Boxes) || !slices.Equal(b.State.Geo.Cap, g.Geo.Cap) || b.State.Replay.CircuitOn != g.CircuitOn {
				t.Fatalf("box=%v placement=%v: playback differs, dropped=%v", box, placement, dropped)
			}
			if box && b.shownBoxes[24] != core.CrateFloor {
				t.Fatal("a Box must show its permanent crate Floor only after landing")
			}
		}
	}
}

func TestHatchDropSoundsLikeAHole(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[6] = core.MapCell{Circuit: core.Latch, Group: 1}
	m.Cells[12] = core.MapCell{Circuit: core.Hatch, Group: 1}
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	effects := v.terrainSounds([]core.TerrainEvent{{Cell: 12}})
	if effects != sound.Fall {
		t.Fatalf("a Hatch's dropped orbs must reuse the Hole cue: %v", effects)
	}
}

func TestHatchArtUsesHoleTerrainAndJoins(t *testing.T) {
	m := core.NewMap(3, 3)
	m.Cells[4] = core.MapCell{Circuit: core.Hatch, Group: 1}
	m.Cells[5].Terrain = core.Hole
	state := MapState{Map: &m}
	if mapArtTerrain(&state, 4) != core.Floor {
		t.Fatal("a closed Hatch must have Floor under its trapdoor")
	}
	state.CircuitOn[1] = true
	if mapArtTerrain(&state, 4) != core.Hole || mapArtWallMask(&state, 4)&mapSouth == 0 {
		t.Fatal("an open Hatch must use Hole terrain and its neighbor joins")
	}
	state.Boxes = make([]uint8, 9)
	state.Boxes[4] = core.CrateFloor
	if mapArtTerrain(&state, 4) != core.Floor {
		t.Fatal("a Box must permanently replace Hatch terrain")
	}
}
