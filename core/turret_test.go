package core

import "testing"

// turretGame restores a 5×5 two-player game after the first round with a Turret at cell turret just taken by player
// 0, and player 1's orbs on the given cells.
func turretGame(t *testing.T, turret int, enemy map[int]uint8) *Game {
	t.Helper()
	m := NewMap(5, 5)
	m.Cells[turret].Feature = Turret
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 3
	s.Count[turret], s.Owner[turret] = 1, 0
	for i, n := range enemy {
		s.Count[i], s.Owner[i] = n, 1
	}
	if g, err = Restore(s); err != nil {
		t.Fatal(err)
	}
	return g
}

// A Turret skips its holder's first move, fires a new orb up, right, down and left after their next four, capturing
// what it hits, then rearms for a move and starts over where it faces.
func TestTurretFiresClockwiseThenRearms(t *testing.T) {
	g := turretGame(t, 12, map[int]uint8{17: 1, 24: 1})
	mine, theirs := []int{0, 1, 2, 3, 4, 5, 10}, []int{24, 23, 22, 21, 20, 19, 14}
	// Orbs at up 11, right 17, down 13, left 7 after each of player 0's moves.
	want := [][4]uint8{{0, 1, 0, 0}, {1, 1, 0, 0}, {1, 2, 0, 0}, {1, 2, 1, 0}, {1, 2, 1, 1}, {1, 2, 1, 1}, {2, 2, 1, 1}}
	for i, cell := range mine {
		before := total(g)
		mustApply(t, g, cell)
		checkTallies(t, g)
		got := [4]uint8{g.Count[11], g.Count[17], g.Count[13], g.Count[7]}
		if got != want[i] {
			t.Fatalf("move %d: orbs up, right, down, left %v, want %v", i+1, got, want[i])
		}
		fired := 0
		if i >= 1 && i != 5 {
			fired = 1
		}
		if total(g) != before+1+fired {
			t.Fatalf("move %d: %d orbs over %d, want the placed one and %d shot", i+1, total(g), before, fired)
		}
		if working := MakerWorking(g.Pressed[12], 0); working != (i >= 1 && i <= 3 || i == 6) {
			t.Fatalf("move %d: state %d, working %v", i+1, g.Pressed[12], working)
		}
		if Shield(g.Pressed[12]) != 0 {
			t.Fatalf("move %d: a shot count is no shield", i+1)
		}
		mustApply(t, g, theirs[i])
	}
	if g.Owner[17] != 0 || g.Durability[12] != 1 {
		t.Fatalf("17 held by %d, Turret step %d: a shot captures, and the Turret faces right again", g.Owner[17], g.Durability[12])
	}
}

// A Turret never faces a closed side: in a corner it alternates between its two routes.
func TestTurretSkipsClosedSides(t *testing.T) {
	g := turretGame(t, 0, map[int]uint8{24: 1})
	for i, cell := range []int{12, 11, 13, 7, 17} {
		mustApply(t, g, cell)
		mustApply(t, g, []int{24, 23, 22, 21, 20}[i])
	}
	if g.Count[5] != 2 || g.Count[1] != 2 || g.Owner[5] != 0 || g.Owner[1] != 0 {
		t.Fatalf("right %d, down %d: four shots must alternate between the corner's two routes", g.Count[5], g.Count[1])
	}
}

// A Vault's shield takes a shot like any other orb, and the Turret keeps its facing when it empties.
func TestTurretShotHitsAShieldAndKeepsItsFacing(t *testing.T) {
	m := NewMap(5, 5)
	m.Cells[12].Feature, m.Cells[11].Feature = Turret, Vault
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 3
	s.Count[12], s.Owner[12], s.Pressed[12] = 1, 0, 1|makerWorking
	s.Count[11], s.Owner[11], s.Pressed[11] = 1, 1, 2|makerWorking|VaultShields*vaultShield
	s.Count[24], s.Owner[24] = 1, 1
	if g, err = Restore(s); err != nil {
		t.Fatal(err)
	}
	before := total(g)
	mustApply(t, g, 0)
	checkTallies(t, g)
	if Shield(g.Pressed[11]) != 1 || g.Owner[11] != 1 || g.Count[11] != 1 || total(g) != before+1 {
		t.Fatalf("shield %d, Vault %d of %d, %d orbs over %d", Shield(g.Pressed[11]), g.Count[11], g.Owner[11], total(g), before)
	}
	g.Count[12], g.Owner[12] = 0, NoOwner
	g.Orbs[0]--
	g.settleMakers()
	if g.Pressed[12] != 0 || g.Durability[12] != 1 {
		t.Fatalf("state %d, step %d: an emptied Turret resets its count, not its facing", g.Pressed[12], g.Durability[12])
	}
}

func TestTurretSnapshotState(t *testing.T) {
	for _, tc := range []struct {
		feature Feature
		state   uint8
		step    uint8
		ok      bool
	}{
		{Turret, 1 | makerWorking | turretFlag | 3*vaultShield, 3, true},
		{Turret, 1 | makerWorking, 0, true},
		{Turret, 1 | makerWorking | turretFlag, 0, false},              // counts start at one
		{Turret, 1 | turretFlag | vaultShield, 0, false},               // only a working Turret counts
		{Turret, 1 | makerWorking | vaultShield, 0, false},             // a shield
		{Turret, 1 | makerWorking, 4, false},                           // four steps
		{Vault, 1 | makerWorking | turretFlag | vaultShield, 0, false}, // a count
		{Furnace, 1 | makerWorking, 1, false},                          // a facing
	} {
		m := NewMap(5, 5)
		m.Cells[12].Feature = tc.feature
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		s := g.Snapshot()
		s.Played = 3
		s.Count[12], s.Owner[12], s.Pressed[12], s.Durability[12] = 1, 0, tc.state, tc.step
		if _, err := Restore(s); (err == nil) != tc.ok {
			t.Fatalf("%v state %d step %d: restore error %v", tc.feature, tc.state, tc.step, err)
		}
	}
}
