package client

import (
	"image"
	"image/color"
	"log"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// Board shapes, in the order their preset groups appear.
const (
	shapePortrait = iota
	shapeBalanced
	shapeLandscape
)

var shapeNames = [...]string{"Portrait", "Balanced", "Landscape"}

// seatChoiceX is where a seat pill's choice starts, in logical px from its left edge: after its 16 px dot.
const seatChoiceX = 44

// An AI's strength shows as a ladder of five pips, pipR in radius and pipStep apart (logical px).
const (
	pips    = 5
	pipR    = 2.5
	pipStep = 9
	pipsW   = (pips-1)*pipStep + 2*pipR
)

// seatPips is how many pips seat option opt fills: the AIs in their easiest-to-hardest order, in five even steps.
// Human shows none.
func seatPips(opt int) int {
	if opt == seatHuman {
		return 0
	}
	return 1 + (opt-1)*pips/(len(seatLabels)-1)
}

// drawPips draws opt's ladder ending at right, filled pips in ink and the rest faint.
func drawPips(dst *ebiten.Image, opt int, right, cy, u float64, ink color.RGBA) {
	n := seatPips(opt)
	if n == 0 {
		return
	}
	for i := range pips {
		c := ink
		if i >= n {
			c = ui.WithAlpha(ink, 0.25)
		}
		ui.FillCircle(dst, right-(pipsW-pipR)*u+float64(i)*pipStep*u, cy, pipR*u, c)
	}
}

// boardSizes are the preset tiers, smallest first, as short × long sides; every shape offers every tier (Balanced
// uses the short side twice). Floor is the largest board whose cells stay at least 24 px, the prototype's smallest
// cell and WCAG's smallest touch target, on a 360×780 phone: 13 across its 328 px wide board, 18 down the ~430 px the
// seat bar leaves it. A landscape board turned upright in a phone browser has the same room. Short sides grow about
// 1.2× a tier (about 1.45× the cells); long sides are about 1.4× the short, and 6 × 9 is the classic board.
var boardSizes = [...]struct {
	name        string
	short, long int
}{
	{"Coaster", 5, 7},
	{"Placemat", 6, 9},
	{"Doormat", 7, 10},
	{"Rug", 9, 13},
	{"Carpet", 11, 15},
	{"Floor", 13, 18},
}

// presetSize is tier t's board in shape g, as columns × rows.
func presetSize(g, t int) (cols, rows int) {
	z := boardSizes[t]
	switch g {
	case shapePortrait:
		return z.short, z.long
	case shapeBalanced:
		return z.short, z.short
	}
	return z.long, z.short
}

// presetDims are the presets' "cols × rows" notes, by shape and tier.
var presetDims = func() (d [len(shapeNames)][len(boardSizes)]string) {
	for g := range d {
		for t := range d[g] {
			c, r := presetSize(g, t)
			d[g][t] = ui.Numbers[c] + " × " + ui.Numbers[r]
		}
	}
	return d
}()

// shapeOf is the shape of a cols × rows board.
func shapeOf(cols, rows int) int {
	switch {
	case cols < rows:
		return shapePortrait
	case cols == rows:
		return shapeBalanced
	}
	return shapeLandscape
}

// presetOf is the tier of a cols × rows board, or -1 for a custom size.
func presetOf(cols, rows int) int {
	g := shapeOf(cols, rows)
	for t := range boardSizes {
		if c, r := presetSize(g, t); c == cols && r == rows {
			return t
		}
	}
	return -1
}

// previewFrame is the preview's frame in cells: the shape's largest preset, or the chosen board when it is bigger.
func previewFrame(a *app) (cols, rows int) {
	c, r := presetSize(shapeOf(a.set.Cols, a.set.Rows), len(boardSizes)-1)
	return max(a.set.Cols, c), max(a.set.Rows, r)
}

// setupScene keeps Players above Board and Start outside the scrollable content.
type setupScene struct {
	menu int       // seat whose type menu is open, or -1
	room *hostRoom // a room this player hosts, or nil for a game on this device
	test bool      // private editor test preferences are never persisted

	// Layout in device px, recomputed when the screen size or the room's lobby changes.
	lv                    ui.Rect
	lu, lscroll           float64
	lp, ln                int
	viewport              ui.Rect
	scroll, maxScroll     float64
	blockPad              float64
	back                  ui.Rect
	titleX, titleY        float64
	players, boardB       ui.Rect
	invite                ui.Rect // the single room settings and invitation button
	lsnack                bool    // the join request snackbar was showing
	headY                 float64
	minusP, plusP         ui.Rect
	seatDot, seatTy       [core.MaxPlayers]ui.Rect
	seatHit               [core.MaxPlayers]ui.Rect
	preview, previewPanel ui.Rect
	showPreview           bool
	shapeLabels           [len(shapeNames)]ui.Rect
	presetPills           [len(shapeNames)][len(boardSizes)]ui.Rect
	colsCtl, rowsCtl      ui.Rect
	minusCol, plusCol     ui.Rect
	minusRow, plusRow     ui.Rect
	start                 ui.Rect

	// The screen clipped to the viewport, kept until either changes.
	content, contentOf *ebiten.Image
	contentV           ui.Rect

	dims, sizeName string // the Board heading: "cols × rows", and the preset's name or Custom
	dimC, dimR     int
}

func newSetup() *setupScene { return &setupScene{menu: -1} }

func (s *setupScene) layout(a *app) {
	v := a.safe()
	snacking := s.room != nil && len(s.room.reqs) > 0
	ln := 0 // players' names come with the lobby
	if s.room != nil && s.room.net != nil {
		ln = s.room.net.lobbyN
	}
	if v == s.lv && a.u == s.lu && a.set.Players == s.lp && s.scroll == s.lscroll && snacking == s.lsnack && ln == s.ln {
		return
	}
	s.lv, s.lu, s.lp, s.lsnack, s.ln = v, a.u, a.set.Players, snacking, ln
	u := a.u
	pad, gap, bp := 20*u, 16*u, 20*u
	if v.W < 1200*u {
		pad, gap, bp = 16*u, 12*u, 16*u
	}
	s.blockPad = bp
	w := v.W - 2*pad
	s.back = ui.BackButton(v.X+pad, v.Y+pad, w, u)
	s.titleX, s.titleY = s.back.Right()+16*u, s.back.Cy()
	s.start = ui.Rect{X: v.X + pad, Y: v.Bottom() - pad - 52*u, W: w, H: 52 * u}
	s.invite = ui.Rect{}
	if s.room != nil {
		s.start.W -= 52*u + gap
		s.invite = ui.Rect{X: s.start.Right() + gap, Y: s.start.Y, W: 52 * u, H: 52 * u}
		s.room.snack.avoid = s.start
	}
	top := s.back.Bottom() + gap
	s.viewport = ui.Rect{X: v.X + pad, Y: top, W: w, H: max(1, s.start.Y-gap-top)}
	snack := 0.0 // room under the join request snackbar, so it hides nothing for good
	if s.room != nil {
		if len(s.room.reqs) > 0 {
			snack = snackH*u + gap
		}
	}

	// Visible controls match Back; their larger hit rows preserve touch usability.
	buttonH, targetH := ui.CompactButtonHeight*u, ui.CompactButtonTarget*u
	rowPitch := targetH + 8*u
	vw := 0.0
	for _, number := range ui.Numbers {
		vw = max(vw, ui.TextWidth(number, a.f.Number))
	}
	vw += 12 * u
	inner := w - 2*bp
	// Presets and dimensions form one vertical group beside or below the preview.
	dimensionMin := max(ui.TextWidth("Columns", a.f.LabelBold), ui.TextWidth("Rows", a.f.LabelBold)) + 8*u + 2*targetH + vw
	controlsMin := max(2*dimensionMin+gap, 288*u)
	sideBySide := inner >= 200*u+gap+controlsMin
	controlsW := inner
	if sideBySide {
		controlsW = max(controlsMin, min(420*u, inner*0.42))
	}
	stackDimensions := v.W < 600*u || controlsW < 2*dimensionMin+gap
	dimensionsH := targetH
	if stackDimensions {
		dimensionsH = 2*targetH + gap
	}
	minSeatW := 220 * u
	for _, label := range seatLabels {
		minSeatW = max(minSeatW, seatChoiceX*u+ui.TextWidth(label, a.f.LabelBold)+(52+pipsW)*u)
	}
	right := (52 + pipsW) * u // what follows a player's name: in a room, their state where an AI's shows pips
	if s.room != nil {
		right = ui.ChipWidth(a.ui(), "Not ready", ui.ChipHeight*u) + 60*u
	}
	for i := range a.set.Players {
		if label, opt, _ := s.seat(a, i); opt == seatHuman {
			minSeatW = max(minSeatW, seatChoiceX*u+ui.TextWidth(label, a.f.LabelBold)+right)
		}
	}
	cols := max(1, min(4, a.set.Players, int((inner+gap)/(minSeatW+gap))))
	rows := (a.set.Players + cols - 1) / cols
	playerH := 2*bp + 44*u + float64(rows)*rowPitch
	groupH := 32*u + 3*targetH + 16*u // a heading, then three rows of two presets
	presetH := float64(len(shapeNames))*groupH + float64(len(shapeNames)-1)*24*u
	controlsH := presetH + gap + dimensionsH
	boardMin := 2*bp + 32*u + controlsH
	if !sideBySide {
		boardMin += 128*u + gap // preview above the controls
	}
	s.maxScroll = max(0, boardMin+gap+playerH+snack-s.viewport.H)
	s.scroll = max(0, min(s.maxScroll, s.scroll))
	s.lscroll = s.scroll
	boardH := max(boardMin, s.viewport.H-gap-playerH)
	s.players = ui.Rect{X: v.X + pad, Y: top - s.scroll, W: w, H: playerH}
	s.boardB = ui.Rect{X: v.X + pad, Y: s.players.Bottom() + gap, W: w, H: boardH}
	p := s.players
	s.headY = p.Y + bp + 22*u
	colW := (inner - float64(cols-1)*gap) / float64(cols)
	clear(s.seatHit[:])
	clear(s.seatTy[:])
	for i := range a.set.Players {
		x := p.X + bp + float64(i%cols)*(colW+gap)
		cy := p.Y + bp + 52*u + float64(i/cols)*rowPitch + targetH/2
		r := ui.Rect{X: x, Y: cy - buttonH/2, W: colW, H: buttonH}
		s.seatTy[i], s.seatHit[i] = r, ui.CompactHit(r, u)
		s.seatDot[i] = ui.Rect{X: x + 16*u, Y: cy - 8*u, W: 16 * u, H: 16 * u}
	}

	// Size controls reflow as groups; their labels and hit areas never shrink with the viewport.
	step := func(right, cy float64) (minus, plus ui.Rect) {
		plus = ui.Rect{X: right - 42*u, Y: cy - buttonH/2, W: buttonH, H: buttonH}
		return ui.Rect{X: plus.X - vw - targetH, Y: plus.Y, W: buttonH, H: buttonH}, plus
	}
	s.minusP, s.plusP = step(p.Right()-bp, s.headY)
	b := s.boardB
	body := ui.Rect{X: b.X + bp, Y: b.Y + bp + 32*u, W: inner, H: b.H - 2*bp - 32*u}
	controls := ui.Rect{X: body.Right() - controlsW, Y: body.Cy() - controlsH/2, W: controlsW, H: controlsH}
	if sideBySide {
		// Follow the visible Board area without changing the page's scroll range or moving its controls.
		top, bottom := max(body.Y, s.viewport.Y), min(body.Bottom(), s.viewport.Bottom())
		s.previewPanel = ui.Rect{X: body.X, Y: top, W: controls.X - gap - body.X, H: max(0, bottom-top)}
	} else {
		s.previewPanel = ui.Rect{X: body.X, Y: body.Y, W: body.W, H: max(128*u, body.H-controlsH-gap)}
		controls.X, controls.Y = body.X, s.previewPanel.Bottom()+gap
	}
	presetArea := ui.Rect{X: controls.X, Y: controls.Y, W: controls.W, H: presetH}
	cw := (controls.W - gap) / 2
	s.colsCtl = ui.Rect{X: controls.X, Y: presetArea.Bottom() + gap, W: cw, H: targetH}
	s.rowsCtl = ui.Rect{X: controls.X + cw + gap, Y: s.colsCtl.Y, W: cw, H: targetH}
	if stackDimensions {
		s.colsCtl.W = controls.W
		s.rowsCtl = ui.Rect{X: controls.X, Y: s.colsCtl.Bottom() + gap, W: controls.W, H: targetH}
	}
	s.minusCol, s.plusCol = step(s.colsCtl.Right(), s.colsCtl.Cy())
	s.minusRow, s.plusRow = step(s.rowsCtl.Right(), s.rowsCtl.Cy())
	inset := (targetH - buttonH) / 2
	pw := (presetArea.W - 8*u) / 2
	for g := range s.presetPills {
		y := presetArea.Y + float64(g)*(groupH+24*u)
		s.shapeLabels[g] = ui.Rect{X: presetArea.X, Y: y, W: presetArea.W, H: 24 * u}
		for t := range s.presetPills[g] {
			s.presetPills[g][t] = ui.Rect{X: presetArea.X + float64(t%2)*(pw+8*u), Y: y + 32*u + float64(t/2)*rowPitch + inset, W: pw, H: buttonH}
		}
	}
	s.preview = s.previewPanel.Inset(12 * u)
	s.showPreview = s.preview.W > 0 && s.preview.H > 0
}

func setupStepHit(r ui.Rect, u float64) ui.Rect {
	return r.Inset(-max(0, (ui.CompactButtonTarget*u-r.H)/2))
}

func (s *setupScene) canStart(a *app) bool {
	if s.room != nil {
		return s.room.canAct(a)
	}
	return mapCanStart(a.set)
}

func hasHuman(seats []int) bool { return slices.Contains(seats, seatHuman) }

// minPlayers is the fewest players Setup allows: two, or in an open room as many as keep the host's own seat.
func (s *setupScene) minPlayers() int {
	if s.room != nil {
		return s.room.minPlayers()
	}
	return core.MinPlayers
}

// changed saves a change to the seats or the board, and hands it to the room this player hosts.
func (s *setupScene) changed(a *app) {
	if s.test {
		return
	}
	a.set.save()
	if s.room != nil {
		s.room.apply(a)
	}
}

func (s *setupScene) setPlayers(a *app, n int) {
	a.set.Players = max(s.minPlayers(), min(core.MaxPlayers, n))
	a.set.fitSeats()
	s.menu = -1
	s.changed(a)
}

func (s *setupScene) resize(a *app, dc, dr int) {
	if s.test {
		return
	}
	a.set.Map = nil
	a.set.Cols = max(core.MinSize, min(core.MaxSize, a.set.Cols+dc))
	a.set.Rows = max(core.MinSize, min(core.MaxSize, a.set.Rows+dr))
	s.changed(a)
}

func (s *setupScene) begin(a *app) {
	if !s.canStart(a) {
		return
	}
	if s.room != nil {
		s.room.act(a)
		return
	}
	if p, err := newPlay(a, a.set.Cols, a.set.Rows, a.set.seats); err == nil {
		a.next = p
	}
}

// menuRects keeps options touch-sized, using two columns when the window is short.
func (s *setupScene) menuRects(a *app) (panel, first ui.Rect) {
	u, v := a.u, a.safe()
	t := s.seatTy[s.menu]
	cols := 1
	n := s.menuCount(a)
	if float64(n)*ui.CompactButtonTarget*u+12*u > v.H-16*u {
		cols = 2
	}
	rows := (n + cols - 1) / cols
	w := min(v.W-16*u, max(t.W, 200*u)*float64(cols))
	h := float64(rows)*ui.CompactButtonTarget*u + 12*u
	y := t.Bottom() + 6*u
	if y+h > v.Bottom()-8*u {
		y = t.Y - 6*u - h
	}
	panel = ui.Rect{X: max(v.X+8*u, min(t.X, v.Right()-8*u-w)), Y: max(v.Y+8*u, min(y, v.Bottom()-8*u-h)), W: w, H: h}
	return panel, ui.Rect{X: panel.X + 6*u, Y: panel.Y + 6*u, W: (w - 12*u) / float64(cols), H: ui.CompactButtonTarget * u}
}

// menuCount is how many options a seat's menu has: Human and the AIs, then Swap position and Remove player, which are
// always listed and greyed out where they can't act.
func (s *setupScene) menuCount(*app) int { return len(seatLabels) + 2 }

// canRemove reports whether Remove player acts: a game keeps at least two players.
func (s *setupScene) canRemove(a *app) bool { return a.set.Players > core.MinPlayers }

// self is this player's seat, or -1 with none: the host's in an open room, otherwise the first human one, which
// carries their name.
func (s *setupScene) self(a *app) int {
	if s.room != nil && s.room.net != nil {
		return s.room.net.seat
	}
	return firstHuman(a.set.seats)
}

// canSwap reports whether Swap position acts on the menu's seat: it moves this player there, so the seat must be
// another one and a bot's, as two humans never trade.
func (s *setupScene) canSwap(a *app) bool {
	me := s.self(a)
	return me >= 0 && me != s.menu && s.seatType(a, s.menu) != seatHuman
}

// seat is how Setup shows seat i: its label, its type and, in an open room, a human seat's state.
func (s *setupScene) seat(a *app, i int) (label string, opt int, state ui.Chip) {
	if s.room != nil {
		return s.room.seat(a, i)
	}
	return a.set.label(a.set.seats, i), a.set.seats[i], ui.Chip{}
}

// seatType is seat i's type as Setup shows it: in an open room, the lobby's.
func (s *setupScene) seatType(a *app, i int) int {
	if s.room != nil {
		return s.room.seatType(a, i)
	}
	return a.set.seats[i]
}

// swapSeats trades seats i and j, with their choices. In an open room the hub moves their players with them.
func (s *setupScene) swapSeats(a *app, i, j int) {
	if s.room != nil && s.room.net != nil {
		if err := s.room.net.room.SwapSeats(i, j); err != nil {
			log.Printf("swap seats %d, %d: %v", i, j, err)
			return
		}
	}
	a.set.seats[i], a.set.seats[j] = a.set.seats[j], a.set.seats[i]
	s.menu = -1
	s.changed(a)
}

// removeSeat takes seat i out, bot or human: the seats after it move up one, keeping their choices. In an open room
// the hub moves their players with them and lets the seat's own player go.
func (s *setupScene) removeSeat(a *app, i int) {
	if s.room != nil && s.room.net != nil {
		if err := s.room.net.room.RemoveSeat(i); err != nil {
			log.Printf("remove seat %d: %v", i, err)
			return
		}
	}
	a.set.seats = slices.Delete(a.set.seats, i, i+1)
	a.set.Players--
	s.menu = -1
	s.changed(a)
}

func setupMenuOption(panel, first ui.Rect, i int, u float64) ui.Rect {
	rows := int(math.Round((panel.H - 12*u) / first.H))
	return ui.Rect{X: first.X + float64(i/rows)*first.W, Y: first.Y + float64(i%rows)*first.H, W: first.W, H: first.H}
}

func (s *setupScene) update(a *app) error {
	if s.room != nil {
		if s.room.pre(a, s) {
			return nil
		}
	}
	s.layout(a)
	if s.room != nil {
		if !a.in.esc && s.room.answerSnack(a) {
			return nil
		}
		if s.room.open {
			s.room.updateDialog(a)
			return nil
		}
	}
	u, set := a.u, a.set
	if s.maxScroll > 0 && (a.in.dragY != 0 || a.in.wheel != 0 && s.viewport.Contains(a.in.x, a.in.y)) {
		s.scroll -= a.in.wheel*(ui.CompactButtonTarget+8)*u + a.in.dragY
		s.menu = -1
		s.layout(a)
	}

	if a.in.esc {
		if s.menu >= 0 {
			s.menu = -1
		} else {
			s.leave(a)
		}
		return nil
	}

	// An open menu sits on top: it takes clicks inside it, and a click anywhere else closes it.
	closing := -1
	if s.menu >= 0 {
		panel, first := s.menuRects(a)
		if a.hover(panel) {
			for o := range s.menuCount(a) {
				opt := setupMenuOption(panel, first, o, u)
				if !a.clicked(opt) {
					continue
				}
				switch {
				case o < len(seatLabels):
					set.seats[s.menu] = o
					s.menu = -1
					s.changed(a)
				case o == len(seatLabels):
					if s.canSwap(a) {
						s.swapSeats(a, s.self(a), s.menu)
					}
				case s.canRemove(a):
					s.removeSeat(a, s.menu)
					s.layout(a)
				}
				break
			}
			return nil
		}
		if a.in.clicked {
			closing, s.menu = s.menu, -1
		}
	}

	switch {
	case !s.test && a.clicked(s.mapButton(a)):
		a.next = newMaps(a, s, func(m *core.MapDefinition) {
			a.set.Map = m
			if m != nil {
				a.set.Cols, a.set.Rows = m.Cols, m.Rows
			}
			s.changed(a)
		})
		return nil
	case a.clicked(ui.CompactHit(s.back, u)):
		s.leave(a)
		return nil
	case s.canStart(a) && a.clicked(s.start):
		s.begin(a)
		return nil
	case s.room != nil && a.clicked(s.invite):
		s.room.openDialog()
		return nil
	}
	if !s.viewport.Contains(a.in.x, a.in.y) {
		return nil
	}
	switch {
	case set.Players > s.minPlayers() && a.clicked(setupStepHit(s.minusP, u)):
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
		case !a.clicked(s.seatHit[i]), s.room != nil && !s.room.editable(i):
		case closing != i:
			s.menu = i
		}
	}
	for g, group := range s.presetPills {
		for t, r := range group {
			if !s.test && a.clicked(ui.CompactHit(r, u)) {
				set.Map = nil
				set.Cols, set.Rows = presetSize(g, t)
				s.changed(a)
			}
		}
	}
	return nil
}

func (s *setupScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	ui.DrawBack(dst, a.ui(), s.back)
	title := "Set up the game"
	switch {
	case s.room != nil && s.room.net != nil:
		title = "Room"
	case s.room != nil:
		title = "Set up the room"
	}
	titleFace := f.Title
	if ui.TextWidth(title, titleFace) > s.viewport.Right()-s.titleX {
		titleFace = f.Heading
	}
	ui.DrawText(dst, title, titleFace, s.titleX, s.titleY, text.AlignStart, sc.Ink)

	v := s.viewport
	if s.contentOf != dst || s.contentV != v {
		s.contentOf, s.contentV = dst, v
		s.content = dst.SubImage(image.Rect(int(v.X), int(v.Y), int(v.Right()), int(v.Bottom()))).(*ebiten.Image)
	}
	s.drawPlayers(s.content, a)
	s.drawBoard(s.content, a)
	if s.maxScroll > 0 {
		h := max(24*u, v.H*v.H/(v.H+s.maxScroll))
		ui.FillRound(dst, ui.Rect{X: v.Right() + 5*u, Y: v.Y + (v.H-h)*s.scroll/s.maxScroll, W: 3 * u, H: h}, 1.5*u, sc.Dim)
	}
	s.drawStart(dst, a)
	if s.room != nil {
		drawRoomButton(dst, a, s.invite)
	}
	if s.menu >= 0 {
		s.drawMenu(dst, a)
	}
	if s.room != nil && s.room.open {
		s.room.drawDialog(dst, a)
	}
}

func (s *setupScene) snackbar() *snackbar {
	if s.room == nil || len(s.room.reqs) == 0 {
		return nil
	}
	return &s.room.snack
}

// stepper draws − value + with the buttons on the block's ground.
func stepper(dst *ebiten.Image, a *app, v, lo, hi int, minus, plus ui.Rect) {
	sc, u := a.sc, a.u
	for i, r := range [2]ui.Rect{minus, plus} {
		enabled := i == 0 && v > lo || i == 1 && v < hi
		ink := ui.DrawButtonSurface(dst, a.ui(), r, ui.ButtonGround, enabled, a.over(setupStepHit(r, u)))
		ui.Line(dst, r.Cx()-6*u, r.Cy(), r.Cx()+6*u, r.Cy(), 2.5*u, ink)
		if i == 1 {
			ui.Line(dst, r.Cx(), r.Cy()-6*u, r.Cx(), r.Cy()+6*u, 2.5*u, ink)
		}
	}
	ui.DrawText(dst, ui.Numbers[v], a.f.Number, (minus.Right()+plus.X)/2, minus.Cy(), text.AlignCenter, sc.Ink)
}

func (s *setupScene) drawPlayers(dst *ebiten.Image, a *app) {
	sc, f, u, set := a.sc, &a.f, a.u, a.set
	p := s.players
	ui.FillRound(dst, p, 24*u, sc.Tile)
	ui.DrawText(dst, "Players", f.Heading, p.X+s.blockPad, s.headY, text.AlignStart, sc.Ink)
	stepper(dst, a, set.Players, s.minPlayers(), core.MaxPlayers, s.minusP, s.plusP)

	for i := range set.Players {
		d, t := s.seatDot[i], s.seatTy[i]
		label, opt, state := s.seat(a, i)
		editable := s.room == nil || s.room.editable(i)
		hot := editable && (s.menu < 0 && a.over(s.seatHit[i]) || s.menu == i)
		style := ui.ButtonGround
		if opt == seatHuman || s.menu == i {
			style = ui.ButtonChosen
		}
		ink := ui.DrawButtonSurface(dst, a.ui(), t, style, true, hot)
		ui.SeatDot(dst, sc, i, d.Cx(), d.Cy(), d.W/2, false, set.Marks)
		ui.DrawText(dst, label, f.LabelBold, t.X+seatChoiceX*u, t.Cy(), text.AlignStart, ink)
		right := t.Right() - 36*u
		if !editable {
			right = t.Right() - 16*u
		}
		if state.Word != "" {
			ui.DrawChip(dst, a.ui(), state, right, t.Cy(), ui.ChipHeight*u)
		} else {
			drawPips(dst, opt, right, t.Cy(), u, ink)
		}
		if !editable {
			continue
		}
		tip := 2.5 * u // the chevron points down, or up while its menu is open
		if s.menu == i {
			tip = -tip
		}
		ui.Chevron(dst, t.Right()-20*u, t.Cy(), 5*u, tip, 2*u, ink)
	}
}

// drawStart draws the Start button; when the board is too small for the players it says so instead.
func (s *setupScene) drawStart(dst *ebiten.Image, a *app) {
	label, ok := "Start", s.canStart(a)
	switch {
	case s.room != nil:
		label = s.room.actionLabel(a)
	case !ok:
		label = "Board too small"
	}
	ui.DrawButton(dst, a.ui(), s.start, label, "", ui.ButtonInk, ok)
}

func (s *setupScene) drawBoard(dst *ebiten.Image, a *app) {
	sc, u, set := a.sc, a.u, a.set
	b := s.boardB
	ui.FillRound(dst, b, 24*u, sc.Tile)
	y := b.Y + s.blockPad + 14*u
	ui.DrawText(dst, "Board", a.f.Heading, b.X+s.blockPad, y, text.AlignStart, sc.Ink)
	g, tier := shapeOf(set.Cols, set.Rows), presetOf(set.Cols, set.Rows)
	if s.dimC != set.Cols || s.dimR != set.Rows {
		s.dimC, s.dimR, s.dims = set.Cols, set.Rows, ui.Numbers[set.Cols]+" × "+ui.Numbers[set.Rows]
		s.sizeName = "Custom"
		if tier >= 0 {
			s.sizeName = boardSizes[tier].name
		}
	}
	right := b.Right() - s.blockPad
	ui.DrawText(dst, s.dims, a.f.Number, right, y, text.AlignEnd, sc.Ink)
	ui.DrawText(dst, s.sizeName, a.f.LabelBold, right-ui.TextWidth(s.dims, a.f.Number)-10*u, y, text.AlignEnd, sc.Ink)

	// The chosen board as a mini mat, with its shape's presets as dashed outlines from the same corner: graded sizes,
	// like the nested lines of a sewing pattern.
	if s.previewPanel.H > 0 {
		ui.FillRound(dst, s.previewPanel, 16*u, sc.Ground)
	}
	if s.showPreview {
		pv := s.preview
		pv.H = max(1, pv.H-48*u)
		if set.Map != nil {
			drawMapThumb(dst, a, *set.Map, pv)
		} else {
			drawPreview(dst, a, pv)
		}
	}

	for shape, group := range s.presetPills {
		label := s.shapeLabels[shape]
		ui.DrawText(dst, shapeNames[shape], a.f.LabelBold, label.X, label.Cy(), text.AlignStart, sc.Ink)
		for t, r := range group {
			style := ui.ButtonGround
			if shape == g && t == tier {
				style = ui.ButtonChosen
			}
			ui.DrawButton(dst, a.ui(), r, boardSizes[t].name, presetDims[shape][t], style, true)
		}
	}
	ui.DrawText(dst, "Columns", a.f.LabelBold, s.colsCtl.X, s.colsCtl.Cy(), text.AlignStart, sc.Ink)
	stepper(dst, a, set.Cols, core.MinSize, core.MaxSize, s.minusCol, s.plusCol)
	ui.DrawText(dst, "Rows", a.f.LabelBold, s.rowsCtl.X, s.rowsCtl.Cy(), text.AlignStart, sc.Ink)
	stepper(dst, a, set.Rows, core.MinSize, core.MaxSize, s.minusRow, s.plusRow)
	ui.DrawButton(dst, a.ui(), s.mapButton(a), "Maps", "", ui.ButtonGround, !s.test)
}

// drawPreview draws the chosen board as a mini mat centred in pv, with its shape's presets as dashed outlines from the
// same corner: graded sizes, like the nested lines of a sewing pattern. Preset outlines too small to read at this
// scale are left out.
func drawPreview(dst *ebiten.Image, a *app, pv ui.Rect) {
	sc, u, set := a.sc, a.u, a.set
	g := shapeOf(set.Cols, set.Rows)
	fc, fr := previewFrame(a)
	k := min(pv.W/float64(fc), pv.H/float64(fr))
	ox := math.Round(pv.X + (pv.W-float64(fc)*k)/2)
	oy := math.Round(pv.Y + (pv.H-float64(fr)*k)/2)
	drawMini(dst, a, k, ox, oy)
	for t := range boardSizes {
		c, r := presetSize(g, t)
		// At small scales an outline inside the chosen board only adds scribble, so it is dropped.
		inside := c <= set.Cols && r <= set.Rows
		if float64(min(c, r))*k >= 12*u && !(inside && k < 6*u) {
			ui.DashedRect(dst, ui.Rect{X: ox, Y: oy, W: float64(c) * k, H: float64(r) * k}, 4*u, 1.5*u, sc.Dim)
		}
	}
	ui.StrokeRound(dst, ui.Rect{X: ox - 3*u, Y: oy - 3*u, W: float64(set.Cols)*k + 6*u, H: float64(set.Rows)*k + 6*u}, 6*u, 2.5*u, sc.Ink)
}

// drawMini draws the chosen board's tiles at k px per cell with their top left at (ox, oy), in one batch.
func drawMini(dst *ebiten.Image, a *app, k, ox, oy float64) {
	g := k * boardview.TileGutter
	var img *ebiten.Image
	for c := range a.set.Cols {
		for r := range a.set.Rows {
			img = ui.QueueRound(ui.Rect{X: ox + float64(c)*k + g, Y: oy + float64(r)*k + g, W: k - 2*g, H: k - 2*g}, k*boardview.TileRadius, 0, nil,
				a.sc.Tile)
		}
	}
	ui.DrawShapes(dst, img)
}

func (s *setupScene) drawMenu(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	panel, first := s.menuRects(a)
	ui.FillRound(dst, panel, 18*u, sc.Ground)
	ui.StrokeRound(dst, panel, 18*u, 2*u, sc.Ink)
	_, current, _ := s.seat(a, s.menu)
	for o := range s.menuCount(a) {
		opt := setupMenuOption(panel, first, o, a.u)
		face := ui.Rect{X: opt.X, Y: opt.Cy() - ui.CompactButtonHeight*a.u/2, W: opt.W, H: ui.CompactButtonHeight * a.u}
		if o == len(seatLabels) {
			ui. // the seat's actions are set apart from its types by a hairline
				FillRound(dst, ui.Rect{X: opt.X + 8*u, Y: opt.Y, W: opt.W - 16*u, H: 1.5 * u}, 0, ui.Mix(sc.Ground, sc.Dim, 0.35))
		}
		switch {
		case o == len(seatLabels): // greyed out where this player can't move to the seat
			ink := sc.Dim
			if s.canSwap(a) {
				ink = ui.DrawButtonSurface(dst, a.ui(), face, ui.ButtonGround, true, a.over(opt))
			}
			ui.DrawText(dst, "Swap position", f.LabelBold, opt.X+14*u, opt.Cy(), text.AlignStart, ink)
			continue
		case o > len(seatLabels):
			// Remove player is destructive: in the danger ink, its hover tinted with it; greyed out at two players.
			ink := sc.Dim
			if s.canRemove(a) {
				ink = sc.Danger
				if a.over(opt) {
					ui.FillPill(dst, face, ui.WithAlpha(sc.Danger, 0.14))
				}
			}
			ui.DrawText(dst, "Remove player", f.LabelBold, opt.X+14*u, opt.Cy(), text.AlignStart, ink)
			continue
		}
		style := ui.ButtonGround
		if o == current {
			style = ui.ButtonInk
		}
		ink := ui.DrawButtonSurface(dst, a.ui(), face, style, true, a.over(opt))
		ui.DrawText(dst, seatLabels[o], f.LabelBold, opt.X+14*u, opt.Cy(), text.AlignStart, ink)
		drawPips(dst, o, opt.Right()-14*u, opt.Cy(), u, ink)
	}
}

// leave goes back Home, or from a room to its join page, closing the room.
func (s *setupScene) leave(a *app) {
	if s.room != nil {
		s.room.leave(a)
		return
	}
	a.next = newHome()
}
