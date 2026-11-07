package client

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
)

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

	middle, right bool // held mouse buttons; editor-only temporary tools
	down          bool
	wheelX        float64
	wheel         float64 // wheel notches this update
	dragX         float64
	dragY         float64 // device px a dragging finger moved down this update
	esc           bool    // Esc, or Android's back button

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
	in.wheelX, in.dragX = 0, 0
	in.middle, in.right = false, false
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
	in.middle = ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	in.right = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	mx, my := ebiten.CursorPosition()
	if mx != in.mx || my != in.my || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || in.middle || in.right {
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
	in.wheelX, in.wheel = ebiten.Wheel()
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
		in.dragX, in.dragY = fx-in.x, fy-in.y
	}
	in.x, in.y = fx, fy
}

// hover reports whether the pointer is over r, and marks the pointer as over something clickable.
func (a *app) hover(r ui.Rect) bool {
	if r.Contains(a.in.x, a.in.y) {
		a.pointer = true
		return true
	}
	return false
}

// clicked reports a click on r.
func (a *app) clicked(r ui.Rect) bool { return a.hover(r) && a.in.clicked }

// over reports whether the pointer is over r, for drawing hover states.
func (a *app) over(r ui.Rect) bool { return r.Contains(a.in.x, a.in.y) }

func (a *app) ui() ui.Context {
	return ui.Context{Scale: a.u, Fonts: &a.f, Palette: a.sc, Input: ui.Input{
		X: a.in.x, Y: a.in.y, Pressed: a.in.pressed, Down: a.in.down, Finger: a.in.finger,
		WheelX: a.in.wheelX, Wheel: a.in.wheel, DragX: a.in.dragX, DragY: a.in.dragY,
	}}
}
