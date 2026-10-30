package ai

import (
	"math"
	"slices"

	"orbitrix/core"
)

// Tactician runs a depth-limited alpha-beta search over shortlisted moves, scoring leaves against every opponent
// at once (material, territory, explosive power, corners, suicide risk). Each search node is one budget unit (plus its chain's cost).

const maxDepth = 3

// tactic holds the search parameters fixed at the root. density stays the root's for the whole search, as in the
// prototype.
type tactic struct {
	b             *Bot
	density       float64
	branch, depth int
}

func (b *Bot) tactician() int {
	total := 0
	for _, c := range b.g.Count {
		total += int(c)
	}
	t := tactic{b: b, density: float64(total) / float64(b.geo.Cells())}
	switch {
	case t.density < 0.2:
		t.branch, t.depth = 8, 2
	case t.density < 0.5:
		t.branch, t.depth = 12, 3
	default:
		t.branch, t.depth = 10, 3
	}

	cands := t.candidates(b.g, b.me, 0)
	best, bestValue := -1, math.Inf(-1)
	for _, c := range cands {
		if b.left <= 0 {
			break
		}
		child := &b.nodes[0]
		b.play(child, b.g, c.cell)
		if child.Over() && child.Winner == b.me {
			return c.cell // fixed: the prototype also saw a "win" whenever only its own orbs were on the board yet
		}
		if v := t.minimax(child, t.depth-1, math.Inf(-1), math.Inf(1), 1); v > bestValue {
			best, bestValue = c.cell, v
		}
	}
	if best < 0 {
		return b.pick(b.moves)
	}
	return best
}

// minimax is the prototype's minimax_multi with two fixes: turns follow the real rules (eliminated players are
// skipped, where the prototype used (player+1) % players), and only our own turns maximize (the prototype simply
// alternated, so with 3+ players it played some opponents as allies). Out of budget, it scores the node as a leaf,
// as the prototype did when out of time.
func (t *tactic) minimax(n *core.Game, depth int, alpha, beta float64, ply int) float64 {
	b := t.b
	if depth == 0 || b.left <= 0 {
		return t.evaluate(n, ply)
	}
	if n.Over() {
		if n.Winner == b.me {
			return float64(10000 - ply)
		}
		return float64(-10000 + ply)
	}
	cands := t.candidates(n, int8(n.Turn), ply)
	if len(cands) == 0 {
		return t.evaluate(n, ply)
	}
	maximizing := int8(n.Turn) == b.me
	value := math.Inf(1)
	if maximizing {
		value = math.Inf(-1)
	}
	child := &b.nodes[ply]
	for i, c := range cands {
		if i > 0 && b.left <= 0 {
			break
		}
		b.play(child, n, c.cell)
		v := t.minimax(child, depth-1, alpha, beta, ply+1)
		if maximizing {
			value, alpha = max(value, v), max(alpha, v)
		} else {
			value, beta = min(value, v), min(beta, v)
		}
		if beta <= alpha {
			break
		}
	}
	return value
}

// candidates returns the best t.branch moves for player on n, highest first (ties in board order), scored by the
// prototype's get_multi_enemy_safe_candidates. Fixed: enemies are everyone but player, where the prototype used the
// root player's opponents, so at enemy turns it saw their own cells as threats and ours as harmless. Its
// attack-scoring branch is dropped: it scores enemy cells, which are never legal moves.
func (t *tactic) candidates(n *core.Game, player int8, level int) []scored {
	geo := n.Geo
	dst := t.b.levels[level][:0]
	for i, o := range n.Owner {
		if o != core.NoOwner && o != player {
			continue
		}
		var threat float64
		var enemies [4]int8
		distinct, owned := 0, 0
		for _, j := range geo.Neighbours(i) {
			eo := n.Owner[j]
			if eo == player {
				owned++
			}
			if eo == core.NoOwner || eo == player {
				continue
			}
			switch c, cp := int(n.Count[j]), int(geo.Cap[j]); {
			case c >= cp-1:
				threat += 3
			case c >= cp-2:
				threat++
			default:
				threat += 0.5
			}
			if !slices.Contains(enemies[:distinct], eo) {
				enemies[distinct] = eo
				distinct++
			}
		}
		corner, edge := geo.Cap[i] == 2, geo.Cap[i] < 4
		var penalty float64
		switch {
		case corner && t.density < 0.4:
			if distinct >= 1 {
				penalty = threat * 60
			}
		case edge && t.density < 0.3:
			if distinct >= 2 {
				penalty = threat * 30
			}
		}

		var score float64
		if o == player {
			c, cp := int(n.Count[i]), int(geo.Cap[i])
			score += float64(c * 3)
			switch {
			case c >= cp-1:
				score += 12
			case c >= cp-2:
				score += 6
			}
		} else {
			score += float64(4 + owned*3)
		}
		if t.density > 0.3 {
			switch {
			case corner:
				score += 4
			case edge:
				score += 2
			}
		}
		final := score - penalty
		if t.density < 0.3 && final <= -30 {
			continue
		}
		dst = insertTop(dst, scored{final, i}, t.branch)
	}
	t.b.levels[level] = dst
	return dst
}

// insertTop inserts s into list, kept sorted highest first with ties in insertion order, holding at most k.
func insertTop(list []scored, s scored, k int) []scored {
	pos := len(list)
	for pos > 0 && list[pos-1].score < s.score {
		pos--
	}
	if pos >= k {
		return list
	}
	if len(list) < k {
		list = append(list, scored{})
	}
	copy(list[pos+1:], list[pos:len(list)-1])
	list[pos] = s
	return list
}

// evaluate is the prototype's multi_enemy_evaluate: our standing against all opponents together, from our side.
func (t *tactic) evaluate(n *core.Game, ply int) float64 {
	me, geo := t.b.me, n.Geo
	var terr, expl, corners [core.MaxPlayers]float64
	suicide := 0.0
	for i, o := range n.Owner {
		if o == core.NoOwner {
			continue
		}
		c, cp := int(n.Count[i]), int(geo.Cap[i])
		terr[o]++
		switch {
		case c >= cp-1:
			expl[o] += 3
		case c >= cp-2:
			expl[o]++
		}
		corner := cp == 2
		if corner {
			corners[o]++
		}
		if ply <= 1 && o == me {
			threats := 0
			for _, j := range geo.Neighbours(i) {
				if eo := n.Owner[j]; eo != core.NoOwner && eo != me {
					if n.Count[j]+1 >= geo.Cap[j] {
						threats += 3
					} else {
						threats++
					}
				}
			}
			switch {
			case corner && threats > 0:
				suicide += float64(threats * 80)
			case threats > 2: // surrounded
				suicide += float64(threats * 40)
			}
		}
	}

	my := float64(n.Orbs[me])
	var opponents, sumMat, sumTerr, sumExpl, first, second, ahead float64
	for p := 0; p < n.Players; p++ {
		v := float64(n.Orbs[p])
		if p == int(me) || v == 0 {
			continue
		}
		opponents++
		sumMat, sumTerr, sumExpl = sumMat+v, sumTerr+terr[p], sumExpl+expl[p]
		if v > first {
			first, second = v, first
		} else if v > second {
			second = v
		}
		if v > my {
			ahead++
		}
	}
	if opponents == 0 {
		return float64(10000 - ply) // we won
	}
	material := (my-sumMat/opponents)*0.4 + (my-first)*0.4 + (my-(first+second))*0.2
	territory := terr[me] - sumTerr/opponents
	explosive := expl[me] - sumExpl/opponents
	adjust := 0.0
	if ahead >= 2 {
		adjust -= ahead * 50 // several enemies ahead of us
	}
	if opponents >= 2 && my >= (first+second)/2*0.8 {
		adjust += 100 // still competitive with the top two
	}
	switch {
	case t.density < 0.3:
		return material*0.6 + territory*1.2 + explosive*0.4 + corners[me]*20 + adjust - suicide
	case t.density < 0.6:
		return material + territory*0.8 + explosive*1.2 + corners[me]*30 + adjust - suicide*0.7
	default:
		return material*1.6 + territory*0.3 + explosive*1.5 + corners[me]*15 + adjust - suicide*0.5
	}
}
