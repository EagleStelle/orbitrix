package client

import (
	"testing"

	"orbitrix/core"
)

func TestTallyCreditsTheMover(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	g := p.game
	g.Played, g.Moves = 1<<4-1, 9
	g.Orbs = [core.MaxPlayers]int32{0, 3, 0, 2} // seats 0 and 2 are out
	p.tally(2)                                  // seat 2's own move: it put out seat 0 and itself
	p.tally(1)                                  // nothing new
	if p.kills != [core.MaxPlayers]int{2: 1} || p.outAt[0] != 9 || p.outAt[2] != 9 || p.outAt[1] != 0 {
		t.Fatalf("kills %v, out at %v", p.kills[:4], p.outAt[:4])
	}
}

func TestRankSharesPlaces(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.game.Winner, p.game.Moves, p.biggest = 0, 12, 3
	p.outAt = [core.MaxPlayers]int{0, 5, 9, 9}
	p.kills = [core.MaxPlayers]int{2, 0, 1, 1}
	p.rank()
	none := 3 // past the medals
	for _, c := range []struct {
		board         int
		order, medals [4]int
		words         [4]string
	}{
		{0, [4]int{0, 2, 3, 1}, [4]int{0, none, 1, 1}, [4]string{"1st", "4th", "2nd", "2nd"}},
		{1, [4]int{0, 2, 3, 1}, [4]int{0, none, 1, 1}, [4]string{"2", "0", "1", "1"}},
	} {
		for i := range 4 {
			if p.order[c.board][i] != c.order[i] || p.medal[c.board][i] != c.medals[i] || p.word[c.board][i] != c.words[i] {
				t.Fatalf("board %d: order %v, medals %v, words %v", c.board, p.order[c.board][:4], p.medal[c.board][:4], p.word[c.board][:4])
			}
		}
	}
	if p.stats != [2]string{"12", "×3"} {
		t.Fatalf("stats %v", p.stats)
	}
}

func TestWatchAskedOnceWhenHumansAreOut(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4) // seat 0 is the human
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.settle()
	if p.watch != nil {
		t.Fatal("a human still in must not be asked")
	}
	p.game.Played = 1<<4 - 1
	p.game.Orbs = [core.MaxPlayers]int32{0, 3, 1, 2}
	p.settle()
	if p.watch == nil || p.watch.no != "Leave" {
		t.Fatal("every human out must ask Continue or Leave")
	}
	p.watch = nil
	p.settle()
	if p.watch != nil {
		t.Fatal("the question is asked once a game")
	}
}

func TestKnockoutSnackbar(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {390, 844}} {
		a := setupTestApp(size[0], size[1], 1, 4)
		p, err := newPlay(a, 5, 5, a.set.seats)
		if err != nil {
			t.Fatal(err)
		}
		p.layout(a)
		g := p.game
		g.Played, g.Moves = 1<<4-1, 9
		g.Orbs = [core.MaxPlayers]int32{0, 3, 0, 2} // seats 0 and 2 are out
		p.tally(2)                                  // seat 2's own move: it put out seat 0 and itself
		p.b.Playing, p.orbs[0] = true, 1            // the chain still shows an orb of seat 0's
		if p.tickOuts(a); sceneSnackbar(p) != nil {
			t.Fatal("a knockout the board has not shown yet must wait")
		}
		p.orbs[0] = 0
		p.tickOuts(a)
		bar := sceneSnackbar(p)
		if by, out := p.knockPair(); bar == nil || by != 2 || out != 0 || bar.more != "" {
			t.Fatalf("%v: first knockout: bar %v, %v", size, bar != nil, p.outs)
		}
		bar.ticks = snackIn // risen into place
		r, _, _ := bar.frame(a)
		board, fixed, names := p.boardView.Viewport, 0.0, 0.0
		if fixed, names = p.knockWidth(a); r.W != fixed+names || r.Cx() != board.Cx() || r.X < 0 || r.Bottom() > a.h-snackGap || a.portrait() && r.Bottom() > p.name.Y {
			t.Fatalf("%v: bar %+v must hug its words, centred on the board %+v and clear of the name %+v", size, r, board, p.name)
		}
		if p.paused = true; sceneSnackbar(p) != nil {
			t.Fatal("a dialog hides the knockout bar")
		}
		p.paused, p.b.Playing = false, false
		for range knockTicks - 2 {
			p.tickOuts(a)
		}
		if _, _, opacity := p.snack.frame(a); opacity >= 1 {
			t.Fatal("a bar fades away before the next rises")
		}
		p.tickOuts(a)
		p.tickOuts(a)
		if by, out := p.knockPair(); sceneSnackbar(p) == nil || by != -1 || out != 2 || p.snack.ticks != 1 {
			t.Fatalf("%v: a seat that put itself out follows, alone, in a bar of its own: %v", size, p.outs)
		}
		for range knockTicks {
			p.tickOuts(a)
		}
		if sceneSnackbar(p) != nil || len(p.outs) != 0 {
			t.Fatal("the bar goes once every knockout was shown")
		}
		g.Orbs[3], g.Winner = 0, 1 // the move that ends the game: the results say it
		if p.tally(1); len(p.outs) != 0 {
			t.Fatal("the winning move's knockouts go to the results, not the bar")
		}
	}
}
