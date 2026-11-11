// Package boardview draws game boards and replays resolved moves wave by wave.
// It depends on game state and UI primitives, never on application scenes.
package boardview

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/sound"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
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
	TileGutter    = 0.07 // gap on each side of a tile, as a fraction of the cell
	TileRadius    = 0.2  // tile corner radius, as a fraction of the cell
	tileTint      = 0.3  // how much of the player's ink their tiles take on their turn
	burstSrcR     = 64   // radius of the burst ring source image
	fallTicks     = 20   // an orb sent into a Hole drops from its centre and vanishes
	landTicks     = 4    // an orb or Box filling its pit shows on the bottom this long, landed, before the pit is Floor
)

// fallingOrb drops into a pit; in a Landfill it settles on the bottom instead of fading away.
type fallingOrb struct {
	hole      int16
	owner     int8
	start     float64 // Board.Clock when it landed
	landfill  bool
	remaining uint8 // Landfill state once this fall reaches the bottom
	slot      uint8 // its place on the bottom's orbit, so orbs sharing a bottom stay apart
	box       bool  // a Box sinking into the pit it fills
}

// ticks is how long the orb or Box is shown: its fall, and for one filling its pit a moment on the bottom.
func (o fallingOrb) ticks() float64 {
	if o.box || o.landfill && o.remaining == 0 {
		return fallTicks + landTicks
	}
	return fallTicks
}

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
type Board struct {
	State *view
	Orbs  *orbField

	// Playback of the move in progress.
	Waves     []core.Wave
	Wave      int     // wave in flight
	t, dur    float64 // ticks into it, and its length
	Playing   bool
	settling  bool // a completing Landfill fall finishes before the next wave or turn
	mover     int8
	produceAt int // waves shown before the mover's Furnaces and Crushers run; -1 once they have
	crushed   []int16
	tailCap   float64 // see waveTicks
	skipCap   float64 // 0 until the player skips
	Clock     float64 // ticks since the board was created

	burstStart, burstLen []float64
	burstOwner           []int8
	falls                []fallingOrb
	landfillDurability   []uint8
	landfillTarget       []uint8    // orbs each Landfill takes to fill, worked out from the map design as core does
	shownBoxes           []uint8    // Replay.Boxes for the map layer: a pit a Box fills shows as Floor once it has sunk in
	slides               []boxSlide // what the wave in flight moves
	routes               []int16    // their Portal and Spring hops
	moving               []uint8    // per cell, what moves there in the wave in flight; nil without Boxes or Springs

	// Layout in device pixels.
	area   ui.Rect
	Turned bool    // drawn a quarter turn clockwise: a wide board in a tall area
	base   float64 // base cell size
	k      float64 // device px per base unit
	ox, oy float64 // board origin

	TileImg     *ebiten.Image
	OrbImg      *ebiten.Image // atlas of square cells: rim disc, fill disc, then the twelve marks
	OrbSize     float64       // side of each atlas cell
	OrbDrawSize float64       // side on screen; large atlases may render at a smaller resolution
	Palette     *ui.Palette   // the mode being drawn
	marks       bool          // draw shape marks on orbs
	markInk     [core.MaxPlayers]color.RGBA
	burstImg    *ebiten.Image
	Verts       []ebiten.Vertex
	idx         []uint32
	tri         ebiten.DrawTrianglesOptions
	badge       *text.GoTextFace

	mapImg      *ebiten.Image // physical map pieces, rebuilt only when their appearance changes
	mapTheme    *ui.Palette
	mapRevision uint64
	mapMarks    bool
}

// New creates the board renderer and its wave playback state.
func New(g *core.Game) *Board {
	n := g.Geo.Cells()
	b := &Board{
		State: NewView(g), Orbs: newOrbField(g.Geo),
		burstStart: make([]float64, n), burstLen: make([]float64, n), burstOwner: make([]int8, n),
		landfillDurability: append([]uint8(nil), g.Durability...),
		shownBoxes:         append([]uint8(nil), g.Boxes...),
	}
	if g.Map != nil {
		b.landfillTarget = make([]uint8, n)
		for i, c := range g.Map.Cells {
			if c.Terrain == core.Landfill {
				b.landfillTarget[i] = uint8(g.Map.FillTarget(i))
			}
			if (c.Feature == core.Box || c.Feature == core.Spring) && b.moving == nil {
				b.moving, b.routes = make([]uint8, n), make([]int16, 0, 16)
			}
		}
	}
	b.tri.Filter = ebiten.FilterLinear
	s := 2*burstSrcR + 4
	b.burstImg = ebiten.NewImage(s, s)
	ui.Ring(b.burstImg, float64(s)/2, float64(s)/2, burstSrcR, burstSrcR*0.16, ui.PureWhite)
	return b
}

// layout fits the board into area (device px) at UI scale u, centred along the slack axis.
func (b *Board) Layout(area ui.Rect, u float64) {
	if area == b.area && b.TileImg != nil {
		return
	}
	b.area = area
	b.Turned = b.turns(area)
	cols, rows := b.ScreenSize()
	previousBase, previousScale := b.base, b.k
	b.base = cellSize(cols, rows, area.W/u, area.H/u)
	// The outer tiles' edges, not the cells around them, meet the area's edges, so the board shares the screen margin.
	bw, bh := (float64(cols)-2*TileGutter)*b.base, (float64(rows)-2*TileGutter)*b.base
	b.k = min(area.W/bw, area.H/bh)
	g := TileGutter * b.base * b.k
	b.ox, b.oy = math.Round(area.X+(area.W-bw*b.k)/2-g), math.Round(area.Y+(area.H-bh*b.k)/2-g)
	if b.TileImg != nil && b.base == previousBase && b.k == previousScale {
		return // scrolling moves the board without rebuilding its tile and orb images
	}

	if b.TileImg != nil {
		b.TileImg.Deallocate()
		b.OrbImg.Deallocate()
	}
	b.releaseMapLayers()
	cell := b.base * b.k
	ts := int(math.Ceil(cell*(1-2*TileGutter))) + 2
	b.TileImg = ebiten.NewImage(ts, ts)
	ui.FillRound(b.TileImg, ui.Rect{X: 1, Y: 1, W: float64(ts - 2), H: float64(ts - 2)}, cell*TileRadius, ui.PureWhite)

	// Orbs are flat discs with a rim in the mat's colour, so overlapping orbs read as separate pieces, and
	// optionally the owner's shape mark. Rim, fill and the twelve marks come from one atlas, so every orb batches
	// together and keeps its painter's order.
	r := orbRadius(b.base) * b.k
	drawSize := math.Ceil(2*r) + 4
	s := OrbAtlasSide(drawSize, ebiten.MaxImageSize())
	fr := (r - max(1.5, r*0.16)) * s / drawSize
	r *= s / drawSize
	b.OrbSize, b.OrbDrawSize = s, drawSize
	b.OrbImg = ebiten.NewImage(int((2+core.MaxPlayers)*s), int(s))
	ui.FillCircle(b.OrbImg, s/2, s/2, r, ui.PureWhite)
	ui.FillCircle(b.OrbImg, s+s/2, s/2, fr, ui.PureWhite)
	for p := range core.MaxPlayers {
		ui.MarkShape(b.OrbImg, p, float64(2+p)*s+s/2, s/2, fr, ui.PureWhite)
	}
	b.badge = ui.NewFace(ui.SrcUI, max(9, 0.17*cell), 700)
}

// orbAtlasSide bounds the source texture while keeping large full-width boards at their intended scale.
func OrbAtlasSide(drawSize float64, limit int) float64 {
	return math.Floor(drawSize * ui.ScreenFit(drawSize*(2+core.MaxPlayers), drawSize, limit))
}

// turns reports whether the board turns a quarter in area: a wide board in a tall area (an upright phone browser)
// turns so its long side runs down the screen and its tiles stay big enough to tap. Only the drawing turns; the game
// is the same. Where the app can rotate the screen itself (Android), the game goes landscape instead
// (wantsLandscape).
func (b *Board) turns(area ui.Rect) bool {
	return area.H > area.W && b.State.Geo.Cols > b.State.Geo.Rows && platform.Rotate == nil
}

// size is the board's width and height, outer tile edge to outer tile edge, as layout would fit it into area.
func (b *Board) Size(area ui.Rect) (w, h float64) {
	cols, rows := float64(b.State.Geo.Cols), float64(b.State.Geo.Rows)
	if b.turns(area) {
		cols, rows = rows, cols
	}
	cols, rows = cols-2*TileGutter, rows-2*TileGutter
	k := min(area.W/cols, area.H/rows)
	return cols * k, rows * k
}

// screenSize is the board's columns and rows as drawn.
func (b *Board) ScreenSize() (cols, rows int) {
	if b.Turned {
		return b.State.Geo.Rows, b.State.Geo.Cols
	}
	return b.State.Geo.Cols, b.State.Geo.Rows
}

// cellAt returns the cell under device point (x, y), or -1.
func (b *Board) CellAt(x, y float64) int {
	if b.k == 0 {
		return -1
	}
	sc := int(math.Floor((x - b.ox) / b.k / b.base))
	sr := int(math.Floor((y - b.oy) / b.k / b.base))
	cols, rows := b.ScreenSize()
	if sc < 0 || sr < 0 || sc >= cols || sr >= rows {
		return -1
	}
	if b.Turned {
		return b.State.Geo.Index(sr, b.State.Geo.Rows-1-sc)
	}
	return b.State.Geo.Index(sc, sr)
}

// centre of cell i in device px.
func (b *Board) Centre(i int) (float64, float64) {
	c, r := b.State.Geo.Coords(i)
	if b.Turned { // a quarter turn clockwise: the first row becomes the last column
		c, r = b.State.Geo.Rows-1-r, c
	}
	return b.ox + (float64(c)+0.5)*b.base*b.k, b.oy + (float64(r)+0.5)*b.base*b.k
}

// tileRect is cell i's tile in device px.
func (b *Board) tileRect(i int) ui.Rect {
	x, y := b.Centre(i)
	h := b.base * b.k * (0.5 - TileGutter)
	return ui.Rect{X: x - h, Y: y - h, W: 2 * h, H: 2 * h}
}

// bounds is the whole board, outer tile edge to outer tile edge, in device px.
func (b *Board) Bounds() ui.Rect {
	cell, g := b.base*b.k, TileGutter*b.base*b.k
	cols, rows := b.ScreenSize()
	return ui.Rect{X: b.ox + g, Y: b.oy + g, W: float64(cols)*cell - 2*g, H: float64(rows)*cell - 2*g}
}

// start shows a move: mover's orb lands in cell, then waves (from core.Game.Apply) play out, with the mover's
// Furnaces and Crushers running after produceAt of them (core.Game.ProducedAt). snd may be nil.
func (b *Board) Start(cell int, mover int8, waves []core.Wave, produceAt int, snd *sound.Sound) {
	effects := b.State.terrainSounds(b.State.Place(cell, mover))
	b.dropTaken() // a Sinkhole this placement filled
	b.Waves, b.Wave, b.t, b.Playing = waves, -1, 0, true
	b.settling = false
	b.mover, b.produceAt = mover, produceAt
	b.tailCap, b.skipCap = tailCap(len(waves)), 0
	_, produced := b.next() // a Furnace's orb made now shares the placement note
	snd.Place(int(mover), effects|produced|b.linkSound())
}

// dropTaken drops the orbs on the cells the last terrain step took away: a broken floor or a collapsed Sinkhole.
// shortcut: a whole stack falls as one orb from its centre; drop each from its orbit spot if stacks should read as
// several.
func (b *Board) dropTaken() {
	for _, o := range b.State.dropped {
		o.start = b.Clock
		b.falls = append(b.falls, o)
	}
}

// sync makes the view match g without animating.
func (b *Board) Sync(g *core.Game) {
	b.State.sync(g)
	b.falls = b.falls[:0]
	b.settling = false
	b.landfillDurability = append(b.landfillDurability[:0], g.Durability...)
	b.shownBoxes = append(b.shownBoxes[:0], g.Boxes...)
	b.slides = b.slides[:0]
	clear(b.moving)
}

// next runs the mover's Furnaces and Crushers at once when their turn comes, then launches the following wave or
// ends playback. It reports whether a Furnace made an orb, and the cues of crushed ones.
func (b *Board) next() (made bool, effects sound.Effects) {
	if b.produceAt == b.Wave+1 {
		b.produceAt = -1
		made, effects = b.produce()
	}
	b.Wave++
	if b.Wave >= len(b.Waves) {
		b.Playing, b.t = false, 0
		b.State.settle()
		b.planSlides()
		return
	}
	w := b.Waves[b.Wave]
	b.State.Launch(w)
	b.dur = waveTicks(b.Wave, b.tailCap, b.skipCap)
	l := max(minBurstTicks, b.dur*burstTicks/transferTicks)
	for k, e := range w {
		b.burstStart[e], b.burstLen[e], b.burstOwner[e] = b.Clock, l, b.State.from[k]
	}
	b.planSlides()
	return
}

// fastForward plays the rest of the chain quickly.
func (b *Board) FastForward() {
	if b.settling {
		for i := range b.falls {
			if b.falls[i].landfill || b.falls[i].box {
				b.falls[i].start = b.Clock - b.falls[i].ticks()
			}
		}
	}
	if b.Playing && b.skipCap == 0 {
		b.skipCap = skipBudget / float64(len(b.Waves)-b.Wave)
		b.dur = min(b.dur, b.skipCap)
	}
}

// tick advances the animations by one 60 Hz tick. Short waves can finish several per tick.
func (b *Board) Tick(snd *sound.Sound) {
	b.Clock++
	filled := b.settleFalls()
	if b.Playing && b.settling && !b.landfillFalling() {
		b.settling = false
		made, effects := b.next()
		b.stepSound(snd, made, effects|filled|b.linkSound())
		return
	}
	if !b.Playing || b.settling {
		snd.Effect(filled)
		return
	}
	b.t++
	advanced, made := false, false
	effects := filled
	for b.Playing && b.t >= b.dur {
		b.t -= b.dur
		fallStart := len(b.falls)
		effects |= b.dropIntoHoles()
		events := b.State.Land(b.Waves[b.Wave])
		effects |= b.State.terrainSounds(events)
		b.boxesLanded(events)
		b.dropTaken()
		for i := fallStart; i < len(b.falls); i++ {
			if o := &b.falls[i]; o.landfill {
				o.remaining, o.slot = b.State.Replay.Durability[o.hole], b.landfillSlot(o.hole, i)
				if b.skipCap > 0 {
					o.start = b.Clock - o.ticks() // lands at once; the next tick settles it
				} else if o.remaining == 0 {
					b.settling = true
				}
			}
		}
		if b.settling {
			b.t = 0
			snd.Effect(effects)
			return
		}
		m, produced := b.next()
		made, effects = made || m, effects|produced|b.linkSound()
		advanced = true
	}
	if advanced {
		b.stepSound(snd, made, effects)
	} else {
		snd.Effect(effects)
	}
}

// settleFalls drops finished falls. An orb reaching a Landfill bottom updates its opening only then, and the cue for
// a completed Floor plays as the last fall lands.
func (b *Board) settleFalls() (effects sound.Effects) {
	live := b.falls[:0]
	for _, o := range b.falls {
		if b.Clock-o.start < o.ticks() {
			live = append(live, o)
		} else if o.box {
			effects |= b.boxSunk(o.hole)
		} else if o.landfill && o.remaining < b.landfillDurability[o.hole] {
			b.landfillDurability[o.hole] = o.remaining
			b.State.revision++
			if o.remaining == 0 {
				effects |= sound.LandfillFull
			}
		}
	}
	b.falls = live
	return
}

// landfillSlot counts the orbs already resting on, or falling to, Landfill hole's bottom ahead of fall before.
func (b *Board) landfillSlot(hole int16, before int) uint8 {
	n := int(b.landfillTarget[hole]) - int(b.landfillDurability[hole])
	for _, o := range b.falls[:before] {
		if o.landfill && o.hole == hole {
			n++
		}
	}
	return uint8(min(n, 3))
}

// landfillFalling reports whether an orb or Box is still falling into a pit it fills, which holds playback.
func (b *Board) landfillFalling() bool {
	for _, o := range b.falls {
		if o.landfill || o.box {
			return true
		}
	}
	return false
}

func (b *Board) stepSound(snd *sound.Sound, made bool, effects sound.Effects) {
	switch {
	case b.Playing:
		snd.Wave(b.Wave, len(b.Waves[b.Wave]), effects)
	case made:
		snd.Place(int(b.mover), effects)
	default:
		snd.Effect(effects)
	}
}

// produce runs the mover's Furnaces and Crushers; each crushed orb shrinks and fades like one lost in a Hole.
func (b *Board) produce() (made bool, effects sound.Effects) {
	made, crushed := b.State.Produce(b.mover, b.crushed[:0])
	b.crushed = crushed
	for _, cell := range crushed {
		b.falls = append(b.falls, fallingOrb{hole: cell, owner: b.mover, start: b.Clock})
		effects |= sound.Fall
	}
	return made, effects
}

// draw renders the board on the mat at animation time tms (ms). The tiles of player turn light up in their ink;
// every other tile stays printed blank (turn -1 lights none).
func (b *Board) Draw(dst *ebiten.Image, sc *ui.Palette, tms float64, turn int, marks bool) {
	b.Palette, b.marks = sc, marks
	if marks {
		for p := range b.markInk {
			b.markInk[p] = ui.InkOn(sc.Players[p])
		}
	}
	ts := float64(b.TileImg.Bounds().Dx())
	for i, o := range b.State.Owner {
		c := sc.Tile
		if b.State.Replay != nil {
			terrain := b.State.Replay.TerrainAt(i)
			if b.State.Replay.Map.Cells[i].Terrain == core.Landfill && b.landfillDurability[i] > 0 {
				terrain = core.Landfill
			}
			if terrain == core.Floor && i < len(b.shownBoxes) && b.State.Replay.Boxes[i]&^b.shownBoxes[i]&core.CrateFloor != 0 {
				terrain = core.Hole // a Box is still sinking into it; a Landfill stays one above
			}
			switch terrain {
			case core.Void, core.Hole, core.Landfill: // the map layer draws the pit
				continue
			case core.Wall, core.BreakableWall:
				c = sc.TileHi
			case core.Floor:
				design := b.State.Replay.Map.Cells[i].Terrain
				if boxes := b.State.Replay.Boxes; len(boxes) != 0 && boxes[i]&core.CrateFloor != 0 {
					design = core.Floor // a filled pit, plain Floor even where a Sinkhole stood
				}
				c = mapArtFloorColor(sc, design)
			}
		}
		if turn >= 0 && int(o) == turn {
			c = ui.Mix(c, sc.Players[turn], tileTint)
		}
		x, y := b.Centre(i)
		b.quad(x, y, ts/2, 0, ts, c, 1)
	}
	b.flush(dst, b.TileImg)
	if b.State.Replay != nil {
		b.drawMapLayer(dst, sc)
	}
	b.drawBoxes(dst, sc)
	f := b.flight()

	// Orbs, rim then fill (then mark) for each, in one batch.
	for i, n := range b.State.Count {
		if n == 0 {
			continue
		}
		cx, cy := b.Centre(i)
		if b.moving != nil && b.moving[i]&slidStack != 0 {
			cx, cy = b.slideAt(i, true, f) // shoved along with its Box
		}
		owner := int(b.State.Owner[i])
		if !b.State.Geo.Playable(i) {
			owner = int(core.NoOwner) // presentation only; the saved owner regains control on reopening
		}
		for _, p := range b.Orbs.update(i, int(n), int(b.State.Geo.Cap[i]), tms, b.base) {
			b.Orb(cx+p[0]*b.k, cy+p[1]*b.k, owner)
		}
	}
	for i, d := range b.landfillDurability {
		if d > 0 && b.State.Replay != nil && b.State.Replay.Map.Cells[i].Terrain == core.Landfill {
			// Resting orbs glide into the bigger clump as more fall in.
			n := int(b.landfillTarget[i] - d)
			falling, depth := b.landfillFalls(i)
			for slot := range n {
				x0, y0 := b.landfillSpot(i, slot, n)
				x1, y1 := b.landfillSpot(i, slot, n+falling)
				b.landfillOrb(i, core.NoOwner, 1, x0+(x1-x0)*depth, y0+(y1-y0)*depth)
			}
		}
	}
	b.flush(dst, b.OrbImg)
	b.drawBadges(dst, sc)

	// Explosions: flat rings in the exploding cell's ink.
	for i, start := range b.burstStart {
		if b.burstLen[i] == 0 {
			continue
		}
		f := (b.Clock - start) / b.burstLen[i]
		if f >= 1 {
			b.burstLen[i] = 0
			continue
		}
		cx, cy := b.Centre(i)
		e := 1 - (1-f)*(1-f) // ease out
		r := max(1, (0.25+0.7*e)*b.base*b.k)
		b.quad(cx, cy, r*float64(burstSrcR+2)/burstSrcR, 0, 2*burstSrcR+4, sc.Players[b.burstOwner[i]], 0.9*(1-f))
	}
	b.flush(dst, b.burstImg)

	// Orbs lost in Holes, under the orbs in flight to the targets of the wave being shown.
	for _, o := range b.falls {
		if !o.box {
			b.queueFall(o)
		}
	}
	if b.Playing && !b.settling {
		for k, e := range b.Waves[b.Wave] {
			for connection, target := range b.State.Geo.Targets(int(e)) {
				x, y := b.flightPosition(int(e), connection, b.bounce(int(target), f))
				b.Orb(x, y, int(b.State.from[k]))
			}
			if outlets := b.State.Geo.Outlets; outlets != nil && outlets[e] != 0 {
				// A Reservoir's extra orb trails the one flying the way its arrow points.
				slot, _ := b.State.Geo.Outlet(int(e), b.State.Replay.Pressed[e])
				x, y := b.flightPosition(int(e), slot, b.bounce(int(b.State.Geo.Targets(int(e))[slot]), f*.8))
				b.Orb(x, y, int(b.State.from[k]))
			}
		}
	}
	b.flush(dst, b.OrbImg)
	if b.State.Replay != nil {
		for i, c := range b.State.Replay.Map.Cells {
			if c.Circuit == core.Door && b.State.Count[i] > 0 && !b.State.Geo.Playable(i) {
				x, y := b.Centre(i)
				cell := b.base * b.k
				drawMapSprite(dst, sc, ui.Rect{X: x - cell/2, Y: y - cell/2, W: cell, H: cell}, mapSpriteKey{group: c.Group, lock: true})
			}
		}
	}
}

func (b *Board) flightPosition(cell, connection int, fraction float64) (float64, float64) {
	return b.along(cell, int(b.State.Geo.Targets(cell)[connection]), b.State.Geo.PortalPath(cell, connection), fraction)
}

// along returns the point fraction of the way from cell start to cell end through hops, entry/exit pairs: a Portal
// jumps from its entry to its exit, a Spring (its own pair) turns.
func (b *Board) along(start, end int, hops []int16, fraction float64) (float64, float64) {
	segments := len(hops)/2 + 1
	progress := min(1, fraction) * float64(segments)
	segment := min(int(progress), segments-1)
	if segment > 0 {
		start = int(hops[2*segment-1])
	}
	if segment < segments-1 {
		end = int(hops[2*segment])
	}
	sx, sy := b.Centre(start)
	ex, ey := b.Centre(end)
	f := progress - float64(segment)
	return sx + (ex-sx)*f, sy + (ey-sy)*f
}

// dropIntoHoles starts a fall for every orb the landing wave sent into a Hole.
func (b *Board) dropIntoHoles() sound.Effects {
	geo := b.State.Geo
	if geo.Holes == nil {
		return 0
	}
	var effects sound.Effects
	for k, e := range b.Waves[b.Wave] {
		for _, hole := range geo.Targets(int(e))[geo.Degree[e]:] {
			landfill := b.State.Replay != nil && b.State.Replay.TerrainAt(int(hole)) == core.Landfill
			b.falls = append(b.falls, fallingOrb{hole: hole, owner: b.State.from[k], start: b.Clock, landfill: landfill})
			effects = sound.Fall
		}
	}
	return effects
}

// landfillLayouts place 1 to 4 orbs resting on a Landfill bottom, in resting radii from its centre: a centred clump
// 1.4 radii apart, overlapping as orbs on Floor do. Quarter turns keep each inside the square bottom.
var landfillLayouts = [4][][2]float64{
	{{0, 0}},
	{{-.495, -.495}, {.495, .495}},
	{{.5715, .5715}, {-.7808, .2092}, {.2092, -.7808}},
	{{-.7, -.7}, {.7, -.7}, {.7, .7}, {-.7, .7}},
}

// landfillRest is the radius (device px) of an orb resting on a Landfill bottom: landfillOrbScale of an orb's, or less
// where a layout of landfillLayouts wouldn't fit inside the bottom, the darkest square.
func (b *Board) landfillRest() float64 {
	bottom := landfillSteps[len(landfillSteps)-1].scale
	cell := b.base * b.k
	half, corner := cell*(.5-TileGutter)*bottom, cell*TileRadius*bottom
	fits := func(r float64) bool {
		for _, layout := range landfillLayouts {
			for _, p := range layout {
				x, y := math.Abs(p[0])*r, math.Abs(p[1])*r
				cx, cy := x-(half-corner), y-(half-corner)
				if x+r > half || y+r > half || cx > 0 && cy > 0 && math.Hypot(cx, cy)+r > corner {
					return false
				}
			}
		}
		return true
	}
	lo, hi := 0.0, orbRadius(b.base)*b.k*landfillOrbScale
	if fits(hi) {
		return hi
	}
	for range 24 {
		if mid := (lo + hi) / 2; fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// landfillSpot is where (device px from Landfill i's centre) the orb in slot rests while count orbs rest there. The
// cell's seed turns the layout a quarter turn or more, so every frame and replay agrees.
func (b *Board) landfillSpot(i int, slot, count int) (float64, float64) {
	count = min(max(count, slot+1), len(landfillLayouts))
	p := landfillLayouts[count-1][min(slot, count-1)]
	x, y := p[0], p[1]
	for range b.Orbs.seed[i*orbSlots] & 3 {
		x, y = -y, x
	}
	r := b.landfillRest()
	return x * r, y * r
}

// landfillFalls returns how many orbs are falling into Landfill i and how far down they are (0 to 1; 1 with none).
func (b *Board) landfillFalls(i int) (n int, depth float64) {
	depth = 1
	for _, o := range b.falls {
		if o.landfill && int(o.hole) == i {
			f := min(1, (b.Clock-o.start)/fallTicks)
			n, depth = n+1, min(depth, f*f)
		}
	}
	return n, depth
}

// landfillOrb queues an orb on its way down to Landfill i's bottom (depth 0 to 1) or resting there (depth 1), at
// (dx, dy) from its centre: it shrinks to the bottom's size and turns neutral.
func (b *Board) landfillOrb(i int, owner int8, depth, dx, dy float64) {
	sc, s := b.Palette, b.OrbSize
	x, y := b.Centre(i)
	x, y = x+dx, y+dy
	h := b.OrbDrawSize / 2 * (1 - (1-b.landfillRest()/(orbRadius(b.base)*b.k))*depth)

	fill := sc.Tile
	if owner >= 0 {
		fill = sc.Players[owner]
	}
	b.quad(x, y, h, 0, s, sc.Ground, 1)
	b.quad(x, y, h, s, s, ui.Mix(fill, landfillOrbInk(sc), depth), 1)
	if b.marks && owner >= 0 && depth < 1 {
		b.quad(x, y, h, float64(2+int(owner))*s, s, b.markInk[owner], 1-depth)
	}
}

// queueFall queues an orb lost in a Hole: from the centre it drops away, shrinking, darkening into the pit and
// fading together, faster as it falls.
func (b *Board) queueFall(o fallingOrb) {
	f := min(1, (b.Clock-o.start)/fallTicks)
	depth := f * f // accelerates like a fall
	alpha := 1 - depth
	x, y := b.Centre(int(o.hole))
	s, h, sc := b.OrbSize, b.OrbDrawSize/2*(1-.7*depth), b.Palette
	fill := sc.Tile
	if o.owner >= 0 {
		fill = sc.Players[o.owner]
	}
	if o.landfill {
		// It eases from the centre to its spot in the clump all the falling and resting orbs make.
		n, _ := b.landfillFalls(int(o.hole))
		x, y := b.landfillSpot(int(o.hole), int(o.slot), int(b.landfillTarget[o.hole]-b.landfillDurability[o.hole])+n)
		b.landfillOrb(int(o.hole), o.owner, depth, x*depth, y*depth)
		return
	}
	b.quad(x, y, h, 0, s, ui.Mix(sc.Ground, sc.Pit, .9*depth), alpha)
	b.quad(x, y, h, s, s, ui.Mix(fill, sc.Pit, .6*depth), alpha)
	if b.marks && o.owner >= 0 {
		b.quad(x, y, h, float64(2+int(o.owner))*s, s, ui.Mix(b.markInk[o.owner], sc.Pit, .6*depth), alpha)
	}
}

func (b *Board) releaseMapLayers() {
	if b.mapImg != nil {
		b.mapImg.Deallocate()
	}
	b.mapImg = nil
}

// Only visible terrain changes rebuild the physical layer; locks use shared sprites above orbs.
func (b *Board) drawMapLayer(dst *ebiten.Image, sc *ui.Palette) {
	if b.mapImg == nil || b.mapTheme != sc || b.mapRevision != b.State.revision || b.mapMarks != b.marks {
		cols, rows := b.ScreenSize()
		cell := b.base * b.k
		w, h := int(math.Ceil(float64(cols)*cell)), int(math.Ceil(float64(rows)*cell))
		if b.mapImg == nil || b.mapImg.Bounds().Dx() != w || b.mapImg.Bounds().Dy() != h {
			b.releaseMapLayers()
			b.mapImg = ebiten.NewImage(max(1, w), max(1, h))
		} else {
			b.mapImg.Clear()
		}
		b.renderMapLayer(b.mapImg, sc)
		b.mapTheme, b.mapRevision = sc, b.State.revision
		b.mapMarks = b.marks
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(b.ox, b.oy)
	dst.DrawImage(b.mapImg, &op)
}

func (b *Board) renderMapLayer(dst *ebiten.Image, sc *ui.Palette) {
	g := b.State.Replay
	DrawMapArt(dst, sc, MapState{Map: g.Map, Durability: g.Durability, EdgeDurability: g.EdgeDurability, Pressed: g.Pressed, CircuitOn: g.CircuitOn, Owner: b.State.Owner, Count: b.State.Count, PortalAxes: g.Geo.PortalAxes, Outlets: g.Geo.Outlets, LandfillDurability: b.landfillDurability, Boxes: b.shownBoxes, Moving: true},
		MapTransform{Cell: b.base * b.k, Turned: b.Turned}, false, b.marks)
}

// Orb queues a player's orb, or a floor-colored, unmarked trapped orb for NoOwner.
func (b *Board) Orb(x, y float64, p int) {
	s := b.OrbSize
	h := b.OrbDrawSize / 2
	fill := b.Palette.Tile
	if p >= 0 {
		fill = b.Palette.Players[p]
	}
	b.quad(x, y, h, 0, s, b.Palette.Ground, 1)
	b.quad(x, y, h, s, s, fill, 1)
	if b.marks && p >= 0 {
		b.quad(x, y, h, float64(2+p)*s, s, b.markInk[p], 1)
	}
}

// quad queues a square of the source image starting at x-offset sx with side src, centred at (x, y) with
// half-size h, tinted c at alpha a.
func (b *Board) quad(x, y, h, sx, src float64, c color.RGBA, a float64) {
	n := uint32(len(b.Verts))
	r, g, bl := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
	x0, y0, x1, y1 := float32(x-h), float32(y-h), float32(x+h), float32(y+h)
	s0, s1, t := float32(sx), float32(sx+src), float32(src)
	b.Verts = append(b.Verts,
		ebiten.Vertex{DstX: x0, DstY: y0, SrcX: s0, SrcY: 0, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x1, DstY: y0, SrcX: s1, SrcY: 0, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x0, DstY: y1, SrcX: s0, SrcY: t, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
		ebiten.Vertex{DstX: x1, DstY: y1, SrcX: s1, SrcY: t, ColorR: r, ColorG: g, ColorB: bl, ColorA: float32(a)},
	)
	b.idx = append(b.idx, n, n+1, n+2, n+1, n+3, n+2)
}

// flush draws the queued quads from src. Tiles draw at their cached size and large orbs may upscale; only burst
// rings shrink, so only they pay for mipmaps.
func (b *Board) flush(dst, src *ebiten.Image) {
	if len(b.idx) > 0 {
		b.tri.DisableMipmaps = src != b.burstImg
		dst.DrawTriangles32(b.Verts, b.idx, src, &b.tri)
	}
	b.Verts, b.idx = b.Verts[:0], b.idx[:0]
}

// drawBadges labels crowded cells, including custom-map cells after a long chain.
func (b *Board) drawBadges(dst *ebiten.Image, sc *ui.Palette) {
	for i, n := range b.State.Count {
		if n <= 4 {
			continue
		}
		t := b.tileRect(i)
		h := max(14, t.H*0.3)
		r := ui.Rect{X: t.Right() - h*1.3 - 2, Y: t.Bottom() - h - 2, W: h * 1.3, H: h}
		ui.FillPill(dst, r, sc.Ink)
		ui.DrawText(dst, ui.Numbers[n], b.badge, r.Cx(), r.Cy(), text.AlignCenter, sc.Ground)
	}
}

// drawHover outlines cell i's tile in c: where the player to move would place.
func (b *Board) DrawHover(dst *ebiten.Image, i int, c color.RGBA, u float64) {
	t := b.tileRect(i)
	ui.StrokeRound(dst, t, t.W*TileRadius/(1-2*TileGutter), max(2, 3*u), c)
}
