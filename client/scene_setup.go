package client

import (
	"cmp"
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
	sizePicker
	game                   gameBlock
	mapsB                  ui.Rect // the Map heading's button, which opens the map picker
	lmap                   bool
	lstrip                 float64    // the Rules strip's offset when laid out
	seatScroll, pickScroll ui.Scroll  // the seats and the Map body, each scrolling inside its card
	ask                    *askDialog // the warning before Windows' firewall question
	askJoin                bool       // Continue goes on to the join page, not to hosting on Same Wi-Fi
	backTo                 scene      // where Back goes instead of Home: the map picker that opened Setup
	open                   int        // the expanded card: foldNone, foldRules or foldPlayers
	rulesHead, playersHead ui.Rect    // the headings that fold their cards
	mapOf                  *core.MapDefinition
	dimsW                  float64
	mapW                   float64
	start                  ui.Rect

	// The screen clipped to the viewport, kept until either changes.
	content, contentOf *ebiten.Image
	contentV           ui.Rect

	dims, sizeName string // the Board heading: "cols × rows", and the preset's name or Custom
	dimC, dimR     int
}

// newSetup opens Setup where the player last played: on This Device, or as a room not yet open.
func newSetup(a *app) *setupScene {
	if w := a.set.where(); w != whereDevice {
		return newRoomSetup(a, w == whereWifi)
	}
	return &setupScene{menu: -1}
}

// chooseMap labels the Map heading's button, which opens the map picker.
const chooseMap = "Choose a map"

func (s *setupScene) layout(a *app) {
	v := a.safe()
	snacking := s.room != nil && len(s.room.reqs) > 0
	ln := 0 // players' names come with the lobby
	if s.room != nil && s.room.net != nil {
		ln = s.room.net.lobbyN
	}
	saved := a.set.Map != nil
	if v == s.lv && a.u == s.lu && a.set.Players == s.lp && s.scroll == s.lscroll && snacking == s.lsnack && ln == s.ln && saved == s.lmap && s.game.strip.Offset == s.lstrip {
		return
	}
	s.lv, s.lu, s.lp, s.lsnack, s.ln, s.lmap = v, a.u, a.set.Players, snacking, ln, saved
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
	dimensionMin := sizeDimensionMin(a)
	controlsMin := max(2*sizePresetW(a)+8*u, dimensionMin, 288*u) // two preset pills that hug their labels

	// A screen on its side gives Map the whole right, top to bottom, and stacks Rules over Players in a left column
	// one seat wide: wider only while Map still keeps its preview beside its size picker. An upright screen, or one
	// too narrow for both columns, stacks Rules, Players and Map.
	besideMin, lwMin := 200*u+gap+controlsMin+2*bp, minSeatW+2*bp
	twoCol := !a.portrait() && w >= lwMin+gap+288*u+2*bp
	lw, bw, bx := w, w, v.X+pad
	if twoCol {
		lw = max(lwMin, min(math.Round(w*0.36), 440*u, w-gap-besideMin))
		bw, bx = w-gap-lw, v.X+pad+lw+gap
	}
	gameH := 0.0 // the Rules card and the gutter under it; an editor's test game has none
	if !s.test {
		gameH = s.game.layout(a, v.X+pad, 0, lw, bp, s.open == foldRules) + gap
	}
	pinner, inner := lw-2*bp, bw-2*bp
	cols := max(1, min(4, a.set.Players, int((pinner+gap)/(minSeatW+gap))))
	rows := (a.set.Players + cols - 1) / cols
	if s.open != foldPlayers {
		rows = 0 // collapsed to its heading and count
	}
	// Players never grows past what the screen shows under Rules: its seats scroll inside the card instead.
	seatsH := float64(rows) * rowPitch
	playerH := 2*bp + 44*u + min(seatsH, max(rowPitch, s.viewport.H-gameH-2*bp-44*u)) // one row at least

	// Map is exactly as tall as the screen shows. Side by side it never moves, and the page scrolls only when an
	// open Rules card pushes Players off the screen; stacked, the page scrolls down to it.
	boardH := s.viewport.H
	content := gameH + playerH
	if !twoCol {
		content += gap + boardH
	}
	s.maxScroll = max(0, content+snack-s.viewport.H)
	s.scroll = max(0, min(s.maxScroll, s.scroll))
	s.lscroll = s.scroll
	leftY := top - s.scroll
	if !s.test {
		s.game.layout(a, v.X+pad, leftY, lw, bp, s.open == foldRules)
	}
	s.lstrip = s.game.strip.Offset
	s.players = ui.Rect{X: v.X + pad, Y: leftY + gameH, W: lw, H: playerH}
	s.boardB = ui.Rect{X: bx, Y: s.players.Bottom() + gap, W: bw, H: boardH}
	if twoCol {
		s.boardB.Y = top
	}

	p := s.players
	s.headY = p.Y + bp + 22*u
	s.seatScroll.Set(ui.Rect{X: p.X + bp, Y: p.Y + bp + 52*u, W: pinner, H: max(0, p.H-2*bp-52*u)}, max(0, seatsH-8*u)) // down to the card's padding
	colW := (pinner - float64(cols-1)*gap) / float64(cols)
	clear(s.seatHit[:])
	clear(s.seatTy[:])
	for i := range rows * cols { // none while collapsed
		if i >= a.set.Players {
			break
		}
		x := p.X + bp + float64(i%cols)*(colW+gap)
		cy := s.seatScroll.Viewport.Y - s.seatScroll.Offset + float64(i/cols)*rowPitch + targetH/2
		r := ui.Rect{X: x, Y: cy - buttonH/2, W: colW, H: buttonH}
		s.seatTy[i], s.seatHit[i] = r, ui.CompactHit(r, u)
		s.seatDot[i] = ui.Rect{X: x + 16*u, Y: cy - 8*u, W: 16 * u, H: 16 * u}
	}
	s.minusP, s.plusP = stepRects(a, p.Right()-bp-foldRoom*u, s.headY) // left of the chevron
	// A tap on a heading, clear of its buttons, folds its card.
	head := bp + 50*u
	s.playersHead = ui.Rect{X: p.X, Y: p.Y, W: p.W, H: min(p.H, head)}
	s.rulesHead = ui.Rect{}
	if g := s.game.r; !s.test {
		s.rulesHead = ui.Rect{X: g.X, Y: g.Y, W: g.W, H: min(g.H, head)}
	}

	// The Map card's body scrolls inside the card when it is shorter than what it holds: the classic map's size
	// picker beside its preview, or the preview over the picker; a saved map's thumbnail fills it.
	b := s.boardB
	mw := ui.TextWidth(chooseMap, a.f.LabelBold) + 40*u
	s.mapsB = ui.Rect{X: b.Right() - bp - mw, Y: b.Y + bp + 22*u - buttonH/2, W: mw, H: buttonH}
	body := ui.Rect{X: b.X + bp, Y: b.Y + bp + 54*u, W: inner, H: max(0, b.H-2*bp-54*u)} // 12 px under the heading's button, as Players' seats
	sideBySide := inner >= 200*u+gap+controlsMin
	controlsW := inner
	if sideBySide {
		controlsW = controlsMin // the preview takes the rest
	}
	stackDimensions := v.W < 600*u || controlsW < 2*dimensionMin+gap
	controlsH := sizePickerH(u, gap, stackDimensions)
	s.sizePicker, s.previewPanel = sizePicker{}, body
	switch {
	case saved:
		s.pickScroll.Set(body, body.H)
	case sideBySide:
		s.pickScroll.Set(body, controlsH)
		s.previewPanel.W = body.W - controlsW - gap
		s.sizePicker.layout(a, body.Right()-controlsW, body.Y+max(0, (body.H-controlsH)/2)-s.pickScroll.Offset, controlsW, gap, stackDimensions)
	default:
		s.previewPanel.H = max(128*u, body.H-controlsH-gap)
		s.pickScroll.Set(body, s.previewPanel.H+gap+controlsH)
		s.previewPanel.Y -= s.pickScroll.Offset
		s.sizePicker.layout(a, body.X, s.previewPanel.Bottom()+gap, controlsW, gap, stackDimensions)
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
	if s.ask != nil { // Continue goes on to Same Wi-Fi, where Windows asks; it isn't shown again
		yes, back := s.ask.update(a)
		if yes {
			a.set.Firewall = true
			a.set.save()
			if s.askJoin {
				a.next = newJoin(a, true, "")
			} else {
				s.setWhere(a, whereWifi)
			}
		}
		if yes || back {
			s.ask = nil
		}
		return nil
	}
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
	// A card's own scrolling takes the wheel and the drag before the page: the Rules strip, the seats, the Map body.
	if !s.test && s.game.scrolled(a) || s.seatScroll.Update(a.ui()) || s.pickScroll.Update(a.ui()) {
		s.menu, s.lv = -1, ui.Rect{}
		s.layout(a)
	} else if s.maxScroll > 0 && (a.in.dragY != 0 || a.in.wheel != 0 && s.viewport.Contains(a.in.x, a.in.y)) {
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
	case s.updateGame(a):
		return nil
	case a.clicked(s.rulesHead):
		s.fold(foldRules)
		return nil
	case a.clicked(s.playersHead) && (a.in.x < setupStepHit(s.minusP, u).X || a.in.x > setupStepHit(s.plusP, u).Right()): // not a greyed count button
		s.fold(foldPlayers)
		return nil
	case s.test:
	case a.clicked(ui.CompactHit(s.mapsB, u)):
		s.pickMap(a)
		return nil
	case set.Map == nil && s.pickScroll.Viewport.Contains(a.in.x, a.in.y):
		if c, r, ok := s.pick(a, set.Cols, set.Rows); ok {
			set.Cols, set.Rows = c, r
			s.changed(a)
		}
	}
	// Count changes also move the controls. Do not reuse the old rectangles for this click.
	if set.Players != s.lp {
		s.layout(a)
		return nil
	}
	for i := range set.Players {
		switch {
		case !s.seatScroll.Viewport.Contains(a.in.x, a.in.y), !a.clicked(s.seatHit[i]), s.room != nil && !s.room.editable(i):
		case closing != i:
			s.menu = i
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
	if !s.test {
		off := whereOff()
		if s.locked() {
			off = 0xff
		}
		s.game.draw(s.content, a, a.set.where(), off, false, s.open == foldRules, true)
		ui.DrawButton(s.content, a.ui(), s.game.join, "Join a room", "", ui.ButtonGround, !s.locked()) // always there; held while this player hosts
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
	if s.ask != nil {
		s.ask.draw(dst, a)
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
	foldChevron(dst, a, p.Right()-s.blockPad, s.headY, s.open == foldPlayers)
	ui.DrawText(dst, "Players", f.Heading, p.X+s.blockPad, s.headY, text.AlignStart, sc.Ink)
	stepper(dst, a, set.Players, s.minPlayers(), core.MaxPlayers, s.minusP, s.plusP)

	if s.open != foldPlayers {
		return
	}
	s.seatScroll.DrawScrollbar(dst, a.ui())
	dst = s.seatScroll.Clip(dst) // the seats scroll inside the card
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

// drawBoard draws the Map block: the classic map as its preview and size picker, or a saved map's thumbnail. Its
// heading names the map after its size, and its button opens the map picker.
func (s *setupScene) drawBoard(dst *ebiten.Image, a *app) {
	sc, u, set := a.sc, a.u, a.set
	b := s.boardB
	ui.FillRound(dst, b, 24*u, sc.Tile)
	y := b.Y + s.blockPad + 22*u
	ui.DrawText(dst, "Map", a.f.Heading, b.X+s.blockPad, y, text.AlignStart, sc.Ink)
	x := b.X + s.blockPad + ui.TextWidth("Map", a.f.Heading) + 12*u
	room := s.mapsB.X - 12*u - x
	if s.dimC != set.Cols || s.dimR != set.Rows || s.mapOf != set.Map || s.mapW != room {
		s.dimC, s.dimR, s.mapOf, s.mapW = set.Cols, set.Rows, set.Map, room
		s.dims = ui.Numbers[set.Cols] + " × " + ui.Numbers[set.Rows]
		s.dimsW = ui.TextWidth(s.dims, a.f.Number)
		s.sizeName = "Custom"
		if tier := presetOf(set.Cols, set.Rows); tier >= 0 {
			s.sizeName = boardSizes[tier].name
		}
		if set.Map != nil {
			s.sizeName = cmp.Or(set.Map.Name, "Untitled map")
		}
		s.sizeName = ellipsize(s.sizeName, a.f.LabelBold, max(0, room-s.dimsW-10*u))
	}
	// The size follows the heading where the button leaves it room: the dimensions first, then the map's name.
	if s.dimsW <= room {
		ui.DrawText(dst, s.dims, a.f.Number, x, y, text.AlignStart, sc.Dim)
		if s.sizeName != "…" {
			ui.DrawText(dst, s.sizeName, a.f.LabelBold, x+s.dimsW+10*u, y, text.AlignStart, sc.Dim)
		}
	}
	ui.DrawButton(dst, a.ui(), s.mapsB, chooseMap, "", ui.ButtonGround, !s.test)
	s.pickScroll.DrawScrollbar(dst, a.ui())
	dst = s.pickScroll.Clip(dst) // the body scrolls inside the card
	if s.previewPanel.H > 0 {
		ui.FillRound(dst, s.previewPanel, 16*u, sc.Ground)
	}
	if set.Map != nil {
		if s.showPreview {
			drawMapThumb(dst, a, *set.Map, s.preview)
		}
		return
	}
	// The chosen board as a mini mat, with its shape's presets as dashed outlines from the same corner.
	if s.showPreview {
		drawPreview(dst, a, s.preview)
	}
	s.sizePicker.draw(dst, a, set.Cols, set.Rows)
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
	if s.locked() { // closing the room keeps Setup, its seats and its board
		s.room.leave(a)
		s.setWhere(a, a.set.where())
		return
	}
	a.next = s.backTo
	if a.next == nil {
		a.next = newHome()
	}
}

// locked reports a room that is open or opening, which holds the Play on tiles until it closes.
func (s *setupScene) locked() bool { return s.room != nil && (s.room.net != nil || s.room.opening) }

// whereOff are the Where options this build can't offer: Online without a server.
func whereOff() uint8 {
	if origin == "" {
		return 1 << whereOnline
	}
	return 0
}

// setWhere moves Setup to This Device, or to a room not yet open on Same Wi-Fi or Online, and remembers it.
func (s *setupScene) setWhere(a *app, w int) {
	if a.set.Where != w {
		a.set.Where = w
		a.set.save()
	}
	s.room, s.menu, s.lv = nil, -1, ui.Rect{}
	if w != whereDevice {
		s.room = newRoomSetup(a, w == whereWifi).room
	}
}

// updateGame runs the Rules card: Join a room, and the Play on tiles until a room opens. It reports a click it took.
func (s *setupScene) updateGame(a *app) bool {
	if s.test || s.locked() {
		return false
	}
	if a.clicked(ui.CompactHit(s.game.join, a.u)) { // to the tab that matches Play on; Same Wi-Fi from This Device
		lan := a.set.where() != whereOnline
		if s.ask, s.askJoin = firewallAsk(a, lan), true; s.ask == nil {
			a.next = newJoin(a, lan, "")
		}
		return true
	}
	w := s.game.hit(a, 0)
	if w < 0 || w == a.set.where() || whereOff()&(1<<w) != 0 {
		return false
	}
	if s.ask, s.askJoin = firewallAsk(a, w == whereWifi), false; s.ask == nil {
		s.setWhere(a, w)
	}
	return true
}

// pickMap opens the map picker; the map it hands back goes to the room too.
func (s *setupScene) pickMap(a *app) {
	a.next = newMaps(a, s, func(m *core.MapDefinition) {
		a.set.Map = m
		if m != nil {
			a.set.Cols, a.set.Rows = m.Cols, m.Rows
		}
		s.changed(a)
	})
}

// fold opens card, closing the other one, or closes it when it was the open one.
func (s *setupScene) fold(card int) {
	if s.open == card {
		card = foldNone
	}
	s.open, s.menu, s.lv = card, -1, ui.Rect{}
}
