package core

import "testing"

// spillwayMap is a 5×5 board with Spillways at the centre and in one corner.
func spillwayMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Spillways"
	m.Cells[12].Feature = Spillway
	m.Cells[0].Feature = Spillway
	return m
}

func TestSpillwayLosesItsExtraOrb(t *testing.T) {
	m := spillwayMap()
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatal(issues)
	}
	if m.Capacity(12) != 5 || m.Capacity(0) != 3 {
		t.Fatalf("capacities %d and %d, want 5 and 3", m.Capacity(12), m.Capacity(0))
	}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Geo.Cap[12] != 5 || len(g.Geo.Neighbours(12)) != 4 || len(g.Geo.Targets(12)) != 5 || g.Geo.Targets(12)[4] != 12 {
		t.Fatal("a Spillway must target its own pit after its neighbours")
	}
	for range 2 {
		mustApply(t, g, 0)
		mustApply(t, g, 18)
	}
	waves := mustApply(t, g, 0)
	if len(waves) != 1 || g.Count[0] != 0 || g.Owner[0] != NoOwner || g.Count[1] != 1 || g.Count[5] != 1 {
		t.Fatalf("waves %v counts %v", waves, g.Count)
	}
	if g.Orbs[0] != 2 || g.Orbs[1] != 2 || total(g) != g.Moves-1 {
		t.Fatal("the Spillway's extra orb must be lost and the tallies must match the board")
	}
}

func TestSpillwayDeadEndValidation(t *testing.T) {
	// Column 0 is walled off except cell 2, which then has one open connection.
	for _, tc := range []struct {
		feature Feature
		valid   bool
	}{{NoFeature, false}, {Spillway, true}} {
		m := NewMap(5, 5)
		m.Cells[0].Terrain, m.Cells[1].Terrain, m.Cells[3].Terrain, m.Cells[4].Terrain = Wall, Wall, Wall, Wall
		m.Cells[2].Feature = tc.feature
		if valid := len(m.Validate()) == 0; valid != tc.valid {
			t.Fatalf("feature %d: valid=%v, issues %v", tc.feature, valid, m.Validate())
		}
	}
}
