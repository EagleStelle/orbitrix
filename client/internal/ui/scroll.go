package ui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// scrollView clips overflowing content and owns a touch drag from its starting viewport.
type Scroll struct {
	Horizontal     bool
	Viewport       Rect
	Height, Offset float64
	Dragging       bool
	content, of    *ebiten.Image
	clipped        Rect
}

func (s *Scroll) MaxScroll() float64 {
	extent := s.Viewport.H
	if s.Horizontal {
		extent = s.Viewport.W
	}
	return max(0, s.Height-extent)
}

func (s *Scroll) Set(v Rect, height float64) {
	s.Viewport, s.Height = v, height
	s.Offset = max(0, min(s.MaxScroll(), s.Offset))
}

func (s *Scroll) Update(a Context) bool {
	in := &a.Input
	if in.Pressed {
		s.Dragging = in.Finger && s.Viewport.Contains(in.X, in.Y)
	}
	if !in.Down {
		s.Dragging = false
	}
	before := s.Offset
	if s.Viewport.Contains(in.X, in.Y) {
		wheel := in.Wheel
		if s.Horizontal && in.WheelX != 0 {
			wheel = in.WheelX
		}
		s.Offset -= wheel * (CompactButtonTarget + 8) * a.Scale
	}
	if s.Dragging {
		if s.Horizontal {
			s.Offset -= in.DragX
		} else {
			s.Offset -= in.DragY
		}
	}
	s.Offset = max(0, min(s.MaxScroll(), s.Offset))
	return s.Offset != before
}

func (s *Scroll) Reveal(r Rect) {
	start, end := r.Y, r.Bottom()
	viewStart, viewEnd := s.Viewport.Y, s.Viewport.Bottom()
	if s.Horizontal {
		start, end = r.X, r.Right()
		viewStart, viewEnd = s.Viewport.X, s.Viewport.Right()
	}
	if start < viewStart {
		s.Offset -= viewStart - start
	} else if end > viewEnd {
		s.Offset += end - viewEnd
	}
	s.Offset = max(0, min(s.MaxScroll(), s.Offset))
}

func (s *Scroll) Clip(dst *ebiten.Image) *ebiten.Image {
	if s.of != dst || s.clipped != s.Viewport {
		s.of, s.clipped = dst, s.Viewport
		v := s.Viewport
		s.content = dst.SubImage(image.Rect(int(v.X), int(v.Y), int(v.Right()), int(v.Bottom()))).(*ebiten.Image)
	}
	return s.content
}

func (s *Scroll) DrawScrollbar(dst *ebiten.Image, a Context) {
	if s.MaxScroll() == 0 {
		return
	}
	v, u := s.Viewport, a.Scale
	if s.Horizontal {
		w := min(v.W, max(24*u, v.W*v.W/s.Height))
		FillRound(dst, Rect{v.X + (v.W-w)*s.Offset/s.MaxScroll(), v.Bottom() - 2*u, w, 2 * u}, u, a.Palette.Dim)
		return
	}
	h := min(v.H, max(24*u, v.H*v.H/s.Height))
	FillRound(dst, Rect{v.Right() + 5*u, v.Y + (v.H-h)*s.Offset/s.MaxScroll(), 3 * u, h}, 1.5*u, a.Palette.Dim)
}
