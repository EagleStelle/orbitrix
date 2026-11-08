package core

// CircuitKind is a cell component, separate from passive map features.
type CircuitKind uint8

const (
	NoCircuit CircuitKind = iota
	Latch
	Toggle
	Hold
	Door
)

func (c CircuitKind) IsSwitch() bool { return c == Latch || c == Toggle || c == Hold }
func (c CircuitKind) IsOutput() bool { return c == Door }

// EdgeKind describes an obstruction between orthogonally adjacent cells.
type EdgeKind uint8

const (
	Barrier EdgeKind = iota
	Gate
)

// CircuitOpen applies circuit polarity to an output's individual starting state.
func CircuitOpen(initiallyOpen, reversed bool) bool { return initiallyOpen != reversed }

// circuitInputs batches a step before changing polarity, so simultaneous inputs
// cancel in pairs and never depend on cell or explosion iteration order.
func (g *Game) circuitInputs(wave int, record bool) bool {
	var next [MaxSize * MaxSize]uint8
	var exploding [MaxSize * MaxSize]bool
	var flips, last [MaxMapGroups + 1]int
	if wave >= 0 {
		for _, cell := range g.cur {
			exploding[cell] = true
		}
	}
	for _, cell := range g.topology.switches {
		i := int(cell)
		c := g.Map.Cells[i]
		pressed := g.Pressed[i]
		switch c.Circuit {
		case Latch:
			if g.arrived[i] != 0 {
				pressed = 1
			}
		case Toggle:
			if wave < 0 {
				if g.arrived[i] != 0 && g.Count[i] == 1 {
					pressed ^= 1
				}
			} else if g.arrived[i] != 0 || exploding[i] {
				pressed ^= 1
			}
		case Hold:
			pressed = 0
			if g.Count[i] > 0 {
				pressed = 1
			}
		}
		next[i] = pressed
		if pressed != g.Pressed[i] {
			flips[c.Group]++
			last[c.Group] = i
		}
	}
	for _, cell := range g.topology.switches {
		i := int(cell)
		c := g.Map.Cells[i]
		// Resetting a latch arms it; it is not an additional circuit input.
		if c.Circuit == Latch && (flips[c.Group] > 1 || flips[c.Group] == 1 && last[c.Group] != i) {
			next[i] = 0
		}
		if next[i] != g.Pressed[i] {
			g.Pressed[i] = next[i]
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: i, Group: c.Group, Switch: true, Pressed: next[i] != 0})
			}
		}
	}
	changed := false
	for group, count := range flips {
		if count%2 != 0 {
			g.CircuitOn[group] = !g.CircuitOn[group]
			changed = true
			if record {
				g.TerrainEvents = append(g.TerrainEvents, TerrainEvent{Wave: wave, Cell: -1, Group: uint8(group), Open: g.CircuitOn[group]})
			}
		}
	}
	return changed
}
