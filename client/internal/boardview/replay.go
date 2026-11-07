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

func (v *view) Place(cell int, p int8) {
	v.revision++
	v.wave = 0
	v.Owner[cell] = p
	v.Count[cell]++
	v.applyTerrain(-1)
}

// launch takes one orb per connection out of every cell in w; a Reservoir keeps
// its reserve. The launched orbs are in flight until land.
func (v *view) Launch(w core.Wave) {
	v.revision++
	v.from = v.from[:0]
	for _, e := range w {
		v.from = append(v.from, v.Owner[e])
		v.Count[e] -= v.Geo.Degree[e]
		if v.Count[e] == 0 {
			v.Owner[e] = core.NoOwner
		}
	}
}

// land gives one orb to each neighbour of every cell in w, capturing it for the launching cell's owner.
func (v *view) Land(w core.Wave) {
	v.revision++
	for k, e := range w {
		for _, n := range v.Geo.Neighbours(int(e)) {
			v.Owner[n] = v.from[k]
			v.Count[n]++
		}
	}
	v.applyTerrain(v.wave)
	v.wave++
}

func (v *view) applyTerrain(wave int) {
	end := 0
	for end < len(v.Events) && v.Events[end].Wave == wave {
		end++
	}
	if end > 0 && v.Replay != nil {
		v.Replay.ReplayTerrain(v.Events[:end])
		v.Geo = v.Replay.Geo
		v.revision++
	}
	v.Events = v.Events[end:]
}
