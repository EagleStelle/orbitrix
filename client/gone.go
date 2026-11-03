package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// goneScene says why an online room ended for this player: the host closed it, removed them, or the connection
// dropped, when a joiner can rejoin their seat.
type goneScene struct {
	why    string
	rejoin string // the room code to rejoin, or ""

	lv          rect
	face        *text.GoTextFace
	titleY      float64
	again, home rect
}

func newGone(n *session) *goneScene {
	g := &goneScene{why: n.end}
	if n.lost {
		g.rejoin = n.code
	}
	return g
}

func (g *goneScene) layout(a *app) {
	v := a.safe()
	if v == g.lv {
		return
	}
	g.lv = v
	u := a.u
	g.face = a.f.title
	if textWidth(g.why, g.face) > v.w-32*u {
		g.face = a.f.heading
	}
	bw, bh, gap := min(240*u, v.w-32*u), quietButtonHeight*u, 12*u
	n := 1.0
	if g.rejoin != "" {
		n = 2
	}
	top := v.cy() - (40*u+n*bh+(n-1)*gap)/2
	g.titleY = top
	g.again = rect{v.cx() - bw/2, top + 40*u, bw, bh}
	g.home = g.again
	if g.rejoin != "" {
		g.home.y += bh + gap
	}
}

func (g *goneScene) update(a *app) error {
	g.layout(a)
	switch {
	case g.rejoin != "" && a.clicked(g.again):
		o := newOnline(a, g.rejoin)
		o.start(a, o.code.value())
		a.next = o
	case a.in.esc || a.clicked(g.home):
		a.next = newHome()
	}
	return nil
}

func (g *goneScene) draw(dst *ebiten.Image, a *app) {
	g.layout(a)
	drawText(dst, g.why, g.face, g.lv.cx(), g.titleY, text.AlignCenter, a.sc.ink)
	if g.rejoin != "" {
		drawButton(dst, a, g.again, "Rejoin", "", btnInk, true)
	}
	drawButton(dst, a, g.home, "Main menu", "", btnTile, true)
}
