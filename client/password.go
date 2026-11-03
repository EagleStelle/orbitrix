package client

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// maxPassword caps what the field takes; the server's password is at least 20 characters.
const maxPassword = 128

// passwordScene asks for the host password the server wants, typed on the device's own keyboard. Open room goes back
// to Setup, which tries it and saves it once it works.
type passwordScene struct {
	page
	setup *setupScene
	pw    *field
	show  bool
	wrong bool // the server refused the last one

	loff      float64
	block     rect
	box, eye  rect
	statusY   float64
	shown     string // the box's text, for drawn edits and shownShow
	drawn     int
	shownShow bool
}

// newPassword opens the page with its field taking typing: the page is the field, so its keyboard opens with it.
func newPassword(setup *setupScene, wrong bool) *passwordScene {
	s := &passwordScene{setup: setup, wrong: wrong, pw: passwordField(), drawn: -1}
	s.pw.focused = true
	return s
}

func (s *passwordScene) layout(a *app) {
	if !s.frame(a, true) && s.view.offset == s.loff {
		return
	}
	u, bp := a.u, s.bp
	v := s.view.viewport
	w := min(v.w, 640*u)
	h := bp + 32*u + 52*u + 40*u + bp
	y := s.centred(h)
	s.loff = s.view.offset
	s.block = rect{v.cx() - w/2, y, w, h}
	s.box = rect{s.block.x + bp, y + bp + 32*u, w - 2*bp - 8*u - 92*u, 52 * u}
	s.eye = rect{s.box.right() + 8*u, s.box.cy() - compactButtonHeight*u/2, 92 * u, compactButtonHeight * u}
	s.statusY = s.box.bottom() + 20*u
	s.pw.caret = image.Rect(int(s.box.x), int(s.box.y), int(s.box.right()), int(s.box.bottom()))
}

func (s *passwordScene) update(a *app) error {
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	if s.backed(a) {
		a.next = s.setup
		return nil
	}
	in := s.inBody(a)
	if in {
		s.pw.tap(a, rect{s.box.x, s.box.y, s.eye.right() - s.box.x, s.box.h}) // Show keeps the keyboard open
	}
	edits := s.pw.edits
	enter := s.pw.update(a)
	if s.pw.edits != edits {
		s.wrong = false
	}
	if in && a.clicked(compactHit(s.eye, a.u)) {
		s.show = !s.show
	}
	if enter || a.clicked(s.action) {
		if pw := s.pw.value(); pw != "" {
			a.set.Password = pw
			a.next = s.setup
			s.setup.open(a, pw)
		}
	}
	return nil
}

func (s *passwordScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	s.page.draw(dst, a, "Host password")
	b := s.body(dst)
	fillRound(b, s.block, 24*u, sc.tile)
	drawText(b, "Password", f.heading, s.block.x+s.bp, s.block.y+s.bp+10*u, text.AlignStart, sc.ink)

	fillPill(b, s.box, sc.ground)
	if s.pw.edits != s.drawn || s.show != s.shownShow {
		s.drawn, s.shownShow = s.pw.edits, s.show
		r := s.pw.shown
		if !s.show {
			r = []rune(strings.Repeat("•", len(r)))
		}
		// The box shows its end, where typing happens, when the password is wider than it.
		for len(r) > 1 && textWidth(string(r), f.bodyBold) > s.box.w-40*u {
			r = r[1:]
		}
		s.shown = string(r)
	}
	drawText(b, s.shown, f.bodyBold, s.box.x+20*u, s.box.cy(), text.AlignStart, sc.ink)
	if s.pw.focused { // the chosen outline, and a caret where the next character goes
		strokeRound(b, s.box, s.box.h/2, 2.5*u, sc.ink)
		x := s.box.x + 20*u + textWidth(s.shown, f.bodyBold) + 2*u
		fillRound(b, rect{x, s.box.cy() - 12*u, 2 * u, 24 * u}, u, sc.ink)
	}
	label := "Show"
	if s.show {
		label = "Hide"
	}
	drawButton(b, a, s.eye, label, "", btnGround, true)
	if s.wrong {
		drawText(b, "Wrong password", f.label, s.block.cx(), s.statusY, text.AlignCenter, sc.ink)
	}
	drawButton(dst, a, s.action, "Open room", "", btnInk, len(s.pw.shown) > 0)
}
