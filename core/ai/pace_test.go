package ai

import (
	"os"
	"runtime"
	"sync"
	"testing"

	"orbitrix/core"
)

// TestPaceFit plays bot games on maps with 1 to 4 spread copies of each piece and logs their length against an empty
// board's, the data MapDefinition.Estimate's weights are fit to. It takes minutes, so it runs only with
// ORBITRIX_PACE=1. Add a new piece to pieces, compare its slowdown with a Hole's, and set its weight from that.
func TestPaceFit(t *testing.T) {
	if os.Getenv("ORBITRIX_PACE") != "1" {
		t.Skip("set ORBITRIX_PACE=1 to measure game lengths")
	}
	pieces := []core.MapCell{{Terrain: core.Hole}, {Terrain: core.Landfill}, {Terrain: core.Sinkhole}, {Terrain: core.BreakableFloor}, {Feature: core.Spillway}, {Feature: core.Crusher}, {Feature: core.Furnace}, {Feature: core.Reservoir}}
	for _, size := range []int{9, 15} {
		empty := paceMoves(core.NewMap(size, size))
		emptyMoves, _, _ := core.NewMap(size, size).Estimate()
		t.Logf("%d×%d empty: %.0f moves", size, size, empty)
		for _, piece := range pieces {
			for k := 1; k <= 4; k++ {
				m := spreadPieces(size, k, piece)
				if len(m.Validate()) != 0 {
					break // Endless or Instant from here on
				}
				moves, length, pace := m.Estimate()
				t.Logf("%d×%d %d×%+v: played %.0f‰ of empty, estimate %d‰ %v, moves %d‰", size, size, k, piece, 1000*paceMoves(m)/empty, length, pace, moves*1000/emptyMoves)
			}
		}
	}
}

// spreadPieces places k copies of piece on interior cells with no other piece in their 3×3 neighborhood.
func spreadPieces(size, k int, piece core.MapCell) core.MapDefinition {
	m := core.NewMap(size, size)
	for j := 0; k > 0 && j < 10*len(m.Cells); j++ {
		i := j * 37 % len(m.Cells)
		c, r := i/size, i%size
		if c < 1 || r < 1 || c > size-2 || r > size-2 {
			continue
		}
		free := true
		for dc := -1; dc <= 1; dc++ {
			for dr := -1; dr <= 1; dr++ {
				free = free && m.Cells[(c+dc)*size+r+dr] == core.MapCell{}
			}
		}
		if free {
			m.Cells[i] = piece
			k--
		}
	}
	return m
}

// paceMoves is the mean length of 96 games between 2 to 4 seats drawn from weak and strong bots, capped at 3,000 moves.
func paceMoves(m core.MapDefinition) float64 {
	pool := []Strategy{Mason, Warden, Gambler, Nomad, Raider, Spy, Tactician, Voyager, Hunter}
	const games = 96
	var mu sync.Mutex
	var wg sync.WaitGroup
	total, sem := 0, make(chan struct{}, runtime.NumCPU())
	for seed := range games {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			rng := core.NewRNG(uint64(seed)*7919 + 17)
			g, err := core.NewGameWithMap(m, 2+rng.Intn(3))
			if err != nil {
				panic(err)
			}
			seats := make([]Strategy, g.Players)
			for p := range seats {
				seats[p] = pool[rng.Intn(len(pool))]
			}
			var bot Bot
			moves := 0
			for ; !g.Over() && moves < 3000; moves++ {
				_, _ = g.Apply(bot.Choose(g, seats[g.Turn], uint64(seed)<<32|uint64(moves), DefaultBudget))
			}
			mu.Lock()
			total += moves
			mu.Unlock()
		}()
	}
	wg.Wait()
	return float64(total) / games
}
