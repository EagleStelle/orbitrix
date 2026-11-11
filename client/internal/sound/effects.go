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
)

type effectTone struct {
	notes                 [3]int // indices into the shared pentatonic scale; -1 is unused
	length, attack, decay float64
	delay                 float64
	harmonics             [4]float64
}

// Integer harmonics keep the resonances tuned; envelopes and harmonic balance distinguish the instruments.
var effectTones = [...]effectTone{
	{[3]int{10, 13, -1}, .42, .004, .10, .055, [4]float64{1, .08, .12, .35}}, // Link: glass chimes, C6 to G6
	{[3]int{7, -1, -1}, .18, .0015, .035, 0, [4]float64{1, .55, .3, .12}},    // Latch: firm keyed click, E5
	{[3]int{9, -1, -1}, .14, .001, .025, 0, [4]float64{1, .2, .65, .2}},      // Toggle on: dry reed, A5
	{[3]int{6, -1, -1}, .14, .001, .025, 0, [4]float64{1, .2, .65, .2}},      // Toggle off: D5
	{[3]int{3, -1, -1}, .22, .003, .045, 0, [4]float64{1, .35, .08, 0}},      // Hold on: damped mallet, G4
	{[3]int{0, -1, -1}, .22, .003, .045, 0, [4]float64{1, .35, .08, 0}},      // Hold off: C4
	{[3]int{0, -1, -1}, .16, .001, .025, 0, [4]float64{1, .5, .3, .2}},       // Wall damage: woody knock, C4
	{[3]int{3, 2, 0}, .48, .001, .065, .035, [4]float64{1, .5, .3, .2}},      // Wall break: falling wooden fragments, G4 E4 C4
	{[3]int{9, 6, 2}, .6, .004, .11, .085, [4]float64{1, 0, .3, 0}},          // Hole: hollow drop, wide steps down A5 D5 E4
	{[3]int{0, 3, 5}, .5, .002, .09, .07, [4]float64{1, .35, .08, 0}},        // Landfill full: mallet steps up to level ground, C4 G4 C5
	{[3]int{1, 0, -1}, .24, .002, .03, .07, [4]float64{1, .5, .3, .2}},       // Box slide: two soft woody knocks, D4 C4
	{[3]int{7, 11, -1}, .28, .001, .05, .06, [4]float64{1, .2, .65, .2}},     // Spring: a reed boing rising, E5 to D6
}

func synthEffect(tone effectTone) []byte {
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
	for i := range n {
		v := 0.0
		for k, note := range tone.notes {
			t := float64(i)/sampleRate - float64(k)*tone.delay
			if note < 0 || t < 0 {
				continue
			}
			freq := 440 * math.Pow(2, float64(pentatonic[note]-69)/12)
			env := min(1, t/tone.attack) * math.Exp(-t/tone.decay)
			for h, amplitude := range tone.harmonics {
				v += amplitude * math.Sin(2*math.Pi*freq*float64(h+1)*t) * env * math.Exp(-float64(h)*t/tone.decay)
			}
		}
		fade := min(1, float64(n-1-i)/(.015*sampleRate))
		x := uint16(int16(v / (weight * float64(count)) * fade * .65 * math.MaxInt16))
		buf[4*i], buf[4*i+1], buf[4*i+2], buf[4*i+3] = byte(x), byte(x>>8), byte(x), byte(x>>8)
	}
	return normalize(buf)
}
