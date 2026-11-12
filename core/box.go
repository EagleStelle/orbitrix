package core

import "fmt"

// Box state bits, per cell in Game.Boxes.
const (
	BoxHere    uint8 = 1 << iota // a Box stands on the cell
	CrateFloor                   // a Box filled this Hole or Landfill, which is Floor now
)

// boxPush is an orb an explosion sent into a Box, held until the Box moves on or sends it back.
type boxPush struct {
	box, from int16
	owner     int8
	dir       int8 // the direction it arrived in (left, right, up, down); -1 across a Link
}

// boxMove is a pushed Box's step: to is where it goes (its own cell when it stays), shove where the orbs on to go
// (-1 for none). dir and shoveDir are the directions they set off in, -1 on a diagonal.
type boxMove struct {
	box, to, shove int16
	dx, dy         int8
	dir, shoveDir  int8
}

var steps = [4][2]int8{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

// What a cell is to a Box, or to the orbs it shoves, arriving there.
const (
	landBlocked = iota
	landPit     // a Hole or unfilled Landfill: a Box fills it, orbs are lost in it
	landFloor
)

// moveBoxes resolves this wave's pushes once its transfers are done, against where Boxes and terrain stood when it
// began. Pushes add up: opposite ones cancel and crossing ones move a Box diagonally. A Box moves one cell (through
// Portals and Springs when straight), shoving the orbs there one cell further, and the orbs that pushed it land where
// it stood. Blocked straight ahead, it bounces back the other way instead. It stays, sending the orbs back to the
// cells that exploded them, when it can't move: its shove or diagonal is blocked, or another move claims a cell it
// needs. A Box that enters a Hole or Landfill fills it, which becomes Floor. It reports whether terrain changed.
func (g *Game) moveBoxes(wave int, record bool) (filled bool) {
	n := len(g.Count)
	if len(g.boxAt) != n {
		g.boxAt, g.claims = make([]int16, n), make([]uint8, n)
		for i := range g.boxAt {
			g.boxAt[i] = -1
		}
	}
	// explode left the held orbs on the Boxes' cells, and in the next wave if they filled it: terrainWave rebuilds it.
	for _, p := range g.pushes {
		if b := p.box; g.Count[b] != 0 {
			g.Orbs[g.Owner[b]] -= int32(g.Count[b])
			g.Count[b], g.Owner[b], g.arrived[b] = 0, NoOwner, 0
		}
	}
	moves := g.boxMoves[:0]
	for _, p := range g.pushes {
		k := g.boxAt[p.box]
		if k < 0 {
			k = int16(len(moves))
			g.boxAt[p.box] = k
			moves = append(moves, boxMove{box: p.box})
		}
		if p.dir >= 0 {
			moves[k].dx += steps[p.dir][0]
			moves[k].dy += steps[p.dir][1]
		}
	}
	for k := range moves {
		g.planBox(&moves[k])
	}
	// A cell two moves claim, or one that also gets orbs back, stops both moves; a stop sends more orbs back.
	for again := true; again; {
		again = false
		clear(g.claims)
		for _, p := range g.pushes {
			if m := moves[g.boxAt[p.box]]; m.to == m.box {
				g.claims[p.from] = 1
			}
		}
		for _, m := range moves {
			if m.to != m.box {
				g.claims[m.to] += 2
				if m.shove >= 0 {
					g.claims[m.shove] += 2
				}
			}
		}
		for k := range moves {
			if m := &moves[k]; m.to != m.box && (g.claims[m.to] > 2 || m.shove >= 0 && g.claims[m.shove] > 2) {
				m.to, m.shove, again = m.box, -1, true
			}
		}
	}
	for _, p := range g.pushes {
		at := int(p.from)
		if m := moves[g.boxAt[p.box]]; m.to != m.box {
			at = int(p.box)
			g.arrived[at] = 1
		}
		g.gain(at, p.owner, 1)
	}
	for _, m := range moves {
		g.boxAt[m.box] = -1
		if m.to == m.box {
			continue
		}
		to := int(m.to)
		if m.shove >= 0 {
			s, c, o := int(m.shove), g.Count[to], g.Owner[to]
			g.Count[to], g.Owner[to] = 0, NoOwner
			g.Orbs[o] -= int32(c)
			switch g.TerrainAt(s) {
			case Floor:
				g.gain(s, o, c)
				g.arrived[s] = 1
			case Landfill: // absorbed as fill, like orbs sent into it
				g.Durability[s] -= min(c, g.Durability[s])
				filled = filled || g.Durability[s] == 0
				if record {
					g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: s, Durability: g.Durability[s]})
				}
			}
		}
		if record {
			g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: int(m.box), Box: true, To: to, Shove: int(m.shove), Dir: m.dir, ShoveDir: m.shoveDir})
		}
		g.Boxes[m.box] &^= BoxHere
		if g.landing(to) == landPit {
			g.Boxes[to] |= CrateFloor
			g.Durability[to] = 0 // a Landfill is complete; a Hole or collapsed Sinkhole has none
			filled = true
		} else {
			g.Boxes[to] |= BoxHere
			g.arrived[to] = 1 // presses a switch
		}
	}
	g.boxMoves = moves
	return filled
}

// planBox sets where m's Box goes and where it shoves the orbs there.
func (g *Game) planBox(m *boxMove) {
	m.dx, m.dy = sign(m.dx), sign(m.dy)
	m.to, m.shove, m.dir, m.shoveDir = m.box, -1, -1, -1
	b := int(m.box)
	if m.dx != 0 && m.dy != 0 { // only the target cell matters, and nothing bounces
		to := g.diagonal(b, m.dx, m.dy)
		land := g.boxLanding(to)
		if land == landFloor && g.Count[to] != 0 {
			s := g.diagonal(to, m.dx, m.dy)
			if g.landing(s) == landBlocked {
				return
			}
			m.shove = int16(s)
		}
		if land != landBlocked {
			m.to = int16(to)
		}
		return
	}
	if m.dx == 0 && m.dy == 0 {
		return
	}
	d := 0
	switch {
	case m.dx > 0:
		d = 1
	case m.dy < 0:
		d = 2
	case m.dy > 0:
		d = 3
	}
	for _, d := range [2]int{d, d ^ 1} { // ahead, then bouncing back
		to, arrive := g.travel(b, d)
		land := g.boxLanding(to)
		if land == landBlocked {
			continue
		}
		if land == landFloor && g.Count[to] != 0 {
			s, _ := g.travel(to, arrive)
			if s == to || g.landing(s) == landBlocked { // a Spring sending the orbs straight back counts as blocked
				return
			}
			m.shove, m.shoveDir = int16(s), int8(arrive)
		}
		m.to, m.dir = int16(to), int8(d)
		return
	}
}

// landing classifies cell i for a Box or shoved orbs arriving there, as the wave began. A Portal or Spring is only
// reached diagonally, which can't use them.
func (g *Game) landing(i int) int {
	if i < 0 || g.Boxes[i]&BoxHere != 0 {
		return landBlocked
	}
	switch g.TerrainAt(i) {
	case Hole, Landfill:
		return landPit
	case Floor:
		if f := g.Map.Cells[i].Feature; f != Portal && f != Spring {
			return landFloor
		}
	}
	return landBlocked
}

// boxLanding is landing for the Box itself: a Vault's raised shield, as this wave's hits left it, stops it like a
// Wall. Shoved orbs still reach the shield, which takes what it can.
func (g *Game) boxLanding(i int) int {
	if i >= 0 && g.Pressed[i] >= vaultShield {
		return landBlocked
	}
	return g.landing(i)
}

// travel follows a straight move from cell i on the current topology, through Portals and Springs.
func (g *Game) travel(i, d int) (int, int) {
	j, arrive, _ := g.Map.travel(i, d, g.topology.pairs, g.Geo.Blocked, g.Geo.PortalAxes, nil)
	return j, arrive
}

func (g *Game) diagonal(i int, dx, dy int8) int {
	c, r := g.Geo.Coords(i)
	c, r = c+int(dx), r+int(dy)
	if c < 0 || r < 0 || c >= g.Geo.Cols || r >= g.Geo.Rows {
		return -1
	}
	return g.Geo.Index(c, r)
}

// BoxRoute appends to dst (which must not be nil) the Portal and Spring hops, as entry/exit pairs, of a straight Box
// move or shove from cell in direction d (a TerrainEvent's Dir or ShoveDir) on the current topology: the one its wave
// began with on a replay.
func (g *Game) BoxRoute(cell, d int, dst []int16) []int16 {
	_, _, dst = g.Map.travel(cell, d, g.topology.pairs, g.Geo.Blocked, g.Geo.PortalAxes, dst)
	return dst
}

// restoreBoxes installs a snapshot's Box state on g, fresh from NewGameWithMap with the rest of its terrain restored,
// checking it: no more Boxes than the map starts with, each standing empty on ground it can reach (a closed Door holds
// one dormant) or filling a Hole, an unfinished Landfill or a collapsed Sinkhole.
func (g *Game) restoreBoxes(boxes, count []uint8) error {
	if len(boxes) != len(g.Boxes) {
		return fmt.Errorf("restore: invalid Box state length")
	}
	left := 0
	for _, b := range g.Boxes {
		left += int(b) // BoxHere where each starts
	}
	copy(g.Boxes, boxes)
	for i, b := range boxes {
		c := g.Map.Cells[i]
		t, here, crate := c.Terrain, b&BoxHere != 0, b&CrateFloor != 0
		if b&^(BoxHere|CrateFloor) != 0 || crate && (t != Hole && t != Landfill && t != Sinkhole || g.Durability[i] != 0) ||
			here && (count[i] != 0 || c.Feature == Portal || c.Feature == Spring || g.TerrainAt(i) != Floor && c.Circuit != Door) {
			return fmt.Errorf("restore: invalid Box state at cell %d", i)
		}
		if here || crate {
			left--
		}
	}
	if left < 0 {
		return fmt.Errorf("restore: more Boxes than the map has")
	}
	return nil
}

// gain adds c of owner's orbs to cell i, capturing it as arriving orbs do; a Vault's shield takes what it can.
func (g *Game) gain(i int, owner int8, c uint8) {
	for hit := true; hit && c > 0; {
		if hit, g.Pressed[i] = Shielded(g.Pressed[i], owner); hit {
			c--
		}
	}
	if c == 0 {
		return
	}
	if o := g.Owner[i]; o != owner {
		if o != NoOwner {
			g.Orbs[o] -= int32(g.Count[i])
		}
		g.Orbs[owner] += int32(g.Count[i])
		g.Owner[i] = owner
	}
	g.Count[i] += c
	g.Orbs[owner] += int32(c)
}

func sign(v int8) int8 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// collectPushes finds the orbs this wave's explosions sent into Boxes. explode lands them on the Boxes' cells like any
// orb, keeping its loop as fast on classic boards, and moveBoxes takes them back.
func (g *Game) collectPushes() {
	g.pushes = g.pushes[:0]
	geo := g.Geo
	push := func(k int, e int16, slot int) {
		if n := geo.Nbr[MaxConnections*int(e)+slot]; g.Boxes[n]&BoxHere != 0 {
			g.pushes = append(g.pushes, boxPush{box: n, from: e, owner: g.from[k], dir: geo.Dirs[MaxConnections*int(e)+slot]})
		}
	}
	for k, e := range g.cur {
		for slot := range geo.Degree[e] {
			push(k, e, int(slot))
		}
		if geo.Outlets != nil && geo.Outlets[e] != 0 {
			push(k, e, geo.LastOutlet(int(e), g.Pressed[e])) // its extra orb
		}
	}
}
