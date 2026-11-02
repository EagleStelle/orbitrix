package ai

import "orbitrix/core"

func (b *Bot) fragileOpening() bool {
	if b.g.Orbs[b.me] > 2 {
		return false
	}
	owned := 0
	for _, p := range b.g.Owner {
		if p == b.me {
			owned++
			if owned > 1 {
				return false
			}
		}
	}
	return true
}

// openingMove protects a fragile opening; Choose checks eligibility before calling it.
func (b *Bot) openingMove(cell int) int {
	b.dist[0] = b.distances(b.dist[0], b.enemy)
	dist := b.dist[0]
	danger := b.openingThreats(b.g, cell)
	if dist[cell] >= 3 && !danger[cell] {
		return cell
	}
	for i, o := range b.g.Owner {
		if o == b.me {
			if dist[i] >= 3 && !danger[i] {
				return cell // another base can survive the local fight
			}
		}
	}
	// An explosion can remove the threat or create its own backup; judge the resulting board.
	if b.primed(cell) && b.left > 0 {
		b.play(&b.sim, b.g, cell)
		if b.sim.Over() && b.sim.Winner == b.me {
			return cell
		}
		b.dist[1] = b.distances(b.dist[1], func(i int) bool {
			return b.sim.Owner[i] != core.NoOwner && b.sim.Owner[i] != b.me
		})
		danger = b.openingThreats(&b.sim, -1)
		for i, o := range b.sim.Owner {
			if o == b.me && b.dist[1][i] >= 3 && !danger[i] {
				return cell
			}
		}
	}

	danger = b.openingThreats(b.g, -1)
	best := -1
	for _, m := range b.moves {
		if !b.empty(m) || danger[m] {
			continue
		}
		if best < 0 || dist[m] > dist[best] || dist[m] == dist[best] && b.capacity(m) < b.capacity(best) {
			best = m
		}
	}
	if best >= 0 && (dist[best] >= 3 || danger[cell]) {
		return best
	}
	return cell // a crowded board may have no safe retreat
}

// openingThreats follows primed chains of any colour, counting converging hits.
// added models a non-explosive placement that may make another link in the chain.
// shortcut: each cell spreads once; dense positions need exact opponent reply simulations.
func (b *Bot) openingThreats(g *core.Game, added int) []bool {
	danger := b.marks()
	var incoming [core.MaxSize * core.MaxSize]uint8
	q := b.queue[:0]
	count, capacity := g.Count[:len(g.Owner)], g.Geo.Cap[:len(g.Owner)] // lets the compiler drop bounds checks
	for i, o := range g.Owner {
		if o != core.NoOwner && o != b.me && count[i]+1 >= capacity[i] {
			danger[i] = true
			incoming[i] = 1
			q = append(q, int16(i))
		}
	}
	for k := 0; k < len(q); k++ {
		for _, j := range g.Geo.Neighbours(int(q[k])) {
			danger[j] = true
			before := int(g.Count[j]) + int(incoming[j])
			if int(j) == added {
				before++
			}
			incoming[j]++
			if before < int(g.Geo.Cap[j]) && before+1 >= int(g.Geo.Cap[j]) {
				q = append(q, j)
			}
		}
	}
	b.queue = q
	return danger
}
