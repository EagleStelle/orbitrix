package ui

import (
	"image/color"
	"math"

	"orbitrix/core"
)

// The Party Mat design system (DESIGN.md): a printed party floor mat. Colour belongs to the players; every control
// is ink on vinyl. Flat fills only.

// scheme is one mode of the palette.
type Palette struct {
	Ground  color.RGBA // the mat itself
	Tile    color.RGBA // printed empty cells, setup blocks, quiet controls
	TileHi  color.RGBA // hovered quiet controls
	Ink     color.RGBA // text and primary controls
	InkHi   color.RGBA // hovered primary controls
	Dim     color.RGBA // secondary text, dashed outlines
	Danger  color.RGBA // a destructive choice (Remove player), apart from every player ink
	Pit     color.RGBA // the bottomless dark of a map Hole: deeper than every printed surface
	Players [core.MaxPlayers]color.RGBA
}

const (
	Light = iota
	Dark
)

var Palettes = [2]Palette{
	Light: {
		Ground: hex(0xF4F3F0), Tile: hex(0xE0DED9), TileHi: hex(0xD1CEC8),
		Ink: hex(0x171615), InkHi: hex(0x3B3936), Dim: hex(0x65615B), Danger: hex(0xC21D3A), Pit: hex(0x171615),
		Players: [core.MaxPlayers]color.RGBA{
			hex(0xD55E00), hex(0x0072B2), hex(0x009E73), hex(0xE69F00), hex(0xCC79A7), hex(0x56B4E9),
			hex(0xE8D21D), hex(0x2E2C2A), hex(0x901CFC), hex(0x8B5A2B), hex(0x440048), hex(0x641404),
		},
	},
	Dark: {
		Ground: hex(0x1C1A18), Tile: hex(0x2D2A27), TileHi: hex(0x3B3733),
		Ink: hex(0xF5F2EE), InkHi: hex(0xD4CEC6), Dim: hex(0xA39C92), Danger: hex(0xFF6B7A), Pit: hex(0x000000),
		Players: [core.MaxPlayers]color.RGBA{
			hex(0xE86A1A), hex(0x3A9BE0), hex(0x1FB88C), hex(0xF0AA1A), hex(0xDD8DB8), hex(0x7CC6F0),
			hex(0xF0E442), hex(0xF4F4F4), hex(0x844CFC), hex(0xC98B4E), hex(0xB4FC84), hex(0xB85450),
		},
	},
}

func hex(v uint32) color.RGBA { return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255} }

// mix blends a towards b by f.
func Mix(a, b color.RGBA, f float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-f) + float64(y)*f)) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// withAlpha returns c at alpha a, premultiplied as color.RGBA requires.
func WithAlpha(c color.RGBA, a float64) color.RGBA {
	m := func(v uint8) uint8 { return uint8(float64(v) * a) }
	return color.RGBA{m(c.R), m(c.G), m(c.B), uint8(255 * a)}
}

// luminance is the WCAG relative luminance of c.
func Luminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

var (
	nearBlack = hex(0x171615)
	PureWhite = hex(0xFFFFFF)
)

// inkOn returns near-black or white, whichever contrasts more with c.
func InkOn(c color.RGBA) color.RGBA {
	l := Luminance(c)
	if (l+0.05)/(Luminance(nearBlack)+0.05) > 1.05/(l+0.05) {
		return nearBlack
	}
	return PureWhite
}
