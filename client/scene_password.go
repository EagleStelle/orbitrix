package client

import (
	"image"
	"strings"

	"orbitrix/client/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// maxPassword caps what the field takes; the server's password is at least 20 characters.
const maxPassword = 128

// passwordScene asks for the host password the server wants, typed on the device's own keyboard. Open room goes back
// to the Room page, which tries it and saves it once it works.
type passwordScene struct {
	page
	room  *setupScene
	pw    *field
	show  bool
	wrong bool // the server refused the last one

	loff      float64
	block     ui.Rect
	box, eye  ui.Rect
	statusY   float64
	shown     string // the box's text, for drawn edits and shownShow
	drawn     int
	shownShow bool
}

// newPassword opens the page with its field taking typing: the page is the field, so its keyboard opens with it.
func newPassword(room *setupScene, wrong bool) *passwordScene {
	s := &passwordScene{room: room, wrong: wrong, pw: passwordField(), drawn: -1}
	s.pw.focused = true
	return s
}

func (s *passwordScene) layout(a *app) {
	if !s.frame(a, true) && s.view.Offset == s.loff {
		return
	}
	u, bp := a.u, s.bp
	v := s.view.Viewport
	w := min(v.W, 640*u)
	h := bp + 32*u + 52*u + 40*u + bp
	y := s.centred(h)
	s.loff = s.view.Offset
	s.block = ui.Rect{X: v.Cx() - w/2, Y: y, W: w, H: h}
	s.box = ui.Rect{X: s.block.X + bp, Y: y + bp + 32*u, W: w - 2*bp - 8*u - 92*u, H: 52 * u}
	s.eye = ui.Rect{X: s.box.Right() + 8*u, Y: s.box.Cy() - ui.CompactButtonHeight*u/2, W: 92 * u, H: ui.CompactButtonHeight * u}
	s.statusY = s.box.Bottom() + 20*u
	s.pw.caret = image.Rect(int(s.box.X), int(s.box.Y), int(s.box.Right()), int(s.box.Bottom()))
}

func (s *passwordScene) update(a *app) error {
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	if s.backed(a) {
		a.next = s.room
		return nil
	}
	in := s.inBody(a)
	if in {
		s.pw.tap(a, ui.Rect{X: s.box.X, Y: s.box.Y, W: s.eye.Right() - s.box.X, H: s.box.H}) // Show keeps the keyboard open
	}
	edits := s.pw.edits
	enter := s.pw.update(a)
	if s.pw.edits != edits {
		s.wrong = false
	}
	if in && a.clicked(ui.CompactHit(s.eye, a.u)) {
		s.show = !s.show
	}
	if enter || a.clicked(s.action) {
		if pw := s.pw.value(); pw != "" {
			a.set.Password = pw
			a.next = s.room
			s.room.room.openRoom(a, pw)
		}
	}
	return nil
}

func (s *passwordScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	s.page.draw(dst, a, "Host password")
	b := s.body(dst)
	ui.FillRound(b, s.block, 24*u, sc.Tile)
	ui.DrawText(b, "Password", f.Heading, s.block.X+s.bp, s.block.Y+s.bp+10*u, text.AlignStart, sc.Ink)
	ui.FillPill(b, s.box, sc.Ground)
	if s.pw.edits != s.drawn || s.show != s.shownShow {
		s.drawn, s.shownShow = s.pw.edits, s.show
		r := s.pw.shown
		if !s.show {
			r = []rune(strings.Repeat("•", len(r)))
		}
		// The box shows its end, where typing happens, when the password is wider than it.
		for len(r) > 1 && ui.TextWidth(string(r), f.BodyBold) > s.box.W-40*u {
			r = r[1:]
		}
		s.shown = string(r)
	}
	ui.DrawText(b, s.shown, f.BodyBold, s.box.X+20*u, s.box.Cy(), text.AlignStart, sc.Ink)
	if s.pw.focused {
		ui. // the chosen outline, and a caret where the next character goes
			StrokeRound(b, s.box, s.box.H/2, 2.5*u, sc.Ink)
		x := s.box.X + 20*u + ui.TextWidth(s.shown, f.BodyBold) + 2*u
		ui.FillRound(b, ui.Rect{X: x, Y: s.box.Cy() - 12*u, W: 2 * u, H: 24 * u}, u, sc.Ink)
	}
	label := "Show"
	if s.show {
		label = "Hide"
	}
	ui.DrawButton(b, a.ui(), s.eye, label, "", ui.ButtonGround, true)
	if s.wrong {
		ui.DrawText(b, "Wrong password", f.Label, s.block.Cx(), s.statusY, text.AlignCenter, sc.Ink)
	}
	ui.DrawButton(dst, a.ui(), s.action, "Open room", "", ui.ButtonInk, len(s.pw.shown) > 0)
}
