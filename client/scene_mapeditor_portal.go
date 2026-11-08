package client

import "orbitrix/core"

var portalLabels = [...]string{"All directions", "Horizontal", "Vertical"}

func (e *mapEditor) refreshPortal() {
	if e.portalCell < 0 || e.portalCell >= len(e.m.Cells) || e.m.Cells[e.portalCell].Feature != core.Portal {
		e.portalCell, e.portalAllowed = -1, core.BothAxes
		return
	}
	e.portalAllowed = e.m.AvailablePortalAxes(e.portalCell)
	switch e.m.InitialPortalAxes()[e.portalCell] {
	case core.HorizontalAxis:
		e.portalMode = core.PortalHorizontal
	case core.VerticalAxis:
		e.portalMode = core.PortalVertical
	default:
		e.portalMode = core.PortalBoth
	}
}

func (e *mapEditor) setPortalMode(mode core.PortalMode) bool {
	if e.portalAllowed&mode.Axes() != mode.Axes() {
		return false
	}
	e.finishStroke()
	if e.portalCell < 0 {
		e.portalMode = mode
		return true
	}
	group := e.m.Cells[e.portalCell].Group
	changed := false
	for _, c := range e.m.Cells {
		changed = changed || c.Feature == core.Portal && c.Group == group && c.PortalMode != mode
	}
	if !changed {
		return false
	}
	e.checkpoint()
	for i, c := range e.m.Cells {
		if c.Feature == core.Portal && c.Group == group {
			e.m.Cells[i].PortalMode = mode
		}
	}
	e.changed()
	return true
}
