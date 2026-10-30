package ai

import (
	"cmp"
	"slices"
)

// Gambler is high-risk, high-reward, with a "mood" set by how its orbs compare with the strongest opponent's.

type mood uint8

const (
	cautious  mood = iota // even game: calculated risks
	hotStreak             // well ahead: bigger risks
	desperate             // far behind: only jackpots matter
	chasing               // a little behind: chase losses
)

// riskFactor scales the payout a move needs to count as a jackpot (15×) or a big bet (5×).
var riskFactor = [...]float64{cautious: 0.9, hotStreak: 1.8, desperate: 2.5, chasing: 1.4}

type bet struct {
	score, jackpot, risk float64
	cell                 int
}

func (b *Bot) gambler() int {
	g, me := b.g, b.me
	my := float64(g.Orbs[me])
	var strongest int32
	for p := 0; p < g.Players; p++ {
		if p != int(me) {
			strongest = max(strongest, g.Orbs[p])
		}
	}
	if strongest == 0 {
		return b.pick(b.moves) // no opponent on the board yet
	}
	s := float64(strongest)
	m := cautious
	switch {
	case my > s*1.3:
		m = hotStreak
	case my < s*0.6:
		m = desperate
	case my < s*0.8:
		m = chasing
	}
	rf := riskFactor[m]

	enemyBefore := b.others(g.Orbs)
	jack, big, safe := b.bets[0][:0], b.bets[1][:0], b.bets[2][:0]
	for _, cell := range b.moves {
		t, ok := b.tally(cell)
		if !ok {
			continue
		}
		jv := float64(t[me]) - my + float64(enemyBefore-b.others(t))*0.7
		risk := 2.0 // claiming an empty cell (legal moves are never enemy cells, so no attack bonus)
		if b.mine(cell) {
			risk = 0
			enemies := 0
			for _, n := range b.nbrs(cell) {
				if b.enemy(int(n)) {
					enemies++
					risk++
					if b.primed(int(n)) {
						risk += 2 // the enemy can explode into us
					}
				}
			}
			if b.count(cell)+1 >= b.capacity(cell)-1 {
				risk += float64(enemies * 2) // fixed: the prototype never counted adjacent_enemies, so this added 0
			}
		}
		switch {
		case jv >= 15*rf:
			jack = append(jack, bet{jv - risk, jv, risk, cell})
		case jv >= 5*rf:
			big = append(big, bet{jv - risk*0.5, jv, risk, cell})
		default:
			safe = append(safe, bet{jv - risk*0.3, jv, risk, cell})
		}
	}
	b.bets = [3][]bet{jack, big, safe}

	switch m {
	case hotStreak:
		if len(jack) > 0 {
			rankBets(jack, func(x bet) float64 { return x.jackpot - x.risk*0.3 })
			return b.maybeTopThree(jack, 0.4)
		}
		if len(big) > 0 {
			rankBets(big, func(x bet) float64 { return x.score })
			return big[0].cell
		}
	case desperate:
		if len(jack) > 0 {
			rankBets(jack, func(x bet) float64 { return x.jackpot })
			return jack[0].cell
		}
		if len(big) > 0 {
			rankBets(big, func(x bet) float64 { return x.jackpot - x.risk*0.1 })
			return big[0].cell
		}
		// Any explosive potential. The prototype's second clause (non-own cell at capacity−1) can't match a legal move.
		if cell, ok := b.pickWhere(func(i int) bool { return b.mine(i) && b.count(i) >= b.capacity(i)-2 }); ok {
			return cell
		}
	case chasing:
		all := append(jack, big...)
		b.bets[0] = all // keep the grown buffer
		if len(all) > 0 {
			rankBets(all, func(x bet) float64 { return x.jackpot - x.risk*0.6 })
			return b.maybeTopThree(all, 0.3)
		}
	default: // cautious
		all := append(append(jack, big...), safe...)
		b.bets[0] = all
		if len(all) > 0 {
			// The prototype sorts first, but sorting draws no randomness, so picking the gamble first is equivalent.
			if len(jack) > 0 && b.rng.Float64() < 0.25 {
				return jack[b.rng.Intn(len(jack))].cell
			}
			rankBets(all, func(x bet) float64 { return x.score })
			return all[0].cell
		}
	}

	// Fallback: explosive potential, then expansion, then anything.
	if cell, ok := b.pickWhere(func(i int) bool { return b.mine(i) && b.count(i) >= b.capacity(i)-2 }); ok {
		return cell
	}
	if cell, ok := b.pickWhere(b.empty); ok {
		return cell
	}
	return b.pick(b.moves)
}

// maybeTopThree returns the best bet, or with probability p (when there's a choice) one of the top three.
func (b *Bot) maybeTopThree(ranked []bet, p float64) int {
	if len(ranked) > 1 && b.rng.Float64() < p {
		return ranked[b.rng.Intn(min(2, len(ranked)-1)+1)].cell
	}
	return ranked[0].cell
}

// rankBets sorts by key, highest first, keeping equal keys in their original order like Python's sort.
func rankBets(bets []bet, key func(bet) float64) {
	slices.SortStableFunc(bets, func(x, y bet) int { return cmp.Compare(key(y), key(x)) })
}
