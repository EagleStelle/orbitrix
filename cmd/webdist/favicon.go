package main

import (
	"fmt"
	"math"
)

// The logo, in a 192-unit square centred on the origin: a Vermilion orb with four smaller ones on the axes, and in
// each corner an Ink leaf and crescent bursting outward. Their facing edges are arcs around the centre, and both
// shapes end on the lines tipS either side of their diagonal.
const (
	logoHalf = 96.0 // half the square
	orbR     = 24.0 // centre orb
	sideR    = 9.0  // side orbs
	sideD    = 72.0 // side orbs' distance from the centre
	tipS     = 25.0 // half-width of each corner shape, across its diagonal
	leafOut  = 51.0 // the leaf's outer edge: an arc around the centre
	leafIn   = 40.0 // where the leaf's inner edge crosses the diagonal
	cresIn   = 60.0 // the crescent's inner edge: an arc around the centre
	cresOut  = 70.0 // where the crescent's outer edge crosses the diagonal
)

// favicon returns the logo as an SVG document in the light Party Mat inks, Ink corners and Vermilion orbs, with no
// background.
func favicon() []byte { return logo("#171615", "#D55E00") }

// darkLogo is the logo in the dark mode's Chalk Ink and night Vermilion, for the README on GitHub's dark theme.
func darkLogo() []byte { return logo("#F5F2EE", "#E86A1A") }

// logo returns the logo as an SVG document with the corners in ink and the orbs in orb, with no background.
func logo(ink, orb string) []byte {
	leafTip := math.Sqrt(leafOut*leafOut - tipS*tipS) // where each shape's tips sit along the diagonal
	cresTip := math.Sqrt(cresIn*cresIn - tipS*tipS)
	// One corner's leaf and crescent along +x, from tip to tip and back.
	corner := fmt.Sprintf("M%.2f %.2fA%.2f %.2f 0 0 1 %.2f %.2fA%.2f %.2f 0 0 1 %.2f %.2fZ"+
		"M%.2f %.2fA%.2f %.2f 0 0 1 %.2f %.2fA%.2f %.2f 0 0 0 %.2f %.2fZ",
		leafTip, -tipS, leafOut, leafOut, leafTip, tipS, arcRadius(leafTip, leafIn), arcRadius(leafTip, leafIn), leafTip, -tipS,
		cresTip, -tipS, arcRadius(cresTip, cresOut), arcRadius(cresTip, cresOut), cresTip, tipS, cresIn, cresIn, cresTip, -tipS)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%[1]g %[1]g %[2]g %[2]g"><g fill="%[3]s">`,
		-logoHalf, 2*logoHalf, ink)
	for _, deg := range []int{45, 135, 225, 315} {
		svg += fmt.Sprintf(`<path transform="rotate(%d)" d="%s"/>`, deg, corner)
	}
	svg += fmt.Sprintf(`</g><g fill="%[4]s"><circle r="%[1]g"/><circle cx="%[2]g" r="%[3]g"/><circle cx="-%[2]g" r="%[3]g"/>`+
		`<circle cy="%[2]g" r="%[3]g"/><circle cy="-%[2]g" r="%[3]g"/></g></svg>`+"\n", orbR, sideD, sideR, orb)
	return []byte(svg)
}

// arcRadius is the radius of the arc through a shape's tips, (tip, ±tipS), that crosses the diagonal at t.
func arcRadius(tip, t float64) float64 {
	k := math.Abs(t - tip)
	return (tipS*tipS + k*k) / (2 * k)
}
