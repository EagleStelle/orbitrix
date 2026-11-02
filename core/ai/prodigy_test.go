package ai

import (
	"fmt"
	"math"
	"testing"

	"orbitrix/core"
)

// BenchmarkProdigyForward times one full network evaluation, for pricing netCellsPerUnit.
func BenchmarkProdigyForward(b *testing.B) {
	for _, n := range []int{9, 20, 40} {
		g := position(b, n, 0.8)
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			var e netEval
			for b.Loop() {
				forward(g, prodigyWeights[:], &e)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n*n), "ns/cell")
		})
	}
}

// TestProdigyDoesNotAllocate: after one move has sized the buffers, moves on the same board allocate nothing.
func TestProdigyDoesNotAllocate(t *testing.T) {
	g := grid(t, 2, 0, "a1 . . . .", ". b2 . . .", ". . a3 . .", ". . . b1 .", ". . . . a1")
	var bot Bot
	bot.Choose(g, Prodigy, 1, DefaultBudget)
	seed := uint64(1)
	if n := testing.AllocsPerRun(5, func() { seed++; bot.Choose(g, Prodigy, seed, DefaultBudget) }); n != 0 {
		t.Errorf("%v allocations per move", n)
	}
}

// TestProdigyUpdateMatchesForward: positions evaluated from a player's base give the same value and policy as a full
// forward pass, including after chains, eliminations and a change of who moves next.
func TestProdigyUpdateMatchesForward(t *testing.T) {
	rng := core.NewRNG(5)
	w := make([]float32, netParams)
	for i := range w {
		w[i] = float32(rng.Float64()-0.5) * 0.6
	}
	for _, size := range []int{4, 9, 20} {
		for _, players := range []int{2, 5} {
			g, _ := core.NewGame(size, size, players)
			var legal []int
			for !g.Over() && g.Moves < size*size {
				legal = g.LegalMoves(legal[:0])
				_ = g.Simulate(legal[rng.Intn(len(legal))])
			}
			if g.Over() {
				continue
			}
			var net prodigyNet
			for k := range 40 {
				h := g.Clone()
				for range k % 7 {
					if !h.Over() {
						legal = h.LegalMoves(legal[:0])
						_ = h.Simulate(legal[rng.Intn(len(legal))])
					}
				}
				if h.Over() {
					continue
				}
				win, logit, _ := net.evaluate(h, w)
				var e netEval
				want := forward(h, w, &e)
				if math.Abs(win-want) > 1e-5 {
					t.Fatalf("%dx%d, %d players, case %d: value %v, full pass %v", size, size, players, k, win, want)
				}
				for _, c := range h.LegalMoves(legal[:0]) {
					if logit[c] != e.logit[c] {
						t.Fatalf("%dx%d, %d players, case %d: cell %d logit %v, full pass %v", size, size, players, k, c,
							logit[c], e.logit[c])
					}
				}
			}
		}
	}
}
