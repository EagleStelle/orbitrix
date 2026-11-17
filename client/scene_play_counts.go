package client

import (
	"math"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// drawOrbCount draws player's count in the circle of radius r at (x, y): inside their wheel slice, half radians to
// each side of its middle, or filling the portrait dot when half is 0.
func (p *playScene) drawOrbCount(dst *ebiten.Image, a *app, player int, x, y, r, half float64) {
	value := p.orbText(player)
	face := a.f.BodyBold
	if p.compact && half > 0 {
		face = a.f.LabelBold
	}
	k := boardview.CountKey{Value: value, X: x, Y: y, R: r, U: a.u, Angle: p.wedgeAngle(player), Half: half, Marked: a.set.Marks, Face: face}
	c := &p.orbLabels[player]
	c.Layout(k)
	if c.Scale*face.Size < 8*a.u {
		return // a slice too thin for a readable count; the Players dialog lists it
	}
	ui.DrawScaledText(dst, value, face, c.X, c.Y, text.AlignCenter, c.Scale, ui.InkOn(a.sc.Players[player]))
}

func (p *playScene) drawWheelCounts(dst *ebiten.Image, a *app) {
	for i := range p.seats {
		half := math.Pi * p.share[i]
		if !p.alive[i] || half == 0 {
			continue
		}
		if a.set.Marks {
			r := p.spinR * 0.4
			mr := min(p.spinR*0.18, r*half)
			an := p.wedgeAngle(i)
			ui.DrawMark(dst, i, p.spinX+r*math.Cos(an), p.spinY+r*math.Sin(an), mr, a.sc.Players[i])
		}
		p.drawOrbCount(dst, a, i, p.spinX, p.spinY, p.spinR, half)
	}
}

func (p *playScene) drawTurnCount(dst *ebiten.Image, a *app) {
	c := a.sc.Players[p.shown]
	ui.FillCircle(dst, p.tokX, p.tokY, p.tokR, c)
	p.drawOrbCount(dst, a, p.shown, p.tokX, p.tokY, p.tokR, 0)
	if a.set.Marks {
		x, y, r := p.tokX, p.tokY-p.tokR+3*a.u, 9*a.u
		ui.FillCircle(dst, x, y, r+2*a.u, a.sc.Ground)
		ui.FillCircle(dst, x, y, r, c)
		ui.DrawMark(dst, p.shown, x, y, r, c)
	}
}
