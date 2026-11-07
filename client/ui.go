package client

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Button styles. Solid ink is reserved for the one primary action on a screen; everything else is a quiet fill that
// contrasts with what the button sits on, and a chosen option is outlined in ink.
type btnStyle int

const (
	btnInk    btnStyle = iota // the primary action
	btnTile                   // quiet, on the mat
	btnGround                 // quiet, on a tile block
	btnChosen                 // the selected option among quiet ones, on a tile block
)

// Quiet buttons share menu proportions; compact navigation controls keep their smaller sizes.
const quietButtonHeight, quietButtonMaxWidth = 52, 240

// Compact controls match Back visually while keeping a full touch target.
const compactButtonHeight, compactButtonTightHeight, compactButtonTarget = 40, 28, 44

// compactHit is r's touch target: r itself, grown to the compact target height when its face is shorter.
func compactHit(r rect, u float64) rect {
	h := max(r.h, compactButtonTarget*u)
	return rect{r.x, r.cy() - h/2, r.w, h}
}

// scrollView clips overflowing content and owns a touch drag from its starting viewport.
type scrollView struct {
	horizontal     bool
	viewport       rect
	height, offset float64
	dragging       bool
	content, of    *ebiten.Image
	clipped        rect
}

func (s *scrollView) maxScroll() float64 {
	extent := s.viewport.h
	if s.horizontal {
		extent = s.viewport.w
	}
	return max(0, s.height-extent)
}

func (s *scrollView) set(v rect, height float64) {
	s.viewport, s.height = v, height
	s.offset = max(0, min(s.maxScroll(), s.offset))
}

func (s *scrollView) update(a *app) bool {
	in := &a.in
	if in.pressed {
		s.dragging = in.finger && s.viewport.contains(in.x, in.y)
	}
	if !in.down {
		s.dragging = false
	}
	before := s.offset
	if s.viewport.contains(in.x, in.y) {
		wheel := in.wheel
		if s.horizontal && in.wheelX != 0 {
			wheel = in.wheelX
		}
		s.offset -= wheel * (compactButtonTarget + 8) * a.u
	}
	if s.dragging {
		if s.horizontal {
			s.offset -= in.dragX
		} else {
			s.offset -= in.dragY
		}
	}
	s.offset = max(0, min(s.maxScroll(), s.offset))
	return s.offset != before
}

func (s *scrollView) reveal(r rect) {
	if s.horizontal {
		if r.x < s.viewport.x {
			s.offset -= s.viewport.x - r.x
		} else if r.right() > s.viewport.right() {
			s.offset += r.right() - s.viewport.right()
		}
		s.offset = max(0, min(s.maxScroll(), s.offset))
		return
	}
	if r.y < s.viewport.y {
		s.offset -= s.viewport.y - r.y
	} else if r.bottom() > s.viewport.bottom() {
		s.offset += r.bottom() - s.viewport.bottom()
	}
	s.offset = max(0, min(s.maxScroll(), s.offset))
}

func (s *scrollView) clip(dst *ebiten.Image) *ebiten.Image {
	if s.of != dst || s.clipped != s.viewport {
		s.of, s.clipped = dst, s.viewport
		v := s.viewport
		s.content = dst.SubImage(image.Rect(int(v.x), int(v.y), int(v.right()), int(v.bottom()))).(*ebiten.Image)
	}
	return s.content
}

func (s *scrollView) drawScrollbar(dst *ebiten.Image, a *app) {
	if s.maxScroll() == 0 {
		return
	}
	v, u := s.viewport, a.u
	if s.horizontal {
		w := min(v.w, max(24*u, v.w*v.w/s.height))
		fillRound(dst, rect{v.x + (v.w-w)*s.offset/s.maxScroll(), v.bottom() - 2*u, w, 2 * u}, u, a.sc.dim)
		return
	}
	h := min(v.h, max(24*u, v.h*v.h/s.height))
	fillRound(dst, rect{v.right() + 5*u, v.y + (v.h-h)*s.offset/s.maxScroll(), 3 * u, h}, 1.5*u, a.sc.dim)
}

// backButton is the compact Back pill with its top left at (x, y); on pages narrower than 540 px it shows the arrow
// alone.
func backButton(x, y, pageW, u float64) rect {
	r := rect{x, y, 112 * u, compactButtonHeight * u}
	if pageW < 540*u {
		r.w = compactButtonTarget * u
	}
	return r
}

// drawBack draws a Back pill from backButton.
func drawBack(dst *ebiten.Image, a *app, r rect) {
	u := a.u
	ink := drawButtonSurface(dst, a, r, btnTile, true, a.over(compactHit(r, u)))
	arrowLeft(dst, r.x+22*u, r.cy(), 14*u, 2.5*u, ink)
	if r.w > compactButtonTarget*u {
		drawText(dst, "Back", a.f.labelBold, r.x+42*u, r.cy(), text.AlignStart, ink)
	}
}

// drawButton draws a pill button with label and an optional note after it in the small face. A disabled button
// ignores hover.
func drawButton(dst *ebiten.Image, a *app, r rect, label, note string, style btnStyle, enabled bool) {
	ink := drawButtonSurface(dst, a, r, style, enabled, a.over(compactHit(r, a.u)))
	if style == btnInk {
		// The primary action speaks in the display face.
		drawText(dst, label, a.f.heading, r.cx(), r.cy(), text.AlignCenter, ink)
		return
	}
	pillLabel(dst, a, r, label, note, ink)
}

// drawButtonSurface also serves selectors and icon buttons, which supply their own contents.
func drawButtonSurface(dst *ebiten.Image, a *app, r rect, style btnStyle, enabled, hot bool) color.RGBA {
	fill, ink := buttonColors(a.sc, style, enabled, hot)
	fillPill(dst, r, fill)
	if style == btnChosen {
		strokeRound(dst, r, r.h/2, 2.5*a.u, a.sc.ink)
	}
	return ink
}

func buttonColors(sc *scheme, style btnStyle, enabled, hot bool) (fill, ink color.RGBA) {
	hot = enabled && hot
	switch style {
	case btnInk:
		fill, ink = sc.ink, sc.ground
		if hot {
			fill = sc.inkHi
		}
	case btnTile:
		fill, ink = sc.tile, sc.ink
		if hot {
			fill = sc.tileHi
		}
	default:
		fill, ink = sc.ground, sc.ink
		if hot {
			fill = sc.tileHi
		}
	}
	if !enabled {
		fill, ink = sc.tile, sc.dim
		if style == btnTile {
			fill = mix(sc.tile, sc.ground, 0.5)
		}
	}
	return fill, ink
}

// pillLabel centres label in r, followed by an optional note (a preset's dimensions) in the small face.
func pillLabel(dst *ebiten.Image, a *app, r rect, label, note string, ink color.RGBA) {
	f := &a.f
	if note == "" {
		drawText(dst, label, f.labelBold, r.cx(), r.cy(), text.AlignCenter, ink)
		return
	}
	gap := 8 * a.u
	lw, nw := textWidth(label, f.labelBold), textWidth(note, f.small)
	x := r.cx() - (lw+gap+nw)/2
	drawText(dst, label, f.labelBold, x, r.cy(), text.AlignStart, ink)
	drawText(dst, note, f.small, x+lw+gap, r.cy(), text.AlignStart, ink)
}

// chipHeight is a status chip's height in logical px; rows too short for it pass a smaller one.
const chipHeight = 24

// A chip's kind codes its state by weight: solid ink, a faded fill, or a dim outline. The one hue it takes is the
// danger ink, for a state that is bad news for a player.
type chipKind uint8

const (
	chipOn     chipKind = iota // solid ink: the way is open (Open, Ready, Joining)
	chipBusy                   // faded fill: something is under way (Playing)
	chipOff                    // dim outline: nobody is in it or can get in (Full, Away, an empty seat)
	chipDanger                 // danger outline: a player is out, or holding the start up (Out, Not ready)
)

// chip is a status word drawn as a small pill at the end of a row.
type chip struct {
	word string
	kind chipKind
}

// chipWidth is how wide c's pill is at height h.
func chipWidth(a *app, word string, h float64) float64 { return textWidth(word, a.f.smallBold) + 0.8*h }

// drawChip draws c as a pill h tall, its right end at right and centred on cy.
func drawChip(dst *ebiten.Image, a *app, c chip, right, cy, h float64) {
	sc := a.sc
	r := rect{right - chipWidth(a, c.word, h), cy - h/2, chipWidth(a, c.word, h), h}
	ink := sc.ground
	switch c.kind {
	case chipOn:
		fillPill(dst, r, sc.ink)
	case chipBusy:
		fillPill(dst, r, sc.dim)
	case chipDanger:
		strokeRound(dst, r, h/2, 1.5*a.u, sc.danger)
		ink = sc.danger
	default:
		strokeRound(dst, r, h/2, 1.5*a.u, sc.dim)
		ink = sc.dim
	}
	drawText(dst, c.word, a.f.smallBold, r.cx(), r.cy(), text.AlignCenter, ink)
}

// arrowLeft draws a back arrow centred at (x, y), s wide.
func arrowLeft(dst *ebiten.Image, x, y, s, w float64, c color.RGBA) {
	tip, h := x-s/2, s*0.4
	line(dst, x+s/2, y, tip, y, w, c)
	line(dst, tip+h, y-h, tip, y, w, c)
	line(dst, tip+h, y+h, tip, y, w, c)
}

// hamburger draws the Menu glyph, three lines s wide, centred at (x, y).
func hamburger(dst *ebiten.Image, x, y, s, w float64, c color.RGBA) {
	for k := -1.0; k <= 1; k++ {
		line(dst, x-s/2, y+k*s*0.375, x+s/2, y+k*s*0.375, w, c)
	}
}
