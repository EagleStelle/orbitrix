package client

import (
	"bytes"
	_ "embed"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/core"
)

// The Party Mat design system (DESIGN.md): a printed party floor mat. Colour belongs to the players; every control
// is ink on vinyl. Flat fills only.

// scheme is one mode of the palette.
type scheme struct {
	ground  color.RGBA // the mat itself
	tile    color.RGBA // printed empty cells, setup blocks, quiet controls
	tileHi  color.RGBA // hovered quiet controls
	ink     color.RGBA // text and primary controls
	inkHi   color.RGBA // hovered primary controls
	dim     color.RGBA // secondary text, dashed outlines
	players [core.MaxPlayers]color.RGBA
}

const (
	themeLight = iota
	themeDark
)

var schemes = [2]scheme{
	themeLight: {
		ground: hex(0xF4F3F0), tile: hex(0xE0DED9), tileHi: hex(0xD1CEC8),
		ink: hex(0x171615), inkHi: hex(0x3B3936), dim: hex(0x65615B),
		players: [core.MaxPlayers]color.RGBA{
			hex(0xD55E00), hex(0x0072B2), hex(0x009E73), hex(0xE69F00), hex(0xCC79A7), hex(0x56B4E9),
			hex(0xE8D21D), hex(0x2E2C2A), hex(0x7B4FD6), hex(0x8B5A2B), hex(0x9BCB00), hex(0xE8418C),
		},
	},
	themeDark: {
		ground: hex(0x1C1A18), tile: hex(0x2D2A27), tileHi: hex(0x3B3733),
		ink: hex(0xF5F2EE), inkHi: hex(0xD4CEC6), dim: hex(0xA39C92),
		players: [core.MaxPlayers]color.RGBA{
			hex(0xE86A1A), hex(0x3A9BE0), hex(0x1FB88C), hex(0xF0AA1A), hex(0xDD8DB8), hex(0x7CC6F0),
			hex(0xF0E442), hex(0xF4F4F4), hex(0x9C7BF0), hex(0xC98B4E), hex(0xB4E61E), hex(0xFF5FA8),
		},
	},
}

func hex(v uint32) color.RGBA { return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255} }

// mix blends a towards b by f.
func mix(a, b color.RGBA, f float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-f) + float64(y)*f)) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// withAlpha returns c at alpha a, premultiplied as color.RGBA requires.
func withAlpha(c color.RGBA, a float64) color.RGBA {
	m := func(v uint8) uint8 { return uint8(float64(v) * a) }
	return color.RGBA{m(c.R), m(c.G), m(c.B), uint8(255 * a)}
}

// luminance is the WCAG relative luminance of c.
func luminance(c color.RGBA) float64 {
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
	pureWhite = hex(0xFFFFFF)
)

// inkOn returns near-black or white, whichever contrasts more with c.
func inkOn(c color.RGBA) color.RGBA {
	l := luminance(c)
	if (l+0.05)/(luminance(nearBlack)+0.05) > 1.05/(l+0.05) {
		return nearBlack
	}
	return pureWhite
}

// Fonts: Unbounded (display) and Archivo (UI), both OFL, embedded with their licences in fonts/.
var (
	//go:embed fonts/Unbounded.ttf
	unboundedTTF []byte
	//go:embed fonts/Archivo.ttf
	archivoTTF []byte

	srcDisplay = mustFont(unboundedTTF)
	srcUI      = mustFont(archivoTTF)
	tagWeight  = text.MustParseTag("wght")
)

func mustFont(ttf []byte) *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		panic("client: embedded font: " + err.Error())
	}
	return s
}

func newFace(src *text.GoTextFaceSource, size float64, weight float32) *text.GoTextFace {
	f := &text.GoTextFace{Source: src, Size: size}
	f.SetVariation(tagWeight, weight)
	return f
}

// fonts is the type scale, rebuilt when the UI scale changes. Sizes are logical px.
type fonts struct {
	logo, title, heading, callout, number   *text.GoTextFace // Unbounded
	body, bodyBold, label, labelBold, small *text.GoTextFace // Archivo
}

func newFonts(u float64) fonts {
	return fonts{
		logo:      newFace(srcDisplay, 80*u, 800),
		title:     newFace(srcDisplay, 26*u, 700),
		heading:   newFace(srcDisplay, 17*u, 700),
		callout:   newFace(srcDisplay, 20*u, 800),
		number:    newFace(srcDisplay, 20*u, 700),
		body:      newFace(srcUI, 18*u, 500),
		bodyBold:  newFace(srcUI, 18*u, 700),
		label:     newFace(srcUI, 15*u, 500),
		labelBold: newFace(srcUI, 15*u, 700),
		small:     newFace(srcUI, 13*u, 500),
	}
}

// numbers holds "0".."40" so labels never format numbers while drawing.
var numbers = func() (s [core.MaxSize + 1]string) {
	for i := range s {
		s[i] = strconv.Itoa(i)
	}
	return s
}()

// seatNames are "P1".."P12".
var seatNames = func() (s [core.MaxPlayers]string) {
	for i := range s {
		s[i] = "P" + numbers[i+1]
	}
	return s
}()

// Seat types: index 0 is Human, 1+s is AI strategy s ("Jester", ...).
const seatHuman = 0

var seatLabels = func() []string {
	l := []string{"Human"}
	for _, n := range aiNames() {
		l = append(l, string(n[0]-'a'+'A')+n[1:])
	}
	return l
}()

// Geometry and drawing helpers. They share one path, so call them from the draw goroutine only. They take
// color.RGBA, not color.Color, because boxing a colour into an interface allocates and they run many times a frame.
var (
	scratchPath vector.Path
	pathOp      vector.DrawPathOptions
	strokeOp    = vector.StrokeOptions{LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	textOp      text.DrawOptions
)

type rect struct{ x, y, w, h float64 }

func (r rect) contains(x, y float64) bool { return x >= r.x && y >= r.y && x < r.x+r.w && y < r.y+r.h }
func (r rect) cx() float64                { return r.x + r.w/2 }
func (r rect) cy() float64                { return r.y + r.h/2 }
func (r rect) right() float64             { return r.x + r.w }
func (r rect) bottom() float64            { return r.y + r.h }
func (r rect) inset(d float64) rect       { return rect{r.x + d, r.y + d, r.w - 2*d, r.h - 2*d} }

// addRound adds a rounded rectangle to p as a closed sub-path.
func addRound(p *vector.Path, r rect, rad float64) {
	rad = min(rad, r.w/2, r.h/2)
	x, y, w, h, k := float32(r.x), float32(r.y), float32(r.w), float32(r.h), float32(rad)
	p.MoveTo(x+k, y)
	p.LineTo(x+w-k, y)
	p.ArcTo(x+w, y, x+w, y+k, k)
	p.LineTo(x+w, y+h-k)
	p.ArcTo(x+w, y+h, x+w-k, y+h, k)
	p.LineTo(x+k, y+h)
	p.ArcTo(x, y+h, x, y+h-k, k)
	p.LineTo(x, y+k)
	p.ArcTo(x, y, x+k, y, k)
	p.Close()
}

func fillRound(dst *ebiten.Image, r rect, rad float64, c color.RGBA) {
	scratchPath.Reset()
	addRound(&scratchPath, r, rad)
	fillPath(dst, &scratchPath, c)
}

// fillPill fills r with fully rounded ends.
func fillPill(dst *ebiten.Image, r rect, c color.RGBA) { fillRound(dst, r, r.h/2, c) }

// strokeRound draws a border of width w inside r.
func strokeRound(dst *ebiten.Image, r rect, rad, w float64, c color.RGBA) {
	scratchPath.Reset()
	addRound(&scratchPath, r.inset(w/2), max(0, rad-w/2))
	strokePath(dst, &scratchPath, w, c)
}

// dashedRect draws r's outline as dashes of length d (and gaps of d) at width w.
func dashedRect(dst *ebiten.Image, r rect, d, w float64, c color.RGBA) {
	p := &scratchPath
	p.Reset()
	side := func(x0, y0, x1, y1 float64) {
		l := math.Hypot(x1-x0, y1-y0)
		for s := 0.0; s < l; s += 2 * d {
			e := min(l, s+d)
			p.MoveTo(float32(x0+(x1-x0)*s/l), float32(y0+(y1-y0)*s/l))
			p.LineTo(float32(x0+(x1-x0)*e/l), float32(y0+(y1-y0)*e/l))
		}
	}
	side(r.x, r.y, r.right(), r.y)
	side(r.right(), r.y, r.right(), r.bottom())
	side(r.right(), r.bottom(), r.x, r.bottom())
	side(r.x, r.bottom(), r.x, r.y)
	strokeOp.LineCap = vector.LineCapButt
	strokePath(dst, p, w, c)
	strokeOp.LineCap = vector.LineCapRound
}

func fillCircle(dst *ebiten.Image, x, y, r float64, c color.RGBA) {
	scratchPath.Reset()
	scratchPath.Arc(float32(x), float32(y), float32(r), 0, 2*math.Pi, vector.Clockwise)
	fillPath(dst, &scratchPath, c)
}

// ring draws a circle outline of width w inside radius r.
func ring(dst *ebiten.Image, x, y, r, w float64, c color.RGBA) {
	scratchPath.Reset()
	scratchPath.Arc(float32(x), float32(y), float32(r-w/2), 0, 2*math.Pi, vector.Clockwise)
	scratchPath.Close()
	strokePath(dst, &scratchPath, w, c)
}

func line(dst *ebiten.Image, x0, y0, x1, y1, w float64, c color.RGBA) {
	scratchPath.Reset()
	scratchPath.MoveTo(float32(x0), float32(y0))
	scratchPath.LineTo(float32(x1), float32(y1))
	strokePath(dst, &scratchPath, w, c)
}

// tint scales cs by c, like ColorScale.ScaleWithColor without the interface.
func tint(cs *ebiten.ColorScale, c color.RGBA) {
	cs.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
}

func fillPath(dst *ebiten.Image, p *vector.Path, c color.RGBA) {
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	tint(&pathOp.ColorScale, c)
	vector.FillPath(dst, p, nil, &pathOp)
}

func strokePath(dst *ebiten.Image, p *vector.Path, w float64, c color.RGBA) {
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	tint(&pathOp.ColorScale, c)
	strokeOp.Width = float32(w)
	vector.StrokePath(dst, p, &strokeOp, &pathOp)
}

// drawText draws s with its horizontal anchor at x (start, centre or end) and its vertical centre at y.
func drawText(dst *ebiten.Image, s string, face text.Face, x, y float64, align text.Align, c color.RGBA) {
	textOp = text.DrawOptions{}
	textOp.GeoM.Translate(x, y)
	tint(&textOp.ColorScale, c)
	textOp.PrimaryAlign = align
	textOp.SecondaryAlign = text.AlignCenter
	text.Draw(dst, s, face, &textOp)
}

func textWidth(s string, face text.Face) float64 {
	w, _ := text.Measure(s, face, 0)
	return w
}

// drawMark draws seat p's shape mark centred in a dot of radius r filled with c, in black or white for contrast.
func drawMark(dst *ebiten.Image, p int, x, y, r float64, c color.RGBA) {
	markShape(dst, p, x, y, r, inkOn(c))
}

// markShape draws seat p's shape mark in ink, sized for a dot of radius r centred at (x, y).
// The marks, in seat order: dot, bar, plus, cross, triangle, square, diamond, stem, pair, ring, chevron, frame.
func markShape(dst *ebiten.Image, p int, x, y, r float64, ink color.RGBA) {
	s, w := r*0.42, r*0.2
	p2 := &scratchPath
	poly := func(pts ...float64) {
		p2.Reset()
		p2.MoveTo(float32(pts[0]), float32(pts[1]))
		for i := 2; i < len(pts); i += 2 {
			p2.LineTo(float32(pts[i]), float32(pts[i+1]))
		}
		p2.Close()
		fillPath(dst, p2, ink)
	}
	switch p % core.MaxPlayers {
	case 0:
		fillCircle(dst, x, y, s*0.55, ink)
	case 1:
		line(dst, x-s, y, x+s, y, w, ink)
	case 2:
		line(dst, x-s, y, x+s, y, w, ink)
		line(dst, x, y-s, x, y+s, w, ink)
	case 3:
		line(dst, x-s*0.75, y-s*0.75, x+s*0.75, y+s*0.75, w, ink)
		line(dst, x-s*0.75, y+s*0.75, x+s*0.75, y-s*0.75, w, ink)
	case 4:
		poly(x, y-s, x+s, y+s*0.75, x-s, y+s*0.75)
	case 5:
		fillRound(dst, rect{x - s*0.7, y - s*0.7, s * 1.4, s * 1.4}, 0, ink)
	case 6:
		poly(x, y-s, x+s, y, x, y+s, x-s, y)
	case 7:
		line(dst, x, y-s, x, y+s, w, ink)
	case 8:
		fillCircle(dst, x-s*0.55, y, s*0.38, ink)
		fillCircle(dst, x+s*0.55, y, s*0.38, ink)
	case 9:
		ring(dst, x, y, s*0.8+w/2, w, ink)
	case 10:
		scratchPath.Reset()
		scratchPath.MoveTo(float32(x-s), float32(y+s*0.4))
		scratchPath.LineTo(float32(x), float32(y-s*0.4))
		scratchPath.LineTo(float32(x+s), float32(y+s*0.4))
		strokePath(dst, &scratchPath, w, ink)
	case 11:
		strokeRound(dst, rect{x - s*0.7 - w/2, y - s*0.7 - w/2, s*1.4 + w, s*1.4 + w}, 0, w, ink)
	}
}

// seatDot draws seat p's dot, with its shape mark when marks are on, or an empty ring when the player is out.
func seatDot(dst *ebiten.Image, sc *scheme, p int, x, y, r float64, out, marks bool) {
	if out {
		ring(dst, x, y, r, max(1.5, r*0.14), sc.dim)
		return
	}
	c := sc.players[p]
	fillCircle(dst, x, y, r, c)
	if marks {
		drawMark(dst, p, x, y, r, c)
	}
}

// chevron draws a small open chevron pointing down (or up) centred at (x, y).
func chevron(dst *ebiten.Image, x, y, s, w float64, up bool, c color.RGBA) {
	d := s / 2
	if up {
		d = -d
	}
	scratchPath.Reset()
	scratchPath.MoveTo(float32(x-s), float32(y-d))
	scratchPath.LineTo(float32(x), float32(y+d))
	scratchPath.LineTo(float32(x+s), float32(y-d))
	strokePath(dst, &scratchPath, w, c)
}
