package client

import (
	"slices"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client/platform"
)

func TestGameIdleAndAnimationTransitions(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	if a.epoch.IsZero() || a.tms() > 1000 {
		t.Fatal("test apps need a running animation clock")
	}
	p, err := newPlay(a, 9, 9, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.paused = true
	g := &Game{a: *a, scene: p, fresh: true}
	g.a.in = input{x: offScreen, y: offScreen, finger: true}
	dst := ebiten.NewImage(390, 844)
	defer dst.Deallocate()
	g.Draw(dst)
	redraws := 0
	for range 60 {
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		if g.fresh {
			redraws++
		}
		g.Draw(dst)
	}
	if redraws != 60/stillRedraw || p.b.Clock != 0 {
		t.Fatalf("paused game: %d redraws, board clock %g", redraws, p.b.Clock)
	}
	platform.PressBack()
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if p.paused || !g.fresh || p.b.Clock != 0 {
		t.Fatal("resuming must redraw immediately without advancing the paused clock")
	}
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if p.b.Clock != 1 {
		t.Fatal("resumed board must advance by one tick")
	}
	g.Draw(dst)
	p.over, p.overT = true, wipeTicks-1
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if !g.fresh || !still(p) {
		t.Fatal("the last animated frame must draw before results become idle")
	}
	g.Draw(dst)
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if g.fresh {
		t.Fatal("finished results must stay idle without input")
	}
}

func TestPausedBoardMotionFreezes(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	p, err := newPlay(a, 9, 9, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	cell := p.game.Geo.Index(1, 1)
	p.b.State.Owner[cell], p.b.State.Count[cell], p.b.Clock = 0, 3, 30
	dst := ebiten.NewImage(1, 1)
	defer dst.Deallocate()
	p.draw(dst, a)
	positions := slices.Clone(p.b.Orbs.Pos)
	p.paused = true
	a.epoch = a.epoch.Add(-time.Hour)
	for range 3 {
		p.draw(dst, a)
	}
	if !slices.Equal(positions, p.b.Orbs.Pos) {
		t.Fatal("periodic redraws and elapsed wall time must not move paused orbs")
	}
}

func TestOpaqueResultsSkipBoard(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	p, err := newPlay(a, 9, 9, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.b.State.Owner[0], p.b.State.Count[0], p.b.Clock = 0, 1, 1
	p.over, p.overT = true, wipeTicks
	dst := ebiten.NewImage(1, 1)
	defer dst.Deallocate()
	p.draw(dst, a)
	if p.b.Orbs.N[0] != 0 {
		t.Fatal("the hidden board must not animate behind opaque results")
	}
	p.overT--
	p.draw(dst, a)
	if p.b.Orbs.N[0] != 1 {
		t.Fatal("the board must remain visible until the results wipe finishes")
	}
}

// TestPauseConfirm checks that Restart and Main menu need a second tap within the confirm window.
func TestPauseConfirm(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	p, err := newPlay(a, 9, 9, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.paused = true
	p.layout(a)
	tap := func(i int) {
		r := p.pauseBtn[i]
		a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
		if err := p.updatePause(a); err != nil {
			t.Fatal(err)
		}
		a.in = input{x: offScreen, y: offScreen}
	}
	tap(pauseRestart)
	if a.next != nil || p.confirm != pauseRestart {
		t.Fatal("the first tap on Restart must only ask")
	}
	for range confirmTicks {
		_ = p.updatePause(a)
	}
	tap(pauseRestart)
	if a.next != nil {
		t.Fatal("a tap after the confirm window must ask again")
	}
	tap(pauseRestart)
	if _, ok := a.next.(*playScene); !ok {
		t.Fatal("the second tap must restart")
	}
}
