package core

import (
	"slices"
	"testing"
)

// On the 7×7 boards below cell (c, r) is c*7+r: 24 is the centre, 17 left of it, 31 right, 23 above, 25 below.
func boxMap(boxes ...int) MapDefinition {
	m := NewMap(7, 7)
	m.Name = "Boxes"
	for _, i := range boxes {
		m.Cells[i].Feature = Box
	}
	return m
}

// boxGame starts m with player 1 holding the far corner and every cell in full at capacity, so they all explode in
// the first wave of player 0's move at the near corner.
func boxGame(t *testing.T, m MapDefinition, seed func(g *Game), full ...int) *Game {
	t.Helper()
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 48, 1, 1)
	g.Played = 3
	if seed != nil {
		seed(g)
	}
	for _, i := range full {
		seedMapCell(g, i, 0, g.Geo.Cap[i])
	}
	g.unstable = true
	return g
}

func TestBoxPushes(t *testing.T) {
	walls := func(cells ...int) func(*MapDefinition) {
		return func(m *MapDefinition) {
			for _, i := range cells {
				m.Cells[i].Terrain = Wall
			}
		}
	}
	type cell struct {
		i     int
		owner int8
		count uint8
	}
	for _, tc := range []struct {
		name  string
		boxes []int
		edit  func(*MapDefinition)
		seed  []cell // before the move, besides the full cells
		full  []int
		box   []int  // where Boxes stand after
		want  []cell // counts after
		waves int
	}{
		{name: "straight", boxes: []int{24}, full: []int{17}, box: []int{31}, want: []cell{{24, 0, 1}, {17, NoOwner, 0}}, waves: 1},
		{name: "shove captures", boxes: []int{24}, seed: []cell{{31, 1, 2}, {38, 0, 1}}, full: []int{17}, box: []int{31}, want: []cell{{24, 0, 1}, {31, NoOwner, 0}, {38, 1, 3}}, waves: 1},
		// 38 pops for player 1 in the next wave, pushing the Box back and shoving 24's orb onto the emptied 17.
		{name: "shove over capacity pops next wave", boxes: []int{24}, seed: []cell{{31, 1, 2}, {38, 0, 2}}, full: []int{17}, box: []int{24}, want: []cell{{38, NoOwner, 0}, {31, 1, 1}, {17, 0, 1}, {24, NoOwner, 0}}, waves: 2},
		{name: "bounce off a Wall", boxes: []int{24}, edit: walls(31), full: []int{17}, box: []int{17}, want: []cell{{24, 0, 1}, {17, NoOwner, 0}}, waves: 1},
		{name: "bounce off a Box", boxes: []int{24, 31}, full: []int{17}, box: []int{17, 31}, want: []cell{{24, 0, 1}}, waves: 1},
		{name: "blocked shove stays", boxes: []int{24}, edit: walls(38), seed: []cell{{31, 1, 2}}, full: []int{17}, box: []int{24}, want: []cell{{17, 0, 1}, {31, 1, 2}}, waves: 1},
		{name: "opposite pushes cancel", boxes: []int{24}, full: []int{17, 31}, box: []int{24}, want: []cell{{17, 0, 1}, {31, 0, 1}}, waves: 1},
		{name: "crossing pushes go diagonal", boxes: []int{24}, full: []int{17, 23}, box: []int{32}, want: []cell{{24, 0, 2}}, waves: 1},
		{name: "blocked diagonal stays", boxes: []int{24}, edit: walls(32), full: []int{17, 23}, box: []int{24}, want: []cell{{17, 0, 1}, {23, 0, 1}}, waves: 1},
		{name: "two Boxes claiming one cell stay", boxes: []int{23, 25}, full: []int{22, 26}, box: []int{23, 25}, want: []cell{{22, 0, 1}, {26, 0, 1}, {24, NoOwner, 0}}, waves: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := boxMap(tc.boxes...)
			if tc.edit != nil {
				tc.edit(&m)
			}
			g := boxGame(t, m, func(g *Game) {
				for _, c := range tc.seed {
					seedMapCell(g, c.i, c.owner, c.count)
				}
			}, tc.full...)
			before, parent := total(g), g.Clone()
			waves := mustApply(t, g, 0)
			var boxes []int
			for i, b := range g.Boxes {
				if b&BoxHere != 0 {
					boxes = append(boxes, i)
				}
			}
			if len(waves) != tc.waves || !slices.Equal(boxes, tc.box) || total(g) != before+1 {
				t.Fatalf("waves=%v boxes=%v orbs %d→%d", waves, boxes, before, total(g))
			}
			for _, c := range tc.want {
				if g.Count[c.i] != c.count || g.Owner[c.i] != c.owner {
					t.Fatalf("cell %d: %d orbs of %d, want %d of %d", c.i, g.Count[c.i], g.Owner[c.i], c.count, c.owner)
				}
			}
			checkTallies(t, g)
			if err := parent.Simulate(0); err != nil || parent.Hash() != g.Hash() {
				t.Fatalf("simulation differs: %v", err)
			}
			if r, err := Restore(g.Snapshot()); err != nil || r.Hash() != g.Hash() {
				t.Fatalf("restore: %v", err)
			}
		})
	}
}

func TestBoxBounceShovesThePushersOrbs(t *testing.T) {
	m := boxMap(24)
	m.Cells[31].Terrain = Wall
	g := boxGame(t, m, func(g *Game) { seedMapCell(g, 17, 0, 5) })
	mustApply(t, g, 0)
	// 17 kept one orb after exploding; the Box bounced onto it and shoved it to 10, which also caught one of 17's.
	if g.Boxes[17] != BoxHere || g.Count[17] != 0 || g.Count[10] != 2 || g.Count[24] != 1 {
		t.Fatalf("box %d, counts 17=%d 10=%d 24=%d", g.Boxes[17], g.Count[17], g.Count[10], g.Count[24])
	}
}

func TestBoxFillsHolesAndLandfills(t *testing.T) {
	for _, pit := range []Terrain{Hole, Landfill} {
		m := boxMap(24)
		m.Cells[31].Terrain = pit
		g := boxGame(t, m, nil, 17)
		parent := g.Clone()
		mustApply(t, g, 0)
		if g.Boxes[31] != CrateFloor || g.Boxes[24] != 0 || g.TerrainAt(31) != Floor || g.Geo.Cap[31] != 4 || !g.Legal(31) || g.Count[24] != 1 {
			t.Fatalf("%v: boxes %d %d, terrain %v, capacity %d", pit, g.Boxes[24], g.Boxes[31], g.TerrainAt(31), g.Geo.Cap[31])
		}
		if parent.Hash() == g.Hash() || parent.TerrainAt(31) != pit {
			t.Fatalf("%v: filling changed the search parent", pit)
		}
		replay := parent.Clone()
		replay.ReplayTerrain(g.TerrainEvents)
		if !slices.Equal(replay.Boxes, g.Boxes) || !slices.Equal(replay.Geo.Cap, g.Geo.Cap) {
			t.Fatalf("%v: replay differs", pit)
		}
		if r, err := Restore(g.Snapshot()); err != nil || r.Hash() != g.Hash() || r.Geo.Cap[31] != 4 {
			t.Fatalf("%v: restore %v", pit, err)
		}
	}
}

func TestBoxPressesHold(t *testing.T) {
	m := boxMap(24)
	m.Cells[31].Circuit, m.Cells[31].Group = Hold, 1
	m.Cells[6].Circuit, m.Cells[6].Group, m.Cells[6].InitiallyOpen = Door, 1, true
	g := boxGame(t, m, nil, 17)
	mustApply(t, g, 0)
	if g.Boxes[31] != BoxHere || g.Pressed[31] != 1 || !g.CircuitOn[1] || g.TerrainAt(6) != Wall {
		t.Fatalf("box %d pressed %d circuit %v", g.Boxes[31], g.Pressed[31], g.CircuitOn[1])
	}
	if r, err := Restore(g.Snapshot()); err != nil || r.Hash() != g.Hash() {
		t.Fatalf("restore: %v", err)
	}
}

func TestBoxCellsTakeNoPlacement(t *testing.T) {
	m := boxMap(24)
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Legal(24) || slices.Contains(g.LegalMoves(nil), 24) || m.FloorCount() != 48 || m.Capacity(17) != 4 || g.Geo.Cap[17] != 4 {
		t.Fatal("a Box's cell must route like Floor without taking placement or counting toward players")
	}
}

func TestRestoreRejectsBadBoxes(t *testing.T) {
	m := boxMap(24)
	m.Cells[31].Terrain = Hole
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(*Snapshot){
		"length":         func(s *Snapshot) { s.Boxes = s.Boxes[:3] },
		"extra Box":      func(s *Snapshot) { s.Boxes[10] = BoxHere },
		"unknown bit":    func(s *Snapshot) { s.Boxes[24] |= 4 },
		"orbs under Box": func(s *Snapshot) { s.Count[24], s.Owner[24] = 1, 0 },
		"crate on Floor": func(s *Snapshot) { s.Boxes[24] = CrateFloor },
		"Box on a Hole":  func(s *Snapshot) { s.Boxes[24], s.Boxes[31] = 0, BoxHere },
	} {
		s := g.Snapshot()
		edit(&s)
		if _, err := Restore(s); err == nil {
			t.Errorf("%s: restored", name)
		}
	}
}

func TestBoxEstimate(t *testing.T) {
	empty := NewMap(9, 9)
	emptyMoves, _, _ := empty.Estimate()
	m := NewMap(9, 9)
	m.Cells[40].Feature = Box
	if moves, length, _ := m.Estimate(); moves != emptyMoves || length != 1000 {
		t.Fatalf("a Box alone: %d moves, length %d", moves, length)
	}
	m.Cells[41].Terrain = Hole
	if _, length, _ := m.Estimate(); length != 1072 { // 17 a side instead of the 45 that makes 1219
		t.Fatalf("a Box beside a Hole: length %d", length)
	}
	// Seven Holes down the middle never end; seven Boxes to fill them make it a Normal game.
	for r := range 7 {
		m.Cells[4*9+r].Terrain, m.Cells[r].Feature = Hole, Box
	}
	m.Cells[40].Feature, m.Cells[41].Terrain = NoFeature, Floor
	if _, _, pace := m.Estimate(); pace != Normal || slices.ContainsFunc(m.Validate(), func(i MapIssue) bool { return i.Cells != nil && len(i.Cells) == 7 }) {
		t.Fatalf("Boxes for every Hole: %v", pace)
	}
	if _, length, _ := springMap().Estimate(); length != 1000 {
		t.Fatalf("a Spring drains: length %d", length)
	}
}

// Random play over Boxes beside Holes, Landfills, Springs, Portals and a Hold keeps tallies, determinism and
// snapshots, and never leaves orbs under a Box.
func TestBoxRandomPlay(t *testing.T) {
	m := NewMap(9, 9)
	m.Name = "Crates"
	for _, i := range []int{20, 30, 40, 50, 60, 22, 58} {
		m.Cells[i].Feature = Box
	}
	m.Cells[31].Terrain, m.Cells[49].Terrain = Hole, Landfill
	m.Cells[0].Terrain, m.Cells[1].Feature = Wall, Spring // faces down, along the left edge
	m.Cells[11].Feature, m.Cells[11].Group, m.Cells[69].Feature, m.Cells[69].Group = Portal, 1, Portal, 1
	m.Cells[42].Circuit, m.Cells[42].Group = Hold, 1
	m.Edges = []MapEdge{{A: 52, B: 53, Kind: Gate, Group: 1}}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatal(issues)
	}
	for seed := range uint64(6) {
		g, err := NewGameWithMap(m, 2+int(seed%3))
		if err != nil {
			t.Fatal(err)
		}
		rng := NewRNG(seed)
		var moves []int
		start, moved := slices.Clone(g.Boxes), false
		for turn := 0; turn < 400 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			cell := moves[rng.Intn(len(moves))]
			c := g.Clone()
			mustApply(t, g, cell)
			checkTallies(t, g)
			if err := c.Simulate(cell); err != nil || c.Hash() != g.Hash() {
				t.Fatalf("seed %d turn %d: simulation differs", seed, turn)
			}
			if total(g) > g.Moves {
				t.Fatalf("seed %d turn %d: %d orbs after %d moves", seed, turn, total(g), g.Moves)
			}
			moved = moved || !slices.Equal(g.Boxes, start)
			for i, b := range g.Boxes {
				if b&BoxHere != 0 && g.Count[i] != 0 {
					t.Fatalf("seed %d turn %d: orbs under the Box at %d", seed, turn, i)
				}
			}
			if r, err := Restore(g.Snapshot()); err != nil || r.Hash() != g.Hash() {
				t.Fatalf("seed %d turn %d: restore %v", seed, turn, err)
			}
		}
		if !moved {
			t.Fatalf("seed %d: no Box ever moved", seed)
		}
	}
}

func TestLastOutletUndoesOutlet(t *testing.T) {
	for mask := uint8(1); mask < 16; mask++ {
		geo := Geometry{Outlets: []uint8{mask}}
		for state := range uint8(4) {
			slot, next := geo.Outlet(0, state|ReservoirMade)
			if last := geo.LastOutlet(0, next); last != slot {
				t.Fatalf("mask %04b state %d: Outlet slot %d, LastOutlet %d", mask, state, slot, last)
			}
		}
	}
}
