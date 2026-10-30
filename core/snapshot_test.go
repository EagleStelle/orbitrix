package core

import (
	"slices"
	"testing"
)

func TestSnapshotRoundTrip(t *testing.T) {
	g, _ := NewGame(8, 14, 5)
	playRandom(t, g, 11, 150)
	s := g.Snapshot()
	r, err := Restore(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Hash() != g.Hash() || r.Orbs != g.Orbs {
		t.Fatal("restored game differs")
	}
	s.Count[0]++ // the snapshot must not share memory with the game
	if r.Hash() != g.Hash() {
		t.Fatal("snapshot aliases the game")
	}
	if !slices.Equal(playRandom(t, g, 5, 1_000_000), playRandom(t, r, 5, 1_000_000)) {
		t.Fatal("restored game played out differently")
	}
}

func TestRestoreRejectsBadSnapshots(t *testing.T) {
	valid := func() Snapshot {
		g, _ := NewGame(3, 3, 3)
		g.Apply(0)
		g.Apply(4)
		return g.Snapshot()
	}
	for name, mutate := range map[string]func(*Snapshot){
		"too small":           func(s *Snapshot) { s.Cols = 2 },
		"too many players":    func(s *Snapshot) { s.Players = 13 },
		"short counts":        func(s *Snapshot) { s.Count = s.Count[:8] },
		"short owners":        func(s *Snapshot) { s.Owner = s.Owner[:8] },
		"turn out of range":   func(s *Snapshot) { s.Turn = 3 },
		"negative turn":       func(s *Snapshot) { s.Turn = -1 },
		"negative moves":      func(s *Snapshot) { s.Moves = -1 },
		"played beyond":       func(s *Snapshot) { s.Played = 1 << 3 },
		"winner out of range": func(s *Snapshot) { s.Winner = 3 },
		"owner out of range":  func(s *Snapshot) { s.Owner[0] = 3 },
		"orbs with no owner":  func(s *Snapshot) { s.Owner[0] = NoOwner },
		"owner with no orbs":  func(s *Snapshot) { s.Owner[1] = 0 },
		"over twice capacity": func(s *Snapshot) { s.Count[0] = 4 },
	} {
		s := valid()
		mutate(&s)
		if _, err := Restore(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Restore(valid()); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}
}

func TestHash(t *testing.T) {
	g, _ := NewGame(9, 9, 2)
	h := g.Hash()
	// Pinned so a change to the hash input or order is deliberate: peers on different builds must agree.
	if h != goldenEmpty9x9Hash {
		t.Errorf("empty 9x9 hash %#x, want %#x", h, uint64(goldenEmpty9x9Hash))
	}
	g.Apply(40)
	if g.Hash() == h {
		t.Error("hash ignored a move")
	}
	if g.Clone().Hash() != g.Hash() {
		t.Error("equal games hash differently")
	}
}

const goldenEmpty9x9Hash = 0x5818acf0d8a0c295
