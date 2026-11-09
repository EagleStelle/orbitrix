package core

import "fmt"

const (
	MinSize        = 3
	MaxSize        = 40
	MinPlayers     = 2
	MaxPlayers     = 12
	MaxConnections = 5 // four cardinal routes and one Link

	// NoOwner is the owner of an empty cell, and Game.Winner until someone wins.
	NoOwner int8 = -1

	// MaxWaves caps the waves resolved for one move. The immediate-win check normally ends runaway chains long
	// before this; see Game.Apply for what happens if it doesn't.
	MaxWaves = 10000
)

// Geometry holds immutable connection tables, shared safely by game copies. Cell i is column i/Rows,
// row i%Rows.
type Geometry struct {
	Cols, Rows int
	// Cap is the explosion threshold; blocked terrain has zero capacity.
	Cap    []uint8
	Degree []uint8 // outgoing connections, independent of Reservoir capacity
	// Holes counts connections into Hole terrain, stored in the Nbr slots after the Degree active ones. Nil
	// without Holes.
	Holes []uint8
	// Nbr holds MaxConnections slots per cell, of which Degree are active.
	Nbr []int16
	// PortalRoutes holds entry/exit pairs per connection, only on maps with active Portal routes.
	PortalRoutes [][]int16
	PortalAxes   []PortalAxes
}

// NewGeometry builds the capacity and neighbour tables for a cols×rows board.
func NewGeometry(cols, rows int) (*Geometry, error) {
	if cols < MinSize || cols > MaxSize || rows < MinSize || rows > MaxSize {
		return nil, fmt.Errorf("board %dx%d: each side must be %d..%d", cols, rows, MinSize, MaxSize)
	}
	n := cols * rows
	g := &Geometry{Cols: cols, Rows: rows, Cap: make([]uint8, n), Degree: make([]uint8, n), Nbr: make([]int16, MaxConnections*n)}
	for i := 0; i < n; i++ {
		c, r := g.Coords(i)
		for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nc, nr := c+d[0], r+d[1]
			if nc >= 0 && nc < cols && nr >= 0 && nr < rows {
				g.Nbr[MaxConnections*i+int(g.Cap[i])] = int16(g.Index(nc, nr))
				g.Cap[i]++
			}
		}
		g.Degree[i] = g.Cap[i]
	}
	return g, nil
}

// Cells returns the number of cells on the board.
func (g *Geometry) Cells() int { return len(g.Cap) }

// Index returns the cell index of column c, row r.
func (g *Geometry) Index(c, r int) int { return c*g.Rows + r }

// Coords returns the column and row of cell i.
func (g *Geometry) Coords(i int) (c, r int) { return i / g.Rows, i % g.Rows }

// Neighbours returns cell i's orthogonal neighbours. The slice aliases the table: don't modify it.
func (g *Geometry) Neighbours(i int) []int16 {
	return g.Nbr[MaxConnections*i : MaxConnections*i+int(g.Degree[i])]
}

// Targets returns where cell i's exploding orbs go, one each: its neighbours, then its Holes.
func (g *Geometry) Targets(i int) []int16 {
	n := int(g.Degree[i])
	if g.Holes != nil {
		n += int(g.Holes[i])
	}
	return g.Nbr[MaxConnections*i : MaxConnections*i+n]
}

func (g *Geometry) PortalPath(cell, connection int) []int16 {
	if g.PortalRoutes == nil {
		return nil
	}
	return g.PortalRoutes[MaxConnections*cell+connection]
}

// Playable excludes blocked terrain and Portals, including an empty destroyed wall as Floor.
func (g *Geometry) Playable(i int) bool { return i >= 0 && i < g.Cells() && g.Cap[i] != 0 }
