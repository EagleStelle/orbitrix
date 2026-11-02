package ai

import (
	"fmt"
	"math"
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

// TestTakesTheWin: exploding a's primed edge cell captures all of b. Spy (deliberate noise), Raider
// (opening mode) and Jester can miss it, as in the prototype.
func TestTakesTheWin(t *testing.T) {
	for _, s := range []Strategy{Gambler, Warden, Mason, Voyager, Tactician, Hunter, Prodigy, Sovereign} {
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
	for _, s := range []Strategy{Warden, Raider} {
		for seed := range uint64(20) {
			g, _ := core.NewGame(7, 7, 2)
			var b Bot
			if cell := b.Choose(g, s, seed, DefaultBudget); g.Geo.Cap[cell] != 2 {
				t.Errorf("%v seed %d: opened at %d, want a corner", s, seed, cell)
			}
		}
	}
}

// TestRaiderSurvivesThreat: a's only cell is next to b's primed cell, so Raider founds a cell out of b's reach.
func TestRaiderSurvivesThreat(t *testing.T) {
	g := grid(t, 2, 0,
		"a1 b2 . . .",
		".  .  . . .",
		".  .  . . .",
	)
	for seed := range uint64(20) {
		var b Bot
		c, r := g.Geo.Coords(b.Choose(g, Raider, seed, DefaultBudget))
		if d := max(c-1, 1-c) + r; d < 3 {
			t.Errorf("seed %d: played %d,%d, %d steps from b's primed cell; want at least 3", seed, c, r, d)
		}
	}
}

// TestNomadSpreadsOutOfReach: b's primed cell can explode next turn, so Nomad founds a cell out of its reach and
// away from its own cells.
func TestNomadSpreadsOutOfReach(t *testing.T) {
	g := grid(t, 2, 0,
		". .  . .  . .  .",
		". a1 . .  . b3 .",
		". .  . .  . .  .",
		". .  . .  . .  .",
		". .  . a1 . .  .",
	)
	steps := func(i, c, r int) int {
		ic, ir := g.Geo.Coords(i)
		return max(ic-c, c-ic) + max(ir-r, r-ir)
	}
	for seed := range uint64(20) {
		var b Bot
		cell := b.Choose(g, Nomad, seed, DefaultBudget)
		if g.Owner[cell] != core.NoOwner || steps(cell, 5, 1) < 2 || steps(cell, 1, 1) < 2 || steps(cell, 3, 4) < 2 {
			t.Errorf("seed %d: played %v, want an empty cell 2+ steps from b's primed cell and from a's cells", seed,
				[]int{cell / g.Geo.Rows, cell % g.Geo.Rows})
		}
	}
}

// TestNomadContestsWhenItExplodesFirst: on the edge above b's cell, a's orb explodes after two more moves and b's
// after three, so a wins that race; beside it, both need three and b moves first.
func TestNomadContestsWhenItExplodesFirst(t *testing.T) {
	g := grid(t, 2, 0,
		".  . .  . .",
		".  . b1 . .",
		".  . .  . .",
		"a1 . .  . a1",
	)
	for seed := range uint64(10) {
		var b Bot
		if cell := b.Choose(g, Nomad, seed, DefaultBudget); cell != g.Geo.Index(2, 0) {
			t.Errorf("seed %d: chose %d, want the edge cell above b, %d", seed, cell, g.Geo.Index(2, 0))
		}
	}

	var b Bot
	b.Choose(g, Spy, 1, DefaultBudget) // sets up the bot for a's turn
	danger, exposed := b.spyDanger(g)
	if win, lose := b.spyPlace(g.Geo.Index(2, 0), danger, exposed), b.spyPlace(g.Geo.Index(1, 1), danger, exposed); win <= lose {
		t.Errorf("Spy scores the race it wins %v, the one it loses %v; want the win higher", win, lose)
	}
}

// TestWardenExplodesIntoProvoker: an enemy touches a's primed edge cell, so Warden explodes it.
func TestWardenExplodesIntoProvoker(t *testing.T) {
	g := grid(t, 2, 0,
		". a2 b1 . .",
		". .  .  . .",
		". .  .  . b1",
	)
	var b Bot
	if cell := b.Choose(g, Warden, 1, DefaultBudget); cell != g.Geo.Index(1, 0) {
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

func TestOpeningKeepsAFallback(t *testing.T) {
	for s := range NumStrategies {
		if s == Jester || s == Voyager {
			continue
		}
		for _, budget := range []int{0, 1, DefaultBudget} {
			for seed := range uint64(20) {
				for _, first := range []bool{true, false} {
					g := grid(t, 3, 0,
						"a1 b2 . . .",
						".  .  . . .",
						".  .  . . c1",
					)
					if first {
						g.Owner[0], g.Count[0], g.Orbs[0] = core.NoOwner, 0, 0
						g.Played &^= 1
					}
					before := g.Hash()
					var b Bot
					cell := b.Choose(g, s, seed, budget)
					if g.Hash() != before {
						t.Fatal("Choose changed its input")
					}
					n := g.Clone()
					if err := n.Simulate(cell); err != nil {
						t.Fatal(err)
					}
					if !n.Over() && n.Turn == 1 {
						// The next player can explode the edge cell and capture the corner.
						if err := n.Simulate(g.Geo.Index(1, 0)); err != nil {
							t.Fatal(err)
						}
					}
					if !n.Alive(0) {
						t.Fatalf("%v budget %d seed %d first=%v: move %d loses its last foothold", s, budget, seed, first, cell)
					}
				}
			}
		}
	}
}

func TestOpeningAvoidsNearbyUnprimedEnemies(t *testing.T) {
	for s := range NumStrategies {
		if s == Jester || s == Voyager {
			continue
		}
		for seed := range uint64(20) {
			g := grid(t, 2, 1,
				"a1 . . . .",
				".  . . . .",
				".  . . . .",
				".  . . . .",
				".  . . . .",
			)
			g.Played &^= 1 << 1
			var b Bot
			cell := b.Choose(g, s, seed, DefaultBudget)
			c, r := g.Geo.Coords(cell)
			if c+r < 3 {
				t.Errorf("%v seed %d: first orb at %d,%d is too close to the enemy with no backup", s, seed, c, r)
			}
		}
	}
}

func TestTacticianRecognizesEliminationAtHorizon(t *testing.T) {
	g := grid(t, 3, 0, "a1 b2 .", ". . .", ". . c1")
	if err := g.Simulate(g.Geo.Index(0, 1)); err != nil {
		t.Fatal(err)
	}
	if err := g.Simulate(g.Geo.Index(1, 0)); err != nil {
		t.Fatal(err)
	}
	// The corner was lost, but a's second cell also falls in the chain.
	if g.Alive(0) || g.Over() {
		t.Fatal("fixture must eliminate a while b and c remain")
	}
	b := Bot{me: 0}
	tactic := tactic{b: &b}
	if score, complete := tactic.minimax(g, 0, -1e12, 1e12, 2); !complete || score > -1e6 {
		t.Errorf("eliminated player scored %v at the search horizon", score)
	}
}

func TestOpeningFallbackSurvivesAChain(t *testing.T) {
	for s := range NumStrategies {
		if s == Jester || s == Voyager {
			continue
		}
		for _, budget := range []int{0, 1, DefaultBudget} {
			for seed := range uint64(20) {
				g := grid(t, 3, 0,
					"b1 b2 a2 . . . .",
					".  .  .  . . . .",
					".  .  .  . . . c1",
				)
				var b Bot
				cell := b.Choose(g, s, seed, budget)
				if err := g.Simulate(cell); err != nil {
					t.Fatal(err)
				}
				if !g.Over() && g.Turn == 1 {
					if err := g.Simulate(0); err != nil {
						t.Fatal(err)
					}
				}
				if !g.Alive(0) {
					t.Fatalf("%v budget %d seed %d: move %d leaves every base in the same enemy chain", s, budget, seed, cell)
				}
			}
		}
	}
}

func TestTacticianMinimaxMatchesFullSearch(t *testing.T) {
	positions := []*core.Game{
		grid(t, 2, 0, "a1 . b1", ". a2 .", ". . b1"),
		grid(t, 3, 0, "a1 b2 .", ". . .", ". . c1"),
		grid(t, 4, 1, "a1 b2 .", ". . .", "c1 . ."), // player d has already been eliminated
	}
	// Exhaustive reference: enumerate every legal move, with all other players opposing a.
	var full func(*core.Game, *tactic, int, int) float64
	full = func(g *core.Game, tactic *tactic, depth, ply int) float64 {
		if depth == 0 || g.Over() || !g.Alive(int(tactic.b.me)) {
			return tactic.evaluate(g, ply)
		}
		best := 1e12
		if g.Turn == int(tactic.b.me) {
			best = -1e12
		}
		for _, cell := range g.LegalMoves(nil) {
			n := g.Clone()
			if err := n.Simulate(cell); err != nil {
				t.Fatal(err)
			}
			v := full(n, tactic, depth-1, ply+1)
			if g.Turn == int(tactic.b.me) {
				best = max(best, v)
			} else {
				best = min(best, v)
			}
		}
		return best
	}
	for i, g := range positions {
		for depth := 1; depth <= 3; depth++ {
			b := Bot{me: 0, left: 100000}
			tactic := tactic{b: &b, branch: g.Geo.Cells()}
			want := full(g, &tactic, depth, 0)
			got, complete := tactic.minimax(g, depth, -1e12, 1e12, 0)
			if !complete || got != want {
				t.Errorf("position %d depth %d: minimax = %v (complete=%v), exhaustive search = %v", i, depth, got, complete, want)
			}
		}
	}
}

func TestTacticianKeepsLastCompleteDepth(t *testing.T) {
	g := grid(t, 2, 0, "a1 . .", ". . .", ". . b1")
	rootMoves := len(g.LegalMoves(nil))
	var shallow, interrupted Bot
	want := shallow.Choose(g, Tactician, 1, rootMoves)
	got := interrupted.Choose(g, Tactician, 1, rootMoves+1)
	if got != want {
		t.Errorf("unfinished second depth changed move from %d to %d", want, got)
	}
}

// TestHunterQuiescenceSeesCaptures: at the horizon b can explode its corner into both of a's cells next to it, so
// Hunter's leaf score must fall below the static evaluation.
func TestHunterQuiescenceSeesCaptures(t *testing.T) {
	g := grid(t, 2, 1,
		"b1 a1 . .",
		"a1 .  . .",
		".  .  . a1",
	)
	b := Bot{me: 0, left: 1000}
	hunt := tactic{b: &b, hunt: true, prey: 1}
	static := hunt.evaluate(g, 0)
	if got, complete := hunt.minimax(g, 0, -1e12, 1e12, 0); !complete || got >= static {
		t.Errorf("leaf scored %v (complete=%v), static evaluation %v", got, complete, static)
	}
}

func TestHunterPreysOnTheLeader(t *testing.T) {
	g := grid(t, 4, 0, "a1 b1 c2 .", ". . . .", "d1 . . c1")
	b := Bot{g: g, me: 0}
	if got := b.prey(); got != 2 {
		t.Errorf("prey = %d, want c, who has the most orbs", got)
	}
}

func TestTacticianTerminalScores(t *testing.T) {
	var b Bot
	tactic := tactic{b: &b}
	g, _ := core.NewGame(5, 5, 3)
	if err := g.Simulate(0); err != nil {
		t.Fatal(err)
	}
	if v, complete := tactic.minimax(g, 0, -1e12, 1e12, 1); !complete || v > tacticWin/2 {
		t.Fatalf("first move with unplayed opponents scored as a win: %v", v)
	}
	for _, winner := range []int8{0, 1} {
		g.Winner = winner
		v, complete := tactic.minimax(g, 0, -1e12, 1e12, 3)
		want := tacticWin - 3
		if winner != b.me {
			want = -want
		}
		if !complete || v != want {
			t.Errorf("winner %d at exhausted horizon: got %v, want %v", winner, v, want)
		}
	}
}

func TestCrowdedOpeningUsesASafeCellWithinTheBuffer(t *testing.T) {
	g := grid(t, 3, 0, "a1 b2 .", ". . .", ". . c1")
	var b Bot
	// No cell is three steps from every enemy, but the bottom-left cell survives b's chain.
	b.g, b.geo, b.me = g, g.Geo, 0
	b.moves = g.LegalMoves(nil)
	cell := b.openingMove(g.Geo.Index(0, 1))
	if err := g.Simulate(cell); err != nil {
		t.Fatal(err)
	}
	if err := g.Simulate(g.Geo.Index(1, 0)); err != nil {
		t.Fatal(err)
	}
	if !g.Alive(0) {
		t.Fatalf("move %d lost a even though a safe retreat exists", cell)
	}
}

func TestOpeningThreatsIncludeConvergingChains(t *testing.T) {
	g := grid(t, 3, 0,
		"b1 a2 . . .",
		"a2 a2 . . .",
		".  .  . . .",
		".  .  . . .",
		".  .  . . c1",
	)
	var b Bot
	b.g, b.geo, b.me = g, g.Geo, 0
	// Two branches each send one orb into a's centre, which explodes into the proposed reserve.
	if !b.openingThreats(g, -1)[g.Geo.Index(1, 2)] {
		t.Fatal("missed the threat from converging chains")
	}
}

func TestOpeningSurvivalLimitedToOneCellTwoOrbs(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []string
		want bool
	}{
		{"first move", []string{". . .", ". . .", ". . b1"}, true},
		{"one orb", []string{"a1 . .", ". . .", ". . b1"}, true},
		{"one cell two orbs", []string{". . .", ". a2 .", ". . b1"}, true},
		{"one cell three orbs", []string{". . .", ". a3 .", ". . b1"}, false},
		{"two cells two orbs", []string{"a1 . .", "a1 . .", ". . b1"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := grid(t, 2, 0, test.rows...)
			b := Bot{g: g, me: 0}
			if got := b.fragileOpening(); got != test.want {
				t.Errorf("survival eligible = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOpeningThreatsScratchDoesNotAllocate(t *testing.T) {
	g, _ := core.NewGame(core.MaxSize, core.MaxSize, 2)
	if err := g.Simulate(0); err != nil {
		t.Fatal(err)
	}
	b := Bot{geo: g.Geo, me: 1}
	if allocations := testing.AllocsPerRun(10, func() { b.openingThreats(g, -1) }); allocations != 0 {
		t.Errorf("threat detection allocated %v times after warming its existing buffers", allocations)
	}
}

func TestJesterKeepsRandomOpening(t *testing.T) {
	g := grid(t, 3, 0, "a1 b2 . . .", ". . . . .", ". . . . c1")
	// Seed 3 selects an exposed cell; Jester must keep its random choice.
	for _, budget := range []int{0, 1, DefaultBudget} {
		var b Bot
		if cell, want := b.Choose(g, Jester, 3, budget), g.Geo.Index(0, 1); cell != want {
			t.Errorf("budget %d: chose %d, want original random move %d", budget, cell, want)
		}
	}
}

func TestVoyagerRewardsRespectTurnOrder(t *testing.T) {
	g := grid(t, 2, 0, "a1 b2 . . .", ". . . . .", ". . . . .")
	first := positionShares(g)
	g.Turn = 1
	second := positionShares(g)
	if first[0] <= second[0] {
		t.Errorf("ready corner scored %v moving first and %v moving second", first[0], second[0])
	}
}

func TestVoyagerRewardBoundsAndOutcomes(t *testing.T) {
	empty, _ := core.NewGame(5, 5, 3)
	eliminated := grid(t, 3, 1, ". b1 .", ". . .", ". . c1")
	won := grid(t, 3, 1, ". b1 .", ". . .", ". . c1")
	won.Winner = 1
	for _, g := range []*core.Game{empty, eliminated, won} {
		values := positionShares(g)
		total := 0.0
		for p := 0; p < g.Players; p++ {
			if math.IsNaN(values[p]) || values[p] < 0 || values[p] > 1 {
				t.Fatalf("player %d has invalid MCTS reward %v", p, values[p])
			}
			total += values[p]
		}
		if math.Abs(total-1) > 1e-12 {
			t.Errorf("rewards sum to %v", total)
		}
	}
	if got := positionShares(eliminated)[0]; got != 0 {
		t.Errorf("eliminated player received %v", got)
	}
	if got := positionShares(won); got[1] != 1 || got[0] != 0 || got[2] != 0 {
		t.Errorf("terminal rewards = %v", got)
	}
}
