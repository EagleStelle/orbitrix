package core

import "slices"

type PortalMode uint8

const (
	PortalBoth PortalMode = iota
	PortalHorizontal
	PortalVertical
)

type PortalAxes uint8

const (
	HorizontalAxis PortalAxes = 1 << iota
	VerticalAxis
	BothAxes = HorizontalAxis | VerticalAxis
)

func (mode PortalMode) Axes() PortalAxes {
	switch mode {
	case PortalHorizontal:
		return HorizontalAxis
	case PortalVertical:
		return VerticalAxis
	default:
		return BothAxes
	}
}

// InitialPortalAxes combines pair settings with exits available in the saved layout.
func (m MapDefinition) InitialPortalAxes() []PortalAxes {
	return m.portalAxes(nil, nil, nil, 0)
}

// AvailablePortalAxes ignores this pair's setting so the editor can offer every usable direction.
func (m MapDefinition) AvailablePortalAxes(cell int) PortalAxes {
	if cell < 0 || cell >= len(m.Cells) || m.Cells[cell].Feature != Portal {
		return BothAxes
	}
	return m.portalAxes(nil, nil, nil, m.Cells[cell].Group)[cell]
}

func (m MapDefinition) portalAxes(pairs []int16, blocked []uint8, floor func(int) bool, override uint8) []PortalAxes {
	var axes []PortalAxes
	for i, c := range m.Cells {
		if c.Feature != Portal {
			continue
		}
		if axes == nil {
			axes = make([]PortalAxes, len(m.Cells))
		}
		axes[i] = BothAxes
		for d := range 4 {
			next := m.neighbor(i, d)
			open := next >= 0
			if open && m.Cells[next].Feature == Spring {
				open = false // a Spring closes the axis, as a Hole does
			} else if open && floor != nil {
				open = floor(next)
			} else if open {
				open = m.floor(next) && (m.Cells[next].Circuit != Door || m.Cells[next].InitiallyOpen) || m.Cells[next].Terrain == Landfill
			}
			if !open || m.routeBlocked(i, d, blocked) {
				axes[i] &^= PortalAxes(1 << (d / 2))
			}
		}
	}
	// Removing an axis propagates across paired endpoints and adjoining Portals.
	propagate := func() {
		for changed := true; changed; {
			changed = false
			for i, c := range m.Cells {
				if c.Feature != Portal {
					continue
				}
				nextAxes := axes[i]
				pair := -1
				if pairs != nil {
					pair = int(pairs[i])
				} else {
					pair = m.partner(i)
				}
				if pair >= 0 {
					nextAxes &= axes[pair]
				}
				for d := range 4 {
					next := m.neighbor(i, d)
					axis := PortalAxes(1 << (d / 2))
					if next >= 0 && m.Cells[next].Feature == Portal && axes[next]&axis == 0 {
						nextAxes &^= axis
					}
				}
				if axes[i] != nextAxes {
					axes[i], changed = nextAxes, true
				}
			}
		}
	}
	propagate()
	physical := slices.Clone(axes)
	for i, c := range m.Cells {
		if c.Feature != Portal || c.Group == override && override != 0 {
			continue
		}
		if chosen := axes[i] & c.PortalMode.Axes(); chosen != 0 {
			axes[i] = chosen
		}
	}
	tried := slices.Clone(axes)
	for {
		propagate()
		changed := false
		for i, c := range m.Cells {
			if c.Feature != Portal || axes[i] != 0 {
				continue
			}
			if fallback := physical[i] &^ tried[i]; fallback != 0 {
				axes[i], tried[i], changed = fallback, tried[i]|fallback, true
			}
		}
		if !changed {
			break
		}
	}
	return axes
}
