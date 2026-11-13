package boardview

import (
	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func (v *view) terrainSounds(events []core.TerrainEvent) (effects sound.Effects) {
	if v.Replay == nil {
		return
	}
	for _, e := range events {
		if e.Cell < 0 { // Gates already have their controller's cue.
			continue
		}
		if e.Box { // its slide knocked at launch; filling a pit, it falls in, and levels it as it lands (Board.boxSunk)
			if v.Replay.Boxes[e.To] == core.CrateFloor {
				effects |= sound.Fall
			}
			continue
		}
		if !e.Switch {
			if !e.Edge && (v.Replay.Map.Cells[e.Cell].Terrain == core.Landfill || v.Replay.Map.Cells[e.Cell].Terrain == core.Sinkhole || v.Replay.Map.Cells[e.Cell].Circuit == core.Hatch) {
				effects |= sound.Fall
				continue
			}
			if e.Durability == 0 {
				effects |= sound.WallBreak
			} else {
				effects |= sound.WallHit
			}
			continue
		}
		switch v.Replay.Map.Cells[e.Cell].Circuit {
		case core.Latch:
			effects |= sound.Latch
		case core.Toggle:
			if e.Pressed {
				effects |= sound.ToggleOn
			} else {
				effects |= sound.ToggleOff
			}
		case core.Hold:
			if e.Pressed {
				effects |= sound.HoldOn
			} else {
				effects |= sound.HoldOff
			}
		}
	}
	return
}

// linkSound is the cue of the wave just launched: a chime for orbs or Boxes crossing a Link or Portal, and the Box and
// Spring cues.
func (b *Board) linkSound() sound.Effects {
	if !b.Playing || b.settling || b.State.Replay == nil {
		return 0
	}
	effects := b.boxSounds()
	cells := b.State.Replay.Map.Cells
	portal := func(hops []int16) bool {
		for _, cell := range hops {
			if cells[cell].Feature == core.Portal {
				return true
			}
		}
		return false
	}
	if portal(b.routes) {
		return effects | sound.Link
	}
	for _, cell := range b.Waves[b.Wave] {
		if cells[cell].Feature == core.Link {
			return effects | sound.Link
		}
		for connection := range b.State.Geo.Targets(int(cell)) {
			if portal(b.State.Geo.PortalPath(int(cell), connection)) {
				return effects | sound.Link
			}
		}
	}
	return effects
}
