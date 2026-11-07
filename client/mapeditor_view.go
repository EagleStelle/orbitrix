package client

import (
	"fmt"
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"orbitrix/core"
)

// Compact editor chrome is based on available logical space, independent of device pixel density.
func editorCompact(a *app) bool {
	v := a.safe()
	return min(v.w, v.h) < 480*a.u
}

func (e *mapEditor) layout(a *app) {
	u := a.u
	compact := editorCompact(a)
	pad, button, step, headerStep, split, headerLimit := 16*u, 44*u, 48*u, 52*u, 16*u, 720*u
	if compact {
		pad, button, step, headerStep, split, headerLimit = 8*u, 40*u, 44*u, 44*u, 12*u, 520*u
	}
	v := a.safe().inset(pad)
	e.portrait = v.h > v.w
	gap := 8 * u
	e.p.view.viewport = v
	e.p.back = rect{v.x, v.y, button, button}
	toolbarY := v.y
	headerH := headerStep
	if e.portrait || v.w < headerLimit {
		e.field = rect{e.p.back.right() + gap, v.y, v.w - button - gap, button}
		toolbarY += headerStep
		headerH = 2 * headerStep
	}
	e.toolB = rect{v.right() - button, toolbarY, button, button}
	e.saveB = rect{e.toolB.x - split - button, toolbarY, button, button}
	e.testB = rect{e.saveB.x - 4*u - button, toolbarY, button, button}
	e.previewB = rect{e.testB.x - split - button, toolbarY, button, button}
	e.capacityB = rect{e.previewB.x - 4*u - button, toolbarY, button, button}
	if headerH == headerStep {
		e.field = rect{e.p.back.right() + gap, v.y, e.capacityB.x - e.p.back.right() - 2*gap, button}
	}
	body := rect{v.x, v.y + headerH, v.w, max(1, v.h-headerH)}
	e.setCaret()
	e.portrait = v.h > v.w
	e.wide = !e.portrait && v.w >= 880*u && v.h >= 460*u
	e.control = [7]rect{}
	e.tabs = [3]rect{}
	e.headings = [3]rect{}
	if e.portrait {
		e.toolbarScroll.horizontal = true
		actionWidth := 5*button + 8*u + 2*split
		total := actionWidth + split + 6*step + button
		e.toolbarScroll.set(rect{v.x, toolbarY, v.w, headerStep}, total)
		x := v.x - e.toolbarScroll.offset
		e.capacityB.x = x
		e.previewB.x = x + button + 4*u
		e.testB.x = e.previewB.right() + split
		e.saveB.x = e.testB.right() + 4*u
		e.toolB.x = e.saveB.right() + split
		for i := range e.control {
			e.control[i] = rect{e.toolB.right() + split + float64(i)*step, toolbarY, button, button}
		}
		dockHeight := 152 * u
		if compact {
			dockHeight = 136 * u
		}
		dockH := min(dockHeight, body.h*.38)
		e.dock = rect{v.x, v.bottom() - dockH, v.w, dockH}
		top := body.y
		e.canvas = rect{v.x, top, v.w, max(16*u, e.dock.y-top-gap)}
	} else {
		dw := 176 * u
		if compact {
			dw = 152 * u
		}
		if e.wide {
			dw = 196 * u
		}
		e.dock = rect{v.right() - dw, body.y, dw, body.h}
		work := rect{body.x, body.y, body.w - dw - gap, body.h}
		if e.wide {
			for i := range e.control {
				e.control[i] = rect{work.x, work.y + float64(i)*52*u, button, button}
			}
			e.canvas = rect{work.x + 52*u, work.y, work.w - 52*u, work.h}
		} else {
			cols := min(7, max(1, int((work.w+4*u)/step)))
			rows := (7 + cols - 1) / cols
			top := work.bottom() - float64(rows)*step + 4*u
			for i := range e.control {
				e.control[i] = rect{work.x + float64(i%cols)*step, top + float64(i/cols)*step, button, button}
			}
			e.canvas = rect{work.x, work.y, work.w, max(16*u, top-work.y-gap)}
		}
	}
	infoH := 40 * u
	if compact {
		infoH = 32 * u
	}
	e.info = rect{e.dock.x + 8*u, e.dock.bottom() - infoH - 8*u, e.dock.w - 16*u, infoH}
	e.channelB = rect{e.info.x, e.info.cy() - button/2, button, button}
	e.palette = e.palette[:0]
	e.paletteTools = e.paletteTools[:0]
	if e.portrait {
		tw := (e.dock.w - 16*u) / 3
		for i := range e.tabs {
			e.tabs[i] = rect{e.dock.x + 8*u + float64(i)*tw, e.dock.y + 4*u, tw, button}
		}
		paletteH := 52 * u
		if compact {
			paletteH = 52 * u
		}
		tools := editorGroups[e.category].tools
		bw := 72 * u
		e.dockScroll.horizontal = true
		e.dockScroll.set(rect{e.dock.x + 8*u, e.dock.y + step, e.dock.w - 16*u, paletteH}, float64(len(tools))*(bw+4*u)-4*u)
		x := e.dockScroll.viewport.x - e.dockScroll.offset
		for i, t := range tools {
			e.palette = append(e.palette, rect{x + float64(i)*(bw+4*u), e.dock.y + step, bw, paletteH})
			e.paletteTools = append(e.paletteTools, t)
		}
	} else {
		e.dockScroll.horizontal = false
		top := e.dock.y + 8*u
		rowH := button
		if e.wide {
			rowH = 36 * u
		}
		if !e.wide {
			e.tabs[e.category] = rect{e.dock.x + 8*u, top, e.dock.w - 16*u, button}
			top += headerStep
		}
		viewport := rect{e.dock.x + 8*u, top, e.dock.w - 16*u, max(1, e.info.y-top-8*u)}
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
		e.dockScroll.set(viewport, total)
		y := top - e.dockScroll.offset
		for g, c := range editorGroups {
			if !e.wide && g != e.category {
				continue
			}
			if e.wide {
				e.headings[g] = rect{viewport.x, y, viewport.w, 24 * u}
				y += 24 * u
			}
			for i, t := range c.tools {
				e.palette = append(e.palette, rect{viewport.x, y + float64(i)*rowH, viewport.w, rowH})
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
			before := e.dockScroll.offset
			e.dockScroll.reveal(r)
			if e.dockScroll.offset != before {
				e.layout(a)
			}

			break
		}
	}
	e.options = rect{v.cx() - min(400*u, v.w)/2, v.cy() - 94*u, min(400*u, v.w), 188 * u}
	for i := range e.dims {
		e.dims[i] = rect{e.options.x + 12*u + float64(i%2)*(e.options.w-12*u)/2, e.options.y + 36*u + float64(i/2)*44*u, (e.options.w - 36*u) / 2, 44 * u}
	}
	for i := range e.option {
		e.option[i] = rect{e.options.x + 12*u, e.options.y + 132*u + float64(i)*48*u, e.options.w - 24*u, 44 * u}
	}
	e.channelPanel = rect{v.cx() - min(304*u, v.w)/2, v.cy() - 122*u, min(304*u, v.w), 244 * u}
	cw := (e.channelPanel.w - 32*u) / 4
	for i := range e.channels {
		e.channels[i] = rect{e.channelPanel.x + 16*u + float64(i%4)*cw, e.channelPanel.y + 36*u + float64(i/4)*52*u, cw, 44 * u}
	}
	e.channelDone = rect{e.channelPanel.x + 12*u, e.channelPanel.bottom() - 48*u, e.channelPanel.w - 24*u, 44 * u}
	if e.tools && !e.portrait {
		for i := range e.tabs {
			e.tabs[i] = rect{e.dock.x + 8*u, e.dock.y + 8*u + float64(i)*48*u, e.dock.w - 16*u, 44 * u}
		}
	}
}

func editorControl(dst *ebiten.Image, a *app, r rect, label string, selected, enabled, primary bool) {
	fill, ink := a.sc.tile, a.sc.ink
	if primary {
		fill, ink = a.sc.ink, a.sc.ground
	}
	if !enabled {
		ink = a.sc.dim
	} else if a.over(r) {
		if primary {
			fill = a.sc.inkHi
		} else {
			fill = a.sc.tileHi
		}
	}
	fillRound(dst, r, 8*a.u, fill)
	if selected {
		strokeRound(dst, r, 8*a.u, 2*a.u, a.sc.ink)
	}
	if label != "" {
		face := a.f.labelBold
		if editorCompact(a) {
			face = a.f.smallBold
		}
		drawText(dst, label, face, r.cx(), r.cy(), text.AlignCenter, ink)
	}
}

func editorGlyph(dst *ebiten.Image, a *app, r rect, kind int, enabled bool) {
	u := a.u
	if editorCompact(a) {
		u *= .8
	}
	c := a.sc.ink
	if !enabled {
		c = a.sc.dim
	}
	x, y, k := r.cx(), r.cy(), 12*u
	w := 2 * u
	switch kind {
	case 0, 1:
		sign := 1.0
		if kind == 1 {
			sign = -1
		}
		line(dst, x+k*.7*sign, y+k*.45, x+k*.7*sign, y-k*.2, w, c)
		line(dst, x+k*.7*sign, y-k*.2, x-k*.7*sign, y-k*.2, w, c)
		line(dst, x-k*.7*sign, y-k*.2, x-k*.15*sign, y-k*.75, w, c)
		line(dst, x-k*.7*sign, y-k*.2, x-k*.15*sign, y+k*.35, w, c)
	case 2:
		line(dst, x-k*.75, y+k*.3, x+k*.15, y-k*.75, w, c)
		line(dst, x+k*.15, y-k*.75, x+k*.8, y-k*.05, w, c)
		line(dst, x+k*.8, y-k*.05, x-k*.1, y+k*.8, w, c)
		line(dst, x-k*.1, y+k*.8, x-k*.75, y+k*.3, w, c)
		line(dst, x-k*.38, y-k*.1, x+k*.3, y+k*.55, w, c)
	case 3:
		line(dst, x-k, y, x+k, y, w, c)
		line(dst, x, y-k, x, y+k, w, c)
		for _, s := range []float64{-1, 1} {
			line(dst, x+k*s, y, x+k*.6*s, y-k*.3, w, c)
			line(dst, x+k*s, y, x+k*.6*s, y+k*.3, w, c)
			line(dst, x, y+k*s, x-k*.3, y+k*.6*s, w, c)
			line(dst, x, y+k*s, x+k*.3, y+k*.6*s, w, c)
		}
	case 4, 6:
		line(dst, x-k*.65, y, x+k*.65, y, w, c)
		if kind == 6 {
			line(dst, x, y-k*.65, x, y+k*.65, w, c)
		}
	case 5:
		for _, s := range []float64{-1, 1} {
			for _, t := range []float64{-1, 1} {
				line(dst, x+k*s, y+k*t, x+k*.4*s, y+k*t, w, c)
				line(dst, x+k*s, y+k*t, x+k*s, y+k*.4*t, w, c)
			}
		}
	case 11:
		for col := range 2 {
			for row := range 2 {
				box := rect{x - k + float64(col)*13*u, y - k + float64(row)*13*u, 10 * u, 10 * u}
				strokeRound(dst, box, 2*u, w, c)
				fillCircle(dst, box.cx(), box.cy(), u, c)
			}
		}
	case 12:
		strokeRound(dst, rect{x - k, y - k*.65, 2 * k, 1.3 * k}, 4*u, w, c)
		fillRound(dst, rect{x + 2*u, y - 5*u, 7 * u, 10 * u}, 2*u, c)
	case 8:
		line(dst, x-k*.5, y-k*.8, x+k*.8, y, w, c)
		line(dst, x+k*.8, y, x-k*.5, y+k*.8, w, c)
		line(dst, x-k*.5, y+k*.8, x-k*.5, y-k*.8, w, c)
	case 7:
		for _, dx := range []float64{-.6, 0, .6} {
			fillCircle(dst, x+k*dx, y, 1.7*u, c)
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
	state := mapArtState{Map: &e.m}
	if e.preview {
		if cap(e.previewState) < len(e.m.Cells) {
			e.previewState = make([]uint8, len(e.m.Cells))
		} else {
			e.previewState = e.previewState[:len(e.m.Cells)]
			clear(e.previewState)
		}
		for i, c := range e.m.Cells {
			if c.Feature.IsSwitch() {
				e.previewState[i] = 1
				state.OpenGates[c.Group] = true
			}
		}
		state.Pressed = e.previewState
	}
	drawMapArt(e.mapImage, a.sc, state, mapArtTransform{Cell: cell}, true, a.set.Marks)
	face := newFace(srcUI, cell*.24, 700)
	var opened [core.MaxSize * core.MaxSize]uint8
	if e.capacities && e.preview {
		for _, edge := range e.m.Edges {
			if edge.Kind == core.Gate && state.OpenGates[edge.Group] {
				opened[edge.A]++
				opened[edge.B]++
			}
		}
	}
	for i, c := range e.m.Cells {
		x, y := float64(i/e.m.Rows)*cell, float64(i%e.m.Rows)*cell
		r := rect{x + 1, y + 1, cell - 2, cell - 2}
		if e.capacities && c.Terrain == core.Floor {
			cap := int(e.capacity[i]) + int(opened[i])
			cap = min(255, cap)
			if c.Feature == core.NoFeature {
				drawText(e.mapImage, orbCountLabels[cap], face, x+cell*.5, y+cell*.5, text.AlignCenter, a.sc.ink)
			} else {
				badge := rect{x + cell*.72, y + cell*.04, cell * .24, cell * .25}
				fillRound(e.mapImage, badge, cell*.05, a.sc.ground)
				drawText(e.mapImage, orbCountLabels[cap], face, badge.cx(), badge.cy(), text.AlignCenter, a.sc.ink)
			}
		}
		if e.bad[i] {
			strokeRound(e.mapImage, r, cell*.12, max(1, cell*.04), a.sc.danger)
		}
	}
	e.mapCell, e.mapRevision, e.mapTheme = cell, e.revision, a.sc
	e.mapMarks, e.mapCapacity, e.mapPreview = a.set.Marks, e.capacities, e.preview
}

func (e *mapEditor) renderIcons(a *app) {
	if e.icons != nil && e.iconTheme == a.sc && e.iconMarks == a.set.Marks && e.iconGroup == e.group && e.iconPreview == e.preview {
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
		drawMapBrush(e.icons, a.sc, b, rect{float64(i)*64 + 4, 4, 56, 56}, e.group, e.preview, a.set.Marks)
	}
	// A circuit swatch is a relationship control, separate from component art.
	cx := float64(brushErase*64 + 32)
	fillRound(e.icons, rect{cx - 15, 17, 30, 30}, 5, partnerColor(a.sc, e.group))

	e.iconTheme, e.iconMarks, e.iconGroup, e.iconPreview = a.sc, a.set.Marks, e.group, e.preview
}
func (e *mapEditor) brushIcon(dst *ebiten.Image, tool int, r rect, alpha float32) {
	op := ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Scale(r.w/64, r.h/64)
	op.GeoM.Translate(r.x, r.y)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(e.brushIcons[tool], &op)
}

func (e *mapEditor) draw(dst *ebiten.Image, a *app) {
	e.layout(a)
	e.renderIcons(a)
	e.renderMap(a)
	u, sc := a.u, a.sc
	drawBack(dst, a, e.p.back)
	editorControl(dst, a, e.field, "", e.name.focused, true, false)
	title := string(e.name.shown)
	if title == "" {
		title = "Map name"
	}
	available := max(1, e.field.w-24*u)
	runes := []rune(title)
	for len(runes) > 0 && textWidth(string(runes), a.f.labelBold) > available {
		if e.name.focused {
			runes = runes[1:]
		} else {
			runes = runes[:len(runes)-1]
		}
	}
	title = string(runes)
	drawText(dst, title, a.f.labelBold, e.field.x+12*u, e.field.cy(), text.AlignStart, sc.ink)
	if e.name.focused {
		x := e.field.x + 12*u + textWidth(title, a.f.labelBold) + 2*u
		line(dst, x, e.field.cy()-8*u, x, e.field.cy()+8*u, 2*u, sc.ink)
	}
	toolbarDst := dst
	if e.portrait {
		toolbarDst = e.toolbarScroll.clip(dst)
	}
	editorControl(toolbarDst, a, e.capacityB, "", e.capacities, true, false)
	editorGlyph(toolbarDst, a, e.capacityB, 11, true)
	editorControl(toolbarDst, a, e.previewB, "", e.preview, true, false)
	editorGlyph(toolbarDst, a, e.previewB, 12, true)
	split := 8 * u
	if editorCompact(a) {
		split = 6 * u
	}
	for _, x := range []float64{e.testB.x - split, e.toolB.x - split} {
		line(toolbarDst, x, e.testB.y+12*u, x, e.testB.bottom()-12*u, u, sc.dim)
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
		e.toolbarScroll.drawScrollbar(dst, a)
	}
	fillRound(dst, e.canvas, 10*u, sc.ground)
	if e.canvasOf != dst || e.canvasBounds != e.canvas {
		e.canvasOf, e.canvasBounds = dst, e.canvas
		e.canvasClip = dst.SubImage(image.Rect(int(e.canvas.x), int(e.canvas.y), int(e.canvas.right()), int(e.canvas.bottom()))).(*ebiten.Image)
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
					line(clip, x0, y0, x1, y1, max(1, 1.5*u), withAlpha(partnerColor(sc, c.Group), .55))
				}
			}
		}
	}
	if e.hover >= 0 {
		i := e.hover
		x, y := ox+float64(i/e.m.Rows)*k, oy+float64(i%e.m.Rows)*k
		r := rect{x, y, k, k}
		if highlight, ok := e.edgeHighlight(a, i, a.in.x, a.in.y); ok {
			strokeRound(clip, highlight, max(3*u, k*.1), 2*u, sc.ink)
		} else if e.tool != brushPan && e.tool != brushBarrier && e.tool != brushGate &&
			(e.tool != brushErase || e.m.Cells[i].Terrain != core.Floor || e.m.Cells[i].Feature != core.NoFeature) {
			if !e.stroke && e.tool != brushErase {
				e.brushIcon(clip, e.tool, r, .45)
			}
			strokeRound(clip, r.inset(1), k*.12, max(1, 2*u), sc.ink)
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
	fillRound(dst, e.dock, 10*u, sc.tile)
	if e.portrait {
		for i, r := range e.tabs {
			editorControl(dst, a, r, editorGroups[i].name, i == e.category, true, false)
		}
	} else if !e.wide {
		editorControl(dst, a, e.tabs[e.category], editorGroups[e.category].name, true, true, false)
	}
	body := e.dockScroll.clip(dst)
	if e.wide {
		for i, r := range e.headings {
			if r.h > 0 {
				drawText(body, editorGroups[i].name, a.f.small, r.x+4*u, r.cy(), text.AlignStart, sc.dim)
			}
		}
	}
	for i, r := range e.palette {
		if r.bottom() < e.dockScroll.viewport.y || r.y > e.dockScroll.viewport.bottom() {
			continue
		}
		tool := e.paletteTools[i]
		selected := tool == e.tool
		editorControl(body, a, r, "", selected, true, false)
		size := 32 * u
		if editorCompact(a) {
			size = 24 * u
		}
		ir := rect{r.x + 2*u, r.cy() - size/2, size, size}
		if e.portrait {
			size = 28 * u
			if editorCompact(a) {
				size = 24 * u
			}
			ir = rect{r.cx() - size/2, r.y, size, size}
		}
		e.brushIcon(body, tool, ir, 1)
		if tool == brushVoid {
			strokeRound(body, ir.inset(7*u), 2*u, 1*u, sc.dim)
		}
		if e.portrait {
			labelY, lineY := 38*u, 34*u
			if editorCompact(a) {
				labelY, lineY = 34*u, 30*u
			}
			if tool == brushBreak {
				drawText(body, "Breakable", a.f.small, r.cx(), r.y+lineY, text.AlignCenter, sc.ink)
				drawText(body, "wall", a.f.small, r.cx(), r.y+lineY+11*u, text.AlignCenter, sc.ink)
			} else {
				drawText(body, editorTools[tool], a.f.small, r.cx(), r.y+labelY, text.AlignCenter, sc.ink)
			}
		} else {
			drawText(body, editorTools[tool], a.f.small, ir.right()+6*u, r.cy(), text.AlignStart, sc.ink)
		}
	}
	e.dockScroll.drawScrollbar(dst, a)
	fillRound(dst, e.info, 8*u, sc.tile)
	x := e.info.x + 4*u
	if e.channelTool() {
		editorControl(dst, a, e.channelB, "", e.channelsOpen, true, false)
		e.brushIcon(dst, brushErase, e.channelB, 1)
		x = e.channelB.right() + 4*u
	}
	width := max(1, e.info.right()-x-4*u)
	if e.helpTool != e.tool || e.helpWidth != width {
		e.helpTool, e.helpWidth = e.tool, width
		e.helpLines = wrap(editorHelp[e.tool], a.f.small, width)
	}
	maxLines := 2
	if !e.portrait {
		maxLines = 3
	}
	for i, l := range e.helpLines {
		if i >= maxLines {
			break
		}
		drawText(dst, l, a.f.small, x, e.info.cy()+(float64(i)-float64(min(maxLines, len(e.helpLines))-1)/2)*13*u, text.AlignStart, sc.dim)
	}
	if e.tools && !e.portrait {
		fillRound(dst, rect{e.dock.x, e.dock.y, e.dock.w, 156 * u}, 10*u, sc.ground)
		for i, r := range e.tabs {
			editorControl(dst, a, r, editorGroups[i].name, i == e.category, true, false)
		}
	}
}

func (e *mapEditor) drawOptions(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	fillRound(dst, a.safe(), 0, withAlpha(sc.ground, .8))
	fillRound(dst, e.options, 16*u, sc.tile)
	drawText(dst, fmt.Sprintf("Map size  %d × %d", e.m.Cols, e.m.Rows), a.f.labelBold, e.options.x+16*u, e.options.y+18*u, text.AlignStart, sc.ink)
	for i, label := range []string{"Columns −", "Columns +", "Rows −", "Rows +"} {
		editorControl(dst, a, e.dims[i], label, false, true, false)
	}
	editorControl(dst, a, e.option[0], "Done", false, true, true)
}
func (e *mapEditor) drawChannels(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	fillRound(dst, a.safe(), 0, withAlpha(sc.ground, .8))
	fillRound(dst, e.channelPanel, 16*u, sc.tile)
	label := "Circuit color"
	if e.tool == brushLink {
		label = "Pair color"
	}
	drawText(dst, label, a.f.labelBold, e.channelPanel.x+16*u, e.channelPanel.y+18*u, text.AlignStart, sc.ink)
	for i, r := range e.channels {
		g := i + 1
		editorControl(dst, a, r, "", uint8(g) == e.group, true, false)
		fillRound(dst, rect{r.cx() - 15*u, r.cy() - 15*u, 30 * u, 30 * u}, 0, partnerColor(sc, uint8(g)))

	}
	editorControl(dst, a, e.channelDone, "Done", false, true, true)
}

func editorActionSave(dst *ebiten.Image, a *app, r rect) {
	u, c := a.u, a.sc.ground
	if editorCompact(a) {
		u *= .8
	}
	x, y := r.cx(), r.cy()
	strokeRound(dst, rect{x - 10*u, y - 10*u, 20 * u, 20 * u}, 2*u, 2*u, c)
	strokeRound(dst, rect{x - 5*u, y - 10*u, 10 * u, 7 * u}, u, 2*u, c)
	strokeRound(dst, rect{x - 6*u, y + 2*u, 12 * u, 8 * u}, u, 2*u, c)
}

// Placement and erase share the same boundary geometry and outline.
func (e *mapEditor) edgeHighlight(a *app, i int, x, y float64) (rect, bool) {
	var edge core.MapEdge
	switch e.tool {
	case brushErase:
		index := e.eraseEdgeAt(i, x, y)
		if index < 0 {
			return rect{}, false
		}
		edge = e.m.Edges[index]
	case brushBarrier, brushGate:
		aa, bb := e.edgeAt(i, x, y)
		if aa < 0 {
			return rect{}, false
		}
		edge = core.MapEdge{A: aa, B: bb}
	default:
		return rect{}, false
	}
	x0, y0, x1, y1, ok := mapArtEdge(&e.m, edge)
	if !ok {
		return rect{}, false
	}
	k, ox, oy := e.grid()
	x0, y0, x1, y1 = ox+x0*k, oy+y0*k, ox+x1*k, oy+y1*k
	pad := max(3*a.u, k*.1)
	return rect{min(x0, x1) - pad, min(y0, y1) - pad, math.Abs(x1-x0) + 2*pad, math.Abs(y1-y0) + 2*pad}, true
}
