package ai

// Nomad travels light: it spreads single orbs over open ground away from its own cells, never into the reach of an
// enemy chain, and settles next to an enemy only where it would win the race to explode first. It simulates only
// explosions that can capture, so a move costs two passes over the board and a few simulations.

const (
	nomadSims    = 2    // capturing explosions simulated per move
	nomadRoam    = 3    // steps from our nearest cell that still earn spread
	nomadSpread  = 30   // score per step from our nearest cell, up to nomadRoam
	nomadRace    = 40   // score per enemy orb next to a cell that explodes first; lost per own orb there if it doesn't
	nomadCapture = 60   // score per orb an explosion takes
	nomadDanger  = 1000 // lost by placing where an enemy chain can reach next turn
)

func (b *Bot) nomad() int {
	danger := b.openingThreats(b.g, -1)
	b.dist[0] = b.distances(b.dist[0], b.mine)
	own := b.dist[0]

	b.ties = b.ties[:0]
	top := 0.0
	sims := b.list[:0] // explosions that can capture, scored by the enemy orbs next to them
	for _, m := range b.moves {
		if b.primed(m) {
			if !b.quiet(m) {
				sims = insertTop(sims, scored{float64(b.nomadPrey(m)) + b.rng.Float64()*0.5, m}, nomadSims) // ties at random, not top-left first
			}
			continue // a quiet explosion only clumps our orbs: played below if nothing else is left
		}
		b.offer(&top, b.nomadPlace(m, danger[m], own[m]), m, 0)
	}
	b.list = sims

	others := b.others(b.g.Orbs)
	for _, s := range sims {
		t, ok := b.tally(s.cell)
		if !ok {
			break
		}
		if b.wins(t) {
			return s.cell
		}
		if taken := others - b.others(t); taken > 0 {
			b.offer(&top, float64(100+nomadCapture*int(taken)), s.cell, 0)
		}
	}
	if len(b.ties) == 0 {
		return b.pick(b.moves) // only quiet or capture-less explosions are left
	}
	return b.pick(b.ties)
}

// nomadPlace scores a placement at m that can't explode. Open ground scores by its distance from our cells; each enemy
// neighbour is a race: m explodes after cap-count-1 more of our moves, the neighbour after cap-count of theirs, and
// they move first.
func (b *Bot) nomadPlace(m int, danger bool, fromOwn int16) float64 {
	c := b.count(m)
	score := 20.0 // stacking is for attacks and crowded boards
	if b.empty(m) {
		score = 100 + float64(nomadSpread*min(int(fromOwn), nomadRoam))
	}
	ours := b.capacity(m) - c - 1
	for _, j := range b.nbrs(m) {
		if !b.enemy(int(j)) {
			continue
		}
		if theirs := b.capacity(int(j)) - b.count(int(j)); ours < theirs {
			score += float64(nomadRace * (b.count(int(j)) + 1))
		} else {
			score -= float64(nomadRace * (c + 1))
		}
	}
	if danger {
		score -= nomadDanger
	}
	return score
}

// nomadPrey counts the enemy orbs next to m.
func (b *Bot) nomadPrey(m int) int {
	n := 0
	for _, j := range b.nbrs(m) {
		if b.enemy(int(j)) {
			n += b.count(int(j))
		}
	}
	return n
}
