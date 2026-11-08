package core

import (
	"slices"
	"strings"
	"testing"
)

func portalMap() MapDefinition {
	m := NewMap(7, 7)
	m.Name = "Portals"
	m.Cells[16] = MapCell{Feature: Portal, Group: 1}
	m.Cells[32] = MapCell{Feature: Portal, Group: 1}
	return m
}

func TestPortalDirections(t *testing.T) {
	for _, pair := range [][2]int{{16, 32}, {32, 16}} {
		for direction, offset := range []int{-7, 7, -1, 1} {
			m := portalMap()
			g, err := NewGameWithMap(m, 2)
			if err != nil {
				t.Fatal(err)
			}
			source, landing := pair[0]-offset, pair[1]+offset
			if m.FloorCount() != 47 || g.Legal(pair[0]) || slices.Contains(g.LegalMoves(nil), pair[1]) || m.Capacity(pair[0]) != 0 {
				t.Fatal("Portal endpoints must never hold or accept orbs")
			}
			if _, err := g.Apply(pair[0]); err == nil {
				t.Fatal("placed an orb on a Portal")
			}
			if g.Geo.Cap[source] != 4 || m.Capacity(source) != 4 || g.Geo.Neighbours(source)[direction] != int16(landing) {
				t.Fatalf("direction %d: source capacity/routes = %d/%v", direction, g.Geo.Cap[source], g.Geo.Neighbours(source))
			}
			if got := g.Geo.PortalPath(source, direction); !slices.Equal(got, []int16{int16(pair[0]), int16(pair[1])}) {
				t.Fatalf("Portal path = %v", got)
			}
			seedMapCell(g, source, 0, 3)
			seedMapCell(g, landing, 1, 1)
			seedMapCell(g, 48, 1, 1)
			g.Played = 3
			simulation := g.Clone()
			waves, err := g.Apply(source)
			if err != nil || len(waves) != 1 || g.Count[landing] != 2 || g.Owner[landing] != 0 || g.Orbs[0]+g.Orbs[1] != 6 {
				t.Fatalf("direction %d: landing count/owner = %d/%d, waves = %v, err = %v", direction, g.Count[landing], g.Owner[landing], waves, err)
			}
			for _, endpoint := range pair {
				if g.Count[endpoint] != 0 || g.Owner[endpoint] != NoOwner {
					t.Fatal("an explosion left an orb on a Portal")
				}
			}
			if err := simulation.Simulate(source); err != nil || simulation.Hash() != g.Hash() {
				t.Fatal("Portal simulation differs from Apply")
			}
			restored, err := Restore(g.Snapshot())
			if err != nil || restored.Hash() != g.Hash() {
				t.Fatalf("Portal snapshot differs: %v", err)
			}
		}
	}
}

func TestPortalChainingAndGroupIsolation(t *testing.T) {
	m := NewMap(9, 9)
	for _, pair := range [][3]int{{20, 50, 1}, {59, 30, 2}} {
		for _, endpoint := range pair[:2] {
			m.Cells[endpoint] = MapCell{Feature: Portal, Group: uint8(pair[2])}
		}
	}
	m.Cells[10], m.Cells[70] = MapCell{Feature: Link, Group: 1}, MapCell{Feature: Link, Group: 1}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Geo.PortalPath(11, 1); !slices.Equal(got, []int16{20, 50, 59, 30}) || g.Geo.Neighbours(11)[1] != 39 {
		t.Fatalf("chained route = %v, neighbors = %v", got, g.Geo.Neighbours(11))
	}
	if got := g.Geo.PortalPath(39, 0); !slices.Equal(got, []int16{30, 59, 50, 20}) || g.Geo.Neighbours(39)[0] != 11 {
		t.Fatalf("reverse chained route = %v", got)
	}
	if !slices.Contains(g.Geo.Neighbours(10), 70) || g.Geo.Cap[10] != 5 {
		t.Fatal("Portal color interfered with a Link pair")
	}
	for i := range m.Cells {
		initial := m.InitialConnections(i, nil)
		actual := g.Geo.Neighbours(i)
		if len(initial) != len(actual) || m.Capacity(i) != int(g.Geo.Cap[i]) {
			t.Fatalf("editor and game disagree at %d", i)
		}
		for j := range initial {
			if initial[j] != int(actual[j]) {
				t.Fatalf("editor route differs at %d", i)
			}
		}
	}
	for _, cell := range []int{20, 50} {
		m.Cells[cell].PortalMode = PortalHorizontal
	}
	for _, cell := range []int{59, 30} {
		m.Cells[cell].PortalMode = PortalVertical
	}
	g, err = NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Geo.PortalAxes[20] != VerticalAxis || m.AvailablePortalAxes(20) != VerticalAxis {
		t.Fatal("a Portal closing the horizontal chain must lock its upstream pair vertically")
	}
}

func TestPortalValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(*MapDefinition)
		issue string
	}{
		{"missing pair", func(m *MapDefinition) { m.Cells[32] = MapCell{} }, "Portal group"},
		{"adjacent pair", func(m *MapDefinition) { m.Cells[17], m.Cells[32] = m.Cells[32], MapCell{} }, "Portal group"},
		{"third endpoint", func(m *MapDefinition) { m.Cells[24] = m.Cells[16] }, "Portal group"},
		{"zero group", func(m *MapDefinition) { m.Cells[16].Group, m.Cells[32].Group = 0, 0 }, "Portal group"},
		{"extra color", func(m *MapDefinition) { m.Cells[16].Group, m.Cells[32].Group = 13, 13 }, "12 group colors"},
		{"both directions blocked", func(m *MapDefinition) { m.Cells[39].Terrain, m.Cells[33].Terrain = Wall, Wall }, "Portal needs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := portalMap()
			tc.edit(&m)
			if err := m.ValidateStructure(); err != nil {
				t.Fatalf("invalid layout must remain a saveable draft: %v", err)
			}
			for _, issue := range m.Validate() {
				if strings.Contains(issue.Message, tc.issue) {
					if _, err := NewGameWithMap(m, 2); err == nil {
						t.Fatal("invalid Portal layout started a match")
					}
					return
				}
			}
			t.Fatalf("missing %q issue", tc.issue)
		})
	}
	g, err := NewGameWithMap(portalMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := g.Snapshot()
	snapshot.Count[16], snapshot.Owner[16] = 1, 0
	if _, err := Restore(snapshot); err == nil {
		t.Fatal("restored an orb onto a Portal")
	}
}

func TestPortalModesAndBlockedAxes(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode PortalMode
		edit func(*MapDefinition)
		want PortalAxes
	}{
		{"all", PortalBoth, nil, BothAxes},
		{"horizontal", PortalHorizontal, nil, HorizontalAxis},
		{"vertical", PortalVertical, nil, VerticalAxis},
		{"right wall", PortalBoth, func(m *MapDefinition) { m.Cells[39].Terrain = Wall }, VerticalAxis},
		{"left void", PortalBoth, func(m *MapDefinition) { m.Cells[9].Terrain = Void }, VerticalAxis},
		{"top wall", PortalBoth, func(m *MapDefinition) { m.Cells[15].Terrain = Wall }, HorizontalAxis},
		{"bottom barrier", PortalBoth, func(m *MapDefinition) { m.Edges = []MapEdge{{A: 32, B: 33, Kind: Barrier}} }, HorizontalAxis},
		{"forced vertical", PortalHorizontal, func(m *MapDefinition) { m.Cells[39].Terrain = Wall }, VerticalAxis},
		{"board edge", PortalBoth, func(m *MapDefinition) { m.Cells[46], m.Cells[32] = m.Cells[32], MapCell{} }, VerticalAxis},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := portalMap()
			m.Cells[16].PortalMode, m.Cells[32].PortalMode = tc.mode, tc.mode
			if tc.edit != nil {
				tc.edit(&m)
			}
			g, err := NewGameWithMap(m, 2)
			if err != nil {
				t.Fatal(err)
			}
			axes := m.InitialPortalAxes()
			for i, c := range m.Cells {
				if c.Feature == Portal && (axes[i] != tc.want || g.Geo.PortalAxes[i] != tc.want) {
					t.Fatalf("endpoint %d axes = %v/%v, want %v", i, axes[i], g.Geo.PortalAxes[i], tc.want)
				}
				if int(g.Geo.Cap[i]) != m.Capacity(i) {
					t.Fatalf("editor/game capacity differs at %d", i)
				}
				for j, landing := range g.Geo.Neighbours(i) {
					path := g.Geo.PortalPath(i, j)
					if len(path) == 0 {
						continue
					}
					axis := VerticalAxis
					if i/m.Rows != int(path[0])/m.Rows {
						axis = HorizontalAxis
					}
					if axis&tc.want == 0 || m.Cells[landing].Feature == Portal {
						t.Fatal("orb used a disabled axis or landed on a Portal")
					}
				}
			}
			restored, err := Restore(g.Snapshot())
			if err != nil || restored.Hash() != g.Hash() {
				t.Fatalf("directional snapshot differs: %v", err)
			}
		})
	}
}

func TestPortalAxisReopensAfterDestruction(t *testing.T) {
	m := portalMap()
	m.Cells[39].Terrain = BreakableWall
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	before := g.Geo
	if before.PortalAxes[16] != VerticalAxis {
		t.Fatal("breakable wall did not lock the pair vertically")
	}
	g.ReplayTerrain([]TerrainEvent{{Cell: 39, Durability: 0}})
	if g.Geo.PortalAxes[16] != BothAxes || before.PortalAxes[16] != VerticalAxis {
		t.Fatal("destroyed wall did not reopen the axis in an isolated topology")
	}
}
