package ai

import (
	"orbitrix/core"
	"testing"
)

func TestBotsOnCustomMaps(t *testing.T) {
	barriers := core.NewMap(5, 5)
	barriers.Name = "Breakable barriers"
	barriers.Edges = []core.MapEdge{{A: 12, B: 13, Kind: core.BreakableBarrier}, {A: 12, B: 17, Kind: core.BreakableBarrier}}
	portals := core.NewMap(7, 7)
	portals.Name = "Portals"
	portals.Cells[16], portals.Cells[32] = core.MapCell{Feature: core.Portal, Group: 1}, core.MapCell{Feature: core.Portal, Group: 1}
	holes := core.NewMap(5, 5)
	holes.Name = "Holes"
	holes.Cells[12].Terrain, holes.Cells[0].Terrain = core.Hole, core.Hole
	landfills := core.NewMap(5, 5)
	landfills.Name = "Landfills"
	landfills.Cells[12].Terrain, landfills.Cells[0].Terrain = core.Landfill, core.Landfill
	floors := core.NewMap(5, 5)
	floors.Name = "Breakable Floors"
	floors.Cells[12].Terrain, floors.Cells[6].Terrain = core.BreakableFloor, core.BreakableFloor
	sinkholes := core.NewMap(5, 5)
	sinkholes.Name = "Sinkholes"
	sinkholes.Cells[12].Terrain, sinkholes.Cells[0].Terrain = core.Sinkhole, core.Sinkhole
	spillways := core.NewMap(5, 5)
	spillways.Name = "Spillways"
	spillways.Cells[12].Feature, spillways.Cells[0].Feature = core.Spillway, core.Spillway
	furnaces := core.NewMap(5, 5)
	furnaces.Name = "Furnaces and Crushers"
	furnaces.Cells[12].Feature, furnaces.Cells[2].Feature, furnaces.Cells[22].Feature = core.Furnace, core.Furnace, core.Crusher
	reservoirs := core.NewMap(5, 5)
	reservoirs.Name = "Reservoirs"
	reservoirs.Cells[12].Feature, reservoirs.Cells[7].Feature, reservoirs.Cells[2].Feature = core.Reservoir, core.Reservoir, core.Reservoir
	for _, m := range append(core.StarterMaps(), barriers, portals, holes, landfills, floors, sinkholes, spillways, furnaces, reservoirs) {
		t.Run(m.Name, func(t *testing.T) {
			g, err := core.NewGameWithMap(m, 3)
			if err != nil {
				t.Fatal(err)
			}
			rng := core.NewRNG(321)
			var legal []int
			for ply := 0; ply < 24 && !g.Over(); ply++ {
				legal = g.LegalMoves(legal[:0])
				_, _ = g.Apply(legal[rng.Intn(len(legal))])
			}
			if g.Over() {
				return
			}
			for strategy := Strategy(0); strategy < NumStrategies; strategy++ {
				t.Run(strategy.String(), func(t *testing.T) {
					var bot Bot
					before := g.Hash()
					first := bot.Choose(g, strategy, 123, 40)
					second := bot.Choose(g, strategy, 123, 40)
					if !g.Legal(first) {
						t.Fatalf("illegal move %d", first)
					}
					if first != second {
						t.Fatalf("nondeterministic moves %d %d", first, second)
					}
					if g.Hash() != before {
						t.Fatal("AI mutated game")
					}
				})
			}
		})
	}
}

// The slowest Hole map that validation accepts must still end: a plain 9×9 game takes about 150 moves.
func TestSlowestValidMapEnds(t *testing.T) {
	m := core.NewMap(9, 9)
	for k := range len(m.Cells) {
		i := k * 37 % len(m.Cells)
		m.Cells[i].Terrain = core.Hole
		if len(m.Validate()) != 0 {
			m.Cells[i].Terrain = core.Floor
		}
	}
	if _, _, pace := m.Estimate(); pace != core.Slow {
		t.Fatalf("pace %v", pace)
	}
	for seed := range uint64(4) {
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		var bots [2]Bot
		for move := 0; !g.Over(); move++ {
			if move == 1000 {
				t.Fatalf("seed %d: no winner after %d moves", seed, move)
			}
			_, _ = g.Apply(bots[g.Turn].Choose(g, Spy, seed<<32|uint64(move), DefaultBudget))
		}
	}
}

// deviceMap has Furnaces, Crushers and a Latch whose Gate changes capacities.
func deviceMap() core.MapDefinition {
	m := core.NewMap(5, 5)
	m.Cells[12].Feature, m.Cells[2].Feature = core.Furnace, core.Furnace
	m.Cells[10].Feature, m.Cells[22].Feature = core.Crusher, core.Crusher
	m.Cells[6].Circuit, m.Cells[6].Group = core.Latch, 1
	m.Edges = []core.MapEdge{{A: 16, B: 17, Kind: core.Gate, Group: 1}}
	return m
}

func TestTallyMatchesSimulation(t *testing.T) {
	// Filling a Sinkhole explodes nothing but loses orbs, so the quick no-explosion tally must not assume one more.
	sinkholes := core.NewMap(5, 5)
	for _, i := range []int{0, 12} {
		sinkholes.Cells[i].Terrain = core.Sinkhole
	}
	for _, m := range []core.MapDefinition{deviceMap(), sinkholes} {
		g, err := core.NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		rng := core.NewRNG(7)
		var b Bot
		var sim core.Game
		var legal []int
		checked, collapsed := 0, false
		for ply := 0; ply < 300 && !g.Over(); ply++ {
			b.g, b.geo, b.me, b.left = g, g.Geo, int8(g.Turn), 1<<30
			b.production()
			legal = g.LegalMoves(legal[:0])
			for _, cell := range legal {
				sim.CopyFrom(g)
				_ = sim.Simulate(cell)
				if got, ok := b.tally(cell); !ok || got != sim.Orbs {
					t.Fatalf("ply %d cell %d: tally %v, simulation %v", ply, cell, got[:2], sim.Orbs[:2])
				}
				collapsed = collapsed || m.Cells[cell].Terrain == core.Sinkhole && sim.TerrainAt(cell) == core.Hole
				checked++
			}
			_, _ = g.Apply(legal[rng.Intn(len(legal))])
		}
		if checked < 500 {
			t.Fatalf("%s: only %d positions checked", m.Name, checked)
		}
		if m.Cells[0].Terrain == core.Sinkhole && !collapsed {
			t.Fatal("no placement filled a Sinkhole")
		}
	}
}

func TestHeuristicBotsSkipWastedCrusherMoves(t *testing.T) {
	g, err := core.NewGameWithMap(deviceMap(), 2)
	if err != nil {
		t.Fatal(err)
	}
	rng := core.NewRNG(11)
	var b Bot
	var legal []int
	for ply := 0; ply < 80 && !g.Over(); ply++ {
		legal = g.LegalMoves(legal[:0])
		other := false
		for _, cell := range legal {
			other = other || !wasted(g, cell, int8(g.Turn))
		}
		for _, s := range []Strategy{Mason, Warden, Gambler, Nomad, Raider, Spy} {
			if cell := b.Choose(g, s, uint64(ply), DefaultBudget); other && wasted(g, cell, int8(g.Turn)) {
				t.Fatalf("ply %d: %s fed Crusher %d", ply, s, cell)
			}
		}
		_, _ = g.Apply(legal[rng.Intn(len(legal))])
	}
}
