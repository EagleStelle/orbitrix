package boardview

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/client/internal/sound"
	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// boxPending is an orb in flight into a Box, held as core holds it: it lands where the Box stood once the Box moves,
// or goes back to the cell that sent it.
type boxPending struct {
	box, from int16
	owner     int8
}

func (v *view) boxed(i int) bool {
	return v.Replay != nil && len(v.Replay.Boxes) != 0 && v.Replay.Boxes[i]&core.BoxHere != 0
}

// moveBoxes applies the orb side of a wave's Box events, before the terrain replay moves the Boxes: shoved stacks
// move on or drop into pits, held orbs land where their Box stood, and those of Boxes that stayed go back.
func (v *view) moveBoxes(events []core.TerrainEvent) {
	for _, e := range events {
		if !e.Box {
			continue
		}
		if e.Shove >= 0 {
			c, o := v.Count[e.To], v.Owner[e.To]
			v.inkChanged(e.To, core.NoOwner)
			v.Count[e.To], v.Owner[e.To] = 0, core.NoOwner
			if t := v.Replay.TerrainAt(e.Shove); t == core.Floor {
				for c > 0 && v.shielded(e.Shove, o, -1) {
					c--
				}
				if c > 0 {
					v.inkChanged(e.Shove, o)
					v.Owner[e.Shove] = o
					v.Count[e.Shove] += c
				}
			} else {
				v.dropped = append(v.dropped, fallingOrb{hole: int16(e.Shove), owner: o, landfill: t == core.Landfill})
			}
		}
		for k := range v.pending {
			if p := &v.pending[k]; p.box == int16(e.Cell) {
				v.land(e.Cell, p.owner)
				p.box = -1
			}
		}
	}
	for _, p := range v.pending {
		if p.box >= 0 {
			v.land(int(p.from), p.owner)
		}
	}
	v.pending = v.pending[:0]
}

func (v *view) land(i int, owner int8) {
	if v.shielded(i, owner, -1) {
		return
	}
	v.inkChanged(i, owner)
	v.Owner[i] = owner
	v.Count[i]++
}

// boxSlide is a Box, or the stack it shoves, moving during the wave in flight: from from to to through the Portal and
// Spring hops at routes[lo:hi].
type boxSlide struct {
	from, to int16
	lo, hi   int
	stack    bool
}

// What moves on a cell during the wave in flight, in Board.moving.
const (
	slidBox uint8 = 1 << iota
	slidStack
	squashed // a Spring launching something
)

// planSlides finds what the wave just launched moves: its Box events, and the Springs its orbs and Boxes go through.
func (b *Board) planSlides() {
	if b.moving == nil {
		return
	}
	clear(b.moving)
	b.slides, b.routes = b.slides[:0], b.routes[:0]
	g := b.State.Replay
	if !b.Playing {
		return
	}
	for _, e := range b.State.Events {
		if e.Wave != b.Wave {
			break
		}
		if e.Box {
			b.addSlide(g, e.Cell, e.To, e.Dir, false)
			if e.Shove >= 0 {
				b.addSlide(g, e.To, e.Shove, e.ShoveDir, true)
			}
		}
	}
	b.squash(b.routes)
	for _, e := range b.Waves[b.Wave] {
		for connection := range b.State.Geo.Targets(int(e)) {
			b.squash(b.State.Geo.PortalPath(int(e), connection))
		}
	}
}

func (b *Board) addSlide(g *core.Game, from, to int, dir int8, stack bool) {
	lo := len(b.routes)
	if dir >= 0 {
		b.routes = g.BoxRoute(from, int(dir), b.routes)
	}
	b.slides = append(b.slides, boxSlide{from: int16(from), to: int16(to), lo: lo, hi: len(b.routes), stack: stack})
	if stack {
		b.moving[from] |= slidStack
	} else {
		b.moving[from] |= slidBox
	}
}

func (b *Board) squash(hops []int16) {
	for _, cell := range hops {
		if b.State.Replay.Map.Cells[cell].Feature == core.Spring {
			b.moving[cell] |= squashed
		}
	}
}

// flight is how far the wave in flight has come, 0 to 1; 0 when none is.
func (b *Board) flight() float64 {
	if !b.Playing || b.settling {
		return 0
	}
	return min(1, b.t/b.dur)
}

// slideAt returns where what slides off cell i (a Box, or a stack) is at fraction f of the wave.
func (b *Board) slideAt(i int, stack bool, f float64) (float64, float64) {
	for _, s := range b.slides {
		if int(s.from) == i && s.stack == stack {
			return b.along(i, int(s.to), b.routes[s.lo:s.hi], f)
		}
	}
	return b.Centre(i)
}

// bounce is the flight fraction f of player p's orb heading into cell i: one hitting a Box that stays goes half way
// and back, and one a Vault's shield takes stops at its edge.
func (b *Board) bounce(i int, p int8, f float64) float64 {
	if b.moving != nil && b.State.boxed(i) && b.moving[i]&slidBox == 0 {
		return min(f, 1-f)
	}
	if g := b.State.Replay; g != nil {
		// shortcut: every orb a shield would take stops, even past its last point in one wave; those land at the end.
		if hit, _ := core.Shielded(g.Pressed[i], p); hit {
			return min(f, shieldStop)
		}
	}
	return f
}

// drawBoxes draws the Springs, squashed while they launch something, and the Boxes where they stand or slide, sinking
// into the pits they fill.
func (b *Board) drawBoxes(dst *ebiten.Image, sc *ui.Palette) {
	g := b.State.Replay
	if b.moving == nil || g == nil {
		return
	}
	f, cell := b.flight(), b.base*b.k
	at := func(x, y float64) ui.Rect { return ui.Rect{X: x - cell/2, Y: y - cell/2, W: cell, H: cell} }
	tr := MapTransform{Turned: b.Turned}
	for i, c := range g.Map.Cells {
		if c.Feature == core.Spring {
			key := mapSpriteKey{feature: core.Spring, arrow: springSide(g.Map, tr, i)}
			if b.moving[i]&squashed != 0 {
				key.squash = uint8(math.Round(springSquash * math.Sin(math.Pi*f)))
			}
			drawMapSprite(dst, sc, at(b.Centre(i)), key)
		}
	}
	for i, v := range g.Boxes {
		if v&core.BoxHere != 0 && g.TerrainAt(i) != core.Wall { // a closed Door hides one
			drawMapSprite(dst, sc, at(b.slideAt(i, false, f)), mapSpriteKey{feature: core.Box})
		}
	}
	for _, o := range b.falls {
		if !o.box {
			continue
		}
		// Like an orb filling a Landfill it accelerates down, shrinking to the Floor crate it becomes and turning into it:
		// that crate underneath, the Box fading off it. It lands as the Floor crate, so the pit turns to Floor around it.
		f := min(1, (b.Clock-o.start)/fallTicks)
		depth, r := f*f, at(b.Centre(int(o.hole)))
		scale := 1 - (1-crateFloorScale)*depth
		drawMapSpriteFaded(dst, sc, r, mapSpriteKey{crate: true}, scale/crateFloorScale, 1)
		drawMapSpriteFaded(dst, sc, r, mapSpriteKey{feature: core.Box}, scale, 1-depth)
	}
}

// boxesLanded starts the fall of every Box the landed wave's events dropped into a pit. Like an orb completing a Landfill,
// it holds playback until it lands, and the pit shows as Floor only then (boxSunk).
func (b *Board) boxesLanded(events []core.TerrainEvent) {
	g := b.State.Replay
	for _, e := range events {
		if e.Box && g.Boxes[e.To] == core.CrateFloor {
			o := fallingOrb{hole: int16(e.To), owner: core.NoOwner, start: b.Clock, box: true}
			if b.skipCap > 0 {
				o.start = b.Clock - o.ticks() // lands at once; the next tick settles it
			} else {
				b.settling = true
			}
			b.falls = append(b.falls, o)
		}
	}
}

// boxSunk shows the pit Box fall hole filled: Floor with the crate in it, and the filled Landfill's cue.
func (b *Board) boxSunk(hole int16) sound.Effects {
	b.shownBoxes[hole] = b.State.Replay.Boxes[hole]
	b.landfillDurability[hole] = 0
	b.State.revision++
	return sound.LandfillFull
}

// boxSounds adds the cues of what the wave just launched moves: a knock for sliding Boxes, a boing for Springs.
func (b *Board) boxSounds() (effects sound.Effects) {
	for _, s := range b.slides {
		if !s.stack {
			effects |= sound.BoxSlide
		}
	}
	for _, m := range b.moving {
		if m&squashed != 0 {
			return effects | sound.Spring
		}
	}
	return
}

// crateFloorScale sizes the crate in a filled pit's Floor: the Box's inner frame, so it reads as sunk in, not standing.
const crateFloorScale = .72

// renderCrate draws a Box: a crate filling the Floor tile, a step darker than Floor, with an inner frame and an X
// brace, in fill and ink. A pit a Box filled shows the same crate in its Floor at crateFloorScale, on Floor's fill, its
// lines darker toward the pit ink: deeper is darker.
func renderCrate(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, fill, ink color.RGBA) {
	body, _, radius, _, width := mapDeviceGeometry(r)
	ui.FillRound(dst, body, radius, fill)
	ui.StrokeRound(dst, body, radius, width, ink)
	inset := body.W * .14
	inner, innerRadius := body.Inset(inset), max(0, radius-inset)
	ui.StrokeRound(dst, inner, innerRadius, width, ink)
	cut := innerRadius * (1 - math.Sqrt2/2) // the brace meets the frame's rounded corners
	ui.Line(dst, inner.X+cut, inner.Y+cut, inner.Right()-cut, inner.Bottom()-cut, width, ink)
	ui.Line(dst, inner.Right()-cut, inner.Y+cut, inner.X+cut, inner.Bottom()-cut, width, ink)
}

// springSquash is how many steps a Spring's pad can be drawn pressed toward its base.
const springSquash = 4

// springReach is how far a Spring stands off its mount at rest, in tile bodies: about 80% of the tile, leaving its
// front open so it reads which way it launches.
const springReach = .656

// renderSpring draws a Spring side on, facing screen side (1 up to 4 left): a looped coil as wide as the tile, standing
// on the mount drawSpringMounts draws on its Wall's edge, under a pad plate pressed squash steps of springSquash
// toward the mount.
func renderSpring(dst *ebiten.Image, sc *ui.Palette, r ui.Rect, side, squash uint8) {
	body, _, _, _, width := mapDeviceGeometry(r)
	cx, cy, w := body.Cx(), body.Cy(), body.W
	base := -.5 - (r.W*TileGutter-barrierWidth(r.W)/2)/w // the mount's face, in the tile's gutter
	pad := base + springReach*(1-.44*float64(squash)/springSquash)
	amp := .5 - width/2/w                            // the coil fills the tile across
	point := func(u, v float64) (float64, float64) { // u across, v from the base (-) to the pad (+), facing up
		switch side {
		case 2:
			return cx + v*w, cy + u*w
		case 3:
			return cx - u*w, cy + v*w
		case 4:
			return cx - v*w, cy - u*w
		}
		return cx + u*w, cy - v*w
	}
	x0, y0 := point(-amp, pad)
	x1, y1 := point(amp, pad)
	ui.Line(dst, x0, y0, x1, y1, width, sc.Dim)
	// A prolate cycloid: its loops run back on themselves, a coil seen from the side. lo is its lowest point, so the
	// loops just touch the mount and the pad.
	const loops, b, steps = 3, 2.2, 72
	c := math.Acos(1 / b)
	lo := c - b*math.Sin(c)
	span := 2*math.Pi*loops - 2*lo
	var path vector.Path
	for k := range steps + 1 {
		t := 2 * math.Pi * loops * float64(k) / steps
		x, y := point(-amp*math.Cos(t), base+(pad-base)*(t-b*math.Sin(t)-lo)/span)
		if k == 0 {
			path.MoveTo(float32(x), float32(y))
		} else {
			path.LineTo(float32(x), float32(y))
		}
	}
	ui.StrokePath(dst, &path, width, sc.Dim)
}
