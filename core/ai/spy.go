package ai

import (
	"math"

	"orbitrix/core"
)

// Spy simulates a bounded shortlist (explosive cells, our cells, cells next to ours, corners, then the rest) and
// scores the lead it would have over the best opponent, with a little noise so it isn't perfect. It also reads off
// the board, without simulating replies, which of its orbs enemy chains could reach next turn, and out of their reach
// it infiltrates, settling next to enemy cells it would explode before.

const (
	spyMaxEvals       = 48
	spyReads          = 8   // explosions whose resulting board is read
	spyExposureWeight = 2.0 // lead lost per orb left where an enemy chain could reach it
	spyThreatWeight   = 1.0 // lead counted now per enemy orb a safe cell would explode into first
)

func (b *Bot) spy() int {
	g, me := b.g, b.me

	// Each group starts at a random move: in board order, as in the prototype, a big board's shortlist never got past
	// the cells nearest the top-left corner.
	seen := b.marks()
	cands := b.cands[:0]
	add := func(keep func(int) bool) {
		start := b.rng.Intn(len(b.moves)) // drawn even when full, so the moves don't depend on how soon it fills
		for k := 0; k < len(b.moves) && len(cands) < spyMaxEvals; k++ {
			m := b.moves[(start+k)%len(b.moves)]
			if !seen[m] && keep(m) {
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

	lead := func(t [core.MaxPlayers]int32) float64 {
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
		return score
	}

	// Moves that can't explode share one reading of the board, taken before any explosion's board is read.
	danger, exposed := b.spyDanger(g)
	list := b.list[:0]
	best := math.Inf(-1)
	for _, m := range cands {
		if !b.primed(m) {
			t, _ := b.tally(m) // can't fail: nothing explodes
			s := lead(t) + b.spyPlace(m, danger, exposed) + b.uniform(-0.15, 0.15)
			list = append(list, scored{s, m})
			best = max(best, s)
		}
	}

	// Explosions keep the boards of the spyReads best leads (b.nodes, Tactician's scratch), which are then read best
	// first until a lead, the most a move can score, can't beat the best score. The rest count the current exposure.
	var reads [spyReads]scored // cell is the index into b.nodes; cells are in explode
	var explode [spyReads]int
	n := 0
	for _, m := range cands {
		if !b.primed(m) {
			continue
		}
		t, ok := b.tally(m)
		if !ok {
			continue
		}
		s := lead(t) + b.uniform(-0.15, 0.15)
		slot := n
		if n == spyReads {
			slot = 0
			for k := range reads {
				if reads[k].score < reads[slot].score {
					slot = k
				}
			}
			if s <= reads[slot].score {
				list = append(list, scored{s - exposed*spyExposureWeight, m})
				continue
			}
			list = append(list, scored{reads[slot].score - exposed*spyExposureWeight, explode[slot]})
		} else {
			n++
		}
		reads[slot], explode[slot] = scored{s, slot}, m
		b.nodes[slot].CopyFrom(&b.sim) // tally left the exploded board in b.sim
	}
	sortDesc(reads[:n])
	for _, r := range reads[:n] {
		s := r.score - exposed*spyExposureWeight
		if r.score > best {
			_, after := b.spyDanger(&b.nodes[r.cell])
			s = r.score - after*spyExposureWeight
			best = max(best, s)
		}
		list = append(list, scored{s, explode[r.cell]})
	}
	b.list = list
	if len(list) == 0 {
		return b.pick(b.moves)
	}
	var top [3]scored // the best three, as a stable sort of list would order them
	k := 0
	for _, s := range list {
		k = len(insertTop(top[:k], s, len(top)))
	}
	if b.rng.Float64() < 0.12 && k > 1 {
		return top[b.rng.Intn(k)].cell // natural imperfection
	}
	return top[0].cell
}

// spyDanger reads g without simulating: danger marks the cells enemy chains could reach next turn, and exposed counts
// our orbs on them. danger is b.mark, valid until the next marks call.
func (b *Bot) spyDanger(g *core.Game) (danger []bool, exposed float64) {
	danger = b.openingThreats(g, -1)
	danger, count := danger[:len(g.Owner)], g.Count[:len(g.Owner)] // lets the compiler drop bounds checks
	for i, o := range g.Owner {
		if o == b.me && danger[i] {
			exposed += float64(count[i])
		}
	}
	return danger, exposed
}

// spyPlace scores a non-explosive placement at m from the current board's danger and exposed: it loses the orb it
// adds within enemy reach, and our neighbours it would carry an enemy chain into once it is one short of exploding.
// Out of reach it infiltrates: it gains a share of the orbs of each enemy neighbour it would explode before, and
// loses the orbs it leaves next to one that would explode first.
// shortcut: a chain carried through m spreads one step; exact needs reading the board again per move.
func (b *Bot) spyPlace(m int, danger []bool, exposed float64) float64 {
	ours := b.capacity(m) - b.count(m) - 1 // our moves until m explodes, the explosion included
	if danger[m] {
		exposed++
		if ours <= 1 {
			for _, j := range b.nbrs(m) {
				if b.mine(int(j)) && !danger[j] {
					exposed += float64(b.count(int(j)))
				}
			}
		}
		return -exposed * spyExposureWeight
	}
	var prey float64
	for _, j := range b.nbrs(m) {
		if b.enemy(int(j)) && ours < b.capacity(int(j))-b.count(int(j)) { // they move next, so a tie is theirs
			prey += float64(b.count(int(j)))
		}
	}
	return prey*spyThreatWeight - exposed*spyExposureWeight
}
