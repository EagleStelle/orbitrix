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
		switch v.Replay.Map.Cells[e.Cell].Feature {
		case core.Switch:
			effects |= sound.Latch
		case core.ToggleSwitch:
			if e.Pressed {
				effects |= sound.ToggleOn
			} else {
				effects |= sound.ToggleOff
			}
		case core.HoldSwitch:
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
		}
	}
	return 0
}
