package core

import (
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

func TestHoleRandomPlayLosesOnlyHoleOrbs(t *testing.T) {
	m := holeMap()
	encoded, err := EncodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := DecodeMap(encoded); err != nil || decoded.Cells[12].Terrain != Hole {
		t.Fatalf("Hole did not survive the map codec: %v", err)
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
			t.Fatal("simulation differs")
		}
		for _, w := range waves {
			for _, e := range w {
				lost += int(g.Geo.Holes[e]) // this map has no terrain that changes its geometry
			}
		}
		if total(g) != g.Moves-lost {
			t.Fatalf("turn %d: %d orbs after %d moves and %d lost", turn, total(g), g.Moves, lost)
		}
		r, err := Restore(g.Snapshot())
		if err != nil || r.Hash() != g.Hash() {
			t.Fatalf("turn %d: restore %v", turn, err)
		}
	}
	if lost == 0 {
		t.Fatal("random play never reached a Hole")
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
