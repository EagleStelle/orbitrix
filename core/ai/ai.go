package ai

import (
	"cmp"
	"fmt"
	"slices"

	"orbitrix/core"
)

// Strategy is an AI personality, ordered from easiest to hardest for the setup dropdown.
type Strategy uint8

const (
	Jester Strategy = iota
	Mason
	Warden
	Gambler
	Nomad
	Raider
	Spy
	Prodigy
	Tactician
	Voyager
	Hunter
	Sovereign
	NumStrategies
)

var names = [NumStrategies]string{
	"jester", "mason", "warden", "gambler", "nomad", "raider", "spy", "prodigy", "tactician", "voyager", "hunter",
	"sovereign",
}

func (s Strategy) String() string {
	if s < NumStrategies {
		return names[s]
	}
	return fmt.Sprintf("Strategy(%d)", s)
}

// Parse returns the strategy with the given name.
func Parse(name string) (Strategy, bool) {
	i := slices.Index(names[:], name)
	return Strategy(i), i >= 0
}

// DefaultBudget is the work limit per move that replaces the prototype's wall-clock limits. Each full simulation
// (Tactician: search node) costs one unit plus one per unitExplosions cells its chain explodes, so the limit tracks
// real work while staying deterministic. Moves that can't explode are scored without simulating and cost nothing.
// Sized for the 150 ms target on a 40×40 board; measurements in docs/decisions.md.
const DefaultBudget = 1200

// Budget returns s's work limit per move where the AI thinks on its own thread (desktop and Android): DefaultBudget,
// except Sovereign, which may think four times as long (≈60 ms on a desktop, about the 150 ms target on a mid-range
// phone). In a browser, where thinking holds up drawing, every strategy gets the same budget.
func (s Strategy) Budget() int {
	if s == Sovereign {
		return 4 * DefaultBudget
	}
	return DefaultBudget
}

// unitExplosions makes a unit cost about the same however it's spent: 512 explosions take roughly as long as one
// Tactician evaluation of a 40×40 board (≈10 µs each on a desktop).
const unitExplosions = 512

// Bot chooses moves. It keeps scratch buffers between calls, so keep one per game or room; it isn't safe for
// concurrent use. The zero value is ready to use.
type Bot struct {
	g    *core.Game
	geo  *core.Geometry
	me   int8
	rng  core.RNG
	left int   // budget units left this move
	boom int   // explosions not yet charged (under unitExplosions)
	made int32 // orbs our working Furnaces make minus those our Crushers eat after any move
	pops bool  // one of our working Furnaces explodes after any move

	sim   core.Game
	moves []int
	list  []scored
	ties  []int
	mark  []bool
	queue []int16
	dist  [3][]int16
	bets  [3][]bet
	cands []int

	frontier []float64 // Raider: enemy fill within two steps of each cell

	tree  []mctsNode // Voyager: search tree
	path  []int32    // Voyager: nodes visited this iteration
	legal []int      // Voyager: rollout fallback

	nodes  [maxDepth]core.Game // Tactician, Hunter: one board per search depth
	levels [maxDepth][]scored  // Tactician, Hunter: candidates per search depth

	ptree   []puctNode // Prodigy: search tree
	net     prodigyNet // Prodigy: network evaluations
	weights []float32  // Prodigy: network weights, nil for prodigyWeights; set by training
	noise   float64    // Prodigy: share of root prior replaced by noise; training only
	explore float64    // Prodigy: PUCT exploration, 0 for prodigyExplore; training only

	sov      sovState // Sovereign: accumulators, transposition table, killer moves
	sweights []int16  // Sovereign: network weights, nil for sovereignWeights; set by training
}

type scored struct {
	score float64
	cell  int
}

// Choose returns the cell the player to move in g should play, or -1 if the game is over. The result depends only
// on g, s, seed and budget (see DefaultBudget); an unknown strategy plays like Jester, as in the prototype.
func (b *Bot) Choose(g *core.Game, s Strategy, seed uint64, budget int) int {
	b.moves = g.LegalMoves(b.moves[:0])
	if len(b.moves) == 0 {
		return -1
	}
	b.g, b.geo, b.me, b.rng, b.left, b.boom = g, g.Geo, int8(g.Turn), core.NewRNG(seed), budget, 0
	switch s {
	case Sovereign:
		if g.Map != nil {
			// shortcut: the learned pattern table covers rectangular capacity-2..4
			// boards only; use the existing exact search until map-trained weights exist.
			return b.tactician()
		}
		return b.sovereign()
	case Prodigy:
		return b.prodigy()
	case Voyager:
		return b.voyager()
	case Hunter:
		return b.hunter()
	case Tactician:
		return b.tactician()
	case Jester:
		return b.jester()
	}
	b.production()
	if g.Map != nil {
		// The heuristic bots keep their personalities but stop feeding Crushers.
		if kept := slices.DeleteFunc(b.moves, func(m int) bool { return wasted(g, m, b.me) }); len(kept) > 0 {
			b.moves = kept
		} else {
			b.moves = g.LegalMoves(b.moves[:0])
		}
	}
	fragile := b.fragileOpening()
	// Keep one simulation for checking an opening counterattack after the strategy spends its budget.
	reserve := 0
	if b.left > 0 && fragile {
		reserve = 1
		b.left--
	}
	var cell int
	switch s {
	case Gambler:
		cell = b.gambler()
	case Warden:
		cell = b.warden()
	case Raider:
		cell = b.raider()
	case Spy:
		cell = b.spy()
	case Mason:
		cell = b.mason()
	case Nomad:
		cell = b.nomad()
	default:
		return b.jester()
	}
	b.left += reserve
	if fragile {
		return b.openingMove(cell)
	}
	return cell
}

// tally returns every player's orbs after the player to move plays cell: the prototype's simulate_after_move,
// but exact. ok is false once the simulation budget is spent; strategies handle that the way the prototype handled
// a failed simulation (None).
func (b *Bot) tally(cell int) (t [core.MaxPlayers]int32, ok bool) {
	t = b.g.Orbs
	quiet := !b.primed(cell) && !b.pops
	if m := b.g.Map; quiet && m != nil {
		// A switch can change capacities; our working Furnace gains the placed orb and then its own.
		f := m.Cells[cell]
		quiet = !f.Circuit.IsSwitch() &&
			!(f.Feature == core.Furnace && works(b.g.Pressed[cell], b.me) && b.count(cell)+2 >= b.capacity(cell))
	}
	if quiet {
		// Nothing explodes, so the outcome is known without simulating: our orb plus what our Furnaces and Crushers
		// do. shortcut: ignores cells left over capacity by a chain cut at core.MaxWaves, which resolve on the next
		// move; harmless for scoring.
		t[b.me] += 1 + b.made
		return t, true
	}
	if b.left <= 0 {
		return t, false
	}
	b.play(&b.sim, b.g, cell)
	return b.sim.Orbs, true
}

// works reports whether a Furnace or Crusher in state acts after its holder p's next move.
func works(state uint8, p int8) bool {
	_, w := core.MakerTurn(state, p)
	return w
}

// production sets made and pops from the mover's Furnaces and Crushers that act after this move; the rest skip it.
func (b *Bot) production() {
	b.made, b.pops = 0, false
	g := b.g
	if g.Map == nil {
		return
	}
	for i, c := range g.Map.Cells {
		if !c.Feature.IsMaker() || g.Owner[i] != b.me || !works(g.Pressed[i], b.me) {
			continue
		}
		if c.Feature == core.Crusher {
			b.made--
		} else {
			b.made++
			b.pops = b.pops || g.Count[i]+1 >= g.Geo.Cap[i]
		}
	}
}

// wasted reports p's quiet orb on a Crusher: once working it eats one per move, so placements never fill it. One that
// skips this move can still be primed by p's next orb.
func wasted(g *core.Game, i int, p int8) bool {
	if g.Map == nil || g.Map.Cells[i].Feature != core.Crusher {
		return false
	}
	c, cp := int(g.Count[i]), int(g.Geo.Cap[i])
	return c+1 < cp && (works(g.Pressed[i], p) || c+2 < cp)
}

// pull orders p's search candidates on maps: claim empty Furnaces, which pay their holder an orb every move, and
// leave wasted Crusher moves for last.
func pull(g *core.Game, i int, p int8) int {
	switch {
	case g.Map == nil:
		return 0
	case wasted(g, i, p):
		return -100
	case g.Map.Cells[i].Feature == core.Furnace && g.Owner[i] == core.NoOwner:
		return 24
	}
	return 0
}

// play copies parent into child and plays cell there, charging the budget for the work.
func (b *Bot) play(child, parent *core.Game, cell int) {
	child.CopyFrom(parent)
	_ = child.Simulate(cell) // callers pass legal cells
	b.charge(1, child.Exploded)
}

// charge spends units, plus one per unitExplosions explosions. The remainder carries over, so many small chains
// are charged as fully as one big one.
func (b *Bot) charge(units, explosions int) {
	b.boom += explosions
	b.left -= units + b.boom/unitExplosions
	b.boom %= unitExplosions
}

// others sums the orbs of every player except the one to move.
func (b *Bot) others(t [core.MaxPlayers]int32) int32 {
	var n int32
	for p := 0; p < b.g.Players; p++ {
		if p != int(b.me) {
			n += t[p]
		}
	}
	return n
}

// wins reports whether no opponent has orbs left in t.
func (b *Bot) wins(t [core.MaxPlayers]int32) bool { return b.others(t) == 0 }

func (b *Bot) mine(i int) bool                { return b.g.Owner[i] == b.me }
func (b *Bot) empty(i int) bool               { return b.geo.Playable(i) && b.g.Owner[i] == core.NoOwner }
func (b *Bot) enemy(i int) bool               { return b.g.Owner[i] != core.NoOwner && !b.mine(i) }
func (b *Bot) count(i int) int                { return int(b.g.Count[i]) }
func (b *Bot) capacity(i int) int             { return int(b.geo.Cap[i]) }
func (b *Bot) corner(i int) bool              { return b.geo.Cap[i] == 2 }
func (b *Bot) edge(i int) bool                { return b.geo.Cap[i] < 4 } // includes corners, like the prototype's c == 0 or …
func (b *Bot) primed(i int) bool              { return b.count(i)+1 >= b.capacity(i) }
func (b *Bot) nbrs(i int) []int16             { return b.geo.Neighbours(i) }
func (b *Bot) uniform(lo, hi float64) float64 { return lo + (hi-lo)*b.rng.Float64() }
func (b *Bot) pick(cells []int) int           { return cells[b.rng.Intn(len(cells))] }

// threats counts i's neighbours that an enemy can explode next turn: the prototype's exposure_if_placed / danger.
func (b *Bot) threats(i int) int {
	n := 0
	for _, j := range b.nbrs(i) {
		if b.enemy(int(j)) && b.primed(int(j)) {
			n++
		}
	}
	return n
}

// around counts i's neighbours that are ours, an enemy's and empty.
func (b *Bot) around(i int) (mine, enemy, empty int) {
	for _, j := range b.nbrs(i) {
		switch {
		case b.mine(int(j)):
			mine++
		case b.empty(int(j)):
			empty++
		default:
			enemy++
		}
	}
	return mine, enemy, empty
}

// pickWhere returns a uniformly random legal move that keep accepts.
func (b *Bot) pickWhere(keep func(cell int) bool) (int, bool) {
	b.ties = b.ties[:0]
	for _, m := range b.moves {
		if keep(m) {
			b.ties = append(b.ties, m)
		}
	}
	if len(b.ties) == 0 {
		return -1, false
	}
	return b.pick(b.ties), true
}

// offer tracks the best score seen in b.ties, keeping every cell within eps of it so ties can be broken at random.
// Reset with b.ties = b.ties[:0].
func (b *Bot) offer(top *float64, score float64, cell int, eps float64) {
	switch {
	case len(b.ties) == 0 || score > *top:
		*top, b.ties = score, append(b.ties[:0], cell)
	case *top-score <= eps:
		b.ties = append(b.ties, cell)
	}
}

// sortDesc sorts by score, highest first, keeping equal scores in their original order like Python's sort.
func sortDesc(list []scored) {
	slices.SortStableFunc(list, func(x, y scored) int { return cmp.Compare(y.score, x.score) })
}

// marks returns a cleared flag per cell.
func (b *Bot) marks() []bool {
	b.mark = slices.Grow(b.mark[:0], b.geo.Cells())[:b.geo.Cells()]
	clear(b.mark)
	return b.mark
}

// distances fills dst with connection distances to the nearest selected cell.
// Breadth-first search also follows Links and detours around map obstructions.
func (b *Bot) distances(dst []int16, from func(i int) bool) []int16 {
	n := b.geo.Cells()
	far := int16(b.geo.Cols + b.geo.Rows) // longer than any real distance, so it also marks "unvisited"
	if b.g.Map != nil {
		far = int16(n + 1) // custom routes may be much longer than cols+rows
	}
	dst = slices.Grow(dst[:0], n)[:n]
	q := b.queue[:0]
	for i := range dst {
		dst[i] = far
		if b.geo.Playable(i) && from(i) {
			dst[i] = 0
			q = append(q, int16(i))
		}
	}
	for k := 0; k < len(q); k++ {
		i := q[k]
		for _, j := range b.nbrs(int(i)) {
			if dst[j] == far {
				dst[j] = dst[i] + 1
				q = append(q, j)
			}
		}
	}
	b.queue = q
	return dst
}
