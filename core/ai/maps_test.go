package ai

import (
	"orbitrix/core"
	"testing"
)

func TestBotsOnCustomMaps(t *testing.T) {
	barriers := core.NewMap(5, 5)
	barriers.Name = "Breakable barriers"
	barriers.Edges = []core.MapEdge{{A: 12, B: 13, Kind: core.BreakableBarrier}, {A: 12, B: 17, Kind: core.BreakableBarrier}}
	for _, m := range append(core.StarterMaps(), barriers) {
		t.Run(m.Name, func(t *testing.T) {
			g, err := core.NewGameWithMap(m, 3)
			if err != nil {
				t.Fatal(err)
			}
			rng := core.NewRNG(321)
			var legal []int
			for ply := 0; ply < 24 && !g.Over(); ply++ {
				legal = g.LegalMoves(legal[:0])
				_, _ = g.Apply(legal[rng.Intn(len(legal))])
			}
			if g.Over() {
				return
			}
			for strategy := Strategy(0); strategy < NumStrategies; strategy++ {
				t.Run(strategy.String(), func(t *testing.T) {
					var bot Bot
					before := g.Hash()
					first := bot.Choose(g, strategy, 123, 40)
					second := bot.Choose(g, strategy, 123, 40)
					if !g.Legal(first) {
						t.Fatalf("illegal move %d", first)
					}
					if first != second {
						t.Fatalf("nondeterministic moves %d %d", first, second)
					}
					if g.Hash() != before {
						t.Fatal("AI mutated game")
					}
				})
			}
		})
	}
}
