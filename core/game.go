package core

import (
	"errors"
	"fmt"
	"math/bits"
)

var (
	ErrGameOver = errors.New("game is over")
	ErrIllegal  = errors.New("illegal move")
)

// Wave lists the cells that explode together in one step of a chain reaction.
type Wave []int16

// Game is the full rules state. Count, Owner, Orbs and the scalar fields are exported for fast reads by the AI
// and the client; change them only through methods, or the per-player tallies stop matching the board.
type Game struct {
	Geo     *Geometry
	Count   []uint8 // orbs in each cell
	Owner   []int8  // owner of each cell, NoOwner when empty
	Players int
	Turn    int               // player to move
	Played  uint16            // bit p is set once player p has had a turn
	Moves   int               // moves applied so far
	Winner  int8              // NoOwner until someone wins
	Orbs    [MaxPlayers]int32 // orbs on the board per player

	// Exploded counts the cell explosions in the last move: a deterministic measure of its cost, which the AI's
	// budget charges for. It isn't part of the game state (not hashed or snapshotted).
	Exploded int

	// unstable is set when a chain hit MaxWaves and left cells at or over capacity. The next move resolves them
	// together with its own explosions.
	unstable bool

	// Scratch reused by every move so the hot path doesn't allocate.
	cur, next []int16 // cells exploding in this wave / the next one
	from      []int8  // owner of each cur cell at the start of the wave
	waveCells []int16 // every recorded wave, back to back
	waveEnds  []int32 // end of each wave in waveCells
	waves     []Wave
}

// NewGame returns an empty cols×rows board for players players, player 0 to move.
func NewGame(cols, rows, players int) (*Game, error) {
	if players < MinPlayers || players > MaxPlayers {
		return nil, fmt.Errorf("%d players: must be %d..%d", players, MinPlayers, MaxPlayers)
	}
	geo, err := NewGeometry(cols, rows)
	if err != nil {
		return nil, err
	}
	// Nothing explodes in the first round, so the last player to move needs a cell nobody has taken yet.
	if players > geo.Cells() {
		return nil, fmt.Errorf("%d players on %d cells: each needs a free cell for their first move", players, geo.Cells())
	}
	g := &Game{Players: players, Winner: NoOwner}
	g.setGeometry(geo)
	for i := range g.Owner {
		g.Owner[i] = NoOwner
	}
	return g, nil
}

// setGeometry sizes the board and scratch for geo. Each wave holds a cell at most once, so N is enough.
func (g *Game) setGeometry(geo *Geometry) {
	g.Geo = geo
	n := geo.Cells()
	if len(g.Count) == n {
		return
	}
	g.Count, g.Owner = make([]uint8, n), make([]int8, n)
	g.cur, g.next, g.from = make([]int16, 0, n), make([]int16, 0, n), make([]int8, 0, n)
}

// CopyFrom makes g an exact copy of src, reusing g's buffers when the board size matches. This, followed by
// Simulate, is the AI's allocation-free way to try a move.
func (g *Game) CopyFrom(src *Game) {
	g.setGeometry(src.Geo)
	copy(g.Count, src.Count)
	copy(g.Owner, src.Owner)
	g.Players, g.Turn, g.Played, g.Moves, g.Winner = src.Players, src.Turn, src.Played, src.Moves, src.Winner
	g.Orbs, g.Exploded, g.unstable = src.Orbs, src.Exploded, src.unstable
}

// Clone returns an independent copy of g.
func (g *Game) Clone() *Game {
	c := new(Game)
	c.CopyFrom(g)
	return c
}

// Over reports whether the game has a winner.
func (g *Game) Over() bool { return g.Winner != NoOwner }

// Alive reports whether player p is still in the game: they have orbs on the board or haven't had a turn yet.
func (g *Game) Alive(p int) bool { return g.Orbs[p] > 0 || g.Played&(1<<p) == 0 }

// Legal reports whether the current player may place an orb in cell: the game isn't over and the cell is
// empty or already theirs.
func (g *Game) Legal(cell int) bool {
	return !g.Over() && cell >= 0 && cell < len(g.Owner) &&
		(g.Owner[cell] == NoOwner || g.Owner[cell] == int8(g.Turn))
}

// LegalMoves appends the current player's legal cells to dst and returns it.
func (g *Game) LegalMoves(dst []int) []int {
	if g.Over() {
		return dst
	}
	p := int8(g.Turn)
	for i, o := range g.Owner {
		if o == NoOwner || o == p {
			dst = append(dst, i)
		}
	}
	return dst
}

// Apply places the current player's orb in cell, resolves every explosion and passes the turn on. It returns the
// waves in order so the client can animate them; the slices are reused by the next Apply.
//
// The chain stops early when a player wins, leaving any remaining cells unresolved. If it reaches MaxWaves
// instead, the cells still at or over capacity explode first in the next move's chain.
func (g *Game) Apply(cell int) ([]Wave, error) {
	g.waveCells, g.waveEnds, g.waves = g.waveCells[:0], g.waveEnds[:0], g.waves[:0]
	if err := g.play(cell, true); err != nil {
		return nil, err
	}
	start := int32(0)
	for _, end := range g.waveEnds {
		g.waves = append(g.waves, Wave(g.waveCells[start:end:end]))
		start = end
	}
	return g.waves, nil
}

// Simulate is Apply without recording waves: the fast path for AI search, usually on a copy made with CopyFrom.
func (g *Game) Simulate(cell int) error { return g.play(cell, false) }

func (g *Game) play(cell int, record bool) error {
	if g.Over() {
		return ErrGameOver
	}
	if !g.Legal(cell) {
		return fmt.Errorf("player %d at cell %d: %w", g.Turn, cell, ErrIllegal)
	}
	p := int8(g.Turn)
	g.Played |= 1 << g.Turn
	g.Moves++
	g.Owner[cell] = p
	g.Count[cell]++
	g.Orbs[p]++

	g.cur = g.cur[:0]
	if g.unstable {
		for i, n := range g.Count {
			if n >= g.Geo.Cap[i] {
				g.cur = append(g.cur, int16(i))
			}
		}
	} else if g.Count[cell] >= g.Geo.Cap[cell] {
		g.cur = append(g.cur, int16(cell))
	}
	g.resolve(record)
	if !g.Over() {
		g.advance()
	}
	return nil
}

// waveLimit is MaxWaves, lowered by tests: a chain that never settles spreads to every cell and wins, so the
// real limit is practically unreachable.
var waveLimit = MaxWaves

// resolve explodes g.cur wave by wave until the board is stable, a player wins or the wave limit is reached.
func (g *Game) resolve(record bool) {
	g.unstable, g.Exploded = false, 0
	for wave := 0; ; wave++ {
		if g.checkWin() || len(g.cur) == 0 {
			return
		}
		if wave == waveLimit {
			g.unstable = true
			return
		}
		if record {
			g.waveCells = append(g.waveCells, g.cur...)
			g.waveEnds = append(g.waveEnds, int32(len(g.waveCells)))
		}
		g.Exploded += len(g.cur)
		g.explode()
		g.cur, g.next = g.next, g.cur
	}
}

// explode resolves one wave: every cell in g.cur explodes at once, and g.next collects the cells at or over
// capacity afterwards.
//
// It runs in two passes so the result doesn't depend on the order of g.cur: first every exploding cell gives
// up its orbs, then every neighbour receives. After the first pass only receiving raises counts, so a cell
// reaches capacity at most once per wave and g.next needs no de-duplication.
func (g *Game) explode() {
	geo := g.Geo
	g.next, g.from = g.next[:0], g.from[:0]
	for _, e := range g.cur {
		p, capacity := g.Owner[e], geo.Cap[e]
		g.from = append(g.from, p)
		g.Count[e] -= capacity
		g.Orbs[p] -= int32(capacity)
		switch {
		case g.Count[e] == 0:
			g.Owner[e] = NoOwner
		case g.Count[e] >= capacity: // only after a MaxWaves cut, when the placed cell can start at 2×capacity
			g.next = append(g.next, e)
		}
	}
	for k, e := range g.cur {
		p := g.from[k]
		for _, n := range geo.Nbr[4*int(e) : 4*int(e)+int(geo.Cap[e])] {
			if o := g.Owner[n]; o != p {
				// Capture: the cell's existing orbs change hands with it.
				if o != NoOwner {
					g.Orbs[o] -= int32(g.Count[n])
				}
				g.Orbs[p] += int32(g.Count[n])
				g.Owner[n] = p
			}
			g.Count[n]++
			g.Orbs[p]++
			if g.Count[n] == geo.Cap[n] {
				g.next = append(g.next, n)
			}
		}
	}
}

// checkWin sets Winner and reports true once at least two players have played and only one has orbs left.
func (g *Game) checkWin() bool {
	if bits.OnesCount16(g.Played) < 2 {
		return false
	}
	winner := NoOwner
	for p := 0; p < g.Players; p++ {
		if g.Orbs[p] > 0 {
			if winner != NoOwner {
				return false
			}
			winner = int8(p)
		}
	}
	g.Winner = winner
	return winner != NoOwner
}

// advance passes the turn to the next alive player. The mover always has orbs, so the loop always ends.
func (g *Game) advance() {
	for i := 1; i <= g.Players; i++ {
		if p := (g.Turn + i) % g.Players; g.Alive(p) {
			g.Turn = p
			return
		}
	}
}
