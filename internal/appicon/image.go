package appicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Image draws the logo on a transparent background, with supersampling for small window icons.
func Image(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	leafR := arcRadius(math.Sqrt(leafOut*leafOut-tipS*tipS), leafIn)
	cresR := arcRadius(math.Sqrt(cresIn*cresIn-tipS*tipS), cresOut)
	disk := func(x, y, r float64) bool { return x*x+y*y <= r*r }
	for py := range size {
		for px := range size {
			var red, green, blue, alpha uint32
			for sy := range 4 {
				for sx := range 4 {
					x := (float64(px)+(float64(sx)+0.5)/4)*2*logoHalf/float64(size) - logoHalf
					y := (float64(py)+(float64(sy)+0.5)/4)*2*logoHalf/float64(size) - logoHalf
					c := color.RGBA{}
					// Reflection maps all four corners onto the same diagonal lens and crescent.
					u, v := (math.Abs(x)+math.Abs(y))/math.Sqrt2, (math.Abs(y)-math.Abs(x))/math.Sqrt2
					if disk(u, v, leafOut) && disk(u-leafIn-leafR, v, leafR) ||
						!disk(u, v, cresIn) && disk(u-cresOut+cresR, v, cresR) {
						c = color.RGBA{0x17, 0x16, 0x15, 0xFF}
					}
					if disk(x, y, orbR) || disk(math.Abs(x)-sideD, y, sideR) || disk(x, math.Abs(y)-sideD, sideR) {
						c = color.RGBA{0xD5, 0x5E, 0x00, 0xFF}
					}
					red, green, blue = red+uint32(c.R), green+uint32(c.G), blue+uint32(c.B)
					alpha += uint32(c.A)
				}
			}
			img.SetRGBA(px, py, color.RGBA{uint8(red / 16), uint8(green / 16), uint8(blue / 16), uint8(alpha / 16)})
		}
	}
	return img
}

func PNG(size int) ([]byte, error) {
	var b bytes.Buffer
	err := png.Encode(&b, Image(size))
	return b.Bytes(), err
}
