package client

import (
	"strings"

	"orbitrix/client/internal/ui"

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

	lv    ui.Rect // the safe area it was laid out for
	d     ui.Rect
	b     [2]ui.Rect
	lines []string
}

func (d *askDialog) layout(a *app) {
	v, u := a.safe(), a.u
	if v == d.lv && d.lines != nil {
		return
	}
	d.lv = v
	bw, bh, gap, lh := min(300*u, v.W-88*u), ui.QuietButtonHeight*u, 12*u, 26*u
	d.lines = wrap(d.text, a.f.Heading, bw)
	textH := float64(len(d.lines)) * lh
	h := 28*u + textH + 24*u + 2*bh + gap + 28*u
	d.d = ui.Rect{X: v.Cx() - bw/2 - 28*u, Y: v.Cy() - h/2, W: bw + 56*u, H: h}
	for i := range d.b {
		d.b[i] = ui.Rect{X: d.d.X + 28*u, Y: d.d.Y + 28*u + textH + 24*u + float64(i)*(bh+gap), W: bw, H: bh}
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
	ui.FillRound(dst, ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, 0, ui.WithAlpha(sc.Ground, 0.8))
	ui.FillRound(dst, d.d, 28*u, sc.Tile)
	for i, l := range d.lines {
		ui.DrawText(dst, l, a.f.Heading, d.d.Cx(), d.d.Y+28*u+(float64(i)+0.5)*26*u, text.AlignCenter, sc.Ink)
	}
	ui.DrawButton(dst, a.ui(), d.b[0], d.yes, "", ui.ButtonInk, true)
	ui.DrawButton(dst, a.ui(), d.b[1], "Back", "", ui.ButtonGround, true)
}

// wrap breaks s into lines no wider than w in face.
func wrap(s string, face text.Face, w float64) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line == "" || ui.TextWidth(line+" "+word, face) <= w {
			line = strings.TrimSpace(line + " " + word)
			continue
		}
		lines = append(lines, line)
		line = word
	}
	return append(lines, line)
}
