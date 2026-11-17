package sound

import "math"

// Effects coalesces each map cue to one voice per animation step.
type Effects uint16

const (
	Link Effects = 1 << iota
	Latch
	ToggleOn
	ToggleOff
	HoldOn
	HoldOff
	WallHit
	WallBreak
	Fall
	LandfillFull
	BoxSlide
	Spring
	Shield
	Turret
	Knockout // a player put out; synthKnockout renders it, after the effectTones
)

type effectTone struct {
	notes                 [3]int // tones of the chord being played, 0 low to 8 high; -1 is unused
	length, attack, decay float64
	delay                 float64 // between its notes, on the half beat
	harmonics             [4]float64
}

// Every cue plays tones of the current chord, so cues that sound together form that chord. Integer harmonics keep
// the resonances tuned; contours, envelopes and harmonic balance distinguish the instruments.
var effectTones = [...]effectTone{
	{[3]int{6, 8, -1}, .42, .004, .10, .05, [4]float64{1, .08, .12, .35}}, // Link: glass chimes rising, high
	{[3]int{4, -1, -1}, .18, .0015, .035, 0, [4]float64{1, .55, .3, .12}}, // Latch: firm keyed click
	{[3]int{5, -1, -1}, .14, .001, .025, 0, [4]float64{1, .2, .65, .2}},   // Toggle on: dry reed
	{[3]int{3, -1, -1}, .14, .001, .025, 0, [4]float64{1, .2, .65, .2}},   // Toggle off: two tones lower
	{[3]int{2, -1, -1}, .22, .003, .045, 0, [4]float64{1, .35, .08, 0}},   // Hold on: damped mallet
	{[3]int{1, -1, -1}, .22, .003, .045, 0, [4]float64{1, .35, .08, 0}},   // Hold off: one tone lower
	{[3]int{0, -1, -1}, .16, .001, .025, 0, [4]float64{1, .5, .3, .2}},    // Wall damage: woody knock, the lowest tone
	{[3]int{2, 1, 0}, .48, .001, .065, .05, [4]float64{1, .5, .3, .2}},    // Wall break: falling wooden fragments
	{[3]int{5, 3, 1}, .6, .004, .11, .1, [4]float64{1, 0, .3, 0}},         // Hole: hollow drop, wide steps down
	{[3]int{0, 2, 3}, .5, .002, .09, .1, [4]float64{1, .35, .08, 0}},      // Landfill full: mallet steps up to level ground
	{[3]int{1, 0, -1}, .24, .002, .03, .1, [4]float64{1, .5, .3, .2}},     // Box slide: two soft woody knocks
	{[3]int{4, 7, -1}, .28, .001, .05, .05, [4]float64{1, .2, .65, .2}},   // Spring: a reed boing rising an octave
	{[3]int{3, 5, 7}, .4, .006, .12, .05, [4]float64{1, .6, .1, .3}},      // Shield: a bright hum sweeping up and ringing
	{[3]int{0, 5, -1}, .3, .001, .05, .05, [4]float64{1, .5, .3, .2}},     // Turret: a woody thump and its shell's ping
}

// synthKnockout renders a player put out, longer and heavier than any map cue: a hit sagging an octave onto C4,
// A6, G6 and E6 cracking over it, the whole scale falling from A6 to C4, and C4 and G4 aching underneath, each
// against a slightly flat twin that throbs fast at first and slows like a fading pulse.
func synthKnockout() []byte {
	const length = 2.6
	n := int(length * sampleRate)
	mono := make([]float64, n)
	phase, peak := 0.0, 0.0
	for i := range n {
		t := float64(i) / sampleRate
		phase += 2 * math.Pi * hz(0) * (1 + math.Exp(-t/.05)) / sampleRate // C5 sagging to C4
		v := 1.4 * (math.Sin(phase) + .5*math.Sin(2*phase)) * min(1, t/.002) * math.Exp(-t/.35)
		for _, note := range [...]int{14, 13, 12} {
			v += .35 * math.Sin(2*math.Pi*hz(note)*t) * min(1, t/.001) * math.Exp(-t/.04)
		}
		for k := range len(pentatonic) {
			s := t - .1 - float64(k)*.05
			if s < 0 {
				break
			}
			f := hz(len(pentatonic) - 1 - k)
			v += .45 * (math.Sin(2*math.Pi*f*s) + .3*math.Sin(4*math.Pi*f*s)) * min(1, s/.002) * math.Exp(-s/.09)
		}
		if s := t - .15; s > 0 {
			env := min(1, s/.25) * math.Exp(-s/1)
			drift := s + 3*.6*(1-math.Exp(-s/.6)) // the twin's flatness integrated: 4 Hz of beating easing to 1 Hz
			for _, note := range [...]int{0, 3} {
				v += .5 * env * (math.Sin(2*math.Pi*hz(note)*s) + math.Sin(2*math.Pi*(hz(note)*s-drift)))
			}
		}
		mono[i] = v * min(1, float64(n-1-i)/(.05*sampleRate))
		peak = max(peak, math.Abs(mono[i]))
	}
	buf := make([]byte, 4*n)
	for i, v := range mono {
		x := uint16(int16(v / peak * .65 * math.MaxInt16))
		buf[4*i], buf[4*i+1], buf[4*i+2], buf[4*i+3] = byte(x), byte(x>>8), byte(x), byte(x>>8)
	}
	return normalize(buf)
}

// synthEffect renders tone on the tones of chord.
func synthEffect(tone effectTone, chord int) []byte {
	n := int(tone.length * sampleRate)
	buf := make([]byte, 4*n)
	var weight float64
	for _, h := range tone.harmonics {
		weight += h
	}
	count := 0
	for _, note := range tone.notes {
		if note >= 0 {
			count++
		}
	}
	var freq [len(tone.notes)]float64
	for k, note := range tone.notes {
		if note >= 0 {
			freq[k] = hz(chords[chord][note])
		}
	}
	for i := range n {
		v := 0.0
		for k, note := range tone.notes {
			t := float64(i)/sampleRate - float64(k)*tone.delay
			if note < 0 || t < 0 {
				continue
			}
			decay := math.Exp(-t / tone.decay)
			env := min(1, t/tone.attack) * decay
			for h, amplitude := range tone.harmonics {
				v += amplitude * math.Sin(2*math.Pi*freq[k]*float64(h+1)*t) * env
				env *= decay // each higher harmonic dies sooner
			}
		}
		fade := min(1, float64(n-1-i)/(.015*sampleRate))
		x := uint16(int16(v / (weight * float64(count)) * fade * .65 * math.MaxInt16))
		buf[4*i], buf[4*i+1], buf[4*i+2], buf[4*i+3] = byte(x), byte(x>>8), byte(x), byte(x>>8)
	}
	return normalize(buf)
}
