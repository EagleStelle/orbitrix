package core

// randomCells are RandomMap's one-cell pieces; repeats weigh them. Furnaces, Reservoirs and Turrets, which make orbs,
// are about a third of all rolls.
var randomCells = [...]MapCell{
	{Terrain: Wall}, {Terrain: Wall}, {Terrain: Wall}, {Terrain: Wall},
	{Terrain: Void}, {Terrain: Void}, {Terrain: Void},
	{Terrain: Hole}, {Terrain: Hole},
	{Terrain: Landfill}, {Terrain: Sinkhole}, {Terrain: BreakableFloor}, {Terrain: BreakableWall}, {Terrain: BreakableWall},
	{Feature: Spillway}, {Feature: Crusher}, {Feature: Vault}, {Feature: Box},
	{Feature: Furnace}, {Feature: Furnace}, {Feature: Furnace}, {Feature: Furnace},
	{Feature: Reservoir}, {Feature: Reservoir}, {Feature: Reservoir}, {Feature: Reservoir},
	{Feature: Turret}, {Feature: Turret}, {Feature: Turret}, {Feature: Turret},
}

// RandomMap's other rolls, numbered after randomCells: three Barriers, two Breakable Barriers, a Spring, a Link pair,
// two circuits and a Portal pair.
const (
	rollSpring = len(randomCells) + 5 + iota
	rollLink
	rollCircuit
	rollPortal = rollCircuit + 2
	rollCount  = rollPortal + 1
)

// RandomMap rolls a playable map. Pieces land one at a time on plain Floor, and each stays only while the map still
// validates, keeps room for every seat and plays no slower than Normal, so the result has no issues and its pace is
// Fast or Normal. The same seed rolls the same map.
func RandomMap(cols, rows int, seed uint64) MapDefinition {
	m := NewMap(cols, rows)
	m.Name = "Random map"
	rng := NewRNG(seed)
	n := len(m.Cells)
	var groups [3]uint8 // Link pairs, Portal pairs and circuits used so far
	try := func(roll int) {
		a, far := rng.Intn(n), rng.Intn(n)
		b := m.neighbor(a, rng.Intn(4))
		kind, open := rng.Intn(3), rng.Intn(2) == 0
		if b < 0 || far == a || far == b || m.Cells[a] != (MapCell{}) || m.Cells[b] != (MapCell{}) || m.Cells[far] != (MapCell{}) {
			return
		}
		edges, used := len(m.Edges), groups
		group := func(space int) uint8 { // the next unused color, 0 once all are taken
			if groups[space] == MaxMapGroups {
				return 0
			}
			groups[space]++
			return groups[space]
		}
		switch {
		case roll < len(randomCells):
			m.Cells[a] = randomCells[roll]
		case roll < rollSpring-2:
			m.Edges = append(m.Edges, MapEdge{A: a, B: b})
		case roll < rollSpring:
			m.Edges = append(m.Edges, MapEdge{A: a, B: b, Kind: BreakableBarrier})
		case roll == rollSpring: // it stands against its own Wall
			m.Cells[a], m.Cells[b] = MapCell{Feature: Spring}, MapCell{Terrain: Wall}
		case roll == rollLink:
			m.Cells[a] = MapCell{Feature: Link, Group: group(0)}
			m.Cells[far] = m.Cells[a]
		case roll == rollPortal:
			m.Cells[a] = MapCell{Feature: Portal, Group: group(1)}
			m.Cells[far] = m.Cells[a]
		default: // a switch at far working a Gate, Door or Hatch at a
			g := group(2)
			m.Cells[far] = MapCell{Circuit: Latch + CircuitKind(rng.Intn(3)), Group: g}
			if kind == 2 {
				m.Edges = append(m.Edges, MapEdge{A: a, B: b, Kind: Gate, Group: g, InitiallyOpen: open})
			} else {
				m.Cells[a] = MapCell{Circuit: Door + CircuitKind(kind), Group: g, InitiallyOpen: open}
			}
		}
		// A piece left without a color fails validation like any other that doesn't fit.
		if _, _, pace := m.Estimate(); pace == Slow || m.FloorCount() < min(n, MaxPlayers) || len(m.Validate()) != 0 {
			m.Cells[a], m.Cells[b], m.Cells[far] = MapCell{}, MapCell{}, MapCell{}
			m.Edges, groups = m.Edges[:edges], used
		}
	}
	portals := 0
	// shortcut: at most 150 pieces, since each one validates the whole map (0.4 s for 266 on 40×40), so boards past
	// 30×30 roll sparser; drop the cap once Validate routes drafts from tables as Geometry does.
	for range min(n/6, 150) {
		if roll := rng.Intn(rollCount); roll == rollPortal {
			portals++ // rolled last: validating a draft slows with every Portal on it
		} else {
			try(roll)
		}
	}
	for range portals {
		try(rollPortal)
	}
	return m
}
