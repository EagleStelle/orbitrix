package core

import "fmt"

// Fog of war (add-on): a player sees the cells within one step, diagonals included, of every cell holding their orbs
// or a teammate's. Nothing blocks sight. After their first orb, which goes anywhere, a player places only where they
// see. The rules resolve the whole board as ever: the client hides what its player doesn't see, and Believed hides it
// from a bot. Out of sight, each side keeps the map as it last saw it (Memory).

// SetFog makes g a fog of war game before its first move.
func (g *Game) SetFog() error {
	if g.Moves != 0 || g.Played != 0 {
		return fmt.Errorf("fog: the game has started")
	}
	g.setFog()
	return nil
}

// setFog turns the fog on, with what every side sees now as the first thing it has seen.
func (g *Game) setFog() {
	g.Fog, g.Memory = true, Memory{}
	g.look()
}

// look adds what every side sees now to its memory.
func (g *Game) look() {
	if len(g.sight) != len(g.Count) {
		g.sight = make([]uint16, len(g.Count))
	}
	clear(g.sight)
	g.Observe(g.Geo, g.sight)
	g.Memory.See(g, &g.Cells, g.sight)
}

// Side is who p sees and wins with: their team in a team game, else p alone.
func (c *Cells) Side(p int8) int8 {
	if c.Mix != nil {
		return int8(c.Team[p])
	}
	return p
}

// Sees reports whether p sees cell i of a geo board: an orb of theirs or a teammate's is on it or next to it.
func (c *Cells) Sees(geo *Geometry, i int, p int8) bool {
	col, row := geo.Coords(i)
	for x := max(col-1, 0); x <= min(col+1, geo.Cols-1); x++ {
		for y := max(row-1, 0); y <= min(row+1, geo.Rows-1); y++ {
			if o := c.Owner[x*geo.Rows+y]; o != NoOwner && c.Allied(o, p) {
				return true
			}
		}
	}
	return false
}

// Observe sets in seen, a bit per Side for each cell, every cell that side sees now, and leaves the bits already set.
func (c *Cells) Observe(geo *Geometry, seen []uint16) {
	for j, o := range c.Owner {
		if o == NoOwner {
			continue
		}
		bit := uint16(1) << c.Side(o)
		col, row := geo.Coords(j)
		for x := max(col-1, 0); x <= min(col+1, geo.Cols-1); x++ {
			for y := max(row-1, 0); y <= min(row+1, geo.Rows-1); y++ {
				seen[x*geo.Rows+y] |= bit
			}
		}
	}
}

// Reach reports whether the fog lets p place in cell i: always without fog and for p's first orb, else only where p
// sees.
func (g *Game) Reach(i int, p int8) bool {
	return !g.Fog || g.Played&(1<<p) == 0 || g.Sees(g.Geo, i, p)
}

// Memory is what each side has seen: Seen, a bit per Side for every cell it has seen, and on a map each cell and edge
// as that side last saw it. Orbs are kept only for the ink of the devices they held.
type Memory struct {
	Seen  []uint16
	sides [MaxPlayers]*recall
}

// recall is one side's map as it last saw it.
type recall struct {
	durability, pressed, boxes, count []uint8
	owner                             []int8
	open                              []bool // a circuit output was open
	edgeDurability                    []uint8
	edgeOpen                          []bool // a Gate was open
}

// See remembers every cell each side sees in sight (a bit per Side): its state on g's map and, from cells, its orbs.
// g may be nil or a classic board, which have nothing to remember but Seen. It returns the sides whose memory of the
// map changed.
func (m *Memory) See(g *Game, cells *Cells, sight []uint16) (changed uint16) {
	if m.Seen == nil {
		m.Seen = make([]uint16, len(sight))
	}
	for i, bits := range sight {
		m.Seen[i] |= bits
	}
	if g == nil || g.Map == nil {
		return 0
	}
	for i, bits := range sight {
		c := g.Map.Cells[i]
		open, boxes := c.Circuit.IsOutput() && CircuitOpen(c.InitiallyOpen, g.CircuitOn[c.Group]), uint8(0)
		if len(g.Boxes) != 0 {
			boxes = g.Boxes[i]
		}
		owner, count := NoOwner, uint8(0)
		if c.Feature != NoFeature { // only a device shows whose orbs it held
			owner, count = cells.Owner[i], cells.Count[i]
		}
		for s := range MaxPlayers {
			if bits&(1<<s) == 0 {
				continue
			}
			r := m.side(s, g)
			if r.durability[i] != g.Durability[i] || r.pressed[i] != g.Pressed[i] || r.boxes[i] != boxes ||
				r.owner[i] != owner || r.count[i] != count || r.open[i] != open {
				changed |= 1 << s
			}
			r.durability[i], r.pressed[i], r.boxes[i], r.owner[i], r.count[i], r.open[i] = g.Durability[i], g.Pressed[i], boxes, owner, count, open
		}
	}
	for k, e := range g.Map.Edges {
		open := e.Kind == Gate && CircuitOpen(e.InitiallyOpen, g.CircuitOn[e.Group])
		bits := sight[e.A] | sight[e.B]
		for s := range MaxPlayers {
			if bits&(1<<s) == 0 {
				continue
			}
			r := m.side(s, g)
			if r.edgeDurability[k] != g.EdgeDurability[k] || r.edgeOpen[k] != open {
				changed |= 1 << s
			}
			r.edgeDurability[k], r.edgeOpen[k] = g.EdgeDurability[k], open
		}
	}
	return changed
}

// side is side s's memory of g's map, made on first use.
func (m *Memory) side(s int, g *Game) *recall {
	if m.sides[s] == nil {
		n, e := len(g.Map.Cells), len(g.Map.Edges)
		m.sides[s] = &recall{durability: make([]uint8, n), pressed: make([]uint8, n), boxes: make([]uint8, n),
			count: make([]uint8, n), owner: make([]int8, n), open: make([]bool, n), edgeDurability: make([]uint8, e),
			edgeOpen: make([]bool, e)}
	}
	return m.sides[s]
}

// Orbs are the orbs side s last saw on each device, nil if it has seen none of the map: only the ink they gave it shows.
func (m *Memory) Orbs(s int8) (owner []int8, count []uint8) {
	if r := m.sides[s]; r != nil {
		return r.owner, r.count
	}
	return nil, nil
}

// CopyFrom makes m a copy of src.
func (m *Memory) CopyFrom(src *Memory) {
	if src.Seen == nil {
		*m = Memory{}
		return
	}
	m.Seen = append(m.Seen[:0], src.Seen...)
	for s, r := range src.sides {
		if r == nil {
			m.sides[s] = nil
			continue
		}
		if m.sides[s] == nil {
			m.sides[s] = new(recall)
		}
		d := m.sides[s]
		d.durability, d.pressed = append(d.durability[:0], r.durability...), append(d.pressed[:0], r.pressed...)
		d.boxes, d.count = append(d.boxes[:0], r.boxes...), append(d.count[:0], r.count...)
		d.owner, d.open = append(d.owner[:0], r.owner...), append(d.open[:0], r.open...)
		d.edgeDurability, d.edgeOpen = append(d.edgeDurability[:0], r.edgeDurability...), append(d.edgeOpen[:0], r.edgeOpen...)
	}
}

// Recall makes dst g's map as player p's side remembers it in m, with g's orbs: each cell and edge as the side last
// saw it, with circuits set as they were then, and cells never seen plain Floor without the Barriers and Gates between
// two of them. dst has no memory of its own. Classic boards have nothing to recall: dst is a copy.
func (g *Game) Recall(dst *Game, m *Memory, p int8) {
	dst.CopyFrom(g)
	dst.Memory, dst.sight = Memory{}, nil
	s := g.Side(p)
	r := m.sides[s]
	if g.Map == nil {
		return
	}
	bit := uint16(1) << s
	seen := func(i int) bool { return r != nil && m.Seen[i]&bit != 0 }
	md := g.Map.Clone()
	md.Edges, dst.EdgeDurability = md.Edges[:0], dst.EdgeDurability[:0]
	for k, e := range g.Map.Edges {
		if seen(e.A) || seen(e.B) {
			if e.Kind == Gate {
				e.InitiallyOpen = r.edgeOpen[k]
			}
			md.Edges, dst.EdgeDurability = append(md.Edges, e), append(dst.EdgeDurability, r.edgeDurability[k])
		}
	}
	dst.CircuitOn = [256]bool{} // outputs start as remembered, and a switch flipped in thought flips them from there
	for i := range md.Cells {
		boxes := uint8(0)
		if seen(i) {
			dst.Durability[i], dst.Pressed[i], boxes = r.durability[i], r.pressed[i], r.boxes[i]
			if md.Cells[i].Circuit.IsOutput() {
				md.Cells[i].InitiallyOpen = r.open[i]
			}
		} else {
			md.Cells[i], dst.Durability[i], dst.Pressed[i] = MapCell{}, 0, 0
		}
		if len(dst.Boxes) != 0 {
			dst.Boxes[i] = boxes
		}
	}
	// A half-known map plays as it reads: a Portal or Link whose pair is unseen leads nowhere, and a Spring whose Wall
	// is unseen is shut.
	dst.Map, dst.topology = &md, newMapTopology(&md)
	dst.Geo = dst.mapGeometry()
}

// Believed makes dst the game as player p knows it, for a bot to think on: the map as p's side remembers it (Recall),
// and only the orbs p sees. A rival whose orbs are all out of sight counts as yet to have a turn, so the game never
// looks won. dst keeps the fog's placement rule but remembers nothing as it plays, so search copies cost nothing more.
// Without fog dst is a plain copy.
func (g *Game) Believed(dst *Game, p int8) {
	if !g.Fog {
		dst.CopyFrom(g)
		return
	}
	g.Recall(dst, &g.Memory, p)
	for i, n := range dst.Count {
		if n == 0 || g.Sees(g.Geo, i, p) {
			continue
		}
		dst.Lift(i, nil)
		if g.Map != nil && (dst.Map.Cells[i].Feature.IsMaker() || dst.Map.Cells[i].Circuit == Hold) {
			dst.Pressed[i] = 0 // as empty: no holder, and no press for the circuit to answer
		}
	}
	for q := range g.Players {
		if g.Orbs[q] > 0 && dst.Orbs[q] == 0 {
			dst.Played &^= 1 << q
		}
	}
}
