package client

import (
	"testing"

	"orbitrix/core"
)

func TestEditorPortalPairingAndHistory(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	m := core.NewMap(9, 9)
	m.Cells[10], m.Cells[70] = core.MapCell{Feature: core.Link, Group: 1}, core.MapCell{Feature: core.Link, Group: 1}
	e := newMapEditor(a, newHome(), m, nil)
	e.selectTool(brushPortal)
	if !e.channelTool() || e.category != 1 {
		t.Fatal("Portal must use the Devices palette and pair-color picker")
	}
	e.checkpoint()
	if !e.paint(20, 0, 0) || e.pair != 20 || e.paint(21, 0, 0) {
		t.Fatal("Portal pair must begin on Floor and reject an adjacent partner")
	}
	e.m.Cells[60].Terrain = core.Wall
	if e.paint(60, 0, 0) || e.pair != 20 {
		t.Fatal("Portal accepted a Wall or discarded its pending pair")
	}
	if !e.paint(50, 0, 0) || e.pair != -1 {
		t.Fatal("Portal pair was not completed")
	}
	e.changed()
	for _, cell := range []int{20, 50} {
		if e.m.Cells[cell] != (core.MapCell{Feature: core.Portal, Group: 1}) || e.capacity[cell] != 0 {
			t.Fatal("Portal pair has wrong feature, color or capacity")
		}
	}
	if e.paint(30, 0, 0) {
		t.Fatal("Portal reused an occupied pair color")
	}
	e.history(false)
	if e.m.Cells[20].Feature != core.NoFeature || e.m.Cells[50].Feature != core.NoFeature {
		t.Fatal("undo did not remove Portal pair")
	}
	e.history(true)
	if e.m.Cells[20].Feature != core.Portal || e.m.Cells[50].Feature != core.Portal {
		t.Fatal("redo did not restore Portal pair")
	}
	e.group = 2
	e.paint(30, 0, 0)
	e.selectTool(brushLink)
	if e.pair != -1 {
		t.Fatal("switching pair types retained a pending Portal")
	}
}

func TestEditorPortalDirectionControls(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {568, 320}} {
		a := setupTestApp(size[0], size[1], 1, 2)
		m := core.NewMap(7, 7)
		m.Cells[16], m.Cells[32] = core.MapCell{Feature: core.Portal, Group: 1}, core.MapCell{Feature: core.Portal, Group: 1}
		e := newMapEditor(a, newHome(), m, nil)
		e.selectTool(brushPortal)
		if e.paint(16, 0, 0) || e.portalCell != 16 || e.pair != -1 {
			t.Fatal("selecting a Portal must inspect its pair without repainting")
		}
		e.layout(a)
		a.in = input{x: e.initialB.Cx(), y: e.initialB.Cy(), clicked: true}
		e.update(a)
		if !e.portalOpen {
			t.Fatal("direction control did not open its choices")
		}
		r := e.portalChoices[core.PortalHorizontal]
		a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
		e.update(a)
		if e.portalOpen || !e.dirty || len(e.undo) != 1 || e.m.Cells[16].PortalMode != core.PortalHorizontal || e.m.Cells[32].PortalMode != core.PortalHorizontal {
			t.Fatal("choice must update the whole pair in one undo step")
		}
		e.history(false)
		if e.m.Cells[16].PortalMode != core.PortalBoth || e.m.Cells[32].PortalMode != core.PortalBoth {
			t.Fatal("undo did not restore both direction settings")
		}
		e.history(true)
		if e.m.Cells[32].PortalMode != core.PortalHorizontal {
			t.Fatal("redo did not restore direction settings")
		}
		e.m.Cells[39].Terrain = core.Wall
		e.changed()
		if e.portalAllowed != core.VerticalAxis || e.portalMode != core.PortalVertical || e.setPortalMode(core.PortalHorizontal) || e.setPortalMode(core.PortalBoth) {
			t.Fatal("blocked horizontal exits must lock the pair vertically")
		}
		e.layout(a)
		a.in = input{x: e.initialB.Cx(), y: e.initialB.Cy(), clicked: true}
		e.update(a)
		if e.portalOpen {
			t.Fatal("a forced direction must not allow switching")
		}
	}
}
