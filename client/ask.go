package client

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// firewallText is said to Windows players before the first Same Wi-Fi page, which listens on the network and so
// brings up Windows' own question.
const firewallText = "Windows will ask to let Orbitrix on your network. Allow it."

// askDialog is a question in the pause menu's dialog style over a dimmed screen: its words, then the answer (ink)
// above Back.
type askDialog struct {
	text, yes string

	lv    rect // the safe area it was laid out for
	d     rect
	b     [2]rect
	lines []string
}

func (d *askDialog) layout(a *app) {
	v, u := a.safe(), a.u
	if v == d.lv && d.lines != nil {
		return
	}
	d.lv = v
	bw, bh, gap, lh := min(300*u, v.w-88*u), quietButtonHeight*u, 12*u, 26*u
	d.lines = wrap(d.text, a.f.heading, bw)
	textH := float64(len(d.lines)) * lh
	h := 28*u + textH + 24*u + 2*bh + gap + 28*u
	d.d = rect{v.cx() - bw/2 - 28*u, v.cy() - h/2, bw + 56*u, h}
	for i := range d.b {
		d.b[i] = rect{d.d.x + 28*u, d.d.y + 28*u + textH + 24*u + float64(i)*(bh+gap), bw, bh}
	}
}

// update reports the answer: yes, or back (Back or Esc); neither while the player hasn't answered.
func (d *askDialog) update(a *app) (yes, back bool) {
	d.layout(a)
	return a.clicked(d.b[0]), a.in.esc || a.clicked(d.b[1])
}

func (d *askDialog) draw(dst *ebiten.Image, a *app) {
	d.layout(a)
	sc, u := a.sc, a.u
	fillRound(dst, rect{0, 0, a.w, a.h}, 0, withAlpha(sc.ground, 0.8))
	fillRound(dst, d.d, 28*u, sc.tile)
	for i, l := range d.lines {
		drawText(dst, l, a.f.heading, d.d.cx(), d.d.y+28*u+(float64(i)+0.5)*26*u, text.AlignCenter, sc.ink)
	}
	drawButton(dst, a, d.b[0], d.yes, "", btnInk, true)
	drawButton(dst, a, d.b[1], "Back", "", btnGround, true)
}

// wrap breaks s into lines no wider than w in face.
func wrap(s string, face text.Face, w float64) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line == "" || textWidth(line+" "+word, face) <= w {
			line = strings.TrimSpace(line + " " + word)
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
}
