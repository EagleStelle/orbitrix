package client

import (
	"image"
	"image/color"
	"log"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

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
			c = withAlpha(ink, 0.25)
		}
		fillCircle(dst, right-(pipsW-pipR)*u+float64(i)*pipStep*u, cy, pipR*u, c)
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
			d[g][t] = numbers[c] + " × " + numbers[r]
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
	lv                    rect
	lu, lscroll           float64
	lp, ln                int
	viewport              rect
	scroll, maxScroll     float64
	blockPad              float64
	back                  rect
	titleX, titleY        float64
	players, boardB       rect
	invite                rect // the single room settings and invitation button
	lsnack                bool // the join request snackbar was showing
	headY                 float64
	minusP, plusP         rect
	seatDot, seatTy       [core.MaxPlayers]rect
	seatHit               [core.MaxPlayers]rect
	preview, previewPanel rect
	showPreview           bool
	shapeLabels           [len(shapeNames)]rect
	presetPills           [len(shapeNames)][len(boardSizes)]rect
	colsCtl, rowsCtl      rect
	minusCol, plusCol     rect
	minusRow, plusRow     rect
	start                 rect

	// The screen clipped to the viewport, kept until either changes.
	content, contentOf *ebiten.Image
	contentV           rect

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
	if v.w < 1200*u {
		pad, gap, bp = 16*u, 12*u, 16*u
	}
	s.blockPad = bp
	w := v.w - 2*pad
	s.back = backButton(v.x+pad, v.y+pad, w, u)
	s.titleX, s.titleY = s.back.right()+16*u, s.back.cy()
	s.start = rect{v.x + pad, v.bottom() - pad - 52*u, w, 52 * u}
	s.invite = rect{}
	if s.room != nil {
		s.start.w -= 52*u + gap
		s.invite = rect{s.start.right() + gap, s.start.y, 52 * u, 52 * u}
		s.room.snack.avoid = s.start
	}
	top := s.back.bottom() + gap
	s.viewport = rect{v.x + pad, top, w, max(1, s.start.y-gap-top)}
	snack := 0.0 // room under the join request snackbar, so it hides nothing for good
	if s.room != nil {
		if len(s.room.reqs) > 0 {
			snack = snackH*u + gap
		}
	}

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
	stackDimensions := v.w < 600*u || controlsW < 2*dimensionMin+gap
	dimensionsH := targetH
	if stackDimensions {
		dimensionsH = 2*targetH + gap
	}
	minSeatW := 220 * u
	for _, label := range seatLabels {
		minSeatW = max(minSeatW, seatChoiceX*u+textWidth(label, a.f.labelBold)+(52+pipsW)*u)
	}
	right := (52 + pipsW) * u // what follows a player's name: in a room, their state where an AI's shows pips
	if s.room != nil {
		right = chipWidth(a, "Not ready", chipHeight*u) + 60*u
	}
	for i := range a.set.Players {
		if label, opt, _ := s.seat(a, i); opt == seatHuman {
			minSeatW = max(minSeatW, seatChoiceX*u+textWidth(label, a.f.labelBold)+right)
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
	s.maxScroll = max(0, boardMin+gap+playerH+snack-s.viewport.h)
	s.scroll = max(0, min(s.maxScroll, s.scroll))
	s.lscroll = s.scroll
	boardH := max(boardMin, s.viewport.h-gap-playerH)
	s.players = rect{v.x + pad, top - s.scroll, w, playerH}
	s.boardB = rect{v.x + pad, s.players.bottom() + gap, w, boardH}
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
		// Follow the visible Board area without changing the page's scroll range or moving its controls.
		top, bottom := max(body.y, s.viewport.y), min(body.bottom(), s.viewport.bottom())
		s.previewPanel = rect{body.x, top, controls.x - gap - body.x, max(0, bottom-top)}
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
	inset := (targetH - buttonH) / 2
	pw := (presetArea.w - 8*u) / 2
	for g := range s.presetPills {
		y := presetArea.y + float64(g)*(groupH+24*u)
		s.shapeLabels[g] = rect{presetArea.x, y, presetArea.w, 24 * u}
		for t := range s.presetPills[g] {
			s.presetPills[g][t] = rect{presetArea.x + float64(t%2)*(pw+8*u), y + 32*u + float64(t/2)*rowPitch + inset, pw, buttonH}
		}
	}
	s.preview = s.previewPanel.inset(12 * u)
	s.showPreview = s.preview.w > 0 && s.preview.h > 0
}

func setupStepHit(r rect, u float64) rect {
	return r.inset(-max(0, (compactButtonTarget*u-r.h)/2))
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
func (s *setupScene) menuRects(a *app) (panel, first rect) {
	u, v := a.u, a.safe()
	t := s.seatTy[s.menu]
	cols := 1
	n := s.menuCount(a)
	if float64(n)*compactButtonTarget*u+12*u > v.h-16*u {
		cols = 2
	}
	rows := (n + cols - 1) / cols
	w := min(v.w-16*u, max(t.w, 200*u)*float64(cols))
	h := float64(rows)*compactButtonTarget*u + 12*u
	y := t.bottom() + 6*u
	if y+h > v.bottom()-8*u {
		y = t.y - 6*u - h
	}
	panel = rect{max(v.x+8*u, min(t.x, v.right()-8*u-w)), max(v.y+8*u, min(y, v.bottom()-8*u-h)), w, h}
	return panel, rect{panel.x + 6*u, panel.y + 6*u, (w - 12*u) / float64(cols), compactButtonTarget * u}
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
func (s *setupScene) seat(a *app, i int) (label string, opt int, state chip) {
	if s.room != nil {
		return s.room.seat(a, i)
	}
	return a.set.label(a.set.seats, i), a.set.seats[i], chip{}
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

func setupMenuOption(panel, first rect, i int, u float64) rect {
	rows := int(math.Round((panel.h - 12*u) / first.h))
	return rect{first.x + float64(i/rows)*first.w, first.y + float64(i%rows)*first.h, first.w, first.h}
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
	if s.maxScroll > 0 && (a.in.dragY != 0 || a.in.wheel != 0 && s.viewport.contains(a.in.x, a.in.y)) {
		s.scroll -= a.in.wheel*(compactButtonTarget+8)*u + a.in.dragY
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
	case a.clicked(compactHit(s.back, u)):
		s.leave(a)
		return nil
	case s.canStart(a) && a.clicked(s.start):
		s.begin(a)
		return nil
	case s.room != nil && a.clicked(s.invite):
		s.room.openDialog()
		return nil
	}
	if !s.viewport.contains(a.in.x, a.in.y) {
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
			if !s.test && a.clicked(compactHit(r, u)) {
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

	drawBack(dst, a, s.back)
	title := "Set up the game"
	switch {
	case s.room != nil && s.room.net != nil:
		title = "Room"
	case s.room != nil:
		title = "Set up the room"
	}
	titleFace := f.title
	if textWidth(title, titleFace) > s.viewport.right()-s.titleX {
		titleFace = f.heading
	}
	drawText(dst, title, titleFace, s.titleX, s.titleY, text.AlignStart, sc.ink)

	v := s.viewport
	if s.contentOf != dst || s.contentV != v {
		s.contentOf, s.contentV = dst, v
		s.content = dst.SubImage(image.Rect(int(v.x), int(v.y), int(v.right()), int(v.bottom()))).(*ebiten.Image)
	}
	s.drawPlayers(s.content, a)
	s.drawBoard(s.content, a)
	if s.maxScroll > 0 {
		h := max(24*u, v.h*v.h/(v.h+s.maxScroll))
		fillRound(dst, rect{v.right() + 5*u, v.y + (v.h-h)*s.scroll/s.maxScroll, 3 * u, h}, 1.5*u, sc.dim)
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
	stepper(dst, a, set.Players, s.minPlayers(), core.MaxPlayers, s.minusP, s.plusP)

	for i := range set.Players {
		d, t := s.seatDot[i], s.seatTy[i]
		label, opt, state := s.seat(a, i)
		editable := s.room == nil || s.room.editable(i)
		hot := editable && (s.menu < 0 && a.over(s.seatHit[i]) || s.menu == i)
		style := btnGround
		if opt == seatHuman || s.menu == i {
			style = btnChosen
		}
		ink := drawButtonSurface(dst, a, t, style, true, hot)
		seatDot(dst, sc, i, d.cx(), d.cy(), d.w/2, false, set.Marks)
		drawText(dst, label, f.labelBold, t.x+seatChoiceX*u, t.cy(), text.AlignStart, ink)
		right := t.right() - 36*u
		if !editable {
			right = t.right() - 16*u
		}
		if state.word != "" {
			drawChip(dst, a, state, right, t.cy(), chipHeight*u)
		} else {
			drawPips(dst, opt, right, t.cy(), u, ink)
		}
		if !editable {
			continue
		}
		tip := 2.5 * u // the chevron points down, or up while its menu is open
		if s.menu == i {
			tip = -tip
		}
		chevron(dst, t.right()-20*u, t.cy(), 5*u, tip, 2*u, ink)
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
	drawButton(dst, a, s.start, label, "", btnInk, ok)
}

func (s *setupScene) drawBoard(dst *ebiten.Image, a *app) {
	sc, u, set := a.sc, a.u, a.set
	b := s.boardB
	fillRound(dst, b, 24*u, sc.tile)
	y := b.y + s.blockPad + 14*u
	drawText(dst, "Board", a.f.heading, b.x+s.blockPad, y, text.AlignStart, sc.ink)
	g, tier := shapeOf(set.Cols, set.Rows), presetOf(set.Cols, set.Rows)
	if s.dimC != set.Cols || s.dimR != set.Rows {
		s.dimC, s.dimR, s.dims = set.Cols, set.Rows, numbers[set.Cols]+" × "+numbers[set.Rows]
		s.sizeName = "Custom"
		if tier >= 0 {
			s.sizeName = boardSizes[tier].name
		}
	}
	right := b.right() - s.blockPad
	drawText(dst, s.dims, a.f.number, right, y, text.AlignEnd, sc.ink)
	drawText(dst, s.sizeName, a.f.labelBold, right-textWidth(s.dims, a.f.number)-10*u, y, text.AlignEnd, sc.ink)

	// The chosen board as a mini mat, with its shape's presets as dashed outlines from the same corner: graded sizes,
	// like the nested lines of a sewing pattern.
	if s.previewPanel.h > 0 {
		fillRound(dst, s.previewPanel, 16*u, sc.ground)
	}
	if s.showPreview {
		pv := s.preview
		pv.h = max(1, pv.h-48*u)
		if set.Map != nil {
			drawMapThumb(dst, a, *set.Map, pv)
		} else {
			drawPreview(dst, a, pv)
		}
	}

	for shape, group := range s.presetPills {
		label := s.shapeLabels[shape]
		drawText(dst, shapeNames[shape], a.f.labelBold, label.x, label.cy(), text.AlignStart, sc.ink)
		for t, r := range group {
			style := btnGround
			if shape == g && t == tier {
				style = btnChosen
			}
			drawButton(dst, a, r, boardSizes[t].name, presetDims[shape][t], style, true)
		}
	}
	drawText(dst, "Columns", a.f.labelBold, s.colsCtl.x, s.colsCtl.cy(), text.AlignStart, sc.ink)
	stepper(dst, a, set.Cols, core.MinSize, core.MaxSize, s.minusCol, s.plusCol)
	drawText(dst, "Rows", a.f.labelBold, s.rowsCtl.x, s.rowsCtl.cy(), text.AlignStart, sc.ink)
	stepper(dst, a, set.Rows, core.MinSize, core.MaxSize, s.minusRow, s.plusRow)
	drawButton(dst, a, s.mapButton(a), "Maps", "", btnGround, !s.test)
}

// drawPreview draws the chosen board as a mini mat centred in pv, with its shape's presets as dashed outlines from the
// same corner: graded sizes, like the nested lines of a sewing pattern. Preset outlines too small to read at this
// scale are left out.
func drawPreview(dst *ebiten.Image, a *app, pv rect) {
	sc, u, set := a.sc, a.u, a.set
	g := shapeOf(set.Cols, set.Rows)
	fc, fr := previewFrame(a)
	k := min(pv.w/float64(fc), pv.h/float64(fr))
	ox := math.Round(pv.x + (pv.w-float64(fc)*k)/2)
	oy := math.Round(pv.y + (pv.h-float64(fr)*k)/2)
	drawMini(dst, a, k, ox, oy)
	for t := range boardSizes {
		c, r := presetSize(g, t)
		// At small scales an outline inside the chosen board only adds scribble, so it is dropped.
		inside := c <= set.Cols && r <= set.Rows
		if float64(min(c, r))*k >= 12*u && !(inside && k < 6*u) {
			dashedRect(dst, rect{ox, oy, float64(c) * k, float64(r) * k}, 4*u, 1.5*u, sc.dim)
		}
	}
	strokeRound(dst, rect{ox - 3*u, oy - 3*u, float64(set.Cols)*k + 6*u, float64(set.Rows)*k + 6*u}, 6*u, 2.5*u, sc.ink)
}

// drawMini draws the chosen board's tiles at k px per cell with their top left at (ox, oy), in one batch.
func drawMini(dst *ebiten.Image, a *app, k, ox, oy float64) {
	g := k * tileGutter
	var img *ebiten.Image
	for c := range a.set.Cols {
		for r := range a.set.Rows {
			img = queueRound(rect{ox + float64(c)*k + g, oy + float64(r)*k + g, k - 2*g, k - 2*g}, k*tileRadius, 0, nil,
				a.sc.tile)
		}
	}
	drawShapes(dst, img)
}

func (s *setupScene) drawMenu(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	panel, first := s.menuRects(a)
	fillRound(dst, panel, 18*u, sc.ground)
	strokeRound(dst, panel, 18*u, 2*u, sc.ink)
	_, current, _ := s.seat(a, s.menu)
	for o := range s.menuCount(a) {
		opt := setupMenuOption(panel, first, o, a.u)
		face := rect{opt.x, opt.cy() - compactButtonHeight*a.u/2, opt.w, compactButtonHeight * a.u}
		if o == len(seatLabels) { // the seat's actions are set apart from its types by a hairline
			fillRound(dst, rect{opt.x + 8*u, opt.y, opt.w - 16*u, 1.5 * u}, 0, mix(sc.ground, sc.dim, 0.35))
		}
		switch {
		case o == len(seatLabels): // greyed out where this player can't move to the seat
			ink := sc.dim
			if s.canSwap(a) {
				ink = drawButtonSurface(dst, a, face, btnGround, true, a.over(opt))
			}
			drawText(dst, "Swap position", f.labelBold, opt.x+14*u, opt.cy(), text.AlignStart, ink)
			continue
		case o > len(seatLabels):
			// Remove player is destructive: in the danger ink, its hover tinted with it; greyed out at two players.
			ink := sc.dim
			if s.canRemove(a) {
				ink = sc.danger
				if a.over(opt) {
					fillPill(dst, face, withAlpha(sc.danger, 0.14))
				}
			}
			drawText(dst, "Remove player", f.labelBold, opt.x+14*u, opt.cy(), text.AlignStart, ink)
			continue
		}
		style := btnGround
		if o == current {
			style = btnInk
		}
		ink := drawButtonSurface(dst, a, face, style, true, a.over(opt))
		drawText(dst, seatLabels[o], f.labelBold, opt.x+14*u, opt.cy(), text.AlignStart, ink)
		drawPips(dst, o, opt.right()-14*u, opt.cy(), u, ink)
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
