package client

import "orbitrix/core"

// view is the board as the players see it. core.Game.Apply resolves a whole move at once; the view starts from
// the position before the move and replays the recorded waves one by one as they animate, with the same two-pass
// rule as core (all exploding cells launch, then all neighbours receive), so it ends equal to the game.
type view struct {
	geo   *core.Geometry
	count []uint8
	owner []int8
	from  []int8 // owner of each cell of the wave in flight, as it launched
}

func newView(g *core.Game) *view {
	n := g.Geo.Cells()
	v := &view{geo: g.Geo, count: make([]uint8, n), owner: make([]int8, n), from: make([]int8, 0, n)}
	v.sync(g)
	return v
}

func (v *view) sync(g *core.Game) {
	copy(v.count, g.Count)
	copy(v.owner, g.Owner)
}

func (v *view) place(cell int, p int8) {
	v.owner[cell] = p
	v.count[cell]++
}

// launch takes capacity orbs out of every cell in w; they are in flight until land.
func (v *view) launch(w core.Wave) {
	v.from = v.from[:0]
	for _, e := range w {
		v.from = append(v.from, v.owner[e])
		v.count[e] -= v.geo.Cap[e]
		if v.count[e] == 0 {
			v.owner[e] = core.NoOwner
		}
	}
}

// land gives one orb to each neighbour of every cell in w, capturing it for the launching cell's owner.
func (v *view) land(w core.Wave) {
	for k, e := range w {
		for _, n := range v.geo.Neighbours(int(e)) {
			v.owner[n] = v.from[k]
			v.count[n]++
		}
	}
}
