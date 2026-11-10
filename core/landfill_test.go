package core

import (
	"slices"
	"strings"
	"testing"
)

func landfillMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Landfills"
	m.Cells[12].Terrain = Landfill
	return m
}

func TestLandfillFillsWithoutChangingNearbyCapacity(t *testing.T) {
	m := landfillMap()
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Legal(12) || m.Capacity(7) != 4 || g.Geo.Cap[7] != 4 || g.Durability[12] != 4 {
		t.Fatal("an empty Landfill must keep nearby capacity without accepting placement")
	}
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	// One orb from each side, so no neighbor fills up on the way.
	for hit, source := range []int{7, 11, 13, 17} {
		g.Turn = 0
		seedMapCell(g, source, 0, 3)
		before := total(g)
		simulated := g.Clone()
		parentHash := simulated.Hash()
		waves := mustApply(t, g, source)
		if len(waves) != 1 || g.Durability[12] != uint8(3-hit) || g.Geo.Cap[source] != 4 {
			t.Fatalf("hit %d: waves=%v durability=%d capacity=%d", hit, waves, g.Durability[12], g.Geo.Cap[source])
		}
		if total(g) != before || g.Count[12] != 0 || g.Owner[12] != NoOwner {
			t.Fatal("incoming orbs must be neutral fill, not owned or scored")
		}
		if simulated.Hash() != parentHash || simulated.Durability[12] != uint8(4-hit) {
			t.Fatal("filling mutated a search parent")
		}
		if err := simulated.Simulate(source); err != nil || simulated.Hash() != g.Hash() {
			t.Fatalf("simulation differs: %v", err)
		}
		if restored, err := Restore(g.Snapshot()); err != nil || restored.Hash() != g.Hash() {
			t.Fatalf("snapshot differs: %v", err)
		}
		if hit < 3 && (g.TerrainAt(12) != Landfill || g.Legal(12)) {
			t.Fatal("a partly filled Landfill became playable")
		}
	}
	if g.TerrainAt(12) != Floor || g.Geo.Cap[12] != 4 || !g.Legal(12) || m.Cells[12].Terrain != Landfill {
		t.Fatal("a full Landfill must become empty Floor without changing its saved design")
	}
	g.Turn = 0
	mustApply(t, g, 12)
	if g.Count[12] != 1 || g.Owner[12] != 0 {
		t.Fatal("the new Floor did not accept normal play")
	}
}

func TestLandfillConsumesSimultaneousArrivalsBeforeBecomingFloor(t *testing.T) {
	for _, arrivals := range []int{2, 4} {
		g, err := NewGameWithMap(landfillMap(), 2)
		if err != nil {
			t.Fatal(err)
		}
		g.Durability[12] = 2 // half full: two arrivals finish it, four overfill it

		for _, source := range []int{7, 11, 13, 17}[:arrivals] {
			seedMapCell(g, source, 0, 4)
		}
		g.unstable = true
		before := total(g)
		waves := mustApply(t, g, 0)
		if len(waves) != 1 || g.TerrainAt(12) != Floor || g.Count[12] != 0 || total(g) != before+1-arrivals {
			t.Fatalf("%d arrivals: waves=%v count=%d total=%d", arrivals, waves, g.Count[12], total(g))
		}
		if len(g.TerrainEvents) != 1 || g.TerrainEvents[0].Wave != 0 || g.TerrainEvents[0].Durability != 0 {
			t.Fatalf("fill must finish once, after the wave: %v", g.TerrainEvents)
		}
	}
}

func TestLandfillOnlyFillsThroughOpenRoutes(t *testing.T) {
	for _, kind := range []EdgeKind{Barrier, BreakableBarrier, Gate} {
		m := landfillMap()
		m.Edges = []MapEdge{{A: 7, B: 12, Kind: kind}}
		if kind == Gate {
			m.Edges[0].Group = 1
			m.Cells[0] = MapCell{Circuit: Latch, Group: 1}
		}
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		seedMapCell(g, 7, 0, 2)
		mustApply(t, g, 7)
		if g.Durability[12] != 3 { // the blocked side doesn't count toward its fill either
			t.Fatalf("edge kind %d let an orb into the Landfill", kind)
		}
	}
	m := portalMap()
	m.Cells[39].Terrain = Landfill
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	start := g.Durability[39]
	if m.Capacity(9) != 4 || g.Geo.Cap[9] != 4 {
		t.Fatal("a Landfill beyond a Portal changed the source capacity")
	}
	slot := len(g.Geo.Targets(9)) - 1
	if g.Geo.Targets(9)[slot] != 39 || !slices.Equal(g.Geo.PortalPath(9, slot), []int16{16, 32}) {
		t.Fatal("a Landfill beyond a Portal lost its route or animation path")
	}
	seedMapCell(g, 9, 0, 3)
	mustApply(t, g, 9)
	if g.Durability[39] != start-1 {
		t.Fatal("a Portal arrival did not fill the Landfill")
	}
}

func TestLandfillValidationAndSharing(t *testing.T) {
	m := NewMap(7, 7)
	for i := range 7 {
		m.Cells[3*7+i].Terrain = Landfill
	}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("Landfills must connect future Floor without permanently draining orbs: %v", issues)
	}
	if _, _, pace := m.Estimate(); pace != Normal {
		t.Fatal("a Landfill's finite fill cost must not count as a permanent Hole")
	}
	code, err := MapCode(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := ParseMapCode(code); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
		t.Fatalf("Landfill map did not survive sharing: %v", err)
	}
	sealed := landfillMap()
	for _, source := range []int{7, 11, 13, 17} {
		sealed.Edges = append(sealed.Edges, MapEdge{A: min(source, 12), B: max(source, 12), Kind: Barrier})
	}
	if !slices.ContainsFunc(sealed.Validate(), func(issue MapIssue) bool {
		return strings.Contains(issue.Message, "All Floor must connect") && slices.Contains(issue.Cells, 12)
	}) {
		t.Fatal("an unreachable Landfill must be reported")
	}
	g, err := NewGameWithMap(landfillMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Durability[12] = 5
	if _, err := Restore(s); err == nil {
		t.Fatal("accepted a Landfill emptier than its fill target")
	}
	s.Durability[12] = 4
	if _, err := Restore(s); err != nil {
		t.Fatalf("refused an empty Landfill: %v", err)
	}
	s = g.Snapshot()
	s.Count[12], s.Owner[12] = 1, 0
	if _, err := Restore(s); err == nil {
		t.Fatal("accepted owned orbs inside an unfilled Landfill")
	}
}

func TestLandfillFillTargetIsItsStartingCapacityAsFloor(t *testing.T) {
	for name, tc := range map[string]struct {
		cell, beside int
		near         MapCell
		want         int
	}{
		"interior":           {12, 7, MapCell{}, 4},
		"edge":               {2, 7, MapCell{}, 3},
		"corner":             {0, 7, MapCell{}, 2},
		"beside a Wall":      {12, 7, MapCell{Terrain: Wall}, 3},
		"beside a Hole":      {12, 7, MapCell{Terrain: Hole}, 4},
		"beside a Landfill":  {12, 7, MapCell{Terrain: Landfill}, 4},
		"beside closed Door": {12, 7, MapCell{Circuit: Door, Group: 1}, 3},
		"cornered by a Wall": {0, 1, MapCell{Terrain: Wall}, 1},
	} {
		m := NewMap(5, 5)
		m.Cells[tc.cell].Terrain, m.Cells[tc.beside] = Landfill, tc.near
		if got := m.FillTarget(tc.cell); got != tc.want {
			t.Errorf("%s: fill target %d, want %d", name, got, tc.want)
		}
		refused := slices.ContainsFunc(m.Validate(), func(issue MapIssue) bool {
			return strings.HasPrefix(issue.Message, "Landfill needs initial capacity") && slices.Contains(issue.Cells, tc.cell)
		})
		if refused != (tc.want < 2) {
			t.Errorf("%s: refused=%v; the Floor it becomes needs capacity two", name, refused)
		}
	}
}

// Bot games: a Landfill drains nothing, but each orb it absorbs costs about a move before its Floor adds room.
func TestLandfillEstimate(t *testing.T) {
	estimate := func(cells ...int) int {
		m := NewMap(9, 9)
		for _, i := range cells {
			m.Cells[i].Terrain = Landfill
		}
		moves, length, pace := m.Estimate()
		if length != 1000 || pace != Normal {
			t.Fatalf("Landfills %v changed the pace: %d %v", cells, length, pace)
		}
		return moves
	}
	empty, four := estimate(), estimate(20, 24, 56, 60)
	if four*1000/empty != 1096 { // played 1089‰
		t.Fatalf("4 Landfills: %d of %d moves", four, empty)
	}
	// A corner Landfill fills with two orbs, an interior one with four.
	if corner, interior := estimate(0), estimate(40); corner >= interior {
		t.Fatalf("corner Landfill %d moves, interior %d", corner, interior)
	}
}
