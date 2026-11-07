package client

import (
	"context"
	"errors"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/lan"
)

// LAN retries: every two seconds while the host holds the seat (ticks).
const (
	retryEvery = 2 * 60
	retryFor   = int(hub.DefaultGrace/time.Second) * 60
)

// goneScene says why an online or LAN room ended for this player: the host closed it, removed them, or the connection
// dropped, when a joiner can rejoin their seat. A LAN host that went quiet (its device slept, or left the app) is
// tried again on its own for as long as it holds the seat.
type goneScene struct {
	why    string
	lost   bool   // the connection dropped: the player may rejoin their seat
	code   string // the online room's code
	addr   string // the LAN host to retry, or ""
	token  proto.Token
	left   int  // ticks of retrying left
	trying bool // a retry is under way
	cancel context.CancelFunc
	result chan lanResult

	lv          ui.Rect
	face        *text.GoTextFace
	titleY      float64
	again, home ui.Rect
}

func newGone(n *session) *goneScene {
	g := &goneScene{why: n.end}
	if n.lost {
		g.lost, g.code, g.addr, g.token = true, n.code, n.addr, n.token
	}
	if g.addr != "" {
		g.left, g.result = retryFor, make(chan lanResult, 1)
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
	g.face = a.f.Title
	if ui.TextWidth(g.why, g.face) > v.W-32*u {
		g.face = a.f.Heading
	}
	bw, bh, gap := min(240*u, v.W-32*u), ui.QuietButtonHeight*u, 12*u
	n := 1.0
	if g.lost {
		n = 2
	}
	top := v.Cy() - (40*u+n*bh+(n-1)*gap)/2
	g.titleY = top
	g.again = ui.Rect{X: v.Cx() - bw/2, Y: top + 40*u, W: bw, H: bh}
	g.home = g.again
	if g.lost {
		g.home.Y += bh + gap
	}
}

func (g *goneScene) retrying() bool { return g.left > 0 || g.trying }

func (g *goneScene) update(a *app) error {
	g.layout(a)
	if g.retrying() && g.retry(a) {
		return nil
	}
	switch {
	case g.lost && !g.retrying() && a.clicked(g.again):
		j := newJoin(a, g.addr != "", g.code)
		if g.addr != "" && g.code == "" {
			j.start(a, g.addr, "", false) // a public room: back to its host
		} else {
			j.start(a, "", g.code, true)
		}
		a.next = j
	case a.in.esc || a.clicked(g.home):
		g.stop()
		a.next = newHome()
	}
	return nil
}

// retry tries the LAN host again every two seconds and takes the player back into the room once it answers. When the
// host is back but refuses the seat, or the seat's hold runs out, it stops and leaves Rejoin to the player. It reports
// whether the scene changes.
func (g *goneScene) retry(a *app) bool {
	if g.left > 0 {
		g.left--
	}
	select {
	case r := <-g.result:
		g.trying, g.cancel = false, nil
		var refused lan.Refused
		switch {
		case r.err == nil:
			n := lanSession(a, r.c, r.w, r.addr, r.code)
			a.net, a.next = n, newRoom(n)
			return true
		case errors.As(r.err, &refused):
			g.left, g.why = 0, byeWhy(refused.Reason)
		}
	default:
	}
	switch {
	case g.trying:
	case g.left > 0 && g.left%retryEvery == 0:
		var ctx context.Context
		ctx, g.cancel = context.WithCancel(context.Background())
		g.trying = true
		go joinLAN(ctx, []string{g.addr}, a.set.name(), g.code, g.token, g.result)
	case g.left == 0:
		if g.why == "Host disconnected" {
			g.why = "Connection lost"
		}
		g.lv = ui.Rect{} // the title may need the smaller face
	}
	return false
}

// stop abandons a retry under way; a seat it wins after all is let go at once.
func (g *goneScene) stop() {
	if !g.trying {
		return
	}
	g.cancel()
	go func() {
		if r := <-g.result; r.err == nil {
			r.c.Close()
		}
	}()
}

func (g *goneScene) draw(dst *ebiten.Image, a *app) {
	g.layout(a)
	ui.DrawText(dst, g.why, g.face, g.lv.Cx(), g.titleY, text.AlignCenter, a.sc.Ink)
	switch {
	case g.lost && g.retrying():
		ui.DrawButton(dst, a.ui(), g.again, "Reconnecting", "", ui.ButtonInk, false)
	case g.lost:
		ui.DrawButton(dst, a.ui(), g.again, "Rejoin", "", ui.ButtonInk, true)
	}
	ui.DrawButton(dst, a.ui(), g.home, "Main menu", "", ui.ButtonTile, true)
}
