package core

import (
	"slices"
	"testing"
)

// factoryMap is a 5×5 board with Factories on the left edge and at the centre, and a Crusher on the right edge.
func factoryMap() MapDefinition {
	m := NewMap(5, 5)
	m.Name = "Factories"
	m.Cells[2].Feature = Factory
	m.Cells[12].Feature = Factory
	m.Cells[22].Feature = Crusher
	return m
}

func newFactoryGame(t *testing.T) *Game {
	t.Helper()
	m := factoryMap()
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatal(issues)
	}
	if m.Capacity(2) != 3 || m.Capacity(12) != 4 || m.Capacity(22) != 3 {
		t.Fatal("Factories and Crushers must keep the normal capacity")
	}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestFactoryProducesForItsOwnerUntilItPops(t *testing.T) {
	g := newFactoryGame(t)
	steps := []struct {
		cell, factory int
		produced      bool
	}{
		{12, 1, true}, // a: the placed orb; the Factory skips its first turn
		{6, 1, false}, // b owns no Factory
		{20, 2, true}, // a moves elsewhere and the Factory makes one
		{7, 2, false}, // b
		{21, 3, true}, // a
		{8, 3, false}, // b
		{23, 0, true}, // a: the Factory reaches capacity and explodes, capturing b's cell 7
	}
	for i, s := range steps {
		waves := mustApply(t, g, s.cell)
		checkTallies(t, g)
		if g.Count[12] != uint8(s.factory) || (g.ProducedAt >= 0) != s.produced {
			t.Fatalf("step %d: Factory %d orbs, produced at %d", i, g.Count[12], g.ProducedAt)
		}
		if working := MakerWorking(g.Pressed[12], g.Owner[12]); working != (i >= 2 && s.factory > 0) {
			t.Fatalf("step %d: Factory working=%v", i, working)
		}
		if i == len(steps)-1 && (g.ProducedAt != 0 || len(waves) != 1 || !slices.Equal(waves[0], Wave{12})) {
			t.Fatalf("the Factory's chain must follow the move: produced at %d, waves %v", g.ProducedAt, waves)
		}
	}
	if g.Owner[12] != NoOwner || g.Pressed[12] != 0 || g.Owner[7] != 0 || g.Orbs[1] != 2 {
		t.Fatal("a popped Factory must turn neutral, start over and its chain capture as usual")
	}
	if total(g) != g.Moves+3 {
		t.Fatalf("%d orbs after %d moves and 3 made", total(g), g.Moves)
	}
	if err := g.Pass(); err != nil || total(g) != g.Moves+3 {
		t.Fatal("a Pass is not a move and must not run Factories")
	}
}

// A Factory or Crusher that pops empty and is refilled by its holder in the same move keeps working.
func TestMakerRefilledByItsHolderKeepsWorking(t *testing.T) {
	for f, want := range map[Feature]uint8{Factory: 3, Crusher: 1} {
		m := NewMap(5, 5)
		m.Cells[12].Feature = f
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		s := g.Snapshot()
		s.Played = 3
		s.Count[12], s.Owner[12], s.Pressed[12] = 3, 0, 1|makerWorking
		s.Count[7], s.Owner[7], s.Count[11], s.Owner[11] = 3, 0, 3, 0 // both pop back into the centre
		s.Count[24], s.Owner[24] = 1, 1
		if g, err = Restore(s); err != nil {
			t.Fatal(err)
		}
		mustApply(t, g, 12) // pops; 7 and 11 refill it with two orbs, then it works for a
		checkTallies(t, g)
		if g.Count[12] != want || g.Owner[12] != 0 || !MakerWorking(g.Pressed[12], 0) || g.ProducedAt < 0 {
			t.Fatalf("%v: %d orbs, owner %d, state %d", f, g.Count[12], g.Owner[12], g.Pressed[12])
		}
	}
}

func TestFactoryAndCrusherNeedCapacityThree(t *testing.T) {
	for _, f := range []Feature{Factory, Crusher} {
		for _, tc := range []struct {
			cell  int
			walls []int
			valid bool
		}{
			{0, nil, false},           // corner: capacity two
			{12, []int{7, 11}, false}, // interior walled down to two
			{2, nil, true},            // edge: three
			{12, nil, true},           // interior: four
		} {
			m := NewMap(5, 5)
			m.Cells[tc.cell].Feature = f
			for _, w := range tc.walls {
				m.Cells[w].Terrain = Wall
			}
			if valid := len(m.Validate()) == 0; valid != tc.valid {
				t.Fatalf("feature %d at %d: valid=%v, issues %v", f, tc.cell, valid, m.Validate())
			}
		}
	}
}

func TestCrusherTakesOneUntilEmpty(t *testing.T) {
	g := newFactoryGame(t)
	mustApply(t, g, 6)
	mustApply(t, g, 16)
	// Give a three orbs on the Crusher, as a chain would.
	g.Count[22], g.Owner[22], g.Orbs[0] = 3, 0, g.Orbs[0]+3
	cells := []int{7, 17, 8, 13, 11, 18, 3, 19} // a and b alternate, on single orbs that never explode
	for i, want := range []uint8{3, 3, 2, 2, 1, 1, 0, 0} {
		mustApply(t, g, cells[i])
		checkTallies(t, g)
		if g.Count[22] != want || MakerWorking(g.Pressed[22], g.Owner[22]) != (i >= 2 && want > 0) {
			t.Fatalf("step %d: Crusher has %d orbs, state %d", i, g.Count[22], g.Pressed[22])
		}
	}
	if g.Owner[22] != NoOwner || g.Pressed[22] != 0 {
		t.Fatal("an empty Crusher must be neutral and start over")
	}
}

func TestCrusherSkipsItsFirstMeal(t *testing.T) {
	g := newFactoryGame(t)
	mustApply(t, g, 22) // a's first orb survives the move it was placed
	if g.Count[22] != 1 || g.Owner[22] != 0 || MakerWorking(g.Pressed[22], 0) {
		t.Fatal("a Crusher must not eat on the move it becomes someone's")
	}
	mustApply(t, g, 7)
	mustApply(t, g, 6) // a's next move: it eats
	if g.Count[22] != 0 || g.Owner[22] != NoOwner || g.Orbs[0] != 1 || g.Over() {
		t.Fatal("a Crusher must eat on its holder's next move")
	}
	for _, tc := range []struct {
		state uint8
		p     int8
		next  uint8
		eat   bool
	}{
		{0, 0, 1, false},               // new
		{1, 0, 1 | makerWorking, true}, // its holder's next move
		{1 | makerWorking, 0, 1 | makerWorking, true},
		{1 | makerWorking, 1, 2, false}, // captured: another skipped meal
	} {
		if next, eat := MakerTurn(tc.state, tc.p); next != tc.next || eat != tc.eat {
			t.Fatalf("MakerTurn(%d, %d) = %d, %v", tc.state, tc.p, next, eat)
		}
	}
}

func TestRestoreChecksCrusherState(t *testing.T) {
	g := newFactoryGame(t)
	mustApply(t, g, 22)
	for _, state := range []uint8{makerWorking, 3, 32} {
		s := g.Snapshot()
		s.Pressed[22] = state
		if _, err := Restore(s); err == nil {
			t.Fatalf("Crusher state %d restored", state)
		}
	}
	s := g.Snapshot()
	s.Count[22], s.Owner[22] = 0, NoOwner // an empty Crusher with a holder
	if _, err := Restore(s); err == nil {
		t.Fatal("empty Crusher state restored")
	}
	s.Pressed[22] = 0
	if _, err := Restore(s); err != nil {
		t.Fatalf("an empty, reset Crusher must restore: %v", err)
	}
}

func TestFactoryRandomPlay(t *testing.T) {
	m := factoryMap()
	m.Cells[6].Feature, m.Cells[18].Feature = Crusher, Factory
	encoded, err := EncodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := DecodeMap(encoded); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
		t.Fatalf("Factories and Crushers did not survive the map codec: %v", err)
	}
	ran, chained := false, false
	for seed := uint64(1); seed <= 20; seed++ {
		g, err := NewGameWithMap(m, 3)
		if err != nil {
			t.Fatal(err)
		}
		rng := NewRNG(seed)
		var moves []int
		for turn := 0; turn < 300 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			cell := moves[rng.Intn(len(moves))]
			c := g.Clone()
			waves := mustApply(t, g, cell)
			checkTallies(t, g)
			if err := c.Simulate(cell); err != nil || c.Hash() != g.Hash() {
				t.Fatalf("seed %d turn %d: simulation differs", seed, turn)
			}
			if g.ProducedAt > len(waves) {
				t.Fatalf("produced after wave %d of %d", g.ProducedAt, len(waves))
			}
			ran = ran || g.ProducedAt >= 0
			chained = chained || g.ProducedAt >= 0 && len(waves) > g.ProducedAt
			r, err := Restore(g.Snapshot())
			if err != nil || r.Hash() != g.Hash() {
				t.Fatalf("seed %d turn %d: restore %v", seed, turn, err)
			}
		}
	}
	if !ran || !chained {
		t.Fatal("random play never ran a Factory or never popped one")
	}
}
