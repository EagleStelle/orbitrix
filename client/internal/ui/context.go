// Package ui provides scene-independent drawing, controls and layout primitives.
// Draw calls share caches and scratch state and belong on the game goroutine.
package ui

// Context supplies presentation and pointer data without application state.
type Context struct {
	Scale   float64
	Fonts   *Fonts
	Palette *Palette
	Input   Input
}

type Input struct {
	X, Y                        float64
	Pressed, Down, Finger       bool
	WheelX, Wheel, DragX, DragY float64
}

func (c Context) Over(r Rect) bool { return r.Contains(c.Input.X, c.Input.Y) }
