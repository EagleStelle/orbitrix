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
}

// Snapshot returns a copy of g's state that shares no memory with g.
func (g *Game) Snapshot() Snapshot {
	return Snapshot{
		Cols: g.Geo.Cols, Rows: g.Geo.Rows, Players: g.Players,
		Turn: g.Turn, Moves: g.Moves, Played: g.Played, Winner: g.Winner,
		Count: append([]uint8(nil), g.Count...), Owner: append([]int8(nil), g.Owner...),
	}
}

// Restore rebuilds a game from s. s may come from the network, so every field is checked.
func Restore(s Snapshot) (*Game, error) {
	g, err := NewGame(s.Cols, s.Rows, s.Players)
	if err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}
	n := g.Geo.Cells()
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
		case c >= 2*g.Geo.Cap[i]: // explosions never push a cell past 2×capacity−1
			return nil, fmt.Errorf("restore: cell %d has %d orbs, capacity %d", i, c, g.Geo.Cap[i])
		}
		if o != NoOwner {
			g.Orbs[o] += int32(c)
		}
		if c >= g.Geo.Cap[i] {
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
	return h
}
