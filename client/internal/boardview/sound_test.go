package boardview

import (
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestPlacementSwitchSoundsFollowRecordedChanges(t *testing.T) {
	for _, tc := range []struct {
		feature core.Feature
		want    sound.Effects
	}{{core.Switch, sound.Latch}, {core.ToggleSwitch, sound.ToggleOn}, {core.HoldSwitch, sound.HoldOn}} {
		m := core.NewMap(7, 7)
		m.Cells[24].Feature, m.Cells[24].Group = tc.feature, 1
		m.Edges = []core.MapEdge{{A: 8, B: 9, Kind: core.Gate, Group: 1}}
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		v := NewView(g)
		for range 2 {
			g.Turn = 0
			if _, err := g.Apply(24); err != nil {
				t.Fatal(err)
			}
			v.Events = g.TerrainEvents
			if got := v.terrainSounds(v.Place(24, 0)); got != tc.want {
				t.Fatalf("feature %v: placement cue %v, want %v", tc.feature, got, tc.want)
			}
			if v.Replay.Pressed[24] != g.Pressed[24] || v.Replay.OpenGates[1] != g.OpenGates[1] {
				t.Fatalf("feature %v: placement replay differs from game", tc.feature)
			}
			tc.want = 0 // Another arrival on an already-pressed switch makes no new transition.
		}
	}
}

func TestTerrainSoundVariantsAndCoalescing(t *testing.T) {
	m := core.NewMap(7, 7)
	m.Cells[16].Feature, m.Cells[16].Group = core.ToggleSwitch, 1
	m.Cells[32].Feature, m.Cells[32].Group = core.HoldSwitch, 2
	m.Cells[24].Terrain = core.BreakableWall
	m.Edges = []core.MapEdge{{A: 8, B: 9, Kind: core.Gate, Group: 1}, {A: 40, B: 41, Kind: core.Gate, Group: 2}}
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	for _, tc := range []struct {
		events []core.TerrainEvent
		want   sound.Effects
	}{
		{[]core.TerrainEvent{{Cell: 16, Switch: true, Pressed: true}, {Cell: -1, Open: true}}, sound.ToggleOn},
		{[]core.TerrainEvent{{Cell: 16, Switch: true}}, sound.ToggleOff},
		{[]core.TerrainEvent{{Cell: 32, Switch: true, Pressed: true}}, sound.HoldOn},
		{[]core.TerrainEvent{{Cell: 32, Switch: true}}, sound.HoldOff},
		{[]core.TerrainEvent{{Cell: 24, Durability: 1}, {Cell: 24, Durability: 1}}, sound.WallHit},
		{[]core.TerrainEvent{{Cell: 24, Durability: 0}}, sound.WallBreak},
		{[]core.TerrainEvent{{Cell: -1}}, 0},
	} {
		if got := v.terrainSounds(tc.events); got != tc.want {
			t.Fatalf("events %+v: cue %v, want %v", tc.events, got, tc.want)
		}
	}
}

func TestWallSoundsWaitForWaveLanding(t *testing.T) {
	for _, durability := range []uint8{1, 2} {
		m := core.NewMap(7, 7)
		m.Cells[24].Terrain = core.BreakableWall
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.Durability[24] = durability
		g.Owner[17], g.Count[17] = 0, g.Geo.Cap[17]-1
		g.Orbs[0] = int32(g.Count[17])
		v := NewView(g)
		waves, err := g.Apply(17)
		if err != nil || len(waves) != 1 {
			t.Fatalf("waves %v: %v", waves, err)
		}
		v.Events = g.TerrainEvents
		if got := v.terrainSounds(v.Place(17, 0)); got != 0 {
			t.Fatal("wall sounded before the transfer")
		}
		v.Launch(waves[0])
		if v.Replay.Durability[24] != durability {
			t.Fatal("wall changed before landing")
		}
		want := sound.WallHit
		if durability == 1 {
			want = sound.WallBreak
		}
		if got := v.terrainSounds(v.Land(waves[0])); got != want {
			t.Fatalf("final-wave wall cue %v, want %v", got, want)
		}
		if len(v.Events) != 0 || !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) {
			t.Fatal("sound integration changed replay results or left events unconsumed")
		}
	}
}

func TestLinkSoundsOnlyForTransfersAcrossPair(t *testing.T) {
	m := core.NewMap(7, 7)
	m.Cells[16].Feature, m.Cells[16].Group = core.Link, 1
	m.Cells[32].Feature, m.Cells[32].Group = core.Link, 1
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	b := &Board{State: NewView(g), Playing: true, Waves: []core.Wave{{17}, {16}, {16, 32}}}
	for wave, want := range []sound.Effects{0, sound.Link, sound.Link} {
		b.Wave = wave
		if got := b.linkSound(); got != want {
			t.Fatalf("wave %d: Link cue %v, want %v", wave, got, want)
		}
	}
	b.Playing = false
	if b.linkSound() != 0 {
		t.Fatal("an idle Link sounded")
	}
	g.Map = nil
	b.State, b.Playing = NewView(g), true
	if b.linkSound() != 0 {
		t.Fatal("a classic board produced a map cue")
	}
}
