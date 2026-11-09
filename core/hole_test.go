package core

import (
	"slices"
	"strings"
	"testing"
)

// holeMap is a 5×5 board with Holes at the centre and in one corner.
func holeMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Holes"
	m.Cells[12].Terrain = Hole
	m.Cells[0].Terrain = Hole
	return m
}

func TestHoleKeepsCapacityAndSwallowsOrbs(t *testing.T) {
	m := holeMap()
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatal(issues)
	}
	// Cell 7 borders the centre Hole, cells 1 and 5 the corner one.
	for cell, want := range map[int]int{7: 4, 1: 3, 5: 3, 12: 0, 0: 0} {
		if got := m.Capacity(cell); got != want {
			t.Fatalf("cell %d capacity %d, want %d", cell, got, want)
		}
	}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Legal(12) || g.Legal(0) {
		t.Fatal("a Hole accepted a placement")
	}
	if g.Geo.Cap[7] != 4 || len(g.Geo.Neighbours(7)) != 3 || len(g.Geo.Targets(7)) != 4 || g.Geo.Targets(7)[3] != 12 {
		t.Fatal("a Hole must count toward capacity without becoming a neighbour")
	}
	for range 3 {
		mustApply(t, g, 7)
		mustApply(t, g, 18)
	}
	waves := mustApply(t, g, 7)
	if len(waves) != 1 || g.Count[7] != 0 || g.Count[2] != 1 || g.Count[6] != 1 || g.Count[8] != 1 || g.Count[12] != 0 || g.Owner[12] != NoOwner {
		t.Fatalf("waves %v counts %v", waves, g.Count)
	}
	if g.Orbs[0] != 3 || g.Orbs[1] != 3 || total(g) != g.Moves-1 {
		t.Fatal("the orb sent into the Hole must be lost and the tallies must match the board")
	}
}

func TestHoleValidation(t *testing.T) {
	split := NewMap(5, 5)
	for r := range 5 {
		split.Cells[10+r].Terrain = Hole
	}
	pit := NewMap(5, 5)
	pit.Cells[12].Terrain, pit.Cells[13].Terrain = Hole, Hole
	pit.Edges = []MapEdge{{A: 12, B: 13, Kind: Barrier}}
	feature := holeMap()
	feature.Cells[12].Feature = Reservoir
	// The centre Hole 12 borders Floor 7, 11, 13 and 17; the corner Hole 0 borders 1 and 5.
	sealed := func(kinds ...EdgeKind) MapDefinition {
		m := holeMap()
		for k, kind := range kinds {
			m.Edges = append(m.Edges, MapEdge{A: []int{7, 11, 13, 17}[k], B: 12, Kind: kind, Group: map[bool]uint8{true: 1}[kind == Gate]})
		}
		return m
	}
	corner := holeMap()
	corner.Edges = []MapEdge{{A: 0, B: 1, Kind: Barrier}, {A: 0, B: 5, Kind: BreakableBarrier}}
	const enclosed = "A Hole needs a Floor side without a Barrier"
	for name, tc := range map[string]struct {
		m       MapDefinition
		want    string
		present bool
	}{
		"Holes do not connect Floor": {split, "All Floor must connect", true},
		"edge between Holes":         {pit, "need Floor on both sides", true},
		"feature on a Hole":          {feature, "Features need Floor", true},
		"Barrier beside a Hole":      {sealed(Barrier), "Floor on both sides", false},
		"one open Floor side":        {sealed(Barrier, Barrier, BreakableBarrier), enclosed, false},
		"sealed by Barriers":         {sealed(Barrier, Barrier, BreakableBarrier, Barrier), enclosed, true},
		"a Gate keeps it open":       {sealed(Barrier, Barrier, Barrier, Gate), enclosed, false},
		"sealed corner":              {corner, enclosed, true},
	} {
		found := false
		for _, issue := range tc.m.Validate() {
			found = found || strings.Contains(issue.Message, tc.want)
		}
		if found != tc.present {
			t.Fatalf("%s: %q present=%v in %v", name, tc.want, found, tc.m.Validate())
		}
	}
}

func TestEstimate(t *testing.T) {
	marked := func(m MapDefinition) []int {
		for _, issue := range m.Validate() {
			if strings.HasPrefix(issue.Message, "Too many") {
				return issue.Cells
			}
		}
		return nil
	}
	// column puts Holes down the middle column from the top: 2*holes+1 open Floor sides.
	column := func(size, holes int) MapDefinition {
		m := NewMap(size, size)
		for r := range holes {
			m.Cells[size/2*size+r].Terrain = Hole
		}
		return m
	}
	for _, size := range []int{9, 40} {
		empty, room := NewMap(size, size), 0
		for i := range empty.Cells {
			room += empty.Capacity(i) - 1
		}
		if moves, length, pace := empty.Estimate(); moves != room*3/4 || length != 1000 || pace != Normal {
			t.Fatalf("%d×%d empty: %d moves %v, want %d Normal", size, size, moves, pace, room*3/4)
		}
		// The pace follows the number of Hole sides, whatever the board's size.
		for holes, want := range map[int]Pace{3: Normal, 4: Slow, 6: Slow, 7: Endless} {
			m := column(size, holes)
			if _, _, pace := m.Estimate(); pace != want {
				t.Fatalf("%d×%d, %d Holes: %v, want %v", size, size, holes, pace, want)
			}
			if cells := marked(m); (want == Endless) != (len(cells) == holes) {
				t.Fatalf("%d×%d, %d Holes: marked %v", size, size, holes, cells)
			}
		}
	}
	// One piece in the open: a Hole drains four sides, then a Crusher, then a Spillway.
	var last int
	for _, cell := range []MapCell{{}, {Feature: Spillway}, {Feature: Crusher}, {Terrain: Hole}} {
		m := NewMap(9, 9)
		m.Cells[40] = cell
		moves, _, _ := m.Estimate()
		if moves <= last {
			t.Fatalf("%+v: %d moves, not over %d", cell, moves, last)
		}
		last = moves
	}
	// Factories make up for Holes, less when a Hole is beside them.
	endless := column(9, 7)
	away, beside := endless.Clone(), endless.Clone()
	away.Cells[10].Feature, beside.Cells[3*9].Feature = Factory, Factory
	movesAway, _, paceAway := away.Estimate()
	movesBeside, _, paceBeside := beside.Estimate()
	if paceAway != Slow || paceBeside != Slow || movesAway >= movesBeside {
		t.Fatalf("Factory away %d %v, beside %d %v", movesAway, paceAway, movesBeside, paceBeside)
	}
	// A 9×9 packed with Factories is fast, never instant; a 4×4 can be.
	packed := NewMap(9, 9)
	for c := 1; c < 8; c++ {
		for r := 1; r < 8; r++ {
			packed.Cells[c*9+r].Feature = Factory
		}
	}
	if moves, _, pace := packed.Estimate(); pace != Fast || moves < 20 || marked(packed) != nil {
		t.Fatalf("49 Factories on 9×9: %d moves %v", moves, pace)
	}
	tiny := NewMap(4, 4)
	var edges []int // Factories need capacity three, so not the corners
	for i := range tiny.Cells {
		if tiny.Capacity(i) >= 3 {
			edges = append(edges, i)
		}
	}
	for _, i := range edges[:10] {
		tiny.Cells[i].Feature = Factory
	}
	if moves, _, pace := tiny.Estimate(); pace != Fast || moves != 10 || marked(tiny) != nil {
		t.Fatalf("10 Factories on 4×4: %d moves %v", moves, pace)
	}
	tiny.Cells[edges[10]].Feature = Factory
	if moves, _, pace := tiny.Estimate(); pace != Instant || moves != 9 || !slices.Equal(marked(tiny), edges[:11]) {
		t.Fatalf("11 Factories on 4×4: %d moves %v, marked %v", moves, pace, marked(tiny))
	}
}

func TestHoleRandomPlayLosesOnlyHoleOrbs(t *testing.T) {
	for name, m := range map[string]MapDefinition{"Hole": holeMap(), "Spillway": spillwayMap()} {
		encoded, err := EncodeMap(m)
		if err != nil {
			t.Fatal(err)
		}
		if decoded, err := DecodeMap(encoded); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
			t.Fatalf("%s did not survive the map codec: %v", name, err)
		}
		g, err := NewGameWithMap(m, 3)
		if err != nil {
			t.Fatal(err)
		}
		rng := NewRNG(789)
		lost := 0
		var moves []int
		for turn := 0; turn < 300 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			cell := moves[rng.Intn(len(moves))]
			c := g.Clone()
			waves := mustApply(t, g, cell)
			if err := c.Simulate(cell); err != nil || c.Hash() != g.Hash() {
				t.Fatalf("%s: simulation differs", name)
			}
			for _, w := range waves {
				for _, e := range w {
					lost += int(g.Geo.Holes[e]) // these maps have no terrain that changes their geometry
				}
			}
			if total(g) != g.Moves-lost {
				t.Fatalf("%s turn %d: %d orbs after %d moves and %d lost", name, turn, total(g), g.Moves, lost)
			}
			r, err := Restore(g.Snapshot())
			if err != nil || r.Hash() != g.Hash() {
				t.Fatalf("%s turn %d: restore %v", name, turn, err)
			}
		}
		if lost == 0 {
			t.Fatalf("random play never lost an orb to a %s", name)
		}
	}
}

func mustApply(t *testing.T, g *Game, cell int) []Wave {
	t.Helper()
	waves, err := g.Apply(cell)
	if err != nil {
		t.Fatal(err)
	}
	return waves
}
