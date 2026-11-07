package core

import (
	"fmt"
	"testing"
)

func seedMapCell(g *Game, i int, p int8, n uint8) {
	g.Owner[i] = p
	g.Count[i] = n
	g.Orbs[p] += int32(n)
}

func TestReservoirConserves(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[12].Feature = Reservoir
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 12, 0, 4)
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	w, err := g.Apply(12)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 1 || g.Count[12] != 1 || g.Orbs[0]+g.Orbs[1] != 6 {
		t.Fatalf("waves=%v count=%d totals=%v", w, g.Count[12], g.Orbs)
	}
}

func TestMapWaveEventsAndIsolation(t *testing.T) {
	m := NewMap(7, 7)
	m.Cells[24].Terrain = BreakableWall
	m.Cells[16].Feature = Switch
	m.Cells[16].Group = 1
	m.Edges = []MapEdge{{A: 8, B: 9, Kind: Gate, Group: 1}}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 17, 0, g.Geo.Cap[17])
	seedMapCell(g, 23, 0, g.Geo.Cap[23])
	seedMapCell(g, 16, 0, g.Geo.Cap[16]-1)
	seedMapCell(g, 48, 1, 1)
	g.Played = 3
	before := g.Clone()
	beforeHash := before.Hash()
	_, err = g.Apply(16)
	if err != nil {
		t.Fatal(err)
	}
	if g.Durability[24] != 0 || !g.OpenGates[1] || g.Pressed[16] != 1 {
		t.Fatalf("terrain state wall=%d gate=%v switch=%d", g.Durability[24], g.OpenGates[1], g.Pressed[16])
	}
	if before.Hash() != beforeHash || before.Durability[24] != 2 || before.OpenGates[1] {
		t.Fatal("simulation mutated parent")
	}
	before.ReplayTerrain(g.TerrainEvents)
	if before.Geo.Cap[24] != g.Geo.Cap[24] || before.Durability[24] != 0 || !before.OpenGates[1] {
		t.Fatal("events do not reconstruct topology")
	}
	r, err := Restore(g.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if r.Hash() != g.Hash() {
		t.Fatal("snapshot hash mismatch")
	}
}

func TestCustomMapConservationAndDeterminism(t *testing.T) {
	for _, m := range StarterMaps() {
		t.Run(m.Name, func(t *testing.T) {
			g, err := NewGameWithMap(m, 3)
			if err != nil {
				t.Fatal(err)
			}
			rng := NewRNG(456)
			var moves []int
			for turn := 0; turn < 300 && !g.Over(); turn++ {
				moves = g.LegalMoves(moves[:0])
				if len(moves) == 0 {
					t.Fatal("no legal move")
				}
				cell := moves[rng.Intn(len(moves))]
				c := g.Clone()
				before := g.Orbs[0] + g.Orbs[1] + g.Orbs[2]
				if _, err := g.Apply(cell); err != nil {
					t.Fatal(err)
				}
				if err := c.Simulate(cell); err != nil {
					t.Fatal(err)
				}
				if c.Hash() != g.Hash() {
					t.Fatal("simulation differs")
				}
				if g.Orbs[0]+g.Orbs[1]+g.Orbs[2] != before+1 {
					t.Fatal("orbs lost")
				}
				r, err := Restore(g.Snapshot())
				if err != nil {
					t.Fatalf("turn %d: %v", turn, err)
				}
				if r.Hash() != g.Hash() {
					t.Fatal("restored state differs")
				}
			}
		})
	}
}

func BenchmarkCustomMapChain(b *testing.B) {
	for _, n := range []int{9, 20, 40} {
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			m := NewMap(n, n)
			m.Cells[(n/2)*n+n/2].Terrain = BreakableWall
			m.Cells[0].Feature, m.Cells[0].Group = Switch, 1
			m.Cells[n+1].Feature = Reservoir
			m.Cells[2*n+2].Feature, m.Cells[2*n+2].Group = Link, 1
			m.Cells[(n-3)*n+n-3].Feature, m.Cells[(n-3)*n+n-3].Group = Link, 1
			m.Edges = []MapEdge{{A: n + 1, B: 2*n + 1, Kind: Gate, Group: 1}}
			base, err := NewGameWithMap(m, 2)
			if err != nil {
				b.Fatal(err)
			}
			for i := range base.Count {
				if base.Geo.Playable(i) {
					p := int8(0)
					if i == len(base.Count)-1 {
						p = 1
					}
					seedMapCell(base, i, p, base.Geo.Cap[i]-1)
				}
			}
			base.Played = 3
			g := base.Clone()
			var waves []Wave
			b.ReportAllocs()
			for b.Loop() {
				g.CopyFrom(base)
				waves, err = g.Apply(0)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(waves)), "waves/op")
		})
	}
}
