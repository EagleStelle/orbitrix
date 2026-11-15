package client

import (
	"fmt"
	"slices"
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

func TestFitTeams(t *testing.T) {
	s := &settings{Players: 5, Teams: 2}
	s.fitTeams()
	if !slices.Equal(s.Team, []uint8{0, 1, 0, 1, 0}) || s.TeamCap != 3 {
		t.Fatalf("new line-up %v of %d a team", s.Team, s.TeamCap)
	}
	s.Team = []uint8{1, 1, 1, 1, 0} // one over the capacity: the last on team B moves
	s.fitTeams()
	if !slices.Equal(s.Team, []uint8{1, 1, 1, 0, 0}) {
		t.Fatalf("over capacity: %v", s.Team)
	}
	s.Players, s.Teams = 7, 9 // more teams than the game has, and a capacity too small for the players
	s.fitTeams()
	if s.Teams != core.MaxTeams || len(s.Team) != 7 || s.TeamCap != 3 || core.CheckTeams(s.Team, s.Teams, s.TeamCap) != nil {
		t.Fatalf("%d teams of %d: %v", s.Teams, s.TeamCap, s.Team)
	}
	s.Players, s.Teams, s.TeamCap = 4, 2, 1 // a team gone, and room for nobody
	s.fitTeams()
	if s.TeamCap != 2 || core.CheckTeams(s.Team, 2, 2) != nil || !s.rivals() {
		t.Fatalf("2 teams of %d: %v", s.TeamCap, s.Team)
	}
	s.Team = []uint8{1, 1, 1, 1}
	if s.rivals() {
		t.Fatal("one team has no rivals")
	}
	if s.Teams = 0; !s.rivals() {
		t.Fatal("Teams off always plays")
	}
}

func TestNextTeam(t *testing.T) {
	team := []uint8{0, 0, 1, 2}
	for _, tc := range []struct {
		seat, capacity int
		want           uint8
		ok             bool
	}{
		{0, 2, 1, true},
		{2, 2, 2, true},
		{3, 2, 1, true},  // team A is full: on to B
		{3, 1, 2, false}, // every other team is full
	} {
		if got, ok := nextTeam(team, tc.seat, 3, tc.capacity); got != tc.want || ok != tc.ok {
			t.Errorf("seat %d of %v, %d a team: %d %v, want %d %v", tc.seat, team, tc.capacity, got, ok, tc.want, tc.ok)
		}
	}
}

// The Teams tile turns teams on, their steppers and chips sit inside the Players card on every screen, a tap on a chip
// changes the team instead of opening the seat's menu, and Start needs two teams.
func TestSetupTeams(t *testing.T) {
	for _, size := range [][2]float64{{1440, 900}, {1040, 640}, {540, 844}, {320, 640}, {844, 390}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size[0], size[1]), func(t *testing.T) {
			a, s := setupTestApp(size[0], size[1], 1, 4), &setupScene{menu: -1, open: foldRules}
			click := func(at func() ui.Rect) {
				t.Helper()
				s.layout(a)
				if low := at().Bottom() - s.viewport.Bottom(); low > 0 { // a short screen scrolls down to it
					s.scroll += low
					s.layout(a)
				}
				r := at()
				a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
				if !s.viewport.Contains(a.in.x, a.in.y) {
					t.Fatalf("%v is off the screen", r)
				}
				if err := s.update(a); err != nil {
					t.Fatal(err)
				}
				a.in = input{}
				s.layout(a)
			}
			s.layout(a)
			click(func() ui.Rect { return s.game.tiles[len(gameRows)-1][0] })
			if a.set.Teams != 2 || !slices.Equal(a.set.Team, []uint8{0, 1, 0, 1}) || a.set.TeamCap != 2 {
				t.Fatalf("Teams on: %d teams of %d, %v", a.set.Teams, a.set.TeamCap, a.set.Team)
			}
			s.fold(foldPlayers)
			s.layout(a)
			for _, r := range []ui.Rect{s.minusT, s.plusT, s.minusC, s.plusC} {
				if !containsRect(s.players, r) || r.Bottom() > s.seatScroll.Viewport.Y || r.Y < s.minusP.Bottom() {
					t.Fatalf("stepper %v must sit in the card %v between its heading and its seats", r, s.players)
				}
			}
			if overlaps(s.minusT, s.minusC) || s.plusT.X < s.minusT.Right() || s.teamX[0] >= s.minusT.X || s.teamX[1] >= s.minusC.X {
				t.Fatal("the steppers and their labels must not overlap")
			}
			for i := range a.set.Players {
				if !containsRect(s.seatTy[i], s.teamChip[i]) {
					t.Fatalf("seat %d's chip %v is outside its row %v", i, s.teamChip[i], s.seatTy[i])
				}
			}
			click(func() ui.Rect { return s.plusT })
			click(func() ui.Rect { return s.plusC })
			if a.set.Teams != 3 || a.set.TeamCap != 3 {
				t.Fatalf("steppers: %d teams of %d", a.set.Teams, a.set.TeamCap)
			}
			s.seatScroll.Offset = 0
			click(func() ui.Rect { return s.teamChip[0] })
			if a.set.Team[0] != 1 || s.menu >= 0 {
				t.Fatalf("a tap on seat 0's chip: team %d, menu %d", a.set.Team[0], s.menu)
			}
			a.set.Team = []uint8{1, 1, 1, 1}
			a.set.TeamCap = 4
			if s.canStart(a) {
				t.Fatal("Start with every player on one team")
			}
			a.set.Team[3] = 2
			s.begin(a)
			p, ok := a.next.(*playScene)
			if !ok || !slices.Equal(p.game.Team[:4], []uint8{1, 1, 1, 2}) || p.game.Mix == nil {
				t.Fatal("Start didn't begin the team game")
			}
		})
	}
}

func TestTeamResults(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	a.set.Teams, a.set.Team = 2, []uint8{1, 0, 1, 0}
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	plain := setupTestApp(1040, 640, 1, 4)
	q, err := newPlay(plain, 5, 5, plain.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.tableCols, q.tableCols = 1, 1
	if p.seatTableWidth(a) != q.seatTableWidth(plain)+p.teamRoom(a) || p.teamRoom(a) == 0 {
		t.Fatal("the seat table must make room for the team chips")
	}
	g := p.game
	g.Played, g.Moves, g.Winner = 1<<4-1, 12, 2
	g.Orbs = [core.MaxPlayers]int32{0, 0, 5, 0} // seat 2 won it for team B; its teammate went out before
	p.outAt = [core.MaxPlayers]int{4, 9, 0, 9}
	p.settle()
	if p.winTitle != "Team B wins!" || p.shown != 2 {
		t.Fatalf("title %q in seat %d's ink", p.winTitle, p.shown)
	}
	if p.word[0] != [core.MaxPlayers]string{"1st", "3rd", "1st", "3rd"} {
		t.Fatalf("survival places %v", p.word[0][:4])
	}
}
