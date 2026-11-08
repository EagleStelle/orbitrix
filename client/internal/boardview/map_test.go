package boardview

import (
	"math"
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

func TestReservoirColorFollowsDisplayedOccupancy(t *testing.T) {
	for mode := range ui.Palettes {
		sc := &ui.Palettes[mode]
		state := MapState{Owner: []int8{0}, Count: []uint8{1}}
		for owner := range core.MaxPlayers {
			state.Owner[0] = int8(owner)
			if got := mapReservoirColor(sc, state, 0); got != sc.Players[owner] {
				t.Fatalf("mode %d owner %d: reservoir ink %v", mode, owner, got)
			}
		}
		state.Count[0] = 0
		if got := mapReservoirColor(sc, state, 0); got != sc.Dim {
			t.Fatal("empty reservoir retained ownership color")
		}
		state.Count[0], state.Owner[0] = 1, core.NoOwner
		if got := mapReservoirColor(sc, state, 0); got != sc.Dim {
			t.Fatal("unowned reservoir used a player color")
		}
		if got := mapReservoirColor(sc, MapState{}, 0); got != sc.Dim {
			t.Fatal("editor reservoir is not neutral")
		}
	}
}

func TestMapRevisionTracksArtChanges(t *testing.T) {
	m := core.NewMap(5, 5)
	m.Cells[12].Feature = core.Reservoir
	m.Cells[18] = core.MapCell{Circuit: core.Hold, Group: 1}
	m.Cells[8] = core.MapCell{Circuit: core.Door, Group: 1, InitiallyOpen: true}
	g, err := core.NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	v := NewView(g)
	revision := v.revision
	v.Place(0, 0)
	v.Count[0] = v.Geo.Cap[0]
	v.Launch(core.Wave{0})
	v.Land(core.Wave{0})
	if v.revision != revision {
		t.Fatal("ordinary placement or wave invalidates unchanged map art")
	}
	v.Place(12, 0)
	if v.revision == revision {
		t.Fatal("occupying a Reservoir did not change its ink")
	}
	revision = v.revision
	v.Place(12, 0)
	if v.revision != revision {
		t.Fatal("same-owner Reservoir placement invalidates its unchanged ink")
	}
	v.Count[11], v.Owner[11] = v.Geo.Cap[11], 1
	v.Launch(core.Wave{11})
	v.Land(core.Wave{11})
	if v.revision == revision || v.Owner[12] != 1 {
		t.Fatal("capturing a Reservoir did not invalidate its old ink")
	}
	revision = v.revision
	v.Events = []core.TerrainEvent{{Wave: -1, Cell: 18, Switch: true, Pressed: true}, {Wave: -1, Cell: -1, Group: 1, Open: true}}
	v.Place(18, 0)
	if v.revision == revision || v.Geo.Playable(8) {
		t.Fatal("circuit art or Door topology did not update")
	}
}

func TestGateEndpointIndexMatchesScan(t *testing.T) {
	m := core.NewMap(40, 40)
	for x := 0; x < m.Cols; x++ {
		for y := 0; y+1 < m.Rows; y++ {
			m.Edges = append(m.Edges, core.MapEdge{A: x*m.Rows + y, B: x*m.Rows + y + 1, Kind: core.Gate, Group: uint8(x%core.MaxMapGroups + 1), InitiallyOpen: y%5 == 0})
		}
	}
	shared := mapGateVertices(&m)
	for i, e := range m.Edges {
		x0, y0, x1, y1, _ := MapArtEdge(&m, e)
		for _, point := range [2][2]float64{{x0, y0}, {x1, y1}} {
			got := shared[int(point[0])*(m.Rows+1)+int(point[1])]&mapGateBit(e) != 0
			if got != mapArtGateContinuesAt(&m, m.Edges, i, point[0], point[1]) {
				t.Fatalf("edge %d endpoint %v changed its open Gate cap", i, point)
			}
		}
	}
}

func TestMapGroupsUseExactOrbColors(t *testing.T) {
	for mode := range ui.Palettes {
		sc := &ui.Palettes[mode]
		for group := 1; group <= core.MaxMapGroups; group++ {
			if got := PartnerColor(sc, uint8(group)); got != sc.Players[group-1] {
				t.Errorf("mode %d group %d: %v differs from orb %v", mode, group, got, sc.Players[group-1])
			}
		}
		for _, invalid := range []uint8{0, core.MaxMapGroups + 1, 255} {
			if got := PartnerColor(sc, invalid); got != sc.Dim {
				t.Errorf("invalid group %d aliases a player color", invalid)
			}
		}
	}
}

func TestMapArtWallConnectionsFollowRuntimeTerrain(t *testing.T) {
	m := core.NewMap(3, 3)
	m.Cells[4].Terrain = core.Wall
	m.Cells[3].Terrain = core.BreakableWall // north, same column
	m.Cells[7].Terrain = core.Wall          // east
	m.Cells[5].Terrain = core.BreakableWall // south
	m.Cells[1].Terrain = core.Void          // west
	state := MapState{Map: &m}
	if got, want := mapArtWallMask(&state, 4), mapNorth|mapEast|mapSouth; got != want {
		t.Fatalf("initial connection mask %04b, want %04b", got, want)
	}
	state.Durability = make([]uint8, len(m.Cells))
	state.Durability[5] = 1
	if got, want := mapArtWallMask(&state, 4), mapEast|mapSouth; got != want {
		t.Errorf("broken north wall still connects: %04b, want %04b", got, want)
	}
	if got := mapArtTerrain(&state, 3); got != core.Floor {
		t.Errorf("destroyed wall terrain %v, want Floor", got)
	}
	if got := mapArtTerrain(&state, 5); got != core.BreakableWall {
		t.Errorf("damaged wall terrain %v, want BreakableWall", got)
	}
	if got := mapArtTerrain(&state, 4); got != core.Wall {
		t.Errorf("solid wall used durability: %v", got)
	}
}

func TestMapArtWallConnectionsDoNotWrapRows(t *testing.T) {
	m := core.NewMap(3, 3)
	m.Cells[2].Terrain, m.Cells[3].Terrain = core.Wall, core.Wall
	state := MapState{Map: &m}
	if got := mapArtWallMask(&state, 2); got != 0 {
		t.Errorf("bottom of column connects to next column's top: %04b", got)
	}
	if got := mapArtWallMask(&state, 3); got != 0 {
		t.Errorf("top of column connects to previous column's bottom: %04b", got)
	}
}

func TestMapArtEdgesUseSharedGridVertices(t *testing.T) {
	m := core.NewMap(3, 4)
	for _, tc := range []struct {
		a, b int
		want [4]float64
	}{
		{1, 5, [4]float64{1, 1, 1, 2}},
		{5, 1, [4]float64{1, 1, 1, 2}},
		{5, 6, [4]float64{1, 2, 2, 2}},
		{6, 5, [4]float64{1, 2, 2, 2}},
	} {
		x0, y0, x1, y1, ok := MapArtEdge(&m, core.MapEdge{A: tc.a, B: tc.b})
		if got := [4]float64{x0, y0, x1, y1}; !ok || got != tc.want {
			t.Errorf("edge %d/%d = %v, %v; want %v, true", tc.a, tc.b, got, ok, tc.want)
		}
	}
	for _, e := range []core.MapEdge{{A: -1, B: 0}, {A: 0, B: 5}, {A: 3, B: 4}, {A: 0, B: 12}} {
		if _, _, _, _, ok := MapArtEdge(&m, e); ok {
			t.Errorf("invalid edge accepted: %+v", e)
		}
	}
}

func TestMapArtQuarterTurnKeepsVertexJoins(t *testing.T) {
	tr := MapTransform{X: 7, Y: 11, Cell: 20, Turned: true}
	for _, tc := range []struct {
		x, y, wantX, wantY float64
	}{{0, 0, 87, 11}, {1, 2, 47, 31}, {3, 4, 7, 71}} {
		x, y := tr.Point(4, tc.x, tc.y)
		if math.Abs(x-tc.wantX) > 1e-9 || math.Abs(y-tc.wantY) > 1e-9 {
			t.Errorf("turned vertex (%v,%v) = (%v,%v), want (%v,%v)", tc.x, tc.y, x, y, tc.wantX, tc.wantY)
		}
	}
}

func TestWallVerticesCoverEveryLCorner(t *testing.T) {
	for missing := range 4 {
		m := core.NewMap(3, 3)
		cells := [4]int{0, 3, 4, 1}
		for _, i := range cells {
			m.Cells[i].Terrain = core.Wall
		}
		m.Cells[cells[missing]].Terrain = core.Floor
		state := MapState{Map: &m}
		terrain, count := mapArtWallVertex(&state, 1, 1)
		if count != 3 || mapArtSolid(terrain[missing]) {
			t.Fatalf("L orientation %d has wrong vertex topology", missing)
		}
		for _, i := range cells {
			m.Cells[i].Terrain = core.BreakableWall
		}
		state.Durability = make([]uint8, len(m.Cells))
		for _, i := range cells {
			state.Durability[i] = 1
		}
		state.Durability[cells[missing]] = 0
		terrain, count = mapArtWallVertex(&state, 1, 1)
		if count != 3 || mapArtSolid(terrain[missing]) {
			t.Fatalf("destroyed wall did not expose L orientation %d", missing)
		}
	}
}
