package ui

import "testing"

func TestScrollInputAndReveal(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		s := Scroll{Horizontal: horizontal}
		s.Set(Rect{0, 0, 100, 100}, 400)
		input := Context{Scale: 1, Input: Input{X: 50, Y: 50, Wheel: -1, WheelX: -2}}
		s.Update(input)
		want := float64(CompactButtonTarget + 8)
		if horizontal {
			want *= 2
		}
		if s.Offset != want {
			t.Fatalf("horizontal=%v: wheel offset %g, want %g", horizontal, s.Offset, want)
		}
		s.Offset = 0
		s.Reveal(Rect{160, 160, 20, 20})
		if s.Offset != 80 {
			t.Fatalf("horizontal=%v: reveal offset %g, want 80", horizontal, s.Offset)
		}
		s.Update(Context{Scale: 1, Input: Input{X: 50, Y: 50, Pressed: true, Down: true, Finger: true, DragX: -20, DragY: -20}})
		if !s.Dragging || s.Offset != 100 {
			t.Fatalf("horizontal=%v: drag did not advance the captured scroll", horizontal)
		}
		s.Update(Context{Scale: 1, Input: Input{X: 500, Y: 500, Pressed: true, Down: true, Finger: true, DragX: -20, DragY: -20}})
		if s.Dragging || s.Offset != 100 {
			t.Fatalf("horizontal=%v: a press outside the viewport captured the drag", horizontal)
		}
		s.Set(s.Viewport, 20)
		if s.Offset != 0 || s.MaxScroll() != 0 {
			t.Fatal("shrinking content must clamp the scroll offset")
		}
	}
}
