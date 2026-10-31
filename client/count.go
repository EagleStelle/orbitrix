package client

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// orbCountKey describes the exact value and space available to an upright count. Zero slices means the portrait dot.
type orbCountKey struct {
	value             string
	x, y, r, u, angle float64
	slices            int
	marked            bool
	face              *text.GoTextFace
}

type orbCounter struct {
	key         orbCountKey
	bounds      rect
	x, y, scale float64
}

// contains keeps every glyph corner inside its circle and, on the wheel, clear of slice dividers and the rim pointer.
func (k orbCountKey) contains(x, y float64) bool {
	dx, dy := x-k.x, y-k.y
	margin := 2 * k.u
	if k.slices > 0 {
		margin = 8 * k.u
	}
	if math.Hypot(dx, dy) > k.r-margin {
		return false
	}
	if k.slices > 0 {
		half := math.Pi / float64(k.slices)
		lo, hi := k.angle-half, k.angle+half
		return math.Cos(lo)*dy-math.Sin(lo)*dx >= 2*k.u &&
			dx*math.Sin(hi)-dy*math.Cos(hi) >= 2*k.u
	}
	if k.marked && dy < -5*k.u {
		return false
	}
	return true
}

func (c *orbCounter) layout(k orbCountKey) {
	if c.key == k {
		return
	}
	c.key = k
	glyphs := labelGlyphs(k.value, k.face, text.AlignCenter)
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, g := range glyphs {
		if g.Image != nil {
			left, top = min(left, g.X), min(top, g.Y)
			right, bottom = max(right, g.X+float64(g.Image.Bounds().Dx())), max(bottom, g.Y+float64(g.Image.Bounds().Dy()))
		}
	}
	g := rect{left, top, right - left, bottom - top}
	x, y := k.x, k.y
	if k.slices > 0 {
		x += 0.74 * k.r * math.Cos(k.angle)
		y += 0.74 * k.r * math.Sin(k.angle)
	} else if k.marked {
		y += 3 * k.u
	}
	// Find the largest uniform glyph scale; count changes never resize the wheel or dot.
	fits := func(s float64) bool {
		w, h := g.w*s/2, g.h*s/2
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
	c.scale = lo
	c.bounds = rect{x - g.w*lo/2, y - g.h*lo/2, g.w * lo, g.h * lo}
	c.x, c.y = x-g.cx()*lo, y-g.cy()*lo
}

func (p *playScene) drawOrbCount(dst *ebiten.Image, a *app, player int, x, y, r float64, slices int) {
	value := p.orbS[player]
	if value == "" {
		value = "0"
	}
	face := a.f.bodyBold
	if p.compact && slices > 0 {
		face = a.f.labelBold
	}
	k := orbCountKey{value, x, y, r, a.u, p.wedgeAngle(player), slices, a.set.Marks, face}
	c := &p.orbLabels[player]
	c.layout(k)
	drawScaledText(dst, value, face, c.x, c.y, text.AlignCenter, c.scale, inkOn(a.sc.players[player]))
}

func (p *playScene) drawWheelCounts(dst *ebiten.Image, a *app) {
	for i := range p.seats {
		if !p.alive[i] {
			continue
		}
		if a.set.Marks {
			r := p.spinR * 0.4
			mr := min(p.spinR*0.18, math.Pi*r/float64(len(p.seats)))
			an := p.wedgeAngle(i)
			drawMark(dst, i, p.spinX+r*math.Cos(an), p.spinY+r*math.Sin(an), mr, a.sc.players[i])
		}
		p.drawOrbCount(dst, a, i, p.spinX, p.spinY, p.spinR, len(p.seats))
	}
}

func (p *playScene) drawTurnCount(dst *ebiten.Image, a *app) {
	c := a.sc.players[p.shown]
	fillCircle(dst, p.tokX, p.tokY, p.tokR, c)
	p.drawOrbCount(dst, a, p.shown, p.tokX, p.tokY, p.tokR, 0)
	if a.set.Marks {
		x, y, r := p.tokX, p.tokY-p.tokR+3*a.u, 9*a.u
		fillCircle(dst, x, y, r+2*a.u, a.sc.ground)
		fillCircle(dst, x, y, r, c)
		drawMark(dst, p.shown, x, y, r, c)
	}
}
