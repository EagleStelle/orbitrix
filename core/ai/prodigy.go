package ai

import (
	"math"
	"slices"

	"orbitrix/core"
)

// Prodigy plays like AlphaZero (Silver et al. 2018): PUCT tree search guided by a small neural network it learned by
// playing against itself. The network reads the board from the mover's side and returns a prior for every legal cell
// (policy) and the mover's chance of winning (value), which takes the place of rollouts. Its layers are graph
// convolutions over each cell's orthogonal neighbours with one weight for all four directions, so the same weights
// play every board size, rotation and reflection alike. A cell's output depends only on cells netLayers steps away,
// so the search evaluates each player's first position in full and the rest by recomputing the cells near what
// changed. The weights live in prodigy_weights.go, written by TestTrainProdigy in prodigy_train_test.go (build tag
// train).

const (
	netPlanes = 8  // input features per cell, see encode
	netWidth  = 16 // channels per hidden layer; conv needs a multiple of 8
	netLayers = 4  // graph convolutions: a cell sees cells up to this many steps away
	netGlobal = 3  // value-head inputs besides the pooled channels, see globals

	// netParams counts the weights: each layer's self and neighbour matrices and bias, then the policy head (a weight
	// per channel and a bias) and the value head (a weight per pooled channel and global input, and a bias).
	netParams = 2*netPlanes*netWidth + netWidth + (netLayers-1)*(2*netWidth*netWidth+netWidth) +
		netWidth + 1 + netWidth + netGlobal + 1

	// netCellsPerUnit prices network work in budget units: one per this many cells computed through every layer.
	// netScanPerUnit prices the whole-board passes each evaluation makes besides (finding the changed cells, Voyager's
	// position share, the top cells): one per this many cells. Measurements in docs/decisions.md.
	netCellsPerUnit = 16
	netScanPerUnit  = 1024

	prodigyRootMoves  = 32  // root children: the network's top cells; 24–48 play alike on 9×9, 16 and fewer worse
	prodigyInnerMoves = 8   // children of deeper nodes; 5 and 12 played worse
	prodigyExplore    = 0.4 // PUCT exploration constant: 0.3–0.7 play alike on 9×9, 1.5 won a third fewer games
)

const _ uint = -(netWidth % 8) // compile-time check: conv works in blocks of eight channels

type puctNode struct {
	reward float64 // summed reward of mover over all visits
	prior  float32 // network probability of the move that leads here
	value  float32 // network winning chance of the player to move here, once expanded
	visits int32
	first  int32 // index of the first child in Bot.ptree, -1 until expanded
	cell   int16 // move that leads here
	mover  int8  // player who made it
	kids   uint8
}

// netEval is one network evaluation: the input planes and each layer's output per cell, a policy logit per cell
// and the final layer's channels summed over the board.
type netEval struct {
	act   [netLayers + 1][]float32
	logit []float32
	pool  [netWidth]float32
}

// netBase is a full evaluation of one position for its player to move.
type netBase struct {
	netEval
	count []uint8
	owner []int8
	next  int8 // the next mover, which the input planes depend on
	ok    bool // evaluated during this move
}

// prodigyNet evaluates the search's positions: in full for the first one each player is to move in, which becomes
// that player's base, and incrementally from that base for the rest.
type prodigyNet struct {
	bases [core.MaxPlayers]netBase
	near  netEval  // the latest incremental evaluation: activations at recomputed cells only, every cell's logit
	stamp []uint32 // stamp[i] == gen marks cell i as recomputed in near
	dist  []uint8  // for stamped cells, the steps to the nearest changed cell
	order []int16  // stamped cells, nearest first
	gen   uint32
}

func (b *Bot) prodigy() int {
	b.ptree = b.ptree[:0] // an empty tree tells training that no search ran
	if cell, ok := b.winningMove(); ok {
		return cell
	}
	w := b.weights
	if w == nil {
		w = prodigyWeights[:]
	}
	for p := range b.net.bases {
		b.net.bases[p].ok = false
	}
	explore := prodigyExplore
	if b.explore > 0 {
		explore = b.explore
	}
	tree := append(b.ptree[:0], puctNode{first: -1})
	defer func() { b.ptree = tree }()

	sim := &b.sim
	for len(tree) == 1 || b.left > 0 { // the root is always expanded, so even no budget follows the policy
		sim.CopyFrom(b.g)
		path := append(b.path[:0], 0)
		node, exploded := int32(0), 0
		for tree[node].first >= 0 && !sim.Over() {
			node = selectPUCT(tree, node, explore)
			_ = sim.Simulate(int(tree[node].cell)) // children are legal cells
			exploded += sim.Exploded
			path = append(path, node)
		}

		var values [core.MaxPlayers]float64
		cost := 1
		if sim.Over() {
			values[sim.Winner] = 1
		} else {
			k := prodigyInnerMoves
			if node == 0 {
				k = prodigyRootMoves
			}
			var win float64
			tree, win, cost = b.expandNet(tree, node, sim, w, k)
			values = shares(sim, win)
			if node == 0 && b.noise > 0 {
				b.addNoise(tree[tree[0].first : tree[0].first+int32(tree[0].kids)])
			}
		}
		for _, k := range path {
			n := &tree[k]
			n.visits++
			if k != 0 {
				n.reward += values[n.mover]
			}
		}
		b.path = path
		b.charge(cost, exploded)
		if tree[0].kids == 1 {
			break
		}
	}

	// The most visited move is the most trusted; ties go to the higher prior, which comes first.
	root := tree[0]
	best := root.first
	for i := root.first + 1; i < root.first+int32(root.kids); i++ {
		if tree[i].visits > tree[best].visits {
			best = i
		}
	}
	return int(tree[best].cell)
}

// winningMove tries our primed cells next to an enemy and returns one whose chain wins outright, spending at most
// half the budget: the network can't see where a chain ends, but the rules engine can.
func (b *Bot) winningMove() (int, bool) {
	stop := b.left / 2
	for _, m := range b.moves {
		if b.left <= stop {
			break
		}
		if _, enemy, _ := b.around(m); !b.mine(m) || !b.primed(m) || enemy == 0 {
			continue
		}
		b.play(&b.sim, b.g, m)
		if b.sim.Over() && b.sim.Winner == b.me {
			return m, true
		}
	}
	return -1, false
}

// selectPUCT returns node's child with the best PUCT score: its mean reward plus c times exploration scaled by its
// prior. An unvisited child is valued at the node's own network value; any first-play penalty below it played worse.
func selectPUCT(tree []puctNode, node int32, c float64) int32 {
	n := tree[node]
	explore := c * math.Sqrt(float64(n.visits))
	best, bestScore := n.first, math.Inf(-1)
	for i := n.first; i < n.first+int32(n.kids); i++ {
		k := &tree[i]
		q := float64(n.value)
		if k.visits > 0 {
			q = k.reward / float64(k.visits)
		}
		if s := q + explore*float64(k.prior)/float64(1+k.visits); s > bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// expandNet evaluates g with the network and gives node one child per top-k legal cell, with the priors renormalised
// over them. It returns the mover's winning chance and the evaluation's cost in budget units.
func (b *Bot) expandNet(tree []puctNode, node int32, g *core.Game, w []float32, k int) ([]puctNode, float64, int) {
	win, logit, cost := b.net.evaluate(g, w)
	p := int8(g.Turn)
	list := b.list[:0]
	for i, o := range g.Owner {
		if g.Geo.Playable(i) && (o == core.NoOwner || o == p) {
			list = insertTop(list, scored{float64(logit[i]), i}, k)
		}
	}
	b.list = list
	sum := 0.0
	for i := range list {
		list[i].score = math.Exp(list[i].score - list[0].score)
		sum += list[i].score
	}
	tree[node].first, tree[node].kids, tree[node].value = int32(len(tree)), uint8(len(list)), float32(win)
	for _, s := range list {
		tree = append(tree, puctNode{first: -1, prior: float32(s.score / sum), cell: int16(s.cell), mover: p})
	}
	return tree, win, cost
}

// addNoise mixes Dirichlet(1) noise into the root priors so self-play games explore. Training only.
func (b *Bot) addNoise(kids []puctNode) {
	sum := 0.0
	noise := b.list[:len(kids)] // expandNet's list, no longer needed
	for i := range noise {
		noise[i].score = -math.Log(1 - b.rng.Float64()) // exponential draws, normalised below
		sum += noise[i].score
	}
	for i := range kids {
		kids[i].prior = float32((1-b.noise)*float64(kids[i].prior) + b.noise*noise[i].score/sum)
	}
}

// shares turns the mover's winning chance into a reward per player, splitting the rest evenly between the other
// alive players. An unfinished game always has at least two.
func shares(g *core.Game, win float64) (values [core.MaxPlayers]float64) {
	alive := 0
	for p := range g.Players {
		if g.Alive(p) {
			alive++
		}
	}
	for p := range g.Players {
		if p != g.Turn && g.Alive(p) {
			values[p] = (1 - win) / float64(alive-1)
		}
	}
	values[g.Turn] = win
	return values
}

// evaluate runs the network on g for the player to move and returns their winning chance, a policy logit per cell
// (meaningful for legal cells only) and the cost in budget units. The first position of each player to move this
// move is evaluated in full and kept as their base; later ones are evaluated from it.
func (net *prodigyNet) evaluate(g *core.Game, w []float32) (float64, []float32, int) {
	n, base, next := g.Geo.Cells(), &net.bases[g.Turn], nextMover(g)
	scan := 1 + n/netScanPerUnit
	if g.Map != nil || !base.ok || base.next != next {
		win := forward(g, w, &base.netEval)
		base.count, base.owner = append(base.count[:0], g.Count...), append(base.owner[:0], g.Owner...)
		base.next, base.ok = next, true
		return win, base.logit, scan + n/netCellsPerUnit
	}
	win, work := net.update(g, w, base)
	return win, net.near.logit, scan + work/(netLayers*netCellsPerUnit)
}

// update evaluates g from base, the same player's evaluation of a position that differs from g in some cells. Only
// the cells within l steps of a difference change at layer l, so only those are recomputed, into net.near; the rest
// are read from base. It returns the winning chance and the cells recomputed, summed over the layers.
func (net *prodigyNet) update(g *core.Game, w []float32, base *netBase) (float64, int) {
	geo, n, near := g.Geo, g.Geo.Cells(), &net.near
	stamp, dist := grow(&net.stamp, n), grow(&net.dist, n)
	if net.gen++; net.gen == 0 { // wrapped: old stamps could match again
		clear(stamp)
		net.gen = 1
	}
	gen, order := net.gen, net.order[:0]
	for i := range n {
		if g.Count[i] != base.count[i] || g.Owner[i] != base.owner[i] {
			stamp[i], dist[i] = gen, 0
			order = append(order, int16(i))
		}
	}
	var ends [netLayers + 1]int // order[:ends[l]] are the cells within l steps of a difference
	ends[0] = len(order)
	for l, k := 1, 0; l <= netLayers; l++ {
		for end := len(order); k < end; k++ {
			for _, j := range geo.Neighbours(int(order[k])) {
				if stamp[j] != gen {
					stamp[j], dist[j] = gen, uint8(l)
					order = append(order, j)
				}
			}
		}
		ends[l] = len(order)
	}
	net.order = order
	at := func(l, j, c int) []float32 { // layer l's output (0: the input planes) at cell j, c channels
		if stamp[j] == gen && int(dist[j]) <= l {
			return near.act[l][j*c : (j+1)*c]
		}
		return base.act[l][j*c : (j+1)*c]
	}

	x, p := grow(&near.act[0], n*netPlanes), int8(g.Turn)
	for _, i := range order[:ends[0]] {
		encode(g, int(i), p, base.next, x[int(i)*netPlanes:])
	}
	work := 0
	for l := 1; l <= netLayers; l++ {
		wl, bias, cin := netLayer(w, l-1)
		out := grow(&near.act[l], n*netWidth)
		for _, i := range order[:ends[l]] {
			var s [netWidth]float32
			for _, j := range geo.Neighbours(int(i)) {
				for c, v := range at(l-1, int(j), cin) {
					s[c] += v
				}
			}
			conv(out[int(i)*netWidth:], at(l-1, int(i), cin), s[:cin], wl, bias)
		}
		work += ends[l]
	}

	policy, _ := netHeads(w)
	logit := grow(&near.logit, n)
	copy(logit, base.logit)
	near.pool = base.pool
	h, old := near.act[netLayers], base.act[netLayers]
	for _, i := range order[:ends[netLayers]] {
		hi, oi := h[int(i)*netWidth:(int(i)+1)*netWidth], old[int(i)*netWidth:(int(i)+1)*netWidth]
		logit[i] = head(policy, hi)
		for c, v := range hi {
			near.pool[c] += v - oi[c]
		}
	}
	return value(g, w, &near.pool), work
}

// forward runs the whole network on g for the player to move into e and returns their winning chance.
func forward(g *core.Game, w []float32, e *netEval) float64 {
	geo, n := g.Geo, g.Geo.Cells()
	x, p, next := grow(&e.act[0], n*netPlanes), int8(g.Turn), nextMover(g)
	for i := range n {
		encode(g, i, p, next, x[i*netPlanes:])
	}
	for l := range netLayers {
		wl, bias, cin := netLayer(w, l)
		in, out := e.act[l], grow(&e.act[l+1], n*netWidth)
		for i := range n {
			var s [netWidth]float32
			for _, j := range geo.Neighbours(i) {
				for c, v := range in[int(j)*cin : (int(j)+1)*cin] {
					s[c] += v
				}
			}
			conv(out[i*netWidth:], in[i*cin:(i+1)*cin], s[:cin], wl, bias)
		}
	}
	policy, _ := netHeads(w)
	logit, h := grow(&e.logit, n), e.act[netLayers]
	e.pool = [netWidth]float32{}
	for i := range n {
		hi := h[i*netWidth : (i+1)*netWidth]
		logit[i] = head(policy, hi)
		for c, v := range hi {
			e.pool[c] += v
		}
	}
	return value(g, w, &e.pool)
}

// head is a cell's policy logit from its final-layer channels h.
func head(policy, h []float32) float32 {
	s := policy[netWidth]
	for c, v := range h[:netWidth] {
		s += v * policy[c]
	}
	return s
}

// value is the mover's winning chance from the final layer's channels summed over the board and the whole-board
// inputs.
func value(g *core.Game, w []float32, pool *[netWidth]float32) float64 {
	in := valueInputs(g, pool)
	_, head := netHeads(w)
	u := head[netWidth+netGlobal]
	for k, v := range in {
		u += v * head[k]
	}
	return 1 / (1 + math.Exp(-float64(u)))
}

// valueInputs returns the value head's inputs: the final layer's channels averaged over the board, then globals.
func valueInputs(g *core.Game, pool *[netWidth]float32) (in [netWidth + netGlobal]float32) {
	for c, v := range pool {
		in[c] = v / float32(g.Geo.Cells())
	}
	globals(g, in[netWidth:])
	return in
}

// conv computes one cell of a graph convolution, relu(self·x + nbr·s + bias), from the cell's input x and its
// neighbours' summed input s into o. wl holds netWidth weights per input channel: the self rows, then the neighbour
// rows.
func conv(o, x, s, wl, bias []float32) {
	var nz [2 * netWidth]struct {
		v  float32
		at int
	}
	k := 0
	for c, v := range x {
		if v != 0 { // inputs and ReLU outputs are mostly zero
			nz[k].v, nz[k].at = v, c*netWidth
			k++
		}
	}
	for c, v := range s {
		if v != 0 {
			nz[k].v, nz[k].at = v, (len(x)+c)*netWidth
			k++
		}
	}
	for b := 0; b < netWidth; b += 8 { // eight accumulators, so they stay in registers
		r := bias[b : b+8]
		a0, a1, a2, a3, a4, a5, a6, a7 := r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7]
		for _, e := range nz[:k] {
			r := wl[e.at+b : e.at+b+8]
			a0 += e.v * r[0]
			a1 += e.v * r[1]
			a2 += e.v * r[2]
			a3 += e.v * r[3]
			a4 += e.v * r[4]
			a5 += e.v * r[5]
			a6 += e.v * r[6]
			a7 += e.v * r[7]
		}
		r = o[b : b+8]
		r[0], r[1], r[2], r[3] = max(a0, 0), max(a1, 0), max(a2, 0), max(a3, 0)
		r[4], r[5], r[6], r[7] = max(a4, 0), max(a5, 0), max(a6, 0), max(a7, 0)
	}
}

// encode writes cell i's netPlanes features into f, for player p to move with next to move after: ours, theirs, our
// fill (orbs/capacity), their fill, ours ready to explode, theirs ready, owned by next, and how few neighbours the
// cell has.
func encode(g *core.Game, i int, p, next int8, f []float32) {
	f = f[:netPlanes]
	clear(f)
	cp := g.Geo.Cap[i]
	f[7] = float32(4-int(cp)) / 2
	o := g.Owner[i]
	if o == core.NoOwner {
		return
	}
	side := 0
	if o != p {
		side = 1
	}
	f[side], f[2+side] = 1, float32(g.Count[i])/float32(cp)
	if g.Count[i]+1 >= cp {
		f[4+side] = 1
	}
	if o == next {
		f[6] = 1
	}
}

// nextMover returns the alive player who moves after the player to move.
func nextMover(g *core.Game) int8 {
	for i := 1; i < g.Players; i++ {
		if q := (g.Turn + i) % g.Players; g.Alive(q) {
			return int8(q)
		}
	}
	return core.NoOwner
}

// globals writes the value head's whole-board inputs for the player to move into v: their share of the orbs, 1/alive
// players, and Voyager's rules-based position share, which the network learns how far to trust.
func globals(g *core.Game, v []float32) {
	var total int32
	alive := 0
	for p := range g.Players {
		total += g.Orbs[p]
		if g.Alive(p) {
			alive++
		}
	}
	v[0] = 0
	if total > 0 {
		v[0] = float32(g.Orbs[g.Turn]) / float32(total)
	}
	v[1], v[2] = 1/float32(alive), float32(positionShares(g)[g.Turn])
}

// netLayer returns layer l's weights (the self matrix, then the neighbour matrix, netWidth columns each), its bias
// and its input channels.
func netLayer(w []float32, l int) (wl, bias []float32, in int) {
	off := 0
	in = netPlanes
	if l > 0 {
		off, in = 2*netPlanes*netWidth+netWidth+(l-1)*(2*netWidth*netWidth+netWidth), netWidth
	}
	m := 2 * in * netWidth
	return w[off : off+m], w[off+m : off+m+netWidth], in
}

// netHeads returns the policy head (a weight per channel, then the bias) and the value head (a weight per pooled
// channel and global input, then the bias).
func netHeads(w []float32) (policy, value []float32) {
	off := netParams - (netWidth + 1) - (netWidth + netGlobal + 1)
	return w[off : off+netWidth+1], w[off+netWidth+1:]
}

// grow resizes *s to n, keeping its storage when it's big enough.
func grow[T any](s *[]T, n int) []T {
	*s = slices.Grow((*s)[:0], n)[:n]
	return *s
}
