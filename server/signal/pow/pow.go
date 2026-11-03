// Package pow is signaling's proof of work. The server issues an HMAC-signed, single-use challenge, and the client
// must find a number whose SHA-256 together with the challenge starts with the challenge's number of zero bits.
// Server and every client (native and web) import this same package.
package pow

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/bits"
	"strings"
	"sync"
	"time"
)

// MaxUsed bounds the cache of answered challenges kept until they expire.
const MaxUsed = 100_000

var (
	ErrForged  = errors.New("pow: challenge not issued here")
	ErrExpired = errors.New("pow: challenge expired")
	ErrWrong   = errors.New("pow: wrong answer")
	ErrReused  = errors.New("pow: challenge already answered")
	ErrBusy    = errors.New("pow: too many open challenges")
)

var b64 = base64.RawURLEncoding

// body is a challenge's signed part: a random nonce, the expiry and the difficulty.
type body struct {
	Nonce  [16]byte
	Expiry int64 // Unix seconds
	Bits   uint8
}

const bodyLen = 16 + 8 + 1

// Issuer signs and checks challenges. Its key lives only in memory, so a restart invalidates open challenges.
type Issuer struct {
	key  [32]byte
	mu   sync.Mutex
	used map[[16]byte]int64 // nonce → expiry
}

// NewIssuer returns an issuer with a fresh random key.
func NewIssuer() *Issuer {
	i := &Issuer{used: map[[16]byte]int64{}}
	_, _ = rand.Read(i.key[:]) // never fails
	return i
}

// Issue returns a challenge of the given difficulty that expires after ttl.
func (i *Issuer) Issue(zeroBits int, ttl time.Duration, now time.Time) string {
	var raw [bodyLen]byte
	_, _ = rand.Read(raw[:16])
	binary.BigEndian.PutUint64(raw[16:24], uint64(now.Add(ttl).Unix()))
	raw[24] = uint8(zeroBits)
	return b64.EncodeToString(raw[:]) + "." + b64.EncodeToString(i.sign(raw[:]))
}

// Check accepts answer for challenge once: the challenge must be one this issuer signed, unexpired and unanswered.
func (i *Issuer) Check(challenge string, answer uint64, now time.Time) error {
	b, ok := i.parse(challenge)
	switch {
	case !ok:
		return ErrForged
	case now.Unix() >= b.Expiry:
		return ErrExpired
	case !Valid(challenge, int(b.Bits), answer):
		return ErrWrong
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, seen := i.used[b.Nonce]; seen {
		return ErrReused
	}
	if len(i.used) >= MaxUsed {
		for n, exp := range i.used {
			if now.Unix() >= exp {
				delete(i.used, n)
			}
		}
		if len(i.used) >= MaxUsed {
			return ErrBusy
		}
	}
	i.used[b.Nonce] = b.Expiry
	return nil
}

func (i *Issuer) sign(raw []byte) []byte {
	m := hmac.New(sha256.New, i.key[:])
	m.Write(raw)
	return m.Sum(nil)
}

func (i *Issuer) parse(challenge string) (body, bool) {
	var b body
	data, sig, ok := strings.Cut(challenge, ".")
	raw, err1 := b64.DecodeString(data)
	mac, err2 := b64.DecodeString(sig)
	if !ok || err1 != nil || err2 != nil || len(raw) != bodyLen || !hmac.Equal(mac, i.sign(raw)) {
		return b, false
	}
	copy(b.Nonce[:], raw)
	b.Expiry, b.Bits = int64(binary.BigEndian.Uint64(raw[16:24])), raw[24]
	return b, true
}

// Solve finds the smallest answer for challenge. Each extra bit doubles the expected work.
func Solve(challenge string, zeroBits int) uint64 {
	buf := append([]byte(challenge), make([]byte, 8)...)
	for n := uint64(0); ; n++ {
		if lead(buf, n, zeroBits) {
			return n
		}
	}
}

// Valid reports whether answer solves challenge at the given difficulty.
func Valid(challenge string, zeroBits int, answer uint64) bool {
	return lead(append([]byte(challenge), make([]byte, 8)...), answer, zeroBits)
}

// lead reports whether SHA-256(challenge ‖ n as 8 big-endian bytes) starts with zeroBits zero bits; buf holds the
// challenge followed by 8 spare bytes.
func lead(buf []byte, n uint64, zeroBits int) bool {
	binary.BigEndian.PutUint64(buf[len(buf)-8:], n)
	sum := sha256.Sum256(buf)
	for k := 0; k < len(sum) && zeroBits > 0; k += 8 {
		word := binary.BigEndian.Uint64(sum[k:])
		if z := bits.LeadingZeros64(word); z < min(zeroBits, 64) {
			return false
		}
		zeroBits -= 64
	}
	return true
}
