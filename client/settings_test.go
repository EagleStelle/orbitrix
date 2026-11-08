package client

import (
	"testing"

	"orbitrix/core"
)

func TestSeats(t *testing.T) {
	rng := core.NewRNG(1)
	s := &settings{Players: 12}
	s.shuffleSeats(&rng)
	if len(s.seats) != 12 || s.seats[0] != seatHuman {
		t.Fatalf("shuffleSeats: %v, want 12 seats with player 1 human", s.seats)
	}
	for _, opt := range s.seats[1:] {
		if opt == seatHuman {
			t.Fatalf("shuffleSeats: %v has a second human", s.seats)
		}
	}
	s.Players = 3
	s.fitSeats()
	if len(s.seats) != 3 {
		t.Fatalf("fitSeats to 3: %v", s.seats)
	}
	for opt := range seatLabels {
		if got, ok := parseSeat(seatName(opt)); !ok || got != opt {
			t.Errorf("seat %d round-trips to %d, %v", opt, got, ok)
		}
	}
}

func TestSelectedMapIndexRequiresUniqueName(t *testing.T) {
	m := core.NewMap(7, 7)
	m.Name = "Doritos"
	other := m.Clone()
	other.Name = "Other map"
	for _, tc := range []struct {
		name     string
		selected *core.MapDefinition
		maps     []core.MapDefinition
		want     int
	}{
		{"classic", nil, []core.MapDefinition{m}, -1},
		{"missing", &m, []core.MapDefinition{other}, -1},
		{"unique", &m, []core.MapDefinition{other, m}, 1},
		{"ambiguous", &m, []core.MapDefinition{m, m.Clone()}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &settings{Map: tc.selected}
			if got := s.selectedMapIndex(tc.maps); got != tc.want {
				t.Fatalf("selected map = %d, want %d", got, tc.want)
			}
		})
	}
}
