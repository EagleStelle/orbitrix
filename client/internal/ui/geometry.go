package ui

type Rect struct{ X, Y, W, H float64 }

func (r Rect) Contains(x, y float64) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }
func (r Rect) Cx() float64                { return r.X + r.W/2 }
func (r Rect) Cy() float64                { return r.Y + r.H/2 }
func (r Rect) Right() float64             { return r.X + r.W }
func (r Rect) Bottom() float64            { return r.Y + r.H }
func (r Rect) Inset(d float64) Rect       { return Rect{r.X + d, r.Y + d, r.W - 2*d, r.H - 2*d} }

// ScreenFit is the scale, at most 1, that keeps a w × h px screen within a GPU whose largest image is m px. A smaller
// screen keeps every logical size; the browser or OS scales it up.
func ScreenFit(w, h float64, m int) float64 {
	if m == 0 {
		m = 4096 // before the graphics driver reports its limit; the common floor on phones
	}
	lim := float64(m - 2) // Ebitengine pads every image by a pixel, and Layout rounds the size up
	return min(1, lim/w, lim/h)
}

func AbsInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
