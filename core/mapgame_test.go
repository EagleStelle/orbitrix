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

func TestReservoirThrowsItsExtraOrbAndMakesOne(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[12].Feature = Reservoir
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 12, 0, 4)
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	// Its arrow starts up (cell 11), then turns clockwise to the right (cell 17).
	for k, outlet := range []int{11, 17} {
		simulated := g.Clone()
		before := g.Count[outlet]
		w, err := g.Apply(12)
		if err != nil {
			t.Fatal(err)
		}
		// Five orbs pop: one to each side and the extra along the arrow; then a new one appears.
		if len(w) != 1 || g.Count[12] != 1 || g.Owner[12] != 0 || g.Count[outlet] != before+2 || g.Orbs[0] != int32(6+k*5) {
			t.Fatalf("pop %d: waves=%v counts=%v totals=%v", k, w, g.Count, g.Orbs)
		}
		if err := simulated.Simulate(12); err != nil || simulated.Hash() != g.Hash() {
			t.Fatalf("simulation differs: %v", err)
		}
		if restored, err := Restore(g.Snapshot()); err != nil || restored.Hash() != g.Hash() {
			t.Fatalf("snapshot round trip differs: %v", err)
		}
		g.Count[12] += 3
		g.Orbs[0] += 3
		g.Turn = 0
	}
	if g.Pressed[12]&3 != 2 {
		t.Fatalf("arrow step %d, want 2 (down)", g.Pressed[12]&3)
	}
}

// TestReservoirMakesOneOrbPerMove pops a corner Reservoir twice in one chain: only its first explosion makes an orb,
// or chains that never settle would grow without bound.
func TestReservoirMakesOneOrbPerMove(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[0].Feature = Reservoir
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Corner 0 (capacity 3) borders edge cells 1 and 5, each one short of exploding back into it.
	seedMapCell(g, 0, 0, 2)
	seedMapCell(g, 1, 0, 2)
	seedMapCell(g, 5, 0, 2)
	seedMapCell(g, 24, 1, 1)
	g.Played = 3
	w, err := g.Apply(0)
	if err != nil {
		t.Fatal(err)
	}
	pops := 0
	for _, wave := range w {
		for _, e := range wave {
			if e == 0 {
				pops++
			}
		}
	}
	if pops < 2 || g.Orbs[0] != 2+2+2+1+1 || g.Pressed[0]&ReservoirMade == 0 {
		t.Fatalf("pops %d totals %v state %d", pops, g.Orbs, g.Pressed[0])
	}
}

func TestMapWaveEventsAndIsolation(t *testing.T) {
	m := NewMap(7, 7)
	m.Cells[24].Terrain = BreakableWall
	m.Cells[16].Circuit = Latch
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
	if g.Durability[24] != 0 || !g.CircuitOn[1] || g.Pressed[16] != 1 {
		t.Fatalf("terrain state wall=%d gate=%v switch=%d", g.Durability[24], g.CircuitOn[1], g.Pressed[16])
	}
	if before.Hash() != beforeHash || before.Durability[24] != 2 || before.CircuitOn[1] {
		t.Fatal("simulation mutated parent")
	}
	before.ReplayTerrain(g.TerrainEvents)
	if before.Geo.Cap[24] != g.Geo.Cap[24] || before.Durability[24] != 0 || !before.CircuitOn[1] {
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
	barriers := NewMap(7, 7)
	barriers.Name = "Breakable barriers"
	barriers.Edges = []MapEdge{{A: 24, B: 25, Kind: BreakableBarrier}, {A: 24, B: 31, Kind: BreakableBarrier}}
	horizontal, vertical := portalMap(), portalMap()
	horizontal.Name, vertical.Name = "Horizontal Portals", "Vertical Portals"
	for _, i := range []int{16, 32} {
		horizontal.Cells[i].PortalMode, vertical.Cells[i].PortalMode = PortalHorizontal, PortalVertical
	}
	for _, m := range append(StarterMaps(), barriers, portalMap(), horizontal, vertical) {
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
				waves, err := g.Apply(cell)
				if err != nil {
					t.Fatal(err)
				}
				if err := c.Simulate(cell); err != nil {
					t.Fatal(err)
				}
				if c.Hash() != g.Hash() {
					t.Fatal("simulation differs")
				}
				var made int32 // each Reservoir that explodes makes one orb a move
				popped := map[int16]bool{}
				for _, w := range waves {
					for _, e := range w {
						if m.Cells[e].Feature == Reservoir && !popped[e] {
							popped[e] = true
							made++
						}
					}
				}
				if g.Orbs[0]+g.Orbs[1]+g.Orbs[2] != before+1+made {
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
			m.Cells[0].Circuit, m.Cells[0].Group = Latch, 1
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
