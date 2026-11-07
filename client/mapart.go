package client

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"orbitrix/core"
)

// The board, editor and brush picker share these physical map pieces. Callers
// cache their rendered layer; this renderer keeps no map-sized image or buffer.
type mapArtState struct {
	Map                 *core.MapDefinition
	Durability, Pressed []uint8
	OpenGates           [256]bool
	Owner               []int8
	Count               []uint8
}

type mapArtTransform struct {
	X, Y, Cell float64
	Turned     bool
}

type mapBrush struct {
	Terrain core.Terrain
	Feature core.Feature
	Edge    int // -1: cell, 0: Barrier, 1: Gate
}

func partnerColor(sc *scheme, group uint8) color.RGBA {
	if group == 0 || group > core.MaxMapGroups {
		return sc.dim
	}
	return sc.players[group-1]
}

func drawPartnerMark(dst *ebiten.Image, sc *scheme, group uint8, x, y, size float64, _ bool) {
	if size <= 0 {
		return
	}
	c := partnerColor(sc, group)
	coreInk := mix(sc.ink, sc.dim, .18)
	if luminance(sc.ground) < .25 {
		coreInk = mix(sc.ground, sc.dim, .18)
	}
	cell := size * .72 / (1 - 2*tileGutter)
	body, rad := mapTileGeometry(rect{x - cell/2, y - cell/2, cell, cell})
	width := mapFeatureStroke(cell)
	fillRound(dst, body, rad, coreInk)
	strokeRound(dst, body, rad, width, c)
	// Inset portal bends inherit the frame radius, with at least one stroke of rounding.
	for _, arc := range [][3]float64{{.25, -.15, 1}, {.13, -.04, -1}} {
		a, end, direction := size*arc[0], size*arc[1], arc[2]
		radius := min(a, max(width, rad-(body.w/2-a)))
		p := &scratchPath
		p.Reset()
		p.MoveTo(float32(x-a), float32(y+direction*end))
		p.LineTo(float32(x-a), float32(y+direction*(a-radius)))
		p.QuadTo(float32(x-a), float32(y+direction*a), float32(x-a+radius), float32(y+direction*a))
		p.LineTo(float32(x+a-radius), float32(y+direction*a))
		p.QuadTo(float32(x+a), float32(y+direction*a), float32(x+a), float32(y+direction*(a-radius)))
		p.LineTo(float32(x+a), float32(y+direction*end))
		pathOp = vector.DrawPathOptions{AntiAlias: true}
		tint(&pathOp.ColorScale, c)
		strokeOp.Width = float32(width)
		vector.StrokePath(dst, p, &strokeOp, &pathOp)
	}

}

func mapFeatureStroke(cell float64) float64 { return max(.8, cell*.04) }

func mapTileGeometry(r rect) (rect, float64) {
	return r.inset(r.w * tileGutter), r.w * tileRadius
}

// Floor and device geometry share the tile footprint; every switch uses this inset.
func mapDeviceGeometry(r rect) (body, inner rect, radius, innerRadius, width float64) {
	body, radius = mapTileGeometry(r)
	inset := body.w * .1875
	inner = body.inset(inset)
	innerRadius = max(0, radius-inset)
	width = mapFeatureStroke(r.w)
	return
}

func drawMapHoldFrame(dst *ebiten.Image, body rect, radius, width float64, ink color.RGBA) {
	// Clip the shared rounded frame into four brackets without painting over the floor.
	span := body.w * .34
	for _, corner := range [][2]float64{{body.x, body.y}, {body.right() - span, body.y}, {body.x, body.bottom() - span}, {body.right() - span, body.bottom() - span}} {
		clip := image.Rect(int(math.Floor(corner[0])), int(math.Floor(corner[1])), int(math.Ceil(corner[0]+span)), int(math.Ceil(corner[1]+span)))
		strokeRound(dst.SubImage(clip).(*ebiten.Image), body, radius, width, ink)
	}
}

func mapReservoirColor(sc *scheme, state mapArtState, i int) color.RGBA {
	if i >= 0 && i < len(state.Owner) && i < len(state.Count) && state.Count[i] > 0 {
		owner := int(state.Owner[i])
		if owner >= 0 && owner < len(sc.players) {
			return sc.players[owner]
		}
	}
	return sc.dim
}

func (tr mapArtTransform) point(rows int, x, y float64) (float64, float64) {
	if tr.Turned {
		x, y = float64(rows)-y, x
	}
	return tr.X + x*tr.Cell, tr.Y + y*tr.Cell
}

func mapArtTerrain(state *mapArtState, i int) core.Terrain {
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

// Four local lookups connect both kinds of wall, even across tile gutters.
func mapArtWallMask(state *mapArtState, i int) (mask uint8) {
	m := state.Map
	if m == nil || m.Rows <= 0 || i < 0 || i >= len(m.Cells) {
		return
	}
	c, r := i/m.Rows, i%m.Rows
	for d, step := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		x, y := c+step[0], r+step[1]
		if x >= 0 && x < m.Cols && y >= 0 && y < m.Rows && mapArtSolid(mapArtTerrain(state, x*m.Rows+y)) {
			mask |= 1 << d
		}
	}
	return
}

func mapArtWallColor(sc *scheme, terrain core.Terrain) color.RGBA {
	if terrain == core.BreakableWall {
		return mix(sc.dim, sc.tile, 0.23)
	}
	return sc.dim
}

func drawMapWall(dst *ebiten.Image, sc *scheme, r rect, terrain core.Terrain, mask uint8, durability uint8) {
	k, g := r.w, r.w*tileGutter
	c := mapArtWallColor(sc, terrain)
	// Overlap shared edges by one render pixel; antialiased rectangles that only touch leave a dark seam.
	const overlap = 1.0
	fillRound(dst, r.inset(g), k*0.14, c)
	if mask&mapNorth != 0 {
		fillRound(dst, rect{r.x + g, r.y - overlap, k - 2*g, k*0.3 + overlap}, 0, c)
	}
	if mask&mapEast != 0 {
		fillRound(dst, rect{r.right() - k*0.3, r.y + g, k*0.3 + overlap, k - 2*g}, 0, c)
	}
	if mask&mapSouth != 0 {
		fillRound(dst, rect{r.x + g, r.bottom() - k*0.3, k - 2*g, k*0.3 + overlap}, 0, c)
	}
	if mask&mapWest != 0 {
		fillRound(dst, rect{r.x - overlap, r.y + g, k*0.3 + overlap, k - 2*g}, 0, c)
	}
	if terrain == core.Wall {
		return
	}
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
		p := &scratchPath
		p.Reset()
		for i, point := range split {
			// Grow the connected fracture as a whole after damage, keeping forks attached.
			x, y := point[0]*spread, point[1]*spread
			px, py := float32(r.cx()+x*k), float32(r.cy()+y*k)
			if i == 0 {
				p.MoveTo(px, py)
			} else {
				p.LineTo(px, py)
			}
		}
		p.Close()
		fillPath(dst, p, sc.ground)
	}
}

// Fill enclosed vertices and round concave corners after all tiles, so draw order cannot clip the bend.
func drawMapWallCorners(dst *ebiten.Image, sc *scheme, state *mapArtState, tr mapArtTransform) {
	m := state.Map
	for gx := 1; gx < m.Cols; gx++ {
		for gy := 1; gy < m.Rows; gy++ {
			terrain, count := mapArtWallVertex(state, gx, gy)
			if count < 3 {
				continue
			}
			for corner, d := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				sx, sy := d[0], d[1]
				if count == 4 {
					x, y := tr.point(m.Rows, float64(gx)+sx*tileGutter/2, float64(gy)+sy*tileGutter/2)
					g := tr.Cell * tileGutter
					fillRound(dst, rect{x - g/2 - 1, y - g/2 - 1, g + 2, g + 2}, 0, mapArtWallColor(sc, terrain[corner]))
				} else if !mapArtSolid(terrain[corner]) {
					x, y := float64(gx)-sx*tileGutter, float64(gy)-sy*tileGutter
					px, py := tr.point(m.Rows, x, y)
					ax, ay := tr.point(m.Rows, x, y+sy*.14)
					bx, by := tr.point(m.Rows, x+sx*.14, y)
					p := &scratchPath
					p.Reset()
					// Extend into the existing arms to avoid antialiased seams at the patch boundary.
					overlap := .75 / tr.Cell
					ix, iy := tr.point(m.Rows, x-sx*overlap, y-sy*overlap)
					ex, ey := tr.point(m.Rows, x-sx*overlap, y+sy*.14)
					fx, fy := tr.point(m.Rows, x+sx*.14, y-sy*overlap)
					p.MoveTo(float32(ix), float32(iy))
					p.LineTo(float32(ex), float32(ey))
					p.LineTo(float32(ax), float32(ay))
					p.QuadTo(float32(px), float32(py), float32(bx), float32(by))
					p.LineTo(float32(fx), float32(fy))
					p.Close()
					fillPath(dst, p, mapArtWallColor(sc, terrain[(corner+2)%4]))
				}
			}
		}
	}
}

func mapArtWallVertex(state *mapArtState, gx, gy int) (terrain [4]core.Terrain, count int) {
	m := state.Map
	for i, d := range [4][2]int{{-1, -1}, {0, -1}, {0, 0}, {-1, 0}} {
		terrain[i] = mapArtTerrain(state, (gx+d[0])*m.Rows+gy+d[1])
		if mapArtSolid(terrain[i]) {
			count++
		}
	}
	return
}

func drawMapFeature(dst *ebiten.Image, sc *scheme, feature core.Feature, group uint8, r rect, pressed, _ bool) {
	x, y := r.cx(), r.cy()
	c := partnerColor(sc, group)
	body, inner, radius, innerRadius, width := mapDeviceGeometry(r)
	switch feature {
	case core.Link:
		drawPartnerMark(dst, sc, group, x, y, body.w/.72, false)
	case core.Reservoir:
		// A recessed square well seen from above; the inset is the reserve pocket.
		strokeRound(dst, body, radius, width, sc.dim)
		pocket := body.inset(body.w * (.30 - .13) / .60)
		strokeRound(dst, pocket, 0, width, sc.dim)
		// Four approach channels distinguish storage from a switch face.
		for _, d := range [][2]float64{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			line(dst, x+d[0]*pocket.w/2, y+d[1]*pocket.h/2,
				x+d[0]*(body.w/2-2*width), y+d[1]*(body.h/2-2*width), width, sc.dim)
		}
	case core.Switch:
		strokeRound(dst, body, radius, width, sc.dim)
		if pressed {
			fillRound(dst, inner, innerRadius, c)
		} else {
			strokeRound(dst, inner, innerRadius, width, c)
		}
	case core.ToggleSwitch:
		strokeRound(dst, body, radius, width, sc.dim)
		filled := inner
		filled.w /= 2
		if pressed {
			filled.x += filled.w
		}
		clip := image.Rect(int(math.Floor(filled.x)), int(math.Floor(filled.y)), int(math.Ceil(filled.right())), int(math.Ceil(filled.bottom())))
		fillRound(dst.SubImage(clip).(*ebiten.Image), inner, innerRadius, c)
		strokeRound(dst, inner, innerRadius, width, c)
		line(dst, inner.cx(), inner.y+width/2, inner.cx(), inner.bottom()-width/2, width, c)
	case core.HoldSwitch:
		// Four corner brackets surround the occupancy plate in plan view.
		drawMapHoldFrame(dst, body, radius, width, sc.dim)
		if pressed {
			fillRound(dst, inner, innerRadius, c)
		} else {
			strokeRound(dst, inner, innerRadius, width, c)
		}
	}
}

// mapArtEdge gives the actual shared grid boundary, including its vertices.
func mapArtEdge(m *core.MapDefinition, e core.MapEdge) (x0, y0, x1, y1 float64, ok bool) {
	if m == nil || m.Rows <= 0 || e.A < 0 || e.B < 0 || e.A >= len(m.Cells) || e.B >= len(m.Cells) {
		return
	}
	ac, ar, bc, br := e.A/m.Rows, e.A%m.Rows, e.B/m.Rows, e.B%m.Rows
	if ar == br && absInt(ac-bc) == 1 {
		x0, y0 = float64(max(ac, bc)), float64(ar)
		return x0, y0, x0, y0 + 1, true
	}
	if ac == bc && absInt(ar-br) == 1 {
		x0, y0 = float64(ac), float64(max(ar, br))
		return x0, y0, x0 + 1, y0, true
	}
	return
}

// Join an edge's vertex to each adjacent solid corner. This is constant work per
// endpoint, so even dense maps never scan the entire edge list for each cell.
func drawMapWallJoins(dst *ebiten.Image, sc *scheme, state *mapArtState, tr mapArtTransform, gx, gy float64) {
	m := state.Map
	x, y := tr.point(m.Rows, gx, gy)
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
		ex, ey := tr.point(m.Rows, gx+ox, gy+oy)
		line(dst, x, y, ex, ey, max(1, tr.Cell*0.105), mapArtWallColor(sc, t))
	}
}

func mapArtGateContinuesAt(m *core.MapDefinition, edges []core.MapEdge, index int, gx, gy float64) bool {
	if m == nil || index < 0 || index >= len(edges) {
		return false
	}
	e := edges[index]
	for j, other := range edges {
		if j == index || other.Kind != core.Gate || other.Group != e.Group {
			continue
		}
		ox0, oy0, ox1, oy1, ok := mapArtEdge(m, other)
		if ok && (ox0 == gx && oy0 == gy || ox1 == gx && oy1 == gy) {
			return true
		}
	}
	return false
}

func drawMapEdgePieceWithEnds(dst *ebiten.Image, sc *scheme, kind core.EdgeKind, group uint8, open, openStart, openEnd bool, x0, y0, x1, y1, k float64, _ bool) {
	dx, dy := x1-x0, y1-y0
	w := max(1.2, k*0.13)
	if kind == core.Barrier {
		line(dst, x0, y0, x1, y1, w, sc.dim)
		return
	}
	c := partnerColor(sc, group)
	if open {
		const openGateRun = .2
		if openStart {
			line(dst, x0, y0, x0+dx*openGateRun, y0+dy*openGateRun, w, c)
		}
		if openEnd {
			line(dst, x0+dx*(1-openGateRun), y0+dy*(1-openGateRun), x1, y1, w, c)
		}
		return
	}
	line(dst, x0, y0, x1, y1, w, c)
}

func drawMapEdgePiece(dst *ebiten.Image, sc *scheme, kind core.EdgeKind, group uint8, open bool, x0, y0, x1, y1, k float64, marks bool) {
	drawMapEdgePieceWithEnds(dst, sc, kind, group, open, true, true, x0, y0, x1, y1, k, marks)
}

func drawMapArt(dst *ebiten.Image, sc *scheme, state mapArtState, tr mapArtTransform, drawFloor, marks bool) {
	m := state.Map
	if m == nil || m.Rows <= 0 || m.Cols <= 0 || tr.Cell <= 0 {
		return
	}
	for i, cfg := range m.Cells {
		t := mapArtTerrain(&state, i)
		if t == core.Void {
			continue
		}
		x, y := tr.point(m.Rows, float64(i/m.Rows)+0.5, float64(i%m.Rows)+0.5)
		r := rect{x - tr.Cell/2, y - tr.Cell/2, tr.Cell, tr.Cell}
		if mapArtSolid(t) {
			mask := mapArtWallMask(&state, i)
			if tr.Turned {
				mask = (mask<<1 | mask>>3) & 15
			}
			durability := uint8(2)
			if i < len(state.Durability) {
				durability = state.Durability[i]
			}
			drawMapWall(dst, sc, r, t, mask, durability)
			continue
		}
		if drawFloor {
			body, radius := mapTileGeometry(r)
			fillRound(dst, body, radius, sc.tile)
		}
		pressed := i < len(state.Pressed) && state.Pressed[i] != 0
		featureScheme := *sc
		if cfg.Feature == core.Reservoir {
			featureScheme.dim = mapReservoirColor(sc, state, i)
		}
		drawMapFeature(dst, &featureScheme, cfg.Feature, cfg.Group, r, pressed, marks)
	}
	drawMapWallCorners(dst, sc, &state, tr)
	for edgeIndex, e := range m.Edges {
		gx0, gy0, gx1, gy1, ok := mapArtEdge(m, e)
		if !ok {
			continue
		}
		drawMapWallJoins(dst, sc, &state, tr, gx0, gy0)
		drawMapWallJoins(dst, sc, &state, tr, gx1, gy1)
		x0, y0 := tr.point(m.Rows, gx0, gy0)
		x1, y1 := tr.point(m.Rows, gx1, gy1)
		open := state.OpenGates[e.Group]
		openStart, openEnd := true, true
		if open {
			openStart = !mapArtGateContinuesAt(m, m.Edges, edgeIndex, gx0, gy0)
			openEnd = !mapArtGateContinuesAt(m, m.Edges, edgeIndex, gx1, gy1)
		}
		drawMapEdgePieceWithEnds(dst, sc, e.Kind, e.Group, open, openStart, openEnd, x0, y0, x1, y1, tr.Cell, marks)
	}
}

func drawMapBrush(dst *ebiten.Image, sc *scheme, brush mapBrush, r rect, group uint8, pressed, marks bool) {
	k := min(r.w, r.h)
	if k <= 0 {
		return
	}
	r = rect{r.cx() - k/2, r.cy() - k/2, k, k}
	if brush.Edge >= 0 {
		fillRound(dst, r.inset(k*tileGutter), k*tileRadius, sc.tile)
		drawMapEdgePiece(dst, sc, core.EdgeKind(brush.Edge), group, pressed, r.x+k*tileGutter, r.cy(), r.right()-k*tileGutter, r.cy(), k, marks)
		return
	}
	if brush.Terrain == core.Void {
		return
	}
	if mapArtSolid(brush.Terrain) {
		drawMapWall(dst, sc, r, brush.Terrain, 0, 2)
		return
	}
	body, radius := mapTileGeometry(r)
	fillRound(dst, body, radius, sc.tile)
	drawMapFeature(dst, sc, brush.Feature, group, r, pressed, marks)
}
