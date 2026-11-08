package core

import "sync"

// Cache a bounded set of immutable tables, shared by search copies and replay.
// Only destruction and circuit polarity affect connections, not ownership or presses.
const topologyCacheSize = 64 // about 1.2 MiB of tables at 40×40, shared by all search copies

type topologyKey struct {
	circuit uint16
	broken  [(MaxSize*MaxSize + 63) / 64]uint64
}

type topologyEntry struct {
	key topologyKey
	geo *Geometry
}

type mapTopology struct {
	switches, walls []int16
	links           []int16
	mu              sync.Mutex
	cache           [topologyCacheSize]topologyEntry
	next            int
}

func newMapTopology(m *MapDefinition) *mapTopology {
	t := &mapTopology{links: make([]int16, len(m.Cells))}
	var first [256]int
	for i := range first {
		first[i] = -1
	}
	for i, c := range m.Cells {
		t.links[i] = -1
		if c.Circuit.IsSwitch() {
			t.switches = append(t.switches, int16(i))
		}
		if c.Terrain == BreakableWall {
			t.walls = append(t.walls, int16(i))
		}
		if c.Feature == Link {
			if j := first[c.Group]; j >= 0 {
				t.links[i], t.links[j] = int16(j), int16(i)
			} else {
				first[c.Group] = i
			}
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
	for group := 1; group <= MaxMapGroups; group++ {
		if g.CircuitOn[group] {
			key.circuit |= 1 << (group - 1)
		}
	}
	return
}
