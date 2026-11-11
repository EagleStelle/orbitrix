package ai

import (
	"math"

	"orbitrix/core"
)

// Tactician uses iterative-deepening alpha-beta minimax. All opponents minimize our score;
// only fully searched depths replace the previous answer. Nodes and explosions share the move budget.

const (
	maxDepth  = core.MaxPlayers + 4
	tacticWin = 1e9
)

type tactic struct {
	b      *Bot
	branch int
	hunt   bool // Hunter: Best-Reply Search with quiescence, scored against prey
	prey   int8
}

func (b *Bot) tactician() int { return tactic{b: b, branch: 8}.deepen() }

// deepen runs the iterative-deepening root shared by Tactician and Hunter.
func (t tactic) deepen() int {
	b := t.b
	cands := t.candidates(b.g, b.me, 0, 24)
	best := cands[0].cell
	for depth := 1; depth <= maxDepth; depth++ {
		choice, value, complete := best, math.Inf(-1), true
		for i := range cands {
			if b.left <= 0 {
				complete = false
				break
			}
			child := &b.nodes[0]
			b.play(child, b.g, cands[i].cell)
			if child.Over() && child.Winner == b.me {
				return cands[i].cell
			}
			v, done := t.minimax(child, depth-1, value, math.Inf(1), 1)
			if !done {
				complete = false
				break
			}
			cands[i].score = v
			if v > value {
				choice, value = cands[i].cell, v
			}
			if depth == 1 {
				best = choice // even a tiny budget gets the best fully evaluated one-ply move
			}
		}
		if !complete {
			break
		}
		best = choice
		sortDesc(cands) // search the previous best move first to improve alpha-beta cutoffs
		if math.Abs(value) > tacticWin/2 {
			break
		}
	}
	return best
}

// minimax returns false when the budget prevents completing this depth. Such a partial
// result cannot compete with another move that faced all of its opponent's replies.
func (t *tactic) minimax(n *core.Game, depth int, alpha, beta float64, ply int) (float64, bool) {
	b := t.b
	if n.Over() || !n.Alive(int(b.me)) || depth == 0 && !t.hunt {
		return t.evaluate(n, ply), true
	}
	if depth == 0 {
		return t.quiesce(n, alpha, beta, ply, hunterQuiet)
	}
	if b.left <= 0 {
		return 0, false
	}
	maximizing := int8(n.Turn) == b.me
	if t.hunt && !maximizing {
		return t.bestReply(n, depth, alpha, beta, ply)
	}
	cands := t.candidates(n, int8(n.Turn), ply, t.branch)
	value := math.Inf(1)
	if maximizing {
		value = math.Inf(-1)
	}
	child := &b.nodes[ply]
	for _, c := range cands {
		if b.left <= 0 {
			return 0, false
		}
		b.play(child, n, c.cell)
		v, complete := t.minimax(child, depth-1, alpha, beta, ply+1)
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

// candidates orders captures, build-ups and safe expansion for player, keeping the best limit.
// Dangerous moves stay available as counterattacks.
func (t *tactic) candidates(n *core.Game, player int8, level, limit int) []scored {
	geo := n.Geo
	dst := t.b.levels[level][:0]
	for i, o := range n.Owner {
		if !n.Placeable(i) || o != core.NoOwner && o != player {
			continue
		}
		threat, enemies, owned := 0, 0, 0
		for _, j := range geo.Neighbours(i) {
			eo := n.Owner[j]
			if eo == player {
				owned++
			}
			if eo == core.NoOwner || eo == player {
				continue
			}
			enemies++
			if n.Count[j]+1 >= geo.Cap[j] {
				threat++
			}
		}

		c, cp := int(n.Count[i]), int(geo.Cap[i])
		score := 10 + 4*owned + 10*(4-cp) - 30*threat
		switch {
		case o == player && c+1 >= cp:
			score = 60 + 40*enemies
		case o == player:
			score += 10 + 8*c
		}
		if n.Orbs[player] <= 8 && c+1 < cp {
			score -= 12 * enemies
		}
		score += pull(n, i, player)
		dst = insertTop(dst, scored{float64(score), i}, limit)
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

// evaluate compares material, territory, readiness and exposure against the strongest
// opponent and the rest of the field. Elimination always outweighs positional gains.
func (t *tactic) evaluate(n *core.Game, ply int) float64 {
	me, geo := t.b.me, n.Geo
	if n.Over() {
		if n.Winner == me {
			return tacticWin - float64(ply)
		}
		return -tacticWin + float64(ply)
	}
	if !n.Alive(int(me)) {
		return -tacticWin + float64(ply)
	}
	var values [core.MaxPlayers]float64
	for i, o := range n.Owner {
		if o == core.NoOwner {
			continue
		}
		c, cp := int(n.Count[i]), int(geo.Cap[i])
		value := float64(3*c + 4 + 2*c/max(1, cp-c))
		if cp == 2 {
			value += 4
		}
		enemies := 0
		for _, j := range geo.Neighbours(i) {
			if eo := n.Owner[j]; eo != core.NoOwner && eo != o {
				enemies++
				if n.Count[j]+1 >= geo.Cap[j] {
					value -= float64(4*c + 6)
				}
			}
		}
		if n.Orbs[o] <= 8 {
			value -= float64(enemies * (c + 3))
			if enemies == 0 {
				near := false
				for _, j := range geo.Neighbours(i) {
					for _, k := range geo.Neighbours(int(j)) {
						if eo := n.Owner[k]; eo != core.NoOwner && eo != o {
							near = true
						}
					}
				}
				if near {
					value -= 6 // an isolated opening needs room before an enemy can reach it
				}
			}
		}
		values[o] += value
	}
	var opponents, sum, strongest float64
	strongest = math.Inf(-1)
	for p := 0; p < n.Players; p++ {
		if p == int(me) || !n.Alive(p) {
			continue
		}
		opponents++
		sum += values[p]
		strongest = max(strongest, values[p])
	}
	if opponents == 0 {
		return values[me]
	}
	if t.hunt && n.Alive(int(t.prey)) {
		return values[me] - (3*values[t.prey]+sum/opponents)/4 // the prey counts three times the field
	}
	return values[me] - (strongest+sum/opponents)/2
}
