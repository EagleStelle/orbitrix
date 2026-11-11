package core

import (
	"slices"
	"testing"
)

// springMap is boxMap's 7×7 board with a Wall at (3,0) and a Spring under it at 22, facing down onto 23.
func springMap(boxes ...int) MapDefinition {
	m := boxMap(boxes...)
	m.Cells[21].Terrain, m.Cells[22].Feature = Wall, Spring
	return m
}

func TestSpringRoutes(t *testing.T) {
	m := springMap()
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatal(issues)
	}
	// 15 beside it reaches 23 through it; 23 in front reaches itself; the Spring takes no orbs or players.
	if m.SpringFacing(22) != 3 || m.Capacity(15) != 4 || m.Capacity(23) != 4 || m.Capacity(22) != 0 || m.FloorCount() != 47 {
		t.Fatalf("facing %d, capacities %d %d %d", m.SpringFacing(22), m.Capacity(15), m.Capacity(23), m.Capacity(22))
	}
	g := boxGame(t, m, nil, 15)
	mustApply(t, g, 0)
	if g.Count[23] != 1 || g.Count[22] != 0 || g.Legal(22) {
		t.Fatalf("beside: 23 has %d, Spring %d", g.Count[23], g.Count[22])
	}
	g = boxGame(t, m, nil, 23)
	mustApply(t, g, 0)
	if g.Count[23] != 1 || g.Owner[23] != 0 || g.Count[24] != 1 {
		t.Fatalf("in front: 23 has %d of %d", g.Count[23], g.Owner[23])
	}
}

func TestSpringLaunchesBoxes(t *testing.T) {
	// A Box beside the Spring lands in front of it; one in front bounces off it like a Wall.
	g := boxGame(t, springMap(15), nil, 8)
	mustApply(t, g, 0)
	if g.Boxes[23] != BoxHere || g.Boxes[15] != 0 || g.Count[15] != 1 {
		t.Fatalf("beside: boxes %d %d", g.Boxes[15], g.Boxes[23])
	}
	events := slices.DeleteFunc(slices.Clone(g.TerrainEvents), func(e TerrainEvent) bool { return !e.Box })
	if len(events) != 1 || events[0] != (TerrainEvent{Cell: 15, Box: true, To: 23, Shove: -1, Dir: 1, ShoveDir: -1}) {
		t.Fatalf("events %+v", events)
	}
	g = boxGame(t, springMap(23), nil, 24)
	mustApply(t, g, 0)
	if g.Boxes[24] != BoxHere || g.Count[23] != 1 {
		t.Fatalf("in front: boxes %d %d", g.Boxes[23], g.Boxes[24])
	}
}

func TestSpringPlacement(t *testing.T) {
	for name, edit := range map[string]func(*MapDefinition){
		"no Wall":           func(m *MapDefinition) { m.Cells[21].Terrain = Floor },
		"Walls either side": func(m *MapDefinition) { m.Cells[23].Terrain = Wall },
		"corner":            func(m *MapDefinition) { m.Cells[15].Terrain = Wall },
		"Void in front":     func(m *MapDefinition) { m.Cells[23].Terrain = Void },
		"Breakable Wall":    func(m *MapDefinition) { m.Cells[23].Terrain = BreakableWall },
		"Door in front":     func(m *MapDefinition) { m.Cells[23].Circuit, m.Cells[23].Group = Door, 1 },
		"Barrier in front":  func(m *MapDefinition) { m.Edges = []MapEdge{{A: 22, B: 23, Kind: Barrier}} },
		"Wall and Barrier":  func(m *MapDefinition) { m.Edges = []MapEdge{{A: 22, B: 15, Kind: Barrier}} },
	} {
		m := springMap()
		edit(&m)
		if !slices.ContainsFunc(m.Validate(), func(i MapIssue) bool { return slices.Equal(i.Cells, []int{22}) }) {
			t.Errorf("%s: accepted", name)
		}
	}
	for name, edit := range map[string]func(*MapDefinition){
		"Hole in front":      func(m *MapDefinition) { m.Cells[23].Terrain = Hole },
		"Furnace in front":   func(m *MapDefinition) { m.Cells[23].Feature = Furnace },
		"Reservoir in front": func(m *MapDefinition) { m.Cells[23].Feature = Reservoir },
		"Barrier behind":     func(m *MapDefinition) { m.Cells[21].Terrain, m.Edges = Floor, []MapEdge{{A: 21, B: 22, Kind: Barrier}} },
	} {
		m := springMap()
		edit(&m)
		if issues := m.Validate(); len(issues) != 0 {
			t.Errorf("%s: %v", name, issues)
		}
	}
}

func TestBoxAndSpringSurviveTheCodec(t *testing.T) {
	m := springMap(24)
	encoded, err := EncodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := DecodeMap(encoded); err != nil || !slices.Equal(decoded.Cells, m.Cells) {
		t.Fatalf("decode: %v", err)
	}
}
