package ai

import (
	"fmt"
	"testing"

	"orbitrix/core"
)

// position plays random moves on an n×n board for 4 players until the board holds about fill orbs per cell, and
// returns the game unfinished.
func position(b *testing.B, n int, fill float64) *core.Game {
	for seed := uint64(1); ; seed++ {
		g, _ := core.NewGame(n, n, 4)
		rng := core.NewRNG(seed)
		var legal []int
		for !g.Over() && float64(g.Moves) < fill*float64(n*n) {
			legal = g.LegalMoves(legal[:0])
			if _, err := g.Apply(legal[rng.Intn(len(legal))]); err != nil {
				b.Fatal(err)
			}
		}
		if !g.Over() {
			return g
		}
	}
}

// BenchmarkChoose times each strategy at its default budget, mid-game (0.5 orbs per cell) and late (1.2, where
// most cells are close to exploding and chains are long).
func BenchmarkChoose(b *testing.B) {
	for _, n := range []int{9, 20, 40} {
		for _, phase := range []struct {
			name string
			fill float64
		}{{"mid", 0.5}, {"late", 1.2}} {
			g := position(b, n, phase.fill)
			for s := range NumStrategies {
				b.Run(fmt.Sprintf("%v/%dx%d/%s", s, n, n, phase.name), func(b *testing.B) {
					var bot Bot
					b.ReportAllocs()
					seed := uint64(0)
					for b.Loop() {
						seed++
						bot.Choose(g, s, seed, DefaultBudget)
					}
				})
			}
		}
	}
}
