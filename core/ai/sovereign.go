package ai

import (
	"cmp"
	"math"
	"slices"

	"orbitrix/core"
)

// Sovereign searches like a chess engine with an NNUE evaluation (Nasu 2018, as in Stockfish): iterative-deepening
// alpha-beta with a transposition table and killer moves, Best-Reply Search between its own moves (Schadd & Winands
// 2011, as Hunter), and quiescence over captures, scoring positions with a small learned network. Everything it knows
// about a position comes from that network's trained weights; the search only looks ahead.
//
// The network reads each cell's local pattern: from a player's side, how full the cell is for its capacity and the
// multiset of its neighbours' states (empty, or that player's or an enemy's, each primed, one orb short or two short).
// Every pattern of an occupied cell has a learned row of sovH numbers, and a player's accumulator is the sum of the
// rows of their cells, seen from their side. A move changes only the patterns of the cells its chain touched and their
// neighbours, so the search keeps one accumulator set per ply and adds and subtracts those rows. The value head divides
// the root player's accumulator and the per-channel maximum of the opponents' by the number of occupied cells, so every
// board size and stage reads alike, adds whose turn it is, how full the board is and how many players are left, and
// runs two clipped-ReLU layers down to one logit: the root player's chance of winning. A second learned table scores
// every pattern as a move (the policy), which picks the moves each node tries and their order. Inference is integer
// fixed point, so it is fast in scalar Go and identical on every platform. The weights live in sovereign_weights.go,
// written by TestTrainSovereign in sovereign_train_test.go (build tag train).

const (
	sovCodes    = 78125 // 5^7: a neighbourhood as the base-5 count of each of the seven neighbour states
	sovOwned    = 826   // occupied-cell patterns: 1, 2 or 3 fills for capacity 2, 3 or 4 × 28, 84 or 210 neighbourhoods
	sovPatterns = 1148  // plus 322 empty-cell patterns (one per neighbourhood), which only the policy reads
	sovH        = 32    // accumulator channels
	sovGlobal   = 5     // value inputs besides the accumulators, see globals
	sovIn       = 2*sovH + sovGlobal
	sovL2       = 16

	// sovParams counts the weights: a row per occupied pattern, the clipped-ReLU biases of the root player's and the
	// opponents' halves, the hidden layer and its biases, the output weights and bias, then a policy logit per pattern.
	sovParams = sovOwned*sovH + 2*sovH + sovIn*sovL2 + sovL2 + sovL2 + 1 + sovPatterns

	// Fixed point: an activation of 1 is sovOne, as are W1, b1, b2, b3 and the policy; W2 and W3 carry 1<<sovShift.
	sovOne   = 256
	sovShift = 10

	sovWin = 1 << 30 // a won game; the network's logits stay far below

	sovRootMoves = 24 // root candidates, the policy's top cells
	sovBranch    = 8  // our candidates at inner nodes; the opponents share twice that, at least three each
	sovQuiet     = 2  // capture plies searched past the horizon
	sovCaptures  = 4  // captures tried per quiescence node, the policy's favourites first
	sovTable     = 1 << 16

	// The search's work is priced in ticks of about 1.2 ns on a desktop: a cell scanned for moves or captures is one,
	// a move played sovNodeTicks plus one per 16 cells copied, a cell whose pattern it changes sovCellTicks, an
	// evaluation sovEvalTicks. sovTicksPerUnit converts them to budget units. Measurements in docs/decisions.md.
	sovTicksPerUnit = 9728
	sovNodeTicks    = 384
	sovCellTicks    = 54
	sovEvalTicks    = 384
)

const _ uint = -(sovL2 % 8) // compile-time check: value works in blocks of eight hidden units

// sovFills maps an occupied cell's fill (0 primed, 1 one orb short, 2 two short) and capacity to the first of its
// patterns; index 3 is the empty cell's.
var sovFills = [5][4]int16{2: {0, -1, -1, 826}, 3: {28, 112, -1, 854}, 4: {196, 406, 616, 938}}

// sovPow5 is each neighbour state's digit: empty, then the player's own primed, short and far, then an enemy's.
var sovPow5 = [7]int32{1, 5, 25, 125, 625, 3125, 15625}

// sovMulti numbers each neighbourhood code among the neighbourhoods of its size, which its digits sum to.
var sovMulti [sovCodes]int16

func init() {
	var next [5]int16
	for code := range sovCodes {
		size := 0
		for c := code; c > 0; c /= 5 {
			size += c % 5
		}
		if size >= 2 && size <= 4 {
			sovMulti[code] = next[size]
			next[size]++
		}
	}
}

// sovAcc is one position's accumulators: each player's summed rows, and how many cells are occupied.
type sovAcc struct {
	sum   [core.MaxPlayers][sovH]int32
	cells int32
}

// sovMove is a candidate move with its policy logit, for player.
type sovMove struct {
	score  int32
	cell   int16
	player int8
}

type sovEntry struct {
	key   uint64
	value int32
	cell  int16
	depth int8
	flag  uint8 // bound in the low two bits, the mover above
}

const (
	sovExact = 1 + iota
	sovLower
	sovUpper
)

// sovState is Sovereign's scratch, kept in the Bot between moves.
type sovState struct {
	acc     [maxDepth + 1]sovAcc // per ply; acc[0] is the root's
	hash    [maxDepth + 1]uint64
	lists   [maxDepth + 1][]sovMove
	killers [maxDepth + 1][2]int32 // 1 + cell | player<<11 of the latest cutoff moves, 0 for none
	tmp     []sovMove
	table   []sovEntry
	stamp   []uint32 // marks cells already listed by update
	gen     uint32
	cells   []int16
	tick    int
	score   int32 // the chosen move's score, for training

	// cache holds, for the position being searched, every player's policy logit for each cell they may play
	// (cache[p*cells+i]), then each occupied cell's pattern from its owner's side (cache[players*cells+i]). play
	// updates it for the cells it changes, logging the old values; undo restores them.
	cache []int16
	log   []sovUndo
	marks [maxDepth + 1]int32 // log[marks[ply]:] undoes the move into ply
}

type sovUndo struct {
	at  int32
	old int16
}

// sovNet is the weights split into layers.
type sovNet struct{ sovParts[int16] }

type sovParts[T any] struct{ w1, b1, w2, b2, w3, b3, policy []T }

// sovCut splits w into the layers sovParams lists.
func sovCut[T any](w []T) (p sovParts[T]) {
	cut := func(k int) []T {
		s := w[:k:k]
		w = w[k:]
		return s
	}
	p.w1, p.b1, p.w2, p.b2, p.w3 = cut(sovOwned*sovH), cut(2*sovH), cut(sovIn*sovL2), cut(sovL2), cut(sovL2)
	p.b3, p.policy = cut(1), cut(sovPatterns)
	return p
}

type sovSearch struct {
	b   *Bot
	s   *sovState
	net sovNet
}

func (b *Bot) sovereign() int {
	w := b.sweights
	if w == nil {
		w = sovereignWeights[:]
	}
	t := sovSearch{b: b, s: &b.sov, net: sovNet{sovCut(w)}}
	return t.root()
}

// root runs iterative deepening over the policy's top cells. A depth cut short by the budget still counts: its first
// move is the previous depth's best, so any move that completes and beats it is better at the deeper depth.
func (t *sovSearch) root() int {
	b, s := t.b, t.s
	t.begin()
	cands := t.moves(b.g, b.me, s.lists[0][:0], sovRootMoves)
	s.lists[0] = cands
	best := int(cands[0].cell)
	if len(cands) == 1 {
		return best
	}
	for depth := 1; depth <= maxDepth-sovQuiet; depth++ {
		alpha, choice, searched := int32(-math.MaxInt32), best, 0
		for i := range cands {
			if b.left <= 0 {
				break
			}
			child := &b.nodes[0]
			t.play(child, b.g, int(cands[i].cell), int(b.me), 1)
			if child.Over() && child.Winner == b.me {
				s.score = sovWin
				return int(cands[i].cell)
			}
			v, done := t.search(child, depth-1, alpha, math.MaxInt32, 1)
			if !done {
				break
			}
			t.undo(1)
			searched++
			cands[i].score = v
			if v > alpha {
				alpha, choice = v, int(cands[i].cell)
			}
		}
		if searched > 0 {
			best, s.score = choice, alpha
		}
		if searched < len(cands) || alpha >= sovWin/2 || alpha <= -sovWin/2 {
			break
		}
		slices.SortStableFunc(cands, func(x, y sovMove) int { return cmp.Compare(y.score, x.score) })
	}
	return best
}

// begin sizes the scratch for the board and evaluates the root from scratch. The table and killers are cleared, so
// a move depends only on its position.
func (t *sovSearch) begin() {
	s, g := t.s, t.b.g
	n := g.Geo.Cells()
	if len(s.stamp) != n {
		s.stamp, s.gen = make([]uint32, n), 0
		s.cells = make([]int16, 0, n)
	}
	if s.table == nil {
		s.table = make([]sovEntry, sovTable)
	}
	clear(s.table)
	s.killers = [maxDepth + 1][2]int32{}
	s.tick, s.score = 0, 0
	t.net.refresh(&s.acc[0], g)
	h := sovTurnKey(g)
	if size := (g.Players + 1) * n; len(s.cache) != size {
		s.cache = make([]int16, size)
		s.log = make([]sovUndo, 0, 4*size) // a few plies of long chains before it has to grow
	}
	for i := range n {
		h ^= sovKey(g, i)
		t.recache(g, i, false)
	}
	s.hash[0] = h
	s.log = s.log[:0]
	t.spend(8 * len(s.cache)) // a pattern per cached value
}

// recache stores cell i's policy logits and pattern for g, logging the old values when log is set. It returns the
// pattern of an occupied cell, -1 for an empty one.
func (t *sovSearch) recache(g *core.Game, i int, log bool) int {
	n := len(g.Owner)
	if o := g.Owner[i]; o != core.NoOwner {
		pat := sovPattern(g, i, o)
		t.set(int(o)*n+i, t.net.policy[pat], log)
		t.set(g.Players*n+i, int16(pat), log)
		return pat
	}
	// Every player sees the same neighbourhood except for which occupied neighbours are their own, so read it once:
	// foe is its code with every occupied neighbour an enemy's, own[m] what neighbour m adds when it's the player's.
	geo := g.Geo
	nbrs := geo.Neighbours(i)
	var owners [4]int8
	var own [4]int32
	foe := int32(0)
	for m, j := range nbrs {
		o := g.Owner[j]
		owners[m] = o
		if o == core.NoOwner {
			foe++
			continue
		}
		f := sovFill(g.Count[j], geo.Cap[j])
		foe += sovPow5[4+f]
		own[m] = sovPow5[1+f] - sovPow5[4+f]
	}
	base := int(sovFills[geo.Cap[i]][3])
	for p := range g.Players {
		code := foe
		for m := range nbrs {
			if owners[m] == int8(p) {
				code += own[m]
			}
		}
		t.set(p*n+i, t.net.policy[base+int(sovMulti[code])], log)
	}
	return -1
}

func (t *sovSearch) set(at int, v int16, log bool) {
	s := t.s
	if old := s.cache[at]; old != v {
		if log {
			s.log = append(s.log, sovUndo{int32(at), old})
		}
		s.cache[at] = v
	}
}

// undo restores the cache to before the move into ply.
func (t *sovSearch) undo(ply int) {
	s := t.s
	for k := len(s.log) - 1; k >= int(s.marks[ply]); k-- {
		s.cache[s.log[k].at] = s.log[k].old
	}
	s.log = s.log[:s.marks[ply]]
}

// search returns the root player's score for n, or false when the budget ran out before it finished. The root
// player's nodes try their policy's top cells; the opponents' layer offers every alive opponent's top cells, played out
// of turn, and keeps the one reply that hurts most.
func (t *sovSearch) search(n *core.Game, depth int, alpha, beta int32, ply int) (int32, bool) {
	b, s := t.b, t.s
	if n.Over() || !n.Alive(int(b.me)) {
		return t.terminal(n, ply), true
	}
	if depth == 0 {
		return t.quiesce(n, alpha, beta, ply, sovQuiet)
	}
	if b.left <= 0 {
		return 0, false
	}
	key := s.hash[ply]
	e := &s.table[key&(sovTable-1)]
	hit := e.key == key && e.flag != 0
	if hit && int(e.depth) >= depth {
		switch e.flag & 3 {
		case sovExact:
			return e.value, true
		case sovLower:
			if e.value >= beta {
				return e.value, true
			}
		case sovUpper:
			if e.value <= alpha {
				return e.value, true
			}
		}
	}

	maximizing := int8(n.Turn) == b.me
	list := t.candidates(n, maximizing, ply)
	front := 0
	if hit {
		front = promote(list, front, e.cell, int8(e.flag>>2))
	}
	for _, k := range s.killers[ply] {
		if k != 0 {
			front = promote(list, front, int16(k-1)&0x7ff, int8((k-1)>>11))
		}
	}

	a0, b0 := alpha, beta
	value, best := int32(math.MaxInt32), list[0]
	if maximizing {
		value = -math.MaxInt32
	}
	child := &b.nodes[ply]
	for _, m := range list {
		if b.left <= 0 {
			return 0, false
		}
		t.play(child, n, int(m.cell), int(m.player), ply+1)
		v, complete := t.search(child, depth-1, alpha, beta, ply+1)
		if !complete {
			return 0, false
		}
		t.undo(ply + 1)
		if maximizing && v > value || !maximizing && v < value {
			value, best = v, m
		}
		if maximizing {
			alpha = max(alpha, v)
		} else {
			beta = min(beta, v)
		}
		if beta <= alpha {
			if k := 1 + int32(best.cell) | int32(best.player)<<11; s.killers[ply][0] != k {
				s.killers[ply][1], s.killers[ply][0] = s.killers[ply][0], k
			}
			break
		}
	}
	flag := uint8(sovExact)
	switch {
	case value <= a0:
		flag = sovUpper
	case value >= b0:
		flag = sovLower
	}
	*e = sovEntry{key: key, value: value, cell: best.cell, depth: int8(depth), flag: flag | uint8(best.player)<<2}
	return value, true
}

// candidates lists the moves n's node tries, best first: our policy's top cells, or each alive opponent's.
func (t *sovSearch) candidates(n *core.Game, maximizing bool, ply int) []sovMove {
	b, s := t.b, t.s
	if maximizing {
		s.lists[ply] = t.moves(n, b.me, s.lists[ply][:0], sovBranch)
		return s.lists[ply]
	}
	opponents := 0
	for p := range n.Players {
		if p != int(b.me) && n.Alive(p) {
			opponents++
		}
	}
	limit := max(3, 2*sovBranch/opponents)
	list := s.lists[ply][:0]
	for i := 1; i < n.Players; i++ {
		q := (int(b.me) + i) % n.Players
		if !n.Alive(q) {
			continue
		}
		s.tmp = t.moves(n, int8(q), s.tmp[:0], limit)
		for _, m := range s.tmp {
			list = sovInsert(list, m, limit*opponents)
		}
	}
	s.lists[ply] = list
	return list
}

// quiesce extends a leaf through captures until the board is quiet: we explode our primed cells into enemies,
// opponents theirs into ours, and the side to move may stand pat on the network's score instead.
func (t *sovSearch) quiesce(n *core.Game, alpha, beta int32, ply, left int) (int32, bool) {
	b, s := t.b, t.s
	if n.Over() || !n.Alive(int(b.me)) {
		return t.terminal(n, ply), true
	}
	stand := t.net.value(&s.acc[ply], n, b.me)
	t.spend(sovEvalTicks)
	if left == 0 || ply == maxDepth {
		return stand, true
	}
	maximizing := int8(n.Turn) == b.me
	if maximizing {
		if stand >= beta {
			return stand, true
		}
		alpha = max(alpha, stand)
	} else {
		if stand <= alpha {
			return stand, true
		}
		beta = min(beta, stand)
	}
	value := stand
	child := &b.nodes[ply]
	for _, m := range t.captures(n, maximizing, ply) {
		if b.left <= 0 {
			return 0, false
		}
		t.play(child, n, int(m.cell), int(m.player), ply+1)
		v, complete := t.quiesce(child, alpha, beta, ply+1, left-1)
		if !complete {
			return 0, false
		}
		t.undo(ply + 1)
		if maximizing {
			value, alpha = max(value, v), max(alpha, v)
		} else {
			value, beta = min(value, v), min(beta, v)
		}
		if beta <= alpha {
			break
		}
	}
	return value, true
}

// captures lists the primed cells that capture when they explode, the policy's favourites first: ours into any
// enemy, or an opponent's into ours.
func (t *sovSearch) captures(n *core.Game, ours bool, ply int) []sovMove {
	me, geo, cells := t.b.me, n.Geo, len(n.Owner)
	list := t.s.lists[ply][:0]
	for i, o := range n.Owner {
		if o == core.NoOwner || (o == me) != ours || n.Count[i]+1 < geo.Cap[i] {
			continue
		}
		for _, j := range geo.Neighbours(i) {
			if q := n.Owner[j]; q != core.NoOwner && q != o && (ours || q == me) {
				m := sovMove{score: int32(t.s.cache[int(o)*cells+i]), cell: int16(i), player: o}
				list = sovInsert(list, m, sovCaptures)
				break
			}
		}
	}
	t.spend(cells)
	t.s.lists[ply] = list
	return list
}

// moves inserts p's legal cells into list by policy logit, keeping the best limit.
func (t *sovSearch) moves(n *core.Game, p int8, list []sovMove, limit int) []sovMove {
	cells := len(n.Owner)
	logit := t.s.cache[int(p)*cells : int(p+1)*cells]
	for i, o := range n.Owner {
		if o == core.NoOwner || o == p {
			list = sovInsert(list, sovMove{score: int32(logit[i]), cell: int16(i), player: p}, limit)
		}
	}
	t.spend(cells)
	return list
}

func (t *sovSearch) terminal(n *core.Game, ply int) int32 {
	if n.Over() && n.Winner == t.b.me {
		return sovWin - int32(ply)
	}
	return -sovWin + int32(ply)
}

// play copies parent into child and plays cell there for player p, out of turn if need be, then gives the move back
// to the root player after an opponent's move (Best-Reply Search), and updates the accumulators and hash at ply.
func (t *sovSearch) play(child, parent *core.Game, cell, p, ply int) {
	me := int(t.b.me)
	child.CopyFrom(parent)
	child.Turn = p
	waves, _ := child.Apply(cell) // candidates and captures are p's legal cells
	if p != me && !child.Over() && child.Alive(me) {
		child.Turn = me
	}
	t.b.charge(0, child.Exploded)
	t.update(ply, child, parent, cell, waves)
}

// update sets the accumulators and hash at ply from ply-1's, for child, which is parent after a move at cell set off
// waves. Only the placed cell, the exploded cells and their neighbours can change; their patterns, and those of the
// cells next to them, are swapped out. When that's more rows than the board has occupied cells, it starts over.
func (t *sovSearch) update(ply int, child, parent *core.Game, cell int, waves []core.Wave) {
	s, geo := t.s, child.Geo
	if s.gen += 2; s.gen < 2 { // wrapped: old stamps could match again
		clear(s.stamp)
		s.gen = 2
	}
	changed, near, stamp := s.gen-1, s.gen, s.stamp
	list := append(s.cells[:0], int16(cell))
	stamp[cell] = changed
	for _, w := range waves {
		for _, e := range w {
			if stamp[e] != changed {
				stamp[e] = changed
				list = append(list, e)
			}
			for _, j := range geo.Neighbours(int(e)) {
				if stamp[j] != changed {
					stamp[j] = changed
					list = append(list, j)
				}
			}
		}
	}
	h := s.hash[ply-1] ^ sovTurnKey(parent) ^ sovTurnKey(child)
	for _, i := range list {
		h ^= sovKey(parent, int(i)) ^ sovKey(child, int(i))
	}
	s.hash[ply] = h

	end := len(list)
	for _, i := range list[:end] {
		for _, j := range geo.Neighbours(int(i)) {
			if stamp[j] != changed && stamp[j] != near {
				stamp[j] = near
				list = append(list, j)
			}
		}
	}
	s.cells = list
	s.marks[ply] = int32(len(s.log))
	dst, src := &s.acc[ply], &s.acc[ply-1]
	pats := s.cache[child.Players*len(child.Owner):]
	t.spend(sovNodeTicks + len(child.Owner)/16 + sovCellTicks*len(list))
	refresh := 2*len(list) > int(src.cells)
	if refresh {
		t.net.refresh(dst, child)
		t.spend(len(child.Owner) + sovCellTicks/2*int(dst.cells))
	} else {
		copy(dst.sum[:child.Players], src.sum[:child.Players])
		dst.cells = src.cells
	}
	for _, i := range list {
		po, co, pp := parent.Owner[i], child.Owner[i], int(pats[i]) // pp is stale for an empty cell
		cp := t.recache(child, int(i), true)
		if refresh || po == co && (po == core.NoOwner || pp == cp) {
			continue
		}
		if po != core.NoOwner {
			t.net.row(&dst.sum[po], pp, -1)
			dst.cells--
		}
		if co != core.NoOwner {
			t.net.row(&dst.sum[co], cp, 1)
			dst.cells++
		}
	}
}

// spend charges ticks of work, carrying the remainder into the next charge.
func (t *sovSearch) spend(ticks int) {
	s := t.s
	s.tick += ticks
	t.b.left -= s.tick / sovTicksPerUnit
	s.tick %= sovTicksPerUnit
}

// refresh sums every occupied cell's row into its owner's accumulator.
func (net *sovNet) refresh(a *sovAcc, g *core.Game) {
	for q := range g.Players {
		a.sum[q] = [sovH]int32{}
	}
	a.cells = 0
	for i, o := range g.Owner {
		if o != core.NoOwner {
			net.row(&a.sum[o], sovPattern(g, i, o), 1)
			a.cells++
		}
	}
}

// row adds sign times pattern pat's row to acc.
func (net *sovNet) row(acc *[sovH]int32, pat int, sign int32) {
	r := (*[sovH]int16)(net.w1[pat*sovH:])
	if sign > 0 {
		for k, v := range r {
			acc[k] += int32(v)
		}
		return
	}
	for k, v := range r {
		acc[k] -= int32(v)
	}
}

// value returns the logit of player p's chance of winning g, scaled by sovOne<<sovShift, from g's accumulators a.
func (net *sovNet) value(a *sovAcc, g *core.Game, p int8) int32 {
	var x [sovIn]int32
	var opp [sovH]int32
	seen := false
	for q := range g.Players {
		switch {
		case int8(q) == p || g.Orbs[q] == 0:
		case !seen:
			opp, seen = a.sum[q], true
		default:
			for k, v := range a.sum[q] {
				opp[k] = max(opp[k], v)
			}
		}
	}
	r := int64(0)
	if a.cells > 0 {
		r = (1 << 24) / int64(a.cells)
	}
	own := &a.sum[p]
	for k := range sovH {
		x[k] = min(max(int32(int64(own[k])*r>>24)+int32(net.b1[k]), 0), sovOne)
		x[sovH+k] = min(max(int32(int64(opp[k])*r>>24)+int32(net.b1[sovH+k]), 0), sovOne)
	}
	globals := sovGlobals(g, p, a.cells)
	copy(x[2*sovH:], globals[:])

	out := int32(net.b3[0]) << sovShift
	for blk := 0; blk < sovL2; blk += 8 { // eight accumulators, so they stay in registers
		b := (*[8]int16)(net.b2[blk:])
		h0, h1, h2, h3 := int32(b[0])<<sovShift, int32(b[1])<<sovShift, int32(b[2])<<sovShift, int32(b[3])<<sovShift
		h4, h5, h6, h7 := int32(b[4])<<sovShift, int32(b[5])<<sovShift, int32(b[6])<<sovShift, int32(b[7])<<sovShift
		for k, v := range x {
			if v == 0 { // clipped ReLU outputs and one-hot inputs are often zero
				continue
			}
			r := (*[8]int16)(net.w2[k*sovL2+blk:])
			h0 += v * int32(r[0])
			h1 += v * int32(r[1])
			h2 += v * int32(r[2])
			h3 += v * int32(r[3])
			h4 += v * int32(r[4])
			h5 += v * int32(r[5])
			h6 += v * int32(r[6])
			h7 += v * int32(r[7])
		}
		w := (*[8]int16)(net.w3[blk:])
		out += sovClip(h0)*int32(w[0]) + sovClip(h1)*int32(w[1]) + sovClip(h2)*int32(w[2]) + sovClip(h3)*int32(w[3]) +
			sovClip(h4)*int32(w[4]) + sovClip(h5)*int32(w[5]) + sovClip(h6)*int32(w[6]) + sovClip(h7)*int32(w[7])
	}
	return out
}

// sovClip is the hidden layer's clipped ReLU, back to the activation scale.
func sovClip(v int32) int32 { return min(max(v>>sovShift, 0), sovOne) }

// sovGlobals returns the value head's whole-board inputs for player p, scaled by sovOne: whether p moves now, next or
// later among the alive players, the share of occupied cells, and 1/alive players. p must be alive.
func sovGlobals(g *core.Game, p int8, cells int32) (x [sovGlobal]int32) {
	alive, d := int32(0), int32(0)
	for i := range g.Players {
		if q := (g.Turn + i) % g.Players; g.Alive(q) {
			if q == int(p) {
				d = alive
			}
			alive++
		}
	}
	x[min(d, 2)] = sovOne
	x[3] = sovOne * cells / int32(len(g.Owner))
	x[4] = sovOne / alive
	return x
}

// sovPattern returns cell i's pattern seen from player p, who owns it or may play it: its fill, or empty, by capacity,
// and its neighbours' states.
func sovPattern(g *core.Game, i int, p int8) int {
	geo := g.Geo
	k := geo.Cap[i]
	code := int32(0)
	for _, j := range geo.Nbr[4*i : 4*i+int(k)] {
		o := g.Owner[j]
		switch {
		case o == core.NoOwner:
			code++
		case o == p:
			code += sovPow5[1+sovFill(g.Count[j], geo.Cap[j])]
		default:
			code += sovPow5[4+sovFill(g.Count[j], geo.Cap[j])]
		}
	}
	fill := 3
	if g.Owner[i] != core.NoOwner {
		fill = sovFill(g.Count[i], k)
	}
	return int(sovFills[k][fill]) + int(sovMulti[code])
}

// sovFill is 0 for a primed cell of capacity k holding c orbs, 1 for one orb short and 2 for two.
func sovFill(c, k uint8) int {
	switch {
	case c+1 >= k:
		return 0
	case c+2 == k:
		return 1
	}
	return 2
}

// sovInsert inserts m into list, kept sorted highest first with ties in insertion order, holding at most k.
func sovInsert(list []sovMove, m sovMove, k int) []sovMove {
	pos := len(list)
	for pos > 0 && list[pos-1].score < m.score {
		pos--
	}
	if pos >= k {
		return list
	}
	if len(list) < k {
		list = append(list, sovMove{})
	}
	copy(list[pos+1:], list[pos:len(list)-1])
	list[pos] = m
	return list
}

// promote moves player's cell to list[front] if it's at or after it, and returns the next front.
func promote(list []sovMove, front int, cell int16, player int8) int {
	for i := front; i < len(list); i++ {
		if list[i].cell == cell && list[i].player == player {
			m := list[i]
			copy(list[front+1:i+1], list[front:i])
			list[front] = m
			return front + 1
		}
	}
	return front
}

// sovKey is cell i's share of the position hash: zero when empty.
func sovKey(g *core.Game, i int) uint64 {
	o := g.Owner[i]
	if o == core.NoOwner {
		return 0
	}
	return sovMix(uint64(i)<<16 | uint64(o)<<8 | uint64(g.Count[i]))
}

// sovTurnKey is the hash's share for whose turn it is and who has played.
func sovTurnKey(g *core.Game) uint64 { return sovMix(1<<40 | uint64(g.Turn)<<16 | uint64(g.Played)) }

// sovMix is splitmix64's finalizer.
func sovMix(x uint64) uint64 {
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}
