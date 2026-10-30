package client

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/core"
)

// Board presets, as columns × rows.
var presets = [...]struct {
	name, dims string
	cols, rows int
}{
	{"Small", "9 × 5", 9, 5},
	{"Compact", "14 × 8", 14, 8},
	{"Medium", "17 × 10", 17, 10},
	{"Broad", "21 × 12", 21, 12},
	{"Large", "25 × 15", 25, 15},
	{"Giant", "30 × 18", 30, 18},
}

// The board preview frame spans at least the Giant preset; bigger boards shrink to fit it.
const previewCols, previewRows = 30, 18

// previewScale is the preview's px per cell for the chosen board.
func (s *setupScene) previewScale(a *app) float64 {
	return min(s.preview.w/float64(max(a.set.Cols, previewCols)), s.preview.h/float64(max(a.set.Rows, previewRows)))
}

// setupScene keeps Players above Board and Start outside the scrollable content.
type setupScene struct {
	menu int // seat whose type menu is open, or -1

	// Layout in device px, recomputed when the screen size changes.
	lw, lh, lu, lscroll      float64
	lp                       int
	viewport                 rect
	scroll, maxScroll        float64
	blockPad                 float64
	back                     rect
	titleX, titleY           float64
	players, boardB          rect
	headY                    float64
	minusP, plusP            rect
	seatDot, seatTag, seatTy [core.MaxPlayers]rect
	seatHit                  [core.MaxPlayers]rect
	preview, previewPanel    rect
	showPreview              bool
	presetPills              [len(presets)]rect
	colsCtl, rowsCtl         rect
	minusCol, plusCol        rect
	minusRow, plusRow        rect
	start                    rect

	// The mini board drawn in the preview, cached until the size, mode or preview changes.
	mini                *ebiten.Image
	miniC, miniR, miniS int
	miniSc              *scheme
	dims                string
	dimC, dimR          int
}

func newSetup() *setupScene { return &setupScene{menu: -1} }

func (s *setupScene) layout(a *app) {
	if a.w == s.lw && a.h == s.lh && a.u == s.lu && a.set.Players == s.lp && s.scroll == s.lscroll {
		return
	}
	s.lw, s.lh, s.lu, s.lp = a.w, a.h, a.u, a.set.Players
	u := a.u
	pad, gap, bp := 20*u, 16*u, 20*u
	if a.w < 1200*u {
		pad, gap, bp = 16*u, 12*u, 16*u
	}
	s.blockPad = bp
	w := a.w - 2*pad
	s.back = backButton(pad, pad, w, u)
	s.titleX, s.titleY = s.back.right()+16*u, s.back.cy()
	s.start = rect{pad, a.h - pad - 52*u, w, 52 * u}
	top := s.back.bottom() + gap
	s.viewport = rect{pad, top, w, max(1, s.start.y-gap-top)}

	// Visible controls match Back; their larger hit rows preserve touch usability.
	buttonH, targetH := compactButtonHeight*u, compactButtonTarget*u
	rowPitch := targetH + 8*u
	vw := 0.0
	for _, number := range numbers {
		vw = max(vw, textWidth(number, a.f.number))
	}
	vw += 12 * u
	inner := w - 2*bp
	// Presets and dimensions form one vertical group beside or below the preview.
	dimensionMin := max(textWidth("Columns", a.f.labelBold), textWidth("Rows", a.f.labelBold)) + 8*u + 2*targetH + vw
	controlsMin := max(2*dimensionMin+gap, 288*u)
	sideBySide := inner >= 200*u+gap+controlsMin
	controlsW := inner
	if sideBySide {
		controlsW = max(controlsMin, min(420*u, inner*0.42))
	}
	stackDimensions := a.w < 600*u || controlsW < 2*dimensionMin+gap
	dimensionsH := targetH
	if stackDimensions {
		dimensionsH = 2*targetH + gap
	}
	tagW := max(30*u, textWidth("P12", a.f.labelBold))
	choiceInset := 44*u + tagW + 8*u
	minSeatW := 220 * u
	for _, label := range seatLabels {
		minSeatW = max(minSeatW, choiceInset+textWidth(label, a.f.labelBold)+40*u)
	}
	cols := max(1, min(4, a.set.Players, int((inner+gap)/(minSeatW+gap))))
	rows := (a.set.Players + cols - 1) / cols
	playerH := 2*bp + 44*u + float64(rows)*rowPitch
	presetH := 3*targetH + 16*u
	controlsH := presetH + gap + dimensionsH
	boardMin := 2*bp + 32*u + controlsH
	if !sideBySide {
		boardMin += 128*u + gap // preview above the controls
	}
	s.maxScroll = max(0, boardMin+gap+playerH-s.viewport.h)
	s.scroll = max(0, min(s.maxScroll, s.scroll))
	s.lscroll = s.scroll
	boardH := max(boardMin, s.viewport.h-gap-playerH)
	s.players = rect{pad, top - s.scroll, w, playerH}
	s.boardB = rect{pad, s.players.bottom() + gap, w, boardH}
	p := s.players
	s.headY = p.y + bp + 22*u
	colW := (inner - float64(cols-1)*gap) / float64(cols)
	clear(s.seatHit[:])
	clear(s.seatTy[:])
	for i := range a.set.Players {
		x := p.x + bp + float64(i%cols)*(colW+gap)
		cy := p.y + bp + 52*u + float64(i/cols)*rowPitch + targetH/2
		r := rect{x, cy - buttonH/2, colW, buttonH}
		s.seatTy[i], s.seatHit[i] = r, compactHit(r, u)
		s.seatDot[i] = rect{x + 16*u, cy - 8*u, 16 * u, 16 * u}
		s.seatTag[i] = rect{x + 44*u, r.y, tagW, r.h}
	}

	// Size controls reflow as groups; their labels and hit areas never shrink with the viewport.
	step := func(right, cy float64) (minus, plus rect) {
		plus = rect{right - 42*u, cy - buttonH/2, buttonH, buttonH}
		return rect{plus.x - vw - targetH, plus.y, buttonH, buttonH}, plus
	}
	s.minusP, s.plusP = step(p.right()-bp, s.headY)
	b := s.boardB
	body := rect{b.x + bp, b.y + bp + 32*u, inner, b.h - 2*bp - 32*u}
	controls := rect{body.right() - controlsW, body.cy() - controlsH/2, controlsW, controlsH}
	if sideBySide {
		s.previewPanel = rect{body.x, body.y, controls.x - gap - body.x, body.h}
	} else {
		s.previewPanel = rect{body.x, body.y, body.w, max(128*u, body.h-controlsH-gap)}
		controls.x, controls.y = body.x, s.previewPanel.bottom()+gap
	}
	presetArea := rect{controls.x, controls.y, controls.w, presetH}
	cw := (controls.w - gap) / 2
	s.colsCtl = rect{controls.x, presetArea.bottom() + gap, cw, targetH}
	s.rowsCtl = rect{controls.x + cw + gap, s.colsCtl.y, cw, targetH}
	if stackDimensions {
		s.colsCtl.w = controls.w
		s.rowsCtl = rect{controls.x, s.colsCtl.bottom() + gap, controls.w, targetH}
	}
	s.minusCol, s.plusCol = step(s.colsCtl.right(), s.colsCtl.cy())
	s.minusRow, s.plusRow = step(s.rowsCtl.right(), s.rowsCtl.cy())
	pw := (presetArea.w - 8*u) / 2
	for i := range s.presetPills {
		s.presetPills[i] = rect{presetArea.x + float64(i%2)*(pw+8*u), presetArea.y + float64(i/2)*rowPitch + (targetH-buttonH)/2, pw, buttonH}
	}
	s.showPreview = true
	s.preview = s.previewPanel.inset(12 * u)
}

func setupStepHit(r rect, u float64) rect {
	return r.inset(-max(0, (compactButtonTarget*u-r.h)/2))
}

func (s *setupScene) canStart(a *app) bool { return a.set.Players <= a.set.Cols*a.set.Rows }

func (s *setupScene) setPlayers(a *app, n int) {
	a.set.Players = max(core.MinPlayers, min(core.MaxPlayers, n))
	a.set.fitSeats()
	s.menu = -1
	a.set.save()
}

func (s *setupScene) resize(a *app, dc, dr int) {
	a.set.Cols = max(core.MinSize, min(core.MaxSize, a.set.Cols+dc))
	a.set.Rows = max(core.MinSize, min(core.MaxSize, a.set.Rows+dr))
	a.set.save()
}

func (s *setupScene) begin(a *app) {
	if !s.canStart(a) {
		return
	}
	if p, err := newPlay(a, a.set.Cols, a.set.Rows, a.set.seats); err == nil {
		a.next = p
	}
}

// menuRects keeps options touch-sized, using two columns when the window is short.
func (s *setupScene) menuRects(a *app) (panel, first rect) {
	u := a.u
	t := s.seatTy[s.menu]
	cols := 1
	if float64(len(seatLabels))*compactButtonTarget*u+12*u > a.h-16*u {
		cols = 2
	}
	rows := (len(seatLabels) + cols - 1) / cols
	w := min(a.w-16*u, max(t.w, 200*u)*float64(cols))
	h := float64(rows)*compactButtonTarget*u + 12*u
	y := t.bottom() + 6*u
	if y+h > a.h-8*u {
		y = t.y - 6*u - h
	}
	panel = rect{max(8*u, min(t.x, a.w-8*u-w)), max(8*u, min(y, a.h-8*u-h)), w, h}
	return panel, rect{panel.x + 6*u, panel.y + 6*u, (w - 12*u) / float64(cols), compactButtonTarget * u}
}

func setupMenuOption(panel, first rect, i int, u float64) rect {
	rows := int(math.Round((panel.h - 12*u) / first.h))
	return rect{first.x + float64(i/rows)*first.w, first.y + float64(i%rows)*first.h, first.w, first.h}
}

func (s *setupScene) update(a *app) error {
	s.layout(a)
	u, set := a.u, a.set
	if s.maxScroll > 0 && a.in.wheel != 0 && s.viewport.contains(a.in.x, a.in.y) {
		s.scroll -= a.in.wheel * (compactButtonTarget + 8) * u
		s.menu = -1
		s.layout(a)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if s.menu >= 0 {
			s.menu = -1
		} else {
			a.next = newHome()
		}
		return nil
	}

	// An open menu sits on top: it takes clicks inside it, and a click anywhere else closes it.
	closing := -1
	if s.menu >= 0 {
		panel, first := s.menuRects(a)
		if a.hover(panel) {
			for o := range seatLabels {
				opt := setupMenuOption(panel, first, o, u)
				if a.clicked(opt) {
					set.seats[s.menu] = o
					s.menu = -1
					set.save()
					break
				}
			}
			return nil
		}
		if a.in.clicked {
			closing, s.menu = s.menu, -1
		}
	}

	switch {
	case a.clicked(compactHit(s.back, u)):
		a.next = newHome()
		return nil
	case s.canStart(a) && a.clicked(s.start):
		s.begin(a)
		return nil
	}
	if !s.viewport.contains(a.in.x, a.in.y) {
		return nil
	}
	switch {
	case set.Players > core.MinPlayers && a.clicked(setupStepHit(s.minusP, u)):
		s.setPlayers(a, set.Players-1)
	case set.Players < core.MaxPlayers && a.clicked(setupStepHit(s.plusP, u)):
		s.setPlayers(a, set.Players+1)
	case set.Cols > core.MinSize && a.clicked(setupStepHit(s.minusCol, u)):
		s.resize(a, -1, 0)
	case set.Cols < core.MaxSize && a.clicked(setupStepHit(s.plusCol, u)):
		s.resize(a, 1, 0)
	case set.Rows > core.MinSize && a.clicked(setupStepHit(s.minusRow, u)):
		s.resize(a, 0, -1)
	case set.Rows < core.MaxSize && a.clicked(setupStepHit(s.plusRow, u)):
		s.resize(a, 0, 1)
	}
	// Count changes also move the controls. Do not reuse the old rectangles for this click.
	if set.Players != s.lp {
		s.layout(a)
		return nil
	}
	for i := range set.Players {
		switch {
		case !a.clicked(s.seatHit[i]):
		case closing != i:
			s.menu = i
		}
	}
	for i, r := range s.presetPills {
		if a.clicked(compactHit(r, u)) {
			set.Cols, set.Rows = presets[i].cols, presets[i].rows
			set.save()
		}
	}
	return nil
}

func (s *setupScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u

	drawBack(dst, a, s.back)
	titleFace := f.title
	if textWidth("Set up the game", titleFace) > a.w-s.titleX-s.back.x {
		titleFace = f.heading
	}
	drawText(dst, "Set up the game", titleFace, s.titleX, s.titleY, text.AlignStart, sc.ink)

	v := s.viewport
	content := dst.SubImage(image.Rect(int(v.x), int(v.y), int(v.right()), int(v.bottom()))).(*ebiten.Image)
	s.drawPlayers(content, a)
	s.drawBoard(content, a)
	if s.maxScroll > 0 {
		h := max(24*u, v.h*v.h/(v.h+s.maxScroll))
		fillRound(dst, rect{v.right() + 5*u, v.y + (v.h-h)*s.scroll/s.maxScroll, 3 * u, h}, 1.5*u, sc.dim)
	}
	s.drawStart(dst, a)
	if s.menu >= 0 {
		s.drawMenu(dst, a)
	}
}

// stepper draws − value + with the buttons on the block's ground.
func stepper(dst *ebiten.Image, a *app, v, lo, hi int, minus, plus rect) {
	sc, u := a.sc, a.u
	for i, r := range [2]rect{minus, plus} {
		enabled := i == 0 && v > lo || i == 1 && v < hi
		ink := drawButtonSurface(dst, a, r, btnGround, enabled, a.over(setupStepHit(r, u)))
		line(dst, r.cx()-6*u, r.cy(), r.cx()+6*u, r.cy(), 2.5*u, ink)
		if i == 1 {
			line(dst, r.cx(), r.cy()-6*u, r.cx(), r.cy()+6*u, 2.5*u, ink)
		}
	}
	drawText(dst, numbers[v], a.f.number, (minus.right()+plus.x)/2, minus.cy(), text.AlignCenter, sc.ink)
}

func (s *setupScene) drawPlayers(dst *ebiten.Image, a *app) {
	sc, f, u, set := a.sc, &a.f, a.u, a.set
	p := s.players
	fillRound(dst, p, 24*u, sc.tile)
	drawText(dst, "Players", f.heading, p.x+s.blockPad, s.headY, text.AlignStart, sc.ink)
	stepper(dst, a, set.Players, core.MinPlayers, core.MaxPlayers, s.minusP, s.plusP)

	for i := range set.Players {
		d, tag, t := s.seatDot[i], s.seatTag[i], s.seatTy[i]
		hot := s.menu < 0 && a.over(s.seatHit[i]) || s.menu == i
		style := btnGround
		if set.seats[i] == seatHuman || s.menu == i {
			style = btnChosen
		}
		ink := drawButtonSurface(dst, a, t, style, true, hot)
		seatDot(dst, sc, i, d.cx(), d.cy(), d.w/2, false, set.Marks)
		tagInk := sc.dim
		if hot {
			tagInk = ink
		}
		drawText(dst, seatNames[i], f.labelBold, tag.x, tag.cy(), text.AlignStart, tagInk)
		drawText(dst, seatLabels[set.seats[i]], f.labelBold, tag.right()+8*u, t.cy(), text.AlignStart, ink)
		chevron(dst, t.right()-20*u, t.cy(), 5*u, 2*u, s.menu == i, ink)
	}
}

// drawStart draws the Start button; when the board is too small for the players it says so instead.
func (s *setupScene) drawStart(dst *ebiten.Image, a *app) {
	label, ok := "Start", s.canStart(a)
	if !ok {
		label = "Board too small"
	}
	drawButton(dst, a, s.start, label, "", btnInk, ok)
}

func (s *setupScene) drawBoard(dst *ebiten.Image, a *app) {
	sc, u, set := a.sc, a.u, a.set
	b := s.boardB
	fillRound(dst, b, 24*u, sc.tile)
	y := b.y + s.blockPad + 14*u
	drawText(dst, "Board", a.f.heading, b.x+s.blockPad, y, text.AlignStart, sc.ink)
	if s.dimC != set.Cols || s.dimR != set.Rows {
		s.dimC, s.dimR, s.dims = set.Cols, set.Rows, numbers[set.Cols]+" × "+numbers[set.Rows]
	}
	drawText(dst, s.dims, a.f.number, b.right()-s.blockPad, y, text.AlignEnd, sc.ink)

	// The chosen board as a mini mat, with every preset as a dashed outline from the same corner: graded sizes,
	// like the nested lines of a sewing pattern.
	if s.showPreview {
		fillRound(dst, s.previewPanel, 16*u, sc.ground)
		// Centre the diagram (the chosen board and the largest preset) in its frame. Preset outlines too small to
		// read at this scale are left out.
		pv := s.preview
		k := s.previewScale(a)
		ox := math.Round(pv.x + (pv.w-float64(max(set.Cols, previewCols))*k)/2)
		oy := math.Round(pv.y + (pv.h-float64(max(set.Rows, previewRows))*k)/2)
		s.drawMini(dst, a, k, ox, oy)
		for _, p := range presets {
			// At small scales an outline inside the chosen board only adds scribble, so it is dropped.
			inside := p.cols <= set.Cols && p.rows <= set.Rows
			if float64(min(p.cols, p.rows))*k >= 12*u && !(inside && k < 6*u) {
				dashedRect(dst, rect{ox, oy, float64(p.cols) * k, float64(p.rows) * k}, 4*u, 1.5*u, sc.dim)
			}
		}
		strokeRound(dst, rect{ox - 3*u, oy - 3*u, float64(set.Cols)*k + 6*u, float64(set.Rows)*k + 6*u}, 6*u,
			2.5*u, sc.ink)
	}

	for i, r := range s.presetPills {
		p := presets[i]
		style := btnGround
		if set.Cols == p.cols && set.Rows == p.rows {
			style = btnChosen
		}
		drawButton(dst, a, r, p.name, p.dims, style, true)
	}
	drawText(dst, "Columns", a.f.labelBold, s.colsCtl.x, s.colsCtl.cy(), text.AlignStart, sc.ink)
	stepper(dst, a, set.Cols, core.MinSize, core.MaxSize, s.minusCol, s.plusCol)
	drawText(dst, "Rows", a.f.labelBold, s.rowsCtl.x, s.rowsCtl.cy(), text.AlignStart, sc.ink)
	stepper(dst, a, set.Rows, core.MinSize, core.MaxSize, s.minusRow, s.plusRow)
}

// drawMini draws the chosen board's tiles at k px per cell with their top left at (ox, oy), cached in an image.
func (s *setupScene) drawMini(dst *ebiten.Image, a *app, k, ox, oy float64) {
	set, sc := a.set, a.sc
	w, h := int(math.Ceil(s.preview.w)), int(math.Ceil(s.preview.h))
	size := w<<16 | h // cache key: both sides
	if s.mini == nil || s.miniC != set.Cols || s.miniR != set.Rows || s.miniS != size || s.miniSc != sc {
		if s.mini == nil || s.miniS != size {
			if s.mini != nil {
				s.mini.Deallocate()
			}
			s.mini = ebiten.NewImage(w, h)
		}
		s.miniC, s.miniR, s.miniS, s.miniSc = set.Cols, set.Rows, size, sc
		s.mini.Clear()
		g := k * tileGutter
		scratchPath.Reset()
		for c := range set.Cols {
			for r := range set.Rows {
				addRound(&scratchPath, rect{float64(c)*k + g, float64(r)*k + g, k - 2*g, k - 2*g}, k*tileRadius)
			}
		}
		fillPath(s.mini, &scratchPath, sc.tile)
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(ox, oy)
	dst.DrawImage(s.mini, &op)
}

func (s *setupScene) drawMenu(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	panel, first := s.menuRects(a)
	fillRound(dst, panel, 18*u, sc.ground)
	strokeRound(dst, panel, 18*u, 2*u, sc.ink)
	current := a.set.seats[s.menu]
	for o, label := range seatLabels {
		opt := setupMenuOption(panel, first, o, a.u)
		style := btnGround
		if o == current {
			style = btnInk
		}
		face := rect{opt.x, opt.cy() - compactButtonHeight*a.u/2, opt.w, compactButtonHeight * a.u}
		ink := drawButtonSurface(dst, a, face, style, true, a.over(opt))
		drawText(dst, label, f.labelBold, opt.x+14*u, opt.cy(), text.AlignStart, ink)
	}
}
