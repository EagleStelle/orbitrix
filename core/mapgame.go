package core

import (
	"fmt"
	"slices"
)

// TerrainEvent is applied after a wave's orb transfers, before the following wave.
// Cell -1 records circuit polarity in Open. Edge marks Cell as an edge index;
// otherwise Cell indexes a cell's durability or switch state.
type TerrainEvent struct {
	Wave       int
	Cell       int
	Edge       bool
	Durability uint8
	Group      uint8
	Switch     bool
	Pressed    bool
	Open       bool
}

// NewGameWithMap starts a match from a private copy of a playable design.
func NewGameWithMap(m MapDefinition, players int) (*Game, error) {
	if err := m.ValidateStructure(); err != nil {
		return nil, err
	}
	if issues := m.Validate(); len(issues) != 0 {
		return nil, fmt.Errorf("map: %s", issues[0].Message)
	}
	if players > m.FloorCount() {
		return nil, fmt.Errorf("map: %d players need at least %d Floor cells", players, players)
	}
	g, err := NewGame(m.Cols, m.Rows, players)
	if err != nil {
		return nil, err
	}
	m = m.Clone()
	g.Map = &m
	g.topology = newMapTopology(&m)
	g.Durability, g.Pressed = make([]uint8, len(m.Cells)), make([]uint8, len(m.Cells))
	for i, c := range m.Cells {
		if c.Terrain == BreakableWall || c.Terrain == Landfill || c.Terrain == BreakableFloor {
			g.Durability[i] = 2
		}
	}
	g.EdgeDurability = make([]uint8, len(m.Edges))
	for i, e := range m.Edges {
		if e.Kind == BreakableBarrier {
			g.EdgeDurability[i] = 2
		}
	}
	g.Geo = g.mapGeometry()
	return g, nil
}

// TerrainAt includes destruction, whereas Map always retains the saved design.
func (g *Game) TerrainAt(i int) Terrain {
	if g.Map == nil {
		return Floor
	}
	c := g.Map.Cells[i]
	t := c.Terrain
	if c.Circuit == Door && !CircuitOpen(c.InitiallyOpen, g.CircuitOn[c.Group]) {
		return Wall
	}
	switch {
	case (t == BreakableWall || t == Landfill) && g.Durability[i] == 0:
		return Floor
	case t == BreakableFloor && g.Durability[i] == 0:
		return Void
	case t == BreakableFloor:
		return Floor
	}
	return t
}

// mapGeometry produces an immutable topology. Search copies can share it safely;
// a later Gate change or destruction installs a new topology only in that game.
func (g *Game) mapGeometry() *Geometry {
	t := g.topology
	key := g.topologyKey()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, entry := range t.cache {
		if entry.geo != nil && entry.key == key {
			return entry.geo
		}
	}
	m, n := g.Map, len(g.Map.Cells)
	geo := &Geometry{Cols: m.Cols, Rows: m.Rows, Cap: make([]uint8, n), Degree: make([]uint8, n), Nbr: make([]int16, n*MaxConnections)}
	var blocked [MaxSize * MaxSize * 4]uint8
	for i, e := range m.Edges {
		if e.Kind == Gate && CircuitOpen(e.InitiallyOpen, g.CircuitOn[e.Group]) || e.Kind == BreakableBarrier && g.EdgeDurability[i] == 0 {
			continue
		}
		d := 0
		switch e.B - e.A {
		case -m.Rows:
			d = 0
		case m.Rows:
			d = 1
		case -1:
			d = 2
		case 1:
			d = 3
		}
		blocked[4*e.A+d], blocked[4*e.B+(d^1)] = 1, 1
	}
	geo.PortalAxes = m.portalAxes(t.pairs, blocked[:], func(i int) bool { return g.TerrainAt(i) == Floor || g.TerrainAt(i) == Landfill }, 0)
	for i, c := range m.Cells {
		if g.TerrainAt(i) != Floor || c.Feature == Portal {
			continue
		}
		var holes [5]int16
		var holePaths [5][]int16
		nholes, outlets := 0, uint8(0)
		for d := range 4 {
			var scratch [4 * MaxMapGroups]int16
			j, path := m.route(i, d, t.pairs, blocked[:], geo.PortalAxes, scratch[:0])
			if j >= 0 && (g.TerrainAt(j) == Hole || g.TerrainAt(j) == Landfill) {
				holes[nholes] = int16(j)
				holePaths[nholes] = slices.Clone(path)
				nholes++
			}
			if j >= 0 && g.TerrainAt(j) == Floor {
				slot := MaxConnections*i + int(geo.Degree[i])
				geo.Nbr[slot] = int16(j)
				if len(path) != 0 {
					if geo.PortalRoutes == nil {
						geo.PortalRoutes = make([][]int16, n*MaxConnections)
					}
					geo.PortalRoutes[slot] = slices.Clone(path)
				}
				geo.Degree[i]++
				outlets |= 1 << d
			}
		}
		if c.Feature == Link && t.pairs[i] >= 0 {
			geo.Nbr[MaxConnections*i+int(geo.Degree[i])] = t.pairs[i]
			geo.Degree[i]++
		}
		if c.Feature == Spillway {
			// Its extra orb drops into its own pit: a Hole target at itself. No Link, so at most 5 targets.
			holes[nholes] = int16(i)
			nholes++
		}
		if nholes > 0 {
			if geo.Holes == nil {
				geo.Holes = make([]uint8, n)
			}
			copy(geo.Nbr[MaxConnections*i+int(geo.Degree[i]):], holes[:nholes])
			geo.Holes[i] = uint8(nholes)
			for h, path := range holePaths[:nholes] {
				if len(path) != 0 {
					if geo.PortalRoutes == nil {
						geo.PortalRoutes = make([][]int16, n*MaxConnections)
					}
					geo.PortalRoutes[MaxConnections*i+int(geo.Degree[i])+h] = path
				}
			}
		}
		geo.Cap[i] = geo.Degree[i] + uint8(nholes)
		if c.Feature == Reservoir && geo.Cap[i] > 0 { // shut in by Gates, it holds its orbs like Floor
			geo.Cap[i]++ // its extra orb
			if geo.Outlets == nil {
				geo.Outlets = make([]uint8, n)
			}
			geo.Outlets[i] = outlets
		}
	}
	t.cache[t.next] = topologyEntry{key: key, geo: geo}
	t.next = (t.next + 1) % len(t.cache)
	return geo
}

func (g *Game) terrainWave(wave int, record bool) {
	var damage [MaxSize * MaxSize]uint8
	changed := false
	if len(g.topology.walls) != 0 {
		for _, e := range g.cur {
			i := int(e)
			if g.Map.Cells[i].Terrain == BreakableFloor && g.Durability[i] > 0 {
				damage[i]++ // only its own explosions crack it
			}
			for _, target := range g.Geo.Targets(i)[g.Geo.Degree[i]:] {
				if g.Map.Cells[target].Terrain == Landfill && g.Durability[target] > 0 {
					damage[target]++
				}
			}
			x, y := g.Geo.Coords(i)
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && ny >= 0 && nx < g.Geo.Cols && ny < g.Geo.Rows {
					j := g.Geo.Index(nx, ny)
					if g.Map.Cells[j].Terrain == BreakableWall && g.Durability[j] > 0 {
						damage[j]++
					}
				}
			}
		}
		for _, cell := range g.topology.walls {
			i, d := int(cell), damage[cell]
			if d > 0 {
				if d >= g.Durability[i] {
					g.Durability[i] = 0
					changed = true
					if g.Count[i] > 0 && g.Map.Cells[i].Terrain == BreakableFloor { // its orbs fall into the new Void
						g.Orbs[g.Owner[i]] -= int32(g.Count[i])
						g.Count[i], g.Owner[i] = 0, NoOwner
					}
				} else {
					g.Durability[i] -= d
				}
				if record {
					g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: i, Durability: g.Durability[i]})
				}
			}
		}
	}
	if len(g.topology.barriers) != 0 {
		var exploding [MaxSize * MaxSize]bool
		for _, i := range g.cur {
			exploding[i] = true
		}
		for _, i := range g.topology.barriers {
			if g.EdgeDurability[i] == 0 {
				continue
			}
			e := g.Map.Edges[i]
			d := uint8(0)
			for _, endpoint := range [2]int{e.A, e.B} {
				if exploding[endpoint] {
					d++
				}
			}
			if d == 0 {
				continue
			}
			g.EdgeDurability[i] -= min(d, g.EdgeDurability[i])
			changed = changed || g.EdgeDurability[i] == 0
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: int(i), Edge: true, Durability: g.EdgeDurability[i]})
			}
		}
	}
	circuitsChanged := g.circuitInputs(wave, record)
	if changed || circuitsChanged {
		g.Geo = g.mapGeometry()
		g.next = g.next[:0]
		for i, c := range g.Count {
			if g.Geo.Playable(i) && c >= g.Geo.Cap[i] {
				g.next = append(g.next, int16(i))
			}
		}
	}
}

// ReplayTerrain applies recorded presentation events to a separate replay state.
func (g *Game) ReplayTerrain(events []TerrainEvent) {
	if g.Map == nil || len(events) == 0 {
		return
	}
	changed := false
	for _, e := range events {
		switch {
		case e.Edge:
			changed = changed || g.EdgeDurability[e.Cell] != 0 && e.Durability == 0
			g.EdgeDurability[e.Cell] = e.Durability
		case e.Cell < 0:
			changed = changed || g.CircuitOn[e.Group] != e.Open
			g.CircuitOn[e.Group] = e.Open
		case e.Switch:
			g.Pressed[e.Cell] = 0
			if e.Pressed {
				g.Pressed[e.Cell] = 1
			}
		default:
			changed = changed || g.Durability[e.Cell] != 0 && e.Durability == 0
			g.Durability[e.Cell] = e.Durability
		}
	}
	if changed {
		g.Geo = g.mapGeometry()
	}
}
