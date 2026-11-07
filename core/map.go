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
	Switch
	ToggleSwitch
	HoldSwitch
)

func (f Feature) IsSwitch() bool { return f == Switch || f == ToggleSwitch || f == HoldSwitch }

const MaxMapGroups = MaxPlayers

// MapCell stores design-time terrain and a feature. Group identifies a Link pair
// or a Switch's Gate group; those two kinds use independent group namespaces.
type MapCell struct {
	Terrain Terrain
	Feature Feature
	Group   uint8
}

// EdgeKind describes an obstruction between orthogonally adjacent cells.
type EdgeKind uint8

const (
	Barrier EdgeKind = iota
	Gate
)

type MapEdge struct {
	A, B  int
	Kind  EdgeKind
	Group uint8 // nonzero for a Gate; zero for a Barrier
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
	for _, c := range m.Cells {
		if c.Terrain == Floor {
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
		if c.Terrain > BreakableWall || c.Feature > HoldSwitch {
			return fmt.Errorf("map cell %d has unknown terrain or feature", i)
		}
	}
	for i, e := range m.Edges {
		if e.Kind > Gate || e.A < 0 || e.A >= n || e.B < 0 || e.B >= n || !m.adjacent(e.A, e.B) {
			return fmt.Errorf("map edge %d must join adjacent cells and use a known kind", i)
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

// InitialConnections appends a cell's open neighbors in left/right/up/down/Link
// order. Every Gate starts closed. Incomplete Link pairs are omitted in previews.
func (m MapDefinition) InitialConnections(cell int, dst []int) []int {
	if !m.floor(cell) || m.Rows <= 0 || m.Cols <= 0 {
		return dst
	}
	c, r := cell/m.Rows, cell%m.Rows
	for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		x, y := c+d[0], r+d[1]
		if x < 0 || x >= m.Cols || y < 0 || y >= m.Rows {
			continue
		}
		j := x*m.Rows + y
		if !m.floor(j) {
			continue
		}
		blocked := false
		for _, e := range m.Edges {
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
	if !m.floor(cell) {
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
	var switches, gates [256][]int
	for i, c := range m.Cells {
		if c.Feature != NoFeature && c.Terrain != Floor {
			issues = append(issues, MapIssue{Message: "Features need Floor", Cells: []int{i}})
		}
		switch c.Feature {
		case Link:
			links[c.Group] = append(links[c.Group], i)
		case Switch, ToggleSwitch, HoldSwitch:
			switches[c.Group] = append(switches[c.Group], i)
		default:
			if c.Group != 0 {
				issues = append(issues, MapIssue{Message: "Only Links and Switches use a group", Cells: []int{i}, Group: c.Group})
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
			issues = append(issues, MapIssue{Message: "Barriers and Gates need Floor on both sides", Cells: []int{e.A, e.B}, Edges: []int{i}})
		}
		key := [2]int{min(e.A, e.B), max(e.A, e.B)}
		if prev, found := seenEdges[key]; found {
			issues = append(issues, MapIssue{Message: "Only one Barrier or Gate can occupy an edge", Cells: []int{e.A, e.B}, Edges: []int{prev, i}})
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
		if group > MaxMapGroups && len(cells)+len(switches[group])+len(gates[group]) > 0 {
			issues = append(issues, MapIssue{Message: "Choose one of the 12 group colors", Cells: append(slices.Clone(cells), switches[group]...), Edges: slices.Clone(gates[group]), Group: uint8(group)})
			continue
		}
		if len(cells) > 0 && (group == 0 || len(cells) != 2 || m.adjacent(cells[0], cells[len(cells)-1])) {
			issues = append(issues, MapIssue{Message: "Link group needs exactly two nonadjacent Floor endpoints and a nonzero group", Cells: slices.Clone(cells), Group: uint8(group)})
		}
		if len(switches[group])+len(gates[group]) > 0 && (group == 0 || len(switches[group]) != 1 || len(gates[group]) == 0) {
			issues = append(issues, MapIssue{Message: "A circuit needs exactly one switch and at least one gate", Cells: slices.Clone(switches[group]), Edges: slices.Clone(gates[group]), Group: uint8(group)})
		}
	}
	var neighbors [5]int
	first := -1
	for i, c := range m.Cells {
		if c.Terrain != Floor {
			continue
		}
		if first < 0 {
			first = i
		}
		if len(m.InitialConnections(i, neighbors[:0])) < 2 {
			issues = append(issues, MapIssue{Message: "Floor needs at least two connections with Gates closed", Cells: []int{i}})
		}
	}
	if first < 0 {
		return append(issues, MapIssue{Message: "Map needs playable Floor"})
	}
	visited := make([]bool, len(m.Cells))
	queue := make([]int, 1, len(m.Cells))
	queue[0], visited[first] = first, true
	for head := 0; head < len(queue); head++ {
		for _, n := range m.InitialConnections(queue[head], neighbors[:0]) {
			if !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}
	var disconnected []int
	for i, c := range m.Cells {
		if c.Terrain == Floor && !visited[i] {
			disconnected = append(disconnected, i)
		}
	}
	if len(disconnected) > 0 {
		issues = append(issues, MapIssue{Message: "All Floor must connect with Gates closed", Cells: disconnected})
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
	changing.Cells[2*9+4].Feature, changing.Cells[2*9+4].Group = Switch, 1
	changing.Cells[6*9+4].Feature, changing.Cells[6*9+4].Group = ToggleSwitch, 2
	changing.Edges = []MapEdge{
		{A: 2*9 + 4, B: 2*9 + 5, Kind: Gate, Group: 1},
		{A: 6*9 + 3, B: 6*9 + 4, Kind: Gate, Group: 2},
		{A: 2*9 + 2, B: 3*9 + 2, Kind: Barrier},
		{A: 5*9 + 6, B: 6*9 + 6, Kind: Barrier},
	}
	return []MapDefinition{choke, links, changing}
}
