package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Button styles. Solid ink is reserved for the one primary action on a screen; everything else is a quiet fill that
// contrasts with what the button sits on, and a chosen option is outlined in ink.
type ButtonStyle int

const (
	ButtonInk    ButtonStyle = iota // the primary action
	ButtonTile                      // quiet, on the mat
	ButtonGround                    // quiet, on a tile block
	ButtonChosen                    // the selected option among quiet ones, on a tile block
)

// Quiet buttons share menu proportions; compact navigation controls keep their smaller sizes.
const QuietButtonHeight, QuietButtonMaxWidth = 52, 240

// Compact controls match Back visually while keeping a full touch target.
const CompactButtonHeight, CompactButtonTightHeight, CompactButtonTarget = 40, 28, 44

// compactHit is r's touch target: r itself, grown to the compact target height when its face is shorter.
func CompactHit(r Rect, u float64) Rect {
	h := max(r.H, CompactButtonTarget*u)
	return Rect{r.X, r.Cy() - h/2, r.W, h}
}

// backButton is the compact Back pill with its top left at (x, y); on pages narrower than 540 px it shows the arrow
// alone.
func BackButton(x, y, pageW, u float64) Rect {
	r := Rect{x, y, 112 * u, CompactButtonHeight * u}
	if pageW < 540*u {
		r.W = CompactButtonTarget * u
	}
	return r
}

// drawBack draws a Back pill from backButton.
func DrawBack(dst *ebiten.Image, a Context, r Rect) {
	u := a.Scale
	ink := DrawButtonSurface(dst, a, r, ButtonTile, true, a.Over(CompactHit(r, u)))
	arrowLeft(dst, r.X+22*u, r.Cy(), 14*u, 2.5*u, ink)
	if r.W > CompactButtonTarget*u {
		DrawText(dst, "Back", a.Fonts.LabelBold, r.X+42*u, r.Cy(), text.AlignStart, ink)
	}
}

// drawButton draws a pill button with label and an optional note after it in the small face. A disabled button
// ignores hover.
func DrawButton(dst *ebiten.Image, a Context, r Rect, label, note string, style ButtonStyle, enabled bool) {
	ink := DrawButtonSurface(dst, a, r, style, enabled, a.Over(CompactHit(r, a.Scale)))
	if style == ButtonInk {
		// The primary action speaks in the display face.
		DrawText(dst, label, a.Fonts.Heading, r.Cx(), r.Cy(), text.AlignCenter, ink)
		return
	}
	PillLabel(dst, a, r, label, note, ink)
}

// drawButtonSurface also serves selectors and icon buttons, which supply their own contents.
func DrawButtonSurface(dst *ebiten.Image, a Context, r Rect, style ButtonStyle, enabled, hot bool) color.RGBA {
	fill, ink := ButtonColors(a.Palette, style, enabled, hot)
	FillPill(dst, r, fill)
	if style == ButtonChosen {
		StrokeRound(dst, r, r.H/2, 2.5*a.Scale, a.Palette.Ink)
	}
	return ink
}

func ButtonColors(sc *Palette, style ButtonStyle, enabled, hot bool) (fill, ink color.RGBA) {
	hot = enabled && hot
	switch style {
	case ButtonInk:
		fill, ink = sc.Ink, sc.Ground
		if hot {
			fill = sc.InkHi
		}
	case ButtonTile:
		fill, ink = sc.Tile, sc.Ink
		if hot {
			fill = sc.TileHi
		}
	default:
		fill, ink = sc.Ground, sc.Ink
		if hot {
			fill = sc.TileHi
		}
	}
	if !enabled {
		fill, ink = sc.Tile, sc.Dim
		if style == ButtonTile {
			fill = Mix(sc.Tile, sc.Ground, 0.5)
		}
	}
	return fill, ink
}

// pillLabel centres label in r, followed by an optional note (a preset's dimensions) in the small face.
func PillLabel(dst *ebiten.Image, a Context, r Rect, label, note string, ink color.RGBA) {
	f := a.Fonts
	if note == "" {
		DrawText(dst, label, f.LabelBold, r.Cx(), r.Cy(), text.AlignCenter, ink)
		return
	}
	gap := 8 * a.Scale
	lw, nw := TextWidth(label, f.LabelBold), TextWidth(note, f.Small)
	x := r.Cx() - (lw+gap+nw)/2
	DrawText(dst, label, f.LabelBold, x, r.Cy(), text.AlignStart, ink)
	DrawText(dst, note, f.Small, x+lw+gap, r.Cy(), text.AlignStart, ink)
}

// chipHeight is a status chip's height in logical px; rows too short for it pass a smaller one.
const ChipHeight = 24

// A chip's kind codes its state by weight: solid ink, a faded fill, or a dim outline. The one hue it takes is the
// danger ink, for a state that is bad news for a player.
type ChipKind uint8

const (
	ChipOn     ChipKind = iota // solid ink: the way is open (Open, Ready, Joining)
	ChipBusy                   // faded fill: something is under way (Playing)
	ChipOff                    // dim outline: nobody is in it or can get in (Full, Away, an empty seat)
	ChipDanger                 // danger outline: a player is out, or holding the start up (Out, Not ready)
)

// chip is a status word drawn as a small pill at the end of a row.
type Chip struct {
	Word string
	Kind ChipKind
}

// chipWidth is how wide c's pill is at height h.
func ChipWidth(a Context, word string, h float64) float64 {
	return TextWidth(word, a.Fonts.SmallBold) + 0.8*h
}

// drawChip draws c as a pill h tall, its right end at right and centred on cy.
func DrawChip(dst *ebiten.Image, a Context, c Chip, right, cy, h float64) {
	sc := a.Palette
	width := ChipWidth(a, c.Word, h)
	r := Rect{right - width, cy - h/2, width, h}
	ink := sc.Ground
	switch c.Kind {
	case ChipOn:
		FillPill(dst, r, sc.Ink)
	case ChipBusy:
		FillPill(dst, r, sc.Dim)
	case ChipDanger:
		StrokeRound(dst, r, h/2, 1.5*a.Scale, sc.Danger)
		ink = sc.Danger
	default:
		StrokeRound(dst, r, h/2, 1.5*a.Scale, sc.Dim)
		ink = sc.Dim
	}
	DrawText(dst, c.Word, a.Fonts.SmallBold, r.Cx(), r.Cy(), text.AlignCenter, ink)
}

// arrowLeft draws a back arrow centred at (x, y), s wide.
func arrowLeft(dst *ebiten.Image, x, y, s, w float64, c color.RGBA) {
	tip, h := x-s/2, s*0.4
	Line(dst, x+s/2, y, tip, y, w, c)
	Line(dst, tip+h, y-h, tip, y, w, c)
	Line(dst, tip+h, y+h, tip, y, w, c)
}

// hamburger draws the Menu glyph, three lines s wide, centred at (x, y).
func Hamburger(dst *ebiten.Image, x, y, s, w float64, c color.RGBA) {
	for k := -1.0; k <= 1; k++ {
		Line(dst, x-s/2, y+k*s*0.375, x+s/2, y+k*s*0.375, w, c)
	}
}
