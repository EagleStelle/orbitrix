package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"testing"
	"time"
)

type testPlayer struct {
	playing bool
	volume  float64
	plays   int
	group   *[]*testPlayer
	peak    *int
}

func (p *testPlayer) SetVolume(v float64) { p.volume = v }
func (p *testPlayer) Rewind() error       { return nil }
func (p *testPlayer) Pause()              { p.playing = false }
func (p *testPlayer) IsPlaying() bool     { return p.playing }
func (p *testPlayer) Play() {
	p.playing = true
	p.plays++
	n := 0
	for _, other := range *p.group {
		if other.playing {
			n++
		}
	}
	*p.peak = max(*p.peak, n)
}

func testSound() (*Sound, []*testPlayer, *int) {
	s := &Sound{Volume: 1}
	var group []*testPlayer
	peak := new(int)
	newPlayer := func() *testPlayer {
		p := &testPlayer{group: &group, peak: peak}
		group = append(group, p)
		return p
	}
	for i := range s.notes {
		s.notes[i] = newPlayer()
	}
	for i := range s.effects {
		s.effects[i] = newPlayer()
	}
	return s, group, peak
}

func TestEffectsShareBurstAndRateLimit(t *testing.T) {
	s, _, _ := testSound()
	s.Place(0, Latch|HoldOn)
	for i, p := range []player{s.notes[5], s.effects[1], s.effects[4]} {
		if !p.IsPlaying() {
			t.Fatal("placement suppressed a simultaneous switch cue")
		}
		want := .55 / math.Sqrt(2)
		if i > 0 {
			want = .55 / 2
		}
		if got := p.(*testPlayer).volume; math.Abs(got-want) > 1e-9 {
			t.Fatalf("unbalanced burst volume: %v", got)
		}
	}
	s.Wave(0, 1, Link)
	if s.effects[0].IsPlaying() {
		t.Fatal("map effects bypassed the global minimum gap")
	}
	s.last = time.Now().Add(-minGap)
	s.Wave(0, 1, Link)
	if !s.effects[0].IsPlaying() || !s.notes[0].IsPlaying() {
		t.Fatal("wave and Link must sound in the same accepted burst")
	}
	s.last = time.Now().Add(-minGap)
	s.Effect(WallBreak)
	if !s.effects[7].IsPlaying() {
		t.Fatal("final-wave terrain effect needs no following explosion")
	}
}

func TestRateLimitedMapCuesDrainAfterFinalWave(t *testing.T) {
	s, _, _ := testSound()
	s.Place(0, 0)
	s.Effect(WallBreak)
	s.Effect(HoldOff | WallBreak)
	s.Update()
	if s.effects[7].IsPlaying() || s.pending != WallBreak|HoldOff {
		t.Fatal("rate-limited final-wave cues must wait and coalesce")
	}
	s.last = time.Now().Add(-minGap)
	s.Update()
	if s.pending != 0 || !s.effects[7].IsPlaying() || !s.effects[5].IsPlaying() {
		t.Fatal("final-wave cues were dropped instead of drained")
	}
	s.Update()
	if s.effects[7].(*testPlayer).plays != 1 {
		t.Fatal("a coalesced wall break played more than once")
	}
	for _, muted := range []bool{true, false} {
		s.pending = Link
		s.Muted = muted
		if !muted {
			s.Volume = 0
		}
		s.Update()
		if s.pending != 0 || s.effects[0].IsPlaying() {
			t.Fatal("muting or zero volume retained a stale cue")
		}
	}
	var demo *Sound
	demo.Update()
}

func TestLargeChordsDoNotBuryMapCues(t *testing.T) {
	for _, cells := range []int{1, 16} {
		s, _, _ := testSound()
		s.Wave(0, cells, WallBreak)
		got := s.effects[7].(*testPlayer).volume
		if math.Abs(got-.55/math.Sqrt(2)) > 1e-9 {
			t.Fatalf("%d exploding cells attenuated the map cue to %v", cells, got)
		}
	}
}

func TestSoundMuteSilenceAndNil(t *testing.T) {
	for _, tc := range []struct {
		muted  bool
		volume float64
	}{{true, 1}, {false, 0}} {
		s, group, _ := testSound()
		s.Muted, s.Volume = tc.muted, tc.volume
		s.Place(0, Latch)
		s.Wave(0, 16, Link|WallBreak)
		s.Effect(HoldOff)
		for _, p := range group {
			if p.plays != 0 {
				t.Fatal("muted or zero-volume sound played")
			}
		}
		if !s.last.IsZero() {
			t.Fatal("silence must not consume the rate limit")
		}
	}
	s, _, _ := testSound()
	s.Effect(0)
	if !s.last.IsZero() {
		t.Fatal("an empty terrain batch consumed the rate limit")
	}
	var demo *Sound
	demo.Place(0, Latch)
	demo.Wave(0, 4, Link)
	demo.Effect(WallHit)
}

func TestCombinedSoundsKeepFifteenVoiceLimit(t *testing.T) {
	s, _, peak := testSound()
	for k := range 30 {
		s.last = time.Now().Add(-minGap)
		s.Wave(k, 16, Link|Latch|ToggleOn|ToggleOff|HoldOn|HoldOff|WallHit|WallBreak|Fall|LandfillFull)
		if *peak > 15 {
			t.Fatalf("simultaneous voices reached %d", *peak)
		}
		for i := range 5 {
			if !s.notes[k%7+2*i].IsPlaying() {
				t.Fatal("voice stealing cut a note from the current burst")
			}
		}
		for _, p := range s.effects {
			if !p.IsPlaying() {
				t.Fatal("voice stealing cut an effect from the current burst")
			}
		}
		for i, p := range s.voices {
			for j := i + 1; j < len(s.voices); j++ {
				if p != nil && p == s.voices[j] {
					t.Fatal("replaying a cue occupied two voice slots")
				}
			}
		}
	}
}

func TestEffectPCMIsTunedDistinctAndFaded(t *testing.T) {
	var rendered [][]byte
	for effect, tone := range effectTones {
		for _, index := range tone.notes {
			if index < 0 {
				continue
			}
			if index >= len(pentatonic) {
				t.Fatalf("effect %d leaves the C4–A6 pentatonic range", effect)
			}
		}
		pcm := synthEffect(tone)
		if len(pcm) != 4*int(tone.length*sampleRate) {
			t.Fatalf("effect %d has an invalid stereo PCM length", effect)
		}
		peak := 0
		for i := 0; i < len(pcm); i += 4 {
			left := int(int16(binary.LittleEndian.Uint16(pcm[i:])))
			right := int(int16(binary.LittleEndian.Uint16(pcm[i+2:])))
			if left != right {
				t.Fatal("stereo channels differ")
			}
			peak = max(peak, int(math.Abs(float64(left))))
		}
		if peak < 1000 || peak > 23000 {
			t.Fatalf("effect %d is silent or clips: peak %d", effect, peak)
		}
		if !bytes.Equal(pcm[:4], make([]byte, 4)) || !bytes.Equal(pcm[len(pcm)-4:], make([]byte, 4)) {
			t.Fatalf("effect %d lacks silent attack/release endpoints", effect)
		}
		for _, previous := range rendered {
			if bytes.Equal(pcm, previous) {
				t.Fatalf("effect %d duplicates another cue", effect)
			}
		}
		rendered = append(rendered, pcm)
	}
}

// Measure short active windows so a cue's silent tail cannot skew its loudness.
func pcmLevels(pcm []byte) (rms, peak float64) {
	window := min(len(pcm)/4, sampleRate/20)
	for offset := 0; offset < len(pcm); offset += 4 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[offset:]))) / math.MaxInt16
		peak = max(peak, math.Abs(v))
	}
	for start := 0; start+window <= len(pcm)/4; start += max(1, window/10) {
		power := 0.0
		for frame := start; frame < start+window; frame++ {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[frame*4:]))) / math.MaxInt16
			power += v * v
		}
		rms = max(rms, math.Sqrt(power/float64(window)))
	}
	return
}

func TestGeneratedCuesHaveConsistentLoudness(t *testing.T) {
	check := func(t *testing.T, pcm []byte) {
		t.Helper()
		rms, peak := pcmLevels(pcm)
		t.Logf("active RMS %.1f dBFS, peak %.1f dBFS", 20*math.Log10(rms), 20*math.Log10(peak))
		if rms < .16 || rms > .19 {
			t.Errorf("active RMS %.3f is outside the shared loudness range", rms)
		}
		if peak > .66 {
			t.Errorf("peak %.3f leaves insufficient mix headroom", peak)
		}
	}
	for _, midi := range pentatonic {
		t.Run("orb-"+strconv.Itoa(midi), func(t *testing.T) {
			check(t, synth(440*math.Pow(2, float64(midi-69)/12)))
		})
	}
	for i, name := range []string{"link", "latch", "toggle-on", "toggle-off", "hold-on", "hold-off", "wall-hit", "wall-break", "fall"} {
		t.Run(name, func(t *testing.T) { check(t, synthEffect(effectTones[i])) })
	}
}

func TestNormalizeSilenceAndTransientHeadroom(t *testing.T) {
	if normalize(nil) != nil {
		t.Fatal("empty PCM changed")
	}
	silence := make([]byte, sampleRate/20*4)
	normalize(silence)
	if rms, peak := pcmLevels(silence); rms != 0 || peak != 0 {
		t.Fatal("normalizing silence introduced noise")
	}
	impulse := make([]byte, sampleRate/20*4)
	binary.LittleEndian.PutUint16(impulse, math.MaxInt16)
	binary.LittleEndian.PutUint16(impulse[2:], math.MaxInt16)
	normalize(impulse)
	if _, peak := pcmLevels(impulse); peak > .651 || peak < .649 {
		t.Fatalf("transient peak %v did not respect headroom", peak)
	}
}

func TestDenseNormalizedBurstsKeepMixHeadroom(t *testing.T) {
	var pcm [][]byte
	for _, midi := range pentatonic {
		pcm = append(pcm, synth(440*math.Pow(2, float64(midi-69)/12)))
	}
	for _, tone := range effectTones {
		pcm = append(pcm, synthEffect(tone))
	}
	for _, cells := range []int{1, 16} {
		for k := range 7 {
			s, players, _ := testSound()
			s.Wave(k, cells, Link|Latch|ToggleOn|ToggleOff|HoldOn|HoldOff|WallHit|WallBreak|Fall|LandfillFull)
			for frame := range int(noteLength * sampleRate) {
				mix := 0.0
				for i, p := range players {
					if p.playing && 4*frame < len(pcm[i]) {
						mix += float64(int16(binary.LittleEndian.Uint16(pcm[i][4*frame:]))) / math.MaxInt16 * p.volume
					}
				}
				if math.Abs(mix) > .9 {
					t.Fatalf("%d cells, wave %d: mix peak %v risks clipping", cells, k, mix)
				}
			}
		}
	}
}
