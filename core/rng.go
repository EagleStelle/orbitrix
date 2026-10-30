package core

import "math/bits"

// RNG is a seeded splitmix64 generator: small, fast and bit-identical on every platform. All game randomness
// goes through it, never through math/rand's global source. The zero value is a valid generator with seed 0.
type RNG struct{ state uint64 }

// NewRNG returns a generator seeded with seed.
func NewRNG(seed uint64) RNG { return RNG{state: seed} }

// Uint64 returns the next 64 random bits.
func (r *RNG) Uint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// Intn returns a uniform int in [0, n). It panics if n <= 0.
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		panic("core: RNG.Intn called with n <= 0")
	}
	// Lemire's multiply-and-reject: unbiased without a division in the common case.
	un := uint64(n)
	hi, lo := bits.Mul64(r.Uint64(), un)
	if lo < un {
		for threshold := -un % un; lo < threshold; {
			hi, lo = bits.Mul64(r.Uint64(), un)
		}
	}
	return int(hi)
}

// Float64 returns a uniform float64 in [0, 1).
func (r *RNG) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

// Shuffle permutes n elements with the Fisher–Yates algorithm, calling swap to exchange elements i and j.
func (r *RNG) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		swap(i, r.Intn(i+1))
	}
}
