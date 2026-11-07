package boardview

import (
	"math"
	"slices"
	"testing"

	"orbitrix/core"
)

// TestViewReplaysMoves plays random games and checks that replaying each move's waves on the view, as the board
// animation does, ends exactly on the game's state.
func TestViewReplaysMoves(t *testing.T) {
	sizes := []struct{ cols, rows, players int }{{3, 3, 2}, {5, 9, 4}, {9, 9, 3}, {12, 7, 12}}
	for _, sz := range sizes {
		for seed := range uint64(20) {
			g, err := core.NewGame(sz.cols, sz.rows, sz.players)
			if err != nil {
				t.Fatal(err)
			}
			v := NewView(g)
			rng := core.NewRNG(seed)
			var moves []int
			for !g.Over() && g.Moves < 2000 {
				moves = g.LegalMoves(moves[:0])
				cell, mover := moves[rng.Intn(len(moves))], int8(g.Turn)
				waves, err := g.Apply(cell)
				if err != nil {
					t.Fatal(err)
				}
				v.Place(cell, mover)
				for _, w := range waves {
					v.Launch(w)
					v.Land(w)
				}
				if !slices.Equal(v.Count, g.Count) || !slices.Equal(v.Owner, g.Owner) {
					t.Fatalf("%dx%d/%d seed %d move %d: view differs from game", sz.cols, sz.rows, sz.players, seed, g.Moves)
				}
			}
		}
	}
}

func TestWaveTicks(t *testing.T) {
	tests := []struct {
		k                int
		tailCap, skipCap float64
		want             float64
	}{
		{0, tailCap(3), 0, transferTicks},
		{4, tailCap(10000), 0, transferTicks},
		{5, tailCap(10), 0, transferTicks * 0.9},
		{60, tailCap(80), 0, 2.5},
		{5, tailCap(10000), 0, transferTicks * 0.7},
		{30, tailCap(10000), 0, tailCap(10000)},
		{0, tailCap(3), 0.5, 0.5},
	}
	for _, tc := range tests {
		if got := waveTicks(tc.k, tc.tailCap, tc.skipCap); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("waveTicks(%d, %v, %v) = %v, want %v", tc.k, tc.tailCap, tc.skipCap, got, tc.want)
		}
	}
	// However long the chain, it plays in about chainBudget ticks; after a skip, the rest in skipBudget.
	for _, n := range []int{1, 50, 500, 10000} {
		total, rest := 0.0, 0.0
		for k := range n {
			total += waveTicks(k, tailCap(n), 0)
			rest += waveTicks(k, tailCap(n), skipBudget/float64(n))
		}
		if total > chainBudget+40 || rest > skipBudget+1e-6 {
			t.Errorf("%d waves take %.0f ticks, %.0f after a skip", n, total, rest)
		}
	}
}

func TestCellSize(t *testing.T) {
	tests := []struct {
		cols, rows int
		w, h, want float64
	}{
		{9, 9, 1720, 1080, 80},
		{40, 40, 1720, 1080, 24},
		{20, 20, 1000, 700, 26},
	}
	for _, tc := range tests {
		if got := cellSize(tc.cols, tc.rows, tc.w, tc.h); got != tc.want {
			t.Errorf("cellSize(%d, %d, %v, %v) = %v, want %v", tc.cols, tc.rows, tc.w, tc.h, got, tc.want)
		}
	}
}

// TestOrbsStayInCell runs every cell of a board through a few seconds of motion at every count and checks that
// orbs stay finite and inside their orbit (a lone orb only jitters about the centre), that every cell size shows the
// same picture scaled, and that updating doesn't allocate.
func TestOrbsStayInCell(t *testing.T) {
	geo, err := core.NewGeometry(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	f, ref := newOrbField(geo), newOrbField(geo)
	r := orbRadius(orbCell)
	for _, cell := range []float64{24, 60, 80} {
		s := cell / orbCell
		for i := range geo.Cells() {
			for count := 1; count <= 7; count++ {
				limit := max(2, orbCell/2-r-12) * s
				if count == 1 {
					limit = 3 * max(0.6, r*0.28) * s
				}
				for tms := 0.0; tms < 3000; tms += 16.7 {
					ps, want := f.update(i, count, int(geo.Cap[i]), tms, cell), ref.update(i, count, int(geo.Cap[i]), tms, orbCell)
					for k, p := range ps {
						if d := math.Hypot(p[0], p[1]); math.IsNaN(d) || d > limit+1e-6 {
							t.Fatalf("cell %v, %d orbs at %v ms: orb %v from centre (max %v)", cell, count, tms, d, limit)
						}
						if math.Abs(p[0]-want[k][0]*s) > 1e-9 || math.Abs(p[1]-want[k][1]*s) > 1e-9 {
							t.Fatalf("cell %v, %d orbs at %v ms: orb %d at %v, want %v scaled by %v", cell, count, tms, k, p, want[k], s)
						}
					}
				}
			}
		}
	}
	if n := testing.AllocsPerRun(100, func() { f.update(7, 3, int(geo.Cap[7]), 1000, 60) }); n != 0 {
		t.Errorf("orbField.update allocates %v times", n)
	}
}

func TestOrbRadius(t *testing.T) {
	for cell, want := range map[float64]float64{24: 5.1, 40: 8.5, 80: 17} {
		if got := orbRadius(cell); got != want {
			t.Errorf("orbRadius(%v) = %v, want %v", cell, got, want)
		}
	}
}
