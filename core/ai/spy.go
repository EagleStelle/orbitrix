package ai

// Spy simulates a bounded shortlist (explosive cells, our cells, cells next to ours, corners, then the rest) and
// scores the lead it would have over the best opponent, with a little noise so it isn't perfect.

const spyMaxEvals = 48

func (b *Bot) spy() int {
	g, me := b.g, b.me

	seen := b.marks()
	cands := b.cands[:0]
	add := func(keep func(int) bool) {
		for _, m := range b.moves {
			if len(cands) < spyMaxEvals && !seen[m] && keep(m) {
				seen[m] = true
				cands = append(cands, m)
			}
		}
	}
	add(func(i int) bool { return b.mine(i) && b.primed(i) })
	add(b.mine)
	add(func(i int) bool { mine, _, _ := b.around(i); return mine > 0 })
	add(b.corner)
	add(func(int) bool { return true })
	b.cands = cands

	strongest := -1 // first opponent with the most orbs, if any has some
	for p := 0; p < g.Players; p++ {
		if p != int(me) && g.Orbs[p] > 0 && (strongest < 0 || g.Orbs[p] > g.Orbs[strongest]) {
			strongest = p
		}
	}

	list := b.list[:0]
	for _, m := range cands {
		t, ok := b.tally(m)
		if !ok {
			continue
		}
		var worst int32
		for p := 0; p < g.Players; p++ {
			if p != int(me) {
				worst = max(worst, t[p])
			}
		}
		score := float64(t[me]-worst) + float64(t[me]-g.Orbs[me])*0.35
		if strongest >= 0 {
			score += float64(g.Orbs[strongest]-t[strongest]) * 0.45
		}
		list = append(list, scored{score + b.uniform(-0.15, 0.15), m})
	}
	b.list = list
	if len(list) == 0 {
		return b.pick(b.moves)
	}
	sortDesc(list)
	if b.rng.Float64() < 0.12 && len(list) > 1 {
		return list[b.rng.Intn(min(3, len(list)))].cell // natural imperfection
	}
	return list[0].cell
}
