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

// Sound plays the generated notes. Each note has one player, so at most 15 voices sound at once; replaying a
// note restarts it. Sounds closer together than minGap are dropped (prototype MIN_PLAY_INTERVAL).
type Sound struct {
	notes  [len(pentatonic)]*audio.Player
	last   time.Time
	Muted  bool
	Volume float64 // master volume, 0..1
}

// New creates the audio context and synthesizes every note once.
func New() *Sound {
	ctx := audio.NewContext(sampleRate)
	s := &Sound{}
	for i, midi := range pentatonic {
		s.notes[i] = ctx.NewPlayerFromBytes(synth(440 * math.Pow(2, float64(midi-69)/12)))
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
	return buf
}

// Place plays one note for an orb placed by player: each seat has its own pitch in the middle octave.
func (s *Sound) Place(player int) { s.chord(5+player%5, 1) }

// Wave plays a chord for the k-th explosion wave of a chain: 1 note for a single cell up to 5 notes for 16 or
// more, rooted higher as the chain goes on so long chains climb and wrap.
func (s *Sound) Wave(k, cells int) { s.chord(k%7, min(5, bits.Len(uint(cells)))) }

// chord plays n notes stacked two scale steps apart from root. A nil sound plays nothing (the Home demo board).
func (s *Sound) chord(root, n int) {
	if s == nil {
		return
	}
	now := time.Now()
	if s.Muted || s.Volume == 0 || now.Sub(s.last) < minGap {
		return
	}
	s.last = now
	vol := 0.55 / math.Sqrt(float64(n)) * s.Volume
	for i := range n {
		note := root + 2*i
		if note >= len(s.notes) {
			break
		}
		p := s.notes[note]
		p.SetVolume(vol)
		_ = p.Rewind() // only fails for a closed player
		p.Play()
	}
}
