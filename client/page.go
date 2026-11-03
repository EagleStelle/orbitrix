package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// page is the frame Setup sets out, shared by the online screens: a Back pill and a title across the top, a body that
// scrolls when its blocks are taller than the screen, and an optional full-width action anchored to the bottom.
type page struct {
	lv             rect
	lu             float64
	pad, gap, bp   float64 // outer margin, gutter, block padding
	back           rect
	titleX, titleY float64
	action         rect // zero when the page has none
	view           scrollView
}

// frame lays the frame out for the safe area and reports whether it changed, so the page lays out its blocks again.
func (p *page) frame(a *app, action bool) bool {
	v, u := a.safe(), a.u
	if v == p.lv && u == p.lu {
		return false
	}
	p.lv, p.lu = v, u
	p.pad, p.gap, p.bp = 20*u, 16*u, 20*u
	if v.w < 1200*u {
		p.pad, p.gap, p.bp = 16*u, 12*u, 16*u
	}
	w := v.w - 2*p.pad
	p.back = backButton(v.x+p.pad, v.y+p.pad, w, u)
	p.titleX, p.titleY = p.back.right()+16*u, p.back.cy()
	top, bottom := p.back.bottom()+p.gap, v.bottom()-p.pad
	p.action = rect{}
	if action {
		p.action = rect{v.x + p.pad, bottom - 52*u, w, 52 * u}
		bottom = p.action.y - p.gap
	}
	p.view.viewport = rect{v.x + p.pad, top, w, max(1, bottom-top)}
	return true
}

// content sets the body's height and returns its top, scrolled.
func (p *page) content(h float64) float64 {
	p.view.set(p.view.viewport, h)
	return p.view.viewport.y - p.view.offset
}

// centred is content for a column of blocks: its top sits in the middle of the viewport while the body fits.
func (p *page) centred(h float64) float64 { return p.content(h) + max(0, p.view.viewport.h-h)/2 }

// scrolled follows the wheel and touch drags and reports whether the body moved.
func (p *page) scrolled(a *app) bool { return p.view.update(a) }

// inBody reports whether the pointer is over the visible body, where its controls take clicks.
func (p *page) inBody(a *app) bool { return p.view.viewport.contains(a.in.x, a.in.y) }

// backed reports Esc, Android's back button or a click on Back.
func (p *page) backed(a *app) bool { return a.in.esc || a.clicked(compactHit(p.back, a.u)) }

// draw draws Back and the title; the page draws its body into body(dst).
func (p *page) draw(dst *ebiten.Image, a *app, title string) {
	drawBack(dst, a, p.back)
	face := a.f.title
	if textWidth(title, face) > p.view.viewport.right()-p.titleX {
		face = a.f.heading
	}
	drawText(dst, title, face, p.titleX, p.titleY, text.AlignStart, a.sc.ink)
	p.view.drawScrollbar(dst, a)
}

// body is dst clipped to the scrolling body.
func (p *page) body(dst *ebiten.Image) *ebiten.Image { return p.view.clip(dst) }
