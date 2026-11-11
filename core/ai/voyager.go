package ai

import (
	"math"

	"orbitrix/core"
)

// Voyager runs Monte Carlo tree search. Each iteration descends the tree by UCT, adds one node, plays a short
// greedy-random rollout and backs up every player's position share; with several players each node scores the player who
// moved into it (max^n style). Nodes only consider a shortlist of promising moves, so the tree grows deep instead of
// wide on big boards. An iteration costs one budget unit (its board copy and shortlist scan, ~10 µs on 40×40) plus
// the usual charge for chain explosions.

const (
	voyagerRootMoves  = 24   // root children
	voyagerInnerMoves = 10   // children of deeper nodes
	voyagerRollout    = 4    // plies played after leaving the tree
	voyagerSamples    = 4    // random legal cells compared in each rollout
	voyagerExplore    = 0.25 // UCT exploration constant
)

type mctsNode struct {
	reward float64 // summed reward of mover over all visits
	visits int32
	first  int32 // index of the first child in Bot.tree, -1 until expanded
	cell   int16 // move that leads here
	mover  int8  // player who made it
	kids   uint8
}

func (b *Bot) voyager() int {
	tree := b.expand(append(b.tree[:0], mctsNode{first: -1}), 0, b.g, voyagerRootMoves)
	defer func() { b.tree = tree }()
	if tree[0].kids == 1 {
		return int(tree[1].cell)
	}

	sim := &b.sim
	for b.left > 0 {
		sim.CopyFrom(b.g)
		path := append(b.path[:0], 0)
		node, exploded := int32(0), 0

		// Select and expand: a node is expanded on its second visit; its first ends the descent as a fresh leaf.
		for !sim.Over() {
			if tree[node].first < 0 {
				tree = b.expand(tree, node, sim, voyagerInnerMoves)
			}
			child, fresh := selectChild(tree, node)
			_ = sim.Simulate(int(tree[child].cell)) // shortlisted cells are legal
			exploded += sim.Exploded
			path, node = append(path, child), child
			if len(path) == 2 && sim.Over() && sim.Winner == b.me {
				return int(tree[child].cell) // immediate win
			}
			if fresh {
				break
			}
		}

		// Roll out.
		for d := 0; d < voyagerRollout && !sim.Over(); d++ {
			_ = sim.Simulate(b.rolloutMove(sim))
			exploded += sim.Exploded
		}

		// Back up.
		values := positionShares(sim)
		for _, k := range path {
			n := &tree[k]
			n.visits++
			if k != 0 { // the root needs only its visit count
				n.reward += values[n.mover]
			}
		}
		b.path = path
		b.charge(1, exploded)
	}

	// The most visited move is the most trusted; ties go to the better average.
	root := tree[0]
	best := root.first
	for i := root.first + 1; i < root.first+int32(root.kids); i++ {
		if k, top := tree[i], tree[best]; k.visits > top.visits || k.visits == top.visits && k.reward > top.reward {
			best = i
		}
	}
	return int(tree[best].cell)
}

// positionShares values a rollout cutoff by material, territory, readiness and exposure.
// Finished games use exact win/loss rewards; eliminated players always receive zero.
func positionShares(g *core.Game) (values [core.MaxPlayers]float64) {
	if g.Over() {
		values[g.Winner] = 1
		return values
	}
	var order [core.MaxPlayers]int
	for p := 0; p < g.Players; p++ {
		order[p] = p - g.Turn
		if order[p] < 0 {
			order[p] += g.Players
		}
	}
	for i, p := range g.Owner {
		if p == core.NoOwner {
			continue
		}
		c, cp := int(g.Count[i]), int(g.Geo.Cap[i])
		value := 3*c + 4
		if c+1 >= cp {
			value += 3
		}
		if cp == 2 {
			value += 2
		}
		for _, j := range g.Geo.Neighbours(i) {
			if q := g.Owner[j]; q != core.NoOwner && q != p && g.Count[j]+1 >= g.Geo.Cap[j] {
				// A ready cell can capture its attacker first when its owner's turn comes sooner.
				if c+1 >= cp && order[p] < order[q] {
					continue
				}
				value -= 2*c + 2
			}
		}
		values[p] += float64(max(1, value))
	}
	total := 0.0
	for p := 0; p < g.Players; p++ {
		total += values[p]
	}
	if total == 0 {
		for p := 0; p < g.Players; p++ {
			values[p] = 1 / float64(g.Players)
		}
		return values
	}
	for p := 0; p < g.Players; p++ {
		values[p] /= total
	}
	return values
}

// selectChild returns the first unvisited child of node (in shortlist order), or else the one with the best UCT score.
func selectChild(tree []mctsNode, node int32) (child int32, fresh bool) {
	n := tree[node]
	explore := voyagerExplore * visitMath(n.visits).sqrtLn
	best, bestScore := n.first, math.Inf(-1)
	for i := n.first; i < n.first+int32(n.kids); i++ {
		k := &tree[i]
		if k.visits == 0 {
			return i, true
		}
		m := visitMath(k.visits)
		if s := k.reward*m.inv + explore*m.invSqrt; s > bestScore {
			best, bestScore = i, s
		}
	}
	return best, false
}

// visitTerms holds the UCT terms for a visit count n: 1/n, 1/√n and √ln n.
type visitTerms struct{ inv, invSqrt, sqrtLn float64 }

// visitTable precomputes visitTerms for the counts a default budget reaches, so selection makes no math calls.
var visitTable = func() (t [2048]visitTerms) {
	for n := 1; n < len(t); n++ {
		t[n] = visitTerms{1 / float64(n), 1 / math.Sqrt(float64(n)), math.Sqrt(math.Log(float64(n)))}
	}
	return t
}()

func visitMath(n int32) visitTerms {
	if int(n) < len(visitTable) {
		return visitTable[n]
	}
	f := float64(n)
	return visitTerms{1 / f, 1 / math.Sqrt(f), math.Sqrt(math.Log(f))}
}

// expand gives node one child per shortlisted move for the player to move in g.
func (b *Bot) expand(tree []mctsNode, node int32, g *core.Game, k int) []mctsNode {
	list := b.shortlist(g, k)
	tree[node].first, tree[node].kids = int32(len(tree)), uint8(len(list))
	for _, s := range list {
		tree = append(tree, mctsNode{first: -1, cell: int16(s.cell), mover: int8(g.Turn)})
	}
	return tree
}

// shortlist returns the k most promising legal moves for the player to move in g, by a cheap prior: explode (more if
// it captures), build up our own cells, claim empty cells next to ours or on the border, and keep away from enemy
// cells that can pop first. A salted hash of the cell breaks ties: one random draw per call instead of per cell.
func (b *Bot) shortlist(g *core.Game, k int) []scored {
	geo, p := g.Geo, int8(g.Turn)
	salt := uint32(b.rng.Uint64())
	list := b.list[:0]
	for i, o := range g.Owner {
		if !g.Placeable(i) || o != core.NoOwner && o != p {
			continue
		}
		c, cp := int(g.Count[i]), int(geo.Cap[i])
		// Skip the neighbour scan when even the best case (every neighbour ours, no threats) can't make the list.
		bound := 10 + 8*cp + 12*(4-cp)
		switch {
		case o == p && c+1 >= cp:
			bound = 100
		case o == p:
			bound = 20 + 10*c
		}
		bound += pull(g, i, p)
		if len(list) == k && float64(bound+1) <= list[k-1].score {
			continue
		}
		own, enemies, threats := 0, 0, 0
		for _, j := range geo.Neighbours(i) {
			switch q := g.Owner[j]; {
			case q == p:
				own++
			case q != core.NoOwner:
				enemies++
				if g.Count[j]+1 >= geo.Cap[j] {
					threats++
				}
			}
		}
		var score int
		switch {
		case o == p && c+1 >= cp:
			score = 60 + 40*min(enemies, 1)
		case o == p:
			score = 20 + 10*c - 25*threats
		default:
			score = 10 + 8*own - 25*threats + 12*(4-cp) // corners +24, edges +12
		}
		score += pull(g, i, p)
		tie := float64((uint32(i)^salt)*0x9e3779b1>>22) / 1024 // Fibonacci hashing: [0, 1)
		list = insertTop(list, scored{float64(score) + tie, i}, k)
	}
	b.list = list
	return list
}

// rolloutMove compares a few random legal cells, preferring captures, explosions and building our cells.
func (b *Bot) rolloutMove(g *core.Game) int {
	p, n := int8(g.Turn), uint64(len(g.Owner))
	best, bestScore, found := -1, -1, 0
	var bits uint64
	for tries := 0; tries < 8*voyagerSamples && found < voyagerSamples; tries++ {
		// Four 16-bit samples per random draw; scaling 16 bits to ≤1600 cells skews odds by under 3%, fine for rollouts.
		if tries%4 == 0 {
			bits = b.rng.Uint64()
		}
		i := int((bits & 0xffff) * n >> 16)
		bits >>= 16
		o := g.Owner[i]
		if !g.Placeable(i) || o != core.NoOwner && o != p {
			continue
		}
		found++
		score := 0
		if o == p {
			score = 1
			if g.Count[i]+1 >= g.Geo.Cap[i] {
				score = 2
				for _, j := range g.Geo.Neighbours(i) {
					if q := g.Owner[j]; q != core.NoOwner && q != p {
						score = 3
						break
					}
				}
			}
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best >= 0 {
		return best
	}
	// Few legal cells (many players, late game): scan for them.
	b.legal = g.LegalMoves(b.legal[:0])
	return b.legal[b.rng.Intn(len(b.legal))]
}
