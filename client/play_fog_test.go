package client

import "testing"

// A fog game shows the sight of the human here whose turn it is, keeps it through a bot's turn, covers the board when
// it passes to another human here, and lifts the fog on the win.
func TestFogViewerAndCover(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 3)
	a.set.seats, a.set.Fog = []int{seatHuman, seatHuman, seatHuman + 1}, true
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	if !p.game.Fog || p.b.Viewer != 0 || p.cover {
		t.Fatalf("first turn: viewer %d, cover %v", p.b.Viewer, p.cover)
	}
	p.move(a, 0)
	if p.b.Viewer != 1 || !p.cover || p.coverText != "Pass to "+p.labels[1] {
		t.Fatalf("passed to the second human: viewer %d, cover %v %q", p.b.Viewer, p.cover, p.coverText)
	}
	if p.orbText(1) != "?" || p.orbText(0) != "?" {
		t.Fatal("a count shows under the cover")
	}
	p.cover = false
	p.move(a, 24)
	if p.b.Viewer != 1 || p.cover {
		t.Fatalf("a bot's turn: viewer %d, cover %v", p.b.Viewer, p.cover)
	}
	p.tickRail()
	if p.orbText(0) != "?" || p.orbText(1) != "1" || p.share[1] <= p.share[0] {
		t.Fatalf("a rival counts only the orbs in sight, marked while some are hidden: %q %q, slices %v", p.orbText(0), p.orbText(1), p.share[:3])
	}
	p.move(a, 12)
	if p.b.Viewer != 0 || !p.cover {
		t.Fatalf("back to the first human: viewer %d, cover %v", p.b.Viewer, p.cover)
	}
}

// Without a human here, or with the fog off, the board shows everything.
func TestFogOnlyForHumansHere(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	a.set.seats, a.set.Fog = []int{seatHuman + 1, seatHuman + 1}, true
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil || p.b.Viewer != -1 {
		t.Fatalf("bots alone: %v, viewer %d", err, p.b.Viewer)
	}
	a.set.seats, a.set.Fog = []int{seatHuman, seatHuman + 1}, false
	if p, err = newPlay(a, 5, 5, a.set.seats); err != nil || p.b.Viewer != -1 {
		t.Fatalf("no fog: %v, viewer %d", err, p.b.Viewer)
	}
}
