package client

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/core"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Compact editor chrome is based on available logical space, independent of device pixel density.
func editorCompact(a *app) bool {
	v := a.safe()
	return min(v.W, v.H) < 480*a.u
}

func (e *mapEditor) layout(a *app) {
	u := a.u
	compact := editorCompact(a)
	pad, button, step, headerStep, split, headerLimit := 16*u, 44*u, 48*u, 52*u, 16*u, 720*u
	if compact {
		pad, button, step, headerStep, split, headerLimit = 8*u, 40*u, 44*u, 44*u, 12*u, 520*u
	}
	v := a.safe().Inset(pad)
	e.portrait = v.H > v.W
	gap := 8 * u
	e.p.view.Viewport = v
	e.p.back = ui.Rect{X: v.X, Y: v.Y, W: button, H: button}
	toolbarY := v.Y
	headerH := headerStep
	if e.portrait || v.W < headerLimit {
		e.field = ui.Rect{X: e.p.back.Right() + gap, Y: v.Y, W: v.W - button - gap, H: button}
		toolbarY += headerStep
		headerH = 2 * headerStep
	}
	e.toolB = ui.Rect{X: v.Right() - button, Y: toolbarY, W: button, H: button}
	e.saveB = ui.Rect{X: e.toolB.X - split - button, Y: toolbarY, W: button, H: button}
	e.testB = ui.Rect{X: e.saveB.X - 4*u - button, Y: toolbarY, W: button, H: button}
	e.previewB = ui.Rect{X: e.testB.X - split - button, Y: toolbarY, W: button, H: button}
	e.capacityB = ui.Rect{X: e.previewB.X - 4*u - button, Y: toolbarY, W: button, H: button}
	if headerH == headerStep {
		e.field = ui.Rect{X: e.p.back.Right() + gap, Y: v.Y, W: e.capacityB.X - e.p.back.Right() - 2*gap, H: button}
	}
	body := ui.Rect{X: v.X, Y: v.Y + headerH, W: v.W, H: max(1, v.H-headerH)}
	e.setCaret()
	e.portrait = v.H > v.W
	e.wide = !e.portrait && v.W >= 880*u && v.H >= 460*u
	e.control = [7]ui.Rect{}
	e.tabs = [3]ui.Rect{}
	e.headings = [3]ui.Rect{}
	if e.portrait {
		e.toolbarScroll.Horizontal = true
		actionWidth := 5*button + 8*u + 2*split
		total := actionWidth + split + 6*step + button
		e.toolbarScroll.Set(ui.Rect{X: v.X, Y: toolbarY, W: v.W, H: headerStep}, total)
		x := v.X - e.toolbarScroll.Offset
		e.capacityB.X = x
		e.previewB.X = x + button + 4*u
		e.testB.X = e.previewB.Right() + split
		e.saveB.X = e.testB.Right() + 4*u
		e.toolB.X = e.saveB.Right() + split
		for i := range e.control {
			e.control[i] = ui.Rect{X: e.toolB.Right() + split + float64(i)*step, Y: toolbarY, W: button, H: button}
		}
		dockHeight := 152 * u
		if compact {
			dockHeight = 136 * u
		}
		if e.outputTool() {
			dockHeight += 48 * u
		}
		dockH := min(dockHeight, body.H*.50)
		e.dock = ui.Rect{X: v.X, Y: v.Bottom() - dockH, W: v.W, H: dockH}
		top := body.Y
		e.canvas = ui.Rect{X: v.X, Y: top, W: v.W, H: max(16*u, e.dock.Y-top-gap)}
	} else {
		dw := 176 * u
		if compact {
			dw = 152 * u
		}
		if e.wide {
			dw = 196 * u
		}
		e.dock = ui.Rect{X: v.Right() - dw, Y: body.Y, W: dw, H: body.H}
		work := ui.Rect{X: body.X, Y: body.Y, W: body.W - dw - gap, H: body.H}
		if e.wide {
			for i := range e.control {
				e.control[i] = ui.Rect{X: work.X, Y: work.Y + float64(i)*52*u, W: button, H: button}
			}
			e.canvas = ui.Rect{X: work.X + 52*u, Y: work.Y, W: work.W - 52*u, H: work.H}
		} else {
			cols := min(7, max(1, int((work.W+4*u)/step)))
			rows := (7 + cols - 1) / cols
			top := work.Bottom() - float64(rows)*step + 4*u
			for i := range e.control {
				e.control[i] = ui.Rect{X: work.X + float64(i%cols)*step, Y: top + float64(i/cols)*step, W: button, H: button}
			}
			e.canvas = ui.Rect{X: work.X, Y: work.Y, W: work.W, H: max(16*u, top-work.Y-gap)}
		}
	}
	infoH := 40 * u
	if compact {
		infoH = 32 * u
	}
	if e.outputTool() && !e.portrait {
		infoH += 26 * u
	}
	e.info = ui.Rect{X: e.dock.X + 8*u, Y: e.dock.Bottom() - infoH - 8*u, W: e.dock.W - 16*u, H: infoH}
	e.channelB = ui.Rect{X: e.info.X, Y: e.info.Cy() - button/2, W: button, H: button}
	e.initialB = ui.Rect{}
	paletteBottom := e.info.Y - 8*u
	if e.outputTool() {
		e.initialB = ui.Rect{X: e.info.X, Y: e.info.Y - 48*u, W: e.info.W, H: 44 * u}
		paletteBottom = e.initialB.Y - 8*u
	}
	e.palette = e.palette[:0]
	e.paletteTools = e.paletteTools[:0]
	if e.portrait {
		tw := (e.dock.W - 16*u) / 3
		for i := range e.tabs {
			e.tabs[i] = ui.Rect{X: e.dock.X + 8*u + float64(i)*tw, Y: e.dock.Y + 4*u, W: tw, H: button}
		}
		paletteH := 52 * u
		if compact {
			paletteH = 52 * u
		}
		tools := editorGroups[e.category].tools
		bw := 72 * u
		e.dockScroll.Horizontal = true
		e.dockScroll.Set(ui.Rect{X: e.dock.X + 8*u, Y: e.dock.Y + step, W: e.dock.W - 16*u, H: paletteH}, float64(len(tools))*(bw+4*u)-4*u)
		x := e.dockScroll.Viewport.X - e.dockScroll.Offset
		for i, t := range tools {
			e.palette = append(e.palette, ui.Rect{X: x + float64(i)*(bw+4*u), Y: e.dock.Y + step, W: bw, H: paletteH})
			e.paletteTools = append(e.paletteTools, t)
		}
	} else {
		e.dockScroll.Horizontal = false
		top := e.dock.Y + 8*u
		rowH := button
		if e.wide {
			rowH = 36 * u
		}
		if !e.wide {
			e.tabs[e.category] = ui.Rect{X: e.dock.X + 8*u, Y: top, W: e.dock.W - 16*u, H: button}
			top += headerStep
		}
		viewport := ui.Rect{X: e.dock.X + 8*u, Y: top, W: e.dock.W - 16*u, H: max(1, paletteBottom-top)}
		total := 0.0
		for g, c := range editorGroups {
			if !e.wide && g != e.category {
				continue
			}
			if e.wide {
				total += 24 * u
			}
			total += float64(len(c.tools))*rowH + 8*u
		}
		e.dockScroll.Set(viewport, total)
		y := top - e.dockScroll.Offset
		for g, c := range editorGroups {
			if !e.wide && g != e.category {
				continue
			}
			if e.wide {
				e.headings[g] = ui.Rect{X: viewport.X, Y: y, W: viewport.W, H: 24 * u}
				y += 24 * u
			}
			for i, t := range c.tools {
				e.palette = append(e.palette, ui.Rect{X: viewport.X, Y: y + float64(i)*rowH, W: viewport.W, H: rowH})
				e.paletteTools = append(e.paletteTools, t)
			}
			y += float64(len(c.tools))*rowH + 8*u
		}
	}
	if e.revealTool {
		e.revealTool = false
		for i, tool := range e.paletteTools {
			if tool != e.tool {
				continue
			}
			r := e.palette[i]
			before := e.dockScroll.Offset
			e.dockScroll.Reveal(r)
			if e.dockScroll.Offset != before {
				e.layout(a)
			}

			break
		}
	}
	e.options = ui.Rect{X: v.Cx() - min(400*u, v.W)/2, Y: v.Cy() - 94*u, W: min(400*u, v.W), H: 188 * u}
	for i := range e.dims {
		e.dims[i] = ui.Rect{X: e.options.X + 12*u + float64(i%2)*(e.options.W-12*u)/2, Y: e.options.Y + 36*u + float64(i/2)*44*u, W: (e.options.W - 36*u) / 2, H: 44 * u}
	}
	for i := range e.option {
		e.option[i] = ui.Rect{X: e.options.X + 12*u, Y: e.options.Y + 132*u + float64(i)*48*u, W: e.options.W - 24*u, H: 44 * u}
	}
	e.channelPanel = ui.Rect{X: v.Cx() - min(304*u, v.W)/2, Y: v.Cy() - 122*u, W: min(304*u, v.W), H: 244 * u}
	cw := (e.channelPanel.W - 32*u) / 4
	for i := range e.channels {
		e.channels[i] = ui.Rect{X: e.channelPanel.X + 16*u + float64(i%4)*cw, Y: e.channelPanel.Y + 36*u + float64(i/4)*52*u, W: cw, H: 44 * u}
	}
	e.channelDone = ui.Rect{X: e.channelPanel.X + 12*u, Y: e.channelPanel.Bottom() - 48*u, W: e.channelPanel.W - 24*u, H: 44 * u}
	if e.tools && !e.portrait {
		for i := range e.tabs {
			e.tabs[i] = ui.Rect{X: e.dock.X + 8*u, Y: e.dock.Y + 8*u + float64(i)*48*u, W: e.dock.W - 16*u, H: 44 * u}
		}
	}
}

func editorControl(dst *ebiten.Image, a *app, r ui.Rect, label string, selected, enabled, primary bool) {
	fill, ink := a.sc.Tile, a.sc.Ink
	if primary {
		fill, ink = a.sc.Ink, a.sc.Ground
	}
	if !enabled {
		ink = a.sc.Dim
	} else if a.over(r) {
		if primary {
			fill = a.sc.InkHi
		} else {
			fill = a.sc.TileHi
		}
	}
	ui.FillRound(dst, r, 8*a.u, fill)
	if selected {
		ui.StrokeRound(dst, r, 8*a.u, 2*a.u, a.sc.Ink)
	}
	if label != "" {
		face := a.f.LabelBold
		if editorCompact(a) {
			face = a.f.SmallBold
		}
		ui.DrawText(dst, label, face, r.Cx(), r.Cy(), text.AlignCenter, ink)
	}
}

func editorGlyph(dst *ebiten.Image, a *app, r ui.Rect, kind int, enabled bool) {
	u := a.u
	if editorCompact(a) {
		u *= .8
	}
	c := a.sc.Ink
	if !enabled {
		c = a.sc.Dim
	}
	x, y, k := r.Cx(), r.Cy(), 12*u
	w := 2 * u
	switch kind {
	case 0, 1:
		sign := 1.0
		if kind == 1 {
			sign = -1
		}
		ui.Line(dst, x+k*.7*sign, y+k*.45, x+k*.7*sign, y-k*.2, w, c)
		ui.Line(dst, x+k*.7*sign, y-k*.2, x-k*.7*sign, y-k*.2, w, c)
		ui.Line(dst, x-k*.7*sign, y-k*.2, x-k*.15*sign, y-k*.75, w, c)
		ui.Line(dst, x-k*.7*sign, y-k*.2, x-k*.15*sign, y+k*.35, w, c)
	case 2:
		ui.Line(dst, x-k*.75, y+k*.3, x+k*.15, y-k*.75, w, c)
		ui.Line(dst, x+k*.15, y-k*.75, x+k*.8, y-k*.05, w, c)
		ui.Line(dst, x+k*.8, y-k*.05, x-k*.1, y+k*.8, w, c)
		ui.Line(dst, x-k*.1, y+k*.8, x-k*.75, y+k*.3, w, c)
		ui.Line(dst, x-k*.38, y-k*.1, x+k*.3, y+k*.55, w, c)
	case 3:
		ui.Line(dst, x-k, y, x+k, y, w, c)
		ui.Line(dst, x, y-k, x, y+k, w, c)
		for _, s := range []float64{-1, 1} {
			ui.Line(dst, x+k*s, y, x+k*.6*s, y-k*.3, w, c)
			ui.Line(dst, x+k*s, y, x+k*.6*s, y+k*.3, w, c)
			ui.Line(dst, x, y+k*s, x-k*.3, y+k*.6*s, w, c)
			ui.Line(dst, x, y+k*s, x+k*.3, y+k*.6*s, w, c)
		}
	case 4, 6:
		ui.Line(dst, x-k*.65, y, x+k*.65, y, w, c)
		if kind == 6 {
			ui.Line(dst, x, y-k*.65, x, y+k*.65, w, c)
		}
	case 5:
		for _, s := range []float64{-1, 1} {
			for _, t := range []float64{-1, 1} {
				ui.Line(dst, x+k*s, y+k*t, x+k*.4*s, y+k*t, w, c)
				ui.Line(dst, x+k*s, y+k*t, x+k*s, y+k*.4*t, w, c)
			}
		}
	case 11:
		for col := range 2 {
			for row := range 2 {
				box := ui.Rect{X: x - k + float64(col)*13*u, Y: y - k + float64(row)*13*u, W: 10 * u, H: 10 * u}
				ui.StrokeRound(dst, box, 2*u, w, c)
				ui.FillCircle(dst, box.Cx(), box.Cy(), u, c)
			}
		}
	case 12:
		ui.StrokeRound(dst, ui.Rect{X: x - k, Y: y - k*.65, W: 2 * k, H: 1.3 * k}, 4*u, w, c)
		ui.FillRound(dst, ui.Rect{X: x + 2*u, Y: y - 5*u, W: 7 * u, H: 10 * u}, 2*u, c)
	case 8:
		ui.Line(dst, x-k*.5, y-k*.8, x+k*.8, y, w, c)
		ui.Line(dst, x+k*.8, y, x-k*.5, y+k*.8, w, c)
		ui.Line(dst, x-k*.5, y+k*.8, x-k*.5, y-k*.8, w, c)
	case 7:
		for _, dx := range []float64{-.6, 0, .6} {
			ui.FillCircle(dst, x+k*dx, y, 1.7*u, c)
		}
	}
}

// The cached image is bounded at 48 px/cell, independent of zoom. Zoom scales
// this image inside the canvas instead of allocating a giant render target.
func (e *mapEditor) renderMap(a *app) {
	if e.mapImage != nil && e.mapRevision == e.revision && e.mapTheme == a.sc && e.mapMarks == a.set.Marks && e.mapCapacity == e.capacities && e.mapPreview == e.preview {
		return
	}
	cell := min(48.0, math.Floor(float64(ebiten.MaxImageSize()-2)/float64(max(e.m.Cols, e.m.Rows))))
	if cell < 1 {
		cell = 24
	}
	w, h := int(cell)*e.m.Cols, int(cell)*e.m.Rows
	if e.mapImage == nil || e.mapImage.Bounds().Dx() != w || e.mapImage.Bounds().Dy() != h {
		if e.mapImage != nil {
			e.mapImage.Deallocate()
		}
		e.mapImage = ebiten.NewImage(w, h)
	} else {
		e.mapImage.Clear()
	}
	state := boardview.MapState{Map: &e.m}
	capacityDesign := e.m
	if e.preview {
		capacityDesign = e.m.Clone()
		if cap(e.previewState) < len(e.m.Cells) {
			e.previewState = make([]uint8, len(e.m.Cells))
		} else {
			e.previewState = e.previewState[:len(e.m.Cells)]
			clear(e.previewState)
		}
		for i, c := range e.m.Cells {
			if c.Circuit != core.NoCircuit {
				state.CircuitOn[c.Group] = true
			}
			if c.Circuit.IsOutput() {
				capacityDesign.Cells[i].InitiallyOpen = !c.InitiallyOpen
			}
			if c.Circuit.IsSwitch() {
				e.previewState[i] = 1
				state.CircuitOn[c.Group] = true
			}
		}
		for i, edge := range e.m.Edges {
			if edge.Kind == core.Gate {
				capacityDesign.Edges[i].InitiallyOpen = !edge.InitiallyOpen
				state.CircuitOn[edge.Group] = true
			}
		}
		state.Pressed = e.previewState
	}
	tr := boardview.MapTransform{Cell: cell}
	boardview.DrawMapArt(e.mapImage, a.sc, state, tr, true, a.set.Marks)
	face := ui.NewFace(ui.SrcUI, cell*.24, 700)
	for i, c := range e.m.Cells {
		x, y := float64(i/e.m.Rows)*cell, float64(i%e.m.Rows)*cell
		r := ui.Rect{X: x + 1, Y: y + 1, W: cell - 2, H: cell - 2}
		if e.capacities && c.Terrain == core.Floor {
			cap := int(e.capacity[i])
			if e.preview {
				cap = capacityDesign.Capacity(i)
			}
			cap = min(255, cap)
			if c.Feature == core.NoFeature && c.Circuit == core.NoCircuit {
				ui.DrawText(e.mapImage, ui.Numbers[cap], face, x+cell*.5, y+cell*.5, text.AlignCenter, a.sc.Ink)
			} else {
				badge := ui.Rect{X: x + cell*.72, Y: y + cell*.04, W: cell * .24, H: cell * .25}
				ui.FillRound(e.mapImage, badge, cell*.05, a.sc.Ground)
				ui.DrawText(e.mapImage, ui.Numbers[cap], face, badge.Cx(), badge.Cy(), text.AlignCenter, a.sc.Ink)
			}
		}
		if e.bad[i] {
			ui.StrokeRound(e.mapImage, r, cell*.12, max(1, cell*.04), a.sc.Danger)
		}
	}
	for i, bad := range e.badEdges {
		if bad {
			e.drawEdgeHighlight(e.mapImage, e.m.Edges[i], tr, max(.5, cell*.02), a.sc.Danger)
		}
	}
	e.mapCell, e.mapRevision, e.mapTheme = cell, e.revision, a.sc
	e.mapMarks, e.mapCapacity, e.mapPreview = a.set.Marks, e.capacities, e.preview
}

func (e *mapEditor) renderIcons(a *app) {
	if e.icons != nil && e.iconTheme == a.sc && e.iconMarks == a.set.Marks && e.iconGroup == e.group && e.iconPreview == e.preview && e.iconInitial == e.initiallyOpen {
		return
	}
	if e.icons == nil {
		e.icons = ebiten.NewImage(64*len(editorTools), 64)
		for i := range e.brushIcons {
			e.brushIcons[i] = e.icons.SubImage(image.Rect(i*64, 0, (i+1)*64, 64)).(*ebiten.Image)
		}
	} else {
		e.icons.Clear()
	}
	for i, b := range editorBrushes {
		if i == brushErase || i == brushPan {
			continue
		}
		pressed := e.preview
		if b.Circuit.IsOutput() || b.Edge == int(core.Gate) {
			pressed = core.CircuitOpen(e.initiallyOpen, e.preview)
		}
		boardview.DrawMapBrush(e.icons, a.sc, b, ui.Rect{X: float64(i)*64 + 4, Y: 4, W: 56, H: 56}, e.group, pressed, a.set.Marks)
	}
	// A circuit swatch is a relationship control, separate from component art.
	cx := float64(brushErase*64 + 32)
	ui.FillRound(e.icons, ui.Rect{X: cx - 15, Y: 17, W: 30, H: 30}, 5, boardview.PartnerColor(a.sc, e.group))

	e.iconTheme, e.iconMarks, e.iconGroup, e.iconPreview = a.sc, a.set.Marks, e.group, e.preview
	e.iconInitial = e.initiallyOpen
}
func (e *mapEditor) brushIcon(dst *ebiten.Image, tool int, r ui.Rect, alpha float32) {
	op := ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Scale(r.W/64, r.H/64)
	op.GeoM.Translate(r.X, r.Y)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(e.brushIcons[tool], &op)
}

func (e *mapEditor) draw(dst *ebiten.Image, a *app) {
	e.layout(a)
	e.renderIcons(a)
	e.renderMap(a)
	u, sc := a.u, a.sc
	ui.DrawBack(dst, a.ui(), e.p.back)
	editorControl(dst, a, e.field, "", e.name.focused, true, false)
	title := string(e.name.shown)
	if title == "" {
		title = "Map name"
	}
	available := max(1, e.field.W-24*u)
	runes := []rune(title)
	for len(runes) > 0 && ui.TextWidth(string(runes), a.f.LabelBold) > available {
		if e.name.focused {
			runes = runes[1:]
		} else {
			runes = runes[:len(runes)-1]
		}
	}
	title = string(runes)
	ui.DrawText(dst, title, a.f.LabelBold, e.field.X+12*u, e.field.Cy(), text.AlignStart, sc.Ink)
	if e.name.focused {
		x := e.field.X + 12*u + ui.TextWidth(title, a.f.LabelBold) + 2*u
		ui.Line(dst, x, e.field.Cy()-8*u, x, e.field.Cy()+8*u, 2*u, sc.Ink)
	}
	toolbarDst := dst
	if e.portrait {
		toolbarDst = e.toolbarScroll.Clip(dst)
	}
	editorControl(toolbarDst, a, e.capacityB, "", e.capacities, true, false)
	editorGlyph(toolbarDst, a, e.capacityB, 11, true)
	editorControl(toolbarDst, a, e.previewB, "", e.preview, true, false)
	editorGlyph(toolbarDst, a, e.previewB, 12, true)
	split := 8 * u
	if editorCompact(a) {
		split = 6 * u
	}
	for _, x := range []float64{e.testB.X - split, e.toolB.X - split} {
		ui.Line(toolbarDst, x, e.testB.Y+12*u, x, e.testB.Bottom()-12*u, u, sc.Dim)
	}
	editorControl(toolbarDst, a, e.testB, "", false, len(e.issues) == 0, false)
	editorGlyph(toolbarDst, a, e.testB, 8, len(e.issues) == 0)
	editorControl(toolbarDst, a, e.saveB, "", false, true, true)
	editorActionSave(toolbarDst, a, e.saveB)
	editorControl(toolbarDst, a, e.toolB, "", e.more, true, false)
	editorGlyph(toolbarDst, a, e.toolB, 7, true)
	for i, r := range e.control {
		enabled := i != 0 || len(e.undo) > 0
		enabled = enabled && (i != 1 || len(e.redo) > 0)
		selected := i == 2 && e.tool == brushErase || i == 3 && e.tool == brushPan
		editorControl(toolbarDst, a, r, "", selected, enabled, false)
		editorGlyph(toolbarDst, a, r, i, enabled)
	}
	if e.portrait {
		e.toolbarScroll.DrawScrollbar(dst, a.ui())
	}
	ui.FillRound(dst, e.canvas, 10*u, sc.Ground)
	if e.canvasOf != dst || e.canvasBounds != e.canvas {
		e.canvasOf, e.canvasBounds = dst, e.canvas
		e.canvasClip = dst.SubImage(image.Rect(int(e.canvas.X), int(e.canvas.Y), int(e.canvas.Right()), int(e.canvas.Bottom()))).(*ebiten.Image)
	}
	clip := e.canvasClip
	k, ox, oy := e.grid()
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k/e.mapCell, k/e.mapCell)
	op.GeoM.Translate(ox, oy)
	op.Filter = ebiten.FilterLinear
	clip.DrawImage(e.mapImage, &op)
	// A pair guide is contextual; permanent wires would obscure the map.
	if e.hover >= 0 || e.pair >= 0 {
		i := e.hover
		if e.pair >= 0 {
			i = e.pair
		}
		c := e.m.Cells[i]
		if c.Feature == core.Link {
			for j, p := range e.m.Cells {
				if j != i && p.Feature == core.Link && p.Group == c.Group {
					x0, y0 := ox+(float64(i/e.m.Rows)+.5)*k, oy+(float64(i%e.m.Rows)+.5)*k
					x1, y1 := ox+(float64(j/e.m.Rows)+.5)*k, oy+(float64(j%e.m.Rows)+.5)*k
					ui.Line(clip, x0, y0, x1, y1, max(1, 1.5*u), ui.WithAlpha(boardview.PartnerColor(sc, c.Group), .55))
				}
			}
		}
	}
	if e.hover >= 0 {
		i := e.hover
		x, y := ox+float64(i/e.m.Rows)*k, oy+float64(i%e.m.Rows)*k
		r := ui.Rect{X: x, Y: y, W: k, H: k}
		if edge, ok := e.hoverEdge(i, a.in.x, a.in.y); ok {
			e.drawEdgeHighlight(clip, edge, boardview.MapTransform{X: ox, Y: oy, Cell: k}, u, sc.Ink)
		} else if e.tool != brushPan && editorBrushes[e.tool].Edge < 0 &&
			(e.tool != brushErase || e.m.Cells[i].Terrain != core.Floor || e.m.Cells[i].Feature != core.NoFeature || e.m.Cells[i].Circuit != core.NoCircuit) {
			if !e.stroke && e.tool != brushErase {
				e.brushIcon(clip, e.tool, r, .45)
			}
			ui.StrokeRound(clip, r.Inset(1), k*.12, max(1, 2*u), sc.Ink)
		}
	}
	e.drawDock(dst, a)
	if e.more {
		e.drawOptions(dst, a)
	}
	if e.channelsOpen {
		e.drawChannels(dst, a)
	}
	if e.ask != nil {
		e.ask.draw(dst, a)
	}
}

func (e *mapEditor) drawDock(dst *ebiten.Image, a *app) {
	u, sc := a.u, a.sc
	ui.FillRound(dst, e.dock, 10*u, sc.Tile)
	if e.portrait {
		for i, r := range e.tabs {
			editorControl(dst, a, r, editorGroups[i].name, i == e.category, true, false)
		}
	} else if !e.wide {
		editorControl(dst, a, e.tabs[e.category], editorGroups[e.category].name, true, true, false)
	}
	body := e.dockScroll.Clip(dst)
	if e.wide {
		for i, r := range e.headings {
			if r.H > 0 {
				ui.DrawText(body, editorGroups[i].name, a.f.Small, r.X+4*u, r.Cy(), text.AlignStart, sc.Dim)
			}
		}
	}
	for i, r := range e.palette {
		if r.Bottom() < e.dockScroll.Viewport.Y || r.Y > e.dockScroll.Viewport.Bottom() {
			continue
		}
		tool := e.paletteTools[i]
		selected := tool == e.tool
		editorControl(body, a, r, "", selected, true, false)
		size := 32 * u
		if editorCompact(a) {
			size = 24 * u
		}
		ir := ui.Rect{X: r.X + 2*u, Y: r.Cy() - size/2, W: size, H: size}
		if e.portrait {
			size = 28 * u
			if editorCompact(a) {
				size = 24 * u
			}
			ir = ui.Rect{X: r.Cx() - size/2, Y: r.Y, W: size, H: size}
		}
		e.brushIcon(body, tool, ir, 1)
		if tool == brushVoid {
			ui.StrokeRound(body, ir.Inset(7*u), 2*u, 1*u, sc.Dim)
		}
		if e.portrait {
			labelY, lineY := 38*u, 34*u
			if editorCompact(a) {
				labelY, lineY = 34*u, 30*u
			}
			if tool == brushBreak {
				ui.DrawText(body, "Breakable", a.f.Small, r.Cx(), r.Y+lineY, text.AlignCenter, sc.Ink)
				ui.DrawText(body, "wall", a.f.Small, r.Cx(), r.Y+lineY+11*u, text.AlignCenter, sc.Ink)
			} else {
				ui.DrawText(body, editorTools[tool], a.f.Small, r.Cx(), r.Y+labelY, text.AlignCenter, sc.Ink)
			}
		} else {
			ui.DrawText(body, editorTools[tool], a.f.Small, ir.Right()+6*u, r.Cy(), text.AlignStart, sc.Ink)
		}
	}
	e.dockScroll.DrawScrollbar(dst, a.ui())
	if e.outputTool() {
		label := "Starts closed"
		if e.initiallyOpen {
			label = "Starts open"
		}
		editorControl(dst, a, e.initialB, label, e.initiallyOpen, true, false)
	}
	ui.FillRound(dst, e.info, 8*u, sc.Tile)
	x := e.info.X + 4*u
	if e.channelTool() {
		editorControl(dst, a, e.channelB, "", e.channelsOpen, true, false)
		e.brushIcon(dst, brushErase, e.channelB, 1)
		x = e.channelB.Right() + 4*u
	}
	width := max(1, e.info.Right()-x-4*u)
	if e.helpTool != e.tool || e.helpWidth != width {
		e.helpTool, e.helpWidth = e.tool, width
		e.helpLines = wrap(editorHelp[e.tool], a.f.Small, width)
	}
	maxLines := 2
	if !e.portrait {
		maxLines = 3
		if e.outputTool() {
			maxLines = 4
		}
	}
	for i, l := range e.helpLines {
		if i >= maxLines {
			break
		}
		ui.DrawText(dst, l, a.f.Small, x, e.info.Cy()+(float64(i)-float64(min(maxLines, len(e.helpLines))-1)/2)*13*u, text.AlignStart, sc.Dim)
	}
	if e.tools && !e.portrait {
		ui.FillRound(dst, ui.Rect{X: e.dock.X, Y: e.dock.Y, W: e.dock.W, H: 8*u + 48*u*float64(len(e.tabs))}, 10*u, sc.Ground)
		for i, r := range e.tabs {
			editorControl(dst, a, r, editorGroups[i].name, i == e.category, true, false)
		}
	}
}

func (e *mapEditor) drawOptions(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	ui.FillRound(dst, a.safe(), 0, ui.WithAlpha(sc.Ground, .8))
	ui.FillRound(dst, e.options, 16*u, sc.Tile)
	ui.DrawText(dst, fmt.Sprintf("Map size  %d × %d", e.m.Cols, e.m.Rows), a.f.LabelBold, e.options.X+16*u, e.options.Y+18*u, text.AlignStart, sc.Ink)
	for i, label := range []string{"Columns −", "Columns +", "Rows −", "Rows +"} {
		editorControl(dst, a, e.dims[i], label, false, true, false)
	}
	editorControl(dst, a, e.option[0], "Done", false, true, true)
}
func (e *mapEditor) drawChannels(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	ui.FillRound(dst, a.safe(), 0, ui.WithAlpha(sc.Ground, .8))
	ui.FillRound(dst, e.channelPanel, 16*u, sc.Tile)
	label := "Circuit color"
	if e.tool == brushLink {
		label = "Pair color"
	}
	ui.DrawText(dst, label, a.f.LabelBold, e.channelPanel.X+16*u, e.channelPanel.Y+18*u, text.AlignStart, sc.Ink)
	for i, r := range e.channels {
		g := i + 1
		editorControl(dst, a, r, "", uint8(g) == e.group, true, false)
		ui.FillRound(dst, ui.Rect{X: r.Cx() - 15*u, Y: r.Cy() - 15*u, W: 30 * u, H: 30 * u}, 0, boardview.PartnerColor(sc, uint8(g)))

	}
	editorControl(dst, a, e.channelDone, "Done", false, true, true)
}

func editorActionSave(dst *ebiten.Image, a *app, r ui.Rect) {
	u, c := a.u, a.sc.Ground
	if editorCompact(a) {
		u *= .8
	}
	x, y := r.Cx(), r.Cy()
	ui.StrokeRound(dst, ui.Rect{X: x - 10*u, Y: y - 10*u, W: 20 * u, H: 20 * u}, 2*u, 2*u, c)
	ui.StrokeRound(dst, ui.Rect{X: x - 5*u, Y: y - 10*u, W: 10 * u, H: 7 * u}, u, 2*u, c)
	ui.StrokeRound(dst, ui.Rect{X: x - 6*u, Y: y + 2*u, W: 12 * u, H: 8 * u}, u, 2*u, c)
}

func (e *mapEditor) hoverEdge(i int, x, y float64) (core.MapEdge, bool) {
	if e.tool == brushErase {
		index := e.eraseEdgeAt(i, x, y)
		if index < 0 {
			return core.MapEdge{}, false
		}
		return e.m.Edges[index], true
	}
	if editorBrushes[e.tool].Edge >= 0 {
		aa, bb := e.edgeAt(i, x, y)
		if e.stroke {
			aa, bb = e.edgeA, e.edgeB
		}
		if aa < 0 {
			return core.MapEdge{}, false
		}
		return core.MapEdge{A: aa, B: bb}, true
	}
	return core.MapEdge{}, false
}

// Issues, placement and erase share the same boundary geometry and outline.
func (e *mapEditor) drawEdgeHighlight(dst *ebiten.Image, edge core.MapEdge, tr boardview.MapTransform, u float64, c color.RGBA) {
	x0, y0, x1, y1, ok := boardview.MapArtEdge(&e.m, edge)
	if !ok {
		return
	}
	x0, y0 = tr.Point(e.m.Rows, x0, y0)
	x1, y1 = tr.Point(e.m.Rows, x1, y1)
	pad := max(3*u, tr.Cell*.1)
	r := ui.Rect{X: min(x0, x1) - pad, Y: min(y0, y1) - pad, W: math.Abs(x1-x0) + 2*pad, H: math.Abs(y1-y0) + 2*pad}
	ui.StrokeRound(dst, r, pad, 2*u, c)
}
