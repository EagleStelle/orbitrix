package ai

import "math"

// Sentinel is defensive and reactive: it answers enemies that touch its cells, then prefers safe corners, then
// edges, then a short simulated search.

const (
	sentinelSafeBuffer  = 2 // keep this far from enemies unless attacking
	sentinelTopSim      = 8 // cheap-scored candidates to simulate in the fallback
	sentinelPrimeDanger = 1 // most primed enemy neighbours a defensive prime may have
)

func (b *Bot) sentinel() int {
	if cell, ok := b.sentinelReact(); ok {
		return cell
	}

	// Safe corners, taking a corner explosion at once if it wins (or if it can't be simulated, as the prototype did).
	b.ties = b.ties[:0]
	enemyInCorner := false
	for _, m := range b.moves {
		if !b.corner(m) {
			continue
		}
		if b.primed(m) {
			if t, ok := b.tally(m); !ok || b.wins(t) {
				return m
			}
		}
		if !(b.count(m)+1 == b.capacity(m)-1 && b.threats(m) > 0) {
			b.ties = append(b.ties, m)
		}
	}
	if len(b.ties) > 0 {
		return b.pick(b.ties)
	}
	for _, i := range [...]int{0, b.geo.Rows - 1, b.geo.Cells() - b.geo.Rows, b.geo.Cells() - 1} {
		enemyInCorner = enemyInCorner || b.enemy(i)
	}

	// Edges. The prototype's enemy-cell branch and its simulations can't run: enemy cells are never legal moves.
	enemyDist := b.distances(b.dist[0], b.enemy)
	b.dist[0] = enemyDist
	base := 140.0
	if enemyInCorner {
		base = 80
	}
	b.ties = b.ties[:0]
	top := 0.0
	for _, m := range b.moves {
		if !b.edge(m) || b.corner(m) {
			continue
		}
		c, cp := b.count(m), b.capacity(m)
		score := base
		if b.empty(m) {
			score += 60
		} else {
			score += 30
			if c < cp-1 {
				score += float64(30 + c*12)
			}
			if c+1 == cp-1 {
				score += 70
			}
		}
		_, enemies, _ := b.around(m)
		score += float64(enemies * 36)
		if d := int(enemyDist[m]); d < sentinelSafeBuffer {
			score -= float64(300 + (sentinelSafeBuffer-d)*120)
		}
		if c+1 == cp-1 && b.threats(m) > 0 {
			score -= 1000 // priming next to a cell that can pop us
		}
		score += b.rng.Float64() * 0.001
		b.offer(&top, score, m, 1e-9)
	}
	if len(b.ties) > 0 && top > -900 {
		return b.pick(b.ties)
	}

	return b.sentinelFallback(enemyDist)
}

// sentinelReact answers enemy cells touching ours. The prototype has no move history, so every such cell counts
// as provoking. Its options A and D target the enemy cell itself, which is never a legal move, so only these remain:
// B, explode our primed neighbour into it (the prototype simulated first but played it either way), and C, prime a
// neighbour to punish it if few enemies could pop that neighbour first.
func (b *Bot) sentinelReact() (int, bool) {
	for e := range b.g.Owner {
		if !b.enemy(e) {
			continue
		}
		if mine, _, _ := b.around(e); mine == 0 {
			continue
		}
		for _, n := range b.nbrs(e) {
			if b.mine(int(n)) && b.primed(int(n)) {
				return int(n), true
			}
		}
		b.ties = b.ties[:0]
		low := sentinelPrimeDanger + 1
		for _, n := range b.nbrs(e) {
			i := int(n)
			if !b.mine(i) || b.count(i)+1 != b.capacity(i)-1 {
				continue
			}
			d := b.threats(i)
			if d > sentinelPrimeDanger || d > low {
				continue
			}
			if d < low {
				low, b.ties = d, b.ties[:0]
			}
			b.ties = append(b.ties, i) // least danger wins, ties at random like the prototype's random sort key
		}
		if len(b.ties) > 0 {
			return b.pick(b.ties), true
		}
	}
	return -1, false
}

// sentinelFallback cheaply scores every move, then simulates the best few.
func (b *Bot) sentinelFallback(enemyDist []int16) int {
	list := b.list[:0]
	for _, m := range b.moves {
		c, cp := b.count(m), b.capacity(m)
		score := 0.0
		switch {
		case b.corner(m):
			score += 120
		case b.edge(m):
			score += 30
		}
		if b.empty(m) {
			score += 45
		} else {
			score += float64(24 + c*10)
			if c+1 == cp-1 {
				score += 90
			}
		}
		mine, enemies, _ := b.around(m)
		score += float64(enemies*34 + mine*10)
		if c+1 == cp-1 {
			if b.threats(m) > 0 {
				score -= 240
			} else if b.edge(m) {
				score -= 20
			}
		}
		if d := int(enemyDist[m]); d < sentinelSafeBuffer {
			score -= float64(220 + (sentinelSafeBuffer-d)*60)
		}
		score += b.rng.Float64() * 0.001
		list = append(list, scored{score, m})
	}
	b.list = list
	sortDesc(list)

	me, orbs, othersBefore := b.me, b.g.Orbs, b.others(b.g.Orbs)
	b.ties = b.ties[:0]
	top := math.Inf(-1)
	for _, s := range list[:min(sentinelTopSim, len(list))] {
		var score float64
		t, ok := b.tally(s.cell)
		if !ok {
			score = -40 + b.rng.Float64()
		} else {
			if b.wins(t) {
				return s.cell
			}
			self, opp := float64(t[me]-orbs[me]), float64(othersBefore-b.others(t))
			score = self*140 + opp*72
			if self > 0 {
				score += self * 12
			}
			if b.count(s.cell)+1 == b.capacity(s.cell)-1 && b.threats(s.cell) > 0 {
				score -= 300
			}
			score += s.score * 0.02
		}
		b.offer(&top, score, s.cell, 1e-9)
	}
	// The prototype's later fallbacks are unreachable: at least one candidate is always scored above.
	return b.pick(b.ties)
}
