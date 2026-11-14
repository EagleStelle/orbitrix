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
	Landfill       // absorbs its FillTarget in incoming orbs, then becomes empty Floor
	BreakableFloor // Floor until its second explosion breaks it into Void, which takes its orbs
	Sinkhole       // Floor that never explodes: filled to capacity, it collapses into a Hole that takes its orbs
)

// Feature is an optional mechanic attached to a Floor cell.
type Feature uint8

const (
	NoFeature Feature = iota
	Link
	Reservoir // takes one extra orb to explode, throws it along its arrow and makes a new one
	Portal
	Spillway // takes one extra orb to explode and loses it, like one sent into a Hole
	Furnace  // gains an orb after each of its owner's moves
	Crusher  // loses an orb after each of its owner's moves
	Box      // marks where a crate starts; explosions push it, and the first Hole or Landfill it enters becomes Floor
	Spring   // takes no orbs: what arrives leaves through the side facing away from its one Wall
	Vault    // raises a two-hit shield after its owner's moves, which destroys other players' orbs arriving
	Turret   // loads two shots after its owner's moves; each makes an orb it explodes into another player's cell land as two
)

// IsMaker reports whether f acts after its holder's moves.
func (f Feature) IsMaker() bool { return f == Furnace || f == Crusher || f == Vault || f == Turret }

const MaxMapGroups = MaxPlayers

// Pace sorts a map's estimated game length against an empty board with the same room.
type Pace uint8

const (
	Instant Pace = iota // Furnaces end games within a few turns
	Fast
	Normal
	Slow
	Endless // many games never end
)

// PaceLimits are the game lengths, in thousandths of an empty board's, where Normal, Slow and Endless begin.
var PaceLimits = [3]int{750, 1500, 2500}

// instantMoves is the estimate under which Furnaces make a game instant: about five turns each for two players.
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
		if m.initialFloor(i) && m.Cells[i].Feature != Box {
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
		if c.Terrain > Sinkhole || c.Feature > Turret || c.Circuit > Hatch {
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

// floor includes Breakable Floor and Sinkhole, which play as Floor until they break or collapse.
func (m MapDefinition) floor(i int) bool {
	if i < 0 || i >= len(m.Cells) {
		return false
	}
	t := m.Cells[i].Terrain
	return t == Floor || t == BreakableFloor || t == Sinkhole
}

// EdgeFits reports whether a Barrier or Gate may join adjacent cells a and b: Floor on both sides, or Floor
// beside a Hole. A Landfill can become Floor on either side.
func (m MapDefinition) EdgeFits(a, b int) bool {
	hole := func(i int) bool { return i >= 0 && i < len(m.Cells) && m.Cells[i].Terrain == Hole }
	floor := func(i int) bool { return m.floor(i) || i >= 0 && i < len(m.Cells) && m.Cells[i].Terrain == Landfill }
	return floor(a) && (floor(b) || hole(b)) || hole(a) && floor(b)
}

// initialFloor includes a Box's cell: neighbors route into it, and it is Floor once the Box moves on.
func (m MapDefinition) initialFloor(i int) bool {
	return m.floor(i) && m.Cells[i].Feature != Portal && m.Cells[i].Feature != Spring && !m.shut(i)
}

// shut reports whether cell i holds a circuit output that starts as something other than Floor: a closed Door, a Wall,
// or an open Hatch, a Hole.
func (m MapDefinition) shut(i int) bool {
	c := m.Cells[i]
	return c.Circuit == Door && !c.InitiallyOpen || c.Circuit == Hatch && c.InitiallyOpen
}

// hole reports whether cell i starts as a Hole: one, or an open Hatch.
func (m MapDefinition) hole(i int) bool {
	return i >= 0 && (m.Cells[i].Terrain == Hole || m.Cells[i].Circuit == Hatch && m.Cells[i].InitiallyOpen)
}

// SpringFacing is the direction (left, right, up, down) a Spring launches toward, away from the one orthogonal Wall
// or permanent Barrier behind it; -1 without exactly one.
func (m MapDefinition) SpringFacing(cell int) int {
	facing := -1
	for d := range 4 {
		j := m.neighbor(cell, d)
		if j >= 0 && (m.Cells[j].Terrain == Wall || slices.ContainsFunc(m.Edges, func(e MapEdge) bool {
			return e.Kind == Barrier && (e.A == cell && e.B == j || e.B == cell && e.A == j)
		})) {
			if facing >= 0 {
				return -1
			}
			facing = d ^ 1
		}
	}
	return facing
}

// springFits reports whether a Spring has one Wall or Barrier behind it and, in front with no edge between, a cell that takes
// orbs or passes them on: Floor without a Spring or Door, a Hole or a Landfill.
func (m MapDefinition) springFits(cell int) bool {
	facing := m.SpringFacing(cell)
	front := -1
	if facing >= 0 {
		front = m.neighbor(cell, facing)
	}
	if front < 0 {
		return false
	}
	f := m.Cells[front]
	if !m.floor(front) && f.Terrain != Hole && f.Terrain != Landfill || f.Feature == Spring || f.Circuit == Door {
		return false
	}
	return !slices.ContainsFunc(m.Edges, func(e MapEdge) bool { return e.A == cell && e.B == front || e.B == cell && e.A == front })
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

// route follows a cardinal move through Portal pairs without changing direction, and through Springs, which turn
// it to face away from their Wall. A nonnil path collects entry/exit pairs for presentation (a Spring is its own
// pair); nil avoids draft allocations.
func (m MapDefinition) route(cell, direction int, pairs []int16, blocked []uint8, axes []PortalAxes, path []int16) (int, []int16) {
	j, _, path := m.travel(cell, direction, pairs, blocked, axes, path)
	return j, path
}

// travel is route that also returns the direction the move arrives in.
func (m MapDefinition) travel(cell, direction int, pairs []int16, blocked []uint8, axes []PortalAxes, path []int16) (int, int, []int16) {
	for hops := 0; hops <= len(m.Cells); hops++ {
		next := m.neighbor(cell, direction)
		if next < 0 || m.routeBlocked(cell, direction, blocked) {
			return -1, direction, path
		}
		if m.Cells[next].Feature == Spring {
			if direction = m.SpringFacing(next); direction < 0 {
				return -1, direction, path
			}
			if path != nil {
				path = append(path, int16(next), int16(next))
			}
			cell = next
			continue
		}
		if m.Cells[next].Feature != Portal {
			return next, direction, path
		}
		if axes == nil {
			axes = m.InitialPortalAxes()
		}
		if axes[next]&PortalAxes(1<<(direction/2)) == 0 {
			return -1, direction, path
		}
		pair := -1
		if pairs != nil {
			pair = int(pairs[next])
		} else {
			pair = m.partner(next)
		}
		if pair < 0 {
			return -1, direction, path
		}
		if path != nil {
			path = append(path, int16(next), int16(pair))
		}
		cell = pair
	}
	return -1, direction, path // A Portal-only loop has no landing cell.
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
			switch {
			case m.hole(j):
				holes++
			case m.Cells[j].Terrain == Landfill:
				n++
			}
		}
	}
	if f := m.Cells[cell].Feature; f == Reservoir || f == Spillway {
		n++
	}
	return n + holes, holes
}

// FillTarget is how many orbs Landfill cell absorbs before it becomes Floor: the capacity it would have as Floor in
// the starting layout, its routes into Floor, Holes and other Landfills.
func (m MapDefinition) FillTarget(cell int) int {
	n := 0
	for d := range 4 {
		if j, _ := m.route(cell, d, nil, nil, nil, nil); m.initialFloor(j) || m.hole(j) || j >= 0 && m.Cells[j].Terrain == Landfill {
			n++
		}
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
		if c.Feature == Spring && c.Terrain == Floor && !m.springFits(i) {
			issues = append(issues, MapIssue{Message: "A Spring needs exactly one Wall or Barrier beside it and no solid objects in front", Cells: []int{i}})
		}
		if c.Terrain == Landfill && m.FillTarget(i) < 2 { // the Floor it becomes would explode on its first orb
			issues = append(issues, MapIssue{Message: "Landfill needs initial capacity of at least two", Cells: []int{i}})
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
			if j < 0 || !m.floor(j) && m.Cells[j].Terrain != BreakableWall && m.Cells[j].Terrain != Landfill {
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
			issues = append(issues, MapIssue{Message: "A circuit needs at least one switch and one Gate, Door or Hatch", Cells: circuitCells, Edges: slices.Clone(gates[group]), Group: uint8(group)})
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
		case capacity < 3 && (f == Furnace || f == Crusher):
			issues = append(issues, MapIssue{Message: "Furnaces and Crushers need initial capacity of at least three", Cells: []int{i}})
		}
	}
	if first < 0 {
		return append(issues, MapIssue{Message: "Map needs playable Floor"})
	}
	if slices.ContainsFunc(m.Cells, func(c MapCell) bool { return c.Terrain == BreakableFloor }) {
		issues = append(issues, m.sealedByBreaking()...)
	}
	// Destructible terrain can join regions later without changing starting capacities. Breakable Floors and
	// Sinkholes can only split them, so the board must hold together without them, and with every Hatch open.
	future := m.Clone()
	for i, c := range future.Cells {
		if c.Circuit == Hatch {
			future.Cells[i] = MapCell{Terrain: Hole}
			continue
		}
		switch c.Terrain {
		case BreakableWall, Landfill:
			future.Cells[i].Terrain = Floor
		case BreakableFloor, Sinkhole:
			future.Cells[i].Terrain = Void
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
	ref := 0 // the region of the first playable Floor that can't break away
	for i, c := range m.Cells {
		if m.initialFloor(i) && c.Terrain == Floor && c.Circuit != Hatch {
			ref = regions[i]
			break
		}
	}
	var disconnected []int
	for i, c := range m.Cells {
		if (c.Terrain == Floor || c.Terrain == Landfill) && c.Feature != Portal && c.Feature != Spring && c.Circuit != Hatch && regions[i] != ref {
			disconnected = append(disconnected, i)
		}
	}
	if len(disconnected) > 0 {
		issues = append(issues, MapIssue{Message: "All Floor must connect through Landfills, breakable walls, breakable barriers or reachable Gates, never only Breakable Floors, Sinkholes or Hatches", Cells: disconnected})
	}
	if _, _, pace := m.Estimate(); pace == Instant || pace == Endless {
		issue := MapIssue{Message: "Too many Holes, Sinkholes, Hatches, Spillways, Crushers and Vaults: games would never end"}
		if pace == Instant {
			issue.Message = "Too many Furnaces, Reservoirs and Turrets: games would end almost at once"
		}
		for i, c := range m.Cells {
			drains := c.Terrain == Hole || c.Terrain == Sinkhole || c.Circuit == Hatch || c.Feature == Spillway || c.Feature == Crusher || c.Feature == Vault
			if pace == Instant && (c.Feature == Furnace || c.Feature == Reservoir || c.Feature == Turret) || pace == Endless && drains {
				issue.Cells = append(issue.Cells, i)
			}
		}
		issues = append(issues, issue)
	}
	return issues
}

// sealedByBreaking reports cells that every Breakable Floor around them breaking, walls standing, would leave without
// a route: their orbs could never explode or be captured.
func (m MapDefinition) sealedByBreaking() []MapIssue {
	broken := m.Clone()
	for i, c := range broken.Cells {
		if c.Terrain == BreakableFloor {
			broken.Cells[i].Terrain = Void
		}
	}
	var sealed []int
	for i, c := range m.Cells {
		if !m.initialFloor(i) && c.Terrain != BreakableWall && c.Terrain != Landfill {
			continue
		}
		was := broken.Cells[i].Terrain
		// Still standing, or opened, while the floors around it have broken; a shut-in Reservoir holds its orbs.
		broken.Cells[i].Terrain = Floor
		if n := broken.Capacity(i); n == 0 || n == 1 && c.Feature == Reservoir {
			sealed = append(sealed, i)
		}
		broken.Cells[i].Terrain = was
	}
	if len(sealed) == 0 {
		return nil
	}
	return []MapIssue{{Message: "Floor needs a route that breaking Breakable Floors can't remove", Cells: sealed}}
}

// Estimate predicts a game's average moves on the starting layout (0 when Endless), its length in thousandths of an
// empty board's, and its pace. Bot games of 2 to 4 mixed seats fit it on 7×7 to 40×40 boards: an empty board lasts
// about 0.75 moves per orb of room (what its Floor holds without exploding), and fills at the orb each move adds.
// Each open Hole side, Spillway, Crusher and Vault (whose shields destroy orbs) loses part of it, the same on every
// board size. Furnaces add to it, each
// a little less as they crowd, and less still beside a Hole, which they feed. More players make Furnace maps longer,
// since each Furnace works only on its holder's turn. Reservoirs add to it too, less, as they make an orb only when
// they explode. Turrets add the least: a shot's orb needs an enemy beside a loaded Turret as it explodes. A Landfill
// drains nothing: filling it costs about a move per orb, then its Floor adds room. A Sinkhole
// collapses early in a game and slows it like the Hole it becomes, so it counts as one.
// A Breakable Floor drains nothing, so it leaves the pace alone, but breaking into Void takes its room
// and a route from each neighbor, about two thirds of the way into a game on average. Each Box fills a Hole (or a
// collapsed Sinkhole) early in a game, assuming it reaches one, so the Holes Boxes can fill drain only for a while.
// Boxes and Springs otherwise drain and add nothing; a Spring takes its own cell's room. Integers keep validation
// identical on every device. Hatches drain nearly as much as Holes in bot games, so count their open sides regardless
// of starting state; like Sinkholes, their temporary Floor adds no lasting room.
func (m MapDefinition) Estimate() (moves, length int, pace Pace) {
	room, growth, furnaces, reservoirs, broken := 0, 1000, 0, 0, 0 // growth in thousandths of an empty board's
	pits, pitSides, boxes := 0, 0, 0
	for i := range m.Cells {
		switch {
		case m.Cells[i].Terrain == Hole || m.Cells[i].Terrain == Sinkhole || m.Cells[i].Circuit == Hatch:
			pits++
		case m.Cells[i].Feature == Box:
			boxes++
		}
		if m.Cells[i].Terrain == Landfill {
			// A finite cost, not a drain on growth: the orbs it absorbs, about a move each, and the Floor it becomes.
			room += 2 * m.FillTarget(i)
		}
		if m.Cells[i].Terrain == Sinkhole || m.Cells[i].Circuit == Hatch {
			continue // weighed as the Hole it soon becomes, through its neighbors' routes into it
		}
		capacity, holes := m.capacity(i)
		if capacity == 0 {
			continue
		}
		var near [5]int
		connections := m.InitialConnections(i, near[:0])
		for _, j := range connections {
			if m.Cells[j].Terrain == Sinkhole || m.Cells[j].Circuit == Hatch {
				holes++
			}
		}
		room += capacity - 1
		growth -= 45 * holes
		pitSides += holes
		if m.Cells[i].Terrain == BreakableFloor {
			broken += capacity - 1 // its own room, and one route from each neighbor that outlasts it
			for _, j := range connections {
				if t := m.Cells[j].Terrain; t != BreakableFloor && t != Sinkhole && m.Cells[j].Circuit != Hatch {
					broken++
				}
			}
		}
		switch m.Cells[i].Feature {
		case Spillway, Vault:
			growth -= 35
		case Crusher:
			growth -= 80
		case Turret:
			growth += 6
		case Furnace:
			furnaces++
			growth -= 30 * holes
		case Reservoir:
			if m.InitialOutlets(i) != 0 {
				reservoirs++
			}
		}
	}
	room -= broken * 2 / 3 // Breakable Floors break partway through a game
	// A Hole a Box fills costs 17 a side over its game instead of 45, spread over the Holes it might be.
	growth += (45 - 17) * pitSides * min(boxes, pits) / max(1, pits)
	growth += 150 * 83 * furnaces / (83 + furnaces)
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
