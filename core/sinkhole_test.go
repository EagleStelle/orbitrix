package core

import (
	"slices"
	"strings"
	"testing"
)

func sinkholeMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Sinkholes"
	m.Cells[12].Terrain = Sinkhole
	return m
}

// checkMove compares a move with its search copy and a snapshot round trip.
func checkMove(t *testing.T, g *Game, cell int) []Wave {
	t.Helper()
	simulated := g.Clone()
	waves := mustApply(t, g, cell)
	if err := simulated.Simulate(cell); err != nil || simulated.Hash() != g.Hash() {
		t.Fatalf("simulation differs: %v", err)
	}
	if restored, err := Restore(g.Snapshot()); err != nil || restored.Hash() != g.Hash() {
		t.Fatalf("snapshot differs: %v", err)
	}
	return waves
}

func TestSinkholeCollapsesWhenAPlacementFillsIt(t *testing.T) {
	m := sinkholeMap()
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Legal(12) || m.Capacity(12) != 4 || g.Geo.Cap[12] != 4 || g.Durability[12] != 1 || m.FloorCount() != 25 {
		t.Fatal("a standing Sinkhole must play as Floor")
	}
	seedMapCell(g, 24, 1, 1)
	seedMapCell(g, 12, 0, 3)
	seedMapCell(g, 0, 0, 1)
	g.Played = 3
	before := total(g)
	waves := checkMove(t, g, 12)
	if len(waves) != 0 || g.TerrainAt(12) != Hole || g.Count[12] != 0 || g.Owner[12] != NoOwner || g.Orbs[0] != 1 || total(g) != before-3 {
		t.Fatalf("waves=%v terrain=%v count=%d orbs=%v", waves, g.TerrainAt(12), g.Count[12], g.Orbs[:2])
	}
	if !slices.Equal(g.TerrainEvents, []TerrainEvent{{Wave: -1, Cell: 12}}) {
		t.Fatalf("the collapse must be a placement event: %v", g.TerrainEvents)
	}
	if g.Legal(12) || g.Geo.Cap[7] != 4 || g.Geo.Cap[11] != 4 || m.Cells[12].Terrain != Sinkhole {
		t.Fatal("the Hole must keep its neighbors' capacity and leave the saved design alone")
	}
}

func TestSinkholeCollapsesInsteadOfExploding(t *testing.T) {
	g, err := NewGameWithMap(sinkholeMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 12, 1, 2)
	seedMapCell(g, 6, 0, 3)
	seedMapCell(g, 7, 0, 3)
	seedMapCell(g, 11, 0, 3)
	g.Played = 3
	before := total(g)
	// 6 pops 7 and 11 together, and both send an orb into the Sinkhole: 2 + 2 reaches its capacity.
	waves := checkMove(t, g, 6)
	if len(waves) != 2 || slices.Contains(waves[1], 12) || g.TerrainAt(12) != Hole {
		t.Fatalf("waves=%v terrain=%v", waves, g.TerrainAt(12))
	}
	if !slices.Equal(g.TerrainEvents, []TerrainEvent{{Wave: 1, Cell: 12}}) {
		t.Fatalf("the collapse must follow its filling wave: %v", g.TerrainEvents)
	}
	if total(g) != before+1-4 || g.Count[13] != 0 || g.Count[17] != 0 {
		t.Fatal("its orbs must be lost, never sent on")
	}
	if !g.Over() || g.Winner != 0 {
		t.Fatal("losing the last orbs into a Sinkhole must eliminate the player")
	}
}

func TestSinkholeCollapsesWhenItsCapacityFalls(t *testing.T) {
	m := sinkholeMap()
	m.Cells[0] = MapCell{Circuit: Latch, Group: 1}
	m.Cells[13] = MapCell{Circuit: Door, Group: 1, InitiallyOpen: true}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 12, 1, 3)
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	// Closing the Door takes a route, so the three orbs fill it where a Floor would explode.
	waves := checkMove(t, g, 0)
	if len(waves) != 0 || g.TerrainAt(12) != Hole || g.Orbs[1] != 1 {
		t.Fatalf("waves=%v terrain=%v orbs=%v", waves, g.TerrainAt(12), g.Orbs[:2])
	}
}

func TestSinkholeRandomPlayMatchesSearchAndSnapshots(t *testing.T) {
	m := NewMap(6, 6)
	for _, i := range []int{0, 14, 35} {
		m.Cells[i].Terrain = Sinkhole
	}
	g, err := NewGameWithMap(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	rng := NewRNG(55)
	var legal []int
	for turn := 0; turn < 300 && !g.Over(); turn++ {
		legal = g.LegalMoves(legal[:0])
		checkMove(t, g, legal[rng.Intn(len(legal))])
		for i, c := range g.Count {
			if m.Cells[i].Terrain == Sinkhole && g.Geo.Playable(i) && c >= g.Geo.Cap[i] {
				t.Fatalf("move %d: Sinkhole %d stands full", turn, i)
			}
		}
	}
	collapsed := 0
	for i := range m.Cells {
		if m.Cells[i].Terrain == Sinkhole && g.TerrainAt(i) == Hole {
			collapsed++
		}
	}
	if collapsed == 0 {
		t.Fatal("random play never filled a Sinkhole")
	}
}

func TestSinkholeSnapshotRejections(t *testing.T) {
	g, err := NewGameWithMap(sinkholeMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(s *Snapshot){
		"full":            func(s *Snapshot) { s.Count[12], s.Owner[12] = 4, 0 },
		"orbs in a hole":  func(s *Snapshot) { s.Durability[12], s.Count[12], s.Owner[12] = 0, 1, 0 },
		"durability of 2": func(s *Snapshot) { s.Durability[12] = 2 },
	} {
		s := g.Snapshot()
		change(&s)
		if _, err := Restore(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	s := g.Snapshot()
	s.Count[12], s.Owner[12] = 3, 0
	if _, err := Restore(s); err != nil {
		t.Fatalf("refused a Sinkhole one orb short: %v", err)
	}
}

func TestSinkholeValidationAndSharing(t *testing.T) {
	has := func(m MapDefinition, message string, cell int) bool {
		return slices.ContainsFunc(m.Validate(), func(issue MapIssue) bool {
			return strings.HasPrefix(issue.Message, message) && slices.Contains(issue.Cells, cell)
		})
	}
	m := sinkholeMap()
	m.Cells[12].Feature = Furnace
	if !has(m, "Features need Floor", 12) {
		t.Fatal("a feature on a Sinkhole must be refused")
	}
	// Players split apart could never meet once the Sinkholes between them collapse.
	split := NewMap(5, 5)
	for r := range 5 {
		split.Cells[2*5+r].Terrain = Sinkhole
	}
	if !has(split, "All Floor must connect", 24) {
		t.Fatal("a board held together only by Sinkholes must be refused")
	}
	m = sinkholeMap()
	m.Cells[0].Terrain = Sinkhole
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("Sinkholes beside Floor are valid: %v", issues)
	}
	code, err := MapCode(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := ParseMapCode(code); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
		t.Fatalf("Sinkhole map did not survive sharing: %v", err)
	}
}

// Bot games: a Sinkhole collapses early and slows a game like the Hole it becomes (1 played 1108‰ against a Hole's
// 1116‰ on 9×9, 3 played 2059‰ against 2069‰), so Estimate counts it as one.
func TestSinkholeEstimate(t *testing.T) {
	for _, cells := range [][]int{{40}, {20, 60}, {20, 24, 56}, {0, 40}, {39, 40}, {20, 24, 56, 60}} {
		sinks, holes := NewMap(9, 9), NewMap(9, 9)
		for _, i := range cells {
			sinks.Cells[i].Terrain, holes.Cells[i].Terrain = Sinkhole, Hole
		}
		sm, sl, sp := sinks.Estimate()
		hm, hl, hp := holes.Estimate()
		if sm != hm || sl != hl || sp != hp {
			t.Fatalf("Sinkholes %v: %d moves %d‰ %v; as Holes %d moves %d‰ %v", cells, sm, sl, sp, hm, hl, hp)
		}
	}
	m := NewMap(9, 9)
	for _, i := range []int{20, 24, 56, 60} {
		m.Cells[i].Terrain = Sinkhole
	}
	if _, _, pace := m.Estimate(); pace != Endless {
		t.Fatalf("four interior Sinkholes (played 3135‰) estimated %v", pace)
	}
	for _, i := range []int{20, 24, 56, 60} {
		if !slices.ContainsFunc(m.Validate(), func(issue MapIssue) bool {
			return strings.HasPrefix(issue.Message, "Too many Holes, Sinkholes") && slices.Contains(issue.Cells, i)
		}) {
			t.Fatalf("Endless must mark Sinkhole %d", i)
		}
	}
}
