package core

import (
	"slices"
	"testing"
)

// playRandom plays uniformly random legal moves until someone wins or maxMoves is reached, checking the rules'
// invariants after every move. It returns the hash after each move.
func playRandom(t testing.TB, g *Game, seed uint64, maxMoves int) []uint64 {
	t.Helper()
	rng := NewRNG(seed)
	var legal []int
	var hashes []uint64
	for move := 0; move < maxMoves && !g.Over(); move++ {
		legal = g.LegalMoves(legal[:0])
		if len(legal) == 0 {
			t.Fatalf("move %d: player %d has no legal move", move, g.Turn)
		}
		before, mover := total(g), g.Turn
		waves, err := g.Apply(legal[rng.Intn(len(legal))])
		if err != nil {
			t.Fatal(err)
		}
		if total(g) != before+1 {
			t.Fatalf("move %d: orbs %d → %d", move, before, total(g))
		}
		checkTallies(t, g)
		checkAfterMove(t, g, mover, len(waves))
		hashes = append(hashes, g.Hash())
	}
	return hashes
}

func checkAfterMove(t testing.TB, g *Game, mover, waves int) {
	t.Helper()
	if g.Over() {
		for p := 0; p < g.Players; p++ {
			if (g.Orbs[p] > 0) != (p == int(g.Winner)) {
				t.Fatalf("winner %d, but player %d has %d orbs", g.Winner, p, g.Orbs[p])
			}
		}
		if int(g.Winner) != mover {
			t.Fatalf("player %d won on player %d's move", g.Winner, mover)
		}
		return
	}
	if waves >= MaxWaves {
		t.Fatalf("chain hit the wave limit")
	}
	for i, c := range g.Count {
		if c >= g.Geo.Cap[i] {
			t.Fatalf("cell %d left at %d orbs, capacity %d", i, c, g.Geo.Cap[i])
		}
	}
	if !g.Alive(g.Turn) || g.Turn == mover {
		t.Fatalf("turn passed from %d to %d (alive %v)", mover, g.Turn, g.Alive(g.Turn))
	}
}

// TestRandomGames plays full games over a spread of sizes and player counts, including 12 players.
func TestRandomGames(t *testing.T) {
	for _, tt := range []struct{ cols, rows, players int }{
		{3, 3, 2}, {3, 3, 4}, {5, 9, 2}, {9, 9, 3}, {8, 14, 6}, {10, 10, 12}, {3, 40, 5}, {20, 20, 12},
	} {
		for seed := uint64(0); seed < 4; seed++ {
			g, err := NewGame(tt.cols, tt.rows, tt.players)
			if err != nil {
				t.Fatal(err)
			}
			playRandom(t, g, seed, 1_000_000)
			if !g.Over() {
				t.Fatalf("%dx%d, %d players, seed %d: no winner after %d moves", tt.cols, tt.rows, tt.players, seed, g.Moves)
			}
			if g.Moves != total(g) {
				t.Fatalf("%d moves but %d orbs", g.Moves, total(g))
			}
		}
	}
}

// TestDeterminism replays the same seeded game twice, and checks Simulate on a copy lands where Apply does.
func TestDeterminism(t *testing.T) {
	a, _ := NewGame(9, 9, 4)
	b, _ := NewGame(9, 9, 4)
	if !slices.Equal(playRandom(t, a, 7, 1_000_000), playRandom(t, b, 7, 1_000_000)) {
		t.Fatal("the same seed produced different games")
	}

	g, _ := NewGame(9, 9, 3)
	playRandom(t, g, 3, 120)
	var sim Game
	for _, cell := range g.LegalMoves(nil) {
		sim.CopyFrom(g)
		if err := sim.Simulate(cell); err != nil {
			t.Fatal(err)
		}
		applied := g.Clone()
		if _, err := applied.Apply(cell); err != nil {
			t.Fatal(err)
		}
		if sim.Hash() != applied.Hash() {
			t.Fatalf("cell %d: Simulate and Apply disagree", cell)
		}
	}
}

func TestApplyDoesNotAllocate(t *testing.T) {
	base := flood(t, 20, 20)
	g := base.Clone()
	g.Apply(0) // grow the wave buffers once
	allocs := testing.AllocsPerRun(20, func() {
		g.CopyFrom(base)
		if _, err := g.Apply(0); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("Apply allocates %v times per move", allocs)
	}
}

// FuzzGame plays random games from fuzzed sizes, player counts and seeds. `go test` runs the seed corpus;
// `go test -fuzz FuzzGame ./core` explores further.
func FuzzGame(f *testing.F) {
	f.Add(uint64(1), uint8(3), uint8(3), uint8(2))
	f.Add(uint64(2), uint8(9), uint8(9), uint8(12))
	f.Add(uint64(3), uint8(3), uint8(12), uint8(7))
	f.Fuzz(func(t *testing.T, seed uint64, cols, rows, players uint8) {
		c, r, p := MinSize+int(cols)%10, MinSize+int(rows)%10, MinPlayers+int(players)%(MaxPlayers-1)
		g, err := NewGame(c, r, p)
		if p > c*r {
			if err == nil {
				t.Fatalf("%d players on %dx%d accepted", p, c, r)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		playRandom(t, g, seed, 5000)
	})
}
