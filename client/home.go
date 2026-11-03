package client

import (
	"math"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
)

// homeScene is the title screen: one column as wide as the wordmark (the wordmark, the twelve seat dots spread
// across it, the PLAY dot centred in it, and a stack of full-width buttons), beside a live demo mat where four AIs
// play real chain reactions. On a screen taller than wide the column opens around the mat instead: wordmark and
// dots above it, PLAY and the buttons below it. The button stack is a list so the Local, LAN and Online entries
// can join it later.
type homeScene struct {
	demo     *demo
	choosing bool       // PLAY was pressed: the column offers the ways to play instead
	ask      *askDialog // the warning before Windows' firewall question, while it is open

	// Layout in device px, recomputed when the screen size changes.
	lv                  rect
	logo                *text.GoTextFace
	logoX               [len(logoLetters)]float64
	x, w                float64 // the column: left edge and width (the wordmark's)
	logoY, dotsY, dotR  float64
	playX, playY, playR float64
	playFace            *text.GoTextFace
	items               []homeItem
	modes               []homeItem // the ways to play, then Back, where PLAY and its buttons were
	mat                 rect
}

// homeItem is one button in the stack under the PLAY dot.
type homeItem struct {
	label string
	r     rect
}

var logoLetters = [...]string{"O", "R", "B", "I", "T", "R", "I", "X"}

func newHome() *homeScene { return &homeScene{} }

func (s *homeScene) layout(a *app) {
	v := a.safe()
	if v == s.lv {
		return
	}
	s.lv = v
	u := a.u
	portrait := a.portrait()
	pad := 64 * u
	col := max(420*u, v.w*0.36)
	room := col - pad - 24*u // the wordmark's width beside the mat
	if portrait {
		pad, col = 24*u, v.w
		room = col - 2*pad
	}

	// The wordmark sets the width of everything under it: 60 px tall at most, so it leads without crowding the PLAY dot.
	// It is set letter by letter with a little tracking: at this weight Unbounded's R and B touch when set solid.
	track := func(f *text.GoTextFace) float64 {
		w := 0.0
		for i, l := range logoLetters {
			s.logoX[i] = w
			w += textWidth(l, f) + f.Size*0.045
		}
		return w - f.Size*0.045
	}
	size := min(60*u, a.f.logo.Size*room/track(a.f.logo))
	s.logo = newFace(srcDisplay, size, 800)
	s.w = track(s.logo)
	s.x = v.x + max(pad, (col-s.w)/2) // centre the column in the area left of the mat

	labels := []string{"Settings"}
	if platform.CanExit {
		labels = append(labels, "Exit")
	}
	s.dotR = max(8*u, min(16*u, s.w/12*0.36))
	bh, gap := quietButtonHeight*u, 12*u
	stack := float64(len(labels))*(bh+gap) - gap
	s.playX = s.x + s.w/2
	var y float64 // top of the button stack
	if portrait {
		y = s.layoutPortrait(a, v, size, stack)
	} else {
		y = s.layoutBeside(a, v, size, stack, col)
	}
	s.playFace = newFace(srcDisplay, s.playR*0.34, 800) // the label grows with the dot
	s.items = s.items[:0]
	buttonW := min(2*s.playR, quietButtonMaxWidth*u)
	for _, l := range labels {
		s.items = append(s.items, homeItem{l, rect{s.playX - buttonW/2, y, buttonW, bh}})
		y += bh + gap
	}

	// The ways to play stand where PLAY and its buttons stood, centred on them.
	labels = append(playModes(), "Back")
	modesH := float64(len(labels))*(bh+gap) - gap
	my := (s.playY - s.playR + y - gap - modesH) / 2
	modeW := min(s.w, max(buttonW, 220*u))
	s.modes = s.modes[:0]
	for _, l := range labels {
		s.modes = append(s.modes, homeItem{l, rect{s.playX - modeW/2, my, modeW, bh}})
		my += bh + gap
	}

	rows := 7
	cols := max(5, min(12, int(math.Round(float64(rows)*s.mat.w/s.mat.h))))
	if s.demo == nil || s.demo.cols != cols {
		s.demo = newDemo(cols, rows, a.rng.Uint64())
	}
	s.demo.layout(s.mat, u)
}

// layoutBeside puts the column left of the mat, centred top to bottom, and returns the top of its button stack. On a
// screen too short for it, the gaps tighten and the PLAY dot shrinks until the column fits.
func (s *homeScene) layoutBeside(a *app, v rect, size, stack, col float64) float64 {
	u := a.u
	gLogo, gDots, gPlay := 30*u, 44*u, 32*u
	rest := func() float64 { return size + gLogo + 2*s.dotR + gDots + gPlay + stack }
	s.playR = max(64*u, min(s.w*0.3, v.h*0.17, 150*u))
	if rest()+2*s.playR > v.h-32*u {
		gLogo, gDots, gPlay = 16*u, 20*u, 20*u
		s.playR = max(40*u, min(s.playR, (v.h-32*u-rest())/2))
	}
	s.logoY = v.y + (v.h-rest()-2*s.playR)/2 + size/2
	s.dotsY = s.logoY + size/2 + gLogo + s.dotR
	s.playY = s.dotsY + s.dotR + gDots + s.playR
	s.mat = rect{v.x + col, v.y + 24*u, v.w - col - 24*u, v.h - 48*u}
	return s.playY + s.playR + gPlay
}

// layoutPortrait opens the column around the mat: the wordmark and dots at the top, PLAY and the buttons at the
// bottom within thumb reach, and the mat filling the middle. It returns the top of the button stack.
func (s *homeScene) layoutPortrait(a *app, v rect, size, stack float64) float64 {
	u := a.u
	s.playR = max(48*u, min(s.w*0.3, v.h*0.11, 150*u))
	s.logoY = v.y + 32*u + size/2
	s.dotsY = s.logoY + size/2 + 24*u + s.dotR
	stackY := v.bottom() - 24*u - stack
	s.playY = stackY - 24*u - s.playR
	top := s.dotsY + s.dotR + 24*u
	s.mat = rect{v.x + 16*u, top, v.w - 32*u, max(1, s.playY-s.playR-24*u-top)}
	return stackY
}

func (s *homeScene) playRect() rect {
	return rect{s.playX - s.playR, s.playY - s.playR, 2 * s.playR, 2 * s.playR}
}

func (s *homeScene) update(a *app) error {
	s.layout(a)
	s.demo.tick()
	if s.ask != nil {
		s.updateAsk(a)
		return nil
	}
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
	s.demo.b.draw(dst, sc, a.tms(), -1, a.set.Marks)

	for i, l := range logoLetters {
		drawText(dst, l, s.logo, s.x+s.logoX[i], s.logoY, text.AlignStart, sc.ink)
	}
	// The twelve seats, spread edge to edge under the wordmark.
	step := (s.w - 2*s.dotR) / (core.MaxPlayers - 1)
	for p := range core.MaxPlayers {
		seatDot(dst, sc, p, s.x+s.dotR+float64(p)*step, s.dotsY, s.dotR, false, a.set.Marks)
	}

	if s.choosing {
		for _, it := range s.modes {
			style := btnInk
			if it.label == "Back" {
				style = btnTile
			}
			drawButton(dst, a, it.r, it.label, "", style, true)
		}
		if s.ask != nil {
			s.ask.draw(dst, a)
		}
		return
	}
	fill := sc.ink
	if a.over(s.playRect()) {
		fill = sc.inkHi
	}
	fillCircle(dst, s.playX, s.playY, s.playR, fill)
	drawText(dst, "PLAY", s.playFace, s.playX, s.playY-2*u, text.AlignCenter, sc.ground)

	for _, it := range s.items {
		drawButton(dst, a, it.r, it.label, "", btnTile, true)
	}
}

// demo plays a silent four-AI game on a small board, over and over, for the Home screen.
type demo struct {
	cols, rows int
	game       *core.Game
	b          *board
	bot        ai.Bot
	rng        core.RNG
	wait       int
	area       rect
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
	d.game, d.b = g, newBoard(g)
	for range 3 * len(demoStrategies) {
		d.step(false)
	}
	d.b.sync(g)
	if d.u != 0 {
		d.b.layout(d.area, d.u)
	}
	d.wait = 30
}

func (d *demo) layout(area rect, u float64) {
	d.area, d.u = area, u
	d.b.layout(area, u)
}

func (d *demo) step(animate bool) {
	p := d.game.Turn
	cell := d.bot.Choose(d.game, demoStrategies[p], d.rng.Uint64(), 120)
	waves, err := d.game.Apply(cell)
	if err != nil {
		return
	}
	if animate {
		d.b.start(cell, int8(p), waves, nil)
	}
}

func (d *demo) tick() {
	d.b.tick(nil)
	switch {
	case d.b.playing:
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
func playModes() []string { return waysToPlay(runtime.GOOS == "js", origin) }

// waysToPlay is This Device always (alone against bots, or passing it round), Same Wi-Fi in every app (a browser can
// neither host nor reach a LAN host), and Online where the build has a server.
func waysToPlay(browser bool, origin string) []string {
	modes := []string{"This Device"}
	if !browser {
		modes = append(modes, "Same Wi-Fi")
	}
	if origin != "" {
		modes = append(modes, "Online")
	}
	return modes
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
