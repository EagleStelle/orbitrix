package sound

import (
	"encoding/binary"
	"math"
)

const (
	loudWindow  = sampleRate / 5 // 200 ms, about what the ear sums into one loudness
	loudTarget  = .048           // every cue's level; low enough that the shortest, lowest knock reaches it
	peakCeiling = .9
)

// aCorners are the A-weighting curve's corner frequencies in Hz: four high-pass, then two low-pass.
var aCorners = [...]float64{20.6, 20.6, 107.7, 737.9, 12194, 12194}

// weigh filters x in place with the A-weighting curve, so levels compare as the ear hears pitch. Its overall gain
// is arbitrary: only levels relative to each other are used.
func weigh(x []float64) {
	for i, f := range aCorners {
		w := math.Tan(math.Pi * f / sampleRate) // bilinear transform, prewarped
		lastIn, lastOut := 0.0, 0.0
		for n, in := range x {
			out := in - lastIn
			if i >= 4 {
				out = w * (in + lastIn)
			}
			out = (out + (1-w)*lastOut) / (1 + w)
			lastIn, lastOut, x[n] = in, out, out
		}
	}
}

// loudness is the A-weighted RMS of pcm's loudest 200 ms and its peak, both 0..1. A shorter cue is measured over
// the whole 200 ms, as the ear hears a shorter sound as quieter.
func loudness(pcm []byte) (level, peak float64) {
	x := make([]float64, len(pcm)/4)
	for i := range x {
		x[i] = float64(int16(binary.LittleEndian.Uint16(pcm[4*i:]))) / math.MaxInt16
		peak = max(peak, math.Abs(x[i]))
	}
	weigh(x)
	energy, loudest := 0.0, 0.0
	for i, v := range x {
		energy += v * v
		if i >= loudWindow {
			energy -= x[i-loudWindow] * x[i-loudWindow]
		}
		loudest = max(loudest, energy)
	}
	return math.Sqrt(loudest / loudWindow), peak
}

// normalize brings pcm to the shared loudness, so no cue is quieter or louder than another to the ear, under a peak
// ceiling. Synthesis produces mono duplicated into stereo; this runs only while building the startup buffers.
func normalize(pcm []byte) []byte {
	level, peak := loudness(pcm)
	if peak == 0 {
		return pcm
	}
	gain := min(loudTarget/level, peakCeiling/peak)
	for offset := 0; offset < len(pcm); offset += 4 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[offset:])))
		x := uint16(int16(math.Round(v * gain)))
		binary.LittleEndian.PutUint16(pcm[offset:], x)
		binary.LittleEndian.PutUint16(pcm[offset+2:], x)
	}
	return pcm
}
