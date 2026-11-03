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
	danger  color.RGBA // a destructive choice (Remove player), apart from every player ink
	players [core.MaxPlayers]color.RGBA
}

const (
	themeLight = iota
	themeDark
)

var schemes = [2]scheme{
	themeLight: {
		ground: hex(0xF4F3F0), tile: hex(0xE0DED9), tileHi: hex(0xD1CEC8),
		ink: hex(0x171615), inkHi: hex(0x3B3936), dim: hex(0x65615B), danger: hex(0xC21D3A),
		players: [core.MaxPlayers]color.RGBA{
			hex(0xD55E00), hex(0x0072B2), hex(0x009E73), hex(0xE69F00), hex(0xCC79A7), hex(0x56B4E9),
			hex(0xE8D21D), hex(0x2E2C2A), hex(0x901CFC), hex(0x8B5A2B), hex(0x440048), hex(0x641404),
		},
	},
	themeDark: {
		ground: hex(0x1C1A18), tile: hex(0x2D2A27), tileHi: hex(0x3B3733),
		ink: hex(0xF5F2EE), inkHi: hex(0xD4CEC6), dim: hex(0xA39C92), danger: hex(0xFF6B7A),
		players: [core.MaxPlayers]color.RGBA{
			hex(0xE86A1A), hex(0x3A9BE0), hex(0x1FB88C), hex(0xF0AA1A), hex(0xDD8DB8), hex(0x7CC6F0),
			hex(0xF0E442), hex(0xF4F4F4), hex(0x844CFC), hex(0xC98B4E), hex(0xB4FC84), hex(0xB85450),
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
	logo, title, heading, callout, calloutCompact, calloutTight, number *text.GoTextFace // Unbounded
	body, bodyBold, label, labelBold, small, smallBold                  *text.GoTextFace // Archivo
}

func newFonts(u float64) fonts {
	return fonts{
		logo:           newFace(srcDisplay, 80*u, 800),
		title:          newFace(srcDisplay, 26*u, 700),
		heading:        newFace(srcDisplay, 17*u, 700),
		callout:        newFace(srcDisplay, 20*u, 800),
		calloutCompact: newFace(srcDisplay, 15*u, 800),
		calloutTight:   newFace(srcDisplay, 13*u, 800),
		number:         newFace(srcDisplay, 20*u, 700),
		body:           newFace(srcUI, 18*u, 500),
		bodyBold:       newFace(srcUI, 18*u, 700),
		label:          newFace(srcUI, 15*u, 500),
		labelBold:      newFace(srcUI, 15*u, 700),
		small:          newFace(srcUI, 13*u, 500),
		smallBold:      newFace(srcUI, 13*u, 700),
	}
}

// numbers holds "0".."40" so labels never format numbers while drawing.
var numbers = func() (s [core.MaxSize + 1]string) {
	for i := range s {
		s[i] = strconv.Itoa(i)
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

// Geometry and drawing helpers. They share scratch buffers, so call them from the draw goroutine only. They take
// color.RGBA, not color.Color, because boxing a colour into an interface allocates and they run many times a frame.
var (
	scratchPath vector.Path
	pathOp      vector.DrawPathOptions
	strokeOp    = vector.StrokeOptions{LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	imageOp     ebiten.DrawImageOptions
	shapeVerts  []ebiten.Vertex
	shapeIdx    []uint32
	shapeOp     = ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
		DisableMipmaps: true, // sprites are drawn at about their own size
	}
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

// Shapes are drawn from small white sprites that vector renders once, tinted and stretched as nine-slices: corners
// keep the sprite's scale, edges and middle stretch. Filling paths with vector every frame costs several GPU passes
// and a render-target switch per path, which phones can't afford for a screen of controls.
const (
	spritePad = 2   // transparent margin, so filtering never reads past a sprite
	spriteMax = 512 // the largest corner a sprite holds; bigger corners (the results flood) scale it up
	spriteCap = 256 // sprites kept before the cache starts over; resizing a window keeps making new sizes
)

// spriteKey names a sprite: a rounded square 2c px across with corner radius r and border width w (0 fills), both
// in quarter px, or the shape mark of seat mark-1 in a dot of radius c.
type spriteKey struct{ mark, c, r, w int }

var sprites = map[spriteKey]*ebiten.Image{}

// newSprite makes and keeps the image for k: side px plus the margin.
func newSprite(k spriteKey, side int) *ebiten.Image {
	if len(sprites) >= spriteCap {
		clear(sprites) // dropped images are released when collected
	}
	img := ebiten.NewImage(side+2*spritePad, side+2*spritePad)
	sprites[k] = img
	return img
}

// roundSprite is the rounded square sprite with corner c and radius r and border w in quarter px (see spriteKey).
func roundSprite(c, r, w int) *ebiten.Image {
	k := spriteKey{c: c, r: r, w: w}
	if img, ok := sprites[k]; ok {
		return img
	}
	img := newSprite(k, 2*c)
	box, rad, bw := rect{spritePad, spritePad, float64(2 * c), float64(2 * c)}, float64(r)/4, float64(w)/4
	scratchPath.Reset()
	if w == 0 {
		addRound(&scratchPath, box, rad)
		fillPath(img, &scratchPath, pureWhite)
		return img
	}
	addRound(&scratchPath, box.inset(bw/2), max(0, rad-bw/2))
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	strokeOp.Width = float32(bw)
	vector.StrokePath(img, &scratchPath, &strokeOp, &pathOp)
	return img
}

// queueRound queues r with corners of radius rad, filled (w = 0) or as a border w wide inside r, mapped through g
// (nil leaves it upright), and returns the sprite drawShapes draws the queue from. Shapes queued together must share
// it, so only queue several shapes of one size and radius.
func queueRound(r rect, rad, w float64, g *ebiten.GeoM, c color.RGBA) *ebiten.Image {
	if r.w <= 0 || r.h <= 0 {
		return nil
	}
	rad = max(0, min(rad, r.w/2, r.h/2))
	corner := min(max(rad, w, 1), r.w/2, r.h/2)
	cs := math.Ceil(corner)
	if cs > 64 {
		cs = min(spriteMax, math.Ceil(cs/8)*8) // fewer sizes of big sprite, each drawn at most 12% smaller
	}
	k := cs / corner // sprite px per screen px
	img := roundSprite(int(cs), int(math.Round(rad*k*4)), int(math.Round(w*k*4)))

	xs := [4]float64{r.x, r.x + corner, r.right() - corner, r.right()}
	ys := [4]float64{r.y, r.y + corner, r.bottom() - corner, r.bottom()}
	ss := [4]float32{spritePad, spritePad + float32(cs), spritePad + float32(cs), spritePad + 2*float32(cs)}
	cr, cg, cb, ca := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
	n := uint32(len(shapeVerts))
	for j, y := range ys {
		for i, x := range xs {
			px, py := x, y
			if g != nil {
				px, py = g.Apply(x, y)
			}
			shapeVerts = append(shapeVerts, ebiten.Vertex{DstX: float32(px), DstY: float32(py), SrcX: ss[i], SrcY: ss[j],
				ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca})
		}
	}
	for j := range uint32(3) {
		for i := range uint32(3) {
			// A border's middle is empty, and a pill or circle has no middle row or column.
			if w > 0 && i == 1 && j == 1 || i == 1 && r.w-2*corner <= 0 || j == 1 && r.h-2*corner <= 0 {
				continue
			}
			v := n + 4*j + i
			shapeIdx = append(shapeIdx, v, v+1, v+4, v+1, v+5, v+4)
		}
	}
	return img
}

// drawShapes draws the queued shapes from img in one batch and empties the queue.
func drawShapes(dst, img *ebiten.Image) {
	if len(shapeIdx) > 0 {
		dst.DrawTriangles32(shapeVerts, shapeIdx, img, &shapeOp)
	}
	shapeVerts, shapeIdx = shapeVerts[:0], shapeIdx[:0]
}

func fillRound(dst *ebiten.Image, r rect, rad float64, c color.RGBA) {
	drawShapes(dst, queueRound(r, rad, 0, nil, c))
}

// fillPill fills r with fully rounded ends.
func fillPill(dst *ebiten.Image, r rect, c color.RGBA) { fillRound(dst, r, r.h/2, c) }

// strokeRound draws a border of width w inside r.
func strokeRound(dst *ebiten.Image, r rect, rad, w float64, c color.RGBA) {
	drawShapes(dst, queueRound(r, rad, w, nil, c))
}

// dashedRect draws r's outline as dashes of length d (and gaps of d) at width w, centred on its edges: the top and
// right sides start at the top-left and top-right corners, the bottom and left sides at the opposite ones.
func dashedRect(dst *ebiten.Image, r rect, d, w float64, c color.RGBA) {
	var img *ebiten.Image
	for s := 0.0; s < r.w; s += 2 * d {
		l := min(d, r.w-s)
		img = queueRound(rect{r.x + s, r.y - w/2, l, w}, 0, 0, nil, c)
		queueRound(rect{r.right() - s - l, r.bottom() - w/2, l, w}, 0, 0, nil, c)
	}
	for s := 0.0; s < r.h; s += 2 * d {
		l := min(d, r.h-s)
		img = queueRound(rect{r.right() - w/2, r.y + s, w, l}, 0, 0, nil, c)
		queueRound(rect{r.x - w/2, r.bottom() - s - l, w, l}, 0, 0, nil, c)
	}
	drawShapes(dst, img)
}

func fillCircle(dst *ebiten.Image, x, y, r float64, c color.RGBA) {
	fillRound(dst, rect{x - r, y - r, 2 * r, 2 * r}, r, c)
}

// ring draws a circle outline of width w inside radius r.
func ring(dst *ebiten.Image, x, y, r, w float64, c color.RGBA) {
	strokeRound(dst, rect{x - r, y - r, 2 * r, 2 * r}, r, w, c)
}

// line draws a stroke w wide with round caps from (x0, y0) to (x1, y1).
func line(dst *ebiten.Image, x0, y0, x1, y1, w float64, c color.RGBA) {
	var g ebiten.GeoM
	g.Rotate(math.Atan2(y1-y0, x1-x0))
	g.Translate(x0, y0)
	drawShapes(dst, queueRound(rect{-w / 2, -w / 2, math.Hypot(x1-x0, y1-y0) + w, w}, w/2, 0, &g, c))
}

// tint scales cs by c, like ColorScale.ScaleWithColor without the interface.
func tint(cs *ebiten.ColorScale, c color.RGBA) {
	cs.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
}

// fillPath fills p with vector. It is for rendering cached images; drawing the screen every frame uses sprites.
func fillPath(dst *ebiten.Image, p *vector.Path, c color.RGBA) {
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	tint(&pathOp.ColorScale, c)
	vector.FillPath(dst, p, nil, &pathOp)
}

// labelKey names a label laid out by drawText.
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
func drawText(dst *ebiten.Image, s string, face text.Face, x, y float64, align text.Align, c color.RGBA) {
	drawScaledText(dst, s, face, x, y, align, 1, c)
}

func labelGlyphs(s string, face text.Face, align text.Align) []text.Glyph {
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
func drawScaledText(dst *ebiten.Image, s string, face text.Face, x, y float64, align text.Align, scale float64, c color.RGBA) {
	glyphs := labelGlyphs(s, face, align)
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
		tint(&imageOp.ColorScale, c)
		dst.DrawImage(g.Image, &imageOp)
	}
}

func textWidth(s string, face text.Face) float64 {
	w, _ := text.Measure(s, face, 0)
	return w
}

// drawMark draws seat p's shape mark centred in a dot of radius r filled with c, in black or white for contrast.
// The mark comes from a sprite rendered once per seat and size.
func drawMark(dst *ebiten.Image, p int, x, y, r float64, c color.RGBA) {
	drawMarkIn(dst, p, x, y, r, inkOn(c))
}

// drawMarkIn draws seat p's shape mark in ink.
func drawMarkIn(dst *ebiten.Image, p int, x, y, r float64, ink color.RGBA) {
	sr := max(1, math.Ceil(r))
	k := spriteKey{mark: p + 1, c: int(sr)}
	img, ok := sprites[k]
	if !ok {
		img = newSprite(k, 2*int(sr))
		markShape(img, p, spritePad+sr, spritePad+sr, sr, pureWhite)
	}
	imageOp = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true} // at least half size
	imageOp.GeoM.Translate(-spritePad-sr, -spritePad-sr)
	imageOp.GeoM.Scale(r/sr, r/sr)
	imageOp.GeoM.Translate(x, y)
	tint(&imageOp.ColorScale, ink)
	dst.DrawImage(img, &imageOp)
}

// markShape draws seat p's shape mark in ink, sized for a dot of radius r centred at (x, y). Two marks fill vector
// paths, so it renders cached images (drawMark's sprites, the board's orb atlas), not the screen each frame.
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
		chevron(dst, x, y, s, -s*0.4, w, ink)
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

// chevron draws an open chevron centred at (x, y), s wide on each side, its point d below y (above when d < 0).
func chevron(dst *ebiten.Image, x, y, s, d, w float64, c color.RGBA) {
	line(dst, x-s, y-d, x, y+d, w, c)
	line(dst, x, y+d, x+s, y-d, w, c)
}
