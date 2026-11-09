package core

import (
	"slices"
	"strings"
	"testing"
)

func breakableFloorMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Breakable Floors"
	m.Cells[12].Terrain = BreakableFloor
	return m
}

func TestBreakableFloorBreaksIntoVoidAndDropsItsOrbs(t *testing.T) {
	m := breakableFloorMap()
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Legal(12) || m.Capacity(12) != 4 || g.Geo.Cap[12] != 4 || g.Geo.Cap[7] != 4 || g.Durability[12] != 2 {
		t.Fatal("an intact Breakable Floor must play as Floor")
	}
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	seedMapCell(g, 7, 0, 3)
	mustApply(t, g, 7)
	if g.Durability[12] != 2 || g.Count[12] != 1 {
		t.Fatal("a neighbor's explosion must not crack it")
	}
	for hit := 1; hit <= 2; hit++ {
		g.Turn = 0
		g.Orbs[0] += int32(4 - g.Count[12])
		g.Count[12] = 4 // the placed orb makes 5: it explodes and keeps one
		if hit == 2 {
			g.Count[7]++ // its orb arrives to fill 7 to the capacity left once the floor is gone
			g.Orbs[0]++
		}
		before := total(g)
		simulated := g.Clone()
		waves := mustApply(t, g, 12)
		if g.Durability[12] != uint8(2-hit) || g.Orbs[0]+g.Orbs[1] != int32(total(g)) {
			t.Fatalf("hit %d: durability %d, tallies %v of %d", hit, g.Durability[12], g.Orbs[:2], total(g))
		}
		if err := simulated.Simulate(12); err != nil || simulated.Hash() != g.Hash() {
			t.Fatalf("hit %d: simulation differs: %v", hit, err)
		}
		if restored, err := Restore(g.Snapshot()); err != nil || restored.Hash() != g.Hash() {
			t.Fatalf("hit %d: snapshot differs: %v", hit, err)
		}
		if hit == 1 && (g.TerrainAt(12) != Floor || g.Count[12] != 1 || total(g) != before+1) {
			t.Fatal("its first explosion must only crack the floor")
		}
		// Like a closing Door, the lost route lowers 7's capacity, so it explodes in the next wave.
		if hit == 2 && (total(g) != before || g.Count[12] != 0 || g.Owner[12] != NoOwner || len(waves) != 2 || waves[1][0] != 7) {
			t.Fatalf("its leftover orb must fall into the Void and 7 explode: total %d, want %d, waves %v", total(g), before, waves)
		}
	}
	if g.TerrainAt(12) != Void || g.Legal(12) || g.Geo.Cap[7] != 3 || m.Cells[12].Terrain != BreakableFloor {
		t.Fatal("a broken floor must become Void and take a route from its neighbors, without changing the saved design")
	}
	s := g.Snapshot()
	s.Count[12], s.Owner[12] = 1, 0
	if _, err := Restore(s); err == nil {
		t.Fatal("accepted orbs on a broken floor")
	}
}

// Bot games: Breakable Floors drain nothing, but games shorten by about two thirds of the room they take when broken.
func TestBreakableFloorEstimate(t *testing.T) {
	estimate := func(cells ...int) int {
		m := NewMap(9, 9)
		for _, i := range cells {
			m.Cells[i].Terrain = BreakableFloor
		}
		moves, length, pace := m.Estimate()
		if length != 1000 || pace != Normal {
			t.Fatalf("Breakable Floors %v changed the pace: %d %v", cells, length, pace)
		}
		return moves
	}
	empty, four := estimate(), estimate(20, 24, 56, 60)
	if four*1000/empty != 909 { // played 910‰
		t.Fatalf("4 Breakable Floors: %d of %d moves", four, empty)
	}
	// Side by side, neither takes the shared route from a neighbor that outlasts it.
	if apart, together := estimate(20, 60), estimate(20, 29); together <= apart {
		t.Fatalf("adjacent Breakable Floors %d moves, apart %d", together, apart)
	}
}

func TestBreakableFloorValidationAndSharing(t *testing.T) {
	has := func(m MapDefinition, message string, cell int) bool {
		return slices.ContainsFunc(m.Validate(), func(issue MapIssue) bool {
			return strings.HasPrefix(issue.Message, message) && slices.Contains(issue.Cells, cell)
		})
	}
	m := breakableFloorMap()
	m.Cells[12].Feature = Factory
	if !has(m, "Features need Floor", 12) {
		t.Fatal("a feature on Breakable Floor must be refused")
	}
	// The corner keeps no route once both its neighbors break, even with a Breakable Wall to open later.
	for _, corner := range []Terrain{Floor, BreakableFloor, BreakableWall} {
		m = NewMap(5, 5)
		m.Cells[0].Terrain, m.Cells[1].Terrain, m.Cells[5].Terrain = corner, BreakableFloor, BreakableFloor
		if !has(m, "Floor needs a route", 0) {
			t.Fatalf("%v corner sealed by Breakable Floors must be refused", corner)
		}
	}
	split := NewMap(7, 7)
	for i := range 7 {
		split.Cells[3*7+i].Terrain = BreakableFloor
	}
	if !has(split, "All Floor must connect", 48) {
		t.Fatal("a board that Breakable Floors alone hold together must be refused")
	}
	m = NewMap(7, 7)
	m.Cells[8].Terrain, m.Cells[24].Terrain, m.Cells[40].Terrain = BreakableFloor, BreakableFloor, BreakableFloor
	if issues := m.Validate(); len(issues) != 0 || m.FloorCount() != 49 {
		t.Fatalf("Breakable Floor must count as playable Floor: %v", issues)
	}
	code, err := MapCode(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := ParseMapCode(code); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
		t.Fatalf("Breakable Floor map did not survive sharing: %v", err)
	}
}
