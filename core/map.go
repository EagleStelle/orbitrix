package core

import (
	"fmt"
	"math"
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
	Hole
	Landfill // absorbs two incoming orbs, then becomes empty Floor
)

// Feature is an optional mechanic attached to a Floor cell.
type Feature uint8

const (
	NoFeature Feature = iota
	Link
	Reservoir // takes one extra orb to explode, throws it along its arrow and makes a new one
	Portal
	Spillway // takes one extra orb to explode and loses it, like one sent into a Hole
	Factory  // gains an orb after each of its owner's moves
	Crusher  // loses an orb after each of its owner's moves
)

// IsMaker reports whether f acts after its holder's moves.
func (f Feature) IsMaker() bool { return f == Factory || f == Crusher }

const MaxMapGroups = MaxPlayers

// Pace sorts a map's estimated game length against an empty board with the same room.
type Pace uint8

const (
	Instant Pace = iota // Factories end games within a few turns
	Fast
	Normal
	Slow
	Endless // many games never end
)

// PaceLimits are the game lengths, in thousandths of an empty board's, where Normal, Slow and Endless begin.
var PaceLimits = [3]int{750, 1500, 2500}

// instantMoves is the estimate under which Factories make a game instant: about five turns each for two players.
const instantMoves = 10

// MapCell stores terrain, a passive feature or a circuit component. Link pairs,
// Portal pairs and circuits have independent group namespaces.
type MapCell struct {
	Terrain       Terrain
	Feature       Feature
	Circuit       CircuitKind
	Group         uint8
	InitiallyOpen bool
	PortalMode    PortalMode
}

type MapEdge struct {
	A, B          int
	Kind          EdgeKind
	Group         uint8 // nonzero for a Gate; zero for either Barrier kind
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
		if c.Terrain > Landfill || c.Feature > Crusher || c.Circuit > Door {
			return fmt.Errorf("map cell %d has unknown terrain or feature", i)
		}
		if c.Feature != NoFeature && c.Circuit != NoCircuit {
			return fmt.Errorf("map cell %d cannot contain both a feature and a circuit component", i)
		}
		if c.InitiallyOpen && !c.Circuit.IsOutput() {
			return fmt.Errorf("map cell %d has an initial state without an output", i)
		}
		if c.PortalMode > PortalVertical || c.PortalMode != PortalBoth && c.Feature != Portal {
			return fmt.Errorf("map cell %d has invalid Portal directions", i)
		}
	}
	for i, e := range m.Edges {
		if e.Kind > BreakableBarrier || e.A < 0 || e.A >= n || e.B < 0 || e.B >= n || !m.adjacent(e.A, e.B) {
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

// EdgeFits reports whether a Barrier or Gate may join adjacent cells a and b: Floor on both sides, or Floor
// beside a Hole. A Landfill can become Floor on either side.
func (m MapDefinition) EdgeFits(a, b int) bool {
	hole := func(i int) bool { return i >= 0 && i < len(m.Cells) && m.Cells[i].Terrain == Hole }
	floor := func(i int) bool { return m.floor(i) || i >= 0 && i < len(m.Cells) && m.Cells[i].Terrain == Landfill }
	return floor(a) && (floor(b) || hole(b)) || hole(a) && floor(b)
}

func (m MapDefinition) initialFloor(i int) bool {
	return m.floor(i) && m.Cells[i].Feature != Portal && (m.Cells[i].Circuit != Door || m.Cells[i].InitiallyOpen)
}

func (m MapDefinition) neighbor(cell, direction int) int {
	d := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}[direction]
	x, y := cell/m.Rows+d[0], cell%m.Rows+d[1]
	if x < 0 || x >= m.Cols || y < 0 || y >= m.Rows {
		return -1
	}
	return x*m.Rows + y
}

func (m MapDefinition) partner(cell int) int {
	c := m.Cells[cell]
	if c.Group == 0 || c.Terrain != Floor {
		return -1
	}
	pair, count := -1, 0
	for i, other := range m.Cells {
		if other.Terrain == Floor && other.Feature == c.Feature && other.Group == c.Group {
			count++
			if i != cell {
				pair = i
			}
		}
	}
	if count != 2 || pair < 0 || m.adjacent(cell, pair) {
		return -1
	}
	return pair
}

func (m MapDefinition) routeBlocked(cell, direction int, blocked []uint8) bool {
	if blocked != nil {
		return blocked[4*cell+direction] != 0
	}
	next := m.neighbor(cell, direction)
	for _, e := range m.Edges {
		if e.Kind == Gate && e.InitiallyOpen {
			continue
		}
		if e.A == cell && e.B == next || e.B == cell && e.A == next {
			return true
		}
	}
	return false
}

// route follows a cardinal move through Portal pairs without changing direction.
// A nonnil path collects entry/exit pairs for presentation; nil avoids draft allocations.
func (m MapDefinition) route(cell, direction int, pairs []int16, blocked []uint8, axes []PortalAxes, path []int16) (int, []int16) {
	for hops := 0; hops <= len(m.Cells); hops++ {
		next := m.neighbor(cell, direction)
		if next < 0 || m.routeBlocked(cell, direction, blocked) {
			return -1, path
		}
		if m.Cells[next].Feature != Portal {
			return next, path
		}
		if axes == nil {
			axes = m.InitialPortalAxes()
		}
		if axes[next]&PortalAxes(1<<(direction/2)) == 0 {
			return -1, path
		}
		pair := -1
		if pairs != nil {
			pair = int(pairs[next])
		} else {
			pair = m.partner(next)
		}
		if pair < 0 {
			return -1, path
		}
		if path != nil {
			path = append(path, int16(next), int16(pair))
		}
		cell = pair
	}
	return -1, path // A Portal-only loop has no landing cell.
}

// InitialOutlets returns a cell's open Floor routes at the start, bit d for direction d (left, right, up, down), as
// Geometry.Outlets holds them for a Reservoir.
func (m MapDefinition) InitialOutlets(cell int) (mask uint8) {
	for d := range 4 {
		if j, _ := m.route(cell, d, nil, nil, nil, nil); m.initialFloor(j) {
			mask |= 1 << d
		}
	}
	return mask
}

// InitialConnections appends a cell's open neighbors in left/right/up/down/Link
// order, respecting output starting states and routing through Portals.
// Incomplete pairs are omitted.
func (m MapDefinition) InitialConnections(cell int, dst []int) []int {
	if !m.initialFloor(cell) || m.Rows <= 0 || m.Cols <= 0 {
		return dst
	}
	for d := range 4 {
		j, _ := m.route(cell, d, nil, nil, nil, nil)
		if m.initialFloor(j) {
			dst = append(dst, j)
		}
	}
	cfg := m.Cells[cell]
	if cfg.Feature != Link || cfg.Group == 0 {
		return dst
	}
	if pair := m.partner(cell); pair >= 0 {
		dst = append(dst, pair)
	}
	return dst
}

// Capacity is the initial explosion threshold, including extra orbs and routes into Holes or Landfills.
func (m MapDefinition) Capacity(cell int) int {
	n, _ := m.capacity(cell)
	return n
}

// capacity also returns how many routes permanently lose an orb to a Hole.
func (m MapDefinition) capacity(cell int) (n, holes int) {
	if !m.initialFloor(cell) {
		return 0, 0
	}
	var neighbors [5]int
	n = len(m.InitialConnections(cell, neighbors[:0]))
	for d := range 4 {
		if j, _ := m.route(cell, d, nil, nil, nil, nil); j >= 0 {
			switch m.Cells[j].Terrain {
			case Hole:
				holes++
			case Landfill:
				n++
			}
		}
	}
	if f := m.Cells[cell].Feature; f == Reservoir || f == Spillway {
		n++
	}
	return n + holes, holes
}

// Validate returns all actionable gameplay issues. Structurally sound drafts
// may be saved and shared even when they contain these issues.
func (m MapDefinition) Validate() []MapIssue {
	if err := m.ValidateStructure(); err != nil {
		return []MapIssue{{Message: err.Error()}}
	}
	var issues []MapIssue
	var links [256][]int
	var portals [256][]int
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
		case c.Feature == Portal:
			portals[c.Group] = append(portals[c.Group], i)
		case c.Circuit.IsSwitch():
			switches[c.Group] = append(switches[c.Group], i)
		case c.Circuit.IsOutput():
			doors[c.Group] = append(doors[c.Group], i)
		default:
			if c.Group != 0 {
				issues = append(issues, MapIssue{Message: "Only Links, Portals and circuit components use a group", Cells: []int{i}, Group: c.Group})
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
		if !m.EdgeFits(e.A, e.B) {
			issues = append(issues, MapIssue{Message: "Barriers and Gates need Floor or Landfill on both sides, or beside a Hole", Edges: []int{i}})
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
	// A Hole sealed by Barriers on every Floor side can never swallow an orb; Gates may open.
	for i, c := range m.Cells {
		if c.Terrain != Hole {
			continue
		}
		sides := 0
		var sealed []int
		for d := range 4 {
			j := m.neighbor(i, d)
			if j < 0 || m.Cells[j].Terrain != Floor && m.Cells[j].Terrain != BreakableWall && m.Cells[j].Terrain != Landfill {
				continue
			}
			sides++
			if k, found := seenEdges[[2]int{min(i, j), max(i, j)}]; found && m.Edges[k].Kind != Gate {
				sealed = append(sealed, k)
			}
		}
		if sides > 0 && len(sealed) == sides {
			issues = append(issues, MapIssue{Message: "A Hole needs a Floor side without a Barrier", Cells: []int{i}, Edges: sealed})
		}
	}
	for group := 0; group < len(links); group++ {
		cells := links[group]
		portalCells := portals[group]
		circuitCells := append(slices.Clone(switches[group]), doors[group]...)
		if group > MaxMapGroups && len(cells)+len(portalCells)+len(circuitCells)+len(gates[group]) > 0 {
			issues = append(issues, MapIssue{Message: "Choose one of the 12 group colors", Cells: append(append(slices.Clone(cells), portalCells...), circuitCells...), Edges: slices.Clone(gates[group]), Group: uint8(group)})
			continue
		}
		if len(cells) > 0 && (group == 0 || len(cells) != 2 || m.adjacent(cells[0], cells[len(cells)-1])) {
			issues = append(issues, MapIssue{Message: "Link group needs exactly two nonadjacent Floor endpoints and a nonzero group", Cells: slices.Clone(cells), Group: uint8(group)})
		}
		if len(portalCells) > 0 && (group == 0 || len(portalCells) != 2 || m.adjacent(portalCells[0], portalCells[len(portalCells)-1])) {
			issues = append(issues, MapIssue{Message: "Portal group needs exactly two nonadjacent Floor endpoints and a nonzero group", Cells: slices.Clone(portalCells), Group: uint8(group)})
		}
		if len(circuitCells)+len(gates[group]) > 0 && (group == 0 || len(switches[group]) == 0 || len(gates[group])+len(doors[group]) == 0) {
			issues = append(issues, MapIssue{Message: "A circuit needs at least one switch and one Gate or Door", Cells: circuitCells, Edges: slices.Clone(gates[group]), Group: uint8(group)})
		}
	}
	portalAxes := m.InitialPortalAxes()
	for entry, c := range m.Cells {
		if c.Feature != Portal {
			continue
		}
		pair := m.partner(entry)
		if pair < 0 {
			continue
		}
		if portalAxes[entry] == 0 {
			issues = append(issues, MapIssue{Message: "Portal needs an open horizontal or vertical route", Cells: []int{entry, pair}, Group: c.Group})
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
		switch capacity, f := m.Capacity(i), m.Cells[i].Feature; {
		case capacity < 2:
			issues = append(issues, MapIssue{Message: "Floor needs initial capacity of at least two", Cells: []int{i}})
		case capacity < 3 && (f == Factory || f == Crusher):
			issues = append(issues, MapIssue{Message: "Factories and Crushers need initial capacity of at least three", Cells: []int{i}})
		}
	}
	if first < 0 {
		return append(issues, MapIssue{Message: "Map needs playable Floor"})
	}
	// Destructible terrain can join regions later without changing starting capacities.
	future := m.Clone()
	for i, c := range future.Cells {
		if c.Terrain == BreakableWall || c.Terrain == Landfill {
			future.Cells[i].Terrain = Floor
		}
	}
	future.Edges = slices.DeleteFunc(future.Edges, func(e MapEdge) bool { return e.Kind == BreakableBarrier })
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
		if (c.Terrain == Floor || c.Terrain == Landfill) && c.Feature != Portal && regions[i] != regions[first] {
			disconnected = append(disconnected, i)
		}
	}
	if len(disconnected) > 0 {
		issues = append(issues, MapIssue{Message: "All Floor must connect through Landfills, breakable walls, breakable barriers or reachable Gates", Cells: disconnected})
	}
	if _, _, pace := m.Estimate(); pace == Instant || pace == Endless {
		issue := MapIssue{Message: "Too many Holes, Spillways and Crushers: games would never end"}
		if pace == Instant {
			issue.Message = "Too many Factories and Reservoirs: games would end almost at once"
		}
		for i, c := range m.Cells {
			drains := c.Terrain == Hole || c.Feature == Spillway || c.Feature == Crusher
			if pace == Instant && (c.Feature == Factory || c.Feature == Reservoir) || pace == Endless && drains {
				issue.Cells = append(issue.Cells, i)
			}
		}
		issues = append(issues, issue)
	}
	return issues
}

// Estimate predicts a game's average moves on the starting layout (0 when Endless), its length in thousandths of an
// empty board's, and its pace. Bot games of 2 to 4 mixed seats fit it on 7×7 to 40×40 boards: an empty board lasts
// about 0.75 moves per orb of room (what its Floor holds without exploding), and fills at the orb each move adds.
// Each open Hole side, Spillway and Crusher loses part of it, the same on every board size. Factories add to it, each
// a little less as they crowd, and less still beside a Hole, which they feed. More players make Factory maps longer,
// since each Factory works only on its holder's turn. Reservoirs add to it too, less, as they make an orb only when
// they explode. Integers keep validation identical on every device.
func (m MapDefinition) Estimate() (moves, length int, pace Pace) {
	room, growth, factories, reservoirs := 0, 1000, 0, 0 // growth in thousandths of an empty board's
	for i := range m.Cells {
		if m.Cells[i].Terrain == Landfill {
			room += 2 // a finite fill cost, not a permanent drain on growth
		}
		capacity, holes := m.capacity(i)
		if capacity == 0 {
			continue
		}
		room += capacity - 1
		growth -= 45 * holes
		switch m.Cells[i].Feature {
		case Spillway:
			growth -= 35
		case Crusher:
			growth -= 80
		case Factory:
			factories++
			growth -= 30 * holes
		case Reservoir:
			if m.InitialOutlets(i) != 0 {
				reservoirs++
			}
		}
	}
	growth += 150 * 83 * factories / (83 + factories)
	growth += 7600 * reservoirs / (300 + reservoirs)
	length = math.MaxInt32
	if growth > 0 {
		length = 1_000_000 / growth
		moves = room * 750 / growth
	}
	switch {
	case length >= PaceLimits[2]:
		return 0, length, Endless
	case length >= PaceLimits[1]:
		pace = Slow
	case moves < instantMoves && length < 1000:
		pace = Instant
	case length >= PaceLimits[0]:
		pace = Normal
	default:
		pace = Fast
	}
	return moves, length, pace
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
