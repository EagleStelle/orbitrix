package ai

import (
	"fmt"
	"strings"
	"testing"

	"orbitrix/core"
)

// grid builds a game from one string per row: "." for an empty cell, or a player letter (a = player 0) followed by
// its orb count. Every player is marked as having played.
func grid(t testing.TB, players, turn int, rows ...string) *core.Game {
	t.Helper()
	var cells [][]string
	for _, row := range rows {
		cells = append(cells, strings.Fields(row))
	}
	s := core.Snapshot{Cols: len(cells[0]), Rows: len(cells), Players: players, Turn: turn,
		Played: 1<<players - 1, Winner: core.NoOwner}
	for c := 0; c < s.Cols; c++ {
		for r := 0; r < s.Rows; r++ {
			cell := cells[r][c]
			if cell == "." {
				s.Count, s.Owner = append(s.Count, 0), append(s.Owner, core.NoOwner)
				continue
			}
			var n uint8
			if _, err := fmt.Sscanf(cell[1:], "%d", &n); err != nil {
				t.Fatalf("bad cell %q: %v", cell, err)
			}
			s.Count, s.Owner = append(s.Count, n), append(s.Owner, int8(cell[0]-'a'))
		}
	}
	g, err := core.Restore(s)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestParse(t *testing.T) {
	for s := range NumStrategies {
		if got, ok := Parse(s.String()); !ok || got != s {
			t.Errorf("Parse(%q) = %v, %v", s, got, ok)
		}
	}
	if _, ok := Parse("Human"); ok {
		t.Error(`Parse("Human") succeeded`)
	}
}

// TestGamesAreLegalAndDeterministic plays whole games with every strategy at every seat, at the default budget and
// with the budget exhausted, checking each move is legal and that a fresh bot picks the same move from the same
// state and seed.
func TestGamesAreLegalAndDeterministic(t *testing.T) {
	type setup struct{ cols, rows, players int }
	setups := []setup{{3, 3, 2}, {5, 9, 2}, {9, 9, 3}, {8, 6, 5}, {12, 12, 12}}
	if testing.Short() {
		setups = setups[:3]
	}
	for _, st := range setups {
		for _, starved := range []bool{false, true} {
			name := fmt.Sprintf("%dx%d/%dp/starved=%v", st.cols, st.rows, st.players, starved)
			t.Run(name, func(t *testing.T) {
				for offset := range NumStrategies {
					g, err := core.NewGame(st.cols, st.rows, st.players)
					if err != nil {
						t.Fatal(err)
					}
					var bot, check Bot
					rng := core.NewRNG(uint64(offset))
					for move := 0; !g.Over(); move++ {
						if move > 50*g.Geo.Cells() {
							t.Fatalf("game didn't end after %d moves", move)
						}
						s := (offset + Strategy(g.Turn)) % NumStrategies
						seed, budget := rng.Uint64(), DefaultBudget
						if starved {
							budget = move % 2
						}
						cell := bot.Choose(g, s, seed, budget)
						if !g.Legal(cell) {
							t.Fatalf("move %d: %v (player %d) chose illegal cell %d", move, s, g.Turn, cell)
						}
						if again := check.Choose(g.Clone(), s, seed, budget); again != cell {
							t.Fatalf("move %d: %v chose %d, then %d from the same state and seed", move, s, cell, again)
						}
						if _, err := g.Apply(cell); err != nil {
							t.Fatal(err)
						}
					}
				}
			})
		}
	}
}

func TestChooseOnFinishedGame(t *testing.T) {
	g := grid(t, 2, 0, "a1 . .", ". . .", ". . .")
	g.Winner = 0
	var b Bot
	if cell := b.Choose(g, Tactician, 1, DefaultBudget); cell != -1 {
		t.Errorf("got %d on a finished game, want -1", cell)
	}
}

// TestTakesTheWin: exploding a's primed edge cell captures all of b. Spy (deliberate noise), Conqueror
// (opening mode) and Jester can miss it, as in the prototype.
func TestTakesTheWin(t *testing.T) {
	for _, s := range []Strategy{Gambler, Sentinel, Architect, Voyager, Tactician} {
		for seed := range uint64(20) {
			g := grid(t, 2, 0,
				". a2 b2 . .",
				". .  .  . .",
				". .  .  . .",
				". .  .  . .",
			)
			var b Bot
			if cell, want := b.Choose(g, s, seed, DefaultBudget), g.Geo.Index(1, 0); cell != want {
				t.Errorf("%v seed %d: chose %d, want the winning cell %d", s, seed, cell, want)
			}
		}
	}
}

// TestVoyagerTakesBigCapture: a's chain captures four of b's five orbs without winning, so only the search's value
// estimates (not the instant-win check) can pick it.
func TestVoyagerTakesBigCapture(t *testing.T) {
	for seed := range uint64(10) {
		g := grid(t, 2, 0,
			". a2 b2 b1 .",
			". .  b1 .  .",
			". .  .  .  .",
			". .  .  .  b1",
		)
		var b Bot
		if cell, want := b.Choose(g, Voyager, seed, DefaultBudget), g.Geo.Index(1, 0); cell != want {
			t.Errorf("seed %d: chose %d, want the capture at %d", seed, cell, want)
		}
	}
}

func TestOpensInACorner(t *testing.T) {
	for _, s := range []Strategy{Sentinel, Conqueror} {
		for seed := range uint64(20) {
			g, _ := core.NewGame(7, 7, 2)
			var b Bot
			if cell := b.Choose(g, s, seed, DefaultBudget); g.Geo.Cap[cell] != 2 {
				t.Errorf("%v seed %d: opened at %d, want a corner", s, seed, cell)
			}
		}
	}
}

// TestConquerorRetreats: a's only cell is next to b's primed cell, so Conqueror plays as far from b as it can.
func TestConquerorRetreats(t *testing.T) {
	g := grid(t, 2, 0,
		"a1 b2 . . .",
		".  .  . . .",
		".  .  . . .",
	)
	var b Bot
	cell := b.Choose(g, Conqueror, 1, DefaultBudget)
	if c, r := g.Geo.Coords(cell); c != 4 || r != 2 {
		t.Errorf("retreated to %d,%d, want the far corner 4,2", c, r)
	}
}

// TestSentinelExplodesIntoProvoker: an enemy touches a's primed edge cell, so Sentinel explodes it.
func TestSentinelExplodesIntoProvoker(t *testing.T) {
	g := grid(t, 2, 0,
		". a2 b1 . .",
		". .  .  . .",
		". .  .  . b1",
	)
	var b Bot
	if cell := b.Choose(g, Sentinel, 1, DefaultBudget); cell != g.Geo.Index(1, 0) {
		t.Errorf("chose %d, want the primed cell %d", cell, g.Geo.Index(1, 0))
	}
}

func TestInsertTop(t *testing.T) {
	var list []scored
	for i, s := range []float64{1, 5, 3, 5, 2, 9} {
		list = insertTop(list, scored{s, i}, 4)
	}
	want := []scored{{9, 5}, {5, 1}, {5, 3}, {3, 2}}
	if fmt.Sprint(list) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", list, want)
	}
}
