package core

import (
	"slices"
	"sync"
	"testing"
)

func denseCircuitMap(n int) MapDefinition {
	m := NewMap(n, n)
	for x := 1; x+1 < n; x += 2 {
		for y := 1; y+1 < n; y += 2 {
			group := uint8((x/2*7+y/2)%MaxMapGroups + 1)
			m.Cells[x*n+y] = MapCell{Circuit: Latch + CircuitKind((x/2+y/2)%3), Group: group}
			m.Cells[(x+1)*n+y] = MapCell{Circuit: Door, Group: group, InitiallyOpen: true}
			m.Edges = append(m.Edges, MapEdge{A: x*n + y + 1, B: (x+1)*n + y + 1, Kind: Gate, Group: group, InitiallyOpen: true})
		}
	}
	m.Cells[0].Terrain = BreakableWall
	return m
}

func TestTopologyCacheIsolationAndEviction(t *testing.T) {
	g, err := NewGameWithMap(denseCircuitMap(40), 12)
	if err != nil {
		t.Fatal(err)
	}
	geo := g.Geo
	capacity, neighbors := slices.Clone(geo.Cap), slices.Clone(geo.Nbr)
	parentHash := g.Hash()
	child := g.Clone()
	for state := 1; state <= topologyCacheSize+1; state++ {
		for group := 1; group <= MaxMapGroups; group++ {
			child.CircuitOn[group] = state&(1<<(group-1)) != 0
		}
		child.Geo = child.mapGeometry()
	}
	child.CircuitOn = g.CircuitOn
	child.Geo = child.mapGeometry()
	if !slices.Equal(child.Geo.Cap, capacity) || !slices.Equal(child.Geo.Nbr, neighbors) {
		t.Fatal("evicted topology did not rebuild identically")
	}
	if g.Hash() != parentHash || !slices.Equal(geo.Cap, capacity) || !slices.Equal(geo.Nbr, neighbors) {
		t.Fatal("search or eviction mutated a published topology")
	}
	if n := testing.AllocsPerRun(100, func() { child.Geo = child.mapGeometry() }); n != 0 {
		t.Fatalf("cached topology allocates %v times", n)
	}
	wall := 0
	child.Durability[wall] = 1
	if child.mapGeometry() != child.Geo {
		t.Fatal("wall damage changed connections before destruction")
	}
	child.Durability[wall] = 0
	child.Geo = child.mapGeometry()
	if !child.Geo.Playable(wall) || geo.Playable(wall) {
		t.Fatal("wall destruction did not stay isolated")
	}
	// Separate simulation copies can share the bounded cache across goroutines.
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			copy := g.Clone()
			for state := 0; state < 64; state++ {
				copy.CircuitOn[1] = state%2 != 0
				copy.Geo = copy.mapGeometry()
			}
		})
	}
	wg.Wait()
}

func BenchmarkDenseCircuitTopology(b *testing.B) {
	g, err := NewGameWithMap(denseCircuitMap(40), 12)
	if err != nil {
		b.Fatal(err)
	}
	for _, cached := range []bool{true, false} {
		name := "cached"
		if !cached {
			name = "changing"
		}
		b.Run(name, func(b *testing.B) {
			state := 0
			b.ReportAllocs()
			for b.Loop() {
				if !cached {
					state++
					for group := 1; group <= MaxMapGroups; group++ {
						g.CircuitOn[group] = state&(1<<(group-1)) != 0
					}
				}
				g.Geo = g.mapGeometry()
			}
		})
	}
}
