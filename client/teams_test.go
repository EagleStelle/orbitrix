package client

import (
	"fmt"
	"slices"
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// lineUp makes s a team game of n teams, its seats dealt to them in turn.
func lineUp(s *settings, n int) {
	s.Teams, s.Team = true, make([]uint8, s.Players)
	for i := range s.Team {
		s.Team[i] = uint8(i % n)
	}
}

func TestFitTeams(t *testing.T) {
	s := &settings{Players: 5, Teams: true}
	s.fitTeams()
	if !slices.Equal(s.Team, []uint8{0, 1, 0, 1, 0}) {
		t.Fatalf("new line-up %v", s.Team)
	}
	s.Team = []uint8{1, 1, 1, 1, 0} // lopsided teams stay as chosen
	s.Players = 7                   // and new seats join the emptiest
	s.fitTeams()
	if !slices.Equal(s.Team, []uint8{1, 1, 1, 1, 0, 0, 0}) {
		t.Fatalf("lopsided: %v", s.Team)
	}
	s.Players, s.Team = 4, []uint8{2, 2, 9} // a team the game doesn't have; team B counts though empty
	s.fitTeams()
	if !slices.Equal(s.Team, []uint8{2, 2, 0, 1}) || !s.rivals() {
		t.Fatalf("a bad team: %v", s.Team)
	}
	s.Team = []uint8{1, 1, 1, 1}
	if s.rivals() {
		t.Fatal("one team has no rivals")
	}
	if s.Teams = false; !s.rivals() {
		t.Fatal("Teams off always plays")
	}
}

// A deal keeps the teams that have players, a second one joining a lone team, and makes them equal, or one apart
// where the players don't divide; the seats on each change.
func TestDealTeams(t *testing.T) {
	rng := core.NewRNG(3)
	for _, tc := range []struct {
		team []uint8
		want [core.MaxTeams]int // players on each team afterwards
	}{
		{[]uint8{0, 0, 0, 0, 1, 2}, [core.MaxTeams]int{2, 2, 2}},
		{[]uint8{3, 3, 3, 3, 3, 5, 5}, [core.MaxTeams]int{3: 4, 5: 3}},
		{[]uint8{4, 4, 4}, [core.MaxTeams]int{0: 1, 4: 2}},
		{[]uint8{0, 0}, [core.MaxTeams]int{1, 1}},
		{make([]uint8, 0, 5)[:0:5], [core.MaxTeams]int{3, 2}}, // no line-up yet: A and B
	} {
		s := &settings{Players: cap(tc.team), Teams: true, Team: tc.team}
		s.dealTeams(&rng)
		var size [core.MaxTeams]int
		for _, team := range s.Team {
			size[team]++
		}
		if len(s.Team) != s.Players || size != tc.want {
			t.Errorf("%v dealt %v", tc.team, s.Team)
		}
	}
	s, seen := &settings{Players: 12, Teams: true}, map[string]bool{}
	for range 8 {
		s.dealTeams(&rng)
		seen[string(s.Team)] = true
	}
	if len(seen) < 2 {
		t.Fatal("every deal seats the teams alike")
	}
	s = &settings{Players: 3, Team: []uint8{4, 4, 4}}
	if s.dealTeams(&rng); !slices.Equal(s.Team, []uint8{4, 4, 4}) {
		t.Fatal("Teams off deals nothing")
	}
}

func TestRollBots(t *testing.T) {
	rng := core.NewRNG(5)
	s := &settings{seats: []int{1, seatHuman, 1, 1, seatHuman, 1}}
	s.rollBots(&rng)
	seen := map[int]bool{}
	for i, opt := range s.seats {
		if human := i == 1 || i == 4; human != (opt == seatHuman) || !human && seen[opt] {
			t.Fatalf("rolled %v", s.seats)
		}
		seen[opt] = true
	}
}

// The Teams tile turns teams on; each seat's chip sits in its row and opens the seat's team menu, never its type
// menu; any team takes any number of players, and Start needs two teams. Shuffle players spans the open card and
// deals the seats, bots and teams.
func TestSetupTeams(t *testing.T) {
	for _, size := range [][2]float64{{1440, 900}, {1040, 640}, {540, 844}, {320, 640}, {844, 390}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size[0], size[1]), func(t *testing.T) {
			a, s := setupTestApp(size[0], size[1], 1, 4), &setupScene{menu: -1, open: foldRules}
			tap := func(r ui.Rect) {
				t.Helper()
				a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
				if err := s.update(a); err != nil {
					t.Fatal(err)
				}
				a.in = input{}
				s.layout(a)
			}
			click := func(at func() ui.Rect) {
				t.Helper()
				s.layout(a)
				if low := at().Bottom() - s.viewport.Bottom(); low > 0 { // a short screen scrolls down to it
					s.scroll += low
					s.layout(a)
				}
				if r := at(); !s.viewport.Contains(r.Cx(), r.Cy()) {
					t.Fatalf("%v is off the screen", r)
				}
				tap(at())
			}
			inCard := func(at func() ui.Rect) { // Shuffle players or a seat, scrolled to inside the card first
				t.Helper()
				s.layout(a)
				s.seatScroll.Reveal(at())
				s.lv = ui.Rect{}
				click(at)
			}
			s.layout(a)
			click(func() ui.Rect { return s.game.tiles[len(gameRows)-1][0] })
			if !a.set.Teams || !slices.Equal(a.set.Team, []uint8{0, 1, 0, 1}) {
				t.Fatalf("Teams on: %v, %v", a.set.Teams, a.set.Team)
			}
			s.fold(foldPlayers)
			s.layout(a)
			if b, body := s.shuffleB, s.seatScroll.Viewport; b.X != body.X || b.W != body.W || b.Y < body.Y || b.Bottom() > s.seatTy[0].Y {
				t.Fatalf("Shuffle players %v must span the card's body %v over its first seat %v", b, body, s.seatTy[0])
			}
			for i := range a.set.Players {
				c, seat := s.teamChip[i], s.seatTy[i]
				if c.X < s.seatScroll.Viewport.X || c.Right() >= seat.X || c.Y != seat.Y || c.H != seat.H || c.W < ui.CompactButtonTarget*a.u {
					t.Fatalf("seat %d's team pill %v must stand left of its seat pill %v, as tall", i, c, seat)
				}
				for _, label := range seatLabels { // a narrower seat pill still holds every type, 8 px clear of its pips
					if need := (seatChoiceX+8+36+pipsW)*a.u + ui.TextWidth(label, a.f.LabelBold); need > seat.W {
						t.Fatalf("seat pill %v is %v short of %q", seat, need-seat.W, label)
					}
				}
			}
			inCard(func() ui.Rect { return s.teamChip[0] })
			if s.menu != 0 || !s.team {
				t.Fatalf("a tap on seat 0's chip: menu %d, team %v", s.menu, s.team)
			}
			panel, first := s.menuRects(a)
			if v := a.safe(); !containsRect(v, panel) {
				t.Fatalf("team menu %v leaves the screen %v", panel, v)
			}
			tap(setupMenuOption(panel, first, core.MaxTeams-1, a.u))
			if a.set.Team[0] != core.MaxTeams-1 || s.menu >= 0 {
				t.Fatalf("Team F chosen: team %d, menu %d", a.set.Team[0], s.menu)
			}
			inCard(func() ui.Rect { return s.teamChip[2] })
			if panel, _ := s.menuRects(a); panel.Contains(s.teamChip[2].Cx(), s.teamChip[2].Cy()) {
				s.menu = -1 // a short screen's menu covers the chip
			} else if tap(s.teamChip[2]); s.menu >= 0 { // a second tap closes it
				t.Fatalf("a second tap on a chip left menu %d open", s.menu)
			}
			a.set.Team = []uint8{1, 1, 1, 1} // a team holds any number, but a game needs two
			if s.canStart(a) {
				t.Fatal("Start with every player on one team")
			}
			inCard(func() ui.Rect { return s.shuffleB })
			humans, size := 0, [core.MaxTeams]int{}
			for i, opt := range a.set.seats {
				if size[a.set.Team[i]]++; opt == seatHuman {
					humans++
				}
			}
			if humans != 1 || size != [core.MaxTeams]int{2, 2} || !s.canStart(a) { // team A joins the lone team B
				t.Fatalf("shuffled to seats %v, teams %v", a.set.seats, a.set.Team)
			}
			a.set.Team = []uint8{1, 1, 1, 2}
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
	a.set.Teams, a.set.Team = true, []uint8{1, 0, 1, 0}
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
