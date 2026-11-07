package core

import (
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
			m.Cells[6] = MapCell{Feature: Switch, Group: 13}
			m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 13}}
		}, "12 group colors"},
		{"orphan Switch", func(m *MapDefinition) { m.Cells[12] = MapCell{Feature: Switch, Group: 1} }, "circuit"},
		{"orphan Gate", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 12, B: 13, Kind: Gate, Group: 1}} }, "circuit"},
		{"duplicate controller", func(m *MapDefinition) {
			m.Cells[6] = MapCell{Feature: Switch, Group: 1}
			m.Cells[12] = MapCell{Feature: ToggleSwitch, Group: 1}
			m.Edges = []MapEdge{{A: 7, B: 8, Kind: Gate, Group: 1}}
		}, "exactly one switch"},
		{"feature on Wall", func(m *MapDefinition) { m.Cells[12] = MapCell{Terrain: Wall, Feature: Reservoir} }, "Features need Floor"},
		{"unused group", func(m *MapDefinition) { m.Cells[12].Group = 1 }, "Only Links and Switches"},
		{"Barrier group", func(m *MapDefinition) { m.Edges = []MapEdge{{A: 12, B: 13, Group: 1}} }, "Barriers do not use"},
		{"duplicate edge", func(m *MapDefinition) {
			m.Edges = []MapEdge{{A: 12, B: 13}, {A: 13, B: 12}}
		}, "Only one Barrier or Gate"},
		{"edge on Wall", func(m *MapDefinition) {
			m.Cells[12].Terrain = Wall
			m.Edges = []MapEdge{{A: 12, B: 13}}
		}, "need Floor on both sides"},
		{"corner dead end", func(m *MapDefinition) { m.Cells[1].Terrain = Wall }, "at least two connections"},
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

func TestMapGroupLimit(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[6] = MapCell{Feature: Switch, Group: MaxMapGroups}
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
