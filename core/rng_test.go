package core

import (
	"slices"
	"testing"
)

func TestRNGKnownAnswers(t *testing.T) {
	// Reference splitmix64 output for seed 0.
	r := NewRNG(0)
	for _, want := range []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f} {
		if got := r.Uint64(); got != want {
			t.Fatalf("got %#x, want %#x", got, want)
		}
	}
}

func TestRNGRanges(t *testing.T) {
	r := NewRNG(42)
	var hits [7]int
	for i := 0; i < 70_000; i++ {
		hits[r.Intn(7)]++
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 %v out of [0, 1)", f)
		}
	}
	for v, n := range hits {
		if n < 9_000 || n > 11_000 {
			t.Errorf("Intn(7) returned %d %d times in 70000", v, n)
		}
	}

	s := []int{0, 1, 2, 3, 4, 5, 6, 7}
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	if sorted := slices.Sorted(slices.Values(s)); !slices.Equal(sorted, []int{0, 1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("Shuffle lost elements: %v", s)
	}
}
