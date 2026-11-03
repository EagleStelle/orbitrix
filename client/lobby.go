package client

import (
	"log"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
)

// maxRequests is how many joiners signaling lets wait for the host at once.
const maxRequests = 3

// lobbyScene is an online room before its game, laid out like Setup: a Room block (the code, the link to share and,
// for the host, Lock) beside or above a Players block (join requests, then every seat), with Start for the host or
// Ready for a joiner along the bottom.
type lobbyScene struct {
	page
	net     *session
	reqs    []online.Request
	copiedT int // ticks "Copied" stays on Share

	loff               float64
	lseats, lreqs      int
	roomB, playersB    rect
	codeFace           *text.GoTextFace
	codeY, linkY       float64
	share, lock        rect
	noteY              float64
	rows, kick         [core.MaxPlayers]rect
	reqRow, reqNo, req [maxRequests]rect
}

func newLobby(n *session) *lobbyScene { return &lobbyScene{net: n} }

// note is the host's reminder of what keeps the room alive, or signaling's state; "" when there is nothing to say.
func (s *lobbyScene) note() string {
	h := s.net.host
	switch {
	case h == nil:
		return ""
	case h.Offline():
		return "Reconnecting to the server"
	case runtime.GOOS == "js":
		return "Keep this tab open"
	case runtime.GOOS == "android":
		return "Leaving the app pauses the game"
	}
	return ""
}

func (s *lobbyScene) layout(a *app) {
	n := s.net
	if !s.frame(a, true) && s.view.offset == s.loff && len(n.lobby.Seats) == s.lseats && len(s.reqs) == s.lreqs {
		return
	}
	s.lseats, s.lreqs = len(n.lobby.Seats), len(s.reqs)
	u, bp, gap := a.u, s.bp, s.gap
	v := s.view.viewport
	side := v.w >= 760*u
	roomW := v.w
	if side {
		roomW = max(300*u, v.w*0.34)
	}
	pitch := (compactButtonTarget + 8) * u
	roomH := bp + 32*u + 64*u + 28*u + 16*u + 52*u + bp
	if n.host != nil {
		roomH += 12*u + compactButtonTarget*u + 32*u
	}
	playersH := bp + 40*u + float64(len(s.reqs)+len(n.lobby.Seats))*pitch + bp
	total := max(roomH, playersH)
	if !side {
		total = roomH + gap + playersH
	}
	y := s.content(total)
	s.loff = s.view.offset
	s.roomB = rect{v.x, y, roomW, roomH}
	s.playersB = rect{v.x + roomW + gap, y, v.w - roomW - gap, playersH}
	if !side {
		s.playersB = rect{v.x, s.roomB.bottom() + gap, v.w, playersH}
	}

	inner := roomW - 2*bp
	s.codeFace = newFace(srcDisplay, min(48*u, inner/codeLen*0.95), 800)
	s.codeY = y + bp + 32*u + 32*u
	s.linkY = s.codeY + 32*u + 14*u
	s.share = rect{s.roomB.x + bp, s.linkY + 30*u, inner, 52 * u}
	s.lock = rect{s.roomB.x + bp, s.share.bottom() + 12*u, inner, compactButtonTarget * u}
	s.noteY = s.lock.bottom() + 16*u

	p := s.playersB
	ry := p.y + bp + 40*u
	for i := range s.reqs {
		r := rect{p.x + bp, ry + float64(i)*pitch, p.w - 2*bp, compactButtonHeight * u}
		s.reqRow[i] = r
		in := r.inset(5 * u) // the answers sit inside the outlined row
		s.req[i] = rect{in.right() - 88*u, in.y, 88 * u, in.h}
		s.reqNo[i] = rect{s.req[i].x - 6*u - 56*u, in.y, 56 * u, in.h}
	}
	ry += float64(len(s.reqs)) * pitch
	for i := range n.lobby.Seats {
		r := rect{p.x + bp, ry + float64(i)*pitch, p.w - 2*bp, compactButtonHeight * u}
		s.rows[i] = r
		s.kick[i] = rect{r.right() - r.h + 5*u, r.y + 5*u, r.h - 10*u, r.h - 10*u}
	}
}

func (s *lobbyScene) update(a *app) error {
	n := s.net
	n.pump(a)
	if n.route(a, s) {
		return nil
	}
	if h := n.host; h != nil {
		s.reqs = h.Requests(s.reqs[:0])
		// Nobody new can sit down once the seats are taken: turn them away rather than ask.
		if s.openSeats() == 0 {
			for _, r := range s.reqs {
				h.Deny(r.Peer)
			}
			s.reqs = s.reqs[:0]
		}
		s.reqs = s.reqs[:min(len(s.reqs), maxRequests)]
	}
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	if s.copiedT > 0 {
		s.copiedT--
	}
	if s.backed(a) {
		n.leave(a)
		a.net, a.next = nil, newOnline(a, "")
		return nil
	}
	if a.clicked(s.action) && s.canAct() {
		s.act()
		return nil
	}
	if !s.inBody(a) {
		return nil
	}
	if a.clicked(s.share) && platform.Share(joinLink(s.code())) {
		s.copiedT = 120
	}
	h := n.host
	if h == nil {
		return nil
	}
	if a.clicked(s.lock) {
		h.Lock(!h.Locked())
	}
	for i, r := range s.reqs {
		switch {
		case a.clicked(s.req[i]):
			h.Approve(r.Peer)
		case a.clicked(s.reqNo[i]):
			h.Deny(r.Peer)
		}
	}
	for i := range n.lobby.Seats {
		if s.kickable(i) && a.clicked(s.kick[i]) {
			if err := h.Room.Kick(i); err != nil {
				log.Printf("kick seat %d: %v", i, err)
			}
		}
	}
	return nil
}

func (s *lobbyScene) code() string {
	if s.net.host != nil {
		return s.net.host.Code()
	}
	return s.net.code
}

func (s *lobbyScene) openSeats() int {
	k := 0
	for _, st := range s.net.lobby.Seats {
		if st.State == proto.SeatOpen {
			k++
		}
	}
	return k
}

// kickable reports whether the host can remove seat i's player: someone else holds it, here or away.
func (s *lobbyScene) kickable(i int) bool {
	st := s.net.lobby.Seats[i].State
	return s.net.host != nil && i != s.net.seat && (st == proto.SeatHere || st == proto.SeatAway)
}

// canAct reports whether the bottom action works: the host starts once every player who is here is ready (open
// seats go to bots); a joiner toggles Ready once seated.
func (s *lobbyScene) canAct() bool {
	n := s.net
	if !n.got || n.seat < 0 {
		return false
	}
	if n.host == nil {
		return true
	}
	for _, st := range n.lobby.Seats {
		if st.State == proto.SeatAway || st.State == proto.SeatHere && !st.Ready {
			return false
		}
	}
	return true
}

// act starts the game (the host, filling open seats with bots) or flips Ready (a joiner).
func (s *lobbyScene) act() {
	n := s.net
	if n.host == nil {
		n.link.Send(proto.Ready{On: !n.lobby.Seats[n.seat].Ready})
		return
	}
	for i, st := range n.lobby.Seats {
		if st.State == proto.SeatOpen {
			_ = n.host.Room.Replace(i, defaultAI(i))
		}
	}
	if err := n.host.Room.Start(); err != nil {
		log.Printf("start: %v", err) // a player dropped or unreadied meanwhile; the next Lobby shows who
	}
}

// defaultAI is the bot Setup would give seat i.
func defaultAI(i int) ai.Strategy {
	return ai.Strategy((i - 1 + int(ai.NumStrategies)) % int(ai.NumStrategies))
}

// actionLabel is the bottom button's label and style.
func (s *lobbyScene) actionLabel() (string, btnStyle) {
	n := s.net
	switch {
	case !n.got || n.seat < 0:
		return "Joining", btnInk
	case n.host == nil && n.lobby.Seats[n.seat].Ready:
		return "Not ready", btnTile
	case n.host == nil:
		return "Ready", btnInk
	case !s.canAct():
		return "Waiting for players", btnInk
	case s.openSeats() > 0:
		return "Start with bots", btnInk
	}
	return "Start", btnInk
}

// seatState is the right-hand word on seat i's row.
func seatState(st proto.Seat) string {
	switch st.State {
	case proto.SeatOpen:
		return "open"
	case proto.SeatAway:
		return "away"
	case proto.SeatHere:
		if st.Ready {
			return "ready"
		}
		return "not ready"
	}
	return ""
}

func (s *lobbyScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u, n := a.sc, &a.f, a.u, s.net
	s.page.draw(dst, a, "Room")
	label, style := s.actionLabel()
	drawButton(dst, a, s.action, label, "", style, s.canAct())

	b := s.body(dst)
	fillRound(b, s.roomB, 24*u, sc.tile)
	fillRound(b, s.playersB, 24*u, sc.tile)
	x := s.roomB.x + s.bp
	drawText(b, "Code", f.heading, x, s.roomB.y+s.bp+10*u, text.AlignStart, sc.ink)
	code := s.code()
	drawText(b, code, s.codeFace, x, s.codeY, text.AlignStart, sc.ink)
	drawText(b, joinLink(code), f.small, x, s.linkY, text.AlignStart, sc.dim)
	shareLabel := "Share link"
	if s.copiedT > 0 {
		shareLabel = "Copied"
	}
	drawButton(b, a, s.share, shareLabel, "", btnGround, true)
	if h := n.host; h != nil {
		drawText(b, "Lock", f.bodyBold, s.lock.x, s.lock.cy(), text.AlignStart, sc.ink)
		drawToggle(b, a, rect{s.lock.right() - 60*u, s.lock.cy() - 17*u, 60 * u, 34 * u}, h.Locked(), a.over(s.lock))
		drawText(b, s.note(), f.label, x, s.noteY, text.AlignStart, sc.dim)
	}

	p := s.playersB
	drawText(b, "Players", f.heading, p.x+s.bp, p.y+s.bp+10*u, text.AlignStart, sc.ink)
	for i, r := range s.reqs {
		row := s.reqRow[i]
		fillPill(b, row, sc.ground)
		strokeRound(b, row, row.h/2, 2.5*u, sc.ink)
		drawText(b, r.Name+" wants to join", f.labelBold, row.x+16*u, row.cy(), text.AlignStart, sc.ink)
		drawButton(b, a, s.reqNo[i], "No", "", btnTile, true)
		drawButton(b, a, s.req[i], "Let in", "", btnChosen, true)
	}
	for i, st := range n.lobby.Seats {
		r := s.rows[i]
		fillPill(b, r, sc.ground)
		seatDot(b, sc, i, r.x+24*u, r.cy(), 8*u, false, a.set.Marks)
		name := n.seatLabel(i)
		if i == n.seat {
			name += " (you)"
		}
		drawText(b, name, f.labelBold, r.x+seatChoiceX*u, r.cy(), text.AlignStart, sc.ink)
		right := r.right() - 16*u
		if s.kickable(i) {
			ink := drawButtonSurface(b, a, s.kick[i], btnTile, true, a.over(s.kick[i]))
			line(b, s.kick[i].cx()-5*u, s.kick[i].cy()-5*u, s.kick[i].cx()+5*u, s.kick[i].cy()+5*u, 2.5*u, ink)
			line(b, s.kick[i].cx()-5*u, s.kick[i].cy()+5*u, s.kick[i].cx()+5*u, s.kick[i].cy()-5*u, 2.5*u, ink)
			right = s.kick[i].x - 12*u
		}
		if st.State == proto.SeatAI {
			drawPips(b, 1+int(st.AI), right, r.cy(), u, sc.ink)
			continue
		}
		c := sc.dim
		if st.Ready {
			c = sc.ink
		}
		drawText(b, seatState(st), f.label, right, r.cy(), text.AlignEnd, c)
	}
}
