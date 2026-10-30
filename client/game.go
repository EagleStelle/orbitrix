// Package client is the Ebitengine game: scenes, rendering, input, audio and settings.
package client

import (
	"math"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/core"
)

// scene is one screen. update runs at 60 Hz; draw once per frame.
type scene interface {
	update(a *app) error
	draw(dst *ebiten.Image, a *app)
}

// app is what every scene shares.
type app struct {
	w, h  float64 // screen size in device pixels
	u     float64 // UI scale: device pixels per logical pixel
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

// input is the pointer state for one update.
type input struct {
	x, y    float64 // device px
	clicked bool    // primary button pressed this update
	down    bool
	wheel   float64
}

func (in *input) read(u float64) {
	x, y := ebiten.CursorPosition()
	in.x, in.y = float64(x)*u, float64(y)*u
	in.clicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	in.down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	_, in.wheel = ebiten.Wheel()
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
	if a.set.Fullscreen {
		ebiten.SetFullscreen(true)
	}
	a.u = 1
	a.f = newFonts(1)
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
	u, sc := g.a.u, g.a.sc
	r := rect{8 * u, 8 * u, textWidth(g.fpsText, g.a.f.small) + 20*u, 24 * u}
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
	return w, h
}
