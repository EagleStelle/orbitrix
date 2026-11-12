package core

import "testing"

// vaultGame restores a 5×5 two-player game after the first round with a Vault at the centre (12) held by player 0
// with orbs orbs in state, and player 1's orbs on the given cells.
func vaultGame(t *testing.T, orbs, state uint8, enemy map[int]uint8) *Game {
	t.Helper()
	m := NewMap(5, 5)
	m.Cells[12].Feature = Vault
	g, err := NewGameWithMap(m, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := g.Snapshot()
	s.Played = 3
	s.Count[12], s.Owner[12], s.Pressed[12] = orbs, 0, state
	s.Count[24], s.Owner[24] = 1, 0
	for i, n := range enemy {
		s.Count[i], s.Owner[i] = n, 1
	}
	if g, err = Restore(s); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestVaultShieldTakesTwoEnemyOrbsThenFalls(t *testing.T) {
	g := vaultGame(t, 1, 1|makerWorking, map[int]uint8{17: 3, 7: 3, 6: 3, 11: 3})
	mustApply(t, g, 0) // a: the working Vault raises its shield
	if Shield(g.Pressed[12]) != VaultShields || !MakerWorking(g.Pressed[12], 0) {
		t.Fatalf("state %d: a working Vault must raise a shield of %d", g.Pressed[12], VaultShields)
	}
	before := total(g)
	mustApply(t, g, 17) // b: one orb hits the shield and is lost
	checkTallies(t, g)
	if Shield(g.Pressed[12]) != 1 || g.Count[12] != 1 || g.Owner[12] != 0 || total(g) != before {
		t.Fatalf("shield %d, %d orbs of %d, %d orbs in all", Shield(g.Pressed[12]), g.Count[12], g.Owner[12], total(g))
	}
	mustApply(t, g, 4) // a: a damaged shield holds as it is
	if Shield(g.Pressed[12]) != 1 {
		t.Fatal("a damaged shield must not refill")
	}
	// b: 7 breaks the shield, its chain through 6 and 11 reaches the Vault again and takes it.
	mustApply(t, g, 7)
	checkTallies(t, g)
	if g.Owner[12] != 1 || g.Count[12] != 2 {
		t.Fatalf("a broken shield must let the next orb capture: %d orbs of %d", g.Count[12], g.Owner[12])
	}
	if g.Pressed[12] != 2 {
		t.Fatalf("state %d: the new holder skips a turn and holds no shield", g.Pressed[12])
	}
}

func TestVaultRaisesANewShieldAfterItBreaks(t *testing.T) {
	g := vaultGame(t, 1, 1|makerWorking|vaultShield, map[int]uint8{17: 3, 3: 1})
	mustApply(t, g, 0)
	if Shield(g.Pressed[12]) != 1 {
		t.Fatal("a damaged shield must not refill")
	}
	mustApply(t, g, 17) // b breaks it
	if Shield(g.Pressed[12]) != 0 || g.Owner[12] != 0 {
		t.Fatal("the last hit must break the shield and keep the Vault")
	}
	mustApply(t, g, 4) // a: a new shield
	if Shield(g.Pressed[12]) != VaultShields {
		t.Fatal("a broken shield must come back whole after its holder's next move")
	}
}

// Within a move, the shield guards the Vault's claim even while it stands empty, and Box shoves hit it like orbs.
func TestVaultShieldOutlastsAnEmptyVaultWithinAMove(t *testing.T) {
	g := vaultGame(t, 3, 1|makerWorking|2*vaultShield, map[int]uint8{7: 3})
	g.Count[7]++ // at capacity
	g.Orbs[1]++
	g.Orbs[0] -= 3
	g.Count[12], g.Owner[12] = 0, NoOwner // as if it had just popped
	g.cur = append(g.cur[:0], 7)
	g.explode()
	if g.Count[12] != 0 || Shield(g.Pressed[12]) != 1 {
		t.Fatalf("%d orbs, shield %d: the shield must still take the orb", g.Count[12], Shield(g.Pressed[12]))
	}
	g.gain(12, 1, 3)
	if g.Count[12] != 2 || g.Owner[12] != 1 || Shield(g.Pressed[12]) != 0 {
		t.Fatalf("%d orbs of %d, shield %d: a shoved stack loses what the shield takes", g.Count[12], g.Owner[12], Shield(g.Pressed[12]))
	}
}

// A raised shield stops a Box like a Wall, so it bounces back; a broken one lets it in, shoving the Vault's orbs on.
func TestVaultShieldBlocksBoxes(t *testing.T) {
	for _, shield := range []uint8{1, 0} {
		m := boxMap(24)
		m.Cells[31].Feature = Vault
		g := boxGame(t, m, func(g *Game) {
			seedMapCell(g, 31, 1, 1)
			g.Pressed[31] = 2 | makerWorking | shield*vaultShield
		}, 17)
		before := total(g)
		mustApply(t, g, 0)
		checkTallies(t, g)
		box, vault, at38 := 31, uint8(0), uint8(1)
		if shield != 0 {
			box, vault, at38 = 17, 1, 0
		}
		if g.Boxes[box]&BoxHere == 0 || g.Count[31] != vault || g.Count[38] != at38 || g.Count[24] != 1 || total(g) != before+1 {
			t.Fatalf("shield %d: Box at 17/24/31 %d/%d/%d, Vault %d orbs, 38 %d", shield, g.Boxes[17], g.Boxes[24], g.Boxes[31], g.Count[31], g.Count[38])
		}
	}
}

// Shields destroy orbs, so Vaults drain like Spillways (measured with TestPaceFit) and too many make a map Endless.
func TestVaultEstimate(t *testing.T) {
	m := NewMap(9, 9)
	m.Cells[40].Feature = Vault
	if _, length, _ := m.Estimate(); length != 1_000_000/965 {
		t.Fatalf("one Vault: length %d", length)
	}
	for i := 0; i < 36; i += 2 {
		m.Cells[i].Feature = Vault
	}
	issues := m.Validate()
	if _, _, pace := m.Estimate(); pace != Endless || len(issues) != 1 || len(issues[0].Cells) != 19 {
		t.Fatalf("pace %v, issues %v", pace, issues)
	}
}

func TestVaultSnapshotState(t *testing.T) {
	for _, tc := range []struct {
		feature Feature
		state   uint8
		ok      bool
	}{
		{Vault, 1 | makerWorking | 2*vaultShield, true},
		{Vault, 1 | makerWorking | 3*vaultShield, false},
		{Vault, 1 | vaultShield, false}, // only a working Vault has a shield
		{Furnace, 1 | makerWorking | vaultShield, false},
	} {
		m := NewMap(5, 5)
		m.Cells[12].Feature = tc.feature
		g, err := NewGameWithMap(m, 2)
		if err != nil {
			t.Fatal(err)
		}
		s := g.Snapshot()
		s.Played = 3
		s.Count[12], s.Owner[12], s.Pressed[12] = 1, 0, tc.state
		if _, err := Restore(s); (err == nil) != tc.ok {
			t.Fatalf("%v state %d: restore error %v", tc.feature, tc.state, err)
		}
	}
	m := NewMap(5, 5)
	m.Cells[0].Feature = Vault
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("a corner Vault needs no extra capacity: %v", issues)
	}
}
