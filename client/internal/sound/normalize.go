package sound

import (
	"encoding/binary"
	"math"
)

// normalize matches the loudest 50 ms RMS window, ignoring tail length, with a 0.65 peak ceiling.
// Synthesis produces mono duplicated into stereo; this runs only while building the startup buffers.
func normalize(pcm []byte) []byte {
	window := min(len(pcm)/4, sampleRate/20)
	if window == 0 {
		return pcm
	}
	energy, loudest, peak := 0.0, 0.0, 0.0
	for frame := range len(pcm) / 4 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[4*frame:])))
		peak = max(peak, math.Abs(v))
		energy += v * v
		if frame >= window {
			old := float64(int16(binary.LittleEndian.Uint16(pcm[4*(frame-window):])))
			energy -= old * old
		}
		loudest = max(loudest, energy)
	}
	if peak == 0 {
		return pcm
	}
	gain := min(.18*math.MaxInt16/math.Sqrt(loudest/float64(window)), .65*math.MaxInt16/peak)
	for offset := 0; offset < len(pcm); offset += 4 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[offset:])))
		x := uint16(int16(math.Round(v * gain)))
		binary.LittleEndian.PutUint16(pcm[offset:], x)
		binary.LittleEndian.PutUint16(pcm[offset+2:], x)
	}
	return pcm
}
