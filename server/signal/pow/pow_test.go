package pow

import (
	"errors"
	"testing"
	"time"
)

func TestChallenge(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	is := NewIssuer()
	c := is.Issue(12, 30*time.Second, now)
	n := Solve(c, 12)
	other := NewIssuer().Issue(12, 30*time.Second, now)
	tampered := []byte(c)
	tampered[3] ^= 1 // flips the nonce, so the signature no longer matches

	for _, tc := range []struct { // in order: the first use must come before the second
		name      string
		challenge string
		answer    uint64
		at        time.Time
		want      error
	}{
		{"forged by another key", other, Solve(other, 12), now, ErrForged},
		{"tampered", string(tampered), n, now, ErrForged},
		{"garbage", "nope", 0, now, ErrForged},
		{"expired", c, n, now.Add(30 * time.Second), ErrExpired},
		{"wrong answer", c, wrongAnswer(c, 12), now, ErrWrong},
		{"first use", c, n, now, nil},
		{"reused", c, n, now.Add(time.Second), ErrReused},
	} {
		if err := is.Check(tc.challenge, tc.answer, tc.at); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, err, tc.want)
		}
	}
}

func wrongAnswer(c string, bits int) uint64 {
	for n := uint64(0); ; n++ {
		if !Valid(c, bits, n) {
			return n
		}
	}
}

func TestZeroBitsNeedNoWork(t *testing.T) {
	c := NewIssuer().Issue(0, time.Second, time.Now())
	if Solve(c, 0) != 0 {
		t.Fatal("difficulty 0 should accept the first answer")
	}
}

func TestUsedCacheIsBounded(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	is := NewIssuer()
	for k := 0; len(is.used) < MaxUsed; k++ {
		is.used[[16]byte{byte(k), byte(k >> 8), byte(k >> 16)}] = now.Unix() + 10
	}
	c := is.Issue(0, 30*time.Second, now)
	if err := is.Check(c, 0, now); !errors.Is(err, ErrBusy) {
		t.Fatalf("full cache: %v", err)
	}
	if err := is.Check(c, 0, now.Add(20*time.Second)); err != nil || len(is.used) != 1 {
		t.Fatalf("after the old entries expired: %v, %d cached", err, len(is.used))
	}
}

func BenchmarkSolve16(b *testing.B) {
	is := NewIssuer()
	for i := 0; i < b.N; i++ {
		Solve(is.Issue(16, time.Minute, time.Now()), 16)
	}
}
