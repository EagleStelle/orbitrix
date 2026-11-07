package client

import (
	"math"
	"testing"

	"orbitrix/core"
)

func TestReservoirColorFollowsDisplayedOccupancy(t *testing.T) {
	for mode := range schemes {
		sc := &schemes[mode]
		state := mapArtState{Owner: []int8{0}, Count: []uint8{1}}
		for owner := range core.MaxPlayers {
			state.Owner[0] = int8(owner)
			if got := mapReservoirColor(sc, state, 0); got != sc.players[owner] {
				t.Fatalf("mode %d owner %d: reservoir ink %v", mode, owner, got)
			}
		}
		state.Count[0] = 0
		if got := mapReservoirColor(sc, state, 0); got != sc.dim {
			t.Fatal("empty reservoir retained ownership color")
		}
		state.Count[0], state.Owner[0] = 1, core.NoOwner
		if got := mapReservoirColor(sc, state, 0); got != sc.dim {
			t.Fatal("unowned reservoir used a player color")
		}
		if got := mapReservoirColor(sc, mapArtState{}, 0); got != sc.dim {
			t.Fatal("editor reservoir is not neutral")
		}
	}
}

func TestMapGroupsUseExactOrbColors(t *testing.T) {
	for mode := range schemes {
		sc := &schemes[mode]
		for group := 1; group <= core.MaxMapGroups; group++ {
			if got := partnerColor(sc, uint8(group)); got != sc.players[group-1] {
				t.Errorf("mode %d group %d: %v differs from orb %v", mode, group, got, sc.players[group-1])
			}
		}
		for _, invalid := range []uint8{0, core.MaxMapGroups + 1, 255} {
			if got := partnerColor(sc, invalid); got != sc.dim {
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
	state := mapArtState{Map: &m}
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
	state := mapArtState{Map: &m}
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
		x0, y0, x1, y1, ok := mapArtEdge(&m, core.MapEdge{A: tc.a, B: tc.b})
		if got := [4]float64{x0, y0, x1, y1}; !ok || got != tc.want {
			t.Errorf("edge %d/%d = %v, %v; want %v, true", tc.a, tc.b, got, ok, tc.want)
		}
	}
	for _, e := range []core.MapEdge{{A: -1, B: 0}, {A: 0, B: 5}, {A: 3, B: 4}, {A: 0, B: 12}} {
		if _, _, _, _, ok := mapArtEdge(&m, e); ok {
			t.Errorf("invalid edge accepted: %+v", e)
		}
	}
}

func TestMapArtQuarterTurnKeepsVertexJoins(t *testing.T) {
	tr := mapArtTransform{X: 7, Y: 11, Cell: 20, Turned: true}
	for _, tc := range []struct {
		x, y, wantX, wantY float64
	}{{0, 0, 87, 11}, {1, 2, 47, 31}, {3, 4, 7, 71}} {
		x, y := tr.point(4, tc.x, tc.y)
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
		state := mapArtState{Map: &m}
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
