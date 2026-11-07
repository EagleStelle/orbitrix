package ui

import (
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"rsc.io/qr"
)

// quietZone is the light margin a QR code needs around it, in modules.
const QuietZone = 4

// qrCode is a link drawn as a QR code: its modules kept in a small image, one pixel each, drawn scaled up by a whole
// number with no smoothing. It is dark on light in either theme, which every camera reads.
type QR struct {
	text string
	img  *ebiten.Image // nil when text is "" or didn't encode
	N    int           // modules on a side
	Err  error
}

// set encodes text, unless it already holds it.
func (q *QR) Set(text string) {
	if text == q.text {
		return
	}
	q.text = text
	q.Err = nil
	if q.img != nil {
		q.img.Deallocate()
		q.img = nil
	}
	if text == "" {
		return
	}
	c, err := qr.Encode(text, qr.L) // a screen read by a camera needs no more error correction
	if err != nil {
		q.Err = err
		log.Printf("qr: %v", err)
		return
	}
	ink := Palettes[0].Ink
	pix := make([]byte, 4*c.Size*c.Size)
	for y := range c.Size {
		for x := range c.Size {
			if i := 4 * (y*c.Size + x); c.Black(x, y) {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = ink.R, ink.G, ink.B, 255
			}
		}
	}
	q.N, q.img = c.Size, ebiten.NewImage(c.Size, c.Size)
	q.img.WritePixels(pix)
}

// draw draws the code as large as fits in box, centred, on the light theme's mat with its quiet zone.
func (q *QR) Draw(dst *ebiten.Image, box Rect) {
	if q.img == nil {
		return
	}
	m := math.Floor(min(box.W, box.H) / float64(q.N+2*QuietZone)) // whole device px per module keep every one alike
	if m < 1 {
		return
	}
	side := m * float64(q.N+2*QuietZone)
	x, y := math.Floor(box.Cx()-side/2), math.Floor(box.Cy()-side/2)
	FillRound(dst, Rect{x, y, side, side}, 2*m, Palettes[0].Ground)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(m, m)
	op.GeoM.Translate(x+QuietZone*m, y+QuietZone*m)
	dst.DrawImage(q.img, &op)
}
