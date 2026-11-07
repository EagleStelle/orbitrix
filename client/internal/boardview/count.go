package boardview

import (
	"math"

	"orbitrix/client/internal/ui"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// orbCountKey describes the exact value and space available to an upright count. Zero slices means the portrait dot.
type CountKey struct {
	Value             string
	X, Y, R, U, Angle float64
	Slices            int
	Marked            bool
	Face              *text.GoTextFace
}

type Counter struct {
	Key         CountKey
	Bounds      ui.Rect
	X, Y, Scale float64
}

// contains keeps every glyph corner inside its circle and, on the wheel, clear of slice dividers and the rim pointer.
func (k CountKey) contains(x, y float64) bool {
	dx, dy := x-k.X, y-k.Y
	margin := 2 * k.U
	if k.Slices > 0 {
		margin = 8 * k.U
	}
	if math.Hypot(dx, dy) > k.R-margin {
		return false
	}
	if k.Slices > 0 {
		half := math.Pi / float64(k.Slices)
		lo, hi := k.Angle-half, k.Angle+half
		return math.Cos(lo)*dy-math.Sin(lo)*dx >= 2*k.U &&
			dx*math.Sin(hi)-dy*math.Cos(hi) >= 2*k.U
	}
	if k.Marked && dy < -5*k.U {
		return false
	}
	return true
}

func (c *Counter) Layout(k CountKey) {
	if c.Key == k {
		return
	}
	c.Key = k
	glyphs := ui.LabelGlyphs(k.Value, k.Face, text.AlignCenter)
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, g := range glyphs {
		if g.Image != nil {
			left, top = min(left, g.X), min(top, g.Y)
			right, bottom = max(right, g.X+float64(g.Image.Bounds().Dx())), max(bottom, g.Y+float64(g.Image.Bounds().Dy()))
		}
	}
	g := ui.Rect{X: left, Y: top, W: right - left, H: bottom - top}
	x, y := k.X, k.Y
	if k.Slices > 0 {
		x += 0.74 * k.R * math.Cos(k.Angle)
		y += 0.74 * k.R * math.Sin(k.Angle)
	} else if k.Marked {
		y += 3 * k.U
	}
	// Find the largest uniform glyph scale; count changes never resize the wheel or dot.
	fits := func(s float64) bool {
		w, h := g.W*s/2, g.H*s/2
		return k.contains(x-w, y-h) && k.contains(x+w, y-h) && k.contains(x-w, y+h) && k.contains(x+w, y+h)
	}
	lo, hi := 1.0, 1.0
	if !fits(1) {
		lo = 0
		for range 16 {
			s := (lo + hi) / 2
			if fits(s) {
				lo = s
			} else {
				hi = s
			}
		}
	}
	c.Scale = lo
	c.Bounds = ui.Rect{X: x - g.W*lo/2, Y: y - g.H*lo/2, W: g.W * lo, H: g.H * lo}
	c.X, c.Y = x-g.Cx()*lo, y-g.Cy()*lo
}
