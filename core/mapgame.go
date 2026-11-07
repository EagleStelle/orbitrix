package core

import (
	"fmt"
	"slices"
)

// TerrainEvent is applied after a wave's orb transfers, before the following wave.
// Cell -1 changes a Gate group; otherwise it changes durability or switch state.
type TerrainEvent struct {
	Wave       int
	Cell       int
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
	g.Durability, g.Pressed = make([]uint8, len(m.Cells)), make([]uint8, len(m.Cells))
	for i, c := range m.Cells {
		if c.Terrain == BreakableWall {
			g.Durability[i] = 2
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
	t := g.Map.Cells[i].Terrain
	if t == BreakableWall && g.Durability[i] == 0 {
		return Floor
	}
	return t
}

// mapGeometry produces an immutable topology. Search copies can share it safely;
// a later Gate change or destruction installs a new topology only in that game.
func (g *Game) mapGeometry() *Geometry {
	m, n := g.Map, len(g.Map.Cells)
	geo := &Geometry{Cols: m.Cols, Rows: m.Rows, Cap: make([]uint8, n), Degree: make([]uint8, n), Nbr: make([]int16, n*MaxConnections)}
	blocked := make([]uint8, n*4)
	for _, e := range m.Edges {
		if e.Kind == Gate && g.OpenGates[e.Group] {
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
	var first [256]int
	for i := range first {
		first[i] = -1
	}
	links := make([]int16, n)
	for i := range links {
		links[i] = -1
	}
	for i, c := range m.Cells {
		if c.Feature == Link {
			if j := first[c.Group]; j >= 0 {
				links[i], links[j] = int16(j), int16(i)
			} else {
				first[c.Group] = i
			}
		}
	}
	for i, c := range m.Cells {
		if g.TerrainAt(i) != Floor {
			continue
		}
		x, y := geo.Coords(i)
		for d, off := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nx, ny := x+off[0], y+off[1]
			if nx < 0 || ny < 0 || nx >= m.Cols || ny >= m.Rows || blocked[4*i+d] != 0 {
				continue
			}
			j := geo.Index(nx, ny)
			if g.TerrainAt(j) == Floor {
				geo.Nbr[MaxConnections*i+int(geo.Degree[i])] = int16(j)
				geo.Degree[i]++
			}
		}
		if links[i] >= 0 {
			geo.Nbr[MaxConnections*i+int(geo.Degree[i])] = links[i]
			geo.Degree[i]++
		}
		geo.Cap[i] = geo.Degree[i]
		if c.Feature == Reservoir {
			geo.Cap[i]++
		}
	}
	return geo
}

// switchInputs applies arrivals and occupancy after a placement or wave. Geometry
// is rebuilt by the caller after all topology changes for that step are known.
func (g *Game) switchInputs(wave int, record bool) bool {
	var open [256]bool
	for i, c := range g.Map.Cells {
		if !c.Feature.IsSwitch() {
			continue
		}
		pressed := g.Pressed[i]
		switch c.Feature {
		case Switch:
			if g.arrived[i] != 0 {
				pressed = 1
			}
		case ToggleSwitch:
			if wave >= 0 && (g.arrived[i] != 0 || slices.Contains(g.cur, int16(i))) {
				pressed ^= 1
			}
		case HoldSwitch:
			pressed = 0
			if g.Count[i] > 0 {
				pressed = 1
			}
		}
		if pressed != g.Pressed[i] {
			g.Pressed[i] = pressed
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: i, Group: c.Group, Switch: true, Pressed: pressed != 0})
			}
		}
		open[c.Group] = pressed != 0
	}
	changed := false
	for group, on := range open {
		if g.OpenGates[group] != on {
			g.OpenGates[group] = on
			changed = true
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: -1, Group: uint8(group), Open: on})
			}
		}
	}
	return changed
}

func (g *Game) terrainWave(wave int, record bool) {
	var damage [MaxSize * MaxSize]uint8
	changed := false
	for _, e := range g.cur {
		i := int(e)
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
	for i, d := range damage[:len(g.Count)] {
		if d > 0 {
			if d >= g.Durability[i] {
				g.Durability[i] = 0
				changed = true
			} else {
				g.Durability[i] -= d
			}
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: i, Durability: g.Durability[i]})
			}
		}
	}
	gatesChanged := g.switchInputs(wave, record)
	if changed || gatesChanged {
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
		case e.Cell < 0:
			changed = changed || g.OpenGates[e.Group] != e.Open
			g.OpenGates[e.Group] = e.Open
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
