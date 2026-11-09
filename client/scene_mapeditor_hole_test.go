package client

import (
	"testing"

	"orbitrix/core"
)

func TestEditorSealedHoleIsAnIssueNotARefusal(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 2)
	m := core.NewMap(5, 5)
	m.Cells[12].Terrain = core.Hole
	e := newMapEditor(a, newHome(), m, nil)
	e.selectTool(brushBarrier)
	for _, side := range [][2]int{{7, 12}, {11, 12}, {12, 13}, {12, 17}} {
		if !e.paintEdge(side[0], side[1]) {
			t.Fatalf("Barrier beside the Hole at %v was refused", side)
		}
	}
	e.validate()
	if len(e.m.Edges) != 4 || len(e.issues) != 1 || e.issues[0].Message != "A Hole needs a Floor side without a Barrier" || !e.bad[12] {
		t.Fatalf("issues %v", e.issues)
	}
}
