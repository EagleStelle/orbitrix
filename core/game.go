package core

import (
	"errors"
	"fmt"
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
	Geo            *Geometry
	Count          []uint8 // orbs in each cell
	Owner          []int8  // owner of each cell, NoOwner when empty
	Players        int
	Turn           int               // player to move
	Played         uint16            // bit p is set once player p has had a turn
	Moves          int               // moves applied so far
	Winner         int8              // NoOwner until someone wins
	Orbs           [MaxPlayers]int32 // orbs on the board per player
	Map            *MapDefinition    // immutable design; nil is a classic rectangle
	Durability     []uint8
	EdgeDurability []uint8   // indexed by Map.Edges; only BreakableBarrier uses durability
	CircuitOn      [256]bool // polarity: outputs reverse their individual starting states
	Pressed        []uint8
	TerrainEvents  []TerrainEvent // events in the last recorded move
	topology       *mapTopology   // immutable designs and bounded, shared connection tables

	// Exploded counts the cell explosions in the last move: a deterministic measure of its cost, which the AI's
	// budget charges for. It isn't part of the game state (not hashed or snapshotted).
	Exploded int

	// unstable is set when a chain hit MaxWaves and left cells at or over capacity. The next move resolves them
	// together with its own explosions.
	unstable bool

	// Scratch reused by every move so the hot path doesn't allocate.
	cur, next []int16 // cells exploding in this wave / the next one
	arrived   []uint8 // at least one orb received in this placement or wave
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
	g.arrived = make([]uint8, n)
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
	g.Map, g.CircuitOn = src.Map, src.CircuitOn
	g.topology = src.topology
	g.Durability = append(g.Durability[:0], src.Durability...)
	g.EdgeDurability = append(g.EdgeDurability[:0], src.EdgeDurability...)
	g.Pressed = append(g.Pressed[:0], src.Pressed...)
	g.TerrainEvents = g.TerrainEvents[:0]
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
	return !g.Over() && g.Geo.Playable(cell) && g.Count[cell] < 255 &&
		(g.Owner[cell] == NoOwner || g.Owner[cell] == int8(g.Turn))
}

// LegalMoves appends the current player's legal cells to dst and returns it.
func (g *Game) LegalMoves(dst []int) []int {
	if g.Over() {
		return dst
	}
	p := int8(g.Turn)
	for i, o := range g.Owner {
		if g.Geo.Playable(i) && g.Count[i] < 255 && (o == NoOwner || o == p) {
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
	g.TerrainEvents = g.TerrainEvents[:0]
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

// Pass ends the current player's turn without placing an orb, for a host skipping a seat whose player left. It
// counts as their turn but not as a move: a player who passes before placing anything is out, and the pass can
// leave a single player with orbs, which wins.
func (g *Game) Pass() error {
	if g.Over() {
		return ErrGameOver
	}
	g.Played |= 1 << g.Turn
	if !g.checkWin() {
		g.advance()
	}
	return nil
}

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
	topologyChanged := false
	if g.Map != nil {
		clear(g.arrived)
		g.arrived[cell] = 1
		if g.circuitInputs(-1, record) {
			g.Geo = g.mapGeometry()
			topologyChanged = true
		}
	}

	g.cur = g.cur[:0]
	if g.unstable || topologyChanged {
		for i, n := range g.Count {
			if g.Geo.Playable(i) && n >= g.Geo.Cap[i] {
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
		if g.Map != nil {
			g.terrainWave(wave, record)
		}
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
	if g.Map != nil {
		clear(g.arrived)
	}
	g.next, g.from = g.next[:0], g.from[:0]
	for _, e := range g.cur {
		// One orb per target; those sent into Holes are lost, so only Neighbours receive below.
		p, released := g.Owner[e], uint8(len(geo.Targets(int(e))))
		g.from = append(g.from, p)
		g.Count[e] -= released
		g.Orbs[p] -= int32(released)
		switch {
		case g.Count[e] == 0:
			g.Owner[e] = NoOwner
		case g.Count[e] >= geo.Cap[e]:
			g.next = append(g.next, e)
		}
	}
	for k, e := range g.cur {
		p := g.from[k]
		for _, n := range geo.Neighbours(int(e)) {
			if o := g.Owner[n]; o != p {
				// Capture: the cell's existing orbs change hands with it.
				if o != NoOwner {
					g.Orbs[o] -= int32(g.Count[n])
				}
				g.Orbs[p] += int32(g.Count[n])
				g.Owner[n] = p
			}
			g.Count[n]++
			if g.Map != nil {
				g.arrived[n] = 1
			}
			g.Orbs[p]++
			if g.Count[n] == geo.Cap[n] {
				g.next = append(g.next, n)
			}
		}
	}
}

// checkWin sets Winner and reports true once every player has had a turn and only one has orbs left. Nothing can
// explode in the first round, so in play this is the brief's "once at least two have played"; it differs only
// after a Pass, where a player yet to move must still get their turn.
func (g *Game) checkWin() bool {
	if g.Played != 1<<g.Players-1 {
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
