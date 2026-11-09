package boardview

import "orbitrix/core"

// view is the board as the players see it. core.Game.Apply resolves a whole move at once; the view starts from
// the position before the move and replays the recorded waves one by one as they animate, with the same two-pass
// rule as core (all exploding cells launch, then all neighbours receive), so it ends equal to the game.
type view struct {
	Geo   *core.Geometry
	Count []uint8
	Owner []int8
	from  []int8 // owner of each cell of the wave in flight, as it launched

	// replay keeps terrain at the displayed wave, independent of the fully
	// resolved game's topology. Its orb arrays are not used for playback.
	Replay   *core.Game
	Events   []core.TerrainEvent
	wave     int
	revision uint64
}

func NewView(g *core.Game) *view {
	n := g.Geo.Cells()
	v := &view{Geo: g.Geo, Count: make([]uint8, n), Owner: make([]int8, n), from: make([]int8, 0, n)}
	v.sync(g)
	return v
}

func (v *view) sync(g *core.Game) {
	v.Geo, v.Events, v.wave = g.Geo, nil, 0
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
	copy(v.Count, g.Count)
	copy(v.Owner, g.Owner)
}

func (v *view) Place(cell int, p int8) []core.TerrainEvent {
	v.inkChanged(cell, p)
	v.wave = 0
	if v.Replay != nil {
		for i, c := range v.Replay.Map.Cells {
			if c.Feature == core.Reservoir {
				v.Replay.Pressed[i] &^= core.ReservoirMade // each may make an orb again, as in core.Game
			}
		}
	}
	v.Owner[cell] = p
	v.Count[cell]++
	return v.applyTerrain(-1)
}

// launch takes the orbs every cell in w releases, as core does. The launched orbs are in flight until land; those
// aimed at Holes or a Spillway's pit never land.
func (v *view) Launch(w core.Wave) {
	v.from = v.from[:0]
	for _, e := range w {
		v.from = append(v.from, v.Owner[e])
		v.Count[e] -= v.Geo.Released(int(e))
		if v.Geo.Outlets != nil && v.Geo.Outlets[e] != 0 {
			var makes bool
			if makes, v.Replay.Pressed[e] = core.ReservoirMakes(v.Replay.Pressed[e]); makes {
				v.Count[e]++
			}
		}
		if v.Count[e] == 0 {
			v.inkChanged(int(e), core.NoOwner) // an emptied Spillway returns to neutral ink
			v.Owner[e] = core.NoOwner
		}
	}
}

// land gives one orb to each neighbour of every cell in w, and a Reservoir's extra orb to the side its arrow points
// to, capturing them for the launching cell's owner.
func (v *view) Land(w core.Wave) []core.TerrainEvent {
	for k, e := range w {
		to := v.Geo.Neighbours(int(e))
		if v.Geo.Outlets != nil && v.Geo.Outlets[e] != 0 {
			var slot int
			slot, v.Replay.Pressed[e] = v.Geo.Outlet(int(e), v.Replay.Pressed[e])
			var buf [core.MaxConnections + 1]int16
			to = append(append(buf[:0], to...), to[slot])
			v.revision++ // its arrow turned
		}
		for _, n := range to {
			v.inkChanged(int(n), v.from[k])
			v.Owner[n] = v.from[k]
			v.Count[n]++
		}
	}
	events := v.applyTerrain(v.wave)
	v.wave++
	return events
}

// Produce runs the mover's Factories and Crushers as core.Game does after the move's first chain. It reports whether a
// Factory made an orb and appends each Crusher that lost one to crushed.
func (v *view) Produce(p int8, crushed []int16) (bool, []int16) {
	made := false
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
		case c.Feature == core.Crusher:
			crushed = append(crushed, int16(i))
			if v.Count[i] == 1 {
				v.inkChanged(i, core.NoOwner)
				v.Owner[i] = core.NoOwner
			}
			v.Count[i]--
		case v.Count[i] < 255:
			v.Count[i]++
			made = true
		}
	}
	return made, crushed
}

// settle resets the Factories and Crushers the move left empty, as core.Game does once a move settles.
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

func (v *view) inkChanged(cell int, owner int8) {
	if v.Replay == nil {
		return
	}
	if mapOwnerInked(v.Replay.Map.Cells[cell].Feature) && (v.Count[cell] == 0 || v.Owner[cell] != owner) {
		v.revision++
	}
}

func (v *view) applyTerrain(wave int) []core.TerrainEvent {
	end := 0
	for end < len(v.Events) && v.Events[end].Wave == wave {
		end++
	}
	if end > 0 && v.Replay != nil {
		v.Replay.ReplayTerrain(v.Events[:end])
		v.Geo = v.Replay.Geo
		v.revision++
	}
	events := v.Events[:end]
	v.Events = v.Events[end:]
	return events
}
