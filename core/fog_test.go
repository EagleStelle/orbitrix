package core

import (
	"slices"
	"testing"
)

// fogGame returns a 5×5 fog game of two after player 0 took corner 0 and player 1 corner 24.
func fogGame(t *testing.T, m *MapDefinition) *Game {
	t.Helper()
	g, err := NewGame(5, 5, 2)
	if m != nil {
		g, err = NewGameWithMap(*m, 2)
	}
	if err == nil {
		err = g.SetFog()
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range []int{0, 24} {
		if !g.Legal(cell) {
			t.Fatal("a first orb goes anywhere")
		}
		if _, err := g.Apply(cell); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestFogPlacesOnlyInSight(t *testing.T) {
	g := fogGame(t, nil)
	if got := g.LegalMoves(nil); !slices.Equal(got, []int{0, 1, 5, 6}) {
		t.Fatalf("player 0 may place at %v, want the 3×3 around their corner", got)
	}
	if _, err := g.Apply(12); err == nil {
		t.Fatal("Apply took a cell out of sight")
	}
	if g.Memory.Seen[6] != 1 || g.Memory.Seen[18] != 2 || g.Memory.Seen[12] != 0 {
		t.Fatalf("seen %d %d %d, want each side's own corner only", g.Memory.Seen[6], g.Memory.Seen[18], g.Memory.Seen[12])
	}
	if err := g.SetFog(); err == nil {
		t.Fatal("fog turned on mid-game")
	}

	s, err := Restore(g.Snapshot())
	if err != nil || !s.Fog || s.Legal(12) || !s.Legal(6) || s.Hash() != g.Hash() {
		t.Fatalf("restored fog game: %v, fog %v", err, s.Fog)
	}
	sim := g.Clone()
	if _, err := g.Apply(6); err != nil || sim.Simulate(6) != nil || !slices.Equal(sim.Memory.Seen, g.Memory.Seen) || g.Memory.Seen[12] != 1 {
		t.Fatal("Simulate and Apply see differently, or the new orb sees nothing")
	}
}

func TestFogTeammatesShareSight(t *testing.T) {
	g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
		s.Fog = true
		put(0, 1)
		put(24, 0, 1) // player 0's teammate
		put(12, 0, 0, 1)
	})
	if got := g.LegalMoves(nil); !slices.Equal(got, []int{0, 1, 5, 6, 18, 19, 23}) {
		t.Fatalf("player 0 may place at %v, want around their own and their teammate's orbs", got)
	}
}

func TestFogBelief(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[6].Terrain = Wall                          // in player 0's sight
	m.Cells[12].Terrain = Wall                         // in nobody's
	m.Edges = []MapEdge{{A: 13, B: 18, Kind: Barrier}} // beside player 1
	g := fogGame(t, &m)

	var b Game
	g.Believed(&b, 0)
	switch {
	case b.Count[24] != 0 || b.Orbs[1] != 0 || b.Count[0] != 1:
		t.Fatal("the belief shows orbs out of sight, or hides its own")
	case b.Played&2 != 0 || b.Over():
		t.Fatal("a rival out of sight looks out of the game")
	case b.Map.Cells[6].Terrain != Wall || b.Map.Cells[12].Terrain != Floor || len(b.Map.Edges) != 0:
		t.Fatal("the belief shows terrain never seen, or hides terrain seen")
	case !b.Fog || b.Memory.Seen != nil || !slices.Equal(b.LegalMoves(nil), g.LegalMoves(nil)):
		t.Fatal("the belief plays by other rules")
	case g.Map.Cells[12].Terrain != Wall || len(g.Map.Edges) != 1:
		t.Fatal("the belief changed the game's map")
	}
	if err := b.Simulate(0); err != nil || b.Memory.Seen != nil {
		t.Fatal("a belief learns as it plays")
	}

	var c Game
	g.Believed(&c, 1)
	if c.Count[0] != 0 || c.Map.Cells[6].Terrain != Floor || c.Map.Cells[12].Terrain != Floor || len(c.Map.Edges) != 1 {
		t.Fatal("player 1's belief is not their own")
	}
}

func TestFogOffBeliefIsCopy(t *testing.T) {
	g, err := NewGame(5, 5, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range []int{0, 24} {
		if _, err := g.Apply(cell); err != nil {
			t.Fatal(err)
		}
	}
	var b Game
	g.Believed(&b, 0)
	if b.Hash() != g.Hash() || b.Fog || !b.Legal(12) {
		t.Fatal("without fog the belief is the game")
	}
}

// TestFogRemembersLastSeen: out of sight, a side's map stays as it last saw it.
func TestFogRemembersLastSeen(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[6].Terrain = BreakableWall
	m.Cells[7] = MapCell{Circuit: Door, Group: 1} // closed
	m.Cells[17] = MapCell{Circuit: Toggle, Group: 1}
	m.Cells[12].Terrain = Wall // never seen
	g, err := NewGameWithMap(m, 2)
	if err == nil {
		err = g.SetFog()
	}
	if err != nil {
		t.Fatal(err)
	}
	sight := make([]uint16, 25)
	sight[6], sight[7] = 1, 1
	var mem Memory
	if mem.See(g, &g.Cells, sight) != 1 || mem.See(g, &g.Cells, sight) != 0 {
		t.Fatal("See reports changes it didn't make, or misses one")
	}
	g.Durability[6], g.CircuitOn[1] = 0, true // out of sight the wall breaks and the Door opens
	g.Geo = g.mapGeometry()

	var r Game
	g.Recall(&r, &mem, 0)
	if r.TerrainAt(6) != BreakableWall || r.TerrainAt(7) != Wall || r.TerrainAt(12) != Floor {
		t.Fatalf("recalled %v %v %v, want the wall and closed Door as seen, the unseen Wall unknown", r.TerrainAt(6), r.TerrainAt(7), r.TerrainAt(12))
	}
	if g.TerrainAt(6) != Floor || g.TerrainAt(7) != Floor || g.Map.Cells[12].Terrain != Wall {
		t.Fatal("recalling changed the game")
	}
	if g.Recall(&r, &mem, 1); r.TerrainAt(6) != Floor || r.TerrainAt(7) != Floor {
		t.Fatal("a side recalls what only another side saw")
	}
	if mem.See(g, &g.Cells, sight) != 1 {
		t.Fatal("seeing the change again leaves the memory as it was")
	}
}
