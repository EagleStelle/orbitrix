package core

import (
	"slices"
	"testing"
)

// teamGame returns a 5×5 game of three players, 0 and 1 against 2, restored from the position set makes; every
// player has had a turn and player 0 is to move.
func teamGame(t *testing.T, set func(s *Snapshot, put func(cell int, orbs ...uint8))) *Game {
	t.Helper()
	g, err := NewGame(5, 5, 3)
	if err == nil {
		err = g.SetTeams([]uint8{0, 0, 1})
	}
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 0b111
	set(&s, func(cell int, orbs ...uint8) {
		most := 0
		for p, n := range orbs {
			s.Mix[cell*3+p] = n
			s.Count[cell] += n
			if n > orbs[most] {
				most = p
			}
		}
		s.Owner[cell] = int8(most)
	})
	if g, err = Restore(s); err != nil {
		t.Fatal(err)
	}
	return g
}

func mix(g *Game, cell int) []uint8 { return g.Mix[cell*g.Players : (cell+1)*g.Players] }

func TestTeamsShareCells(t *testing.T) {
	g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
		put(0, 1)       // player 0's corner, about to pop
		put(1, 0, 1)    // their teammate's cell
		put(5, 0, 0, 1) // their rival's
		put(24, 0, 0, 1)
	})
	if g.Legal(1) || g.Legal(5) || !g.Legal(0) {
		t.Fatal("a player may place only on empty cells and their own")
	}
	if _, err := g.Apply(0); err != nil {
		t.Fatal(err)
	}
	// The corner's orbs go up... it has no up: right (5), then down (1).
	if !slices.Equal(mix(g, 1), []uint8{1, 1, 0}) || g.Owner[1] != 1 {
		t.Fatalf("teammate's cell: %v owner %d, want both colours and its holder kept", mix(g, 1), g.Owner[1])
	}
	if !slices.Equal(mix(g, 5), []uint8{2, 0, 0}) || g.Owner[5] != 0 {
		t.Fatalf("rival's cell: %v owner %d, want captured", mix(g, 5), g.Owner[5])
	}
	if g.Orbs != [MaxPlayers]int32{3, 1, 1} {
		t.Fatalf("tallies %v", g.Orbs[:3])
	}
	if err := g.Pass(); err != nil || g.Turn != 2 { // player 1, then the rival
		t.Fatal(err, g.Turn)
	}
	if err := g.Pass(); err != nil || !g.Legal(1) { // player 0 again: their orb is in the teammate's cell now
		t.Fatal("orbs in a teammate's cell make it playable", err)
	}
}

func TestTeamsDealFewestFirstClockwise(t *testing.T) {
	g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
		put(12, 1, 3) // the centre, at capacity
		put(24, 0, 0, 1)
	})
	waves, err := g.Apply(0)
	if err != nil || len(waves) != 1 {
		t.Fatal(err, waves)
	}
	for cell, want := range map[int]int8{11: 0, 17: 1, 13: 1, 7: 1} { // up, right, down, left
		if g.Owner[cell] != want || g.Count[cell] != 1 {
			t.Fatalf("cell %d: owner %d count %d, want player %d's orb", cell, g.Owner[cell], g.Count[cell], want)
		}
	}
	if g.Count[12] != 0 || g.Owner[12] != NoOwner {
		t.Fatalf("centre keeps %d", g.Count[12])
	}
}

func TestTeamsSplitCapture(t *testing.T) {
	for name, tc := range map[string]struct {
		rival uint8
		want  []uint8
	}{
		"odd to the lower seat": {1, []uint8{2, 1, 0}},
		"even":                  {2, []uint8{2, 2, 0}},
	} {
		t.Run(name, func(t *testing.T) {
			g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
				put(11, 4)    // above the centre
				put(13, 0, 4) // below it
				put(12, 0, 0, tc.rival)
				put(24, 0, 0, 1)
			})
			waveLimit = 1 // the full centre would pop on
			defer func() { waveLimit = MaxWaves }()
			if _, err := g.Apply(0); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(mix(g, 12), tc.want) || g.Owner[12] != 0 {
				t.Fatalf("centre %v owner %d, want %v", mix(g, 12), g.Owner[12], tc.want)
			}
		})
	}
}

func TestTeamsWin(t *testing.T) {
	g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
		put(0, 1)
		put(1, 0, 1)
		put(5, 0, 0, 1) // the rival's last orb
		s.Turn = 1
	})
	if _, err := g.Apply(1); err != nil || g.Over() {
		t.Fatal("no win yet", err)
	}
	if err := g.Pass(); err != nil { // the rival
		t.Fatal(err)
	}
	if _, err := g.Apply(0); err != nil || !g.Over() || g.Winner != 0 || !g.Won(1) || g.Won(2) {
		t.Fatalf("winner %d over %v: %v", g.Winner, g.Over(), err)
	}
}

func TestSetTeams(t *testing.T) {
	for name, team := range map[string][]uint8{"one team": {1, 1, 1}, "too few": {0, 1}, "team out of range": {0, 1, MaxTeams}} {
		if g, _ := NewGame(5, 5, 3); g.SetTeams(team) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	g, _ := NewGame(5, 5, 3)
	if _, err := g.Apply(0); err != nil || g.SetTeams([]uint8{0, 0, 1}) == nil {
		t.Error("teams set after the first move")
	}
}

func TestTeamsRestoreRejects(t *testing.T) {
	g := teamGame(t, func(s *Snapshot, put func(int, ...uint8)) {
		put(12, 1, 2)
		put(24, 0, 0, 1)
	})
	for name, change := range map[string]func(s *Snapshot){
		"owner without the most": func(s *Snapshot) { s.Owner[12] = 0 },
		"two teams in a cell":    func(s *Snapshot) { s.Mix[12*3], s.Mix[12*3+2] = 0, 1 },
		"count differs":          func(s *Snapshot) { s.Count[12]++ },
		"no mix":                 func(s *Snapshot) { s.Mix = nil },
		"mix without teams":      func(s *Snapshot) { s.Team = nil },
	} {
		s := g.Snapshot()
		change(&s)
		if _, err := Restore(s); err == nil {
			t.Errorf("%s: restored", name)
		}
	}
}

// checkTeams checks a team game's cells against its tallies: each holds one team's orbs, owned by the colour with the
// most.
func checkTeams(t *testing.T, g *Game) {
	t.Helper()
	var orbs [MaxPlayers]int32
	for i, c := range g.Count {
		sum, o := 0, g.Owner[i]
		for q, m := range mix(g, i) {
			if m != 0 && (!g.Allied(o, int8(q)) || m > mix(g, i)[o]) {
				t.Fatalf("cell %d: %v with owner %d", i, mix(g, i), o)
			}
			sum += int(m)
			orbs[q] += int32(m)
		}
		if sum != int(c) || (c == 0) != (o == NoOwner) {
			t.Fatalf("cell %d: count %d, colours %v, owner %d", i, c, mix(g, i), o)
		}
	}
	if orbs != g.Orbs {
		t.Fatalf("tallies %v, board %v", g.Orbs, orbs)
	}
}

// Random team games on classic boards and rolled maps keep every invariant, replay identically through Simulate and
// a restored snapshot, and end with one team holding every orb.
func TestTeamsRandomGames(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		rng := NewRNG(seed)
		players := 3 + rng.Intn(6)
		team := make([]uint8, players)
		for p := range team {
			team[p] = uint8(rng.Intn(3))
		}
		team[0], team[1] = 0, 1
		g, err := NewGame(6, 7, players)
		if seed%2 == 0 {
			g, err = NewGameWithMap(RandomMap(8, 9, seed), players)
		}
		if err == nil {
			err = g.SetTeams(team)
		}
		if err != nil {
			t.Fatal(err)
		}
		var legal []int
		sim := new(Game)
		for move := 0; move < 600 && !g.Over(); move++ {
			if legal = g.LegalMoves(legal[:0]); len(legal) == 0 {
				if err := g.Pass(); err != nil { // a rolled map can leave a player no cell
					t.Fatal(err)
				}
				continue
			}
			cell, before := legal[rng.Intn(len(legal))], total(g)
			restored, err := Restore(g.Snapshot())
			if err != nil {
				t.Fatalf("seed %d move %d: %v", seed, move, err)
			}
			sim.CopyFrom(g)
			if _, err := g.Apply(cell); err != nil {
				t.Fatal(err)
			}
			checkTeams(t, g)
			if g.Map == nil && total(g) != before+1 {
				t.Fatalf("seed %d move %d: orbs %d → %d", seed, move, before, total(g))
			}
			if err := sim.Simulate(cell); err != nil || sim.Hash() != g.Hash() {
				t.Fatalf("seed %d move %d: Simulate differs from Apply (%v)", seed, move, err)
			}
			if _, err := restored.Apply(cell); err != nil || restored.Hash() != g.Hash() {
				t.Fatalf("seed %d move %d: a restored game differs (%v)", seed, move, err)
			}
		}
		if g.Over() {
			for p := range players {
				if g.Orbs[p] > 0 && !g.Won(int8(p)) {
					t.Fatalf("seed %d: player %d has orbs but lost to %d", seed, p, g.Winner)
				}
			}
		}
	}
}

func TestJoinTeam(t *testing.T) {
	bot := func(j int) bool { return j == 1 }
	for _, tc := range []struct {
		i        int
		to       uint8
		capacity int
		want     []uint8
		ok       bool
	}{
		{2, 0, 3, []uint8{0, 0, 0}, true},  // room
		{2, 0, 2, []uint8{0, 1, 0}, true},  // full: its bot trades teams
		{1, 1, 1, []uint8{0, 0, 1}, false}, // full of a human
		{0, 0, 3, []uint8{0, 0, 1}, false}, // already there
	} {
		team := []uint8{0, 0, 1}
		if ok := JoinTeam(team, tc.i, tc.to, tc.capacity, bot); ok != tc.ok || !slices.Equal(team, tc.want) {
			t.Errorf("player %d to team %d of %d: %v %v, want %v %v", tc.i, tc.to, tc.capacity, team, ok, tc.want, tc.ok)
		}
	}
}
