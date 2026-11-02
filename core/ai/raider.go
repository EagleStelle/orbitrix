package ai

import (
	"cmp"
	"math"
	"slices"

	"orbitrix/core"
)

// Raider is aggressive: it claims corners and edges in the opening, then stacks its cells against enemy cells, wedges
// between enemy groups, preys on the weakest opponent and simulates its most promising explosions. Every move is
// scored from its own neighbourhood, so a move costs little beyond those simulations.

const (
	raiderSimMax        = 16   // explosions simulated per move
	raiderLossThreshold = 0.92 // behind the strongest opponent by more than this ratio: take more risks
	raiderPressure      = 48   // score per full enemy cell within two steps
)

func (b *Bot) raider() int {
	owned, mine := 0, 0
	for _, o := range b.g.Owner {
		if o != core.NoOwner {
			owned++
		}
		if o == b.me {
			mine++
		}
	}
	if mine <= 3 && b.g.Orbs[b.me] <= 6 && owned*10 < b.geo.Cells()*3 {
		return b.raiderOpening()
	}
	return b.raiderAttack()
}

// suicidal reports whether an orb at i would leave it one short of exploding (or exploding) next to two or more
// enemy cells that can pop first: the prototype's obvious_suicide_check.
func (b *Bot) suicidal(i int) bool {
	return b.count(i)+1 >= b.capacity(i)-1 && b.threats(i) >= 2
}

// raiderOpening claims corners, then edges, then cells at or next to ours, never a suicidal one while another move
// is left.
func (b *Bot) raiderOpening() int {
	best, top := b.moves[0], math.Inf(-1)
	for _, m := range b.moves {
		mine, _, empty := b.around(m)
		score := float64(1000*(4-b.capacity(m)) + mine*26 + empty*6 - b.threats(m)*85)
		if b.mine(m) {
			score += float64(b.count(m)*15 - 20)
		}
		if b.suicidal(m) {
			score -= 5000
		}
		if score += b.uniform(-0.4, 0.4); score > top {
			best, top = m, score
		}
	}
	return best
}

// raiderAttack scores every move from its neighbourhood, then simulates the raiderSimMax best-scored explosions for
// captures, with a bonus for orbs taken from the weakest opponent.
func (b *Bot) raiderAttack() int {
	g, me := b.g, b.me
	prey, strongest := -1, int32(0)
	for p := 0; p < g.Players; p++ {
		if p == int(me) || g.Orbs[p] == 0 {
			continue
		}
		strongest = max(strongest, g.Orbs[p])
		if prey < 0 || g.Orbs[p] < g.Orbs[prey] {
			prey = p
		}
	}
	losing := float64(g.Orbs[me]) < float64(strongest)*raiderLossThreshold
	b.findFrontier()

	list := b.list[:0]
	for pass := 0; pass < 2 && len(list) == 0; pass++ { // the second pass allows suicide when nothing else is left
		for _, m := range b.moves {
			score, suicidal := b.raiderScore(m, losing)
			if pass == 0 && !losing && suicidal {
				continue
			}
			list = append(list, scored{score, m})
		}
	}
	b.list = list

	sims := b.ties[:0] // indexes into list of the explosions worth simulating, best-scored first
	for k, s := range list {
		if b.primed(s.cell) && !b.quiet(s.cell) {
			sims = append(sims, k)
		}
	}
	slices.SortStableFunc(sims, func(x, y int) int { return cmp.Compare(list[y].score, list[x].score) })
	b.ties = sims
	others := b.others(g.Orbs)
	for _, k := range sims[:min(len(sims), raiderSimMax)] {
		m := list[k].cell
		t, ok := b.tally(m)
		if !ok {
			list[k].score -= 8
			continue
		}
		if b.wins(t) {
			return m
		}
		list[k].score += float64(max(t[me]-g.Orbs[me], 0)*60 + max(others-b.others(t), 0)*44)
		if prey >= 0 {
			list[k].score += float64(max(g.Orbs[prey]-t[prey], 0) * 64)
		}
	}

	best := 0
	for k := range list {
		list[k].score += b.uniform(-0.4, 0.4)
		if list[k].score > list[best].score {
			best = k
		}
	}
	if len(list) > 3 && b.rng.Float64() < 0.06 { // now and then one of the best three
		sortDesc(list)
		return list[b.rng.Intn(3)].cell
	}
	return list[best].cell
}

// raiderScore is a move's score before simulating, from one look at its neighbours: stacking and connecting our
// cells, pressing enemy cells, wedging between them and not feeding primed ones. It also reports whether the move is
// suicidal. A move whose outcome is known without simulating gains its one orb.
func (b *Bot) raiderScore(m int, losing bool) (score float64, suicidal bool) {
	c, cp, own := b.count(m), b.capacity(m), b.mine(m)
	mine, enemies, threats := 0, 0, 0
	for _, j := range b.nbrs(m) {
		switch o := b.g.Owner[j]; {
		case o == b.me:
			mine++
		case o != core.NoOwner:
			enemies++
			if b.primed(int(j)) {
				threats++
			}
		}
	}
	score = float64(mine*36) + b.frontier[m]*raiderPressure
	if own {
		score += float64(160 + c*30)
	} else {
		score += float64(60 + mine*28)
	}
	if enemies >= 2 && b.flanks(m) {
		score += 300
	}
	if threats > 0 {
		score -= float64(threats * 140)
		if losing {
			score += float64(threats * 80) // accept more risk when behind
		}
	}
	suicidal = c+1 >= cp-1 && threats >= 2 // as b.suicidal(m)
	if suicidal {
		score -= 200
	}
	if c+1 < cp || b.quiet(m) {
		score += 60
	}
	return score, suicidal
}

// findFrontier fills b.frontier with each cell's sum of the fill (orbs / capacity) of enemy cells within two steps
// of it, spreading each enemy cell's fill once instead of gathering it for every move.
func (b *Bot) findFrontier() {
	cols, rows := b.geo.Cols, b.geo.Rows
	f := slices.Grow(b.frontier[:0], cols*rows)[:cols*rows]
	clear(f)
	for col := range cols {
		for row := range rows {
			e := col*rows + row // core.Geometry.Index
			if !b.enemy(e) {
				continue
			}
			fill := float64(b.g.Count[e]) / float64(b.geo.Cap[e])
			for dc := max(-2, -col); dc <= min(2, cols-1-col); dc++ {
				span := 2 - max(dc, -dc)
				base := e + dc*rows
				for dr := max(-span, -row); dr <= min(span, rows-1-row); dr++ {
					f[base+dr] += fill
				}
			}
			f[e] -= fill // the loop above counted the enemy cell itself
		}
	}
	b.frontier = f
}

// flanks reports whether i wedges between enemy groups: it touches two enemy cells of different owners, or of one
// owner that don't meet at the cell diagonal to i between them.
// shortcut: two cells joined by a longer path still count as apart; exact needs labelling every enemy group per move.
func (b *Bot) flanks(i int) bool {
	ns := b.nbrs(i)
	for x, j := range ns {
		for _, k := range ns[x+1:] {
			if b.enemy(int(j)) && b.enemy(int(k)) && (b.g.Owner[j] != b.g.Owner[k] || !b.corners(int(j), int(k), i)) {
				return true
			}
		}
	}
	return false
}

// corners reports whether j and k, both neighbours of i, share another neighbour owned like j.
func (b *Bot) corners(j, k, i int) bool {
	for _, c := range b.nbrs(j) {
		if int(c) != i && b.g.Owner[c] == b.g.Owner[j] && slices.Contains(b.nbrs(k), c) {
			return true
		}
	}
	return false
}

// quiet reports whether exploding i can't chain or capture, because no neighbour is an enemy's or primed. Its outcome
// is then known without simulating: one more orb, nothing taken.
func (b *Bot) quiet(i int) bool {
	for _, j := range b.nbrs(i) {
		if b.enemy(int(j)) || b.primed(int(j)) {
			return false
		}
	}
	return true
}
