package core

import "sync"

// Cache a bounded set of immutable tables, shared by search copies and replay.
// Only destruction and circuit polarity affect connections, not ownership or presses.
const topologyCacheSize = 64 // bounded tables shared by all search copies

type topologyKey struct {
	circuit     uint16
	broken      [(MaxSize*MaxSize + 63) / 64]uint64
	brokenEdges [(2*MaxSize*(MaxSize-1) + 63) / 64]uint64
}

type topologyEntry struct {
	key topologyKey
	geo *Geometry
}

type mapTopology struct {
	switches, walls []int16
	barriers        []int16
	makers          []int16 // Furnaces and Crushers, which act after their owner's move
	reservoirs      []int16
	pairs           []int16
	mu              sync.Mutex
	cache           [topologyCacheSize]topologyEntry
	next            int
}

func newMapTopology(m *MapDefinition) *mapTopology {
	t := &mapTopology{pairs: make([]int16, len(m.Cells))}
	var first [Portal + 1][256]int
	for i := range first {
		for j := range first[i] {
			first[i][j] = -1
		}
	}
	for i, c := range m.Cells {
		t.pairs[i] = -1
		if c.Circuit.IsSwitch() {
			t.switches = append(t.switches, int16(i))
		}
		if c.Terrain == BreakableWall || c.Terrain == Landfill || c.Terrain == BreakableFloor || c.Terrain == Sinkhole {
			t.walls = append(t.walls, int16(i))
		}
		if c.Feature.IsMaker() {
			t.makers = append(t.makers, int16(i))
		}
		if c.Feature == Reservoir {
			t.reservoirs = append(t.reservoirs, int16(i))
		}
		if c.Feature == Link || c.Feature == Portal {
			if j := first[c.Feature][c.Group]; j >= 0 {
				t.pairs[i], t.pairs[j] = int16(j), int16(i)
			} else {
				first[c.Feature][c.Group] = i
			}
		}
	}
	for i, e := range m.Edges {
		if e.Kind == BreakableBarrier {
			t.barriers = append(t.barriers, int16(i))
		}
	}
	return t
}

func (g *Game) topologyKey() (key topologyKey) {
	for _, i := range g.topology.walls {
		if g.Durability[i] == 0 {
			key.broken[i/64] |= 1 << (i % 64)
		}
	}
	for _, i := range g.topology.barriers {
		if g.EdgeDurability[i] == 0 {
			key.brokenEdges[i/64] |= 1 << (i % 64)
		}
	}
	for group := 1; group <= MaxMapGroups; group++ {
		if g.CircuitOn[group] {
			key.circuit |= 1 << (group - 1)
		}
	}
	return
}
