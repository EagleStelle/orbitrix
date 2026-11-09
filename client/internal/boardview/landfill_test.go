package boardview

import (
	"math"
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

func TestLandfillReplayFillsOnLanding(t *testing.T) {
	for _, durability := range []uint8{2, 1} {
		m := core.NewMap(5, 5)
		m.Cells[12].Terrain = core.Landfill
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.Durability[12] = durability
		g.Count[7], g.Owner[7], g.Orbs[0] = 3, 0, 3
		b := New(g)
		waves, err := g.Apply(7)
		if err != nil || len(waves) != 1 {
			t.Fatalf("waves=%v err=%v", waves, err)
		}
		b.State.Events = g.TerrainEvents
		b.Start(7, 0, waves, g.ProducedAt, nil)
		if b.State.Replay.Durability[12] != durability || b.State.Replay.TerrainAt(12) != core.Landfill {
			t.Fatal("the Landfill filled before the arriving wave landed")
		}
		if effects := b.dropIntoHoles(); effects != sound.Fall || len(b.falls) != 1 || !b.falls[0].landfill {
			t.Fatal("an orb filling a Landfill must fall toward its bottom")
		}
		b.falls = b.falls[:0]
		if effects := b.State.terrainSounds(g.TerrainEvents); effects != sound.Fall {
			t.Fatal("filling must not play a wall destruction cue")
		}
		for b.Playing && !b.settling {
			b.Tick(nil)
		}
		if b.landfillDurability[12] != durability {
			t.Fatal("the opening filled before the orb reached its bottom")
		}
		for range fallTicks {
			b.Tick(nil)
		}
		if durability == 1 && (b.landfillDurability[12] != 1 || !b.settling) {
			t.Fatal("the filling orb must rest on the bottom a moment before Floor appears")
		}
		for range landfillHold {
			b.Tick(nil)
		}
		if b.landfillDurability[12] != g.Durability[12] || len(b.falls) != 0 {
			t.Fatal("the Landfill did not settle after its fall")
		}
		if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) || !slices.Equal(b.State.Geo.Cap, g.Geo.Cap) || !slices.Equal(b.State.Replay.Durability, g.Durability) {
			t.Fatal("Landfill replay differs from the resolved game")
		}
		state := MapState{Map: &m, Durability: b.State.Replay.Durability}
		if got := mapArtTerrain(&state, 12); got != g.TerrainAt(12) || got == core.Landfill && mapArtJoin(got) == mapJoinHole {
			t.Fatal("Landfill art must follow fill state without merging with neighboring Holes")
		}
	}
}

func TestCompletingLandfillSettlesBeforeNextWaveOrTurn(t *testing.T) {
	for _, skip := range []bool{false, true} {
		for _, followingWave := range []bool{false, true} {
			m := core.NewMap(5, 5)
			m.Cells[12].Terrain = core.Landfill
			g, err := core.NewGameWithMap(m, 2)
			if err != nil {
				t.Fatal(err)
			}
			g.Durability[12] = 1
			g.Count[7], g.Owner[7] = 3, 0
			g.Count[24], g.Owner[24] = 1, 1
			if followingWave {
				g.Count[6], g.Owner[6], g.Count[11], g.Owner[11] = 4, 0, 3, 0
			}
			g, err = core.Restore(g.Snapshot())
			if err != nil {
				t.Fatal(err)
			}
			b := New(g)
			waves, err := g.Apply(7)
			if err != nil {
				t.Fatal(err)
			}
			b.State.Events = g.TerrainEvents
			b.Start(7, 0, waves, g.ProducedAt, nil)
			for !b.settling && b.Playing {
				b.Tick(nil)
			}
			waitingTicks := fallTicks + landfillHold - 1
			if skip {
				waitingTicks = 3
			}
			for range waitingTicks {
				if !b.Playing || b.State.Count[12] != 0 || b.landfillDurability[12] != 1 {
					t.Fatal("a following wave or next turn was released before the opening closed")
				}
				b.Tick(nil)
			}
			if skip {
				b.FastForward()
				b.Tick(nil)
				if b.landfillDurability[12] != 0 || b.settling {
					t.Fatal("fast-forward did not end the wait for the bottom")
				}
			}
			for ticks := 0; b.Playing && ticks < 100; ticks++ {
				b.Tick(nil)
			}
			if b.Playing || b.landfillDurability[12] != 0 || !slices.Equal(b.State.Count, g.Count) {
				t.Fatalf("skip=%v followingWave=%v: settling did not resume playback to the resolved board", skip, followingWave)
			}
		}
	}
}

func TestLandfillFullCueWaitsForBottom(t *testing.T) {
	for _, remaining := range []uint8{1, 0} {
		b := &Board{State: &view{}, landfillDurability: []uint8{2, 2}}
		// Two arrivals filling one opening and another opening settling alongside it.
		b.falls = []fallingOrb{
			{hole: 0, landfill: true, remaining: remaining, start: 10},
			{hole: 0, landfill: true, remaining: remaining, start: 10},
			{hole: 1, landfill: true, remaining: remaining, start: 10},
		}
		b.Clock = 10 + b.falls[0].ticks() - 1
		if effects := b.settleFalls(); effects != 0 || b.landfillDurability[0] != 2 || len(b.falls) != 3 {
			t.Fatal("the fill state or cue changed before the fall finished")
		}
		b.Clock++
		want := sound.Effects(0)
		if remaining == 0 {
			want = sound.LandfillFull
		}
		if effects := b.settleFalls(); effects != want || b.landfillDurability[0] != remaining || b.landfillDurability[1] != remaining || len(b.falls) != 0 || b.State.revision != 2 {
			t.Fatal("each settled opening must change once, with one coalesced cue only for completed Floor")
		}
		if effects := b.settleFalls(); effects != 0 {
			t.Fatal("the completed Floor sounded again")
		}
	}
}

func TestLandfillFallShrinksAndSettlesOnBottom(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Terrain = core.Landfill
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	b := New(g)
	b.Palette, b.base, b.k, b.OrbSize, b.OrbDrawSize = &ui.Palettes[ui.Dark], 80, 1, 40, 40
	o := fallingOrb{hole: 12, owner: 0, landfill: true}
	x, y := b.Centre(12)
	centre := func() (float64, float64) {
		return float64(b.Verts[4].DstX+b.Verts[7].DstX) / 2, float64(b.Verts[4].DstY+b.Verts[6].DstY) / 2
	}
	const tms = 1234
	previous := float32(1000)
	for _, clock := range []float64{0, fallTicks / 2, fallTicks} {
		b.Clock = clock
		b.Verts, b.idx = b.Verts[:0], b.idx[:0]
		b.queueFall(o, tms)
		cx, cy := centre()
		if d := math.Hypot(cx-x, cy-y); clock == 0 && d > .001 || d > 1.2*landfillDrift*80+.001 {
			t.Fatal("the fall must start at the centre and stay on the bottom")
		}
		width := b.Verts[5].DstX - b.Verts[4].DstX
		if width >= previous {
			t.Fatal("the falling orb did not shrink")
		}
		previous = width
	}
	if math.Abs(float64(previous)-40*landfillOrbScale) > .001 {
		t.Fatal("the orb did not settle at the smaller bottom position")
	}
	ink := landfillOrbInk(b.Palette)
	if v := b.Verts[4]; v.ColorR != float32(ink.R)/255 || v.ColorG != float32(ink.G)/255 || v.ColorB != float32(ink.B)/255 || v.ColorA != 1 {
		t.Fatal("the settled orb lost its neutral fill or faded away")
	}
	// Landing hands over to the resting orb without a jump, and the resting orb keeps moving.
	fx, fy := centre()
	b.Verts, b.idx = b.Verts[:0], b.idx[:0]
	b.landfillOrb(12, core.NoOwner, 1, tms, 0)
	if rx, ry := centre(); math.Abs(rx-fx) > .001 || math.Abs(ry-fy) > .001 {
		t.Fatal("the orb jumped when it came to rest")
	}
	b.Verts, b.idx = b.Verts[:0], b.idx[:0]
	b.landfillOrb(12, core.NoOwner, 1, tms+1000, 0)
	if rx, ry := centre(); math.Hypot(rx-fx, ry-fy) < .5 {
		t.Fatal("the resting orb stopped moving")
	}
	b.Verts, b.idx = b.Verts[:0], b.idx[:0]
	b.landfillOrb(12, core.NoOwner, 1, tms, 1)
	if rx, ry := centre(); math.Hypot(rx-fx, ry-fy) < landfillDrift*80 {
		t.Fatal("the filling orb landed on top of the resting one")
	}
}

func TestPitBandsOnlyDarkenInward(t *testing.T) {
	for theme := range ui.Palettes {
		sc := &ui.Palettes[theme]
		for _, steps := range [][]pitStep{holeSteps, landfillSteps} {
			for k := 1; k < len(steps); k++ {
				if steps[k].scale >= steps[k-1].scale || ui.Luminance(steps[k].ink(sc)) >= ui.Luminance(steps[k-1].ink(sc)) {
					t.Fatalf("theme %d band %d: a lighter or larger inner band reads as a raised rim", theme, k)
				}
			}
		}
		if len(landfillSteps) >= len(holeSteps) || holeSteps[len(holeSteps)-1].ink(sc) != sc.Pit {
			t.Fatal("a Hole must step further down, to Pit, than a Landfill")
		}
		for k, s := range landfillSteps {
			if s.scale != holeSteps[k].scale || ui.Luminance(s.ink(sc)) <= ui.Luminance(holeSteps[k].ink(sc)) {
				t.Fatalf("theme %d band %d: a Landfill keeps the Hole's bands, lighter", theme, k)
			}
		}
	}
}

func TestLandfillJoinsFollowFillState(t *testing.T) {
	m := core.NewMap(3, 3)
	m.Cells[1].Terrain, m.Cells[4].Terrain, m.Cells[7].Terrain = core.Landfill, core.Landfill, core.Hole
	state := MapState{Map: &m}
	if got := mapArtWallMask(&state, 4); got != mapWest {
		t.Fatalf("Landfill must join its neighboring Landfill only: %04b", got)
	}
	state.Durability = make([]uint8, 9)
	state.Durability[4] = 1
	if got := mapArtWallMask(&state, 4); got != 0 {
		t.Fatalf("a filled Landfill must stop joining the remaining opening: %04b", got)
	}
	m.Cells[3].Terrain, m.Cells[0].Terrain = core.Landfill, core.Landfill
	state.Durability[0], state.Durability[1], state.Durability[3] = 2, 1, 2
	if _, count := mapArtWallVertex(&state, 1, 1, mapJoinLandfill); count != 4 {
		t.Fatal("Landfill joins left a gap at a four-tile corner")
	}
	state.Durability[4] = 0
	if _, count := mapArtWallVertex(&state, 1, 1, mapJoinLandfill); count != 3 {
		t.Fatal("filling one corner did not expose the connected L shape")
	}
}

func TestPortalIntoLandfillKeepsItsFlightAndChime(t *testing.T) {
	m := core.NewMap(7, 7)
	m.Cells[16], m.Cells[32] = core.MapCell{Feature: core.Portal, Group: 1}, core.MapCell{Feature: core.Portal, Group: 1}
	m.Cells[39].Terrain = core.Landfill
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.Count[9], g.Owner[9], g.Orbs[0] = 3, 0, 3
	b := New(g)
	b.base, b.k = 80, 1
	waves, err := g.Apply(9)
	if err != nil {
		t.Fatal(err)
	}
	b.State.Events = g.TerrainEvents
	b.Start(9, 0, waves, g.ProducedAt, nil)
	if b.linkSound() != sound.Link {
		t.Fatal("a Portal flight into a Landfill lost its chime")
	}
	x, y := b.flightPosition(9, len(b.State.Geo.Targets(9))-1, .5)
	ex, ey := b.Centre(32)
	if x != ex || y != ey {
		t.Fatal("the incoming orb bypassed its Portal exit")
	}
}

func TestLandfillRandomReplayConservation(t *testing.T) {
	m := core.NewMap(7, 7)
	for _, cell := range []int{17, 24, 31} {
		m.Cells[cell].Terrain = core.Landfill
	}
	g, err := core.NewGameWithMap(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	rng := core.NewRNG(789)
	var legal []int
	lost := 0
	for turn := 0; turn < 300 && !g.Over(); turn++ {
		legal = g.LegalMoves(legal[:0])
		cell, player := legal[rng.Intn(len(legal))], int8(g.Turn)
		simulated := g.Clone()
		waves, err := g.Apply(cell)
		if err != nil {
			t.Fatal(err)
		}
		v.Events = g.TerrainEvents
		v.Place(cell, player)
		for _, w := range waves {
			for _, source := range w {
				lost += len(v.Geo.Targets(int(source))) - len(v.Geo.Neighbours(int(source)))
			}
			v.Launch(w)
			v.Land(w)
		}
		if !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) || !slices.Equal(v.Geo.Cap, g.Geo.Cap) || !slices.Equal(v.Replay.Durability, g.Durability) {
			t.Fatalf("replay differs at move %d", turn)
		}
		if err := simulated.Simulate(cell); err != nil || simulated.Hash() != g.Hash() {
			t.Fatalf("simulation differs at move %d: %v", turn, err)
		}
		if restored, err := core.Restore(g.Snapshot()); err != nil || restored.Hash() != g.Hash() {
			t.Fatalf("restore differs at move %d: %v", turn, err)
		}
		total := int(g.Orbs[0] + g.Orbs[1] + g.Orbs[2])
		if total != g.Moves-lost {
			t.Fatalf("move %d: %d orbs, %d lost", g.Moves, total, lost)
		}
	}
	if lost == 0 {
		t.Fatal("random play never filled a Landfill")
	}
}
