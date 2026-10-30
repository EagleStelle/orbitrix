package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/core"
)

// Board geometry and animation lengths, in base units and 60 Hz ticks. Base units are the prototype's unscaled
// cell size, which the orb motion is defined in.
const (
	boardMargin   = 6  // around the board in the prototype's cell-size formula, in base units
	burstTicks    = 18 // explosion: an expanding, fading ring
	transferTicks = 14 // an orb flying to a neighbour; also the length of a wave
	minBurstTicks = 6  // bursts stay visible however fast a long chain plays
	chainBudget   = 480.0
	skipBudget    = 24.0
	tileGutter    = 0.07 // gap on each side of a tile, as a fraction of the cell
	tileRadius    = 0.2  // tile corner radius, as a fraction of the cell
	tileTint      = 0.3  // how much of the player's ink their tiles take on their turn
	burstSrcR     = 64   // radius of the burst ring source image
)

// waveTicks paces chain playback; the rules have already resolved the move, this only sets how fast it is shown.
// The first five waves play at full length so a move stays readable. Later ones speed up by 10% each down to 2.5
// ticks, and a long chain speeds up faster (30% per wave) to tailCap, which keeps the whole chain within about
// chainBudget ticks (8 s). Once the player skips, no wave is longer than skipCap.
func waveTicks(k int, tailCap, skipCap float64) float64 {
	d := max(2.5, transferTicks*math.Pow(0.9, float64(max(0, k-4))))
	if k > 4 {
		d = min(d, max(tailCap, transferTicks*math.Pow(0.7, float64(k-4))))
	}
	if skipCap > 0 {
		d = min(d, skipCap)
	}
	return d
}

// tailCap is the wave length that fits waves after the first five into the rest of chainBudget.
func tailCap(waves int) float64 {
	return (chainBudget - 5*transferTicks) / float64(max(1, waves-5))
}

// cellSize is the base cell for a cols×rows board in a w×h (logical px) area, between 24 and 80 (from the
// prototype's calculate_cell_size, which the orb motion's proportions depend on). The board is then scaled to fill
// the area.
func cellSize(cols, rows int, w, h float64) float64 {
	c := min(math.Floor((w-2*boardMargin-100)/float64(cols)), math.Floor((h-2*boardMargin-150)/float64(rows)))
	return max(24, min(c, 80))
}

// board shows a game: tiles, orbs, explosions and flying orbs, and the wave-by-wave playback of the last move.
type board struct {
	geo  *core.Geometry
	v    *view
	orbs *orbField

	// Playback of the move in progress.
	waves   []core.Wave
	wave    int     // wave in flight
	t, dur  float64 // ticks into it, and its length
	playing bool
	tailCap float64 // see waveTicks
	skipCap float64 // 0 until the player skips
	clock   float64 // ticks since the board was created

	burstStart, burstLen []float64
	burstOwner           []int8

	// Layout in device pixels.
	area   rect
	base   float64 // base cell size
	k      float64 // device px per base unit
	ox, oy float64 // board origin

	tileImg  *ebiten.Image
	orbImg   *ebiten.Image // atlas of square cells: rim disc, fill disc, then the twelve marks
	orbSize  float64       // side of each atlas cell
	sc       *scheme       // the mode being drawn
	marks    bool          // draw shape marks on orbs
	markInk  [core.MaxPlayers]color.RGBA
	burstImg *ebiten.Image
	verts    []ebiten.Vertex
	idx      []uint32
	tri      ebiten.DrawTrianglesOptions
	badge    *text.GoTextFace
}

func newBoard(g *core.Game) *board {
	n := g.Geo.Cells()
	b := &board{
		geo: g.Geo, v: newView(g), orbs: newOrbField(g.Geo),
		burstStart: make([]float64, n), burstLen: make([]float64, n), burstOwner: make([]int8, n),
	}
	b.tri.Filter = ebiten.FilterLinear
	s := 2*burstSrcR + 4
	b.burstImg = ebiten.NewImage(s, s)
	ring(b.burstImg, float64(s)/2, float64(s)/2, burstSrcR, burstSrcR*0.16, pureWhite)
	return b
}

// layout fits the board into area (device px) at UI scale u, centred along the slack axis.
func (b *board) layout(area rect, u float64) {
	if area == b.area && b.tileImg != nil {
		return
	}
	b.area = area
	cols, rows := b.geo.Cols, b.geo.Rows
	b.base = cellSize(cols, rows, area.w/u, area.h/u)
	// The outer tiles' edges, not the cells around them, meet the area's edges, so the board shares the screen margin.
	bw, bh := (float64(cols)-2*tileGutter)*b.base, (float64(rows)-2*tileGutter)*b.base
	b.k = min(area.w/bw, area.h/bh)
	g := tileGutter * b.base * b.k
	b.ox, b.oy = math.Round(area.x+(area.w-bw*b.k)/2-g), math.Round(area.y+(area.h-bh*b.k)/2-g)

	if b.tileImg != nil {
		b.tileImg.Deallocate()
		b.orbImg.Deallocate()
	}
	cell := b.base * b.k
	ts := int(math.Ceil(cell*(1-2*tileGutter))) + 2
	b.tileImg = ebiten.NewImage(ts, ts)
	fillRound(b.tileImg, rect{1, 1, float64(ts - 2), float64(ts - 2)}, cell*tileRadius, pureWhite)

	// Orbs are flat discs with a rim in the mat's colour, so overlapping orbs read as separate pieces, and
	// optionally the owner's shape mark. Rim, fill and the twelve marks come from one atlas, so every orb batches
	// together and keeps its painter's order.
	r := orbRadius(b.base) * b.k
	s := math.Ceil(2*r) + 4
	fr := r - max(1.5, r*0.16)
	b.orbSize = s
	b.orbImg = ebiten.NewImage(int((2+core.MaxPlayers)*s), int(s))
	fillCircle(b.orbImg, s/2, s/2, r, pureWhite)
	fillCircle(b.orbImg, s+s/2, s/2, fr, pureWhite)
	for p := range core.MaxPlayers {
		markShape(b.orbImg, p, float64(2+p)*s+s/2, s/2, fr, pureWhite)
	}
	b.badge = newFace(srcUI, max(9, 0.17*cell), 700)
}

// cellAt returns the cell under device point (x, y), or -1.
func (b *board) cellAt(x, y float64) int {
	if b.k == 0 {
		return -1
	}
	c := int(math.Floor((x - b.ox) / b.k / b.base))
	r := int(math.Floor((y - b.oy) / b.k / b.base))
	if c < 0 || r < 0 || c >= b.geo.Cols || r >= b.geo.Rows {
		return -1
	}
	return b.geo.Index(c, r)
}

// centre of cell i in device px.
func (b *board) centre(i int) (float64, float64) {
	c, r := b.geo.Coords(i)
	return b.ox + (float64(c)+0.5)*b.base*b.k, b.oy + (float64(r)+0.5)*b.base*b.k
}

// tileRect is cell i's tile in device px.
func (b *board) tileRect(i int) rect {
	x, y := b.centre(i)
	h := b.base * b.k * (0.5 - tileGutter)
	return rect{x - h, y - h, 2 * h, 2 * h}
}

// bounds is the whole board, outer tile edge to outer tile edge, in device px.
func (b *board) bounds() rect {
	cell, g := b.base*b.k, tileGutter*b.base*b.k
	return rect{b.ox + g, b.oy + g, float64(b.geo.Cols)*cell - 2*g, float64(b.geo.Rows)*cell - 2*g}
}

// start shows a move: mover's orb lands in cell, then waves (from core.Game.Apply) play out. snd may be nil.
func (b *board) start(cell int, mover int8, waves []core.Wave, snd *sound) {
	b.v.place(cell, mover)
	b.waves, b.wave, b.t, b.playing = waves, -1, 0, true
	b.tailCap, b.skipCap = tailCap(len(waves)), 0
	b.next(snd)
}

// sync makes the view match g without animating.
func (b *board) sync(g *core.Game) { b.v.sync(g) }

// next launches the following wave, or ends playback.
func (b *board) next(snd *sound) {
	b.wave++
	if b.wave >= len(b.waves) {
		b.playing, b.t = false, 0
		return
	}
	w := b.waves[b.wave]
	b.v.launch(w)
	b.dur = waveTicks(b.wave, b.tailCap, b.skipCap)
	l := max(minBurstTicks, b.dur*burstTicks/transferTicks)
	for k, e := range w {
		b.burstStart[e], b.burstLen[e], b.burstOwner[e] = b.clock, l, b.v.from[k]
	}
	snd.wave(b.wave, len(w))
}

// fastForward plays the rest of the chain quickly.
func (b *board) fastForward() {
	if b.playing && b.skipCap == 0 {
		b.skipCap = skipBudget / float64(len(b.waves)-b.wave)
		b.dur = min(b.dur, b.skipCap)
	}
}

// tick advances the animations by one 60 Hz tick. Short waves can finish several per tick.
func (b *board) tick(snd *sound) {
	b.clock++
	if !b.playing {
		return
	}
	b.t++
	for b.playing && b.t >= b.dur {
		b.t -= b.dur
		b.v.land(b.waves[b.wave])
		b.next(snd)
	}
}

// draw renders the board on the mat at animation time tms (ms). The tiles of player turn light up in their ink;
// every other tile stays printed blank (turn -1 lights none).
func (b *board) draw(dst *ebiten.Image, sc *scheme, tms float64, turn int, marks bool) {
	b.sc, b.marks = sc, marks
	if marks {
		for p := range b.markInk {
			b.markInk[p] = inkOn(sc.players[p])
		}
	}
	ts := float64(b.tileImg.Bounds().Dx())
	lit := sc.tile
	if turn >= 0 {
		lit = mix(sc.tile, sc.players[turn], tileTint)
	}
	for i, o := range b.v.owner {
		c := sc.tile
		if int(o) == turn {
			c = lit
		}
		x, y := b.centre(i)
		b.quad(x, y, ts/2, 0, ts, c, 1)
	}
	b.flush(dst, b.tileImg)

	// Orbs, rim then fill (then mark) for each, in one batch.
	for i, n := range b.v.count {
		if n == 0 {
			continue
		}
		cx, cy := b.centre(i)
		for _, p := range b.orbs.update(i, int(n), tms, b.base) {
			b.orb(cx+p[0]*b.k, cy+p[1]*b.k, int(b.v.owner[i]))
		}
	}
	b.flush(dst, b.orbImg)
	b.drawBadges(dst, sc)

	// Explosions: flat rings in the exploding cell's ink.
	for i, start := range b.burstStart {
		if b.burstLen[i] == 0 {
			continue
		}
		f := (b.clock - start) / b.burstLen[i]
		if f >= 1 {
			b.burstLen[i] = 0
			continue
		}
		cx, cy := b.centre(i)
		e := 1 - (1-f)*(1-f) // ease out
		r := max(1, (0.25+0.7*e)*b.base*b.k)
		b.quad(cx, cy, r*float64(burstSrcR+2)/burstSrcR, 0, 2*burstSrcR+4, sc.players[b.burstOwner[i]], 0.9*(1-f))
	}
	b.flush(dst, b.burstImg)

	// Orbs in flight to the neighbours of the wave being shown.
	if b.playing {
		f := min(1, b.t/b.dur)
		for k, e := range b.waves[b.wave] {
			sx, sy := b.centre(int(e))
			for _, n := range b.geo.Neighbours(int(e)) {
				ex, ey := b.centre(int(n))
				b.orb(sx+(ex-sx)*f, sy+(ey-sy)*f, int(b.v.from[k]))
			}
		}
		b.flush(dst, b.orbImg)
	}
}

// orb queues one of player p's orbs at (x, y): its rim in the mat's colour, its fill, and its mark if marks are on.
func (b *board) orb(x, y float64, p int) {
	s := b.orbSize
	b.quad(x, y, s/2, 0, s, b.sc.ground, 1)
	b.quad(x, y, s/2, s, s, b.sc.players[p], 1)
	if b.marks {
		b.quad(x, y, s/2, float64(2+p)*s, s, b.markInk[p], 1)
	}
}

// quad queues a square of the source image starting at x-offset sx with side src, centred at (x, y) with
// half-size h, tinted c at alpha a.
func (b *board) quad(x, y, h, sx, src float64, c color.RGBA, a float64) {
	n := uint32(len(b.verts))
	r, g, bl := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
	x0, y0, x1, y1 := float32(x-h), float32(y-h), float32(x+h), float32(y+h)
	s0, s1, t := float32(sx), float32(sx+src), float32(src)
	b.verts = append(b.verts,
		ebiten.Vertex{DstX: x0, DstY: y0, SrcX: s0, SrcY: 0, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x1, DstY: y0, SrcX: s1, SrcY: 0, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x0, DstY: y1, SrcX: s0, SrcY: t, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x1, DstY: y1, SrcX: s1, SrcY: t, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
	)
	b.idx = append(b.idx, n, n+1, n+2, n+1, n+3, n+2)
}

func (b *board) flush(dst, src *ebiten.Image) {
	if len(b.idx) > 0 {
		dst.DrawTriangles32(b.verts, b.idx, src, &b.tri)
	}
	b.verts, b.idx = b.verts[:0], b.idx[:0]
}

// drawBadges labels cells holding more than four orbs, which only a chain stopped by a win leaves behind.
func (b *board) drawBadges(dst *ebiten.Image, sc *scheme) {
	for i, n := range b.v.count {
		if n <= 4 {
			continue
		}
		t := b.tileRect(i)
		h := max(14, t.h*0.3)
		r := rect{t.right() - h*1.3 - 2, t.bottom() - h - 2, h * 1.3, h}
		fillPill(dst, r, sc.ink)
		drawText(dst, numbers[n], b.badge, r.cx(), r.cy(), text.AlignCenter, sc.ground)
	}
}

// drawHover outlines cell i's tile in c: where the player to move would place.
func (b *board) drawHover(dst *ebiten.Image, i int, c color.RGBA, u float64) {
	t := b.tileRect(i)
	strokeRound(dst, t, t.w*tileRadius/(1-2*tileGutter), max(2, 3*u), c)
}
