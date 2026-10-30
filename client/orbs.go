package client

import (
	"math"

	"orbitrix/core"
)

// Orb motion, from the prototype's draw_orbs_on_surface. Everything here is in "base" units: the prototype's
// unscaled cell size, before the board is scaled to fit the window. Positions are relative to the cell centre.
const (
	orbSlots     = 8       // per-cell seed and position slots; the prototype cached 8 seeds per cell
	smoothness   = 8.0     // exponential smoothing rate (1/s): lower is smoother but laggier
	angIdle      = 0.00008 // angular speed in rad/ms: slow orbit when idle
	angUrgent    = 0.0016  // and frantic when the cell is about to explode
	relaxPasses  = 8       // pairwise separation passes
	snapAfterMS  = 200     // a cell not drawn for this long snaps to its layout instead of gliding
	tau          = 2 * math.Pi
	invTwoPow32  = 1.0 / 4294967296.0
	cellPhaseMix = 0.7 // radians of per-cell angular offset
)

// Layout angles and radius multipliers for 2, 3 and 4+ orbs; slots beyond 4 reuse them with their own seeds.
var (
	layoutAngles = [3][]float64{{0, math.Pi}, {0, math.Pi, math.Pi / 2}, {0, math.Pi, math.Pi / 2, 3.5}}
	layoutRadii  = [3][]float64{{0.6, 0.6}, {0.5, 0.5, 0.6}, {0.48, 0.48, 0.72, 0.72}}
)

// orbRadius is the prototype's get_orb_radius: at least 8, about 22% of the cell, never above a third of it.
func orbRadius(cell float64) float64 {
	return min(max(8, math.Floor(cell*0.22)), math.Floor(cell/3))
}

// orbField holds the deterministic per-cell variation and the smoothed orb positions of every cell.
type orbField struct {
	geo   *core.Geometry
	seed  []uint32  // cells*orbSlots
	frac  []float64 // seed / 2^32
	phase []uint32  // per-cell phase
	pos   []float64 // cells*orbSlots*2, smoothed positions
	n     []uint8   // orbs currently in pos
	lastT []float64 // ms when the cell was last updated

	tgt [orbSlots][2]float64 // scratch
}

func newOrbField(geo *core.Geometry) *orbField {
	n := geo.Cells()
	f := &orbField{
		geo: geo, seed: make([]uint32, n*orbSlots), frac: make([]float64, n*orbSlots), phase: make([]uint32, n),
		pos: make([]float64, n*orbSlots*2), n: make([]uint8, n), lastT: make([]float64, n),
	}
	for i := range n {
		c, r := geo.Coords(i)
		key := uint64(r&0xffff)<<16 | uint64(c&0xffff)
		for s := range orbSlots {
			rng := core.NewRNG(key<<8 ^ uint64(s)) // first output = splitmix64(key<<8 ^ s)
			v := uint32(rng.Uint64())
			f.seed[i*orbSlots+s], f.frac[i*orbSlots+s] = v, float64(v)*invTwoPow32
		}
		f.phase[i] = uint32(r)*1315423911 ^ uint32(c)*2654435761
	}
	return f
}

// update moves the orbs of cell i (holding count orbs, count >= 1) towards their layout at time tms and returns
// them; the result is only valid until the next call. cell is the base cell size.
func (f *orbField) update(i, count int, tms, cell float64) [][2]float64 {
	capacity := int(f.geo.Cap[i])
	pop := count >= capacity-1
	corner := capacity == 2
	r := orbRadius(cell)
	maxOrbit := max(2, cell/2-r-12)

	jitterAmp, freq, breathAmp := max(0.18, r*0.10), 0.0016, 0.002
	angSpeed, driftAmp, driftRate, driftMix, breathRate, breathMix := angIdle, 0.12*0.45, 0.00012, 0.35, 0.0006, 0.35
	if pop {
		jitterAmp, freq, breathAmp = max(0.6, r*0.28), 0.012, 0.02
		angSpeed, driftAmp, driftRate, driftMix, breathRate, breathMix = angUrgent, 0.12*1.6, 0.0006, 1, 0.0025, 1
	}
	minSep := max(math.Floor(2*r+8), 10)
	if pop {
		minSep = math.Floor(minSep * 1.06)
	}
	slots := min(count, orbSlots)
	base := i * orbSlots
	phase := f.phase[i]
	t := &f.tgt

	// Layout targets on rotating orbits, pushed outwards (and round) when they crowd the earlier ones.
	if count == 1 {
		t[0] = [2]float64{0, 0}
	} else {
		li := min(count, 4) - 2
		angles, radii := layoutAngles[li], layoutRadii[li]
		cellPhase := float64(phase&0xffff) / 65536 * cellPhaseMix
		for k := range slots {
			fr, sv := f.frac[base+k], f.seed[base+k]
			dir := 1.0
			if sv&1 != 0 {
				dir = -1
			}
			angle := angles[k%len(angles)] + (fr-0.5)*math.Pi*0.9 + dir*tms*angSpeed*(0.6+fr*0.8) + cellPhase
			angle += math.Sin(tms*driftRate+float64(sv%1000)) * driftAmp * driftMix
			breath := math.Sin(tms*breathRate+float64(sv%500)) * breathAmp * breathMix
			orbit := maxOrbit * (radii[k%len(radii)]*(0.92+fr*0.08) + breath)
			if count > 2 {
				for attempt := range 3 {
					px, py := orbit*math.Cos(angle), orbit*math.Sin(angle)
					crowded := false
					for j := range k {
						dx, dy := px-t[j][0], py-t[j][1]
						if d2 := dx*dx + dy*dy; d2 < minSep*minSep {
							crowded = true
							orbit = min(maxOrbit, orbit+minSep-math.Sqrt(d2)+1)
							if orbit >= maxOrbit-0.5 {
								rot := 1.0
								if sv&2 != 0 {
									rot = -1
								}
								angle += rot * 0.12 * float64(attempt+1)
							}
							break
						}
					}
					if !crowded {
						break
					}
				}
			}
			t[k] = [2]float64{orbit * math.Cos(angle), orbit * math.Sin(angle)}
		}
	}

	// Jitter: every orb of a busy cell, plus the special case of a lone orb in a corner about to pop.
	amp := 0.28
	if pop {
		amp = 1
	}
	if count == 1 {
		amp = 0
		if corner && pop {
			amp = 0.8
		}
	}
	if amp > 0 {
		f2 := freq
		if pop {
			f2 *= 1.9
		}
		a := jitterAmp * amp
		for k := range slots {
			fr, sv := f.frac[base+k], f.seed[base+k]
			ph := tms*f2 + fr*6.28318 + float64(k)*0.73 + float64(phase&0xff)*0.0123
			wob := math.Sin(tms*f2*0.2+fr*3.1) * a * 0.06
			jx := math.Sin(ph*(1+fr*0.25))*a + wob
			jy := math.Cos(ph*1.13*(1+fr*0.18))*a*0.68 - wob*0.35
			if pop {
				extra := math.Sin(tms*freq*6+float64(sv&255)) * jitterAmp * 0.5 * amp
				jx += extra
				jy -= extra * 0.28
			}
			t[k][0] += jx
			t[k][1] += jy
		}
	}

	// Pairwise relaxation keeps orbs apart, then clamps them inside the orbit.
	if slots > 1 {
		for range relaxPasses {
			moved := false
			for a := range slots {
				for b := a + 1; b < slots; b++ {
					dx, dy := t[a][0]-t[b][0], t[a][1]-t[b][1]
					d := math.Sqrt(dx*dx + dy*dy)
					if d < 1e-4 {
						ang := float64(f.seed[base+a]&0xff) / 255 * tau
						dx, dy, d = math.Cos(ang)*0.5, math.Sin(ang)*0.5, 0.5
					}
					if d < minSep {
						s := (minSep - d) * 0.5 / d
						t[a][0] += dx * s
						t[a][1] += dy * s
						t[b][0] -= dx * s
						t[b][1] -= dy * s
						moved = true
					}
				}
			}
			for k := range slots {
				if d := math.Hypot(t[k][0], t[k][1]); d > maxOrbit {
					t[k][0] *= maxOrbit / d
					t[k][1] *= maxOrbit / d
				}
			}
			if !moved {
				break
			}
		}
	}

	// Frame-rate independent exponential smoothing towards the targets; snap on a new count or a long gap.
	p := f.pos[base*2 : base*2+slots*2]
	if int(f.n[i]) != slots || tms-f.lastT[i] > snapAfterMS {
		for k := range slots {
			p[2*k], p[2*k+1] = t[k][0], t[k][1]
		}
	} else {
		alpha := 1 - math.Exp(-smoothness*max(1e-6, (tms-f.lastT[i])/1000))
		for k := range slots {
			p[2*k] += (t[k][0] - p[2*k]) * alpha
			p[2*k+1] += (t[k][1] - p[2*k+1]) * alpha
		}
	}
	f.n[i], f.lastT[i] = uint8(slots), tms
	for k := range slots {
		t[k] = [2]float64{p[2*k], p[2*k+1]}
	}
	return t[:slots]
}
