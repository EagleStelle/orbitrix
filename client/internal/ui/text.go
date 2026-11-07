package ui

import (
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type labelKey struct {
	s     string
	face  text.Face
	align text.Align
}

const labelCap = 512 // labels kept before the cache starts over; orb counts and new font sizes keep adding them

// labels keeps each label's glyphs, laid out around (0, 0) as text.Draw lays them out before moving them into place,
// so drawing the same text every frame neither shapes it again nor allocates.
var labels = map[labelKey][]text.Glyph{}

// drawText draws s with its horizontal anchor at x (start, centre or end) and its vertical centre at y.
func DrawText(dst *ebiten.Image, s string, face text.Face, x, y float64, align text.Align, c color.RGBA) {
	DrawScaledText(dst, s, face, x, y, align, 1, c)
}

func LabelGlyphs(s string, face text.Face, align text.Align) []text.Glyph {
	k := labelKey{s, face, align}
	glyphs, ok := labels[k]
	if !ok {
		if len(labels) >= labelCap {
			clear(labels)
		}
		glyphs = text.AppendGlyphs(nil, s, face, &text.LayoutOptions{PrimaryAlign: align, SecondaryAlign: text.AlignCenter})
		labels[k] = glyphs
	}
	return glyphs
}

// drawScaledText fits cached glyphs uniformly without creating a font for each changing count.
func DrawScaledText(dst *ebiten.Image, s string, face text.Face, x, y float64, align text.Align, scale float64, c color.RGBA) {
	glyphs := LabelGlyphs(s, face, align)
	for _, g := range glyphs {
		if g.Image == nil {
			continue
		}
		imageOp = ebiten.DrawImageOptions{}
		if scale != 1 {
			imageOp.GeoM.Scale(scale, scale)
			imageOp.Filter = ebiten.FilterLinear
			imageOp.DisableMipmaps = true
		}
		imageOp.GeoM.Translate(g.X*scale+x, g.Y*scale+y)
		Tint(&imageOp.ColorScale, c)
		dst.DrawImage(g.Image, &imageOp)
	}
}

func TextWidth(s string, face text.Face) float64 {
	w, _ := text.Measure(s, face, 0)
	return w
}

// drawMark draws seat p's shape mark centred in a dot of radius r filled with c, in black or white for contrast.
// Numbers holds every byte-sized count so labels never format numbers while drawing.
var Numbers = func() (s [256]string) {
	for i := range s {
		s[i] = strconv.Itoa(i)
	}
	return s
}()
