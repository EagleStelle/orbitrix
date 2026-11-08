package core

import (
	"slices"
	"testing"
)

func mixedCircuitMap() MapDefinition {
	m := NewMap(9, 9)
	m.Cells[20] = MapCell{Circuit: Latch, Group: 1}
	m.Cells[40] = MapCell{Circuit: Toggle, Group: 1}
	m.Cells[60] = MapCell{Circuit: Toggle, Group: 1}
	m.Cells[50] = MapCell{Circuit: Hold, Group: 1}
	m.Cells[30] = MapCell{Circuit: Door, Group: 1}
	m.Cells[31] = MapCell{Circuit: Door, Group: 1, InitiallyOpen: true}
	m.Edges = []MapEdge{{A: 10, B: 11, Kind: Gate, Group: 1}, {A: 70, B: 71, Kind: Gate, Group: 1, InitiallyOpen: true}}
	return m
}

func TestCircuitMultipleSwitchesAndLatchRearming(t *testing.T) {
	g, err := NewGameWithMap(mixedCircuitMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		cell  int
		on    bool
		latch uint8
	}{
		{20, true, 1},  // First latch input reverses the circuit.
		{20, true, 1},  // The latch cannot fire twice on its own.
		{40, false, 0}, // Another switch reverses and rearms the latch.
		{20, true, 1},  // Still occupied, but a new arrival can press it again.
		{60, false, 0}, // A second toggle on the same circuit also reverses it.
		{50, true, 0},  // Hold press reverses the current polarity.
	} {
		g.Turn = 0
		if _, err := g.Apply(step.cell); err != nil {
			t.Fatal(err)
		}
		if g.CircuitOn[1] != step.on || g.Pressed[20] != step.latch {
			t.Fatalf("cell %d: polarity=%v latch=%d", step.cell, g.CircuitOn[1], g.Pressed[20])
		}
		r, err := Restore(g.Snapshot())
		if err != nil || r.Hash() != g.Hash() {
			t.Fatalf("mixed circuit restore: %v", err)
		}
	}
}

func TestCircuitSimultaneousInputsAndHoldRelease(t *testing.T) {
	g, err := NewGameWithMap(mixedCircuitMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	g.Pressed[20] = 1
	g.CircuitOn[1] = true
	g.arrived[40], g.arrived[60] = 1, 1
	if g.circuitInputs(0, true) || !g.CircuitOn[1] || g.Pressed[20] != 0 {
		t.Fatal("two inputs must cancel and rearm the latch")
	}
	if g.Pressed[40] != 1 || g.Pressed[60] != 1 {
		t.Fatal("cancelled inputs must retain their individual switch states")
	}
	clear(g.arrived)
	g.Count[50] = 1
	if !g.circuitInputs(1, true) || g.CircuitOn[1] {
		t.Fatal("Hold press did not reverse")
	}
	g.Count[50] = 0
	if !g.circuitInputs(2, true) || !g.CircuitOn[1] {
		t.Fatal("Hold release did not restore")
	}
}

func TestCircuitAcceptsThreeSameOrMixedSwitchTypes(t *testing.T) {
	for _, kinds := range [][3]CircuitKind{{Toggle, Toggle, Toggle}, {Toggle, Toggle, Hold}, {Latch, Latch, Toggle}} {
		m := NewMap(7, 7)
		for i, kind := range kinds {
			m.Cells[16+8*i] = MapCell{Circuit: kind, Group: 1}
		}
		m.Edges = []MapEdge{{A: 8, B: 9, Kind: Gate, Group: 1}}
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		for i := range kinds {
			g.arrived[16+8*i] = 1
			g.Count[16+8*i] = 1
		}
		if !g.circuitInputs(0, true) || !g.CircuitOn[1] {
			t.Fatalf("three mixed inputs must reverse once: %v", kinds)
		}
	}
}

func TestDoorInitialStatesClosureAndReplay(t *testing.T) {
	g, err := NewGameWithMap(mixedCircuitMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Geo.Playable(30) || !g.Geo.Playable(31) || g.Legal(30) {
		t.Fatal("Door starting states ignored")
	}
	if slices.Contains(g.Geo.Neighbours(10), int16(11)) || !slices.Contains(g.Geo.Neighbours(70), int16(71)) {
		t.Fatal("Gate starting states ignored")
	}
	g.Turn = 0
	if _, err := g.Apply(31); err != nil {
		t.Fatal(err)
	}
	replay := g.Clone()
	if _, err := g.Apply(40); err != nil {
		t.Fatal(err)
	}
	if !g.Geo.Playable(30) || g.Geo.Playable(31) || g.Legal(31) || g.Count[31] != 1 || g.Owner[31] != 0 {
		t.Fatal("Door closure lost trapped orbs or ignored polarity")
	}
	replay.ReplayTerrain(g.TerrainEvents)
	if !slices.Equal(replay.Geo.Cap, g.Geo.Cap) || replay.CircuitOn != g.CircuitOn || !slices.Equal(replay.Pressed, g.Pressed) {
		t.Fatal("Door replay differs")
	}
	r, err := Restore(g.Snapshot())
	if err != nil || r.Hash() != g.Hash() {
		t.Fatalf("closed occupied Door restore: %v", err)
	}
	g.Turn = 0
	if _, err := g.Apply(60); err != nil {
		t.Fatal(err)
	}
	g.Turn = 0
	if !g.Geo.Playable(31) || g.Count[31] != 1 || g.Owner[31] != 0 || !g.Legal(31) {
		t.Fatal("reopened Door did not preserve its orbs")
	}
	g.Turn = 1
	if g.Legal(31) {
		t.Fatal("reopened Door gave its orbs to another player")
	}
}

func TestMixedCircuitConservationSimulationAndSnapshots(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		g, err := NewGameWithMap(mixedCircuitMap(), 3)
		if err != nil {
			t.Fatal(err)
		}
		rng := NewRNG(seed)
		var moves []int
		for turn := 0; turn < 160 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			if len(moves) == 0 {
				t.Fatal("no legal moves")
			}
			cell := moves[rng.Intn(len(moves))]
			copy := g.Clone()
			before := g.Orbs[0] + g.Orbs[1] + g.Orbs[2]
			if _, err := g.Apply(cell); err != nil {
				t.Fatal(err)
			}
			if err := copy.Simulate(cell); err != nil || copy.Hash() != g.Hash() {
				t.Fatalf("simulation differs: %v", err)
			}
			if g.Orbs[0]+g.Orbs[1]+g.Orbs[2] != before+1 {
				t.Fatal("orb conservation failed")
			}
			r, err := Restore(g.Snapshot())
			if err != nil || r.Hash() != g.Hash() {
				t.Fatalf("seed %d turn %d: restore differs: %v", seed, turn, err)
			}
		}
	}
}

func TestCircuitStateValidationAndHash(t *testing.T) {
	m := mixedCircuitMap()
	for _, edit := range []func(*MapDefinition){
		func(m *MapDefinition) { m.Cells[20].InitiallyOpen = true },
		func(m *MapDefinition) { m.Cells[20].Circuit = 255 },
		func(m *MapDefinition) { m.Cells[20].Feature = Link },
		func(m *MapDefinition) { m.Edges[0].Kind = Barrier; m.Edges[0].InitiallyOpen = true },
		func(m *MapDefinition) { m.Edges[0].Kind = BreakableBarrier; m.Edges[0].InitiallyOpen = true },
		func(m *MapDefinition) { m.Edges[0].Kind = BreakableBarrier + 1 },
	} {
		bad := m.Clone()
		edit(&bad)
		if bad.ValidateStructure() == nil {
			t.Fatal("accepted malformed circuit design")
		}
	}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*Snapshot){
		func(s *Snapshot) { s.CircuitOn = []uint8{0} },
		func(s *Snapshot) { s.CircuitOn = []uint8{1, 1} },
		func(s *Snapshot) { s.CircuitOn = []uint8{2} },
		func(s *Snapshot) { s.Pressed[30] = 1 },
		func(s *Snapshot) { s.Pressed[50] = 1 },
	} {
		s := g.Snapshot()
		edit(&s)
		if _, err := Restore(s); err == nil {
			t.Fatal("accepted invalid circuit snapshot")
		}
	}
	before := g.Hash()
	g.Map.Cells[30].InitiallyOpen = true
	if before == g.Hash() {
		t.Fatal("Door initial state missing from hash")
	}
}

func TestInitiallyOpenGatesCanIsolateOccupiedFloor(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[12] = MapCell{Circuit: Toggle, Group: 1}
	m.Edges = []MapEdge{{A: 0, B: 1, Kind: Gate, Group: 1, InitiallyOpen: true}, {A: 0, B: 5, Kind: Gate, Group: 1, InitiallyOpen: true}}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(0); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Apply(12); err != nil {
		t.Fatal(err)
	}
	if g.Geo.Playable(0) || g.Count[0] != 1 {
		t.Fatal("isolated Floor lost orbs")
	}
	if r, err := Restore(g.Snapshot()); err != nil || r.Hash() != g.Hash() {
		t.Fatalf("isolated Floor restore: %v", err)
	}
}
