package core

import (
	"fmt"
	"testing"
)

var benchSizes = []int{9, 20, 40}

// BenchmarkApply resolves the worst-case chain: every cell one orb short of exploding, flooded from a corner
// until the opposite corner is captured.
func BenchmarkApply(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			base := flood(b, n, n)
			g := base.Clone()
			var waves []Wave
			b.ReportAllocs()
			for b.Loop() {
				g.CopyFrom(base)
				waves, _ = g.Apply(0)
			}
			b.ReportMetric(float64(len(waves)), "waves/op")
		})
	}
}

// BenchmarkSimulate is BenchmarkApply without wave recording, as the AI uses it.
func BenchmarkSimulate(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			base := flood(b, n, n)
			var g Game
			b.ReportAllocs()
			for b.Loop() {
				g.CopyFrom(base)
				g.Simulate(0)
			}
		})
	}
}

// BenchmarkMidgame tries every legal move from a typical mid-game position: the AI's basic search step.
func BenchmarkMidgame(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			base, _ := NewGame(n, n, 4)
			playRandom(b, base, 1, n*n)
			if base.Over() {
				b.Fatal("setup game ended early")
			}
			legal := base.LegalMoves(nil)
			var g Game
			b.ReportAllocs()
			for b.Loop() {
				for _, cell := range legal {
					g.CopyFrom(base)
					g.Simulate(cell)
				}
			}
			b.ReportMetric(float64(len(legal)), "moves/op")
		})
	}
}
