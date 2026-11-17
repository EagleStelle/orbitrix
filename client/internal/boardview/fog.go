package boardview

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// fogVeil is how much of the mat a cell takes on while its viewer remembers it but doesn't see it now.
const fogVeil = 0.6

// Look updates what each side sees as the board shows it, wave by wave, and what it remembers of the map. Draw
// calls it; a screen reading Sees between frames calls it first.
func (b *Board) Look() {
	if b.fogNow == nil {
		return
	}
	clear(b.fogNow)
	b.State.Observe(b.State.Geo, b.fogNow)
	b.flights(b.fogNow)
	b.memDirty |= b.mem.See(b.State.Replay, &b.State.Cells, b.fogNow)
}

// fogged reports whether the board shows one player's sight rather than the whole board.
func (b *Board) fogged() bool { return b.Viewer >= 0 && b.fogNow != nil }

// Sees reports whether the viewer sees cell i now; without fog every cell is seen.
func (b *Board) Sees(i int) bool {
	return !b.fogged() || b.fogNow[i]&(1<<b.State.Side(b.Viewer)) != 0
}

// knows reports whether the viewer's side has ever seen cell i: out of sight it shows as the side last saw it.
func (b *Board) knows(i int) bool {
	return !b.fogged() || b.mem.Seen[i]&(1<<b.State.Side(b.Viewer)) != 0
}

// drawFog covers what the viewer doesn't see, at each cell's eased cover and veil (fade), over the map and its orbs. The map
// is already the viewer's memory (recall): a cell never seen is blank there, so nothing of its own shows; its fog tile
// covers it, joined to its fog neighbours under the map (drawFogBase). What the viewer remembers takes a veil of the mat.
func (b *Board) drawFog(dst *ebiten.Image, sc *ui.Palette) {
	if !b.fogged() {
		return
	}
	ts, cell := float64(b.TileImg.Bounds().Dx()), b.base*b.k
	h := cell/2 - cell*TileGutter // from a cell's centre to its tile's edge
	// The veil is laid in pieces that never overlap: each tile, each gutter between two cells, and each crossing of
	// gutters, so an edge lying along a gutter, its ends in the crossings, is veiled evenly. A tile or gutter takes the
	// least veil of the cells beside it: none beside a cell in sight, and none where both are fog, whose joins lie there.
	// A crossing takes the most veil of the gutters meeting there that hold an edge or a Spring's mount, whose ends
	// reach into it; without one, the least of its four cells.
	known := func(j int) float32 {
		if j < 0 || !b.knows(j) {
			return 2 // fog, or the board's edge: no veil of its own
		}
		return b.veil[j]
	}
	for i := range b.veil {
		x, y := b.Centre(i)
		right, down, diag := b.CellAt(x+cell, y), b.CellAt(x, y+cell), b.CellAt(x+cell, y+cell)
		crossing := min(known(i), known(right), known(down), known(diag))
		if diag >= 0 {
			ends := float32(-1)
			for _, arm := range [4][2]int{{i, right}, {i, down}, {right, diag}, {down, diag}} {
				if b.armed(arm[0], arm[1]) {
					ends = max(ends, min(known(arm[0]), known(arm[1])))
				}
			}
			if ends >= 0 {
				crossing = ends
			}
		}
		for _, p := range [4]struct {
			x0, y0, x1, y1 float64
			veil           float32
			there          bool
		}{
			{x - h, y - h, x + h, y + h, known(i), true},
			{x + h, y - h, x + cell - h, y + h, min(known(i), known(right)), right >= 0},
			{x - h, y + h, x + h, y + cell - h, min(known(i), known(down)), down >= 0},
			{x + h, y + h, x + cell - h, y + cell - h, crossing, diag >= 0},
		} {
			if p.there && p.veil > 0 && p.veil <= 1 {
				b.fill(p.x0, p.y0, p.x1, p.y1, ts, sc.Ground, float64(p.veil))
			}
		}
	}
	for i, c := range b.cover {
		if x, y := b.Centre(i); c > 0 {
			b.quad(x, y, ts/2, 0, ts, ui.PureWhite, float64(c))
		}
	}
	b.flush(dst, b.fogImg)
}

// drawFogBase draws the fog's joins under the map: bridges between never-seen neighbours, the gutters' crossing where
// four meet and the rounded inner corner where three do. Under the map, whatever a known cell reaches into the fog with
// (an edge's end, a Spring's mount) stays whole on top; drawFog lays the opaque fog tiles over the cells themselves.
func (b *Board) drawFogBase(dst *ebiten.Image, sc *ui.Palette) {
	if !b.fogged() {
		return
	}
	ts, cell := float64(b.TileImg.Bounds().Dx()), b.base*b.k
	if b.fogImg == nil || b.fogTheme != sc {
		b.renderFog(sc, ts)
	}
	// Cells never seen join across their gutters into one mass, as Walls do: a bridge to each such neighbour, reaching
	// under both tiles' rounded corners, and the gutters' crossing where four meet.
	cover := func(x, y float64) float32 {
		if j := b.CellAt(x, y); j >= 0 {
			return b.cover[j]
		}
		return 0
	}
	t, rad, fog := (ts-2)/2, cell*TileRadius, fogTile(sc) // half the tile without its padding, and its corner
	for i, c := range b.cover {
		if c == 0 {
			continue
		}
		x, y := b.Centre(i)
		right, down := min(c, cover(x+cell, y)), min(c, cover(x, y+cell))
		if right > 0 {
			b.fill(x+t-rad, y-t, x+cell-t+rad, y+t, ts, fog, float64(right))
		}
		if down > 0 {
			b.fill(x-t, y+t-rad, x+t, y+cell-t+rad, ts, fog, float64(down))
		}
		if all := min(right, down, cover(x+cell, y+cell)); all > 0 {
			b.fill(x+t-1, y+t-1, x+cell-t+1, y+cell-t+1, ts, fog, float64(all))
		}
	}
	// Where three of a vertex's four cells are fog, the mass turns a rounded inner corner around the fourth, as Walls do.
	for i := range b.cover {
		x, y := b.Centre(i)
		q := [4]float32{b.cover[i], cover(x+cell, y), cover(x+cell, y+cell), cover(x, y+cell)} // up-left, up-right, down-right, down-left
		open, a := -1, float32(1)
		for k, c := range q {
			switch {
			case c > 0:
				a = min(a, c)
			case open == -1:
				open = k
			default:
				open = -2
			}
		}
		if open >= 0 {
			b.quad(x+cell/2, y+cell/2, cell/2, b.fogCorner(ts, open), cell, ui.PureWhite, float64(a))
		}
	}
	b.flush(dst, b.fogImg)
}

// fogTicks is how long a cell's fog takes to clear or close, so it follows the orbs smoothly as they fly.
const fogTicks = 8

// fade eases each cell's fog toward what the viewer sees now: its veil, and a never-seen cell's fog tile. A new viewer's
// fog is set at once.
func (b *Board) fade() {
	if !b.fogged() {
		return
	}
	snap := b.faded != b.Viewer
	b.faded = b.Viewer
	for i := range b.veil {
		cover, veil := float32(0), float32(fogVeil)
		if !b.knows(i) {
			cover = 1
		}
		if b.Sees(i) {
			veil = 0
		}
		if snap {
			b.cover[i], b.veil[i] = cover, veil
			continue
		}
		b.cover[i], b.veil[i] = ease(b.cover[i], cover, 1.0/fogTicks), ease(b.veil[i], veil, fogVeil/fogTicks)
	}
}

// ease moves v toward to by at most step.
func ease(v, to, step float32) float32 {
	if v < to {
		return min(to, v+step)
	}
	return max(to, v-step)
}

// clarity is how much of cell i's orbs shows through its veil: 1 in sight, 0 once the veil has closed.
func (b *Board) clarity(i int) float64 {
	if !b.fogged() {
		return 1
	}
	return 1 - float64(b.veil[i])/fogVeil
}

// fill queues the rectangle from (x0, y0) to (x1, y1) in c at alpha a, cut from fogImg's white square; ts is the tile's
// side, where that square starts.
func (b *Board) fill(x0, y0, x1, y1, ts float64, c color.RGBA, a float64) {
	n := uint32(len(b.Verts))
	r, g, bl := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
	s0, s1 := float32(ts+4), float32(ts+8)
	for _, v := range [4][4]float64{{x0, y0, 0, 2}, {x1, y0, 1, 2}, {x0, y1, 0, 6}, {x1, y1, 1, 6}} {
		sx := s0
		if v[2] == 1 {
			sx = s1
		}
		b.Verts = append(b.Verts, ebiten.Vertex{DstX: float32(v[0]), DstY: float32(v[1]), SrcX: sx, SrcY: float32(v[3]), ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)})
	}
	b.idx = append(b.idx, n, n+1, n+2, n+1, n+3, n+2)
}

// knownCap is cell i's capacity as the viewer knows it, so the orbs they see turn frantic where they expect: unseen
// terrain beside a cell counts as Floor. mapped is the map Draw draws.
func (b *Board) knownCap(mapped *core.Game, i int) uint8 {
	if mapped == &b.recalled && mapped.Geo.Cap[i] != 0 {
		return mapped.Geo.Cap[i]
	}
	return b.State.Geo.Cap[i]
}

// recall draws memImg, the map layer of the viewer's side's memory, again once that memory or the board's look changed.
func (b *Board) recall(sc *ui.Palette) {
	side := b.State.Side(b.Viewer)
	if b.memImg != nil && b.memTheme == sc && b.memSide == side && b.memMarks == b.marks && b.memDirty&(1<<side) == 0 &&
		b.memRevision == b.State.revision {
		return
	}
	b.memTheme, b.memSide, b.memMarks, b.memDirty = sc, side, b.marks, b.memDirty&^(1<<side)
	b.memRevision = b.State.revision
	b.State.Replay.Recall(&b.recalled, &b.mem, b.Viewer)
	cols, rows := b.ScreenSize()
	cell := b.base * b.k
	w, h := int(math.Ceil(float64(cols)*cell)), int(math.Ceil(float64(rows)*cell))
	if b.memImg == nil || b.memImg.Bounds().Dx() != w || b.memImg.Bounds().Dy() != h {
		b.releaseMemory()
		b.memImg = ebiten.NewImage(max(1, w), max(1, h))
	} else {
		b.memImg.Clear()
	}
	b.arm(&b.recalled)
	r := &b.recalled
	// In sight the layer keeps the board's playback, as drawMapLayer does: a Landfill fills and a Box sinks once its
	// fall has landed.
	b.memFill, b.memBoxes = append(b.memFill[:0], r.Durability...), append(b.memBoxes[:0], r.Boxes...)
	for i := range b.memFill {
		if b.Sees(i) {
			b.memFill[i] = b.landfillDurability[i]
			if i < len(b.memBoxes) && i < len(b.shownBoxes) {
				b.memBoxes[i] = b.shownBoxes[i]
			}
		}
	}
	owner, count := b.mem.Orbs(side)
	DrawMapArt(b.memImg, sc, MapState{Map: r.Map, Durability: r.Durability, EdgeDurability: r.EdgeDurability, Pressed: r.Pressed, CircuitOn: r.CircuitOn, Aims: r.Geo.Aims, Owner: owner, Count: count, PortalAxes: r.Geo.PortalAxes, Outlets: r.Geo.Outlets, LandfillDurability: b.memFill, Boxes: b.memBoxes, Moving: true},
		MapTransform{Cell: cell, Turned: b.Turned}, false, b.marks)
}

// renderFog draws the fog atlas: the fog tile at the tile's size, a white square the veils are cut from, then the fog's
// concave corner for each quadrant (fogCorner), all 2 px apart so filtering never mixes them.
func (b *Board) renderFog(sc *ui.Palette, ts float64) {
	if b.fogImg != nil {
		b.fogImg.Deallocate()
	}
	cell := b.base * b.k
	side := math.Ceil(cell)
	b.fogImg, b.fogTheme = ebiten.NewImage(int(2*ts)+2+4*int(side+2), int(max(ts, side))), sc
	for k := range 4 { // as Walls round theirs
		renderMapSurfaceCorner(b.fogImg, ui.Rect{X: b.fogCorner(ts, k), W: cell, H: cell}, fogTile(sc), k, TileGutter, .14)
	}
	b.fogImg.SubImage(image.Rect(int(ts)+2, 0, int(2*ts)+2, int(ts))).(*ebiten.Image).Fill(ui.PureWhite) // 2 px apart, so filtering never mixes them
	ui.FillRound(b.fogImg, ui.Rect{X: 1, Y: 1, W: ts - 2, H: ts - 2}, b.base*b.k*TileRadius, fogTile(sc))
	unit, ink := (ts-2)/36, fogLine(sc)
	for _, l := range [3][4]float64{{-11, -7, 5, -7}, {-5, 0, 11, 0}, {-11, 7, 3, 7}} { // as iconFog draws them
		ui.Line(b.fogImg, ts/2+l[0]*unit, ts/2+l[1]*unit, ts/2+l[2]*unit, ts/2+l[3]*unit, 3*unit, ink)
	}
}

// fogCorner is where in the fog atlas the concave corner opening toward quadrant k starts (k as Walls' corners: up-left,
// up-right, down-right, down-left of a grid vertex).
func (b *Board) fogCorner(ts float64, k int) float64 {
	return 2*ts + 4 + float64(k)*(math.Ceil(b.base*b.k)+2)
}

// fogTile is a never-seen cell's fill: the tile darkened halfway to the pit, in either theme.
func fogTile(sc *ui.Palette) color.RGBA { return ui.Mix(sc.Tile, sc.Pit, .5) }

// fogLine is the fog tile's lines: the tile halfway to the dim ink.
func fogLine(sc *ui.Palette) color.RGBA { return ui.Mix(sc.Tile, sc.Dim, .5) }

// releaseFog drops the fog's images, which a new layout draws again at its size.
func (b *Board) releaseFog() {
	if b.fogImg != nil {
		b.fogImg.Deallocate()
	}
	b.fogImg = nil
	b.releaseMemory()
}

func (b *Board) releaseMemory() {
	if b.memImg != nil {
		b.memImg.Deallocate()
	}
	b.memImg = nil
}

// arm notes, per cell of map g, the gutters that hold an edge or a Spring's mount, a bit per direction (left, right,
// up, down), for armed.
func (b *Board) arm(g *core.Game) {
	if len(b.arms) != len(g.Map.Cells) {
		b.arms = make([]uint8, len(g.Map.Cells))
	}
	clear(b.arms)
	rows := g.Map.Rows
	step := [4]int{-rows, rows, -1, 1}
	mark := func(a, d int) {
		if n := a + step[d]; n >= 0 && n < len(b.arms) && (d > 1 || n%rows == a%rows) && (d < 2 || n/rows == a/rows) {
			b.arms[a] |= 1 << d
			b.arms[n] |= 1 << (d ^ 1)
		}
	}
	for _, e := range g.Map.Edges {
		for d, s := range step {
			if e.B-e.A == s {
				mark(e.A, d)
			}
		}
	}
	for i, c := range g.Map.Cells {
		if f := g.Map.SpringFacing(i); c.Feature == core.Spring && f >= 0 {
			mark(i, f^1) // its mount, against the Wall behind it
		}
	}
}

// armed reports whether the gutter between neighbouring cells a and c holds an edge or a Spring's mount.
func (b *Board) armed(a, c int) bool {
	if a < 0 || c < 0 || len(b.arms) == 0 {
		return false
	}
	rows := b.State.Geo.Rows
	for d, s := range [4]int{-rows, rows, -1, 1} {
		if c-a == s {
			return b.arms[a]&(1<<d) != 0
		}
	}
	return false
}
