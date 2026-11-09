//go:build capture

package client

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/mapstore"
	"orbitrix/client/internal/ui"
	"orbitrix/core"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type mapCaptureCase struct {
	name                 string
	w, h                 float64
	dark, marks, preview bool
	mode                 string
	tool                 int
}

var mapCaptureCases = []mapCaptureCase{
	{"portal-desktop", 1040, 640, false, false, false, "", brushPortal},
	{"portal-mobile", 390, 844, true, true, false, "", brushPortal},
	{"portal-landscape", 568, 320, false, false, false, "", brushPortal},
	{"portal-art-light", 1100, 500, false, false, false, "portals", brushPortal},
	{"portal-art-dark", 1100, 500, true, true, false, "portals", brushPortal},
	{"portal-options-desktop", 1040, 640, false, false, false, "portal-options", brushPortal},
	{"portal-options-mobile", 390, 844, true, false, false, "portal-options", brushPortal},
	{"portal-locked-desktop", 1040, 640, false, false, false, "portal-vertical", brushPortal},
	{"portal-locked-mobile", 390, 844, true, false, false, "portal-vertical", brushPortal},
	{"portal-locked-landscape", 568, 320, false, false, false, "portal-horizontal", brushPortal},
	{"hole-desktop", 1040, 640, false, false, false, "", brushHole},
	{"hole-mobile-dark", 390, 844, true, false, false, "", brushHole},
	{"spillway-desktop", 1040, 640, false, false, false, "", brushSpillway},
	{"spillway-mobile-dark", 390, 844, true, false, false, "", brushSpillway},
	{"breakable-barrier-desktop", 1040, 640, false, false, false, "", brushBreakBarrier},
	{"breakable-barrier-mobile", 390, 844, true, false, false, "", brushBreakBarrier},
	{"breakable-barrier-small", 320, 640, false, false, false, "", brushBreakBarrier},
	{"breakable-barrier-states", 1100, 500, false, false, false, "barriers", brushBreakBarrier},
	{"breakable-barrier-states-dark", 1100, 500, true, false, false, "barriers", brushBreakBarrier},
	{"breakable-barrier-states-turned", 1100, 500, false, true, true, "barriers", brushBreakBarrier},
	{"door-orbs-light", 1040, 640, false, false, false, "door-orbs", brushDoor},
	{"door-orbs-dark", 1040, 640, true, false, false, "door-orbs", brushDoor},
	{"door-orbs-dark-marks", 1040, 640, true, true, false, "door-orbs", brushDoor},
	{"door-orbs-reopened", 1040, 640, false, true, true, "door-orbs", brushDoor},
	{"door-orbs-reopened-dark", 1040, 640, true, true, true, "door-orbs", brushDoor},
	{"door-orbs-mobile", 390, 844, true, true, false, "door-orbs", brushDoor},
	{"circuit-door-desktop", 1040, 640, false, false, false, "", brushDoor},
	{"circuit-door-mobile", 390, 844, true, false, false, "initial-open", brushDoor},
	{"circuit-gate-small", 320, 640, false, false, false, "initial-open", brushGate},
	{"circuit-door-landscape", 568, 320, true, false, false, "", brushDoor},
	{"editor-place-edge", 1040, 640, false, false, false, "erase-edge", brushGate},
	{"editor-erase-floor", 390, 844, true, false, false, "erase-floor", brushErase},
	{"editor-overflow-end", 320, 640, true, false, false, "overflow-end", brushHold},
	{"wall-corners-light", 1250, 700, false, false, false, "wall-corners", brushFloor},
	{"wall-corners-dark", 1250, 700, true, false, false, "wall-corners", brushFloor},
	{"wall-corners-turned", 1250, 700, true, false, true, "wall-corners", brushFloor},
	{"editor-views-mobile", 320, 640, false, false, true, "views", brushFloor},
	{"editor-views-desktop", 1040, 640, true, false, true, "views", brushFloor},
	{"editor-erase-edge", 1040, 640, false, false, false, "erase-edge", brushErase},
	{"editor-erase-cell", 390, 844, true, false, false, "erase-cell", brushErase},
	{"editor-title-mobile", 320, 640, false, false, false, "title", brushFloor},
	{"editor-issue-mobile", 390, 844, false, false, false, "issue", brushLink},
	{"editor-issue-desktop", 1040, 640, true, false, false, "issue", brushLink},
	{"editor-gate-issue-mobile", 390, 844, true, false, false, "gate-issue", brushGate},
	{"editor-gate-issue-desktop", 1040, 640, false, false, true, "gate-issue", brushGate},
	{"editor-desktop", 1040, 640, false, false, false, "", brushLatch},
	{"editor-mobile", 390, 844, false, false, false, "", brushLatch},
	{"editor-mobile-defenses", 390, 844, false, false, false, "", brushBreak},
	{"editor-small-defenses", 320, 640, true, true, false, "", brushBreak},
	{"editor-landscape", 640, 360, false, false, false, "", brushHold},
	{"editor-desktop-dark", 1040, 640, true, false, false, "", brushLink},
	{"editor-mobile-dark", 390, 844, true, false, false, "", brushToggle},
	{"editor-landscape-dark", 568, 320, true, true, false, "", brushGate},
	{"editor-desktop-pressed", 1040, 640, true, true, true, "", brushHold},
	{"editor-mobile-pressed", 390, 844, true, true, true, "", brushHold},
	{"channels-desktop", 1040, 640, false, true, false, "channels", brushGate},
	{"channels-mobile", 390, 844, true, true, false, "channels", brushLink},
	{"options-mobile", 390, 844, false, false, false, "options", brushFloor},
	{"board-desktop", 1040, 640, false, false, true, "game", brushFloor},
	{"board-mobile-dark", 390, 844, true, true, true, "game", brushFloor},
	{"library-mobile", 390, 844, false, false, false, "library", brushFloor},
	{"large-desktop", 1040, 640, false, false, false, "large", brushWall},
	{"large-mobile-dark", 390, 844, true, true, false, "large", brushLink},
	{"art-states", 1100, 844, false, false, false, "art", brushFloor},
	{"art-colorblind", 1100, 844, false, true, false, "art", brushFloor},
	{"art-dark", 1100, 844, true, false, false, "art", brushFloor},
}

type mapCaptureGame struct {
	dir     string
	index   int
	err     error
	timings map[string]time.Duration
}

func (g *mapCaptureGame) Layout(int, int) (int, int) { return 1100, 844 }
func (g *mapCaptureGame) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.index >= len(mapCaptureCases) {
		return ebiten.Termination
	}
	return nil
}

func editorShowcaseMap() core.MapDefinition {
	m := core.NewMap(13, 11)
	m.Name = "Crossing paths"
	for c := 0; c < 2; c++ {
		for r := 0; r < 2; r++ {
			m.Cells[c*m.Rows+r].Terrain = core.Void
		}
	}
	for c := 4; c <= 6; c++ {
		for r := 4; r <= 6; r++ {
			terrain := core.Wall
			if (c == 4 || c == 6) && (r == 4 || r == 6) {
				terrain = core.BreakableWall
			}
			m.Cells[c*m.Rows+r].Terrain = terrain
		}
	}
	m.Cells[2*m.Rows+2] = core.MapCell{Feature: core.Link, Group: 4}
	m.Cells[10*m.Rows+8] = core.MapCell{Feature: core.Link, Group: 4}
	m.Cells[2*m.Rows+5] = core.MapCell{Feature: core.Portal, Group: 4}
	m.Cells[10*m.Rows+5] = core.MapCell{Feature: core.Portal, Group: 4}
	m.Cells[2*m.Rows+3] = core.MapCell{Circuit: core.Latch, Group: 1}
	m.Cells[9*m.Rows+4] = core.MapCell{Circuit: core.Toggle, Group: 2}
	m.Cells[2*m.Rows+7] = core.MapCell{Circuit: core.Hold, Group: 3}
	m.Cells[8*m.Rows+8].Feature = core.Reservoir
	m.Cells[9*m.Rows+8].Feature = core.Spillway
	m.Cells[6*m.Rows+3] = core.MapCell{Circuit: core.Toggle, Group: 1}
	m.Cells[8*m.Rows+2] = core.MapCell{Circuit: core.Door, Group: 1}
	m.Cells[9*m.Rows+2] = core.MapCell{Circuit: core.Door, Group: 1, InitiallyOpen: true}
	m.Edges = []core.MapEdge{
		{A: 3*m.Rows + 1, B: 4*m.Rows + 1, Kind: core.Gate, Group: 1},
		{A: 3*m.Rows + 2, B: 4*m.Rows + 2, Kind: core.Gate, Group: 1},
		{A: 3*m.Rows + 3, B: 4*m.Rows + 3, Kind: core.Gate, Group: 1},
		{A: 7*m.Rows + 5, B: 7*m.Rows + 6, Kind: core.Gate, Group: 2},
		{A: 8*m.Rows + 5, B: 8*m.Rows + 6, Kind: core.Gate, Group: 2},
		{A: 9*m.Rows + 5, B: 9*m.Rows + 6, Kind: core.Gate, Group: 2},
		{A: 4*m.Rows + 7, B: 5*m.Rows + 7, Kind: core.Gate, Group: 3},
		{A: 4*m.Rows + 8, B: 5*m.Rows + 8, Kind: core.Gate, Group: 3},
		{A: 5*m.Rows + 8, B: 5*m.Rows + 9, Kind: core.Gate, Group: 3},
		{A: 4*m.Rows + 9, B: 5*m.Rows + 9, Kind: core.Barrier},
		{A: 8*m.Rows + 8, B: 8*m.Rows + 9, Kind: core.BreakableBarrier},
	}
	return m
}
func drawArtStates(dst *ebiten.Image, a *app) {
	sc := a.sc
	ui.DrawText(dst, "Shared map pieces", a.f.Title, 32, 32, text.AlignStart, sc.Ink)
	for i, feature := range []core.CircuitKind{core.Latch, core.Toggle, core.Hold, core.Door} {
		x := 32 + float64(i)*270
		label := []string{"Latch", "Toggle", "Hold", "Door"}[i]
		ui.DrawText(dst, label, a.f.Heading, x, 86, text.AlignStart, sc.Ink)
		for j, pressed := range []bool{false, true} {
			r := ui.Rect{X: x + float64(j)*120, Y: 112, W: 104, H: 104}
			boardview.DrawMapBrush(dst, sc, boardview.Brush{Circuit: feature, Edge: -1}, r, uint8(i+1), pressed, a.set.Marks)
			state := "Released"
			if pressed {
				state = "Pressed"
			}
			if feature == core.Door {
				state = "Closed"
				if pressed {
					state = "Open"
				}
			}
			ui.DrawText(dst, state, a.f.LabelBold, r.Cx(), 260, text.AlignCenter, sc.Ink)
		}
	}
	ui.DrawText(dst, "Link pairs", a.f.Heading, 32, 316, text.AlignStart, sc.Ink)
	for i := 0; i < core.MaxMapGroups; i++ {
		boardview.DrawMapBrush(dst, sc, boardview.Brush{Feature: core.Link, Edge: -1}, ui.Rect{X: 30 + float64(i)*87, Y: 352, W: 76, H: 76}, uint8(i+1), false, a.set.Marks)
	}
	for i, label := range []string{"Wall", "Breakable Wall", "Damaged", "Reservoir"} {
		x := 32 + float64(i)*200
		ui.DrawText(dst, label, a.f.LabelBold, x+64, 474, text.AlignCenter, sc.Ink)
		r := ui.Rect{X: x, Y: 500, W: 128, H: 128}
		if i == 3 {
			boardview.DrawMapBrush(dst, sc, boardview.Brush{Feature: core.Reservoir, Edge: -1}, r, 0, false, a.set.Marks)
		} else {
			terrain, durability := core.BreakableWall, uint8(2)
			if i == 0 {
				terrain = core.Wall
			}
			if i == 2 {
				durability = 1
			}
			boardview.DrawMapWall(dst, sc, r, terrain, 0, durability)
		}
	}
	for i, open := range []bool{false, true} {
		x := 48 + float64(i)*548
		label := "Closed span"
		if open {
			label = "Open span"
		}
		ui.DrawText(dst, label, a.f.LabelBold, x, 690, text.AlignStart, sc.Ink)
		for j := range 4 {
			boardview.DrawMapEdgePiece(dst, sc, core.Gate, 2, open, x+float64(j)*100, 752, x+float64(j+1)*100, 752, 100, a.set.Marks)
		}
	}
	m := core.NewMap(3, 3)
	for row, feature := range []core.Feature{core.Reservoir, core.Spillway} {
		m.Cells[0].Feature = feature
		for i, owner := range []int8{core.NoOwner, 0, 1} {
			state := boardview.MapState{Map: &m, Owner: []int8{owner}, Count: []uint8{1}}
			if owner == core.NoOwner {
				state.Count[0] = 0
			}
			piece := ebiten.NewImage(96, 96)
			boardview.DrawMapArt(piece, sc, state, boardview.MapTransform{Cell: 96}, true, a.set.Marks)
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(.8, .8)
			op.GeoM.Translate(832+float64(i)*84, 516+float64(row)*84)
			dst.DrawImage(piece, &op)
			piece.Deallocate()
		}
	}
}

func drawDoorOrbStates(dst *ebiten.Image, a *app, reopen bool) error {
	m := core.NewMap(5, 3)
	groups := []uint8{1, 2, 7, 8, 11}
	for col, group := range groups {
		m.Cells[col*m.Rows+1] = core.MapCell{Circuit: core.Door, Group: group, InitiallyOpen: true}
		m.Cells[col*m.Rows+2] = core.MapCell{Circuit: core.Toggle, Group: group}
	}
	g, err := core.NewGameWithMap(m, 5)
	if err != nil {
		return err
	}
	for col := range groups {
		cell := col*m.Rows + 1
		g.Count[cell], g.Owner[cell], g.Orbs[col] = uint8(col+1), int8(col), int32(col+1)
	}
	b := boardview.New(g)
	b.Layout(ui.Rect{X: 24, Y: 80, W: a.w - 48, H: a.h - 104}, 1)
	label := "Trapped orbs"
	if reopen {
		label = "Reopened Doors"
	}
	draw := func(tms float64) {
		dst.Fill(a.sc.Ground)
		ui.DrawText(dst, label, a.f.Title, 24, 32, text.AlignStart, a.sc.Ink)
		b.Draw(dst, a.sc, tms, 0, a.set.Marks)
	}
	draw(1000)
	for _, group := range groups {
		g.ReplayTerrain([]core.TerrainEvent{{Cell: -1, Group: group, Open: true}})
	}
	b.Sync(g)
	draw(1017)
	before := make([]byte, dst.Bounds().Dx()*dst.Bounds().Dy()*4)
	dst.ReadPixels(before)
	draw(20000)
	after := make([]byte, len(before))
	dst.ReadPixels(after)
	if !bytes.Equal(before, after) {
		return &captureMapError{"locked Door orbs moved between frames"}
	}
	// A lone trapped orb must be Floor-colored, with the shared lock art above it.
	x, y := b.Centre(1)
	nx, ny := b.Centre(4)
	cell := math.Hypot(nx-x, ny-y)
	pixels := image.NewRGBA(dst.Bounds())
	copy(pixels.Pix, after)
	if pixels.RGBAAt(int(x), int(y-cell*.08)) != a.sc.Tile {
		return fmt.Errorf("trapped orb ink %v, want Floor %v at (%v, %v), capacity %d", pixels.RGBAAt(int(x), int(y-cell*.08)), a.sc.Tile, x, y-cell*.08, b.State.Geo.Cap[1])
	}
	reference := ebiten.NewImage(dst.Bounds().Dx(), dst.Bounds().Dy())
	defer reference.Deallocate()
	boardview.DrawMapBrush(reference, a.sc, boardview.Brush{Circuit: core.Door, Edge: -1},
		ui.Rect{X: x - cell/2, Y: y - cell/2, W: cell, H: cell}, groups[0], false, a.set.Marks)
	refPixels := image.NewRGBA(dst.Bounds())
	reference.ReadPixels(refPixels.Pix)
	lockInk := ui.Palettes[ui.Dark].Tile
	if a.sc == &ui.Palettes[ui.Dark] {
		lockInk = ui.Palettes[ui.Light].Tile
	}
	checked := 0
	for py := int(y - cell*.1); py < int(y+cell*.1); py++ {
		for px := int(x - cell*.1); px < int(x+cell*.1); px++ {
			ink := ui.InkOn(a.sc.Players[0])
			if refPixels.RGBAAt(px, py) == ink {
				checked++
				if pixels.RGBAAt(px, py) != lockInk {
					return &captureMapError{"occupied locked icon must use the opposite theme's Floor color above its orbs"}
				}
			}
		}
	}
	if checked == 0 {
		return &captureMapError{"lock reference did not overlap the trapped orb"}
	}
	if reopen {
		for _, group := range groups {
			g.ReplayTerrain([]core.TerrainEvent{{Cell: -1, Group: group, Open: false}})
		}
		b.Sync(g)
		draw(20017)
		for col := range groups {
			cell := col*m.Rows + 1
			if !b.State.Geo.Playable(cell) || b.State.Owner[cell] != int8(col) || b.State.Count[cell] != uint8(col+1) {
				return &captureMapError{"reopening failed to restore the trapped owner's orbs"}
			}
		}
	}
	return nil
}

func (g *mapCaptureGame) Draw(*ebiten.Image) {
	if g.index >= len(mapCaptureCases) || g.err != nil {
		return
	}
	c := mapCaptureCases[g.index]
	a := setupTestApp(c.w, c.h, 1, 2)
	if c.dark {
		a.sc = &ui.Palettes[ui.Dark]
	}
	a.set.Marks = c.marks
	a.in.x, a.in.y = offScreen, offScreen
	m := editorShowcaseMap()
	if issues := m.Validate(); len(issues) > 0 {
		g.err = &captureMapError{issues[0].Message}
		return
	}
	var s scene
	switch c.mode {
	case "game":
		game, err := core.NewGameWithMap(m, 2)
		if err != nil {
			g.err = err
			return
		}
		if c.preview {
			for i, cfg := range m.Cells {
				if cfg.Circuit.IsSwitch() {
					_, _ = game.Apply(i)
				}
			}
		}
		rng := core.NewRNG(66)
		var moves []int
		for i := 0; i < 18 && !game.Over(); i++ {
			moves = game.LegalMoves(moves[:0])
			cell := moves[rng.Intn(len(moves))]
			if m.Cells[cell].Circuit.IsSwitch() || game.Count[cell]+1 >= game.Geo.Cap[cell] {
				continue
			}
			_, _ = game.Apply(cell)
		}
		s = newPlayOn(a, game, []int{seatHuman, 1}, []string{"Ana", "Jester"}, pauseLocal)
	case "library":
		store := mapstore.Load(
			func(string) ([]byte, error) { return nil, fs.ErrNotExist },
			func(string, []byte) error { return nil },
		)
		lib := &mapsScene{back: newHome(), selected: 1, store: store}
		lib.refresh()
		s = lib
	case "art", "wall-corners", "door-orbs", "barriers", "portals":
	default:
		if c.mode == "large" {
			m = resizeMap(m, 40, 40)
			for col := 16; col < 36; col += 5 {
				for row := 3; row < 34; row += 5 {
					for dx := 0; dx < 2; dx++ {
						for dy := 0; dy < 2; dy++ {
							m.Cells[(col+dx)*40+row+dy].Terrain = core.Wall
						}
					}
				}
			}
		}
		e := newMapEditor(a, newHome(), m, func(core.MapDefinition) error { return nil })
		if c.mode == "initial-open" {
			e.initiallyOpen = true
		}
		if c.mode == "title" {
			e.name.set("A long custom map title being edited")
			e.name.focused = true
		}
		if c.mode == "issue" {
			e.m.Cells[3*m.Rows+3].Feature = core.Link
			e.m.Cells[3*m.Rows+3].Group = 12
			e.changed()
			e.snack = snackbar{message: e.note, ticks: snackIn}
		}
		if c.mode == "gate-issue" {
			e.m.Cells[2*m.Rows+3], e.m.Cells[9*m.Rows+4] = core.MapCell{}, core.MapCell{}
			e.changed()
		}
		if c.mode == "views" {
			e.capacities = true
		}
		e.selectTool(c.tool)
		if c.mode == "erase-edge" || c.mode == "erase-cell" || c.mode == "erase-floor" {
			e.layout(a)
			k, ox, oy := e.grid()
			if c.mode == "erase-edge" {
				x0, y0, x1, y1, _ := boardview.MapArtEdge(&e.m, e.m.Edges[0])
				a.in.x, a.in.y = ox+(x0+x1)*k/2, oy+(y0+y1)*k/2
			} else if c.mode == "erase-floor" {
				a.in.x, a.in.y = ox+2.5*k, oy+8.5*k
			} else {
				a.in.x, a.in.y = ox+4.5*k, oy+4.5*k
			}
			e.hover = e.cellAt(a.in.x, a.in.y)
		}
		e.preview = c.preview
		e.group = 4
		if c.tool == brushLatch {
			e.group = 1
		}
		if c.tool == brushToggle {
			e.group = 2
		}
		if c.tool == brushHold || c.tool == brushGate {
			e.group = 3
		}
		if c.mode == "portal-options" || c.mode == "portal-vertical" || c.mode == "portal-horizontal" {
			e.portalCell = 2*m.Rows + 5
			if c.mode == "portal-vertical" {
				e.m.Cells[m.Rows+5].Terrain = core.Wall
			}
			if c.mode == "portal-horizontal" {
				e.m.Cells[2*m.Rows+4].Terrain = core.Wall
			}
			e.changed()
			e.portalOpen = c.mode == "portal-options"
		}
		if c.mode == "channels" {
			e.channelsOpen = true
		}
		if c.mode == "options" {
			e.more = true
		}
		if c.mode == "large" && c.dark {
			e.zoom = 4
		}
		if c.mode == "overflow-end" {
			e.layout(a)
			e.toolbarScroll.Offset = e.toolbarScroll.MaxScroll()
			e.layout(a)
		}
		s = e
	}
	out := ebiten.NewImage(int(c.w), int(c.h))
	defer out.Deallocate()
	out.Fill(a.sc.Ground)
	if c.mode == "portals" {
		for row, feature := range []core.Feature{core.Link, core.Portal} {
			y := 30 + float64(row)*170
			ui.DrawText(out, []string{"Link pairs", "Portal pairs"}[row], a.f.Heading, 30, y, text.AlignStart, a.sc.Ink)
			for i := range core.MaxMapGroups {
				boardview.DrawMapBrush(out, a.sc, boardview.Brush{Feature: feature, Edge: -1}, ui.Rect{X: 30 + float64(i)*87, Y: y + 35, W: 76, H: 76}, uint8(i+1), false, c.marks)
			}
		}
		ui.DrawText(out, "Portal at map and palette sizes", a.f.LabelBold, 30, 380, text.AlignStart, a.sc.Ink)
		for i, size := range []float64{64, 48, 32, 24} {
			boardview.DrawMapBrush(out, a.sc, boardview.Brush{Feature: core.Portal, Edge: -1}, ui.Rect{X: 30 + float64(i)*100, Y: 410, W: size, H: size}, 4, false, c.marks)
		}
		for i, mode := range []core.PortalMode{core.PortalBoth, core.PortalHorizontal, core.PortalVertical} {
			x := 520 + float64(i)*190
			ui.DrawText(out, portalLabels[mode], a.f.LabelBold, x, 380, text.AlignStart, a.sc.Ink)
			boardview.DrawMapBrush(out, a.sc, boardview.Brush{Feature: core.Portal, PortalMode: mode, Edge: -1}, ui.Rect{X: x, Y: 410, W: 64, H: 64}, 4, false, c.marks)
		}
	} else if c.mode == "barriers" {
		ui.DrawText(out, "Breakable Barriers", a.f.Title, 30, 36, text.AlignStart, a.sc.Ink)
		for i, durability := range []uint8{2, 1, 0} {
			m := core.NewMap(3, 3)
			m.Edges = []core.MapEdge{{A: 4, B: 5, Kind: core.BreakableBarrier}, {A: 4, B: 7, Kind: core.BreakableBarrier}}
			state := boardview.MapState{Map: &m, EdgeDurability: []uint8{durability, durability}}
			boardview.DrawMapArt(out, a.sc, state, boardview.MapTransform{X: 30 + float64(i)*350, Y: 100, Cell: 100, Turned: c.preview}, true, c.marks)
			ui.DrawText(out, []string{"Intact", "Damaged", "Destroyed"}[i], a.f.LabelBold, 180+float64(i)*350, 446, text.AlignCenter, a.sc.Ink)
		}
	} else if c.mode == "wall-corners" {
		for missing := range 5 {
			for variant := range 2 {
				m := core.NewMap(3, 3)
				cells := [4]int{0, 3, 4, 1}
				for _, i := range cells {
					m.Cells[i].Terrain = core.Wall
				}
				if missing < 4 {
					m.Cells[cells[missing]].Terrain = core.Floor
				}
				state := boardview.MapState{Map: &m}
				if variant == 1 {
					state.Durability = make([]uint8, len(m.Cells))
					for _, i := range cells {
						m.Cells[i].Terrain = core.BreakableWall
						state.Durability[i] = 1
					}
					if missing < 4 {
						state.Durability[cells[missing]] = 0
					}
				}
				boardview.DrawMapArt(out, a.sc, state, boardview.MapTransform{X: 20.2 + float64(missing)*245, Y: 20.2 + float64(variant)*340, Cell: 75.3, Turned: c.preview}, true, false)
			}
		}
	} else if c.mode == "door-orbs" {
		if err := drawDoorOrbStates(out, a, c.preview); err != nil {
			g.err = err
		}
	} else if c.mode == "art" {
		drawArtStates(out, a)
	} else {
		s.draw(out, a)
		drawSnackbarOverlay(out, a, s)
	}
	pixels := image.NewRGBA(image.Rect(0, 0, int(c.w), int(c.h)))
	out.ReadPixels(pixels.Pix)
	if c.mode == "erase-floor" {
		e := s.(*mapEditor)
		if e.m.Cells[e.hover].Terrain != core.Floor || e.m.Cells[e.hover].Feature != core.NoFeature {
			g.err = &captureMapError{"erase-floor fixture must hover plain floor"}
			return
		}
		before := append([]byte(nil), pixels.Pix...)
		e.hover = -1
		out.Fill(a.sc.Ground)
		e.draw(out, a)
		drawSnackbarOverlay(out, a, s)
		out.ReadPixels(pixels.Pix)
		// Check the canvas; unrelated toolbar antialiasing can differ by one pixel.
		bounds := image.Rect(int(e.canvas.X), int(e.canvas.Y), int(e.canvas.Right()), int(e.canvas.Bottom())).Intersect(pixels.Bounds())
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := pixels.PixOffset(bounds.Min.X, y)
			end := i + 4*bounds.Dx()
			if !bytes.Equal(before[i:end], pixels.Pix[i:end]) {
				g.err = &captureMapError{"erase mode highlighted plain floor"}
				return
			}
		}
	}
	if c.mode == "wall-corners" {
		// Interior joins must remain solid ink even when the grid lands between pixels.
		for missing := range 5 {
			tr := boardview.MapTransform{X: 20.2 + float64(missing)*245, Y: 20.2, Cell: 75.3, Turned: c.preview}
			if missing == 4 {
				x, y := tr.Point(3, 1, 1)
				for dx := -3; dx <= 3; dx++ {
					for dy := -3; dy <= 3; dy++ {
						if pixels.RGBAAt(int(x)+dx, int(y)+dy) != a.sc.Dim {
							g.err = &captureMapError{"enclosed wall vertex contains a seam"}
							return
						}
					}
				}
			}
			for edge, midpoint := range [4][2]float64{{1, .5}, {1.5, 1}, {1, 1.5}, {.5, 1}} {
				if edge == missing || (edge+1)%4 == missing {
					continue
				}
				x, y := tr.Point(3, midpoint[0], midpoint[1])
				for dx := -2; dx <= 2; dx++ {
					for dy := -2; dy <= 2; dy++ {
						got := pixels.RGBAAt(int(x)+dx, int(y)+dy)
						if got != a.sc.Dim {
							g.err = &captureMapError{fmt.Sprintf("%s: wall join at (%d, %d) is %v, want %v", c.name, int(x)+dx, int(y)+dy, got, a.sc.Dim)}
							return
						}
					}
				}
			}
		}
	}
	if c.mode == "large" {
		e := s.(*mapEditor)
		cached := e.mapImage
		start := time.Now()
		for range 20 {
			out.Fill(a.sc.Ground)
			e.draw(out, a)
			out.ReadPixels(pixels.Pix)
		}
		g.timings[c.name] = time.Since(start) / 20
		if e.mapImage != cached {
			g.err = &captureMapError{"steady drawing replaced the map cache"}
			return
		}
	}
	f, err := os.Create(filepath.Join(g.dir, c.name+".png"))
	if err == nil {
		err = png.Encode(f, pixels)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		g.err = err
	}
	g.index++
}

type captureMapError struct{ message string }

func (e *captureMapError) Error() string { return e.message }
func TestCaptureMaps(t *testing.T) {
	dir := os.Getenv("ORBITRIX_CAPTURE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "dist", "map-captures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	ebiten.SetWindowSize(160, 100)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowPosition(-32000, -32000)
	g := &mapCaptureGame{dir: dir, timings: make(map[string]time.Duration)}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	for name, d := range g.timings {
		t.Logf("%s: warm render + GPU readback %v/frame", name, d)
	}
}
