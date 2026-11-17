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
	beat       = 100 * time.Millisecond
	burstGain  = 0.8 // a whole burst at full volume, whatever it holds
)

// pentatonic is C major pentatonic (C D E G A) over three octaves from C4, as MIDI note numbers.
var pentatonic = [...]int{60, 62, 64, 67, 69, 72, 74, 76, 79, 81, 84, 86, 88, 91, 93}

// chords is the song every sound follows, a looped progression: each chord's nine tones in pentatonic, low to high.
var chords = [...][9]int{
	{0, 2, 3, 5, 7, 8, 10, 12, 13}, // C: C E G
	{0, 2, 4, 5, 7, 9, 10, 12, 14}, // Am: C E A
	{1, 3, 4, 6, 8, 9, 11, 13, 14}, // Dsus4: D G A
	{0, 1, 3, 5, 6, 8, 10, 11, 13}, // Gsus4: C D G
}

// roots are the placement notes, the progression's bass line C5 A4 D5 G4.
var roots = [len(chords)]int{5, 4, 6, 3}

type player interface {
	SetVolume(float64)
	Rewind() error
	Play()
	Pause()
	IsPlaying() bool
}

// burst is what sounds together on one beat.
type burst struct {
	effects Effects
	cells   int  // exploding cells of every wave merged into the beat
	placed  bool // a placement note
	closing bool // a chain's last wave is among them
}

// Sound shares a 15-voice limit and one beat across notes and map effects.
type Sound struct {
	notes    [len(pentatonic)]player
	effects  [len(chords)][len(effectTones)]player
	knockout player
	voices   [len(pentatonic)]player
	used     [len(pentatonic)]uint64
	serial   uint64
	last     time.Time
	wait     burst // held for the next beat
	step     int   // note bursts played: the song's place in chords
	lift     int   // waves played in this chain, raising the voicing
	Muted    bool
	Volume   float64 // master volume, 0..1
}

// New creates the audio context and synthesizes every note once.
func New() *Sound {
	ctx := audio.NewContext(sampleRate)
	s := &Sound{}
	for i := range pentatonic {
		s.notes[i] = ctx.NewPlayerFromBytes(synth(hz(i)))
	}
	for chord := range chords {
		for i, tone := range effectTones {
			s.effects[chord][i] = ctx.NewPlayerFromBytes(synthEffect(tone, chord))
		}
	}
	s.knockout = ctx.NewPlayerFromBytes(synthKnockout())
	return s
}

// hz is the frequency of a pentatonic note.
func hz(note int) float64 { return 440 * math.Pow(2, float64(pentatonic[note]-69)/12) }

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

// Place plays the next chord's root for a placed orb, so a match's moves walk the bass line.
func (s *Sound) Place(effects Effects) { s.queue(burst{effects: effects, placed: true}) }

// Wave plays the next chord for an explosion wave of a chain: 1 note for a single cell up to 5 notes for 16 or
// more, voiced higher as the chain goes on. A chain's last wave lands on C.
func (s *Sound) Wave(cells int, last bool, effects Effects) {
	s.queue(burst{effects: effects, cells: cells, closing: last})
}

// Effect plays terrain changes when there is no following explosion wave.
func (s *Sound) Effect(effects Effects) { s.queue(burst{effects: effects}) }

// Update plays what waits for the beat, including the last wave's.
func (s *Sound) Update() { s.queue(burst{}) }

// queue merges b into the waiting burst and plays it on the beat: at once after a pause, else on the next 100 ms.
// A nil sound plays nothing (the Home demo board).
func (s *Sound) queue(b burst) {
	if s == nil {
		return
	}
	if s.Muted || s.Volume == 0 {
		s.wait = burst{}
		return
	}
	waited := s.wait != burst{}
	b.effects, b.cells = b.effects|s.wait.effects, b.cells+s.wait.cells
	b.placed, b.closing = b.placed || s.wait.placed, b.closing || s.wait.closing
	if b == (burst{}) {
		return
	}
	now := time.Now()
	if now.Sub(s.last) < beat {
		s.wait = b
		return
	}
	s.wait = burst{}
	if s.last = s.last.Add(beat); !waited || now.Sub(s.last) >= beat {
		s.last = now // only a burst that waited keeps the running pulse even
	}
	if b.placed {
		s.lift = 0
	}
	count := bits.OnesCount16(uint16(b.effects))
	chord, n := max(s.step-1, 0)%len(chords), 0 // cues alone join the chord still ringing
	if b.cells > 0 || b.placed {
		if b.closing {
			s.step = 0
		}
		chord, n = s.step%len(chords), 1
		s.step++
	}
	if b.cells > 0 {
		n = min(5, bits.Len(uint(b.cells)))
	}
	n = min(n, len(s.voices)-count) // a burst of every cue gives up chord notes rather than steal its own voices
	noteVolume := burstGain / math.Sqrt(float64(max(1, n))) * s.Volume
	effectVolume := burstGain / math.Sqrt(float64(max(1, count))) * s.Volume
	// Split power equally between orbs and map cues so larger chords cannot bury a single effect.
	if n > 0 && count > 0 {
		noteVolume *= math.Sqrt(0.5)
		effectVolume *= math.Sqrt(0.5)
	}
	if b.cells > 0 {
		for i := range n {
			s.play(s.notes[chords[chord][s.lift%5+i]], noteVolume)
		}
		s.lift++
	} else if n > 0 {
		s.play(s.notes[roots[chord]], noteVolume)
	}
	for i, p := range s.effects[chord] {
		if b.effects&(1<<i) != 0 {
			s.play(p, effectVolume)
		}
	}
	if b.effects&Knockout != 0 {
		s.play(s.knockout, effectVolume)
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
