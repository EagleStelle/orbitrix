package ai

// Architect builds its own cells up towards capacity to set up chain reactions.

const architectMaxSims = 25

func (b *Bot) architect() int {
	// Candidates: up to 15 of our cells, then 10 empty cells next to ours or on the border, then 5 others. The
	// prototype's enemy-cell options can't match a legal move.
	cands := b.cands[:0]
	for pass, limit := range [...]int{15, 10, 5} {
		n := 0
		for _, m := range b.moves {
			if n == limit {
				break
			}
			var in bool
			switch pass {
			case 0:
				in = b.mine(m)
			case 1:
				mine, _, _ := b.around(m)
				in = b.empty(m) && (mine > 0 || b.edge(m))
			default:
				mine, _, _ := b.around(m)
				in = b.empty(m) && mine == 0 && !b.edge(m)
			}
			if in {
				cands = append(cands, m)
				n++
			}
		}
	}
	b.cands = cands

	me, orbs, othersBefore := b.me, b.g.Orbs, b.others(b.g.Orbs)
	maxSims, sims := min(architectMaxSims, len(cands)), 0
	b.ties = b.ties[:0]
	top := 0.0
	for _, m := range cands {
		c, cp, own := b.count(m), b.capacity(m), b.mine(m)
		mine, _, _ := b.around(m)
		score := float64(mine * 40)
		if !own {
			score += 70
		} else if c < cp-1 {
			score += float64(220 + c*25 - (cp-1-c)*5)
		}

		// Fixed: the prototype's `limit and a or b or c` applied the simulation limit to the first condition only.
		if sims < maxSims && (own && c >= cp-2 || !own && mine > 0) {
			sims++
			if t, ok := b.tally(m); ok {
				if b.wins(t) {
					return m
				}
				self, opp := int(t[me]-orbs[me]), int(othersBefore-b.others(t))
				if self > 0 {
					score += float64(240 + self*70)
				}
				if opp > 0 {
					score += float64(180 + opp*45)
				}
				if self > 5 || opp > 3 {
					for _, n := range b.nbrs(m) {
						if b.enemy(int(n)) && b.primed(int(n)) {
							// Orbs are conserved, so self = opp+1 and the prototype's "trap we set" (+50 when opp > self)
							// never fires: a primed enemy next door is always a −80 exposure.
							score -= 80
						}
					}
				}
			} else {
				score -= 10
			}
		}
		if own && c == cp-2 {
			score -= float64(b.threats(m) * 70)
		}
		b.offer(&top, score, m, 0)
	}
	if len(b.ties) == 0 {
		return b.pick(b.moves)
	}
	return b.pick(b.ties)
}
