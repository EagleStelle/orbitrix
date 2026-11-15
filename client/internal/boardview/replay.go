package boardview

import (
	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

// view is the board as the players see it. core.Game.Apply resolves a whole move at once; the view starts from
// the position before the move and replays the recorded waves one by one as they animate, with the same two-pass
// rule as core (all exploding cells launch, then all neighbours receive), so it ends equal to the game.
type view struct {
	Geo *core.Geometry
	core.Cells
	from []int8 // owner of each cell of the wave in flight, as it launched
	deal []int8 // a team game's colours of that wave's orbs, by cell and target slot as core.Cells.Pop gives them
	// The cells whose map art takes their holder's ink, and the holder each was drawn with.
	inked []int16
	ink   []int8

	// replay keeps terrain at the displayed wave, independent of the fully
	// resolved game's topology. Its orb arrays are not used for playback.
	Replay   *core.Game
	Events   []core.TerrainEvent
	wave     int
	revision uint64
	dropped  []fallingOrb // orbs on the Breakable Floors, Sinkholes and opened Hatches the last terrain step took away, for Board to drop
	pending  []boxPending // orbs of the wave in flight sent into Boxes
	hits     []fallingOrb // orbs the last landing lost on Vault shields, for Board to shrink at the shield
	shields  sound.Effects
	shots    int // Turrets at the front of the wave in flight that fire rather than explode, as in core.Game
}

// shielded reports whether Vault i's shield takes owner's orb arriving there, and keeps the shield's cue: a knock,
// or falling woody notes as it breaks.
func (v *view) shielded(i int, owner int8, from int16) bool {
	hit, state := core.Shielded(v.Replay.Pressed[i], owner)
	if !hit || v.Mix != nil && v.Allied(core.Holder(v.Replay.Pressed[i]), owner) { // a teammate's orb goes in
		return false
	}
	v.Replay.Pressed[i] = state
	v.hits = append(v.hits, fallingOrb{hole: int16(i), owner: owner, shield: true, from: from})
	v.shields |= sound.WallHit
	if core.Shield(state) == 0 {
		v.shields |= sound.WallBreak
	}
	return true
}

func NewView(g *core.Game) *view {
	n := g.Geo.Cells()
	v := &view{Geo: g.Geo, from: make([]int8, 0, n)}
	v.sync(g)
	return v
}

func (v *view) sync(g *core.Game) {
	v.Geo, v.Events, v.wave, v.hits, v.shots = g.Geo, nil, 0, v.hits[:0], 0
	if g.Map != nil {
		if v.Replay == nil {
			v.Replay = g.Clone()
		} else {
			v.Replay.CopyFrom(g)
		}
		v.Geo = v.Replay.Geo
	} else {
		v.Replay = nil
	}
	v.revision++
	v.Cells.CopyFrom(&g.Cells)
	v.inked, v.ink = v.inked[:0], v.ink[:0]
	if g.Map != nil {
		for i, c := range g.Map.Cells {
			if mapOwnerInked(c.Feature) {
				v.inked, v.ink = append(v.inked, int16(i)), append(v.ink, v.Owner[i])
			}
		}
	}
}

// reink has the map art drawn again once a device's holder, whose ink it takes, has changed.
func (v *view) reink() {
	for k, i := range v.inked {
		if o := v.Owner[i]; o != v.ink[k] {
			v.ink[k] = o
			v.revision++
		}
	}
}

// arrive gives cell i n of owner's orbs, which capture it; a team game's land together at the next Cells.Land.
func (v *view) arrive(i int, owner int8, n uint8) {
	if v.Mix != nil {
		v.Send(i, owner, n)
	} else {
		v.Add(i, owner, n)
	}
}

// flew is the colour of the orb the k-th cell of the wave in flight sent out of target slot, core.MaxConnections for
// a Reservoir's extra orb: its owner's, or in a team game the one core.Cells.Pop dealt.
func (v *view) flew(k, slot int) int8 {
	if v.Mix == nil || k < v.shots {
		return v.from[k]
	}
	return v.deal[k*(core.MaxConnections+1)+slot]
}

func (v *view) Place(cell int, p int8) []core.TerrainEvent {
	v.wave = 0
	if v.Replay != nil {
		for i, c := range v.Replay.Map.Cells {
			if c.Feature == core.Reservoir {
				v.Replay.Pressed[i] &^= core.ReservoirMade // each may make an orb again, as in core.Game
			}
		}
	}
	v.Add(cell, p, 1)
	return v.applyTerrain(-1)
}

// launch takes the orbs every cell in w releases, as core does. The launched orbs are in flight until land; those
// aimed at Holes or a Spillway's pit never land.
func (v *view) Launch(w core.Wave) {
	v.from, v.deal = v.from[:0], v.deal[:0]
	for k, e := range w {
		p := v.Owner[e]
		v.from = append(v.from, p)
		var deal [core.MaxConnections + 1]int8
		switch {
		case k < v.shots: // a Turret's shot is a new orb
		case v.Mix != nil:
			deal = v.Pop(v.Geo, int(e))
		default:
			v.Remove(int(e), p, v.Geo.Released(int(e)))
		}
		if v.Mix != nil {
			v.deal = append(v.deal, deal[:]...)
		}
		if k >= v.shots && v.Geo.Outlets != nil && v.Geo.Outlets[e] != 0 {
			var makes bool
			if makes, v.Replay.Pressed[e] = core.ReservoirMakes(v.Replay.Pressed[e]); makes {
				v.Add(int(e), p, 1)
			}
		}
	}
	v.reink() // an emptied Spillway returns to neutral ink
}

// land gives one orb to each neighbour of every cell in w, and a Reservoir's extra orb to the side its arrow points
// to, capturing them for the launching cell's owner.
func (v *view) Land(w core.Wave) []core.TerrainEvent {
	v.hits, v.shields = v.hits[:0], 0
	for k, e := range w {
		to, degree := v.Geo.Neighbours(int(e)), int(v.Geo.Degree[e])
		if k < v.shots {
			var slot int
			slot, v.Replay.Durability[e] = v.Geo.Aim(int(e), v.Replay.Durability[e])
			to = to[slot : slot+1]
			v.revision++ // its barrel turned
		} else if v.Geo.Outlets != nil && v.Geo.Outlets[e] != 0 {
			var slot int
			slot, v.Replay.Pressed[e] = v.Geo.Outlet(int(e), v.Replay.Pressed[e])
			var buf [core.MaxConnections + 1]int16
			to = append(append(buf[:0], to...), to[slot])
			v.revision++ // its arrow turned
		}
		for slot, n := range to {
			if slot == degree {
				slot = core.MaxConnections // a Reservoir's extra orb
			}
			owner := v.flew(k, slot)
			if v.boxed(int(n)) {
				v.pending = append(v.pending, boxPending{box: n, from: e, owner: owner})
				continue
			}
			if v.Replay != nil && v.shielded(int(n), owner, e) {
				continue
			}
			v.arrive(int(n), owner, 1)
		}
	}
	if v.Mix != nil {
		v.Cells.Land(nil, nil)
	}
	v.reink()
	events := v.applyTerrain(v.wave)
	v.wave++
	v.shots = 0
	return events
}

// Produce runs the mover's Furnaces, Crushers and Vaults as core.Game does after the move's first chain. It reports
// whether a Furnace made an orb, appends each Crusher that lost one to crushed, sets shields when a Vault raised
// one, and counts in shots the Turrets firing in the wave that follows.
func (v *view) Produce(p int8, crushed []int16) (bool, []int16) {
	made := false
	v.shields, v.shots = 0, 0
	for i, c := range v.Replay.Map.Cells {
		if v.Owner[i] != p || !c.Feature.IsMaker() {
			continue
		}
		var works bool
		if v.Replay.Pressed[i], works = core.MakerTurn(v.Replay.Pressed[i], p); !works {
			continue
		}
		v.revision++ // its frame closes once it works
		switch {
		case c.Feature == core.Vault:
			var raised bool
			if v.Replay.Pressed[i], raised = core.RaiseShield(v.Replay.Pressed[i]); raised {
				v.shields |= sound.Shield
			}
		case c.Feature == core.Turret:
			v.Replay.Pressed[i] = core.TurretFired(v.Replay.Pressed[i])
			if v.Geo.Aims != nil && v.Geo.Aims[i] != 0 {
				v.shots++
			}
		case c.Feature == core.Crusher:
			crushed = append(crushed, int16(i))
			v.Remove(i, p, 1)
		case v.Count[i] < 255:
			v.Add(i, p, 1)
			made = true
		}
	}
	v.reink()
	return made, crushed
}

// settle resets the Furnaces and Crushers the move left empty, as core.Game does once a move settles.
func (v *view) settle() {
	if v.Replay == nil {
		return
	}
	for i, c := range v.Replay.Map.Cells {
		if c.Feature.IsMaker() && v.Count[i] == 0 {
			v.Replay.Pressed[i] = 0
		}
	}
}

func (v *view) applyTerrain(wave int) []core.TerrainEvent {
	end := 0
	for end < len(v.Events) && v.Events[end].Wave == wave {
		end++
	}
	v.dropped = v.dropped[:0]
	if v.Replay != nil && (end > 0 || len(v.pending) > 0) {
		v.moveBoxes(v.Events[:end])
	}
	if end > 0 && v.Replay != nil {
		v.Replay.ReplayTerrain(v.Events[:end])
		v.Geo = v.Replay.Geo
		v.revision++
		for _, e := range v.Events[:end] {
			if c := e.Cell; !e.Edge && !e.Switch && !e.Box && c >= 0 && e.Durability == 0 && v.Count[c] > 0 && (v.Replay.Map.Cells[c].Terrain == core.BreakableFloor || v.Replay.Map.Cells[c].Terrain == core.Sinkhole || v.Replay.Map.Cells[c].Circuit == core.Hatch) {
				v.dropped = append(v.dropped, fallingOrb{hole: int16(c), owner: v.Owner[c]})
				v.Lift(c, nil)
			}
		}
	}
	v.reink()
	events := v.Events[:end]
	v.Events = v.Events[end:]
	return events
}
