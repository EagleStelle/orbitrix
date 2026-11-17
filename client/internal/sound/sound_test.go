package sound

import (
	"bytes"
	"encoding/binary"
	"math"
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

// testSound's players are the notes, then each chord's effects, then Knockout.
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
	for chord := range s.effects {
		for i := range s.effects[chord] {
			s.effects[chord][i] = newPlayer()
		}
	}
	s.knockout = newPlayer()
	return s, group, peak
}

// testPCM renders every buffer in testSound's player order.
func testPCM() (pcm [][]byte) {
	for i := range pentatonic {
		pcm = append(pcm, synth(hz(i)))
	}
	for chord := range chords {
		for _, tone := range effectTones {
			pcm = append(pcm, synthEffect(tone, chord))
		}
	}
	return append(pcm, synthKnockout())
}

// heard reports the notes sounding, then silences every player and lets the beat pass.
func heard(s *Sound, group []*testPlayer) (notes []int) {
	for i, p := range s.notes {
		if p.IsPlaying() {
			notes = append(notes, i)
		}
	}
	for _, p := range group {
		p.playing = false
	}
	s.last = time.Now().Add(-beat)
	return
}

func sameNotes(got []int, want ...int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSongWalksProgression(t *testing.T) {
	s, group, _ := testSound()
	for move := range 2 * len(chords) {
		s.Place(0)
		if got := heard(s, group); !sameNotes(got, roots[move%len(chords)]) {
			t.Fatalf("move %d played %v, not the next chord's root", move, got)
		}
	}
	// A chain goes on through the chords, voiced one tone higher each wave, and its last wave lands on C.
	s.Place(Latch)
	if !s.effects[0][1].IsPlaying() {
		t.Fatal("a placement's cue left the placement's chord")
	}
	heard(s, group)
	s.Wave(2, false, 0)
	if got := heard(s, group); !sameNotes(got, chords[1][0], chords[1][1]) {
		t.Fatalf("first wave played %v", got)
	}
	s.Wave(16, false, Link)
	if got := heard(s, group); !sameNotes(got, chords[2][1:6]...) {
		t.Fatalf("second wave played %v", got)
	}
	if s.effects[2][0].(*testPlayer).plays != 1 {
		t.Fatal("a wave's cue left the wave's chord")
	}
	s.Wave(1, true, 0)
	if got := heard(s, group); !sameNotes(got, chords[0][2]) {
		t.Fatalf("last wave played %v, not C", got)
	}
	s.Effect(Fall)
	if s.effects[0][8].(*testPlayer).plays != 1 {
		t.Fatal("a cue alone left the chord still ringing")
	}
	heard(s, group)
	s.Place(0)
	if got := heard(s, group); !sameNotes(got, roots[1]) {
		t.Fatalf("the move after a chain played %v, not the chord after C", got)
	}
	for wave := range 12 { // a long chain's voicing wraps inside the chord
		s.Wave(16, false, 0)
		chord := chords[(2+wave)%len(chords)]
		if got := heard(s, group); !sameNotes(got, chord[wave%5:wave%5+5]...) {
			t.Fatalf("wave %d played %v", wave, got)
		}
	}
}

func TestFastWavesMergeOnTheBeat(t *testing.T) {
	s, group, _ := testSound()
	s.Place(0)
	start := s.last
	s.Wave(1, false, Link)
	s.Wave(3, false, WallHit)
	s.Wave(4, true, WallHit)
	s.Update()
	if got := heard(s, group); !sameNotes(got, roots[0]) || s.effects[0][0].(*testPlayer).plays != 0 {
		t.Fatalf("sounds inside one beat did not wait: %v", got)
	}
	s.last = start.Add(-beat - time.Millisecond) // a frame late
	s.Update()
	if !s.last.Equal(start.Add(-time.Millisecond)) {
		t.Fatal("a late frame shifted the pulse")
	}
	// 8 cells in all make 4 notes; holding the chain's last wave, the beat lands on C.
	if got := heard(s, group); !sameNotes(got, chords[0][:4]...) {
		t.Fatalf("merged waves played %v", got)
	}
	if s.effects[0][0].(*testPlayer).plays != 1 || s.effects[0][6].(*testPlayer).plays != 1 {
		t.Fatal("merged cues must each play once")
	}
	s.Update()
	if len(heard(s, group)) != 0 || s.wait != (burst{}) {
		t.Fatal("a played beat sounded again")
	}
	s.last = time.Now().Add(-3 * beat)
	s.Place(0)
	if time.Since(s.last) > beat/2 {
		t.Fatal("a sound after a pause must start its own pulse")
	}
}

func TestBurstsShareOneLevel(t *testing.T) {
	s, _, _ := testSound()
	s.Place(Latch | HoldOn)
	for i, p := range []player{s.notes[roots[0]], s.effects[0][1], s.effects[0][4]} {
		if !p.IsPlaying() {
			t.Fatal("placement suppressed a simultaneous switch cue")
		}
		want := burstGain / math.Sqrt(2)
		if i > 0 {
			want = burstGain / 2
		}
		if got := p.(*testPlayer).volume; math.Abs(got-want) > 1e-9 {
			t.Fatalf("unbalanced burst volume: %v", got)
		}
	}
	for _, cells := range []int{1, 16} {
		s, group, _ := testSound()
		s.Wave(cells, false, WallBreak|Knockout)
		power := 0.0
		for _, p := range group {
			power += p.volume * p.volume
		}
		if math.Abs(power-burstGain*burstGain) > 1e-9 {
			t.Fatalf("%d exploding cells changed the burst's level to %v", cells, math.Sqrt(power))
		}
		for _, p := range []player{s.effects[0][7], s.knockout} {
			if got := p.(*testPlayer).volume; math.Abs(got-burstGain/2) > 1e-9 {
				t.Fatalf("%d exploding cells attenuated a map cue to %v", cells, got)
			}
		}
	}
}

func TestWaitingCuesDrainAfterFinalWave(t *testing.T) {
	s, _, _ := testSound()
	s.Place(0)
	s.Effect(WallBreak)
	s.Effect(HoldOff | WallBreak)
	s.Update()
	if s.effects[0][7].IsPlaying() || s.wait.effects != WallBreak|HoldOff {
		t.Fatal("final-wave cues must wait for the beat and coalesce")
	}
	s.last = time.Now().Add(-beat)
	s.Update()
	if s.wait != (burst{}) || !s.effects[0][7].IsPlaying() || !s.effects[0][5].IsPlaying() {
		t.Fatal("final-wave cues were dropped instead of drained")
	}
	s.Update()
	if s.effects[0][7].(*testPlayer).plays != 1 {
		t.Fatal("a coalesced wall break played more than once")
	}
	for _, muted := range []bool{true, false} {
		s.wait = burst{effects: Link, cells: 3}
		s.Muted = muted
		if !muted {
			s.Volume = 0
		}
		s.Update()
		if s.wait != (burst{}) || s.effects[0][0].IsPlaying() {
			t.Fatal("muting or zero volume retained a stale burst")
		}
	}
	var demo *Sound
	demo.Update()
}

func TestSoundMuteSilenceAndNil(t *testing.T) {
	for _, tc := range []struct {
		muted  bool
		volume float64
	}{{true, 1}, {false, 0}} {
		s, group, _ := testSound()
		s.Muted, s.Volume = tc.muted, tc.volume
		s.Place(Latch)
		s.Wave(16, true, Link|WallBreak)
		s.Effect(HoldOff)
		for _, p := range group {
			if p.plays != 0 {
				t.Fatal("muted or zero-volume sound played")
			}
		}
		if !s.last.IsZero() || s.step != 0 {
			t.Fatal("silence must not consume the beat or move the song on")
		}
	}
	s, _, _ := testSound()
	s.Effect(0)
	if !s.last.IsZero() {
		t.Fatal("an empty terrain batch consumed the beat")
	}
	var demo *Sound
	demo.Place(Latch)
	demo.Wave(4, false, Link)
	demo.Effect(WallHit)
}

func TestCombinedSoundsKeepFifteenVoiceLimit(t *testing.T) {
	s, _, peak := testSound()
	every := Effects(1<<(len(effectTones)+1) - 1)
	for k := range 30 {
		s.last = time.Now().Add(-beat)
		effects := every
		if k%2 == 1 {
			effects = Link | Fall // leaves room for a full chord
		}
		chord, lift := s.step%len(chords), s.lift%5
		s.Wave(16, false, effects)
		if *peak > 15 {
			t.Fatalf("simultaneous voices reached %d", *peak)
		}
		for i, p := range s.effects[chord] {
			if effects&(1<<i) != 0 && !p.IsPlaying() {
				t.Fatal("voice stealing cut an effect from the current burst")
			}
		}
		if effects == every && !s.knockout.IsPlaying() {
			t.Fatal("voice stealing cut Knockout from the current burst")
		}
		for i := range 5 {
			if effects != every && !s.notes[chords[chord][lift+i]].IsPlaying() {
				t.Fatal("voice stealing cut a note from the current burst")
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

func TestChordsArePentatonicTriads(t *testing.T) {
	for chord, tones := range chords {
		classes := map[int]bool{}
		for i, tone := range tones {
			if tone < 0 || tone >= len(pentatonic) || i > 0 && tone <= tones[i-1] {
				t.Fatalf("chord %d tones are not rising pentatonic notes", chord)
			}
			classes[pentatonic[tone]%12] = true
		}
		if len(classes) != 3 || !classes[pentatonic[roots[chord]]%12] {
			t.Fatalf("chord %d is not three pitch classes holding its root", chord)
		}
	}
}

func TestEffectPCMIsTunedDistinctAndFaded(t *testing.T) {
	for effect, tone := range effectTones {
		for _, index := range tone.notes {
			if index >= len(chords[0]) {
				t.Fatalf("effect %d leaves the chord's tones", effect)
			}
		}
		if beats := tone.delay / .05; math.Abs(beats-math.Round(beats)) > 1e-9 {
			t.Fatalf("effect %d's notes fall off the half beat", effect)
		}
	}
	pcm := testPCM()[len(pentatonic):]
	for i, cue := range pcm {
		chord, effect := i/len(effectTones), i%len(effectTones)
		if i < len(pcm)-1 && len(cue) != 4*int(effectTones[effect].length*sampleRate) {
			t.Fatalf("effect %d has an invalid stereo PCM length", effect)
		}
		peak := 0
		for i := 0; i < len(cue); i += 4 {
			left := int(int16(binary.LittleEndian.Uint16(cue[i:])))
			right := int(int16(binary.LittleEndian.Uint16(cue[i+2:])))
			if left != right {
				t.Fatal("stereo channels differ")
			}
			peak = max(peak, int(math.Abs(float64(left))))
		}
		if peak < 1000 || float64(peak) > peakCeiling*math.MaxInt16+1 {
			t.Fatalf("effect %d is silent or clips: peak %d", effect, peak)
		}
		if !bytes.Equal(cue[:4], make([]byte, 4)) || !bytes.Equal(cue[len(cue)-4:], make([]byte, 4)) {
			t.Fatalf("effect %d lacks silent attack/release endpoints", effect)
		}
		for _, previous := range pcm[chord*len(effectTones) : i] {
			if bytes.Equal(cue, previous) {
				t.Fatalf("effect %d duplicates another cue of its chord", effect)
			}
		}
	}
}

func sine(freq, seconds float64) []byte {
	pcm := make([]byte, 4*int(seconds*sampleRate))
	for i := range len(pcm) / 4 {
		x := uint16(int16(8000 * math.Sin(2*math.Pi*freq*float64(i)/sampleRate)))
		binary.LittleEndian.PutUint16(pcm[4*i:], x)
		binary.LittleEndian.PutUint16(pcm[4*i+2:], x)
	}
	return pcm
}

func TestLoudnessWeighsPitchAndLength(t *testing.T) {
	reference, _ := loudness(sine(1000, 1))
	for _, tc := range []struct {
		freq, seconds, db float64
	}{{100, 1, -19.1}, {250, 1, -8.6}, {2000, 1, 1.2}, {1000, .05, -6}} { // the A-weighting table; a quarter of the window
		level, _ := loudness(sine(tc.freq, tc.seconds))
		if got := 20 * math.Log10(level/reference); math.Abs(got-tc.db) > .5 {
			t.Errorf("%v Hz for %v s measures %.1f dB against 1 kHz, want %.1f", tc.freq, tc.seconds, got, tc.db)
		}
	}
}

func TestEveryCueHasTheSameLoudness(t *testing.T) {
	for i, pcm := range testPCM() {
		level, peak := loudness(pcm)
		if db := 20 * math.Log10(level/loudTarget); math.Abs(db) > .1 {
			t.Errorf("buffer %d is %.2f dB off the shared loudness (peak %.2f)", i, db, peak)
		}
	}
}

func TestNormalizeSilenceAndTransientHeadroom(t *testing.T) {
	if normalize(nil) != nil {
		t.Fatal("empty PCM changed")
	}
	silence := make([]byte, sampleRate/20*4)
	normalize(silence)
	if level, peak := loudness(silence); level != 0 || peak != 0 {
		t.Fatal("normalizing silence introduced noise")
	}
	impulse := make([]byte, sampleRate/20*4)
	binary.LittleEndian.PutUint16(impulse, math.MaxInt16)
	binary.LittleEndian.PutUint16(impulse[2:], math.MaxInt16)
	normalize(impulse)
	if _, peak := loudness(impulse); math.Abs(peak-peakCeiling) > .001 {
		t.Fatalf("transient peak %v did not respect headroom", peak)
	}
}

func TestDenseNormalizedBurstsKeepMixHeadroom(t *testing.T) {
	pcm := testPCM()
	every := Effects(1<<(len(effectTones)+1) - 1)
	worst := 0.0
	for _, effects := range []Effects{every, WallHit, WallHit | BoxSlide | Turret, 0} {
		for _, cells := range []int{1, 16} {
			for k := range 2 * len(chords) {
				s, players, _ := testSound()
				s.step, s.lift = k, k
				s.Wave(cells, false, effects)
				for frame := range int(noteLength * sampleRate) {
					mix := 0.0
					for i, p := range players {
						if p.playing && 4*frame < len(pcm[i]) {
							mix += float64(int16(binary.LittleEndian.Uint16(pcm[i][4*frame:]))) / math.MaxInt16 * p.volume
						}
					}
					worst = max(worst, math.Abs(mix))
				}
			}
		}
	}
	t.Logf("worst mix peak %.3f", worst)
	if worst > .9 {
		t.Fatalf("mix peak %v risks clipping", worst)
	}
}
