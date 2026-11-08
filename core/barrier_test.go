package core

import (
	"fmt"
	"slices"
	"testing"
)

func TestBreakableBarrierDamageOnlyFromEndpoints(t *testing.T) {
	for _, b := range []int{25, 31} {
		for _, simultaneous := range []bool{false, true} {
			t.Run(fmt.Sprintf("b=%d/simultaneous=%v", b, simultaneous), func(t *testing.T) {
				m := NewMap(7, 7)
				m.Edges = []MapEdge{{A: 24, B: b, Kind: BreakableBarrier}}
				g, err := NewGameWithMap(m, 2)
				if err != nil {
					t.Fatal(err)
				}
				// Even other cells touching either endpoint cannot damage this edge.
				for i := range m.Cells {
					if i != 24 && i != b {
						g.cur = append(g.cur, int16(i))
					}
				}
				g.terrainWave(0, true)
				if g.EdgeDurability[0] != 2 || len(g.TerrainEvents) != 0 {
					t.Fatal("explosions outside the two endpoints damaged the barrier")
				}
				g.cur = append(g.cur[:0], 24)
				if simultaneous {
					g.cur = append(g.cur, int16(b))
				} else {
					g.terrainWave(1, true)
					if g.EdgeDurability[0] != 1 || slices.Contains(g.Geo.Neighbours(24), int16(b)) {
						t.Fatal("first endpoint blast must damage the barrier without opening it")
					}
					// A second blast from the same endpoint also destroys it.
				}
				g.terrainWave(2, true)
				if g.EdgeDurability[0] != 0 || !slices.Contains(g.Geo.Neighbours(24), int16(b)) || !slices.Contains(g.Geo.Neighbours(b), 24) {
					t.Fatal("two endpoint blasts must open the connection in both directions")
				}
				g.terrainWave(3, true)
				wantEvents := 2
				if simultaneous {
					wantEvents = 1
				}
				if len(g.TerrainEvents) != wantEvents {
					t.Fatal("destroyed barrier must stop recording damage")
				}
			})
		}
	}
}

func TestBreakableBarrierWaveState(t *testing.T) {
	for _, b := range []int{25, 31} {
		for _, tc := range []struct {
			initial, want uint8
			both          bool
		}{{2, 1, false}, {1, 0, false}, {2, 0, true}} {
			t.Run(fmt.Sprintf("b=%d/initial=%d/both=%v", b, tc.initial, tc.both), func(t *testing.T) {
				m := NewMap(7, 7)
				m.Edges = []MapEdge{{A: 24, B: b, Kind: BreakableBarrier}}
				g, err := NewGameWithMap(m, 2)
				if err != nil {
					t.Fatal(err)
				}
				g.EdgeDurability[0] = tc.initial
				seedMapCell(g, 24, 0, 2)
				if tc.both {
					seedMapCell(g, b, 0, 3)
					g.unstable = true
				}
				seedMapCell(g, 48, 1, 1)
				g.Played = 3
				before, simulated := g.Clone(), g.Clone()
				beforeHash, total := before.Hash(), g.Orbs[0]+g.Orbs[1]
				waves, err := g.Apply(24)
				if err != nil || len(waves) != 1 {
					t.Fatalf("waves=%v err=%v", waves, err)
				}
				if g.EdgeDurability[0] != tc.want || g.Count[24] != 0 || g.Count[b] != 0 || g.Orbs[0]+g.Orbs[1] != total+1 {
					t.Fatal("wave must transfer over its starting connections and conserve orbs")
				}
				wantCap := uint8(3)
				if tc.want == 0 {
					wantCap = 4
				}
				if g.Geo.Cap[24] != wantCap || g.Geo.Cap[b] != wantCap {
					t.Fatal("endpoint capacities did not follow destruction")
				}
				if !slices.Equal(g.TerrainEvents, []TerrainEvent{{Wave: 0, Cell: 0, Edge: true, Durability: tc.want}}) {
					t.Fatalf("unexpected events: %+v", g.TerrainEvents)
				}
				if err := simulated.Simulate(24); err != nil || simulated.Hash() != g.Hash() {
					t.Fatalf("simulation differs: %v", err)
				}
				if before.Hash() != beforeHash || before.EdgeDurability[0] != tc.initial || before.Geo.Cap[24] != 3 {
					t.Fatal("search copy mutated its parent")
				}
				before.ReplayTerrain(g.TerrainEvents)
				if !slices.Equal(before.EdgeDurability, g.EdgeDurability) || before.Geo != g.Geo {
					t.Fatal("replay did not reconstruct edge state and topology")
				}
				restored, err := Restore(g.Snapshot())
				if err != nil || restored.Hash() != g.Hash() {
					t.Fatalf("snapshot differs: %v", err)
				}
			})
		}
	}
}

func TestBreakableBarrierValidation(t *testing.T) {
	m := NewMap(6, 6)
	for r := range m.Rows {
		m.Edges = append(m.Edges, MapEdge{A: 2*m.Rows + r, B: 3*m.Rows + r, Kind: BreakableBarrier})
	}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("destructible partition must be playable: %v", issues)
	}
	for i := range m.Edges {
		m.Edges[i].Kind = Barrier
	}
	if len(m.Validate()) == 0 {
		t.Fatal("permanent partition must remain invalid")
	}
}

func TestBreakableBarrierRechecksNextWaveCapacity(t *testing.T) {
	m := NewMap(7, 7)
	m.Edges = []MapEdge{{A: 24, B: 25, Kind: BreakableBarrier}}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.EdgeDurability[0] = 1
	seedMapCell(g, 24, 0, 2)
	seedMapCell(g, 25, 0, 2)
	seedMapCell(g, 18, 0, 4)
	seedMapCell(g, 48, 1, 1)
	g.unstable = true
	waves, err := g.Apply(24)
	if err != nil || len(waves) != 1 || g.Count[25] != 3 || g.Geo.Cap[25] != 4 {
		t.Fatalf("destruction must cancel an explosion below the new capacity: waves=%v err=%v", waves, err)
	}
}

func TestBreakableBarrierSnapshotValidationAndHash(t *testing.T) {
	m := NewMap(7, 7)
	m.Edges = []MapEdge{{A: 24, B: 25, Kind: BreakableBarrier}, {A: 16, B: 17, Kind: Barrier}}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range [][]uint8{nil, {2}, {2, 0, 0}, {3, 0}, {2, 1}} {
		s := g.Snapshot()
		s.EdgeDurability = state
		if _, err := Restore(s); err == nil {
			t.Fatalf("accepted invalid edge durability %v", state)
		}
	}
	s := g.Snapshot()
	g.EdgeDurability[0] = 1
	if s.EdgeDurability[0] != 2 {
		t.Fatal("snapshot shares edge state with game")
	}
	restored, err := Restore(s)
	if err != nil || restored.Hash() == g.Hash() {
		t.Fatal("edge damage must affect the state hash before destruction")
	}
	// Reordering edges and their state, or reversing endpoints, changes no rules.
	s.Map.Edges[0], s.Map.Edges[1] = s.Map.Edges[1], s.Map.Edges[0]
	s.EdgeDurability[0], s.EdgeDurability[1] = s.EdgeDurability[1], s.EdgeDurability[0]
	s.Map.Edges[1].A, s.Map.Edges[1].B = s.Map.Edges[1].B, s.Map.Edges[1].A
	reordered, err := Restore(s)
	if err != nil || reordered.Hash() != restored.Hash() {
		t.Fatalf("edge order or direction changed hash: %v", err)
	}
	classic, _ := NewGame(7, 7, 2)
	s = classic.Snapshot()
	s.EdgeDurability = []uint8{1}
	if _, err := Restore(s); err == nil {
		t.Fatal("accepted edge state without a map")
	}
}
