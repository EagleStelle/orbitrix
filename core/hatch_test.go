package core

import (
	"slices"
	"strings"
	"testing"
)

func hatchMap() MapDefinition {
	m := NewMap(7, 7)
	m.Cells[8] = MapCell{Circuit: Toggle, Group: 1}
	m.Cells[24] = MapCell{Circuit: Hatch, Group: 1}
	m.Cells[32] = MapCell{Circuit: Hatch, Group: 1, InitiallyOpen: true}
	return m
}

func TestHatchOpeningDropsOrbsAndClosingRestoresFloor(t *testing.T) {
	for _, placement := range []bool{true, false} {
		m := hatchMap()
		m.Cells[9] = MapCell{Circuit: Toggle, Group: 1}
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		if !g.Legal(24) || g.Legal(32) || g.Geo.Cap[25] != 4 || g.Map.Capacity(25) != 4 {
			t.Fatal("closed Hatches must be Floor and open ones Hole routes")
		}
		seedMapCell(g, 24, 1, 3)
		seedMapCell(g, 48, 1, 1)
		cell, eventWave := 8, -1
		if !placement {
			seedMapCell(g, 7, 0, 2) // exploding 7 sends an orb onto Toggle 8
			cell, eventWave = 7, 0
		}
		checkMove(t, g, cell)
		if g.TerrainAt(24) != Hole || g.Count[24] != 0 || g.Owner[24] != NoOwner || g.Orbs[1] != 1 || !g.Geo.Playable(32) || g.Geo.Cap[25] != 4 {
			t.Fatalf("opening lost the wrong orbs or changed capacity: %v", g.Orbs[:2])
		}
		if !slices.Contains(g.TerrainEvents, TerrainEvent{Wave: eventWave, Cell: 24}) {
			t.Fatalf("drop must follow the circuit's step: %v", g.TerrainEvents)
		}
		g.Turn = 0
		checkMove(t, g, 9)
		if g.TerrainAt(24) != Floor || g.Count[24] != 0 || g.TerrainAt(32) != Hole {
			t.Fatal("closing must restore empty Floor without returning lost orbs")
		}
	}
}

func TestHatchOpeningEliminatesItsLastOwner(t *testing.T) {
	g, err := NewGameWithMap(hatchMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	seedMapCell(g, 24, 1, 2)
	g.Played = 3
	if waves := checkMove(t, g, 8); len(waves) != 0 || !g.Over() || g.Winner != 0 {
		t.Fatalf("opening must eliminate before the next wave: %v, winner %d", waves, g.Winner)
	}
}

func TestHatchBoxesFillPermanently(t *testing.T) {
	for _, standing := range []bool{true, false} {
		m := hatchMap()
		m.Cells[40].Feature = Box
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		cell, target := 30, 32
		if standing {
			g.Boxes[40], g.Boxes[24] = 0, BoxHere
			cell, target = 8, 24
		} else {
			g.Boxes[40], g.Boxes[31] = 0, BoxHere
			seedMapCell(g, 30, 0, 3) // pushes 31 into the open Hatch 32
		}
		checkMove(t, g, cell)
		if g.Boxes[target] != CrateFloor || g.TerrainAt(target) != Floor {
			t.Fatal("a Box must fill both a standing Hatch that opens and one it slides into")
		}
		g.Turn = 0
		checkMove(t, g, 8)
		if g.Boxes[target] != CrateFloor || g.TerrainAt(target) != Floor {
			t.Fatal("a filled Hatch must stop responding to its circuit")
		}
	}
}

func TestHatchRandomPlayMatchesSimulationAndSnapshots(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		m := hatchMap()
		m.Cells[16] = MapCell{Circuit: Hold, Group: 1}
		m.Cells[31].Feature = Box
		g, err := NewGameWithMap(m, 3)
		if err != nil {
			t.Fatal(err)
		}
		rng := NewRNG(seed)
		var legal []int
		for move := 0; move < 180 && !g.Over(); move++ {
			legal = g.LegalMoves(legal[:0])
			checkMove(t, g, legal[rng.Intn(len(legal))])
			var tallies [MaxPlayers]int32
			for i, n := range g.Count {
				if n != 0 {
					tallies[g.Owner[i]] += int32(n)
				}
				if g.TerrainAt(i) == Hole && n != 0 {
					t.Fatalf("move %d: orbs remain in open Hatch %d", move, i)
				}
			}
			if tallies != g.Orbs {
				t.Fatalf("move %d: tallies %v, want %v", move, g.Orbs, tallies)
			}
		}
	}
}

func TestHatchValidationAndSnapshotRejections(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[0] = MapCell{Circuit: Toggle, Group: 1}
	for row := range 5 {
		m.Cells[10+row] = MapCell{Circuit: Hatch, Group: 1}
	}
	if !slices.ContainsFunc(m.Validate(), func(issue MapIssue) bool { return strings.HasPrefix(issue.Message, "All Floor must connect") }) {
		t.Fatal("Hatches must not be the only connection between players")
	}
	g, err := NewGameWithMap(hatchMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Count[32], s.Owner[32] = 1, 0
	if _, err := Restore(s); err == nil {
		t.Fatal("snapshot accepted orbs in an open Hatch")
	}
}

// Mixed-bot games put three toggled Hatches close to three Holes, so use the same per-side drain, not half of it.
func TestHatchEstimate(t *testing.T) {
	for _, open := range []bool{false, true} {
		for _, cells := range [][]int{{40}, {20, 60}, {20, 24, 56}, {0, 40}, {39, 40}, {20, 24, 56, 60}} {
			hatches, holes := NewMap(9, 9), NewMap(9, 9)
			hatches.Cells[8], holes.Cells[8] = MapCell{Circuit: Toggle, Group: 1}, MapCell{Circuit: Toggle, Group: 1}
			for _, i := range cells {
				hatches.Cells[i] = MapCell{Circuit: Hatch, Group: 1, InitiallyOpen: open}
				holes.Cells[i].Terrain = Hole
			}
			for _, boxed := range []bool{false, true} {
				if boxed {
					hatches.Cells[10].Feature, holes.Cells[10].Feature = Box, Box
				}
				hm, hl, hp := hatches.Estimate()
				pm, pl, pp := holes.Estimate()
				if hm != pm || hl != pl || hp != pp {
					t.Fatalf("Hatches %v open=%v boxed=%v: %d/%d/%v, Holes %d/%d/%v", cells, open, boxed, hm, hl, hp, pm, pl, pp)
				}
				if hp == Endless {
					for _, cell := range cells {
						if !slices.ContainsFunc(hatches.Validate(), func(issue MapIssue) bool {
							return strings.HasPrefix(issue.Message, "Too many Holes") && slices.Contains(issue.Cells, cell)
						}) {
							t.Fatalf("Endless must mark Hatch %d", cell)
						}
					}
				}
			}
		}
	}
}

func TestHatchChangesPortalAxes(t *testing.T) {
	m := portalMap()
	m.Cells[33] = MapCell{Circuit: Hatch, Group: 1}
	m.Cells[0] = MapCell{Circuit: Toggle, Group: 1}
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.Geo.PortalAxes[16] != BothAxes || m.InitialPortalAxes()[16] != BothAxes {
		t.Fatal("a closed Hatch must allow a Portal exit")
	}
	checkMove(t, g, 0)
	if g.Geo.PortalAxes[16] != HorizontalAxis {
		t.Fatal("an open Hatch must close the Portal's vertical axis like a Hole")
	}
	m.Cells[33].InitiallyOpen = true
	if m.InitialPortalAxes()[16] != HorizontalAxis {
		t.Fatal("the editor must use the same Portal obstruction")
	}
}
