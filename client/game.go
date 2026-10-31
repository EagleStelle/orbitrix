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
	f     fonts
	sc    *scheme // current mode's palette
	snd   *sound
	set   *settings
	rng   core.RNG // presentation randomness: seat shuffles, game seeds, AI delays
	in    input
	epoch time.Time

	next    scene // scene to switch to after this update
	pointer bool  // a clickable thing is under the pointer this update
}

// safe is the part of the screen clear of the display's cutout. Every layout goes in it; only fills that cover the
// whole screen (the mat, the pause veil, the winner's flood) reach past it.
func (a *app) safe() rect {
	return rect{a.ins[0], a.ins[1], a.w - a.ins[0] - a.ins[2], a.h - a.ins[1] - a.ins[3]}
}

// portrait reports whether the safe area is taller than wide, where the game puts its bars above and below the
// board and Home stacks its column around the mat.
func (a *app) portrait() bool { v := a.safe(); return v.h > v.w }

// touchSlop is how far (logical px) a finger may move and still tap; further, it drags.
const touchSlop = 10

// offScreen is the pointer position of a lifted finger: it hovers nothing.
const offScreen = -1e9

// input is the pointer for one update: the mouse, or the first finger on a touch screen. A finger clicks when it
// lifts without dragging, so a drag can scroll without pressing what it started on, and it leaves no hover behind.
type input struct {
	x, y    float64 // device px
	pressed bool    // the mouse button or a finger went down this update
	clicked bool    // the mouse button went down, or a finger lifted without dragging
	down    bool
	wheel   float64 // wheel notches this update
	dragY   float64 // device px a dragging finger moved down this update
	esc     bool    // Esc, or Android's back button

	finger         bool // the pointer is a finger, not the mouse
	touch          ebiten.TouchID
	touching, drag bool
	x0, y0         float64 // where the finger went down
	mx, my         int     // the mouse cursor last update, to notice it move
	ids            []ebiten.TouchID
}

// read takes this update's input. Ebitengine reports positions on the screen Layout returned, which is already in
// device px.
func (in *input) read(u float64) {
	in.pressed, in.clicked, in.wheel, in.dragY = false, false, 0, 0
	in.esc = inpututil.IsKeyJustPressed(ebiten.KeyEscape) || platform.TakeBack()
	if in.touching {
		in.readFinger(u)
		return
	}
	if in.ids = inpututil.AppendJustPressedTouchIDs(in.ids[:0]); len(in.ids) > 0 {
		x, y := ebiten.TouchPosition(in.ids[0])
		in.touch, in.finger, in.touching, in.drag = in.ids[0], true, true, false
		in.x, in.y = float64(x), float64(y)
		in.x0, in.y0 = in.x, in.y
		in.pressed, in.down = true, true
		return
	}
	mx, my := ebiten.CursorPosition()
	if mx != in.mx || my != in.my || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		in.finger = false
	}
	in.mx, in.my = mx, my
	if in.finger {
		in.x, in.y, in.down = offScreen, offScreen, false
		return
	}
	in.x, in.y = float64(mx), float64(my)
	in.pressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	in.clicked = in.pressed
	in.down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	_, in.wheel = ebiten.Wheel()
}

// readFinger follows the finger that is down; when it lifts, the click lands where it last was.
func (in *input) readFinger(u float64) {
	if inpututil.IsTouchJustReleased(in.touch) {
		in.touching, in.down, in.clicked = false, false, !in.drag
		return
	}
	x, y := ebiten.TouchPosition(in.touch)
	fx, fy := float64(x), float64(y)
	in.drag = in.drag || math.Hypot(fx-in.x0, fy-in.y0) > touchSlop*u
	if in.drag {
		in.dragY = fy - in.y
	}
	in.x, in.y = fx, fy
}

// hover reports whether the pointer is over r, and marks the pointer as over something clickable.
func (a *app) hover(r rect) bool {
	if r.contains(a.in.x, a.in.y) {
		a.pointer = true
		return true
	}
	return false
}

// clicked reports a click on r.
func (a *app) clicked(r rect) bool { return a.hover(r) && a.in.clicked }

// over reports whether the pointer is over r, for drawing hover states.
func (a *app) over(r rect) bool { return r.contains(a.in.x, a.in.y) }

// tms is the animation clock in milliseconds.
func (a *app) tms() float64 { return float64(time.Since(a.epoch).Microseconds()) / 1000 }

func (a *app) setTheme(t int) {
	a.set.Theme = themeNames[t]
	a.sc = &schemes[t]
}

func (a *app) toggleTheme() {
	t := themeLight
	if a.sc == &schemes[themeLight] {
		t = themeDark
	}
	a.setTheme(t)
	a.set.save()
}

func (a *app) toggleMute() {
	a.set.Muted = !a.set.Muted
	a.snd.muted = a.set.Muted
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
}

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
	a.snd = newSound()
	a.snd.muted = a.set.Muted
	a.snd.volume = float64(a.set.Volume) / 100
	// A browser only grants fullscreen in answer to a click, so the web build starts windowed.
	if a.set.Fullscreen && platform.CanFullscreen && runtime.GOOS != "js" {
		ebiten.SetFullscreen(true)
	}
	a.u = 1
	a.f = newFonts(1)
	a.in.finger = true // no pointer until the mouse moves or a finger lands
	g.scene = newHome()
	return g
}

// Update advances the game by one tick.
func (g *Game) Update() error {
	a := &g.a
	a.in.read(a.u)
	a.pointer = false
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyF3):
		g.ShowFPS = !g.ShowFPS
	case inpututil.IsKeyJustPressed(ebiten.KeyM):
		a.toggleMute()
	}
	if err := g.scene.update(a); err != nil {
		return err
	}
	if a.next != nil {
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
	if shape != g.cursor {
		g.cursor = shape
		ebiten.SetCursorShape(shape)
	}
	return nil
}

// Draw renders the current frame.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(&g.a.sc.ground) // a pointer, so the colour isn't boxed into a new interface value every frame
	g.scene.draw(screen, &g.a)
	if g.ShowFPS {
		g.drawFPS(screen)
	}
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
	r := rect{v.x + 8*u, v.y + 8*u, textWidth(g.fpsText, g.a.f.small) + 20*u, 24 * u}
	fillPill(dst, r, sc.ink)
	drawText(dst, g.fpsText, g.a.f.small, r.x+10*u, r.cy(), text.AlignStart, sc.ground)
}

// Layout draws at the display's full resolution; the UI scales by the device scale factor.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	u := 1.0
	if m := ebiten.Monitor(); m != nil {
		u = m.DeviceScaleFactor()
	}
	if u != g.a.u {
		g.a.u, g.a.f = u, newFonts(u)
	}
	w, h := int(math.Ceil(float64(outsideWidth)*u)), int(math.Ceil(float64(outsideHeight)*u))
	g.a.w, g.a.h = float64(w), float64(h)
	g.a.ins[0], g.a.ins[1], g.a.ins[2], g.a.ins[3] = platform.SafeArea()
	return w, h
}
