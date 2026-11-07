// Package client is the Ebitengine game: scenes, rendering, input, audio and settings.
package client

import (
	"math"
	"runtime"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/sound"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
)

// scene is one screen. update runs at 60 Hz; draw once per frame.
type scene interface {
	update(a *app) error
	draw(dst *ebiten.Image, a *app)
}

// app is what every scene shares.
type app struct {
	w, h  float64    // screen size in device pixels
	ins   [4]float64 // safe-area insets (left, top, right, bottom) in device pixels
	u     float64    // UI scale: device pixels per logical pixel
	f     ui.Fonts
	sc    *ui.Palette // current mode's palette
	snd   *sound.Sound
	set   *settings
	rng   core.RNG // presentation randomness: seat shuffles, game seeds, AI delays
	in    input
	epoch time.Time

	next       scene    // scene to switch to after this update
	pointer    bool     // a clickable thing is under the pointer this update
	moveCursor bool     // the editor canvas can be panned this update
	net        *session // the online room this player is in, so closing the app can leave it
	typing     *field   // the text field that has the device's keyboard, if any
}

// safe is the part of the screen clear of the display's cutout. Every layout goes in it; only fills that cover the
// whole screen (the mat, the pause veil, the winner's flood) reach past it.
func (a *app) safe() ui.Rect {
	return ui.Rect{X: a.ins[0], Y: a.ins[1], W: a.w - a.ins[0] - a.ins[2], H: a.h - a.ins[1] - a.ins[3]}
}

// portrait reports whether the safe area is taller than wide, where the game puts its bars above and below the
// board and Home stacks its column around the mat.
func (a *app) portrait() bool { v := a.safe(); return v.H > v.W }

// tms is the animation clock in milliseconds.
func (a *app) tms() float64 { return float64(time.Since(a.epoch).Microseconds()) / 1000 }

func (a *app) setTheme(t int) {
	a.set.Theme = themeNames[t]
	a.sc = &ui.Palettes[t]
}

func (a *app) toggleTheme() {
	t := ui.Light
	if a.sc == &ui.Palettes[ui.Light] {
		t = ui.Dark
	}
	a.setTheme(t)
	a.set.save()
}

func (a *app) toggleMute() {
	a.set.Muted = !a.set.Muted
	a.snd.Muted = a.set.Muted
	a.set.save()
}

// Game is the root ebiten.Game shared by every client target.
type Game struct {
	// ShowFPS draws the frame rate and the slowest frame of the last second; F3 toggles it.
	ShowFPS bool

	a                    app
	scene                scene
	cursor               ebiten.CursorShapeType
	lastDraw, frameStart time.Time
	worst                time.Duration
	fpsText              string
	landscape            bool // the screen is held in landscape (platform.Rotate)
	fresh                bool // the screen must be drawn: something moved, or it changed size, since the last Draw
	idle                 int  // ticks since a still screen was last drawn
}

// stillRedraw is how often, in ticks, a still screen redraws with nothing changed: a backstop for a frame the
// platform lost (an Android app back from the background), at a thirtieth of the work.
const stillRedraw = 30

// still reports whether a screen has no animation left to draw.
func still(s scene) bool {
	// Android's GLSurfaceView flips buffers after every frame, even one Ebitengine leaves undrawn, and the undrawn
	// buffer shows an older frame: a still screen there would flicker, so every screen draws every tick.
	if runtime.GOOS == "android" {
		return false
	}
	if bar := sceneSnackbar(s); bar != nil && (bar.ticks < snackIn || bar.timeout > 0 && !bar.expired()) {
		return false
	}
	switch s := s.(type) {
	case *setupScene, *settingsMenu:
		return true
	case *playScene:
		return s.paused && s.net == nil || s.over && !s.b.Playing && s.overT >= wipeTicks
	}
	return false
}

// maxScale caps device px per logical px: denser screens (most phones are 2.6–3.5×) draw at 2× and the OS or browser
// scales the frame up, about 40% fewer pixels to fill on a 2.625× phone for slightly softer text.
const maxScale = 2

// wantsLandscape reports whether s shows a game whose board has more columns than rows, which a phone shows best on
// its side. Settings opened from the pause menu keep the game's orientation.
func wantsLandscape(s scene) bool {
	switch s := s.(type) {
	case *playScene:
		return s.cols > s.rows
	case *settingsMenu:
		return wantsLandscape(s.back)
	}
	return false
}

// New returns the root game, on the Home screen.
func New() *Game {
	g := &Game{}
	a := &g.a
	a.epoch = time.Now()
	a.rng = core.NewRNG(uint64(a.epoch.UnixNano()))
	a.set = loadSettings(&a.rng)
	a.setTheme(themeIndex(a.set.Theme))
	a.snd = sound.New()
	a.snd.Muted = a.set.Muted
	a.snd.Volume = float64(a.set.Volume) / 100
	// A browser only grants fullscreen in answer to a click, so the web build starts windowed.
	if a.set.Fullscreen && platform.CanFullscreen && runtime.GOOS != "js" {
		ebiten.SetFullscreen(true)
	}
	// Draw skips frames with nothing new (between Updates on displays faster than 60 Hz, and still screens nobody
	// touches), and the screen keeps the last one.
	ebiten.SetScreenClearedEveryFrame(false)
	a.u = 1
	a.f = ui.NewFonts(1)
	a.in.finger = true // no pointer until the mouse moves or a finger lands
	switch code := platform.JoinCode(); {
	case platform.MapImport() != "":
		s := newMaps(a, newHome(), nil)
		s.importText = platform.MapImport()
		g.scene = s
	case platform.Invite() != "": // opened from a server-free room's invite link
		g.scene = newReply(platform.Invite())
	case platform.LANHost() != "": // opened from a LAN host's page: its room, and a private one's code
		g.scene = newJoin(a, true, code)
	case code != "" && origin != "": // opened from a /join/ link
		g.scene = newJoin(a, false, code)
	default:
		g.scene = newHome()
	}
	// Closing the window while hosting closes the room first, so the other players hear why.
	ebiten.SetWindowClosingHandled(true)
	return g
}

// Update advances the game by one tick.
func (g *Game) Update() error {
	a := &g.a
	if ebiten.IsWindowBeingClosed() {
		if e, ok := g.scene.(*mapEditor); ok && e.dirty {
			e.recoverSave()
		}
		if a.net != nil {
			a.net.leave(a)
			// shortcut: the players' Bye messages go out on their own goroutines; the app waits a moment for them
			// instead of tracking each send. Track them if players on slow links miss the message.
			time.Sleep(500 * time.Millisecond)
		}
		return ebiten.Termination
	}
	x, y, down := a.in.x, a.in.y, a.in.down
	a.in.read(a.u)
	changed := a.in.x != x || a.in.y != y || a.in.down != down || a.in.clicked || a.in.wheel != 0 || a.in.wheelX != 0 || a.in.esc
	a.pointer, a.moveCursor = false, false
	_, editingMap := g.scene.(*mapEditor)
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyF3):
		g.ShowFPS = !g.ShowFPS
		changed = true
	case a.typing == nil && !editingMap && inpututil.IsKeyJustPressed(ebiten.KeyM): // M types into a field instead
		a.toggleMute()
		changed = true // Settings shows the Sound switch
	}
	animated := !still(g.scene)
	if err := g.scene.update(a); err != nil {
		return err
	}
	g.idle++
	if changed || a.next != nil || g.ShowFPS || animated || !still(g.scene) || g.idle >= stillRedraw {
		g.fresh, g.idle = true, 0
	}
	if a.next != nil {
		if a.typing != nil {
			a.typing.focus(a, false) // the on-screen keyboard closes with its page
		}
		g.scene, a.next = a.next, nil
		if l := wantsLandscape(g.scene); platform.Rotate != nil && l != g.landscape {
			g.landscape = l
			platform.Rotate(l)
		}
	}
	shape := ebiten.CursorShapeDefault
	if a.pointer {
		shape = ebiten.CursorShapePointer
	}
	if a.moveCursor {
		shape = ebiten.CursorShapeMove
	}
	if shape != g.cursor {
		g.cursor = shape
		ebiten.SetCursorShape(shape)
	}
	return nil
}

// Draw renders the current frame, unless nothing has moved since the last one.
func (g *Game) Draw(screen *ebiten.Image) {
	if !g.fresh {
		return
	}
	g.fresh = false
	screen.Fill(&g.a.sc.Ground) // a pointer, so the colour isn't boxed into a new interface value every frame
	g.scene.draw(screen, &g.a)
	if g.ShowFPS {
		g.drawFPS(screen)
	}
	drawSnackbarOverlay(screen, &g.a, g.scene)
}

// drawFPS shows the frame rate and the longest gap between frames over the last second.
func (g *Game) drawFPS(dst *ebiten.Image) {
	now := time.Now()
	if !g.lastDraw.IsZero() {
		g.worst = max(g.worst, now.Sub(g.lastDraw))
	}
	g.lastDraw = now
	if now.Sub(g.frameStart) >= time.Second {
		g.fpsText = strconv.FormatFloat(ebiten.ActualFPS(), 'f', 1, 64) + " FPS, worst " +
			strconv.FormatFloat(float64(g.worst.Microseconds())/1000, 'f', 1, 64) + " ms"
		g.frameStart, g.worst = now, 0
	}
	u, sc, v := g.a.u, g.a.sc, g.a.safe()
	r := ui.Rect{X: v.X + 8*u, Y: v.Y + 8*u, W: ui.TextWidth(g.fpsText, g.a.f.Small) + 20*u, H: 24 * u}
	ui.FillPill(dst, r, sc.Ink)
	ui.DrawText(dst, g.fpsText, g.a.f.Small, r.X+10*u, r.Cy(), text.AlignStart, sc.Ground)
}

// Layout draws at the display's resolution up to maxScale, or as near as the GPU allows (screenFit); the UI scales by
// the device scale factor, capped the same way.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	u := 1.0
	if m := ebiten.Monitor(); m != nil {
		u = m.DeviceScaleFactor()
	}
	k := min(maxScale/u, ui.ScreenFit(float64(outsideWidth)*u, float64(outsideHeight)*u, ebiten.MaxImageSize()))
	u *= k
	if u != g.a.u {
		g.a.u, g.a.f = u, ui.NewFonts(u)
	}
	w, h := int(math.Ceil(float64(outsideWidth)*u)), int(math.Ceil(float64(outsideHeight)*u))
	if float64(w) != g.a.w || float64(h) != g.a.h {
		g.fresh = true // a resized screen starts blank
	}
	g.a.w, g.a.h = float64(w), float64(h)
	l, t, r, b := platform.SafeArea()
	g.a.ins = [4]float64{l * k, t * k, r * k, b * k}
	return w, h
}
