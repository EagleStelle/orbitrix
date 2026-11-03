package client

import (
	"context"
	hexenc "encoding/hex"
	"errors"
	"image"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/online"
	"orbitrix/proto"
	"orbitrix/server/hub"
)

// codeLen is a room code's length.
const codeLen = 6

// onlineScene is the Online page: one centred column with a Host block that leads to Setup above a Join block (the
// code's six slots, typed on the device's own keyboard, then Join), create before join as on Meet or Discord.
type onlineScene struct {
	page
	code    *field
	joining bool
	waiting atomic.Bool // the join request is in front of the host
	status  string      // why the last try failed, or ""
	cancel  context.CancelFunc
	result  chan joinResult

	loff         float64
	joinB, hostB rect
	slots        [codeLen]rect
	row          rect // the slots: pressing them opens the keyboard
	slotFace     *text.GoTextFace
	statusY      float64
	join, host   rect
}

type joinResult struct {
	c     hub.Conn
	err   error
	code  string
	token proto.Token
}

// newOnline opens the Online page with code filled in: from a /join/ link, or the room this player may rejoin. With a
// mouse there is a physical keyboard, so the code takes typing straight away; a touch screen opens its keyboard when
// the player presses the slots.
func newOnline(a *app, code string) *onlineScene {
	if code == "" && a.set.Rejoin != nil {
		code = a.set.Rejoin.Code
	}
	s := &onlineScene{code: codeField(), result: make(chan joinResult, 1)}
	s.code.set(code)
	s.code.focused = !a.in.finger
	return s
}

func (s *onlineScene) layout(a *app) {
	if !s.frame(a, false) && s.view.offset == s.loff {
		return
	}
	u, bp := a.u, s.bp
	v := s.view.viewport
	w := min(v.w, 640*u)
	x := v.cx() - w/2
	slot := min(56*u, (w-2*bp-(codeLen-1)*8*u)/codeLen)
	joinH := bp + 32*u + slot + 44*u + 52*u + bp
	hostH := bp + 32*u + 52*u + bp
	y := s.centred(hostH + s.gap + joinH)
	s.loff = s.view.offset
	s.hostB = rect{x, y, w, hostH}
	s.joinB = rect{x, s.hostB.bottom() + s.gap, w, joinH}

	rowW := codeLen*slot + (codeLen-1)*8*u
	s.row = rect{s.joinB.cx() - rowW/2, s.joinB.y + bp + 32*u, rowW, slot}
	for i := range s.slots {
		s.slots[i] = rect{s.row.x + float64(i)*(slot+8*u), s.row.y, slot, slot}
	}
	s.code.caret = image.Rect(int(s.row.x), int(s.row.y), int(s.row.right()), int(s.row.bottom()))
	s.slotFace = newFace(srcDisplay, slot*0.5, 700)
	s.statusY = s.row.bottom() + 22*u
	s.join = rect{x + bp, s.row.bottom() + 44*u, w - 2*bp, 52 * u}
	s.host = rect{x + bp, s.hostB.y + bp + 32*u, w - 2*bp, 52 * u}
}

func (s *onlineScene) update(a *app) error {
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	in := s.inBody(a)
	if s.joining {
		select {
		case r := <-s.result:
			s.joined(a, r)
		default:
			if s.backed(a) || in && a.clicked(s.join) { // Join reads Cancel meanwhile
				s.cancel()
			}
		}
		return nil
	}
	if s.backed(a) {
		a.next = newHome()
		return nil
	}
	if in && a.clicked(s.host) {
		a.next = newHostSetup()
		return nil
	}
	if in {
		s.code.tap(a, s.row)
	}
	if s.code.update(a) || in && a.clicked(s.join) {
		if code := s.code.value(); len(code) == codeLen {
			s.start(a, code)
		}
	}
	return nil
}

// start asks the room's host to let this player in, off the game loop. A player coming back to the room they left
// carries its rejoin token, and the host's game lets them straight back in.
func (s *onlineScene) start(a *app, code string) {
	var token proto.Token
	if r := a.set.Rejoin; r != nil && r.Code == code {
		_, _ = hexenc.Decode(token[:], []byte(r.Token)) // a damaged token joins as someone new
	}
	s.code.focus(a, false)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.joining, s.status = cancel, true, ""
	s.waiting.Store(false)
	relay, url := a.set.Relay, signalURL()
	go func() {
		c, err := online.Join(ctx, url, code, onlineName, token, relay, func() { s.waiting.Store(true) })
		cancel()
		s.result <- joinResult{c, err, code, token}
	}()
}

func (s *onlineScene) joined(a *app, r joinResult) {
	s.joining = false
	s.code.focused = !a.in.finger
	switch {
	case errors.Is(r.err, context.Canceled):
	case errors.Is(r.err, online.ErrRefused):
		s.status = "That code didn't work"
	case r.err != nil:
		s.status = "Can't reach the server"
	default:
		n := newSession(online.NewLink(r.c))
		n.code = r.code
		n.link.Send(proto.Hello{Name: onlineName, Token: r.token})
		a.net, a.next = n, newLobby(n)
	}
}

func (s *onlineScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	s.page.draw(dst, a, "Online")
	b := s.body(dst)
	fillRound(b, s.joinB, 24*u, sc.tile)
	fillRound(b, s.hostB, 24*u, sc.tile)
	drawText(b, "Join", f.heading, s.joinB.x+s.bp, s.joinB.y+s.bp+10*u, text.AlignStart, sc.ink)
	drawText(b, "Host", f.heading, s.hostB.x+s.bp, s.hostB.y+s.bp+10*u, text.AlignStart, sc.ink)

	// The code's slots are printed tiles, like the board's; while the code takes typing, the next one is outlined.
	code := s.code.shown
	for i, r := range s.slots {
		fillRound(b, r, r.w*tileRadius, sc.ground)
		if i < len(code) {
			drawText(b, string(code[i]), s.slotFace, r.cx(), r.cy(), text.AlignCenter, sc.ink)
		} else if i == len(code) && s.code.focused {
			strokeRound(b, r, r.w*tileRadius, 2.5*u, sc.ink)
		}
	}

	status, c := s.status, sc.ink
	if s.joining {
		status, c = "Connecting", sc.dim
		if s.waiting.Load() {
			status = "Waiting for the host"
		}
	}
	drawText(b, status, f.label, s.joinB.cx(), s.statusY, text.AlignCenter, c)
	switch {
	case s.joining:
		drawButton(b, a, s.join, "Cancel", "", btnGround, true)
	case len(code) == codeLen:
		drawButton(b, a, s.join, "Join", "", btnInk, true)
	default:
		drawButton(b, a, s.join, "Join", "", btnTile, false) // the mat's disabled fill would vanish on the block
	}
	if s.joining {
		drawButton(b, a, s.host, "Set up a room", "", btnTile, false)
	} else {
		drawButton(b, a, s.host, "Set up a room", "", btnGround, true)
	}
}
