package client

import (
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
const compactButtonHeight, compactButtonTarget = 40, 44

// compactHit is r's touch target: r itself, grown to the compact target height when its face is shorter.
func compactHit(r rect, u float64) rect {
	h := max(r.h, compactButtonTarget*u)
	return rect{r.x, r.cy() - h/2, r.w, h}
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
