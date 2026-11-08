package core

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// Terrain is a cell's base surface. Only Floor can hold orbs.
type Terrain uint8

const (
	Floor Terrain = iota
	Wall
	Void
	BreakableWall
)

// Feature is an optional mechanic attached to a Floor cell.
type Feature uint8

const (
	NoFeature Feature = iota
	Link
	Reservoir
)

const MaxMapGroups = MaxPlayers

// MapCell stores terrain, a passive feature or a circuit component. Link pair
// groups and circuit groups have independent namespaces.
type MapCell struct {
	Terrain       Terrain
	Feature       Feature
	Circuit       CircuitKind
	Group         uint8
	InitiallyOpen bool
}

type MapEdge struct {
	A, B          int
	Kind          EdgeKind
	Group         uint8 // nonzero for a Gate; zero for a Barrier
	InitiallyOpen bool
}

// MapDefinition is a saved board design, without any active match state. Cells
// use the same column-major index c*Rows+r as Geometry. Games copy definitions
// when accepting them, so editing a draft cannot change an ongoing match.
type MapDefinition struct {
	Name       string
	Cols, Rows int
	Cells      []MapCell
	Edges      []MapEdge
}

// MapIssue identifies a playable-map error and the elements an editor can mark.
type MapIssue struct {
	Message string
	Cells   []int
	Edges   []int
	Group   uint8
}

// NewMap makes an all-Floor draft. Invalid dimensions are kept for validation
// without allocating an unbounded cell buffer.
func NewMap(cols, rows int) MapDefinition {
	m := MapDefinition{Name: "Untitled map", Cols: cols, Rows: rows}
	if cols >= MinSize && cols <= MaxSize && rows >= MinSize && rows <= MaxSize {
		m.Cells = make([]MapCell, cols*rows)
	}
	return m
}

func (m MapDefinition) Clone() MapDefinition {
	m.Cells = slices.Clone(m.Cells)
	m.Edges = slices.Clone(m.Edges)
	return m
}

func (m MapDefinition) FloorCount() int {
	n := 0
	for i := range m.Cells {
		if m.initialFloor(i) {
			n++
		}
	}
	return n
}

// ValidateStructure accepts unfinished designs but rejects malformed or
// unbounded data at storage, sharing and network trust boundaries.
func (m MapDefinition) ValidateStructure() error {
	if m.Cols < MinSize || m.Cols > MaxSize || m.Rows < MinSize || m.Rows > MaxSize {
		return fmt.Errorf("map dimensions must be %d..%d", MinSize, MaxSize)
	}
	if !utf8.ValidString(m.Name) || len(m.Name) > 64 {
		return fmt.Errorf("map name must be valid UTF-8 and at most 64 bytes")
	}
	n := m.Cols * m.Rows
	if len(m.Cells) != n {
		return fmt.Errorf("map has %d cells; expected %d", len(m.Cells), n)
	}
	if len(m.Edges) > 2*n-m.Cols-m.Rows {
		return fmt.Errorf("map has too many edges")
	}
	for i, c := range m.Cells {
		if c.Terrain > BreakableWall || c.Feature > Reservoir || c.Circuit > Door {
			return fmt.Errorf("map cell %d has unknown terrain or feature", i)
		}
		if c.Feature != NoFeature && c.Circuit != NoCircuit {
			return fmt.Errorf("map cell %d cannot contain both a feature and a circuit component", i)
		}
		if c.InitiallyOpen && !c.Circuit.IsOutput() {
			return fmt.Errorf("map cell %d has an initial state without an output", i)
		}
	}
	for i, e := range m.Edges {
		if e.Kind > Gate || e.A < 0 || e.A >= n || e.B < 0 || e.B >= n || !m.adjacent(e.A, e.B) {
			return fmt.Errorf("map edge %d must join adjacent cells and use a known kind", i)
		}
		if e.InitiallyOpen && e.Kind != Gate {
			return fmt.Errorf("map edge %d has an initial state without an output", i)
		}
	}
	return nil
}

func (m MapDefinition) adjacent(a, b int) bool {
	dc, dr := a/m.Rows-b/m.Rows, a%m.Rows-b%m.Rows
	return max(dc, -dc)+max(dr, -dr) == 1
}

func (m MapDefinition) floor(i int) bool {
	return i >= 0 && i < len(m.Cells) && m.Cells[i].Terrain == Floor
}

func (m MapDefinition) initialFloor(i int) bool {
	return m.floor(i) && (m.Cells[i].Circuit != Door || m.Cells[i].InitiallyOpen)
}

// InitialConnections appends a cell's open neighbors in left/right/up/down/Link
// order, respecting output starting states. Incomplete Link pairs are omitted.
func (m MapDefinition) InitialConnections(cell int, dst []int) []int {
	if !m.initialFloor(cell) || m.Rows <= 0 || m.Cols <= 0 {
		return dst
	}
	c, r := cell/m.Rows, cell%m.Rows
	for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		x, y := c+d[0], r+d[1]
		if x < 0 || x >= m.Cols || y < 0 || y >= m.Rows {
			continue
		}
		j := x*m.Rows + y
		if !m.initialFloor(j) {
			continue
		}
		blocked := false
		for _, e := range m.Edges {
			if e.Kind == Gate && e.InitiallyOpen {
				continue
			}
			if e.A == cell && e.B == j || e.B == cell && e.A == j {
				blocked = true
				break
			}
		}
		if !blocked {
			dst = append(dst, j)
		}
	}
	cfg := m.Cells[cell]
	if cfg.Feature != Link || cfg.Group == 0 {
		return dst
	}
	pair, count := -1, 0
	for i, other := range m.Cells {
		if other.Terrain == Floor && other.Feature == Link && other.Group == cfg.Group {
			count++
			if i != cell {
				pair = i
			}
		}
	}
	if count == 2 && pair >= 0 && !m.adjacent(cell, pair) {
		dst = append(dst, pair)
	}
	return dst
}

// Capacity is the initial explosion threshold, including a Reservoir's reserve.
func (m MapDefinition) Capacity(cell int) int {
	if !m.initialFloor(cell) {
		return 0
	}
	var neighbors [5]int
	n := len(m.InitialConnections(cell, neighbors[:0]))
	if m.Cells[cell].Feature == Reservoir {
		n++
	}
	return n
}

// Validate returns all actionable gameplay issues. Structurally sound drafts
// may be saved and shared even when they contain these issues.
func (m MapDefinition) Validate() []MapIssue {
	if err := m.ValidateStructure(); err != nil {
		return []MapIssue{{Message: err.Error()}}
	}
	var issues []MapIssue
	var links [256][]int
	var switches, gates, doors [256][]int
	for i, c := range m.Cells {
		if c.Feature != NoFeature && c.Terrain != Floor {
			issues = append(issues, MapIssue{Message: "Features need Floor", Cells: []int{i}})
		}
		if c.Circuit != NoCircuit && (c.Terrain != Floor || c.Feature != NoFeature) {
			issues = append(issues, MapIssue{Message: "Circuit components need empty Floor", Cells: []int{i}})
		}
		switch {
		case c.Feature == Link:
			links[c.Group] = append(links[c.Group], i)
		case c.Circuit.IsSwitch():
			switches[c.Group] = append(switches[c.Group], i)
		case c.Circuit.IsOutput():
			doors[c.Group] = append(doors[c.Group], i)
		default:
			if c.Group != 0 {
				issues = append(issues, MapIssue{Message: "Only Links and circuit components use a group", Cells: []int{i}, Group: c.Group})
			}
		}
		if c.Terrain == BreakableWall {
			neighbors := 0
			x, y := i/m.Rows, i%m.Rows
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && nx < m.Cols && ny >= 0 && ny < m.Rows && m.floor(nx*m.Rows+ny) {
					neighbors++
				}
			}
			if neighbors < 2 {
				issues = append(issues, MapIssue{Message: "Breakable Wall needs at least two neighboring Floor cells", Cells: []int{i}})
			}
		}
	}
	seenEdges := make(map[[2]int]int, len(m.Edges))
	for i, e := range m.Edges {
		if !m.floor(e.A) || !m.floor(e.B) {
			issues = append(issues, MapIssue{Message: "Barriers and Gates need Floor on both sides", Edges: []int{i}})
		}
		key := [2]int{min(e.A, e.B), max(e.A, e.B)}
		if prev, found := seenEdges[key]; found {
			issues = append(issues, MapIssue{Message: "Only one Barrier or Gate can occupy an edge", Edges: []int{prev, i}})
		}
		seenEdges[key] = i
		if e.Kind == Gate {
			gates[e.Group] = append(gates[e.Group], i)
		} else if e.Group != 0 {
			issues = append(issues, MapIssue{Message: "Barriers do not use a gate group", Edges: []int{i}, Group: e.Group})
		}
	}
	for group := 0; group < len(links); group++ {
		cells := links[group]
		circuitCells := append(slices.Clone(switches[group]), doors[group]...)
		if group > MaxMapGroups && len(cells)+len(circuitCells)+len(gates[group]) > 0 {
			issues = append(issues, MapIssue{Message: "Choose one of the 12 group colors", Cells: append(slices.Clone(cells), circuitCells...), Edges: slices.Clone(gates[group]), Group: uint8(group)})
			continue
		}
		if len(cells) > 0 && (group == 0 || len(cells) != 2 || m.adjacent(cells[0], cells[len(cells)-1])) {
			issues = append(issues, MapIssue{Message: "Link group needs exactly two nonadjacent Floor endpoints and a nonzero group", Cells: slices.Clone(cells), Group: uint8(group)})
		}
		if len(circuitCells)+len(gates[group]) > 0 && (group == 0 || len(switches[group]) == 0 || len(gates[group])+len(doors[group]) == 0) {
			issues = append(issues, MapIssue{Message: "A circuit needs at least one switch and one Gate or Door", Cells: circuitCells, Edges: slices.Clone(gates[group]), Group: uint8(group)})
		}
	}
	var neighbors [5]int
	first := -1
	for i := range m.Cells {
		if !m.initialFloor(i) {
			continue
		}
		if first < 0 {
			first = i
		}
		if m.Capacity(i) < 2 {
			issues = append(issues, MapIssue{Message: "Floor needs initial capacity of at least two", Cells: []int{i}})
		}
	}
	if first < 0 {
		return append(issues, MapIssue{Message: "Map needs playable Floor"})
	}
	// Destructible terrain can join regions later without changing starting capacities.
	future := m.Clone()
	for i, c := range future.Cells {
		if c.Terrain == BreakableWall {
			future.Cells[i].Terrain = Floor
		}
	}
	regions := make([]int, len(m.Cells))
	queue := make([]int, 0, len(m.Cells))
	for start := range future.Cells {
		if !future.initialFloor(start) || regions[start] != 0 {
			continue
		}
		queue = append(queue[:0], start)
		regions[start] = start + 1
		for head := 0; head < len(queue); head++ {
			for _, n := range future.InitialConnections(queue[head], neighbors[:0]) {
				if regions[n] == 0 {
					regions[n] = start + 1
					queue = append(queue, n)
				}
			}
		}
	}
	// Merge reachable Gate regions until chains of dependent circuits stop opening.
	merge := func(a, b int) {
		for i, region := range regions {
			if region == b {
				regions[i] = a
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, e := range m.Edges {
			if e.Kind != Gate || e.Group == 0 || e.Group > MaxMapGroups || len(switches[e.Group]) == 0 {
				continue
			}
			a, b := regions[e.A], regions[e.B]
			accessible := false
			for _, controller := range switches[e.Group] {
				accessible = accessible || regions[controller] != 0 && (regions[controller] == a || regions[controller] == b)
			}
			if a == 0 || b == 0 || a == b || !accessible {
				continue
			}
			merge(a, b)
			changed = true
		}
		for i, c := range future.Cells {
			if c.Circuit != Door || regions[i] != 0 {
				continue
			}
			future.Cells[i].InitiallyOpen = true
			ns := future.InitialConnections(i, neighbors[:0])
			region := 0
			for _, n := range ns {
				for _, controller := range switches[c.Group] {
					if regions[n] != 0 && regions[n] == regions[controller] {
						region = regions[n]
					}
				}
			}
			if region == 0 {
				future.Cells[i].InitiallyOpen = false
				continue
			}
			regions[i] = region
			for _, n := range ns {
				if other := regions[n]; other != 0 && other != region {
					merge(region, other)
				}
			}
			changed = true
		}
	}
	for i, e := range m.Edges {
		if e.Kind != Gate || e.Group == 0 || e.Group > MaxMapGroups || len(switches[e.Group]) == 0 || !m.floor(e.A) || !m.floor(e.B) {
			continue
		}
		accessible := e.InitiallyOpen
		for _, controller := range switches[e.Group] {
			accessible = accessible || regions[controller] != 0 && (regions[controller] == regions[e.A] || regions[controller] == regions[e.B])
		}
		if !accessible {
			issues = append(issues, MapIssue{Message: "Gate must share a Floor group with a switch", Cells: slices.Clone(switches[e.Group]), Edges: []int{i}, Group: e.Group})
		}
	}
	var disconnected []int
	for i, c := range m.Cells {
		if c.Terrain == Floor && regions[i] != regions[first] {
			disconnected = append(disconnected, i)
		}
	}
	if len(disconnected) > 0 {
		issues = append(issues, MapIssue{Message: "All Floor must connect through breakable walls or reachable Gates", Cells: disconnected})
	}
	return issues
}

// StarterMaps demonstrates the editor's mechanics without requiring a download.
// Definitions are freshly allocated so editing a copy cannot alter a preset.
func StarterMaps() []MapDefinition {
	choke := NewMap(9, 9)
	choke.Name = "Crossroads"
	for c := 2; c <= 6; c++ {
		for r := 2; r <= 6; r++ {
			if c == 4 || r == 4 {
				continue
			}
			choke.Cells[c*9+r].Terrain = Wall
		}
	}
	// The outer ring remains two cells wide; the cross creates central choke points.
	links := NewMap(9, 9)
	links.Name = "Long Distance"
	for _, pair := range [][3]int{{2*9 + 2, 6*9 + 6, 1}, {2*9 + 6, 6*9 + 2, 2}} {
		for _, i := range pair[:2] {
			links.Cells[i].Feature, links.Cells[i].Group = Link, uint8(pair[2])
		}
	}
	for _, i := range []int{3*9 + 4, 5*9 + 4, 4*9 + 3, 4*9 + 5} {
		links.Cells[i].Feature = Reservoir
	}
	changing := NewMap(9, 9)
	changing.Name = "Pressure Points"
	for _, i := range []int{3*9 + 3, 5*9 + 5, 3*9 + 5, 5*9 + 3} {
		changing.Cells[i].Terrain = BreakableWall
	}
	changing.Cells[2*9+4].Circuit, changing.Cells[2*9+4].Group = Latch, 1
	changing.Cells[6*9+4].Circuit, changing.Cells[6*9+4].Group = Toggle, 2
	changing.Edges = []MapEdge{
		{A: 2*9 + 4, B: 2*9 + 5, Kind: Gate, Group: 1},
		{A: 6*9 + 3, B: 6*9 + 4, Kind: Gate, Group: 2},
		{A: 2*9 + 2, B: 3*9 + 2, Kind: Barrier},
		{A: 5*9 + 6, B: 6*9 + 6, Kind: Barrier},
	}
	return []MapDefinition{choke, links, changing}
}
