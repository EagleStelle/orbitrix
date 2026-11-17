package ai

import (
	"testing"

	"orbitrix/core"
)

// TestFogBotsPlayBlind: orbs out of the bot's sight don't change its move.
func TestFogBotsPlayBlind(t *testing.T) {
	position := func(hidden bool) *core.Game {
		g, err := core.NewGame(7, 7, 2)
		if err != nil {
			t.Fatal(err)
		}
		s := g.Snapshot()
		s.Fog, s.Played, s.Moves = true, 0b11, 4
		put := func(cell, p int, n uint8) { s.Count[cell], s.Owner[cell] = n, int8(p) }
		put(0, 0, 1)
		put(8, 0, 2)
		put(48, 1, 1)
		if hidden {
			put(6, 1, 2)
			put(40, 1, 3)
		}
		if g, err = core.Restore(s); err != nil {
			t.Fatal(err)
		}
		return g
	}
	open, hidden := position(false), position(true)
	for s := range NumStrategies {
		var b Bot
		x, y := b.Choose(open, s, 7, 200), b.Choose(hidden, s, 7, 200)
		if x != y || !open.Legal(x) {
			t.Errorf("%v: %d without the hidden orbs, %d with them", s, x, y)
		}
	}
}

// TestFogBotsMoveLegally plays fog games on random maps, every strategy in turn, and checks each move.
func TestFogBotsMoveLegally(t *testing.T) {
	for seed := range uint64(4) {
		g, err := core.NewGameWithMap(core.RandomMap(7, 7, seed+1), 3)
		if err == nil {
			err = g.SetFog()
		}
		if err != nil {
			t.Fatal(err)
		}
		var bots [3]Bot
		for move := 0; move < 120 && !g.Over(); move++ {
			s := Strategy((move + int(seed)) % int(NumStrategies))
			cell := bots[g.Turn].Choose(g, s, seed+uint64(move), 60)
			if !g.Legal(cell) {
				t.Fatalf("map %d move %d: %v chose %d, out of reach", seed, move, s, cell)
			}
			if _, err := g.Apply(cell); err != nil {
				t.Fatal(err)
			}
		}
	}
}
