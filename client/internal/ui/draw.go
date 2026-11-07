package ui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/core"
)

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

// addRound adds a rounded rectangle to p as a closed sub-path.
func addRound(p *vector.Path, r Rect, rad float64) {
	rad = min(rad, r.W/2, r.H/2)
	x, y, w, h, k := float32(r.X), float32(r.Y), float32(r.W), float32(r.H), float32(rad)
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
	SpritePad = 2   // transparent margin, so filtering never reads past a sprite
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
	img := ebiten.NewImage(side+2*SpritePad, side+2*SpritePad)
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
	box, rad, bw := Rect{SpritePad, SpritePad, float64(2 * c), float64(2 * c)}, float64(r)/4, float64(w)/4
	scratchPath.Reset()
	if w == 0 {
		addRound(&scratchPath, box, rad)
		FillPath(img, &scratchPath, PureWhite)
		return img
	}
	addRound(&scratchPath, box.Inset(bw/2), max(0, rad-bw/2))
	StrokePath(img, &scratchPath, bw, PureWhite)
	return img
}

// queueRound queues r with corners of radius rad, filled (w = 0) or as a border w wide inside r, mapped through g
// (nil leaves it upright), and returns the sprite drawShapes draws the queue from. Shapes queued together must share
// it, so only queue several shapes of one size and radius.
func QueueRound(r Rect, rad, w float64, g *ebiten.GeoM, c color.RGBA) *ebiten.Image {
	if r.W <= 0 || r.H <= 0 {
		return nil
	}
	rad = max(0, min(rad, r.W/2, r.H/2))
	corner := min(max(rad, w, 1), r.W/2, r.H/2)
	cs := math.Ceil(corner)
	if cs > 64 {
		cs = min(spriteMax, math.Ceil(cs/8)*8) // fewer sizes of big sprite, each drawn at most 12% smaller
	}
	k := cs / corner // sprite px per screen px
	img := roundSprite(int(cs), int(math.Round(rad*k*4)), int(math.Round(w*k*4)))

	xs := [4]float64{r.X, r.X + corner, r.Right() - corner, r.Right()}
	ys := [4]float64{r.Y, r.Y + corner, r.Bottom() - corner, r.Bottom()}
	ss := [4]float32{SpritePad, SpritePad + float32(cs), SpritePad + float32(cs), SpritePad + 2*float32(cs)}
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
			if w > 0 && i == 1 && j == 1 || i == 1 && r.W-2*corner <= 0 || j == 1 && r.H-2*corner <= 0 {
				continue
			}
			v := n + 4*j + i
			shapeIdx = append(shapeIdx, v, v+1, v+4, v+1, v+5, v+4)
		}
	}
	return img
}

// drawShapes draws the queued shapes from img in one batch and empties the queue.
func DrawShapes(dst, img *ebiten.Image) {
	if len(shapeIdx) > 0 {
		dst.DrawTriangles32(shapeVerts, shapeIdx, img, &shapeOp)
	}
	shapeVerts, shapeIdx = shapeVerts[:0], shapeIdx[:0]
}

func FillRound(dst *ebiten.Image, r Rect, rad float64, c color.RGBA) {
	DrawShapes(dst, QueueRound(r, rad, 0, nil, c))
}

// fillPill fills r with fully rounded ends.
func FillPill(dst *ebiten.Image, r Rect, c color.RGBA) { FillRound(dst, r, r.H/2, c) }

// strokeRound draws a border of width w inside r.
func StrokeRound(dst *ebiten.Image, r Rect, rad, w float64, c color.RGBA) {
	DrawShapes(dst, QueueRound(r, rad, w, nil, c))
}

// dashedRect draws r's outline as dashes of length d (and gaps of d) at width w, centred on its edges: the top and
// right sides start at the top-left and top-right corners, the bottom and left sides at the opposite ones.
func DashedRect(dst *ebiten.Image, r Rect, d, w float64, c color.RGBA) {
	var img *ebiten.Image
	for s := 0.0; s < r.W; s += 2 * d {
		l := min(d, r.W-s)
		img = QueueRound(Rect{r.X + s, r.Y - w/2, l, w}, 0, 0, nil, c)
		QueueRound(Rect{r.Right() - s - l, r.Bottom() - w/2, l, w}, 0, 0, nil, c)
	}
	for s := 0.0; s < r.H; s += 2 * d {
		l := min(d, r.H-s)
		img = QueueRound(Rect{r.Right() - w/2, r.Y + s, w, l}, 0, 0, nil, c)
		QueueRound(Rect{r.X - w/2, r.Bottom() - s - l, w, l}, 0, 0, nil, c)
	}
	DrawShapes(dst, img)
}

func FillCircle(dst *ebiten.Image, x, y, r float64, c color.RGBA) {
	FillRound(dst, Rect{x - r, y - r, 2 * r, 2 * r}, r, c)
}

// ring draws a circle outline of width w inside radius r.
func Ring(dst *ebiten.Image, x, y, r, w float64, c color.RGBA) {
	StrokeRound(dst, Rect{x - r, y - r, 2 * r, 2 * r}, r, w, c)
}

// line draws a stroke w wide with round caps from (x0, y0) to (x1, y1).
func Line(dst *ebiten.Image, x0, y0, x1, y1, w float64, c color.RGBA) {
	var g ebiten.GeoM
	g.Rotate(math.Atan2(y1-y0, x1-x0))
	g.Translate(x0, y0)
	DrawShapes(dst, QueueRound(Rect{-w / 2, -w / 2, math.Hypot(x1-x0, y1-y0) + w, w}, w/2, 0, &g, c))
}

// tint scales cs by c, like ColorScale.ScaleWithColor without the interface.
func Tint(cs *ebiten.ColorScale, c color.RGBA) {
	cs.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
}

// fillPath fills p with vector. It is for rendering cached images; drawing the screen every frame uses sprites.
func FillPath(dst *ebiten.Image, p *vector.Path, c color.RGBA) {
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	Tint(&pathOp.ColorScale, c)
	vector.FillPath(dst, p, nil, &pathOp)
}

// labelKey names a label laid out by drawText.
// The mark comes from a sprite rendered once per seat and size.
func DrawMark(dst *ebiten.Image, p int, x, y, r float64, c color.RGBA) {
	DrawMarkIn(dst, p, x, y, r, InkOn(c))
}

// drawMarkIn draws seat p's shape mark in ink.
func DrawMarkIn(dst *ebiten.Image, p int, x, y, r float64, ink color.RGBA) {
	sr := max(1, math.Ceil(r))
	k := spriteKey{mark: p + 1, c: int(sr)}
	img, ok := sprites[k]
	if !ok {
		img = newSprite(k, 2*int(sr))
		MarkShape(img, p, SpritePad+sr, SpritePad+sr, sr, PureWhite)
	}
	imageOp = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true} // at least half size
	imageOp.GeoM.Translate(-SpritePad-sr, -SpritePad-sr)
	imageOp.GeoM.Scale(r/sr, r/sr)
	imageOp.GeoM.Translate(x, y)
	Tint(&imageOp.ColorScale, ink)
	dst.DrawImage(img, &imageOp)
}

// markShape draws seat p's shape mark in ink, sized for a dot of radius r centred at (x, y). Two marks fill vector
// paths, so it renders cached images (drawMark's sprites, the board's orb atlas), not the screen each frame.
// The marks, in seat order: dot, bar, plus, cross, triangle, square, diamond, stem, pair, ring, chevron, frame.
func MarkShape(dst *ebiten.Image, p int, x, y, r float64, ink color.RGBA) {
	s, w := r*0.42, r*0.2
	p2 := &scratchPath
	poly := func(pts ...float64) {
		p2.Reset()
		p2.MoveTo(float32(pts[0]), float32(pts[1]))
		for i := 2; i < len(pts); i += 2 {
			p2.LineTo(float32(pts[i]), float32(pts[i+1]))
		}
		p2.Close()
		FillPath(dst, p2, ink)
	}
	switch p % core.MaxPlayers {
	case 0:
		FillCircle(dst, x, y, s*0.55, ink)
	case 1:
		Line(dst, x-s, y, x+s, y, w, ink)
	case 2:
		Line(dst, x-s, y, x+s, y, w, ink)
		Line(dst, x, y-s, x, y+s, w, ink)
	case 3:
		Line(dst, x-s*0.75, y-s*0.75, x+s*0.75, y+s*0.75, w, ink)
		Line(dst, x-s*0.75, y+s*0.75, x+s*0.75, y-s*0.75, w, ink)
	case 4:
		poly(x, y-s, x+s, y+s*0.75, x-s, y+s*0.75)
	case 5:
		FillRound(dst, Rect{x - s*0.7, y - s*0.7, s * 1.4, s * 1.4}, 0, ink)
	case 6:
		poly(x, y-s, x+s, y, x, y+s, x-s, y)
	case 7:
		Line(dst, x, y-s, x, y+s, w, ink)
	case 8:
		FillCircle(dst, x-s*0.55, y, s*0.38, ink)
		FillCircle(dst, x+s*0.55, y, s*0.38, ink)
	case 9:
		Ring(dst, x, y, s*0.8+w/2, w, ink)
	case 10:
		Chevron(dst, x, y, s, -s*0.4, w, ink)
	case 11:
		StrokeRound(dst, Rect{x - s*0.7 - w/2, y - s*0.7 - w/2, s*1.4 + w, s*1.4 + w}, 0, w, ink)
	}
}

// seatDot draws seat p's dot, with its shape mark when marks are on, or an empty ring when the player is out.
func SeatDot(dst *ebiten.Image, sc *Palette, p int, x, y, r float64, out, marks bool) {
	if out {
		Ring(dst, x, y, r, max(1.5, r*0.14), sc.Dim)
		return
	}
	c := sc.Players[p]
	FillCircle(dst, x, y, r, c)
	if marks {
		DrawMark(dst, p, x, y, r, c)
	}
}

// chevron draws an open chevron centred at (x, y), s wide on each side, its point d below y (above when d < 0).
func Chevron(dst *ebiten.Image, x, y, s, d, w float64, c color.RGBA) {
	Line(dst, x-s, y-d, x, y+d, w, c)
	Line(dst, x, y+d, x+s, y-d, w, c)
}

// StrokePath draws an antialiased path with the shared rounded stroke style.
func StrokePath(dst *ebiten.Image, p *vector.Path, width float64, c color.RGBA) {
	pathOp = vector.DrawPathOptions{AntiAlias: true}
	Tint(&pathOp.ColorScale, c)
	strokeOp.Width = float32(width)
	vector.StrokePath(dst, p, &strokeOp, &pathOp)
}
