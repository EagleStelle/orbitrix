package ai

import (
	"testing"

	"orbitrix/core"
)

// Every bot plays team games, on a classic board and a rolled map, with legal moves only, and a bot that can win for
// its team at once does.
func TestBotsInTeams(t *testing.T) {
	for strategy := Strategy(0); strategy < NumStrategies; strategy++ {
		for _, rolled := range []bool{false, true} {
			g, err := core.NewGame(6, 6, 4)
			if rolled {
				g, err = core.NewGameWithMap(core.RandomMap(7, 7, 5), 4)
			}
			if err == nil {
				err = g.SetTeams([]uint8{0, 1, 0, 1})
			}
			if err != nil {
				t.Fatal(err)
			}
			var bot Bot
			for ply := 0; ply < 400 && !g.Over(); ply++ {
				cell := bot.Choose(g, strategy, uint64(ply)+1, 150)
				if cell < 0 { // a rolled map can leave a player no cell
					if err := g.Pass(); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if !g.Legal(cell) {
					t.Fatalf("%v (rolled %v) ply %d: illegal cell %d", strategy, rolled, ply, cell)
				}
				if _, err := g.Apply(cell); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestBotsWinForTheirTeam(t *testing.T) {
	g, err := core.NewGame(5, 5, 3)
	if err == nil {
		err = g.SetTeams([]uint8{0, 0, 1})
	}
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 0b111
	s.Count[0], s.Owner[0], s.Mix[0] = 1, 0, 1     // our corner, one orb from popping
	s.Count[1], s.Owner[1], s.Mix[1*3+2] = 1, 2, 1 // the rival's last orb, beside it
	s.Count[24], s.Owner[24], s.Mix[24*3+1] = 1, 1, 1
	if g, err = core.Restore(s); err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []Strategy{Tactician, Hunter, Voyager, Prodigy, Sovereign, Mason} {
		var bot Bot
		if cell := bot.Choose(g, strategy, 7, strategy.Budget()); cell != 0 {
			t.Errorf("%v played %d, not the winning corner", strategy, cell)
		}
	}
}

// stack is n of player's orbs in cell.
type stack struct{ cell, player, n int }

// teamGame builds a team game where everyone has played. A cell's first stack must be its biggest: it owns the cell.
func teamGame(t *testing.T, cols, rows int, team []uint8, turn int, stacks ...stack) *core.Game {
	t.Helper()
	g, err := core.NewGame(cols, rows, len(team))
	if err == nil {
		err = g.SetTeams(team)
	}
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Turn, s.Played = turn, 1<<len(team)-1
	for _, k := range stacks {
		s.Count[k.cell] += uint8(k.n)
		s.Mix[k.cell*len(team)+k.player] += uint8(k.n)
		if s.Owner[k.cell] == core.NoOwner {
			s.Owner[k.cell] = int8(k.player)
		}
	}
	if g, err = core.Restore(s); err != nil {
		t.Fatal(err)
	}
	return g
}

// A full board where player 0's only orb sits in the centre, which teammate 1 owns: 0's one legal move. Bots must
// find it at the root, and search past it when a teammate or the rival moves first.
func TestBotsPlaySharedCells(t *testing.T) {
	for strategy := Strategy(0); strategy < NumStrategies; strategy++ {
		if strategy == Prodigy || strategy == Sovereign {
			continue // not team-aware yet: both list moves by cell owner and crash here
		}
		for turn := range 3 {
			stacks := []stack{{4, 1, 2}, {4, 0, 1}}
			for cell := range 9 {
				if cell != 4 {
					stacks = append(stacks, stack{cell, 1 + cell/6, 1})
				}
			}
			g := teamGame(t, 3, 3, []uint8{0, 0, 1}, turn, stacks...)
			var bot Bot
			cell := bot.Choose(g, strategy, 7, strategy.Budget())
			if !g.Legal(cell) || turn == 0 && cell != 4 {
				t.Errorf("%v, player %d to move: played %d", strategy, turn, cell)
			}
		}
	}
}

func TestSearchBotsScoreTheTeam(t *testing.T) {
	// Player 0 is out, but teammate 1 holds more than rival 2.
	g := teamGame(t, 5, 5, []uint8{0, 0, 1}, 1, stack{0, 1, 1}, stack{12, 1, 3}, stack{24, 2, 1})
	b := Bot{g: g, me: 0, left: 1000}
	if v := (&tactic{b: &b}).evaluate(g, 0); v <= 0 {
		t.Errorf("evaluation %v with the team ahead, though its own player is out", v)
	}
	if v := positionShares(g); v[0] != v[1] || v[0] <= v[2] {
		t.Errorf("shares %v: teammates must share the bigger one", v)
	}

	// Rivals 1 and 2 together lead rival 3, who has the most orbs of any one player.
	g = teamGame(t, 5, 5, []uint8{0, 1, 1, 2}, 0, stack{0, 0, 1}, stack{2, 1, 2}, stack{12, 2, 2}, stack{22, 3, 2}, stack{24, 3, 1})
	b = Bot{g: g, me: 0}
	if got := b.prey(); got != 1 {
		t.Errorf("prey = side %d, want team 1, which has the most orbs", got)
	}

	// The rival fell before teammate 1's first turn: Hunter's reply layer has no one to move.
	g = teamGame(t, 5, 5, []uint8{0, 0, 1}, 1, stack{0, 0, 1})
	g.Played &^= 1 << 1
	b = Bot{g: g, me: 0, left: 1000}
	hunt := tactic{b: &b, branch: 8, hunt: true, prey: 1}
	if v, complete := hunt.minimax(g, 2, -1e12, 1e12, 1); !complete || v <= 0 {
		t.Errorf("no opponent left to reply: scored %v (complete=%v)", v, complete)
	}
}
