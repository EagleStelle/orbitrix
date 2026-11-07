package client

import (
	"cmp"
	"image"
	"math"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
)

// homeScene is the title screen: one column as wide as the wordmark (the wordmark, the twelve seat dots spread
// across it, the welcome plaque with the player's name, the PLAY dot centred in it, and a stack of full-width
// buttons), beside a live demo mat where four AIs play real chain reactions. On a screen taller than wide the column
// opens around the mat instead: wordmark and dots above it, the plaque, PLAY and the buttons below it. The button
// stack is a list so the Local, LAN and Online entries can join it later.
type homeScene struct {
	demo     *demo
	choosing bool       // PLAY was pressed: the column offers the ways to play instead
	ask      *askDialog // the warning before Windows' firewall question, while it is open

	// The welcome plaque: the name field, the edits saved and measured so far, and the plaque as drawn, which hugs
	// its line and so changes with the name. plaqueOff: the ways to play need its room on this screen.
	name                            *field
	named, measured                 int
	nameText                        string
	greetW, nameW, bangW, plaqueMin float64
	plaque                          ui.Rect
	plaqueOff                       bool

	// Layout in device px, recomputed when the screen size changes.
	lv                  ui.Rect
	logo                *text.GoTextFace
	logoX               [len(logoLetters)]float64
	x, w                float64 // the column: left edge and width (the wordmark's)
	logoY, dotsY, dotR  float64
	playX, playY, playR float64
	playFace            *text.GoTextFace
	items               []homeItem
	modes               []homeItem // the ways to play, then Back, where PLAY and its buttons were
	mat                 ui.Rect
}

// homeItem is one button in the stack under the PLAY dot.
type homeItem struct {
	label string
	r     ui.Rect
}

var logoLetters = [...]string{"O", "R", "B", "I", "T", "R", "I", "X"}

// The welcome plaque: its height and the greeting around the name, set in the buttons' label face (logical px).
const (
	plaqueH   = 34
	plaquePad = 16
	caretRoom = 3 // between the name and its "!" while typing
	greeting  = "Welcome, "
)

func newHome() *homeScene { return &homeScene{} }

func (s *homeScene) layout(a *app) {
	if s.name == nil {
		s.name = nameField()
		s.name.set(a.set.Name)
		s.named = s.name.edits
	}
	v := a.safe()
	if v == s.lv {
		return
	}
	s.lv, s.measured = v, -1
	u := a.u
	portrait := a.portrait()
	pad := 64 * u
	col := max(420*u, v.W*0.36)
	room := col - pad - 24*u // the wordmark's width beside the mat
	if portrait {
		pad, col = 24*u, v.W
		room = col - 2*pad
	}

	// The wordmark sets the width of everything under it: 60 px tall at most, so it leads without crowding the PLAY dot.
	// It is set letter by letter with a little tracking: at this weight Unbounded's R and B touch when set solid.
	track := func(f *text.GoTextFace) float64 {
		w := 0.0
		for i, l := range logoLetters {
			s.logoX[i] = w
			w += ui.TextWidth(l, f) + f.Size*0.045
		}
		return w - f.Size*0.045
	}
	size := min(60*u, a.f.Logo.Size*room/track(a.f.Logo))
	s.logo = ui.NewFace(ui.SrcDisplay, size, 800)
	s.w = track(s.logo)
	s.x = v.X + max(pad, (col-s.w)/2) // centre the column in the area left of the mat

	labels := []string{"Maps", "Settings"}
	if platform.CanExit {
		labels = append(labels, "Exit")
	}
	s.dotR = max(8*u, min(16*u, s.w/12*0.36))
	bh, gap := ui.QuietButtonHeight*u, 12*u
	compactActions := !portrait && v.H < 480*u && len(labels) >= 2
	rowsOfActions := len(labels)
	if compactActions {
		rowsOfActions--
	}
	stack := float64(rowsOfActions)*(bh+gap) - gap
	s.playX = s.x + s.w/2
	var y float64 // top of the button stack
	if portrait {
		y = s.layoutPortrait(a, v, size, stack)
	} else {
		y = s.layoutBeside(a, v, size, stack, col)
	}
	s.playFace = ui.NewFace(ui.SrcDisplay, s.playR*0.34, 800) // the label grows with the dot
	s.items = s.items[:0]
	buttonW := min(2*s.playR, ui.QuietButtonMaxWidth*u)
	if compactActions {
		buttonW = max(buttonW, 220*u)
	}
	s.plaqueMin = buttonW
	for i, l := range labels {
		if compactActions && i < 2 {
			half := (buttonW - gap) / 2
			s.items = append(s.items, homeItem{l, ui.Rect{X: s.playX - buttonW/2 + float64(i)*(half+gap), Y: y, W: half, H: bh}})
			if i == 1 {
				y += bh + gap
			}
			continue
		}
		s.items = append(s.items, homeItem{l, ui.Rect{X: s.playX - buttonW/2, Y: y, W: buttonW, H: bh}})
		y += bh + gap
	}

	// The ways to play stand where PLAY and its buttons stood, centred on them, under the plaque. On a screen too short
	// for that they take the plaque's room too, and it steps aside until Back.
	labels = append(playModes(), "Back")
	modesH := float64(len(labels))*(bh+gap) - gap
	top, bottom := s.plaque.Bottom()+gap, y-gap
	if s.plaqueOff = modesH > bottom-top; s.plaqueOff {
		top = s.dotsY + s.dotR + gap
	}
	my := max(top, min((s.playY-s.playR+bottom-modesH)/2, bottom-modesH))
	modeW := min(s.w, max(buttonW, 220*u))
	s.modes = s.modes[:0]
	for _, l := range labels {
		s.modes = append(s.modes, homeItem{l, ui.Rect{X: s.playX - modeW/2, Y: my, W: modeW, H: bh}})
		my += bh + gap
	}

	rows := 7
	cols := max(5, min(12, int(math.Round(float64(rows)*s.mat.W/s.mat.H))))
	if s.demo == nil || s.demo.cols != cols {
		s.demo = newDemo(cols, rows, a.rng.Uint64())
	}
	s.demo.layout(s.mat, u)
	s.fitName(a)
}

// layoutBeside puts the column left of the mat, centred top to bottom, and returns the top of its button stack. On a
// screen too short for it, the gaps tighten and the PLAY dot shrinks until the column fits; on one shorter still, only
// the gaps give again, so the plaque never makes anything smaller.
func (s *homeScene) layoutBeside(a *app, v ui.Rect, size, stack, col float64) float64 {
	u := a.u
	gLogo, gDots, gName, gPlay := 30*u, 24*u, 20*u, 32*u
	rest := func() float64 { return size + gLogo + 2*s.dotR + gDots + plaqueH*u + gName + gPlay + stack }
	s.playR = max(64*u, min(s.w*0.3, v.H*0.17, 150*u))
	if rest()+2*s.playR > v.H-32*u {
		gLogo, gDots, gName, gPlay = 16*u, 12*u, 12*u, 20*u
	}
	if rest()+80*u > v.H-32*u { // too short even for the smallest PLAY dot
		gLogo, gDots, gName, gPlay = 12*u, 8*u, 8*u, 12*u
	}
	s.playR = max(40*u, min(s.playR, (v.H-32*u-rest())/2))
	s.logoY = v.Y + (v.H-rest()-2*s.playR)/2 + size/2
	s.dotsY = s.logoY + size/2 + gLogo + s.dotR
	s.plaque = ui.Rect{X: s.playX, Y: s.dotsY + s.dotR + gDots, W: 0, H: plaqueH * u} // its width follows the name
	s.playY = s.plaque.Bottom() + gName + s.playR
	s.mat = ui.Rect{X: v.X + col, Y: v.Y + 24*u, W: v.W - col - 24*u, H: v.H - 48*u}
	return s.playY + s.playR + gPlay
}

// layoutPortrait opens the column around the mat: the wordmark and dots at the top, PLAY and the buttons at the
// bottom within thumb reach, and the mat filling the middle. It returns the top of the button stack.
func (s *homeScene) layoutPortrait(a *app, v ui.Rect, size, stack float64) float64 {
	u := a.u
	s.playR = max(48*u, min(s.w*0.3, v.H*0.11, 150*u))
	s.logoY = v.Y + 32*u + size/2
	s.dotsY = s.logoY + size/2 + 24*u + s.dotR
	stackY := v.Bottom() - 24*u - stack
	s.playY = stackY - 24*u - s.playR
	s.plaque = ui.Rect{X: s.playX, Y: s.playY - s.playR - 20*u - plaqueH*u, W: 0, H: plaqueH * u}
	top := s.dotsY + s.dotR + 24*u
	s.mat = ui.Rect{X: v.X + 16*u, Y: top, W: v.W - 32*u, H: max(1, s.plaque.Y-20*u-top)}
	return stackY
}

func (s *homeScene) playRect() ui.Rect {
	return ui.Rect{X: s.playX - s.playR, Y: s.playY - s.playR, W: 2 * s.playR, H: 2 * s.playR}
}

func (s *homeScene) update(a *app) error {
	s.layout(a)
	s.demo.tick()
	if s.ask != nil {
		s.updateAsk(a)
		return nil
	}
	s.updateName(a)
	if s.choosing {
		s.updateModes(a)
		return nil
	}
	if a.in.esc {
		platform.Leave() // Android's back button on the first screen; Esc on desktop and web does nothing here
		return nil
	}
	if a.clicked(s.playRect()) {
		if len(playModes()) == 1 {
			a.next = newSetup() // nothing to choose between
		} else {
			s.choosing = true
		}
		return nil
	}
	for _, it := range s.items {
		if !a.clicked(it.r) {
			continue
		}
		switch it.label {
		case "Maps":
			a.next = newMaps(a, s, nil)
		case "Settings":
			a.next = newSettingsMenu(s)
		case "Exit":
			return ebiten.Termination
		}
	}
	return nil
}

func (s *homeScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, u := a.sc, a.u
	s.demo.b.Draw(dst, sc, a.tms(), -1, a.set.Marks)

	for i, l := range logoLetters {
		ui.DrawText(dst, l, s.logo, s.x+s.logoX[i], s.logoY, text.AlignStart, sc.Ink)
	}
	// The twelve seats, spread edge to edge under the wordmark.
	step := (s.w - 2*s.dotR) / (core.MaxPlayers - 1)
	for p := range core.MaxPlayers {
		ui.SeatDot(dst, sc, p, s.x+s.dotR+float64(p)*step, s.dotsY, s.dotR, false, a.set.Marks)
	}
	if !s.choosing || !s.plaqueOff {
		s.drawName(dst, a)
	}

	if s.choosing {
		for _, it := range s.modes {
			style := ui.ButtonInk
			if it.label == "Back" {
				style = ui.ButtonTile
			}
			ui.DrawButton(dst, a.ui(), it.r, it.label, "", style, true)
		}
		if s.ask != nil {
			s.ask.draw(dst, a)
		}
		return
	}
	fill := sc.Ink
	if a.over(s.playRect()) {
		fill = sc.InkHi
	}
	ui.FillCircle(dst, s.playX, s.playY, s.playR, fill)
	ui.DrawText(dst, "PLAY", s.playFace, s.playX, s.playY-2*u, text.AlignCenter, sc.Ground)

	for _, it := range s.items {
		ui.DrawButton(dst, a.ui(), it.r, it.label, "", ui.ButtonTile, true)
	}
}

// demo plays a silent four-AI game on a small board, over and over, for the Home screen.
type demo struct {
	cols, rows int
	game       *core.Game
	b          *boardview.Board
	bot        ai.Bot
	rng        core.RNG
	wait       int
	area       ui.Rect
	u          float64
}

// demoStrategies are picked for lively, chain-heavy play.
var demoStrategies = [4]ai.Strategy{ai.Gambler, ai.Raider, ai.Spy, ai.Jester}

func newDemo(cols, rows int, seed uint64) *demo {
	d := &demo{cols: cols, rows: rows, rng: core.NewRNG(seed)}
	d.reset()
	return d
}

// reset starts a new demo game a few rounds in, so the mat is never empty.
func (d *demo) reset() {
	g, err := core.NewGame(d.cols, d.rows, len(demoStrategies))
	if err != nil {
		panic("client: demo board: " + err.Error()) // fixed sizes, always valid
	}
	d.game, d.b = g, boardview.New(g)
	for range 3 * len(demoStrategies) {
		d.step(false)
	}
	d.b.Sync(g)
	if d.u != 0 {
		d.b.Layout(d.area, d.u)
	}
	d.wait = 30
}

func (d *demo) layout(area ui.Rect, u float64) {
	d.area, d.u = area, u
	d.b.Layout(area, u)
}

func (d *demo) step(animate bool) {
	p := d.game.Turn
	cell := d.bot.Choose(d.game, demoStrategies[p], d.rng.Uint64(), 120)
	waves, err := d.game.Apply(cell)
	if err != nil {
		return
	}
	if animate {
		d.b.Start(cell, int8(p), waves, nil)
	}
}

func (d *demo) tick() {
	d.b.Tick(nil)
	switch {
	case d.b.Playing:
	case d.wait > 0:
		d.wait--
	case d.game.Over():
		d.reset()
	default:
		d.step(true)
		d.wait = 24
		if d.game.Over() {
			d.wait = 150
		}
	}
}

// playModes are the ways to play this build offers.
func playModes() []string { return waysToPlay(origin) }

// waysToPlay is This Device and Same Wi-Fi always (a browser hosts server-free rooms, joined by invite links, and joins
// the LAN host that served its page), and Online where the build has a server.
func waysToPlay(origin string) []string {
	modes := []string{"This Device", "Same Wi-Fi"}
	if origin != "" {
		modes = append(modes, "Online")
	}
	return modes
}

// updateName runs the plaque's name: a tap on the plaque takes typing, Enter (or on a touch screen a tap elsewhere)
// gives it back, and each change is saved before anything else this update can leave Home.
func (s *homeScene) updateName(a *app) {
	s.fitName(a)
	if s.choosing && s.plaqueOff {
		s.name.focused = false
	} else {
		s.name.tap(a, ui.CompactHit(s.plaque, a.u))
	}
	if s.name.update(a) {
		s.name.focus(a, false)
	}
	if s.name.edits == s.named {
		return
	}
	s.named = s.name.edits
	if t := string(s.name.text); t != a.set.Name {
		a.set.Name = t
		a.set.save()
	}
}

// fitName measures the plaque's line after an edit and sizes the plaque to hug it: at least as wide as the buttons
// under PLAY, at most the column.
func (s *homeScene) fitName(a *app) {
	if s.name.edits == s.measured {
		return
	}
	f, u := &a.f, a.u
	s.measured, s.nameText = s.name.edits, string(s.name.shown)
	s.greetW, s.bangW = ui.TextWidth(greeting, f.LabelBold), ui.TextWidth("!", f.LabelBold)
	s.nameW = ui.TextWidth(cmp.Or(s.nameText, seatLabels[seatHuman]), f.LabelBold)
	w := min(s.w, max(s.plaqueMin, s.greetW+s.nameW+caretRoom*u+s.bangW+2*plaquePad*u)) // typing never widens it
	s.plaque.X, s.plaque.W = s.playX-w/2, w
	s.name.caret = image.Rect(int(s.plaque.X), int(s.plaque.Y), int(s.plaque.Right()), int(s.plaque.Bottom()))
}

// drawName draws the welcome plaque, bare words with no fill or border: "Welcome, Ana!", the name in ink between the
// dim greeting and its mark, or Human dimmed while there is none. While the name takes typing a caret follows it.
func (s *homeScene) drawName(dst *ebiten.Image, a *app) {
	s.fitName(a)
	sc, f, u, p := a.sc, &a.f, a.u, s.plaque
	x := p.Cx() - (s.greetW+s.nameW+s.bangW)/2
	ui.DrawText(dst, greeting, f.LabelBold, x, p.Cy(), text.AlignStart, sc.Dim)
	name, ink := s.nameText, sc.Ink
	if name == "" {
		name, ink = seatLabels[seatHuman], sc.Dim
	}
	ui.DrawText(dst, name, f.LabelBold, x+s.greetW, p.Cy(), text.AlignStart, ink)
	mark := x + s.greetW + s.nameW
	if s.name.focused {
		caret := mark + u
		if s.nameText == "" {
			caret = x + s.greetW - u // where typing starts, before the dimmed Human
		} else {
			mark += caretRoom * u // the caret stands between the name and its mark
		}
		ui.FillRound(dst, ui.Rect{X: caret, Y: p.Cy() - 9*u, W: 2 * u, H: 18 * u}, u, sc.Ink)
	}
	ui.DrawText(dst, "!", f.LabelBold, mark, p.Cy(), text.AlignStart, sc.Dim)
}

// updateAsk runs the firewall warning: Continue goes on to Same Wi-Fi, where Windows asks; it isn't shown again.
func (s *homeScene) updateAsk(a *app) {
	switch yes, back := s.ask.update(a); {
	case yes:
		a.set.Firewall = true
		a.set.save()
		a.next = newJoin(a, true, "")
	case back:
		s.ask = nil
	}
}

// updateModes runs the ways to play; Back (or Esc) brings PLAY, Settings and Exit back.
func (s *homeScene) updateModes(a *app) {
	if a.in.esc {
		s.choosing = false
		return
	}
	for _, it := range s.modes {
		if !a.clicked(it.r) {
			continue
		}
		switch it.label {
		case "This Device":
			a.next = newSetup()
		case "Same Wi-Fi":
			if runtime.GOOS == "windows" && !a.set.Firewall {
				s.ask = &askDialog{text: firewallText, yes: "Continue"}
			} else {
				a.next = newJoin(a, true, "")
			}
		case "Online":
			a.next = newJoin(a, false, "")
		case "Back":
			s.choosing = false
		}
	}
}
