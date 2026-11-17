package boardview

import (
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

func TestFogSight(t *testing.T) {
	g, err := core.NewGame(5, 5, 2)
	if err == nil {
		err = g.SetFog()
	}
	if err != nil {
		t.Fatal(err)
	}
	b := New(g)
	for _, cell := range []int{0, 24} {
		waves, err := g.Apply(cell)
		if err != nil {
			t.Fatal(err)
		}
		b.Start(cell, g.Owner[cell], waves, g.ProducedAt, nil)
	}
	b.Look()
	if !b.Sees(24) || !b.Sees(12) {
		t.Fatal("without a viewer every cell shows")
	}
	b.Viewer = 0
	b.Look()
	if !b.Sees(6) || b.Sees(12) || b.Sees(24) || b.knows(24) || !b.knows(6) {
		t.Fatal("player 0 sees around their own orb only")
	}
	b.Viewer = 1
	b.Look()
	if !b.Sees(18) || b.Sees(6) || !b.knows(18) || b.knows(6) {
		t.Fatal("each side keeps its own sight")
	}
}

// TestFogSightFliesWithOrbs: an orb in flight lets its side see around it before it lands.
func TestFogSightFliesWithOrbs(t *testing.T) {
	g, err := core.NewGame(5, 5, 2)
	if err == nil {
		err = g.SetFog()
	}
	if err != nil {
		t.Fatal(err)
	}
	b := New(g)
	for _, cell := range []int{0, 24, 0} { // player 0's second orb in the corner explodes into cells 1 and 5
		mover := int8(g.Turn)
		waves, err := g.Apply(cell)
		if err != nil {
			t.Fatal(err)
		}
		b.Start(cell, mover, waves, g.ProducedAt, nil)
	}
	b.Layout(ui.Rect{W: 500, H: 500}, 1) // an orb's cell is where it is drawn
	b.Viewer = 0
	for b.Playing && b.State.Count[1] == 0 && !b.Sees(2) {
		b.Tick(nil)
	}
	if b.State.Count[1] != 0 || !b.Sees(2) {
		t.Fatal("cell 2 came into sight only once the orb landed beside it")
	}
}
