package core

import (
	"errors"
	"fmt"
	"slices"
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
	Boxes          []uint8        // BoxHere and CrateFloor bits per cell; empty on maps without Boxes
	TerrainEvents  []TerrainEvent // events in the last recorded move
	// ProducedAt counts the waves of the last recorded move before the mover's Furnaces and Crushers ran; -1 when
	// the mover owned none.
	ProducedAt int
	topology   *mapTopology // immutable designs and bounded, shared connection tables

	// Exploded counts the cell explosions in the last move: a deterministic measure of its cost, which the AI's
	// budget charges for. It isn't part of the game state (not hashed or snapshotted).
	Exploded int

	// unstable is set when a chain hit MaxWaves and left cells at or over capacity. The next move resolves them
	// together with its own explosions.
	unstable bool
	// shots counts the Turrets at the front of g.cur that fire in the wave about to explode, rather than explode.
	shots int

	// Scratch reused by every move so the hot path doesn't allocate.
	cur, next []int16 // cells exploding in this wave / the next one
	arrived   []uint8 // at least one orb received in this placement or wave
	from      []int8  // owner of each cur cell at the start of the wave
	waveCells []int16 // every recorded wave, back to back
	waveEnds  []int32 // end of each wave in waveCells
	waves     []Wave
	pushes    []boxPush // orbs this wave sent into Boxes
	boxMoves  []boxMove
	boxAt     []int16 // each pushed Box's index in boxMoves, -1 otherwise
	claims    []uint8
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
	g := &Game{Players: players, Winner: NoOwner, ProducedAt: -1}
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
	g.Boxes = append(g.Boxes[:0], src.Boxes...)
	g.TerrainEvents, g.ProducedAt = g.TerrainEvents[:0], -1
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
	return !g.Over() && g.Placeable(cell) && g.Count[cell] < 255 &&
		(g.Owner[cell] == NoOwner || g.Owner[cell] == int8(g.Turn))
}

// Placeable reports whether cell i can take an orb at all: playable, and no Box stands on it.
func (g *Game) Placeable(i int) bool {
	return g.Geo.Playable(i) && (len(g.Boxes) == 0 || g.Boxes[i]&BoxHere == 0)
}

// LegalMoves appends the current player's legal cells to dst and returns it.
func (g *Game) LegalMoves(dst []int) []int {
	if g.Over() {
		return dst
	}
	p := int8(g.Turn)
	for i, o := range g.Owner {
		if g.Placeable(i) && g.Count[i] < 255 && (o == NoOwner || o == p) {
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
	g.TerrainEvents, g.ProducedAt = g.TerrainEvents[:0], -1
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
	if g.Map != nil {
		for _, e := range g.topology.reservoirs {
			g.Pressed[e] &^= ReservoirMade // each may make an orb again
		}
	}
	g.Owner[cell] = p
	g.Count[cell]++
	g.Orbs[p]++
	topologyChanged := false
	if g.Map != nil {
		clear(g.arrived)
		g.arrived[cell] = 1
		if g.circuitInputs(-1, record) {
			g.dropHatches(-1, record)
			g.Geo = g.mapGeometry()
			topologyChanged = true
		}
		// A Sinkhole this placement fills collapses before the first wave is chosen.
		topologyChanged = g.collapseSinkholes(-1, record) || topologyChanged
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
	g.Exploded = 0
	wave := g.resolve(record, 0)
	if g.Map != nil && len(g.topology.makers) != 0 {
		if !g.Over() {
			g.produce(p, record, wave)
		}
		g.settleMakers()
	}
	if !g.Over() {
		g.advance()
	}
	return nil
}

// ReservoirMade marks a Reservoir's Pressed byte once it has made its orb, until the next move starts; the low two
// bits hold its arrow's clockwise step (0 up).
const ReservoirMade uint8 = 4

// ReservoirMakes reports whether a Reservoir in state makes an orb as it explodes, its first time this move, and its
// state after.
func ReservoirMakes(state uint8) (bool, uint8) {
	return state&ReservoirMade == 0, state | ReservoirMade
}

// A maker's Pressed byte holds its holder (player+1) in makerHolder, makerWorking once it has worked for them, and a
// Vault's shield in multiples of vaultShield. A Turret counts its shots there instead, under turretFlag. Any other
// holder, or 0 after a move leaves it empty, makes it skip its next turn and drops the shield or the count.
const (
	makerHolder  uint8 = 15
	makerWorking uint8 = 16
	vaultShield  uint8 = 32
	turretFlag   uint8 = 128
)

// VaultShields is the shield a working Vault raises after its holder's move once the last one broke.
const VaultShields = 2

// MakerTurn returns a Furnace's, Crusher's or Vault's next state after its holder p moves, and whether it works now:
// it skips its first turn after changing hands or after a move left it empty.
func MakerTurn(state uint8, p int8) (uint8, bool) {
	if state&makerHolder != uint8(p+1) {
		return uint8(p + 1), false
	}
	return state | makerWorking, true
}

// MakerWorking reports whether a Furnace, Crusher or Vault in state with this owner has started working for them.
func MakerWorking(state uint8, owner int8) bool {
	return owner != NoOwner && state&(makerHolder|makerWorking) == uint8(owner+1)|makerWorking
}

// Holder returns the player a Furnace, Crusher or Vault in state works for, NoOwner when none.
func Holder(state uint8) int8 { return int8(state&makerHolder) - 1 }

// Shield returns the hits a Vault in state can still take.
func Shield(state uint8) int {
	if state >= turretFlag {
		return 0 // a Turret's shot count
	}
	return int(state / vaultShield)
}

// RaiseShield returns a working Vault's state after its holder's move, and whether it raised a new shield: only once
// the last one broke, as a damaged shield holds until then.
func RaiseShield(state uint8) (uint8, bool) {
	if Shield(state) != 0 {
		return state, false
	}
	return state + VaultShields*vaultShield, true
}

// Shielded reports whether a cell in state stops an orb of player p, and its state after: a Vault's shield takes
// any orb but its holder's, and the orb is lost. Only Vaults ever hold a shield, so any map cell can be asked.
func Shielded(state uint8, p int8) (bool, uint8) {
	if Shield(state) == 0 || state&makerHolder == uint8(p+1) {
		return false, state
	}
	return true, state - vaultShield
}

// TurretShots is how often a Turret fires for its holder, once after each of their moves, before it must rearm.
const TurretShots = 4

// TurretFired returns a working Turret's state after it fires: its last shot leaves it as if just taken, so it skips
// its holder's next move before firing again.
func TurretFired(state uint8) uint8 {
	fired := uint8(1)
	if state >= turretFlag {
		fired += state / vaultShield & 3
	}
	if fired == TurretShots {
		return 0
	}
	return state&(makerHolder|makerWorking) | turretFlag | fired*vaultShield
}

// produce runs the mover's Furnaces, Crushers, Vaults and Turrets once their move has settled, after each skips its
// first turn: each Furnace they own gains an orb, each Crusher loses one, turning neutral when empty, each Vault whose
// shield broke raises a new one and each Turret fires. Furnaces that reach capacity explode as a further chain
// numbered from wave, whose first wave carries the Turrets' shots, listed first.
func (g *Game) produce(p int8, record bool, wave int) {
	g.cur, g.next, g.shots = g.cur[:0], g.next[:0], 0
	ran := false
	for _, e := range g.topology.makers {
		i := int(e)
		if g.Owner[i] != p {
			continue
		}
		ran = true
		var works bool
		if g.Pressed[i], works = MakerTurn(g.Pressed[i], p); !works {
			continue
		}
		switch f := g.Map.Cells[i].Feature; {
		case f == Vault:
			g.Pressed[i], _ = RaiseShield(g.Pressed[i])
		case f == Turret:
			g.Pressed[i] = TurretFired(g.Pressed[i])
			if g.Geo.Aims != nil && g.Geo.Aims[i] != 0 { // shut in, it fires nowhere
				g.cur = append(g.cur, e)
				g.shots++
			}
		case f == Crusher:
			g.Count[i]--
			g.Orbs[p]--
			if g.Count[i] == 0 {
				g.Owner[i] = NoOwner
			}
		case g.Count[i] < 255:
			g.Count[i]++
			g.Orbs[p]++
			if g.Count[i] == g.Geo.Cap[i] {
				g.next = append(g.next, e)
			}
		}
	}
	if !ran {
		return
	}
	g.cur = append(g.cur, g.next...)
	if record {
		g.ProducedAt = wave
	}
	if g.unstable { // the move's chain hit the wave limit: its leftover cells explode with the Furnaces
		g.cur = g.cur[:g.shots]
		for i, n := range g.Count {
			if g.Geo.Playable(i) && n >= g.Geo.Cap[i] && !slices.Contains(g.cur[:g.shots], int16(i)) {
				g.cur = append(g.cur, int16(i))
			}
		}
	}
	g.resolve(record, wave) // also ends the game if a Crusher took the mover's last orb
}

// settleMakers resets every Furnace, Crusher and Vault the move left empty, so whoever fills it next, its last holder
// included, skips a first turn, and a Vault loses its shield. One that popped or was crushed empty and then refilled by its holder in the same move
// keeps working; a different player's orbs already make it skip.
func (g *Game) settleMakers() {
	for _, e := range g.topology.makers {
		if g.Count[e] == 0 {
			g.Pressed[e] = 0
		}
	}
}

// waveLimit is MaxWaves, lowered by tests: a chain that never settles spreads to every cell and wins, so the
// real limit is practically unreachable.
var waveLimit = MaxWaves

// resolve explodes g.cur wave by wave, numbering waves from first, until the board is stable, a player wins or the
// wave limit is reached. It returns the next wave's number.
func (g *Game) resolve(record bool, first int) int {
	g.unstable = false
	for wave := first; ; wave++ {
		if g.checkWin() || len(g.cur) == 0 {
			return wave
		}
		if wave-first == waveLimit {
			g.unstable = true
			return wave
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
		g.shots = 0
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
	for k, e := range g.cur {
		// One orb per target; those sent into Holes or a Spillway's pit are lost, so only Neighbours receive below.
		p, released := g.Owner[e], geo.Released(int(e))
		g.from = append(g.from, p)
		if k < g.shots {
			continue // a Turret's shot is a new orb: it gives up none of its own
		}
		g.Count[e] -= released
		g.Orbs[p] -= int32(released)
		if geo.Outlets != nil && geo.Outlets[e] != 0 {
			// A Reservoir makes one new orb on its first explosion each move; per explosion, chains that never settle
			// would grow without bound.
			var makes bool
			if makes, g.Pressed[e] = ReservoirMakes(g.Pressed[e]); makes {
				g.Count[e]++
				g.Orbs[p]++
			}
		}
		switch {
		case g.Count[e] == 0:
			g.Owner[e] = NoOwner // a Furnace, Crusher or Vault keeps its state, shield included, until the move settles
		case g.Count[e] >= geo.Cap[e]:
			g.next = append(g.next, e)
		}
	}
	for k, e := range g.cur {
		p, to := g.from[k], geo.Neighbours(int(e))
		if k < g.shots {
			// A Turret fires one new orb the way it faces, then turns clockwise to its next open route.
			var slot int
			slot, g.Durability[e] = geo.Aim(int(e), g.Durability[e])
			to = to[slot : slot+1]
		} else if geo.Outlets != nil && geo.Outlets[e] != 0 {
			// A Reservoir throws its extra orb out the side its arrow points to, then the arrow turns clockwise.
			var slot int
			slot, g.Pressed[e] = geo.Outlet(int(e), g.Pressed[e])
			var buf [MaxConnections + 1]int16
			to = append(append(buf[:0], to...), to[slot])
		}
		for _, n := range to {
			if g.Map != nil && g.Pressed[n] >= vaultShield {
				var hit bool
				if hit, g.Pressed[n] = Shielded(g.Pressed[n], p); hit {
					continue // lost on the shield; the sender already gave it up
				}
			}
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

// advance passes the turn to the next alive player. A game that goes on always has another one: the mover may have
// lost their last orb to a Crusher, but then at least two others have orbs or someone has yet to play.
func (g *Game) advance() {
	for i := 1; i <= g.Players; i++ {
		if p := (g.Turn + i) % g.Players; g.Alive(p) {
			g.Turn = p
			return
		}
	}
}
