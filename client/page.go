package client

import (
	"orbitrix/client/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// page is the frame Setup sets out, shared by the online screens: a Back pill and a title across the top, a body that
// scrolls when its blocks are taller than the screen, and an optional full-width action anchored to the bottom.
type page struct {
	lv             ui.Rect
	lu             float64
	pad, gap, bp   float64 // outer margin, gutter, block padding
	back           ui.Rect
	titleX, titleY float64
	action         ui.Rect // zero when the page has none
	view           ui.Scroll
}

// frame lays the frame out for the safe area and reports whether it changed, so the page lays out its blocks again.
func (p *page) frame(a *app, action bool) bool {
	v, u := a.safe(), a.u
	if v == p.lv && u == p.lu {
		return false
	}
	p.lv, p.lu = v, u
	p.pad, p.gap, p.bp = 20*u, 16*u, 20*u
	if v.W < 1200*u {
		p.pad, p.gap, p.bp = 16*u, 12*u, 16*u
	}
	w := v.W - 2*p.pad
	p.back = ui.BackButton(v.X+p.pad, v.Y+p.pad, w, u)
	p.titleX, p.titleY = p.back.Right()+16*u, p.back.Cy()
	top, bottom := p.back.Bottom()+p.gap, v.Bottom()-p.pad
	p.action = ui.Rect{}
	if action {
		p.action = ui.Rect{X: v.X + p.pad, Y: bottom - 52*u, W: w, H: 52 * u}
		bottom = p.action.Y - p.gap
	}
	p.view.Viewport = ui.Rect{X: v.X + p.pad, Y: top, W: w, H: max(1, bottom-top)}
	return true
}

// content sets the body's height and returns its top, scrolled.
func (p *page) content(h float64) float64 {
	p.view.Set(p.view.Viewport, h)
	return p.view.Viewport.Y - p.view.Offset
}

// centred is content for a column of blocks: its top sits in the middle of the viewport while the body fits.
func (p *page) centred(h float64) float64 { return p.content(h) + max(0, p.view.Viewport.H-h)/2 }

// scrolled follows the wheel and touch drags and reports whether the body moved.
func (p *page) scrolled(a *app) bool { return p.view.Update(a.ui()) }

// inBody reports whether the pointer is over the visible body, where its controls take clicks.
func (p *page) inBody(a *app) bool { return p.view.Viewport.Contains(a.in.x, a.in.y) }

// backed reports Esc, Android's back button or a click on Back.
func (p *page) backed(a *app) bool { return a.in.esc || a.clicked(ui.CompactHit(p.back, a.u)) }

// draw draws Back and the title; the page draws its body into body(dst).
func (p *page) draw(dst *ebiten.Image, a *app, title string) {
	ui.DrawBack(dst, a.ui(), p.back)
	face := a.f.Title
	if ui.TextWidth(title, face) > p.view.Viewport.Right()-p.titleX {
		face = a.f.Heading
	}
	ui.DrawText(dst, title, face, p.titleX, p.titleY, text.AlignStart, a.sc.Ink)
	p.view.DrawScrollbar(dst, a.ui())
}

// body is dst clipped to the scrolling body.
func (p *page) body(dst *ebiten.Image) *ebiten.Image { return p.view.Clip(dst) }
