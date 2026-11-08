package boardview

import (
	"math"
	"slices"
	"testing"

	"orbitrix/client/internal/sound"
	"orbitrix/core"
)

func TestPortalFlightReplayAndSound(t *testing.T) {
	m := core.NewMap(7, 7)
	m.Cells[16], m.Cells[32] = core.MapCell{Feature: core.Portal, Group: 1}, core.MapCell{Feature: core.Portal, Group: 1}
	for direction, offset := range []int{-7, 7, -1, 1} {
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		source, landing := 16-offset, 32+offset
		g.Count[source], g.Owner[source], g.Orbs[0] = 3, 0, 3
		g.Count[48], g.Owner[48], g.Orbs[1], g.Played = 1, 1, 1, 3
		b := &Board{State: NewView(g), base: 80, k: 1}
		waves, err := g.Apply(source)
		if err != nil {
			t.Fatal(err)
		}
		b.Waves, b.Playing = waves, true
		b.State.Place(source, 0)
		b.State.Launch(waves[0])
		if b.linkSound() != sound.Link {
			t.Fatal("Portal transfer must use the exact Link cue")
		}
		for _, turned := range []bool{false, true} {
			b.Turned = turned
			for _, tc := range []struct {
				f        float64
				from, to int
				progress float64
			}{{0, source, 16, 0}, {.25, source, 16, .5}, {.5, 32, landing, 0}, {.75, 32, landing, .5}, {1, 32, landing, 1}} {
				sx, sy := b.Centre(tc.from)
				ex, ey := b.Centre(tc.to)
				x, y := b.flightPosition(source, direction, tc.f)
				if math.Abs(x-(sx+(ex-sx)*tc.progress)) > 1e-8 || math.Abs(y-(sy+(ey-sy)*tc.progress)) > 1e-8 {
					t.Fatalf("direction %d, turned %v, fraction %v: flight left its entry/exit segment", direction, turned, tc.f)
				}
			}
		}
		b.State.Land(waves[0])
		if !slices.Equal(b.State.Count, g.Count) || !slices.Equal(b.State.Owner, g.Owner) {
			t.Fatal("Portal replay differs from the resolved game")
		}
		b.Waves, b.Wave = []core.Wave{{0}}, 0
		if b.linkSound() != 0 {
			t.Fatal("unrelated explosion sounded a Portal")
		}
		b.Waves, b.Playing = waves, false
		if b.linkSound() != 0 {
			t.Fatal("idle Portal sounded")
		}
	}
}
