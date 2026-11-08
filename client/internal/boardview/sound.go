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
		if !e.Switch {
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

func (b *Board) linkSound() sound.Effects {
	if b.Playing && b.State.Replay != nil {
		for _, cell := range b.Waves[b.Wave] {
			if b.State.Replay.Map.Cells[cell].Feature == core.Link {
				return sound.Link
			}
			for connection := range b.State.Geo.Neighbours(int(cell)) {
				if len(b.State.Geo.PortalPath(int(cell), connection)) != 0 {
					return sound.Link
				}
			}
		}
	}
	return 0
}
