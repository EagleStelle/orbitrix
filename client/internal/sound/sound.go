// Package sound synthesizes and plays the client's pentatonic notes.
package sound

import (
	"math"
	"math/bits"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	sampleRate = 44100
	noteLength = 0.8 // seconds
	minGap     = 100 * time.Millisecond
)

// pentatonic is C major pentatonic (C D E G A) over three octaves from C4, as MIDI note numbers.
var pentatonic = [...]int{60, 62, 64, 67, 69, 72, 74, 76, 79, 81, 84, 86, 88, 91, 93}

type player interface {
	SetVolume(float64)
	Rewind() error
	Play()
	Pause()
	IsPlaying() bool
}

// Sound shares a 15-voice limit and a global minimum gap across notes and map effects.
type Sound struct {
	notes   [len(pentatonic)]player
	effects [len(effectTones)]player
	voices  [len(pentatonic)]player
	used    [len(pentatonic)]uint64
	serial  uint64
	last    time.Time
	pending Effects
	Muted   bool
	Volume  float64 // master volume, 0..1
}

// New creates the audio context and synthesizes every note once.
func New() *Sound {
	ctx := audio.NewContext(sampleRate)
	s := &Sound{}
	for i, midi := range pentatonic {
		s.notes[i] = ctx.NewPlayerFromBytes(synth(440 * math.Pow(2, float64(midi-69)/12)))
	}
	for i, tone := range effectTones {
		s.effects[i] = ctx.NewPlayerFromBytes(synthEffect(tone))
	}
	return s
}

// synth renders one note as 16-bit stereo PCM: a sine with soft 2nd and 3rd harmonics, a 6 ms attack and an
// exponential decay, faded out over the last 20 ms so it never clicks.
func synth(freq float64) []byte {
	n := int(noteLength * sampleRate)
	buf := make([]byte, 4*n)
	w := 2 * math.Pi * freq / sampleRate
	for i := range n {
		t := float64(i) / sampleRate
		env := min(1, t/0.006) * math.Exp(-t/0.22) * min(1, float64(n-i)/(0.02*sampleRate))
		v := (math.Sin(w*float64(i)) + 0.25*math.Sin(2*w*float64(i)) + 0.06*math.Sin(3*w*float64(i))) / 1.31
		x := uint16(int16(v * env * 0.6 * math.MaxInt16))
		buf[4*i], buf[4*i+1], buf[4*i+2], buf[4*i+3] = byte(x), byte(x>>8), byte(x), byte(x>>8)
	}
	return normalize(buf)
}

// Place plays one note for an orb placed by player: each seat has its own pitch in the middle octave.
func (s *Sound) Place(player int, effects Effects) { s.chord(5+player%5, 1, effects) }

// Wave plays a chord for the k-th explosion wave of a chain: 1 note for a single cell up to 5 notes for 16 or
// more, rooted higher as the chain goes on so long chains climb and wrap.
func (s *Sound) Wave(k, cells int, effects Effects) {
	s.chord(k%7, min(5, bits.Len(uint(cells))), effects)
}

// Effect plays terrain changes when there is no following explosion wave.
func (s *Sound) Effect(effects Effects) { s.chord(0, 0, effects) }

// Update drains coalesced map cues after the minimum gap, including the last wave's effects.
func (s *Sound) Update() {
	if s != nil && s.pending != 0 {
		s.chord(0, 0, 0)
	}
}

// chord plays n notes stacked two scale steps apart from root. A nil sound plays nothing (the Home demo board).
func (s *Sound) chord(root, n int, effects Effects) {
	if s == nil {
		return
	}
	if s.Muted || s.Volume == 0 {
		s.pending = 0
		return
	}
	effects |= s.pending
	if n == 0 && effects == 0 {
		return
	}
	now := time.Now()
	if now.Sub(s.last) < minGap {
		s.pending = effects
		return
	}
	s.pending = 0
	s.last = now
	count := bits.OnesCount16(uint16(effects))
	noteVolume := 0.55 / math.Sqrt(float64(max(1, n))) * s.Volume
	effectVolume := 0.55 / math.Sqrt(float64(max(1, count))) * s.Volume
	// Split power equally between orbs and map cues so larger chords cannot bury a single effect.
	if n > 0 && count > 0 {
		noteVolume *= math.Sqrt(0.5)
		effectVolume *= math.Sqrt(0.5)
	}
	for i := range n {
		note := root + 2*i
		if note >= len(s.notes) {
			break
		}
		s.play(s.notes[note], noteVolume)
	}
	for i, p := range s.effects {
		if effects&(1<<i) != 0 {
			s.play(p, effectVolume)
		}
	}
}

func (s *Sound) play(p player, volume float64) {
	slot := -1
	for i, active := range s.voices {
		if active == p {
			slot = i
			break
		}
	}
	if slot < 0 {
		slot = 0
		for i, active := range s.voices {
			if active == nil || !active.IsPlaying() {
				slot = i
				break
			}
			if s.used[i] < s.used[slot] {
				slot = i
			}
		}
		if old := s.voices[slot]; old != nil {
			old.Pause()
		}
		s.voices[slot] = p
	}
	s.serial++
	s.used[slot] = s.serial
	p.SetVolume(volume)
	_ = p.Rewind() // only fails for a closed player
	p.Play()
}
