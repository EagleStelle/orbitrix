package client

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
)

// homeScene is the title screen: one column as wide as the wordmark (the wordmark, the twelve seat dots spread
// across it, the round play button with the name bar beside it, and a stack of buttons as wide as the column),
// beside a live demo mat where four AIs play real chain reactions. On a screen taller than wide the column opens
// around the mat instead: wordmark and dots above it, the name row and the buttons below it.
type homeScene struct {
	demo *demo

	// The name bar: its field, the edits saved and measured so far, and the name as drawn.
	name            *field
	named, measured int
	nameText        string
	nameW           float64

	// Layout in device px, recomputed when the screen size changes.
	lv                  ui.Rect
	logo                *text.GoTextFace
	logoX               [len(logoLetters)]float64
	x, w                float64 // the column: left edge and width (the wordmark's)
	logoY, dotsY, dotR  float64
	bar                 ui.Rect // the name bar, right of the play button
	playX, playY, playR float64
	playIcon            *ebiten.Image // the play triangle in white, tinted when drawn
	items               []homeItem
	mat                 ui.Rect
}

// homeItem is one button in the stack under the name row.
type homeItem struct {
	label string
	r     ui.Rect
}

var logoLetters = [...]string{"O", "R", "B", "I", "T", "R", "I", "X"}

// namePrompt stands in the name bar, dimmed, until the player types a name.
const namePrompt = "Your name"

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

	// The wordmark sets the width of everything under it: 60 px tall at most. It is set letter by letter with a
	// little tracking: at this weight Unbounded's R and B touch when set solid.
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
	stack := float64(1+rowsOfActions)*(bh+gap) - gap // the name row, then the buttons
	var y float64                                    // top of the name row
	if portrait {
		y = s.layoutPortrait(a, v, size, stack)
	} else {
		y = s.layoutBeside(a, v, size, stack, col)
	}

	// The play button is a circle as tall as the name bar, at the row's left end.
	s.playR = bh / 2
	s.playX, s.playY = s.x+s.playR, y+s.playR
	s.bar = ui.Rect{X: s.x + bh + gap, Y: y, W: s.w - bh - gap, H: bh}
	s.name.caret = image.Rect(int(s.bar.X), int(s.bar.Y), int(s.bar.Right()), int(s.bar.Bottom()))
	s.renderPlay()
	y += bh + gap
	s.items = s.items[:0]
	for i, l := range labels {
		if compactActions && i < 2 {
			half := (s.w - gap) / 2
			s.items = append(s.items, homeItem{l, ui.Rect{X: s.x + float64(i)*(half+gap), Y: y, W: half, H: bh}})
			if i == 1 {
				y += bh + gap
			}
			continue
		}
		s.items = append(s.items, homeItem{l, ui.Rect{X: s.x, Y: y, W: s.w, H: bh}})
		y += bh + gap
	}

	rows := 7
	cols := max(5, min(12, int(math.Round(float64(rows)*s.mat.W/s.mat.H))))
	if s.demo == nil || s.demo.cols != cols {
		s.demo = newDemo(cols, rows, a.rng.Uint64())
	}
	s.demo.layout(s.mat, u)
}

var playPath vector.Path

// renderPlay caches the play triangle, nudged right of centre so it sits optically centred in its circle.
func (s *homeScene) renderPlay() {
	if s.playIcon != nil {
		s.playIcon.Deallocate()
	}
	r := float32(s.playR)
	s.playIcon = ebiten.NewImage(int(2*r)+1, int(2*r)+1)
	playPath.Reset()
	playPath.MoveTo(r-0.24*r, r-0.4*r)
	playPath.LineTo(r+0.44*r, r)
	playPath.LineTo(r-0.24*r, r+0.4*r)
	playPath.Close()
	ui.FillPath(s.playIcon, &playPath, color.RGBA{255, 255, 255, 255})
}

// layoutBeside puts the column left of the mat, centred top to bottom, and returns the top of its name row. On a
// screen too short for it only the gaps tighten: nothing in the column shrinks.
func (s *homeScene) layoutBeside(a *app, v ui.Rect, size, stack, col float64) float64 {
	u := a.u
	fixed := size + 2*s.dotR + stack
	g := max(4*u, min(28*u, (v.H-32*u-fixed)/2))
	s.logoY = v.Y + max(8*u, (v.H-fixed-2*g)/2) + size/2
	s.dotsY = s.logoY + size/2 + g + s.dotR
	s.mat = ui.Rect{X: v.X + col, Y: v.Y + 24*u, W: v.W - col - 24*u, H: v.H - 48*u}
	return s.dotsY + s.dotR + g
}

// layoutPortrait opens the column around the mat: the wordmark and dots at the top, the name row and the buttons at
// the bottom within thumb reach, and the mat filling the middle. It returns the top of the name row.
func (s *homeScene) layoutPortrait(a *app, v ui.Rect, size, stack float64) float64 {
	u := a.u
	s.logoY = v.Y + 32*u + size/2
	s.dotsY = s.logoY + size/2 + 24*u + s.dotR
	stackY := v.Bottom() - 24*u - stack
	top := s.dotsY + s.dotR + 24*u
	s.mat = ui.Rect{X: v.X + 16*u, Y: top, W: v.W - 32*u, H: max(1, stackY-24*u-top)}
	return stackY
}

func (s *homeScene) playRect() ui.Rect {
	return ui.Rect{X: s.playX - s.playR, Y: s.playY - s.playR, W: 2 * s.playR, H: 2 * s.playR}
}

func (s *homeScene) update(a *app) error {
	s.layout(a)
	s.demo.tick()
	if s.updateName(a) || a.clicked(s.playRect()) { // Enter in the name bar plays too
		a.next = newSetup(a)
		return nil
	}
	if a.in.esc {
		platform.Leave() // Android's back button on the first screen; Esc on desktop and web does nothing here
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

// updateName runs the name bar: a tap on it takes typing, a tap elsewhere on a touch screen gives the keyboard back,
// and each change is saved before anything else this update can leave Home. It reports Enter.
func (s *homeScene) updateName(a *app) bool {
	s.name.tap(a, s.bar)
	enter := s.name.update(a)
	if enter {
		s.name.focus(a, false)
	}
	if s.name.edits != s.named {
		s.named = s.name.edits
		if t := string(s.name.text); t != a.set.Name {
			a.set.Name = t
			a.set.save()
		}
	}
	return enter
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

	// The name bar: the name in ink, or its prompt dimmed; outlined with a caret while it takes typing.
	if s.name.edits != s.measured {
		s.measured, s.nameText = s.name.edits, string(s.name.shown)
		s.nameW = ui.TextWidth(s.nameText, a.f.LabelBold)
	}
	ui.DrawButtonSurface(dst, a.ui(), s.bar, ui.ButtonTile, true, !s.name.focused && a.over(s.bar))
	tx := s.bar.X + 22*u
	if s.name.focused {
		ui.StrokeRound(dst, s.bar, s.bar.H/2, 2.5*u, sc.Ink)
		ui.FillRound(dst, ui.Rect{X: tx + s.nameW + u, Y: s.bar.Cy() - 9*u, W: 2 * u, H: 18 * u}, u, sc.Ink)
	}
	if s.nameText != "" {
		ui.DrawText(dst, s.nameText, a.f.LabelBold, tx, s.bar.Cy(), text.AlignStart, sc.Ink)
	} else if !s.name.focused {
		ui.DrawText(dst, namePrompt, a.f.LabelBold, tx, s.bar.Cy(), text.AlignStart, sc.Dim)
	}

	fill := sc.Ink
	if a.over(s.playRect()) {
		fill = sc.InkHi
	}
	ui.FillCircle(dst, s.playX, s.playY, s.playR, fill)
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(s.playX-s.playR, s.playY-s.playR)
	ui.Tint(&op.ColorScale, sc.Ground)
	dst.DrawImage(s.playIcon, &op)

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
		d.b.Start(cell, int8(p), waves, d.game.ProducedAt, nil)
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
