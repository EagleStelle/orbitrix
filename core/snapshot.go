package core

import "fmt"

// Snapshot is a self-contained copy of a game's state: a full resync after a desync, or a saved game.
type Snapshot struct {
	Cols, Rows, Players int
	Turn, Moves         int
	Played              uint16
	Winner              int8
	Count               []uint8
	Owner               []int8
	Map                 *MapDefinition
	Durability          []uint8
	EdgeDurability      []uint8
	CircuitOn           []uint8
	Pressed             []uint8
}

// Snapshot returns a copy of g's state that shares no memory with g.
func (g *Game) Snapshot() Snapshot {
	s := Snapshot{
		Cols: g.Geo.Cols, Rows: g.Geo.Rows, Players: g.Players,
		Turn: g.Turn, Moves: g.Moves, Played: g.Played, Winner: g.Winner,
		Count: append([]uint8(nil), g.Count...), Owner: append([]int8(nil), g.Owner...),
	}
	if g.Map != nil {
		m := g.Map.Clone()
		s.Map = &m
		s.Durability = append([]uint8(nil), g.Durability...)
		s.EdgeDurability = append([]uint8(nil), g.EdgeDurability...)
		s.Pressed = append([]uint8(nil), g.Pressed...)
		for i, on := range g.CircuitOn {
			if on {
				s.CircuitOn = append(s.CircuitOn, uint8(i))
			}
		}
	}
	return s
}

// Restore rebuilds a game from s. s may come from the network, so every field is checked.
func Restore(s Snapshot) (*Game, error) {
	g, err := NewGame(s.Cols, s.Rows, s.Players)
	if s.Map != nil {
		if s.Map.Cols != s.Cols || s.Map.Rows != s.Rows {
			return nil, fmt.Errorf("restore: map dimensions differ")
		}
		g, err = NewGameWithMap(*s.Map, s.Players)
	}
	if err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}
	n := g.Geo.Cells()
	if len(s.Count) != n || len(s.Owner) != n {
		return nil, fmt.Errorf("restore: invalid orb state lengths")
	}
	if g.Map != nil {
		if len(s.Durability) != n || len(s.Pressed) != n || len(s.EdgeDurability) != len(g.Map.Edges) {
			return nil, fmt.Errorf("restore: invalid terrain state lengths")
		}
		var circuitGroups [256]bool
		for i, e := range g.Map.Edges {
			if s.EdgeDurability[i] > 2 || e.Kind != BreakableBarrier && s.EdgeDurability[i] != 0 {
				return nil, fmt.Errorf("restore: invalid terrain state at edge %d", i)
			}
			if e.Kind == Gate {
				circuitGroups[e.Group] = true
			}
		}
		for i, c := range g.Map.Cells {
			if s.Durability[i] > 2 || c.Terrain != BreakableWall && s.Durability[i] != 0 || s.Pressed[i] > 1 || !c.Circuit.IsSwitch() && s.Pressed[i] != 0 {
				return nil, fmt.Errorf("restore: invalid terrain state at cell %d", i)
			}
			if c.Circuit == Hold && (s.Pressed[i] != 0) != (s.Count[i] > 0) {
				return nil, fmt.Errorf("restore: Hold switch state differs from occupancy")
			}
			if c.Circuit.IsOutput() {
				circuitGroups[c.Group] = true
			}
		}
		for _, group := range s.CircuitOn {
			if group == 0 || !circuitGroups[group] || g.CircuitOn[group] {
				return nil, fmt.Errorf("restore: invalid circuit polarity")
			}
			g.CircuitOn[group] = true
		}
		copy(g.Durability, s.Durability)
		copy(g.EdgeDurability, s.EdgeDurability)
		copy(g.Pressed, s.Pressed)
		g.Geo = g.mapGeometry()
	} else if len(s.Durability)+len(s.EdgeDurability)+len(s.Pressed)+len(s.CircuitOn) != 0 {
		return nil, fmt.Errorf("restore: terrain state without map")
	}
	switch {
	case len(s.Count) != n || len(s.Owner) != n:
		return nil, fmt.Errorf("restore: %d counts and %d owners for %d cells", len(s.Count), len(s.Owner), n)
	case s.Turn < 0 || s.Turn >= s.Players:
		return nil, fmt.Errorf("restore: turn %d out of range", s.Turn)
	case s.Moves < 0:
		return nil, fmt.Errorf("restore: %d moves", s.Moves)
	case s.Played>>s.Players != 0:
		return nil, fmt.Errorf("restore: played mask %#x has bits beyond %d players", s.Played, s.Players)
	case s.Winner < NoOwner || int(s.Winner) >= s.Players:
		return nil, fmt.Errorf("restore: winner %d out of range", s.Winner)
	}
	for i := 0; i < n; i++ {
		c, o := s.Count[i], s.Owner[i]
		switch {
		case o < NoOwner || int(o) >= s.Players:
			return nil, fmt.Errorf("restore: cell %d owner %d out of range", i, o)
		case (c == 0) != (o == NoOwner):
			return nil, fmt.Errorf("restore: cell %d has %d orbs and owner %d", i, c, o)
		case !g.Geo.Playable(i) && c != 0 && (g.Map == nil || g.TerrainAt(i) != Floor && g.Map.Cells[i].Circuit != Door):
			return nil, fmt.Errorf("restore: orbs on blocked cell %d", i)
		case g.Map == nil && g.Geo.Playable(i) && c >= 2*g.Geo.Cap[i]:
			return nil, fmt.Errorf("restore: cell %d has %d orbs, capacity %d", i, c, g.Geo.Cap[i])
		}
		if o != NoOwner {
			g.Orbs[o] += int32(c)
		}
		if g.Geo.Playable(i) && c >= g.Geo.Cap[i] {
			g.unstable = true
		}
	}
	copy(g.Count, s.Count)
	copy(g.Owner, s.Owner)
	g.Turn, g.Moves, g.Played, g.Winner = s.Turn, s.Moves, s.Played, s.Winner
	return g, nil
}

// Hash returns a 64-bit FNV-1a digest of the full game state. Peers compare it after each move to detect desync.
func (g *Game) Hash() uint64 {
	const prime = 0x100000001b3
	h := uint64(0xcbf29ce484222325)
	for _, v := range [...]uint64{
		uint64(g.Geo.Cols), uint64(g.Geo.Rows), uint64(g.Players), uint64(g.Turn),
		uint64(g.Moves), uint64(g.Played), uint64(uint8(g.Winner)),
	} {
		for k := 0; k < 64; k += 8 {
			h = (h ^ (v >> k & 0xff)) * prime
		}
	}
	for i, c := range g.Count {
		h = (h ^ uint64(c)) * prime
		h = (h ^ uint64(uint8(g.Owner[i]))) * prime
	}
	if g.Map != nil {
		for i, c := range g.Map.Cells {
			initial := uint8(0)
			if c.InitiallyOpen {
				initial = 1
			}
			for _, v := range []uint8{uint8(c.Terrain), uint8(c.Feature), uint8(c.Circuit), c.Group, initial, g.Durability[i], g.Pressed[i]} {
				h = (h ^ uint64(v)) * prime
			}
		}
		// Edge order and endpoint direction do not change a map's gameplay.
		var edges uint64
		for i, e := range g.Map.Edges {
			a, b := min(e.A, e.B), max(e.A, e.B)
			v := uint64(a) | uint64(b)<<16 | uint64(e.Kind)<<32 | uint64(e.Group)<<40
			if e.InitiallyOpen {
				v |= 1 << 48
			}
			v |= uint64(g.EdgeDurability[i]) << 49
			edges ^= (v ^ 0x9e3779b97f4a7c15) * prime
		}
		h = (h ^ edges) * prime
		for group, on := range g.CircuitOn {
			if on {
				h = (h ^ uint64(group)) * prime
			}
		}
	}
	return h
}
