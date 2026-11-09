package boardview

import (
	"image"
	"image/color"
	"math"

	"orbitrix/client/internal/ui"
	"orbitrix/core"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var mapPath vector.Path

// The board, editor and brush picker share these physical map pieces. Callers
// cache their rendered layer; this renderer keeps no map-sized image or buffer.
type MapState struct {
	Map                 *core.MapDefinition
	Durability, Pressed []uint8
	EdgeDurability      []uint8
	CircuitOn           [256]bool
	Owner               []int8
	Count               []uint8
	PortalAxes          []core.PortalAxes
}

type MapTransform struct {
	X, Y, Cell float64
	Turned     bool
}

type Brush struct {
	Terrain    core.Terrain
	Feature    core.Feature
	Circuit    core.CircuitKind
	Edge       int // -1: cell; otherwise core.EdgeKind
	PortalMode core.PortalMode
}

func PartnerColor(sc *ui.Palette, group uint8) color.RGBA {
	if group == 0 || group > core.MaxMapGroups {
		return sc.Dim
	}
	return sc.Players[group-1]
}

func drawPartnerMark(dst *ebiten.Image, sc *ui.Palette, group uint8, x, y, size float64, portal bool, axes core.PortalAxes) {
	if size <= 0 {
		return
	}
	c := PartnerColor(sc, group)
	coreInk := ui.Mix(sc.Ink, sc.Dim, .18)
	if ui.Luminance(sc.Ground) < .25 {
		coreInk = ui.Mix(sc.Ground, sc.Dim, .18)
	}
	cell := size * .72 / (1 - 2*TileGutter)
	body, rad := mapTileGeometry(ui.Rect{X: x - cell/2, Y: y - cell/2, W: cell, H: cell})
	width := mapFeatureStroke(cell)
	ui.FillRound(dst, body, rad, coreInk)
	ui.StrokeRound(dst, body, rad, width, c)
	if portal {
		for axis := range 2 {
			if axes&core.PortalAxes(1<<axis) == 0 {
				continue
			}
			for _, side := range []float64{-1, 1} {
				tip, rim, span, bend := .10, .25, .28, .19
				if axes == core.BothAxes {
					tip, rim, span, bend = .23, .30, .17, .10
				}
				p := &mapPath
				p.Reset()
				for j, point := range [4][2]float64{{side * tip, -span}, {side * rim, -bend}, {side * rim, bend}, {side * tip, span}} {
					dx, dy := point[0], point[1]
					if axis == 1 {
						dx, dy = dy, dx
					}
					if j == 0 {
						p.MoveTo(float32(x+dx*body.W), float32(y+dy*body.W))
					} else {
						p.LineTo(float32(x+dx*body.W), float32(y+dy*body.W))
					}
				}
				ui.StrokePath(dst, p, width, c)
			}
		}
		return
	}
	// Inset portal bends inherit the frame radius, with at least one stroke of rounding.
	for _, arc := range [][3]float64{{.25, -.15, 1}, {.13, -.04, -1}} {
		a, end, direction := size*arc[0], size*arc[1], arc[2]
		radius := min(a, max(width, rad-(body.W/2-a)))
		p := &mapPath
		p.Reset()
		p.MoveTo(float32(x-a), float32(y+direction*end))
		p.LineTo(float32(x-a), float32(y+direction*(a-radius)))
		p.QuadTo(float32(x-a), float32(y+direction*a), float32(x-a+radius), float32(y+direction*a))
		p.LineTo(float32(x+a-radius), float32(y+direction*a))
		p.QuadTo(float32(x+a), float32(y+direction*a), float32(x+a), float32(y+direction*(a-radius)))
		p.LineTo(float32(x+a), float32(y+direction*end))
		ui.StrokePath(dst, p, width, c)
	}

}

func mapFeatureStroke(cell float64) float64 { return max(.8, cell*.04) }

func mapTileGeometry(r ui.Rect) (ui.Rect, float64) {
	return r.Inset(r.W * TileGutter), r.W * TileRadius
}

// Floor and device geometry share the tile footprint; every switch uses this inset.
func mapDeviceGeometry(r ui.Rect) (body, inner ui.Rect, radius, innerRadius, width float64) {
	body, radius = mapTileGeometry(r)
	inset := body.W * .1875
	inner = body.Inset(inset)
	innerRadius = max(0, radius-inset)
	width = mapFeatureStroke(r.W)
	return
}

func drawMapHoldFrame(dst *ebiten.Image, body ui.Rect, radius, width float64, ink color.RGBA) {
	// Clip the shared rounded frame into four brackets without painting over the floor.
	span := body.W * .34
	for _, corner := range [][2]float64{{body.X, body.Y}, {body.Right() - span, body.Y}, {body.X, body.Bottom() - span}, {body.Right() - span, body.Bottom() - span}} {
		clip := image.Rect(int(math.Floor(corner[0])), int(math.Floor(corner[1])), int(math.Ceil(corner[0]+span)), int(math.Ceil(corner[1]+span)))
		ui.StrokeRound(dst.SubImage(clip).(*ebiten.Image), body, radius, width, ink)
	}
}

func mapReservoirColor(sc *ui.Palette, state MapState, i int) color.RGBA {
	if i >= 0 && i < len(state.Owner) && i < len(state.Count) && state.Count[i] > 0 {
		owner := int(state.Owner[i])
		if owner >= 0 && owner < len(sc.Players) {
			return sc.Players[owner]
		}
	}
	return sc.Dim
}

func (tr MapTransform) Point(rows int, x, y float64) (float64, float64) {
	if tr.Turned {
		x, y = float64(rows)-y, x
	}
	return tr.X + x*tr.Cell, tr.Y + y*tr.Cell
}

func mapArtTerrain(state *MapState, i int) core.Terrain {
	if state.Map == nil || i < 0 || i >= len(state.Map.Cells) {
		return core.Void
	}
	t := state.Map.Cells[i].Terrain
	if t == core.BreakableWall && state.Durability != nil && i < len(state.Durability) && state.Durability[i] == 0 {
		return core.Floor
	}
	return t
}

const (
	mapNorth uint8 = 1 << iota
	mapEast
	mapSouth
	mapWest
)

func mapArtSolid(t core.Terrain) bool { return t == core.Wall || t == core.BreakableWall }

const (
	mapJoinWall uint8 = 1 + iota
	mapJoinHole
)

// mapArtJoin groups terrain that merges across gutters: both kinds of wall, or Holes. Zero never joins.
func mapArtJoin(t core.Terrain) uint8 {
	switch {
	case mapArtSolid(t):
		return mapJoinWall
	case t == core.Hole:
		return mapJoinHole
	}
	return 0
}

// Four local lookups connect joining terrain, even across tile gutters.
func mapArtWallMask(state *MapState, i int) (mask uint8) {
	m := state.Map
	if m == nil || m.Rows <= 0 || i < 0 || i >= len(m.Cells) {
		return
	}
	join := mapArtJoin(mapArtTerrain(state, i))
	c, r := i/m.Rows, i%m.Rows
	for d, step := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		x, y := c+step[0], r+step[1]
		if join != 0 && x >= 0 && x < m.Cols && y >= 0 && y < m.Rows && mapArtJoin(mapArtTerrain(state, x*m.Rows+y)) == join {
			mask |= 1 << d
		}
	}
	return
}

func mapArtWallColor(sc *ui.Palette, terrain core.Terrain) color.RGBA {
	switch terrain {
	case core.BreakableWall:
		return ui.Mix(sc.Dim, sc.Tile, 0.23)
	case core.Hole:
		return sc.Pit
	}
	return sc.Dim
}

// DrawMapWall draws a wall or Hole, extended across the gutters named by mask.
func DrawMapWall(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, terrain core.Terrain, mask uint8, durability uint8) {
	k, g := r.W, r.W*TileGutter
	c := mapArtWallColor(sc, terrain)
	// Overlap shared edges by one render pixel; antialiased rectangles that only touch leave a dark seam.
	const overlap = 1.0
	radius := k * 0.14
	if terrain == core.Hole {
		radius = k * TileRadius // a lone Hole keeps the Floor footprint
	}
	ui.FillRound(dst, r.Inset(g), radius, c)
	if mask&mapNorth != 0 {
		ui.FillRound(dst, ui.Rect{X: r.X + g, Y: r.Y - overlap, W: k - 2*g, H: k*0.3 + overlap}, 0, c)
	}
	if mask&mapEast != 0 {
		ui.FillRound(dst, ui.Rect{X: r.Right() - k*0.3, Y: r.Y + g, W: k*0.3 + overlap, H: k - 2*g}, 0, c)
	}
	if mask&mapSouth != 0 {
		ui.FillRound(dst, ui.Rect{X: r.X + g, Y: r.Bottom() - k*0.3, W: k - 2*g, H: k*0.3 + overlap}, 0, c)
	}
	if mask&mapWest != 0 {
		ui.FillRound(dst, ui.Rect{X: r.X - overlap, Y: r.Y + g, W: k*0.3 + overlap, H: k - 2*g}, 0, c)
	}
	if terrain == core.BreakableWall {
		drawMapSprite(dst, sc, r, mapSpriteKey{cracks: true, durability: durability})
	}
}

func renderMapCracks(dst *ebiten.Image, r ui.Rect, durability uint8, ink color.RGBA) {
	k := r.W
	spread := 1.0
	if durability == 1 {
		spread = 1.24
	}
	// A connected stress fracture: three tapered splits and two forks, rooted at the centre.
	for _, split := range [][][2]float64{
		{{-.022, .012}, {-.092, -.15}, {.09, -.35}, {-.048, -.145}, {.022, -.012}},
		{{.012, -.025}, {.18, .04}, {.35, -.035}, {.18, .084}, {-.012, .025}},
		{{.024, .008}, {-.04, .18}, {-.27, .34}, {-.082, .17}, {-.024, -.008}},
		{{-.075, -.13}, {-.28, -.22}, {-.056, -.17}},
		{{-.055, .165}, {.085, .34}, {-.092, .19}},
	} {
		p := &mapPath
		p.Reset()
		for i, point := range split {
			// Grow the connected fracture as a whole after damage, keeping forks attached.
			x, y := point[0]*spread, point[1]*spread
			px, py := float32(r.Cx()+x*k), float32(r.Cy()+y*k)
			if i == 0 {
				p.MoveTo(px, py)
			} else {
				p.LineTo(px, py)
			}
		}
		p.Close()
		ui.FillPath(dst, p, ink)
	}
}

// Fill enclosed vertices and round concave corners after all tiles, so draw order cannot clip the bend.
func drawMapWallCorners(dst *ebiten.Image, sc *ui.Palette, state *MapState, tr MapTransform) {
	m := state.Map
	for gx := 1; gx < m.Cols; gx++ {
		for gy := 1; gy < m.Rows; gy++ {
			for _, join := range [2]uint8{mapJoinWall, mapJoinHole} {
				terrain, count := mapArtWallVertex(state, gx, gy, join)
				if count < 3 {
					continue
				}
				for corner, d := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
					sx, sy := d[0], d[1]
					if count == 4 {
						x, y := tr.Point(m.Rows, float64(gx)+sx*TileGutter/2, float64(gy)+sy*TileGutter/2)
						g := tr.Cell * TileGutter
						ui.FillRound(dst, ui.Rect{X: x - g/2 - 1, Y: y - g/2 - 1, W: g + 2, H: g + 2}, 0, mapArtWallColor(sc, terrain[corner]))
					} else if mapArtJoin(terrain[corner]) != join {
						x, y := tr.Point(m.Rows, float64(gx), float64(gy))
						rotation := corner
						if tr.Turned {
							rotation = (rotation + 1) % 4
						}
						drawMapSprite(dst, sc, ui.Rect{X: x - tr.Cell/2, Y: y - tr.Cell/2, W: tr.Cell, H: tr.Cell},
							mapSpriteKey{terrain: terrain[(corner+2)%4], corner: uint8(rotation + 1)})
					}
				}
			}
		}
	}
}

func renderMapWallCorner(dst *ebiten.Image, r ui.Rect, ink color.RGBA, corner int) {
	d := [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}[corner]
	sx, sy := d[0], d[1]
	x, y := r.Cx()-sx*TileGutter*r.W, r.Cy()-sy*TileGutter*r.W
	p := &mapPath
	p.Reset()
	// Extend into both arms so antialiasing cannot leave a seam.
	p.MoveTo(float32(x-sx*.75), float32(y-sy*.75))
	p.LineTo(float32(x-sx*.75), float32(y+sy*.14*r.W))
	p.LineTo(float32(x), float32(y+sy*.14*r.W))
	p.QuadTo(float32(x), float32(y), float32(x+sx*.14*r.W), float32(y))
	p.LineTo(float32(x+sx*.14*r.W), float32(y-sy*.75))
	p.Close()
	ui.FillPath(dst, p, ink)
}

// mapArtWallVertex counts the cells around grid vertex (gx, gy) in join group join.
func mapArtWallVertex(state *MapState, gx, gy int, join uint8) (terrain [4]core.Terrain, count int) {
	m := state.Map
	for i, d := range [4][2]int{{-1, -1}, {0, -1}, {0, 0}, {-1, 0}} {
		terrain[i] = mapArtTerrain(state, (gx+d[0])*m.Rows+gy+d[1])
		if mapArtJoin(terrain[i]) == join {
			count++
		}
	}
	return
}

func renderMapFeature(dst *ebiten.Image, sc *ui.Palette, feature core.Feature, group uint8, axes core.PortalAxes, r ui.Rect) {
	x, y := r.Cx(), r.Cy()
	body, _, radius, _, width := mapDeviceGeometry(r)
	switch feature {
	case core.Link, core.Portal:
		drawPartnerMark(dst, sc, group, x, y, body.W/.72, feature == core.Portal, axes)
	case core.Reservoir:
		ui.StrokeRound( // A recessed square well seen from above; the inset is the reserve pocket.
			dst, body, radius, width, sc.Dim)
		pocket := body.Inset(body.W * (.30 - .13) / .60)
		ui.StrokeRound(dst, pocket, 0, width, sc.Dim)
		// Four approach channels distinguish storage from a switch face.
		for _, d := range [][2]float64{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			ui.Line(dst, x+d[0]*pocket.W/2, y+d[1]*pocket.H/2,
				x+d[0]*(body.W/2-2*width), y+d[1]*(body.H/2-2*width), width, sc.Dim)
		}
	}
}

func renderMapCircuit(dst *ebiten.Image, sc *ui.Palette, component core.CircuitKind, group uint8, r ui.Rect, pressed bool) {
	c := PartnerColor(sc, group)
	body, inner, radius, innerRadius, width := mapDeviceGeometry(r)
	switch component {
	case core.Door:
		ink := sc.Dim
		if !pressed {
			ui.FillRound(dst, body, radius, c)
			ink = ui.InkOn(c)
		}
		ui.StrokePath(dst, mapDoorLockPath(inner, width, pressed), width, ink)
	case core.Latch:
		ui.StrokeRound(dst, body, radius, width, sc.Dim)
		if pressed {
			ui.FillRound(dst, inner, innerRadius, c)
		} else {
			ui.StrokeRound(dst, inner, innerRadius, width, c)
		}
	case core.Toggle:
		ui.StrokeRound(dst, body, radius, width, sc.Dim)
		filled := inner
		filled.W /= 2
		if pressed {
			filled.X += filled.W
		}
		clip := image.Rect(int(math.Floor(filled.X)), int(math.Floor(filled.Y)), int(math.Ceil(filled.Right())), int(math.Ceil(filled.Bottom())))
		ui.FillRound(dst.SubImage(clip).(*ebiten.Image), inner, innerRadius, c)
		ui.StrokeRound(dst, inner, innerRadius, width, c)
		ui.Line(dst, inner.Cx(), inner.Y+width/2, inner.Cx(), inner.Bottom()-width/2, width, c)
	case core.Hold:
		// Four corner brackets surround the occupancy plate in plan view.
		drawMapHoldFrame(dst, body, radius, width, sc.Dim)
		if pressed {
			ui.FillRound(dst, inner, innerRadius, c)
		} else {
			ui.StrokeRound(dst, inner, innerRadius, width, c)
		}
	}
}

func mapDoorLockPath(inner ui.Rect, width float64, open bool) *vector.Path {
	// One vector outline keeps both states crisp within the same centered square.
	lock := inner.Inset(width / 2)
	lock.X += inner.W * .08
	lock.W -= inner.W * .16
	x0, x1 := float32(lock.X), float32(lock.Right())
	y0, y1 := float32(inner.Y+inner.H*.46+width/2), float32(lock.Bottom())
	bend := float32(width)
	p := &mapPath
	p.Reset()
	p.MoveTo(x0+bend, y0)
	p.LineTo(x1-bend, y0)
	p.ArcTo(x1, y0, x1, y0+bend, bend)
	p.LineTo(x1, y1-bend)
	p.ArcTo(x1, y1, x1-bend, y1, bend)
	p.LineTo(x0+bend, y1)
	p.ArcTo(x0, y1, x0, y1-bend, bend)
	p.LineTo(x0, y0+bend)
	p.ArcTo(x0, y0, x0+bend, y0, bend)
	p.Close()
	side, top := float32(lock.W*.20), float32(lock.Y)
	p.MoveTo(x0+side, y0)
	p.LineTo(x0+side, top+bend)
	p.ArcTo(x0+side, top, x0+side+bend, top, bend)
	p.LineTo(x1-side-bend, top)
	p.ArcTo(x1-side, top, x1-side, top+bend, bend)
	if open {
		y0 -= float32(2.5 * width)
	}
	p.LineTo(x1-side, y0)
	return p
}

// mapArtEdge gives the actual shared grid boundary, including its vertices.
func MapArtEdge(m *core.MapDefinition, e core.MapEdge) (x0, y0, x1, y1 float64, ok bool) {
	if m == nil || m.Rows <= 0 || e.A < 0 || e.B < 0 || e.A >= len(m.Cells) || e.B >= len(m.Cells) {
		return
	}
	ac, ar, bc, br := e.A/m.Rows, e.A%m.Rows, e.B/m.Rows, e.B%m.Rows
	if ar == br && ui.AbsInt(ac-bc) == 1 {
		x0, y0 = float64(max(ac, bc)), float64(ar)
		return x0, y0, x0, y0 + 1, true
	}
	if ac == bc && ui.AbsInt(ar-br) == 1 {
		x0, y0 = float64(ac), float64(max(ar, br))
		return x0, y0, x0 + 1, y0, true
	}
	return
}

// Join an edge's vertex to each adjacent solid corner. This is constant work per
// endpoint, so even dense maps never scan the entire edge list for each cell.
func drawMapWallJoins(dst *ebiten.Image, sc *ui.Palette, state *MapState, tr MapTransform, gx, gy float64) {
	m := state.Map
	x, y := tr.Point(m.Rows, gx, gy)
	for _, d := range [4][2]int{{-1, -1}, {0, -1}, {0, 0}, {-1, 0}} {
		c, r := int(gx)+d[0], int(gy)+d[1]
		if c < 0 || c >= m.Cols || r < 0 || r >= m.Rows {
			continue
		}
		t := mapArtTerrain(state, c*m.Rows+r)
		if !mapArtSolid(t) {
			continue
		}
		ox, oy := 0.18, 0.18
		if d[0] < 0 {
			ox = -ox
		}
		if d[1] < 0 {
			oy = -oy
		}
		ex, ey := tr.Point(m.Rows, gx+ox, gy+oy)
		ui.Line(dst, x, y, ex, ey, max(1, tr.Cell*0.105), mapArtWallColor(sc, t))
	}
}

func mapArtGateContinuesAt(m *core.MapDefinition, edges []core.MapEdge, index int, gx, gy float64) bool {
	if m == nil || index < 0 || index >= len(edges) {
		return false
	}
	e := edges[index]
	for j, other := range edges {
		if j == index || other.Kind != core.Gate || other.Group != e.Group || other.InitiallyOpen != e.InitiallyOpen {
			continue
		}
		ox0, oy0, ox1, oy1, ok := MapArtEdge(m, other)
		if ok && (ox0 == gx && oy0 == gy || ox1 == gx && oy1 == gy) {
			return true
		}
	}
	return false
}

func drawMapEdgePieceWithEnds(dst *ebiten.Image, sc *ui.Palette, kind core.EdgeKind, group, durability uint8, open, openStart, openEnd bool, x0, y0, x1, y1, k float64, _ bool) {
	dx, dy := x1-x0, y1-y0
	w := max(1.2, k*0.13)
	if kind == core.Barrier || kind == core.BreakableBarrier {
		ink := sc.Dim
		if kind == core.BreakableBarrier {
			ink = mapArtWallColor(sc, core.BreakableWall)
		}
		ui.Line(dst, x0, y0, x1, y1, w, ink)
		if kind == core.BreakableBarrier {
			spread := 1.0
			if durability == 1 {
				spread = 1.5
			}
			// A transverse fracture follows the edge's orientation and widens after a hit.
			points := [4][2]float64{{-.035, -.075}, {.025, -.02}, {-.025, .025}, {.035, .075}}
			for i := 1; i < len(points); i++ {
				a, b := points[i-1], points[i]
				ui.Line(dst, (x0+x1)/2+dx*a[0]*spread-dy*a[1], (y0+y1)/2+dy*a[0]*spread+dx*a[1],
					(x0+x1)/2+dx*b[0]*spread-dy*b[1], (y0+y1)/2+dy*b[0]*spread+dx*b[1], max(.8, k*.025)*spread, sc.Ground)
			}
		}
		return
	}
	c := PartnerColor(sc, group)
	if open {
		const openGateRun = .2
		if openStart {
			ui.Line(dst, x0, y0, x0+dx*openGateRun, y0+dy*openGateRun, w, c)
		}
		if openEnd {
			ui.Line(dst, x0+dx*(1-openGateRun), y0+dy*(1-openGateRun), x1, y1, w, c)
		}
		return
	}
	ui.Line(dst, x0, y0, x1, y1, w, c)
}

func DrawMapEdgePiece(dst *ebiten.Image, sc *ui.Palette, kind core.EdgeKind, group uint8, open bool, x0, y0, x1, y1, k float64, marks bool) {
	drawMapEdgePieceWithEnds(dst, sc, kind, group, 2, open, true, true, x0, y0, x1, y1, k, marks)
}

func DrawMapArt(dst *ebiten.Image, sc *ui.Palette, state MapState, tr MapTransform, drawFloor, marks bool) {
	m := state.Map
	if m == nil || m.Rows <= 0 || m.Cols <= 0 || m.Rows > core.MaxSize || m.Cols > core.MaxSize || tr.Cell <= 0 {
		return
	}
	if state.PortalAxes == nil {
		state.PortalAxes = m.InitialPortalAxes()
	}
	for i, cfg := range m.Cells {
		t := mapArtTerrain(&state, i)
		if t == core.Void {
			continue
		}
		x, y := tr.Point(m.Rows, float64(i/m.Rows)+0.5, float64(i%m.Rows)+0.5)
		r := ui.Rect{X: x - tr.Cell/2, Y: y - tr.Cell/2, W: tr.Cell, H: tr.Cell}
		if mapArtJoin(t) != 0 {
			mask := mapArtWallMask(&state, i)
			if tr.Turned {
				mask = (mask<<1 | mask>>3) & 15
			}
			durability := uint8(2)
			if i < len(state.Durability) {
				durability = state.Durability[i]
			}
			DrawMapWall(dst, sc, r, t, mask, durability)
			continue
		}
		if drawFloor {
			body, radius := mapTileGeometry(r)
			ui.FillRound(dst, body, radius, sc.Tile)
		}
		pressed := i < len(state.Pressed) && state.Pressed[i] != 0
		featureScheme := *sc
		if cfg.Feature == core.Reservoir {
			featureScheme.Dim = mapReservoirColor(sc, state, i)
		}
		axes := core.BothAxes
		if cfg.Feature == core.Portal {
			axes = state.PortalAxes[i]
			if tr.Turned {
				axes = (axes<<1 | axes>>1) & core.BothAxes
			}
		}
		drawMapFeature(dst, &featureScheme, cfg.Feature, cfg.Group, axes, r)
		if cfg.Circuit.IsOutput() {
			pressed = core.CircuitOpen(cfg.InitiallyOpen, state.CircuitOn[cfg.Group])
		}
		drawMapCircuit(dst, sc, cfg.Circuit, cfg.Group, r, pressed)
	}
	drawMapWallCorners(dst, sc, &state, tr)
	shared := mapGateVertices(m)
	for edgeIndex, e := range m.Edges {
		durability := uint8(2)
		if e.Kind == core.BreakableBarrier && edgeIndex < len(state.EdgeDurability) {
			durability = state.EdgeDurability[edgeIndex]
			if durability == 0 {
				continue
			}
		}
		gx0, gy0, gx1, gy1, ok := MapArtEdge(m, e)
		if !ok {
			continue
		}
		drawMapWallJoins(dst, sc, &state, tr, gx0, gy0)
		drawMapWallJoins(dst, sc, &state, tr, gx1, gy1)
		x0, y0 := tr.Point(m.Rows, gx0, gy0)
		x1, y1 := tr.Point(m.Rows, gx1, gy1)
		open := e.Kind == core.Gate && core.CircuitOpen(e.InitiallyOpen, state.CircuitOn[e.Group])
		openStart, openEnd := true, true
		if open {
			if e.Group <= core.MaxMapGroups {
				bit := mapGateBit(e)
				openStart = shared[int(gx0)*(m.Rows+1)+int(gy0)]&bit == 0
				openEnd = shared[int(gx1)*(m.Rows+1)+int(gy1)]&bit == 0
			} else {
				openStart = !mapArtGateContinuesAt(m, m.Edges, edgeIndex, gx0, gy0)
				openEnd = !mapArtGateContinuesAt(m, m.Edges, edgeIndex, gx1, gy1)
			}
		}
		drawMapEdgePieceWithEnds(dst, sc, e.Kind, e.Group, durability, open, openStart, openEnd, x0, y0, x1, y1, tr.Cell, marks)
	}
}

func mapGateBit(e core.MapEdge) uint32 {
	bit := uint32(1) << (2 * e.Group)
	if e.InitiallyOpen {
		bit <<= 1
	}
	return bit
}

// Count matching endpoints once, rather than scanning all edges for every open Gate.
func mapGateVertices(m *core.MapDefinition) (shared [(core.MaxSize + 1) * (core.MaxSize + 1)]uint32) {
	var seen [len(shared)]uint32
	for _, e := range m.Edges {
		if e.Kind != core.Gate || e.Group > core.MaxMapGroups {
			continue
		}
		x0, y0, x1, y1, ok := MapArtEdge(m, e)
		if !ok {
			continue
		}
		bit := mapGateBit(e)
		for _, v := range [2]int{int(x0)*(m.Rows+1) + int(y0), int(x1)*(m.Rows+1) + int(y1)} {
			shared[v] |= seen[v] & bit
			seen[v] |= bit
		}
	}
	return
}

func DrawMapBrush(dst *ebiten.Image, sc *ui.Palette, brush Brush, r ui.Rect, group uint8, pressed, marks bool) {
	k := min(r.W, r.H)
	if k <= 0 {
		return
	}
	r = ui.Rect{X: r.Cx() - k/2, Y: r.Cy() - k/2, W: k, H: k}
	if brush.Edge >= 0 {
		ui.FillRound(dst, r.Inset(k*TileGutter), k*TileRadius, sc.Tile)
		DrawMapEdgePiece(dst, sc, core.EdgeKind(brush.Edge), group, pressed, r.X+k*TileGutter, r.Cy(), r.Right()-k*TileGutter, r.Cy(), k, marks)
		return
	}
	if brush.Terrain == core.Void {
		return
	}
	if mapArtJoin(brush.Terrain) != 0 {
		DrawMapWall(dst, sc, r, brush.Terrain, 0, 2)
		return
	}
	body, radius := mapTileGeometry(r)
	ui.FillRound(dst, body, radius, sc.Tile)
	drawMapFeature(dst, sc, brush.Feature, group, brush.PortalMode.Axes(), r)
	drawMapCircuit(dst, sc, brush.Circuit, group, r, pressed)
}
