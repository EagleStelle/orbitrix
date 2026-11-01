package ai

import (
	"slices"

	"orbitrix/core"
)

// Raider claims corners and edges in the opening, retreats when its last cell is about to fall, and otherwise
// scores every move for pressure on the weakest opponent, flanking enemy clusters and comebacks.

const (
	raiderSimMax        = 16
	raiderLossThreshold = 0.92 // behind the strongest opponent by more than this ratio: comeback mode
)

type cluster struct{ total, size int32 }

func (b *Bot) raider() int {
	g := b.g
	var territory [core.MaxPlayers]int
	owned, myCell := 0, -1
	for i, o := range g.Owner {
		if o != core.NoOwner {
			territory[o]++
			owned++
		}
		if o == b.me {
			myCell = i
		}
	}
	myTotal, myTerr := g.Orbs[b.me], territory[b.me]
	b.dist[0] = b.distances(b.dist[0], b.enemy)
	enemyDist := b.dist[0]

	if myTerr == 1 && b.threats(myCell) >= 1 {
		return b.raiderRetreat(myCell, enemyDist)
	}

	// Opening. The prototype first looks up move counts on its Game, which keeps none, so this heuristic always decides.
	opening := false
	if myTerr <= 3 && myTotal <= 8 {
		d := int16(0)
		if myTerr > 0 {
			d = enemyDist[myCell]
			for i := range g.Owner {
				if b.mine(i) {
					d = min(d, enemyDist[i])
				}
			}
		}
		opening = d >= 4
	}
	if !opening && float64(owned)/float64(b.geo.Cells()) < 0.30 && myTerr <= 3 && myTotal <= 6 {
		opening = true
	}
	if opening {
		return b.raiderOpening()
	}
	return b.raiderAttack(territory)
}

// suicidal reports whether an orb at i would leave it one short of exploding (or exploding) next to two or more
// enemy cells that can pop first: the prototype's obvious_suicide_check.
func (b *Bot) suicidal(i int) bool {
	return b.count(i)+1 >= b.capacity(i)-1 && b.threats(i) >= 2
}

// raiderRetreat plays away from the enemy when our only cell is under threat: farthest from enemies first, then
// empty cells over our own, then fuller cells; moves with no primed enemy neighbour before the rest.
func (b *Bot) raiderRetreat(dying int, enemyDist []int16) int {
	var safeKey, fallKey [3]int
	safe, fall := -1, -1
	for _, m := range b.moves {
		if m == dying {
			continue
		}
		owner := int(b.g.Owner[m])
		key := [3]int{int(enemyDist[m]), -owner, b.count(m)}
		if b.threats(m) == 0 {
			if safe < 0 || slices.Compare(key[:], safeKey[:]) > 0 {
				safe, safeKey = m, key
			}
		} else if fall < 0 || slices.Compare(key[:], fallKey[:]) > 0 {
			fall, fallKey = m, key
		}
	}
	switch {
	case safe >= 0:
		return safe
	case fall >= 0:
		return fall
	}
	for _, m := range b.moves {
		if b.threats(m) < 2 {
			return m
		}
	}
	return b.pick(b.moves)
}

// raiderOpening claims corners, then edges, then safe cells next to ours.
func (b *Bot) raiderOpening() int {
	list := b.list[:0]
	for _, m := range b.moves {
		if b.corner(m) {
			score := 220.0
			if b.mine(m) {
				score = 200
			}
			list = append(list, scored{score - float64(b.threats(m)*90) + b.uniform(-0.4, 0.4), m})
		}
	}
	sortDesc(list)
	for _, s := range list {
		if !b.suicidal(s.cell) {
			return s.cell
		}
	}

	list = list[:0]
	for _, m := range b.moves {
		if !b.edge(m) || b.corner(m) {
			continue
		}
		score := 160.0
		if b.mine(m) {
			score = float64(140 + b.count(m)*10)
		}
		mine, _, empty := b.around(m)
		score += float64(mine*26+empty*6-b.threats(m)*80) + b.uniform(-0.3, 0.3)
		list = append(list, scored{score, m})
	}
	sortDesc(list)
	for _, s := range list {
		if !b.suicidal(s.cell) {
			return s.cell
		}
	}

	list = list[:0]
	for _, m := range b.moves {
		mine, _, _ := b.around(m)
		if b.threats(m) >= 2 || !b.mine(m) && mine == 0 {
			continue
		}
		score := 104.0 // empty
		if b.mine(m) {
			score = float64(120 + b.count(m)*18)
		}
		list = append(list, scored{score + b.uniform(-0.3, 0.3), m})
	}
	b.list = list
	if len(list) > 0 {
		sortDesc(list)
		return list[0].cell
	}

	for _, m := range b.moves {
		if b.threats(m) < 2 {
			return m
		}
	}
	return b.pick(b.moves)
}

// raiderAttack scores every reasonable move: preying on the weakest opponent, building next to our cells,
// pressing nearby enemy mass, flanking between enemy clusters, and in comeback mode closing on the biggest cluster.
// It simulates the first raiderSimMax promising moves.
func (b *Bot) raiderAttack(territory [core.MaxPlayers]int) int {
	g, me := b.g, b.me
	myTotal := g.Orbs[me]

	// Weakest opponent: fewest orbs, then least territory, then at random. The prototype's "last mover" tie-break
	// reads history its Game doesn't keep, so it never applies.
	weakest, strongest := -1, -1
	b.ties = b.ties[:0]
	for p := 0; p < g.Players; p++ {
		if p == int(me) || g.Orbs[p] == 0 {
			continue
		}
		if strongest < 0 || g.Orbs[p] > g.Orbs[strongest] {
			strongest = p
		}
		if len(b.ties) == 0 || g.Orbs[p] < g.Orbs[b.ties[0]] {
			b.ties = b.ties[:0]
		}
		if len(b.ties) == 0 || g.Orbs[p] == g.Orbs[b.ties[0]] {
			b.ties = append(b.ties, p)
		}
	}
	if len(b.ties) == 1 {
		weakest = b.ties[0]
	} else if len(b.ties) > 1 {
		least := territory[b.ties[0]]
		for _, p := range b.ties {
			least = min(least, territory[p])
		}
		b.ties = slices.DeleteFunc(b.ties, func(p int) bool { return territory[p] != least })
		weakest = b.pick(b.ties)
	}
	strongestTotal := int32(0)
	if strongest >= 0 {
		strongestTotal = g.Orbs[strongest]
	}

	b.findClusters()
	losing := strongestTotal > 0 && float64(myTotal) < float64(strongestTotal)*raiderLossThreshold
	comeback := -1
	if losing && len(b.clusters) > 0 {
		// The biggest cluster by orbs, then by cells, then at random.
		b.ties = b.ties[:0]
		for id, cl := range b.clusters {
			if len(b.ties) > 0 {
				best := b.clusters[b.ties[0]]
				if cl.total < best.total || cl.total == best.total && cl.size < best.size {
					continue
				}
				if cl.total > best.total || cl.size > best.size {
					b.ties = b.ties[:0]
				}
			}
			b.ties = append(b.ties, id)
		}
		comeback = b.pick(b.ties)
	}
	var comebackDist []int16
	if comeback >= 0 {
		b.dist[1] = b.distances(b.dist[1], func(i int) bool { return b.cluster[i] == int32(comeback) })
		comebackDist = b.dist[1]
	}
	b.dist[2] = b.distances(b.dist[2], b.mine)
	nearMine := b.dist[2]

	othersBefore := b.others(g.Orbs)
	sims := 0
	list := b.list[:0]
	for _, m := range b.moves {
		if !losing && b.suicidal(m) {
			continue
		}
		list = append(list, scored{0, m})
	}
	if len(list) == 0 {
		for _, m := range b.moves {
			list = append(list, scored{0, m})
		}
	}
	for k := range list {
		m := list[k].cell
		c, cp, own := b.count(m), b.capacity(m), b.mine(m)
		mine, enemies, _ := b.around(m)
		score := float64(mine * 36) // connectivity
		if own {
			score += float64(160 + c*30)
		} else {
			score += float64(60 + mine*28)
		}
		score += b.frontier(m) * 48

		if b.flanks(m) {
			score += 300
		}
		if comeback >= 0 {
			// The prototype's bonus for playing inside the comeback cluster needs an enemy cell, never a legal move.
			switch total := float64(b.clusters[comeback].total); {
			case comebackDist[m] <= 1:
				score += total * 14
			case comebackDist[m] <= 3:
				score += total * 6
			}
		}
		if p := b.threats(m); p > 0 {
			score -= float64(p * 140)
			if losing {
				score += float64(p * 80) // accept more risk in a comeback
			}
		}
		if d := int(nearMine[m]); d > 6 {
			score -= float64((d - 6) * 20) // too far from our network
		}

		var gain, loss int32
		if (!own || c >= cp-1 || score > 240 || enemies >= 1) && sims < raiderSimMax {
			sims++
			if t, ok := b.tally(m); ok {
				if b.wins(t) {
					return m
				}
				gain, loss = t[me]-myTotal, othersBefore-b.others(t)
				score += float64(max(gain, 0)*60 + max(loss, 0)*44)
				if weakest >= 0 && g.Orbs[weakest] > t[weakest] {
					score += float64((g.Orbs[weakest] - t[weakest]) * 64)
				}
			} else {
				score -= 8
			}
		}
		if b.suicidal(m) && gain <= 0 && loss <= 0 {
			score -= 200
		}
		list[k].score = score + b.uniform(-0.4, 0.4)
	}
	b.list = list
	sortDesc(list)

	top := list[0].score
	b.ties = b.ties[:0]
	for _, s := range list {
		if s.score >= top-1e-6 {
			b.ties = append(b.ties, s.cell)
		}
	}
	if len(list) > 3 && b.rng.Float64() < 0.06 {
		return list[b.rng.Intn(3)].cell
	}
	return b.pick(b.ties)
}

// frontier sums the fill (orbs / capacity) of enemy cells within two steps of i.
func (b *Bot) frontier(i int) float64 {
	col, row := b.geo.Coords(i)
	sum := 0.0
	for dc := -2; dc <= 2; dc++ {
		for dr := -2; dr <= 2; dr++ {
			d := max(dc, -dc) + max(dr, -dr)
			c, r := col+dc, row+dr
			if d == 0 || d > 2 || c < 0 || c >= b.geo.Cols || r < 0 || r >= b.geo.Rows {
				continue
			}
			if j := b.geo.Index(c, r); b.enemy(j) {
				sum += float64(b.count(j)) / float64(b.capacity(j))
			}
		}
	}
	return sum
}

// flanks reports whether i touches two or more different enemy clusters.
func (b *Bot) flanks(i int) bool {
	first := int32(-1)
	for _, j := range b.nbrs(i) {
		if id := b.cluster[j]; id >= 0 {
			if first >= 0 && id != first {
				return true
			}
			first = id
		}
	}
	return false
}

// findClusters labels each enemy cell with its cluster: the orthogonally connected cells of one owner, numbered in
// board order.
func (b *Bot) findClusters() {
	n := b.geo.Cells()
	b.cluster = slices.Grow(b.cluster[:0], n)[:n]
	for i := range b.cluster {
		b.cluster[i] = -1
	}
	b.clusters = b.clusters[:0]
	for start := range b.cluster {
		if b.cluster[start] >= 0 || !b.enemy(start) {
			continue
		}
		id, owner := int32(len(b.clusters)), b.g.Owner[start]
		cl := cluster{}
		stack := append(b.queue[:0], int16(start))
		b.cluster[start] = id
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			cl.total += int32(b.g.Count[i])
			cl.size++
			for _, j := range b.nbrs(int(i)) {
				if b.cluster[j] < 0 && b.g.Owner[j] == owner {
					b.cluster[j] = id
					stack = append(stack, j)
				}
			}
		}
		b.queue = stack
		b.clusters = append(b.clusters, cl)
	}
}
