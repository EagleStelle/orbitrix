package core

import "fmt"

const (
	MinSize    = 3
	MaxSize    = 40
	MinPlayers = 2
	MaxPlayers = 12

	// NoOwner is the owner of an empty cell, and Game.Winner until someone wins.
	NoOwner int8 = -1

	// MaxWaves caps the waves resolved for one move. The immediate-win check normally ends runaway chains long
	// before this; see Game.Apply for what happens if it doesn't.
	MaxWaves = 10000
)

// Geometry holds the immutable per-size tables shared by every Game of that size. Cell i is column i/Rows,
// row i%Rows.
type Geometry struct {
	Cols, Rows int
	// Cap is each cell's capacity, which is also its number of orthogonal neighbours: corner 2, edge 3, interior 4.
	Cap []uint8
	// Nbr holds four slots per cell; cell i's neighbours are Nbr[4*i : 4*i+Cap[i]].
	Nbr []int16
}

// NewGeometry builds the capacity and neighbour tables for a cols×rows board.
func NewGeometry(cols, rows int) (*Geometry, error) {
	if cols < MinSize || cols > MaxSize || rows < MinSize || rows > MaxSize {
		return nil, fmt.Errorf("board %dx%d: each side must be %d..%d", cols, rows, MinSize, MaxSize)
	}
	n := cols * rows
	g := &Geometry{Cols: cols, Rows: rows, Cap: make([]uint8, n), Nbr: make([]int16, 4*n)}
	for i := 0; i < n; i++ {
		c, r := g.Coords(i)
		for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nc, nr := c+d[0], r+d[1]
			if nc >= 0 && nc < cols && nr >= 0 && nr < rows {
				g.Nbr[4*i+int(g.Cap[i])] = int16(g.Index(nc, nr))
				g.Cap[i]++
			}
		}
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
func (g *Geometry) Neighbours(i int) []int16 { return g.Nbr[4*i : 4*i+int(g.Cap[i])] }
