package client

import (
	"testing"

	"orbitrix/core"
)

func TestEditorOutputStartingStatesAcrossLayouts(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {320, 640}, {640, 360}, {568, 320}} {
		a := setupTestApp(size[0], size[1], 1, 2)
		a.in.finger = true
		e := newMapEditor(a, newHome(), core.NewMap(9, 9), func(core.MapDefinition) error { return nil })
		for _, tool := range []int{brushDoor, brushHatch, brushGate} {
			e.selectTool(tool)
			e.layout(a)
			if !containsRect(e.dock, e.initialB) || e.initialB.H < 40*a.u || overlaps(e.initialB, e.info) || overlaps(e.initialB, e.dockScroll.Viewport) {
				t.Fatalf("initial state control clipped or overlapping at %v", size)
			}
			a.in.x, a.in.y = e.initialB.Cx(), e.initialB.Cy()
			a.in.clicked = true
			e.initiallyOpen = false
			if err := e.update(a); err != nil || !e.initiallyOpen {
				t.Fatalf("touch did not toggle initial state: %v", err)
			}
			a.in.clicked = false
			if editorBrushes[tool].Circuit.IsOutput() {
				if !e.paint(40, 0, 0) || !e.m.Cells[40].InitiallyOpen {
					t.Fatal("Door initial state was not painted")
				}
				e.checkpoint()
				e.initiallyOpen = false
				if !e.paint(40, 0, 0) || e.m.Cells[40].InitiallyOpen {
					t.Fatal("Door could not be repainted closed")
				}
				e.history(false)
				if !e.m.Cells[40].InitiallyOpen {
					t.Fatal("undo lost Door initial state")
				}
				e.history(true)
				if e.m.Cells[40].InitiallyOpen {
					t.Fatal("redo lost Door initial state")
				}
			} else {
				if !e.paintEdge(30, 31) || !e.m.Edges[0].InitiallyOpen {
					t.Fatal("Gate initial state was not painted")
				}
			}
		}
		b, err := core.EncodeMap(e.m)
		if err != nil {
			t.Fatal(err)
		}
		m, err := core.DecodeMap(b)
		if err != nil || !m.Edges[0].InitiallyOpen || m.Cells[40].InitiallyOpen {
			t.Fatalf("output states did not survive saving: %v", err)
		}
	}
}
