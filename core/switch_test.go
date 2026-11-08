package core

import "testing"

func newSwitchGame(t *testing.T, feature CircuitKind) *Game {
	t.Helper()
	m := NewMap(7, 7)
	m.Cells[24].Circuit, m.Cells[24].Group = feature, 1
	m.Edges = []MapEdge{{A: 8, B: 9, Kind: Gate, Group: 1}}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSwitchManualPlacementIsImmediate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		feature CircuitKind
	}{
		{"latch", Latch},
		{"toggle", Toggle},
		{"pressure", Hold},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newSwitchGame(t, tc.feature)
			closedCap := g.Geo.Cap[8]
			waves, err := g.Apply(24)
			if err != nil {
				t.Fatal(err)
			}
			if len(waves) != 0 || g.Pressed[24] != 1 || !g.CircuitOn[1] || g.Geo.Cap[8] != closedCap+1 {
				t.Fatalf("waves=%v pressed=%d gate=%v capacity=%d", waves, g.Pressed[24], g.CircuitOn[1], g.Geo.Cap[8])
			}
			if len(g.TerrainEvents) != 2 || g.TerrainEvents[0] != (TerrainEvent{Wave: -1, Cell: 24, Group: 1, Switch: true, Pressed: true}) || g.TerrainEvents[1] != (TerrainEvent{Wave: -1, Cell: -1, Group: 1, Open: true}) {
				t.Fatalf("placement events=%+v", g.TerrainEvents)
			}
			g.Turn = 0
			if _, err := g.Apply(24); err != nil {
				t.Fatal(err)
			}
			wantPressed := uint8(1)
			if g.Pressed[24] != wantPressed || g.CircuitOn[1] != (wantPressed != 0) {
				t.Fatalf("second placement pressed=%d gate=%v", g.Pressed[24], g.CircuitOn[1])
			}
		})
	}
}

func TestSwitchExplosionDoesNotPress(t *testing.T) {
	for _, feature := range []CircuitKind{Latch} {
		g := newSwitchGame(t, feature)
		seedMapCell(g, 24, 0, g.Geo.Cap[24])
		g.cur = append(g.cur[:0], 24)
		g.explode()
		g.terrainWave(0, true)
		if g.Pressed[24] != 0 || g.CircuitOn[1] || len(g.TerrainEvents) != 0 {
			t.Fatalf("feature=%d pressed=%d gate=%v events=%+v", feature, g.Pressed[24], g.CircuitOn[1], g.TerrainEvents)
		}
	}
}

func TestSwitchReceivingWavePressesOnce(t *testing.T) {
	for _, feature := range []CircuitKind{Latch, Toggle} {
		g := newSwitchGame(t, feature)
		for wave := 0; wave < 2; wave++ {
			for _, i := range []int{17, 23} {
				seedMapCell(g, i, 0, g.Geo.Cap[i])
			}
			g.cur = append(g.cur[:0], 17, 23)
			g.TerrainEvents = g.TerrainEvents[:0]
			g.explode()
			g.terrainWave(wave, true)
			wantPressed := uint8(1)
			if feature == Toggle && wave == 1 {
				wantPressed = 0
			}
			if g.Count[24] != uint8(2*(wave+1)) || g.Pressed[24] != wantPressed || g.CircuitOn[1] != (wantPressed != 0) {
				t.Fatalf("feature=%d wave=%d count=%d pressed=%d gate=%v", feature, wave, g.Count[24], g.Pressed[24], g.CircuitOn[1])
			}
			wantEvents := 2
			if feature == Latch && wave == 1 {
				wantEvents = 0
			}
			if len(g.TerrainEvents) != wantEvents {
				t.Fatalf("feature=%d wave=%d events=%+v", feature, wave, g.TerrainEvents)
			}
		}
	}
}

func TestPressureReleaseClosesGateAndRechecksCapacity(t *testing.T) {
	g := newSwitchGame(t, Hold)
	g.Pressed[24], g.CircuitOn[1] = 1, true
	g.Geo = g.mapGeometry()
	openCap := g.Geo.Cap[8]
	seedMapCell(g, 8, 0, openCap-1)
	seedMapCell(g, 24, 0, g.Geo.Cap[24])
	before := g.Orbs[0]
	g.cur = append(g.cur[:0], 24)
	g.explode()
	g.terrainWave(0, true)
	if g.Count[24] != 0 || g.Pressed[24] != 0 || g.CircuitOn[1] || g.Geo.Cap[8] != openCap-1 {
		t.Fatalf("count=%d pressed=%d gate=%v capacity=%d", g.Count[24], g.Pressed[24], g.CircuitOn[1], g.Geo.Cap[8])
	}
	found := false
	for _, i := range g.next {
		found = found || i == 8
	}
	if !found {
		t.Fatalf("gate closure did not schedule newly critical cell 8: %v", g.next)
	}
	if g.Orbs[0] != before {
		t.Fatalf("gate closure changed orb total: before=%d after=%d", before, g.Orbs[0])
	}
	if len(g.TerrainEvents) != 2 || g.TerrainEvents[0].Pressed || g.TerrainEvents[1].Open {
		t.Fatalf("release events=%+v", g.TerrainEvents)
	}
}

func TestReplayTerrainUsesExplicitSwitchAndGateState(t *testing.T) {
	g := newSwitchGame(t, Hold)
	closedCap := g.Geo.Cap[8]
	g.ReplayTerrain([]TerrainEvent{
		{Cell: 24, Switch: true, Pressed: true},
		{Cell: -1, Group: 1, Open: true},
	})
	if g.Pressed[24] != 1 || !g.CircuitOn[1] || g.Geo.Cap[8] != closedCap+1 {
		t.Fatal("replay did not press switch and open gate")
	}
	g.ReplayTerrain([]TerrainEvent{
		{Cell: 24, Switch: true, Pressed: false},
		{Cell: -1, Group: 1, Open: false},
	})
	if g.Pressed[24] != 0 || g.CircuitOn[1] || g.Geo.Cap[8] != closedCap {
		t.Fatal("replay did not release switch and close gate")
	}
}

func TestSwitchConservationSimulationAndSnapshot(t *testing.T) {
	for _, feature := range []CircuitKind{Latch, Toggle, Hold} {
		g := newSwitchGame(t, feature)
		rng := NewRNG(938)
		var moves []int
		for turn := 0; turn < 120 && !g.Over(); turn++ {
			moves = g.LegalMoves(moves[:0])
			if len(moves) == 0 {
				t.Fatal("no legal move")
			}
			cell := moves[rng.Intn(len(moves))]
			before := g.Orbs[0] + g.Orbs[1]
			simulated := g.Clone()
			if _, err := g.Apply(cell); err != nil {
				t.Fatal(err)
			}
			if err := simulated.Simulate(cell); err != nil {
				t.Fatal(err)
			}
			if g.Hash() != simulated.Hash() {
				t.Fatalf("feature=%d turn=%d simulation differs", feature, turn)
			}
			if g.Orbs[0]+g.Orbs[1] != before+1 {
				t.Fatalf("feature=%d turn=%d lost orbs", feature, turn)
			}
			restored, err := Restore(g.Snapshot())
			if err != nil {
				t.Fatalf("feature=%d turn=%d restore: %v", feature, turn, err)
			}
			if restored.Hash() != g.Hash() {
				t.Fatalf("feature=%d turn=%d snapshot differs", feature, turn)
			}
		}
	}
}

func TestTogglePlacementOnlyFlipsWhenEmptyOrPopping(t *testing.T) {
	for _, pressed := range []uint8{0, 1} {
		for _, tc := range []struct {
			name  string
			count uint8
			flip  bool
			waves int
		}{
			{"empty", 0, true, 0},
			{"one orb", 1, false, 0},
			{"two orbs", 2, false, 0},
			{"popping", 3, true, 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				g := newSwitchGame(t, Toggle)
				g.Pressed[24], g.CircuitOn[1] = pressed, pressed != 0
				g.Geo = g.mapGeometry()
				if tc.count > 0 {
					seedMapCell(g, 24, 0, tc.count)
				}
				simulated := g.Clone()
				waves, err := g.Apply(24)
				if err != nil {
					t.Fatal(err)
				}
				want, wantEvents := pressed, 0
				if tc.flip {
					want ^= 1
					wantEvents = 2
				}
				if len(waves) != tc.waves || g.Pressed[24] != want || g.CircuitOn[1] != (want != 0) || len(g.TerrainEvents) != wantEvents {
					t.Fatalf("initial pressed=%d waves=%v pressed=%d gate=%v events=%+v", pressed, waves, g.Pressed[24], g.CircuitOn[1], g.TerrainEvents)
				}
				if tc.flip && (g.TerrainEvents[0] != (TerrainEvent{Wave: tc.waves - 1, Cell: 24, Group: 1, Switch: true, Pressed: want != 0}) || g.TerrainEvents[1] != (TerrainEvent{Wave: tc.waves - 1, Cell: -1, Group: 1, Open: want != 0})) {
					t.Fatalf("transition events=%+v", g.TerrainEvents)
				}
				if err := simulated.Simulate(24); err != nil {
					t.Fatal(err)
				}
				if g.Hash() != simulated.Hash() {
					t.Fatal("simulation differs from applied move")
				}
			})
		}
	}
}

func TestToggleOutgoingPopFlipsOnce(t *testing.T) {
	for _, incoming := range []bool{false, true} {
		g := newSwitchGame(t, Toggle)
		for wave := 0; wave < 2; wave++ {
			seedMapCell(g, 24, 0, g.Geo.Cap[24])
			g.cur = append(g.cur[:0], 24)
			if incoming {
				seedMapCell(g, 17, 0, g.Geo.Cap[17])
				g.cur = append(g.cur, 17)
			}
			g.TerrainEvents = g.TerrainEvents[:0]
			g.explode()
			g.terrainWave(wave, true)
			want := uint8(1 - wave)
			if g.Pressed[24] != want || g.CircuitOn[1] != (want != 0) || len(g.TerrainEvents) != 2 {
				t.Fatalf("incoming=%v wave=%d pressed=%d gate=%v events=%+v", incoming, wave, g.Pressed[24], g.CircuitOn[1], g.TerrainEvents)
			}
		}
	}
}
