package ai

import (
	"math"

	"orbitrix/core"
)

// Hunter runs Best-Reply Search (Schadd & Winands 2011) on Tactician's alpha-beta: between two of its own moves only
// the single most damaging reply from any one opponent is searched, played out of turn. A round then costs one
// opponent ply whatever the player count, so it sees further than paranoid search in big games. Leaves extend
// through captures (quiescence), so a chain one ply past the horizon still counts. It scores the board against its
// prey, the opponent with the most orbs when it starts thinking, and so goes after the leader.

const (
	hunterQuiet    = 2 // capture plies searched past the horizon
	hunterCaptures = 4 // captures tried per quiescence node, biggest haul first
)

func (b *Bot) hunter() int { return tactic{b: b, branch: 8, hunt: true, prey: b.prey()}.deepen() }

// prey returns the opponent with the most orbs, the first after us in turn order on a tie.
func (b *Bot) prey() int8 {
	prey, most := core.NoOwner, int32(-1)
	for i := 1; i < b.g.Players; i++ {
		if p := (int(b.me) + i) % b.g.Players; b.g.Alive(p) && b.g.Orbs[p] > most {
			prey, most = int8(p), b.g.Orbs[p]
		}
	}
	return prey
}

// bestReply is the opponents' layer: every alive opponent's candidates compete to be the one reply.
func (t *tactic) bestReply(n *core.Game, depth int, alpha, beta float64, ply int) (float64, bool) {
	b := t.b
	opponents := 0
	for p := range n.Players {
		if p != int(b.me) && n.Alive(p) {
			opponents++
		}
	}
	limit := max(3, 2*t.branch/opponents) // the layer stays about as wide as two players' worth of candidates
	value := math.Inf(1)
	child := &b.nodes[ply]
	for i := 1; i < n.Players; i++ {
		q := (int(b.me) + i) % n.Players
		if !n.Alive(q) {
			continue
		}
		for _, c := range t.candidates(n, int8(q), ply, limit) {
			if b.left <= 0 {
				return 0, false
			}
			b.playAs(child, n, c.cell, q)
			v, complete := t.minimax(child, depth-1, alpha, beta, ply+1)
			if !complete {
				return 0, false
			}
			value, beta = min(value, v), min(beta, v)
			if beta <= alpha {
				return value, true
			}
		}
	}
	return value, true
}

// quiesce searches captures from a leaf until the board is quiet. The side to move may stand pat on the static
// score instead: we explode our own primed cells into enemies, opponents explode theirs into ours.
func (t *tactic) quiesce(n *core.Game, alpha, beta float64, ply, left int) (float64, bool) {
	b := t.b
	stand := t.evaluate(n, ply)
	if left == 0 || ply == maxDepth || n.Over() || !n.Alive(int(b.me)) {
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
	for _, c := range t.captures(n, maximizing, ply) {
		if b.left <= 0 {
			return 0, false
		}
		if maximizing {
			b.play(child, n, c.cell)
		} else {
			b.playAs(child, n, c.cell, int(n.Owner[c.cell]))
		}
		v, complete := t.quiesce(child, alpha, beta, ply+1, left-1)
		if !complete {
			return 0, false
		}
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

// captures lists the primed cells that capture when they explode, biggest haul first: ours into any enemy, or an
// opponent's into ours.
func (t *tactic) captures(n *core.Game, ours bool, ply int) []scored {
	b, geo := t.b, n.Geo
	dst := b.levels[ply][:0]
	for i, o := range n.Owner {
		if o == core.NoOwner || (o == b.me) != ours || n.Count[i]+1 < geo.Cap[i] {
			continue
		}
		haul := 0
		for _, j := range geo.Neighbours(i) {
			if q := n.Owner[j]; q != core.NoOwner && q != o && (ours || q == b.me) {
				haul += int(n.Count[j]) + 1
			}
		}
		if haul > 0 {
			dst = insertTop(dst, scored{float64(haul), i}, hunterCaptures)
		}
	}
	b.levels[ply] = dst
	return dst
}

// playAs plays cell for player p out of turn, then gives the move back to us: Best-Reply Search's opponent ply.
// Setting Turn directly is safe on a scratch copy, since the per-player tallies don't depend on it.
func (b *Bot) playAs(child, parent *core.Game, cell, p int) {
	child.CopyFrom(parent)
	child.Turn = p
	_ = child.Simulate(cell) // candidates and captures are p's legal cells
	b.charge(1, child.Exploded)
	if !child.Over() && child.Alive(int(b.me)) {
		child.Turn = int(b.me)
	}
}
