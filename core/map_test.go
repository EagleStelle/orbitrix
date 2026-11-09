package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestMapConnectionsAndCapacity(t *testing.T) {
	m := NewMap(5, 5)
	center := 2*5 + 2
	want := []int{1*5 + 2, 3*5 + 2, 2*5 + 1, 2*5 + 3}
	if got := m.InitialConnections(center, nil); !slices.Equal(got, want) {
		t.Fatalf("connections = %v, want %v", got, want)
	}
	m.Cells[center].Feature, m.Cells[center].Group = Link, 7
	m.Cells[0].Feature, m.Cells[0].Group = Link, 7
	if got := m.InitialConnections(center, nil); !slices.Equal(got, append(want, 0)) || m.Capacity(center) != 5 {
		t.Fatalf("linked connections = %v, capacity = %d", got, m.Capacity(center))
	}
	if got := m.InitialConnections(0, nil); !slices.Contains(got, center) {
		t.Fatalf("Link is not bidirectional: %v", got)
	}
	m.Cells[center].Feature, m.Cells[center].Group = Reservoir, 0
	if got := m.Capacity(center); got != 5 {
		t.Fatalf("Reservoir capacity = %d, want 5", got)
	}
	m.Edges = []MapEdge{{A: center, B: center - 1, Kind: Barrier}, {A: center, B: center + 1, Kind: Gate, Group: 4}}
	if got := m.InitialConnections(center, nil); !slices.Equal(got, want[:2]) || m.Capacity(center) != 3 {
		t.Fatalf("blocked connections = %v, capacity = %d", got, m.Capacity(center))
	}
	for _, terrain := range []Terrain{Wall, Void, BreakableWall} {
		m.Cells[center] = MapCell{Terrain: terrain}
		if got := m.InitialConnections(center, nil); len(got) != 0 || m.Capacity(center) != 0 {
			t.Fatalf("terrain %d remains playable", terrain)
		}
	}
}

func TestMapDraftValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*MapDefinition)
		text string
	}{
		{"unpaired Link", func(m *MapDefinition) { m.Cells[12] = MapCell{Feature: Link, Group: 1} }, "Link group"},
		{"adjacent Links", func(m *MapDefinition) {
			m.Cells[12], m.Cells[13] = MapCell{Feature: Link, Group: 1}, MapCell{Feature: Link, Group: 1}
		}, "Link group"},
		{"three Links", func(m *MapDefinition) {
			for _, i := range []int{0, 12, 24} {
				m.Cells[i] = MapCell{Feature: Link, Group: 1}
			}
		}, "Link group"},
		{"zero Link group", func(m *MapDefinition) { m.Cells[12] = MapCell{Feature: Link} }, "Link group"},
		{"extra Link color", func(m *MapDefinition) {
			m.Cells[6], m.Cells[18] = MapCell{Feature: Link, Group: 13}, MapCell{Feature: Link, Group: 13}
		}, "12 group colors"},
		{"extra circuit color", func(m *MapDefinition) {
			m.Cells[6] = MapCell{Circuit: Latch, Group: 13}
			m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 13}}
		}, "12 group colors"},
		{"orphan Switch", func(m *MapDefinition) { m.Cells[12] = MapCell{Circuit: Latch, Group: 1} }, "circuit"},
		{"orphan Gate", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 1}} }, "circuit"},
		{"feature on Wall", func(m *MapDefinition) { m.Cells[12] = MapCell{Terrain: Wall, Feature: Reservoir} }, "Features need Floor"},
		{"unused group", func(m *MapDefinition) { m.Cells[12].Group = 1 }, "Only Links, Portals and circuit"},
		{"Barrier group", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 12, B: 13, Group: 1}} }, "Barriers do not use"},
		{"Breakable Barrier group", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 12, B: 13, Kind: BreakableBarrier, Group: 1}} }, "Barriers do not use"},
		{"Breakable Barrier on Wall", func(m *MapDefinition) {
			m.Cells[12].Terrain = Wall
			m.Edges = []MapEdge{{A: 12, B: 13, Kind: BreakableBarrier}}
		}, "need Floor or Landfill on both sides"},
		{"duplicate edge", func(m *MapDefinition) {
			m.Edges = []MapEdge{{A: 12, B: 13}, {A: 13, B: 12}}
		}, "Only one Barrier or Gate"},
		{"edge on Wall", func(m *MapDefinition) {
			m.Cells[12].Terrain = Wall
			m.Edges = []MapEdge{{A: 12, B: 13}}
		}, "need Floor or Landfill on both sides"},
		{"corner dead end", func(m *MapDefinition) { m.Cells[1].Terrain = Wall }, "capacity of at least two"},
		{"isolated Floor", func(m *MapDefinition) {
			for _, i := range []int{7, 11, 13, 17} {
				m.Cells[i].Terrain = Wall
			}
		}, "All Floor must connect"},
		{"weak Breakable Wall", func(m *MapDefinition) {
			m.Cells[0].Terrain, m.Cells[1].Terrain = BreakableWall, Wall
		}, "Breakable Wall needs"},
		{"no Floor", func(m *MapDefinition) {
			for i := range m.Cells {
				m.Cells[i].Terrain = Void
			}
		}, "Map needs playable Floor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMap(5, 5)
			tt.edit(&m)
			if err := m.ValidateStructure(); err != nil {
				t.Fatalf("unfinished draft is structurally rejected: %v", err)
			}
			issues := m.Validate()
			for _, issue := range issues {
				if strings.Contains(issue.Message, tt.text) {
					return
				}
			}
			t.Fatalf("issues = %v, want %q", issues, tt.text)
		})
	}
}

func TestMapStructureRejectsMalformedData(t *testing.T) {
	tests := []struct {
		name string
		edit func(*MapDefinition)
	}{
		{"small", func(m *MapDefinition) { m.Cols = 2 }},
		{"large", func(m *MapDefinition) { m.Rows = 41 }},
		{"missing cells", func(m *MapDefinition) { m.Cells = m.Cells[:24] }},
		{"unknown terrain", func(m *MapDefinition) { m.Cells[0].Terrain = 255 }},
		{"unknown feature", func(m *MapDefinition) { m.Cells[0].Feature = 255 }},
		{"unknown Portal mode", func(m *MapDefinition) { m.Cells[0] = MapCell{Feature: Portal, PortalMode: 3} }},
		{"Portal mode without Portal", func(m *MapDefinition) { m.Cells[0].PortalMode = PortalHorizontal }},
		{"unknown edge", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 0, B: 1, Kind: 255}} }},
		{"outside edge", func(m *MapDefinition) { m.Edges = []MapEdge{{A: -1, B: 0}} }},
		{"wrapped edge", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 4, B: 5}} }},
		{"diagonal edge", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 0, B: 6}} }},
		{"self edge", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 0, B: 0}} }},
		{"long name", func(m *MapDefinition) { m.Name = strings.Repeat("x", 65) }},
		{"invalid UTF8", func(m *MapDefinition) { m.Name = string([]byte{255}) }},
		{"too many edges", func(m *MapDefinition) { m.Edges = make([]MapEdge, 41) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMap(5, 5)
			tt.edit(&m)
			if m.ValidateStructure() == nil || len(m.Validate()) == 0 {
				t.Fatal("accepted malformed map")
			}
		})
	}
}

func TestMapReservoirDeadEnds(t *testing.T) {
	for _, obstruction := range []struct {
		name    string
		terrain Terrain
		edge    bool
		kind    EdgeKind
	}{
		{name: "Wall", terrain: Wall},
		{name: "Void", terrain: Void},
		{name: "Breakable Wall", terrain: BreakableWall},
		{name: "Barrier", edge: true, kind: Barrier},
		{name: "Gate", edge: true, kind: Gate},
	} {
		for _, connections := range []int{0, 1, 2} {
			for _, feature := range []Feature{NoFeature, Reservoir} {
				t.Run(fmt.Sprintf("%s/connections=%d/feature=%d", obstruction.name, connections, feature), func(t *testing.T) {
					m := NewMap(7, 7)
					const cell = 24
					m.Cells[cell].Feature = feature
					for _, neighbor := range []int{17, 31, 23, 25}[connections:] {
						if obstruction.edge {
							e := MapEdge{A: cell, B: neighbor, Kind: obstruction.kind}
							if e.Kind == Gate {
								e.Group = 1
								m.Cells[8] = MapCell{Circuit: Latch, Group: 1}
							}
							m.Edges = append(m.Edges, e)
						} else {
							m.Cells[neighbor].Terrain = obstruction.terrain
						}
					}
					valid := connections == 2 || connections == 1 && feature == Reservoir
					issues := m.Validate()
					if valid {
						if len(issues) != 0 {
							t.Fatalf("playable Floor rejected: %v", issues)
						}
					} else if !slices.ContainsFunc(issues, func(issue MapIssue) bool { return slices.Contains(issue.Cells, cell) }) {
						t.Fatalf("invalid Floor must be highlighted: %v", issues)
					}
					if _, err := NewGameWithMap(m, 2); (err == nil) != valid {
						t.Fatalf("game acceptance must match Floor validity %v: %v", valid, err)
					}
				})
			}
		}
	}
}

func TestMapGroupLimit(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[6] = MapCell{Circuit: Latch, Group: MaxMapGroups}
	m.Cells[0], m.Cells[24] = MapCell{Feature: Link, Group: MaxMapGroups}, MapCell{Feature: Link, Group: MaxMapGroups}
	m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: MaxMapGroups}}
	if _, err := NewGameWithMap(m, 2); err != nil {
		t.Fatalf("twelfth color rejected: %v", err)
	}
	m.Cells[6].Group = MaxMapGroups + 1
	m.Edges[0].Group = MaxMapGroups + 1
	if _, err := NewGameWithMap(m, 2); err == nil {
		t.Fatal("thirteenth color started a game")
	}
	if err := m.ValidateStructure(); err != nil {
		t.Fatalf("legacy draft must stay editable: %v", err)
	}
}

func TestMapCloneAndStarters(t *testing.T) {
	starters := StarterMaps()
	if len(starters) != 3 {
		t.Fatalf("got %d starters", len(starters))
	}
	for _, m := range starters {
		t.Run(m.Name, func(t *testing.T) {
			if issues := m.Validate(); len(issues) != 0 {
				t.Fatalf("starter is invalid: %v", issues)
			}
			if m.FloorCount() < MaxPlayers {
				t.Fatal("starter cannot support a full room")
			}
			clone := m.Clone()
			clone.Cells[0].Terrain = Wall
			if m.Cells[0].Terrain != Floor {
				t.Fatal("clone aliases source cells")
			}
			if len(clone.Edges) > 0 {
				clone.Edges[0].Group = 99
				if m.Edges[0].Group == 99 {
					t.Fatal("clone aliases source edges")
				}
			}
		})
	}
	starters[0].Cells[0].Terrain = Void
	if StarterMaps()[0].Cells[0].Terrain != Floor {
		t.Fatal("starters share mutable cells")
	}
	for _, size := range []int{MinSize, MaxSize} {
		m := NewMap(size, size)
		if m.FloorCount() != size*size || len(m.Validate()) != 0 {
			t.Fatalf("all-Floor %dx%d is invalid", size, size)
		}
	}
}

func TestLinksCanConnectSeparateRegions(t *testing.T) {
	m := NewMap(5, 5)
	for r := 0; r < 5; r++ {
		m.Cells[2*5+r].Terrain = Wall
	}
	m.Cells[1*5+2] = MapCell{Feature: Link, Group: 1}
	m.Cells[3*5+2] = MapCell{Feature: Link, Group: 1}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("Link should connect both Floor regions: %v", issues)
	}
}

func TestMapEnclosedFloorConnectivity(t *testing.T) {
	for _, terrain := range []Terrain{BreakableWall, Wall, Void} {
		t.Run([]string{"Floor", "Wall", "Void", "Breakable Wall"}[terrain], func(t *testing.T) {
			m := NewMap(9, 9)
			for c := 2; c <= 6; c++ {
				for r := 2; r <= 6; r++ {
					if c == 2 || c == 6 || r == 2 || r == 6 {
						m.Cells[c*m.Rows+r].Terrain = terrain
					}
				}
			}
			m.Cells[4*m.Rows+4] = MapCell{Circuit: Latch, Group: 1}
			m.Edges = []MapEdge{{A: m.Rows + 4, B: m.Rows + 5, Kind: Gate, Group: 1}}
			issues := m.Validate()
			if terrain == BreakableWall {
				if len(issues) != 0 {
					t.Fatalf("breakable enclosure must be playable: %v", issues)
				}
				g, err := NewGameWithMap(m, 2)
				if err != nil {
					t.Fatal(err)
				}
				if g.TerrainAt(2*m.Rows+4) != BreakableWall || g.Geo.Cap[3*m.Rows+3] != 2 {
					t.Fatal("connectivity validation changed the starting terrain or capacity")
				}
			} else if !slices.ContainsFunc(issues, func(issue MapIssue) bool { return strings.Contains(issue.Message, "All Floor must connect") }) {
				t.Fatalf("permanent enclosure must remain invalid: %v", issues)
			}
			if m.Cells[2*m.Rows+4].Terrain != terrain {
				t.Fatal("validation changed the saved terrain")
			}
		})
	}
}

func TestMapGateFloorGroups(t *testing.T) {
	for _, feature := range []CircuitKind{Latch, Toggle, Hold} {
		for _, tt := range []struct {
			name        string
			controllers []int
			groups      []uint8
			link        bool
			valid       bool
			badEdges    []int
		}{
			{name: "switch on first side", controllers: []int{10}, groups: []uint8{1, 1}, valid: true},
			{name: "switch on last side", controllers: []int{45}, groups: []uint8{1, 1}, valid: true},
			{name: "successive circuits", controllers: []int{10, 31}, groups: []uint8{1, 2}, valid: true},
			{name: "circular dependency", controllers: []int{45, 10}, groups: []uint8{1, 2}, badEdges: []int{3, 10}},
			{name: "unrelated switch", controllers: []int{45}, groups: []uint8{1, 0}, badEdges: []int{3}},
			{name: "Link reaches switch", controllers: []int{45}, groups: []uint8{1, 0}, link: true, valid: true},
			{name: "permanent Barriers", groups: []uint8{0, 0}},
		} {
			t.Run(fmt.Sprintf("%d/%s", feature, tt.name), func(t *testing.T) {
				m := NewMap(9, 7)
				for _, c := range []int{2, 5} {
					for r := 0; r < m.Rows; r++ {
						m.Edges = append(m.Edges, MapEdge{A: c*m.Rows + r, B: (c+1)*m.Rows + r, Kind: Barrier})
					}
				}
				for i, group := range tt.groups {
					if group != 0 {
						m.Edges[i*m.Rows+3].Kind, m.Edges[i*m.Rows+3].Group = Gate, group
					}
				}
				for i, cell := range tt.controllers {
					m.Cells[cell] = MapCell{Circuit: feature, Group: uint8(i + 1)}
				}
				if tt.link {
					m.Cells[11], m.Cells[46] = MapCell{Feature: Link, Group: 1}, MapCell{Feature: Link, Group: 1}
				}
				issues := m.Validate()
				if tt.valid {
					if len(issues) != 0 {
						t.Fatalf("reachable circuit must be playable: %v", issues)
					}
					g, err := NewGameWithMap(m, 2)
					if err != nil {
						t.Fatal(err)
					}
					if g.CircuitOn[1] || g.CircuitOn[2] || slices.Contains(g.Geo.Neighbours(17), int16(24)) {
						t.Fatal("connectivity validation opened a starting Gate")
					}
					if _, err := g.Apply(tt.controllers[0]); err != nil {
						t.Fatal(err)
					}
					if !g.CircuitOn[1] || !slices.Contains(g.Geo.Neighbours(17), int16(24)) {
						t.Fatal("pressing the switch did not connect the Gate's Floor regions")
					}
				} else {
					if len(issues) == 0 {
						t.Fatal("permanently separated Floor was accepted")
					}
					var badEdges []int
					for _, issue := range issues {
						if strings.Contains(issue.Message, "Floor group") {
							badEdges = append(badEdges, issue.Edges...)
							if len(issue.Cells) != 1 || !m.Cells[issue.Cells[0]].Circuit.IsSwitch() {
								t.Fatalf("Gate issue must identify its switch: %+v", issue)
							}
						}
					}
					if !slices.Equal(badEdges, tt.badEdges) {
						t.Fatalf("unreachable Gates = %v, want %v; issues: %v", badEdges, tt.badEdges, issues)
					}
				}
				// Reversing edge order must not change whether a Gate chain is reachable.
				slices.Reverse(m.Edges)
				if valid := len(m.Validate()) == 0; valid != tt.valid {
					t.Fatalf("reversed Gate order changed validity to %v", valid)
				}
			})
		}
	}
}
