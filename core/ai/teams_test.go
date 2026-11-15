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
