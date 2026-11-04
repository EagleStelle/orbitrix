package client

import (
	"context"
	"errors"
	"log"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/lan"
	"orbitrix/server/signal"
)

// accessNames are a room's two kinds of access. A public room is listed (on the network, or on the server's list); a
// private one takes its code.
var accessNames = [2]string{"Public", "Private"}

// maxRequests is how many joiners signaling lets wait for the host at once.
const maxRequests = 3

// The join request snackbar: its height, and the ticks it takes to rise into place.
const snackH, snackIn = 56, 12

// moreWaiting is the snackbar's count of the requests behind the one it shows.
var moreWaiting = [maxRequests]string{"", "+1", "+2"}

// hostRoom is the Same Wi-Fi or Online room a player hosts from Setup, which keeps its Players and Board blocks as
// This Device has them. Its action reads Open room, then Start, with an invite button beside it that opens the Room
// dialog: Public or Private, and the code with Share. Join requests show one at a time as a snackbar.
type hostRoom struct {
	lan, private bool
	net          *session // nil until the room opens
	opening      bool
	opened       chan openResult
	note         string // why the room didn't open, shown on the action for noteT ticks
	noteT        int
	reqs         []online.Request
	snackPeer    string
	snackMsg     string
	snackT       int  // ticks since the request on the bar arrived
	copiedT      int  // ticks the copy button shows its check
	open         bool // the Room dialog is showing
	draft        bool // the dialog's Private, until Confirm
	inviteCode   string

	// Layout in device px.
	snack, snackNo, snackYes rect
	dv                       rect // the safe area the dialog was laid out for
	dcode                    string
	ddraft                   bool
	panel                    rect
	titleY, noteY            float64
	opts                     [2]rect // Public, Private
	slots                    [proto.CodeLen]rect
	slotFace                 *text.GoTextFace
	codeRunes                []rune
	copyB, confirmB          rect
}

type openResult struct {
	h    *online.Host
	lan  *lan.Host
	code string // a private LAN room's
	err  error
}

// newRoomSetup opens Setup for a room this player is about to host, public or private as Settings says.
func newRoomSetup(a *app, lanRoom bool) *setupScene {
	private := !a.set.OnlinePublic
	if lanRoom {
		private = a.set.WifiPrivate
	}
	return &setupScene{menu: -1, room: &hostRoom{lan: lanRoom, private: private, opened: make(chan openResult, 1)}}
}

// newRoom shows the room n is in: Setup with the room for its host, back from a game; the seats for a joiner.
func newRoom(n *session) scene {
	if n.room != nil {
		return &setupScene{menu: -1, room: &hostRoom{lan: n.addr != "", private: n.private, net: n,
			opened: make(chan openResult, 1)}}
	}
	return &roomScene{net: n}
}

// code is the room's invite code, prepared before opening when private, or "" for a public Same Wi-Fi room.
func (h *hostRoom) code() string {
	switch n := h.net; {
	case n == nil:
		return h.inviteCode
	case n.host != nil:
		return n.host.Code()
	case h.draft && n.code == "":
		return h.inviteCode
	case h.private || h.draft:
		return n.code
	}
	return ""
}

// shareText is what Share sends: the join link online, the code on Same Wi-Fi.
func (h *hostRoom) shareText() string {
	if h.lan {
		return h.code()
	}
	return joinLink(h.code())
}

// hostNote is the host's reminder of what keeps the room alive, or signaling's state; "" when there is nothing to say.
func (h *hostRoom) hostNote() string {
	switch n := h.net; {
	case n == nil:
		return ""
	case n.host != nil && n.host.Offline():
		return "Reconnecting to the server"
	}
	return ""
}

// layoutSnack places the snackbar over the foot of the body, centred above the action: the body's width on a phone,
// at most 560 px from 600 px wide.
func (h *hostRoom) layoutSnack(a *app, viewport, action rect, gap float64) {
	u := a.u
	sw := viewport.w
	if viewport.w >= 600*u {
		sw = min(560*u, viewport.w)
	}
	h.snack = rect{viewport.cx() - sw/2, action.y - gap - snackH*u, sw, snackH * u}
	h.snackYes = rect{h.snack.right() - 8*u - 96*u, h.snack.cy() - 20*u, 96 * u, 40 * u}
	h.snackNo = rect{h.snackYes.x - 4*u - 64*u, h.snackYes.y, 64 * u, 40 * u}
}

// openDialog shows the Room dialog with the room's access as its draft.
func (h *hostRoom) openDialog() {
	h.open, h.draft, h.dv = true, h.private, rect{}
	if h.draft && h.code() == "" {
		h.inviteCode = proto.NewCode()
	}
}

// layoutDialog centres the Room dialog in the pause menu's style: its title, Public and Private side by side, for a
// private room its code in the join page's six full-width tiles (read-only), and Confirm with a round copy button.
func (h *hostRoom) layoutDialog(a *app) {
	v, u := a.safe(), a.u
	code := h.code()
	if v == h.dv && code == h.dcode && h.draft == h.ddraft && h.slotFace != nil {
		return
	}
	h.dv, h.dcode, h.ddraft, h.codeRunes = v, code, h.draft, []rune(code)
	btnH, pad, gap := quietButtonHeight*u, 28*u, 8*u
	w := min(420*u, v.w-32*u)
	inner := w - 2*pad
	copyW := 52 * u
	slot := min(56*u, (inner-float64(proto.CodeLen-1)*gap)/proto.CodeLen)
	hgt := pad + 40*u + 24*u + btnH + 24*u + btnH + pad
	if h.hostNote() != "" {
		hgt += 28 * u
	}
	full := hgt + 20*u + slot // the top stays where the private dialog's is, so a choice never moves under the pointer
	if h.draft {
		hgt = full
	}
	h.panel = rect{v.cx() - w/2, v.cy() - full/2, w, hgt}
	h.titleY = h.panel.y + pad + 20*u
	y := h.panel.y + pad + 40*u + 24*u
	ow := (inner - gap) / 2
	h.opts = [2]rect{{h.panel.x + pad, y, ow, btnH}, {h.panel.x + pad + ow + gap, y, ow, btnH}}
	y += btnH
	if h.draft {
		y += 20 * u
		rowW := proto.CodeLen*slot + (proto.CodeLen-1)*gap
		x := h.panel.cx() - rowW/2
		for i := range h.slots {
			h.slots[i] = rect{x + float64(i)*(slot+gap), y, slot, slot}
		}
		h.slotFace = newFace(srcDisplay, slot*0.5, 700)
		y += slot
	} else if h.slotFace == nil {
		h.slotFace = newFace(srcDisplay, 24*u, 700) // marks the layout done
	}
	h.noteY = y + 18*u
	if h.hostNote() != "" {
		y += 28 * u
	}
	y += 24 * u
	h.confirmB = rect{h.panel.x + pad, y, inner, btnH}
	if h.draft {
		h.confirmB.w -= gap + copyW
		h.copyB = rect{h.confirmB.right() + gap, y + (btnH-copyW)/2, copyW, copyW}
	}
}

// pre runs the room before Setup's own update: the open session's messages and its scene changes, an Open room
// under way, the join requests. It reports whether the scene changes.
func (h *hostRoom) pre(a *app, s *setupScene) bool {
	if h.copiedT > 0 {
		h.copiedT--
	}
	if h.net == nil {
		return h.pollOpen(a, s)
	}
	h.net.pump(a)
	if h.net.route(a, s) {
		return true
	}
	h.requests()
	return false
}

// requests takes in the online host's waiting joiners, turning them away when no seat is open.
func (h *hostRoom) requests() {
	o := h.net.host
	if o == nil {
		return
	}
	h.reqs = o.Requests(h.reqs[:0])
	if h.openSeats() == 0 {
		for _, q := range h.reqs {
			o.Deny(q.Peer)
		}
		h.reqs = h.reqs[:0]
	}
	h.reqs = h.reqs[:min(len(h.reqs), maxRequests)]
	switch {
	case len(h.reqs) == 0:
		h.snackPeer = ""
	case h.reqs[0].Peer != h.snackPeer: // a new request takes the bar, and rises into place
		h.snackPeer, h.snackMsg, h.snackT = h.reqs[0].Peer, h.reqs[0].Name+" wants to join", 0
	}
	h.snackT++
}

// answerSnack takes a tap on the snackbar: Let in or No for the request on it. It reports whether the bar took the
// pointer, which then reaches nothing under it.
func (h *hostRoom) answerSnack(a *app) bool {
	if len(h.reqs) == 0 {
		return false
	}
	o, q := h.net.host, h.reqs[0]
	switch {
	case a.clicked(compactHit(h.snackYes, a.u)):
		o.Approve(q.Peer)
	case a.clicked(compactHit(h.snackNo, a.u)):
		o.Deny(q.Peer)
	}
	return a.over(h.snack)
}

// updateDialog runs the Room dialog. Public and Private change only its draft, which Confirm applies to the room;
// Esc or a tap outside drops it. The copy button works at once.
func (h *hostRoom) updateDialog(a *app) {
	h.layoutDialog(a)
	switch {
	case a.in.esc || a.in.clicked && !h.panel.contains(a.in.x, a.in.y):
		h.open = false
	case a.clicked(h.confirmB):
		h.open = false
		if h.private != h.draft {
			h.private = h.draft
			h.apply(a)
		}
	case a.clicked(h.opts[0]):
		h.draft = false
	case a.clicked(h.opts[1]):
		h.draft = true
		if h.code() == "" {
			h.inviteCode = proto.NewCode()
		}
	case h.draft && h.code() != "" && a.clicked(compactHit(h.copyB, a.u)):
		if platform.Share(h.shareText()) {
			h.copiedT = 120
		}
	}
}

func (h *hostRoom) openSeats() int {
	k := 0
	if h.net != nil {
		for _, st := range h.net.lobby.Seats {
			if st.State == proto.SeatOpen {
				k++
			}
		}
	}
	return k
}

// held reports whether another player holds seat i in the open room.
func (h *hostRoom) held(i int) bool {
	n := h.net
	if n == nil || !n.got || i >= len(n.lobby.Seats) || i == n.seat {
		return false
	}
	st := n.lobby.Seats[i].State
	return st == proto.SeatHere || st == proto.SeatAway
}

// editable reports whether the host can change seat i: any seat before the room opens, then any but the host's own.
func (h *hostRoom) editable(i int) bool { return h.net == nil || i != h.net.seat }

// minPlayers is the fewest players the room can drop to: once it is open, the host's own seat stays.
func (h *hostRoom) minPlayers() int {
	if h.net != nil {
		return max(core.MinPlayers, h.net.seat+1)
	}
	return core.MinPlayers
}

// leave closes the room this player hosts (or one still opening) and goes back to the join page.
func (h *hostRoom) leave(a *app) {
	if h.opening { // a room that opens after all closes again at once
		go func() {
			if o := <-h.opened; o.h != nil {
				o.h.Close()
			} else if o.lan != nil {
				o.lan.Close()
			}
		}()
	}
	if h.net != nil {
		h.net.leave(a)
		a.net = nil
	}
	a.next = newJoin(a, h.lan, "")
}

// apply hands the host's settings to the open room: the board and seats to its hub (in its lobby, and only once the
// board holds the players), the access to the LAN host or signaling.
func (h *hostRoom) apply(a *app) {
	n := h.net
	if n == nil {
		return
	}
	n.private = h.private
	if a.set.Players <= a.set.Cols*a.set.Rows {
		cfg := hubConfig(a)
		if err := n.room.Configure(cfg.Cols, cfg.Rows, cfg.Seats); err != nil {
			log.Printf("room: %v", err)
		}
	}
	if n.lan != nil {
		code := ""
		if h.private {
			if n.code == "" {
				n.code = h.inviteCode
			}
			code = n.code
		}
		n.lan.SetCode(code)
	}
	n.describe()
}

// canAct reports whether the action works: Open room once the board holds the players and a human seat is the host's;
// Start once the board holds them and every player who is here is ready (open seats go to bots).
func (h *hostRoom) canAct(a *app) bool {
	n := h.net
	switch {
	case a.set.Players > a.set.Cols*a.set.Rows:
		return false
	case n == nil:
		return hasHuman(a.set.seats) && !h.opening && h.noteT == 0
	case !n.got || n.seat < 0:
		return false
	}
	for _, st := range n.lobby.Seats {
		if st.State == proto.SeatAway || st.State == proto.SeatHere && !st.Ready {
			return false
		}
	}
	return true
}

// act opens the room, or starts the game, filling open seats with bots.
func (h *hostRoom) act(a *app) {
	n := h.net
	if n == nil {
		h.openRoom(a, a.set.Password)
		return
	}
	for i, st := range n.lobby.Seats {
		if st.State == proto.SeatOpen && i < len(a.set.seats) {
			_ = n.room.Replace(i, defaultAI(i))
			a.set.seats[i] = 1 + int(defaultAI(i)) // so later changes to the room keep the bot
		}
	}
	a.set.save()
	if err := n.room.Start(); err != nil {
		log.Printf("start: %v", err) // a player dropped or unreadied meanwhile; the next Lobby shows who
	}
}

// actionLabel is the action's label.
func (h *hostRoom) actionLabel(a *app) string {
	n := h.net
	switch {
	case n == nil && h.noteT > 0:
		return h.note
	case n == nil && h.opening:
		return "Opening"
	case a.set.Players > a.set.Cols*a.set.Rows:
		return "Board too small"
	case n == nil && !hasHuman(a.set.seats):
		return "Needs a human seat"
	case n == nil:
		return "Open room"
	case !h.canAct(a):
		return "Waiting for players"
	case h.openSeats() > 0:
		return "Start with bots"
	}
	return "Start"
}

// defaultAI is the bot Setup would give seat i.
func defaultAI(i int) ai.Strategy {
	return ai.Strategy((i - 1 + int(ai.NumStrategies)) % int(ai.NumStrategies))
}

// seatState is the chip at the end of a human seat's row.
func seatState(st proto.Seat) chip {
	switch st.State {
	case proto.SeatOpen:
		return chip{"Open", chipOff}
	case proto.SeatAway:
		return chip{"Away", chipOff}
	case proto.SeatHere:
		if st.Ready {
			return chip{"Ready", chipOn}
		}
		return chip{"Not ready", chipDanger}
	}
	return chip{}
}

// openRoom opens the room off the game loop, on the LAN or on the server; pollOpen takes the answer.
func (h *hostRoom) openRoom(a *app, password string) {
	h.opening = true
	cfg, url, relay, name, private := hubConfig(a), signalURL(), a.set.Relay, platform.DeviceName(), h.private
	if private {
		if h.inviteCode == "" {
			h.inviteCode = proto.NewCode()
		}
		cfg.Code = h.inviteCode
	}
	if h.lan && runtime.GOOS != "js" { // a browser never hosts: the constant keeps lan.Open out of its build
		go func() {
			l, err := lan.Open(cfg, name)
			h.opened <- openResult{lan: l, code: cfg.Code, err: err}
		}()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		o, err := online.Open(ctx, url, password, relay, !private, cfg)
		h.opened <- openResult{h: o, err: err}
	}()
}

// pollOpen takes the open room, staying on Setup; asks for the host password when the server wants it (again, when it
// refused the saved one); and otherwise says on the action why it failed. It reports whether the scene changes.
func (h *hostRoom) pollOpen(a *app, s *setupScene) bool {
	if h.noteT > 0 {
		h.noteT--
	}
	var o openResult
	select {
	case o = <-h.opened:
	default:
		return false
	}
	h.opening = false
	switch {
	case o.err == nil && o.lan != nil:
		n := hostSession(o.lan.Room, online.NewLink(o.lan.Me), a.set.name())
		n.lan, n.addr, n.code, n.private = o.lan, o.lan.Addr, o.code, h.private
		h.net, a.net = n, n
	case o.err == nil:
		a.set.save() // keeps the password that worked
		n := hostSession(o.h.Room, o.h.Me, a.set.name())
		n.host, n.private = o.h, h.private
		h.net, a.net = n, n
	case errors.Is(o.err, online.ErrPassword), errors.Is(o.err, online.ErrRefused):
		wrong := a.set.Password != ""
		a.set.Password = ""
		a.set.save()
		a.next = newPassword(s, wrong)
		return true
	case errors.Is(o.err, online.ErrBusy):
		h.note = "Server full"
	case errors.Is(o.err, signal.ErrVersion):
		h.note = "Update Orbitrix"
	case errors.Is(o.err, lan.ErrNoNetwork):
		h.note = "No local network"
	case h.lan:
		h.note = "Can't open a room"
	default:
		h.note = "Can't reach the server"
	}
	if o.err != nil {
		h.noteT = 180
	}
	return false
}

// drawRoomButton draws the round button beside the action that opens the Room dialog, with a key: the room's access
// and its code.
func drawRoomButton(dst *ebiten.Image, a *app, r rect) {
	u := a.u
	ink := drawButtonSurface(dst, a, r, btnTile, true, a.over(r))
	cx, cy := r.cx(), r.cy()
	strokeRound(dst, rect{cx - 12*u, cy - 6*u, 12 * u, 12 * u}, 6*u, 2.25*u, ink) // the bow
	line(dst, cx, cy, cx+12*u, cy, 2.25*u, ink)                                   // the shaft
	line(dst, cx+7*u, cy, cx+7*u, cy+5*u, 2.25*u, ink)                            // the teeth
	line(dst, cx+11*u, cy, cx+11*u, cy+6*u, 2.25*u, ink)
}

// drawDialog draws the Room dialog over an 80% veil of the mat: Public and Private (the draft outlined); for a private
// room the code in its tiles with the copy button, which shows a check for a moment once it has copied; the host's
// note; Confirm and the copy button.
func (h *hostRoom) drawDialog(dst *ebiten.Image, a *app) {
	h.layoutDialog(a)
	sc, f, u := a.sc, &a.f, a.u
	fillRound(dst, rect{0, 0, a.w, a.h}, 0, withAlpha(sc.ground, 0.8))
	fillRound(dst, h.panel, 28*u, sc.tile)
	drawText(dst, "Room", f.title, h.panel.cx(), h.titleY, text.AlignCenter, sc.ink)
	for i, o := range h.opts {
		style := btnGround
		if h.draft == (i == 1) {
			style = btnChosen
		}
		drawButton(dst, a, o, accessNames[i], "", style, true)
	}
	if h.draft {
		drawCodeSlots(dst, a, h.slots[:], h.slotFace, h.codeRunes, false)
		on := h.code() != ""
		hot := a.over(compactHit(h.copyB, u))
		ink := drawButtonSurface(dst, a, h.copyB, btnGround, on, hot)
		fill, _ := buttonColors(sc, btnGround, on, hot)
		cx, cy := h.copyB.cx(), h.copyB.cy()
		if h.copiedT > 0 { // a check: copied
			line(dst, cx-6*u, cy, cx-2*u, cy+4*u, 2.25*u, ink)
			line(dst, cx-2*u, cy+4*u, cx+6*u, cy-4*u, 2.25*u, ink)
		} else { // two sheets, the front one over the back
			strokeRound(dst, rect{cx - 8*u, cy - 8*u, 12 * u, 12 * u}, 3*u, 2*u, ink)
			fillRound(dst, rect{cx - 4*u, cy - 4*u, 12 * u, 12 * u}, 3*u, fill)
			strokeRound(dst, rect{cx - 4*u, cy - 4*u, 12 * u, 12 * u}, 3*u, 2*u, ink)
		}
	}
	if note := h.hostNote(); note != "" {
		drawText(dst, note, f.label, h.panel.cx(), h.noteY, text.AlignCenter, sc.dim)
	}
	drawButton(dst, a, h.confirmB, "Confirm", "", btnInk, true)
}

// drawSnack shows the oldest join request as a snackbar over the foot of the page, in the inverse of the page (ink,
// with the mat's colour for its words, as the primary action is): who wants in, how many more wait, and No and Let in.
// Each request rises into place as it reaches the bar.
func (h *hostRoom) drawSnack(dst *ebiten.Image, a *app) {
	if len(h.reqs) == 0 {
		return
	}
	sc, f, u := a.sc, &a.f, a.u
	p := min(1, float64(h.snackT)/snackIn)
	e := 1 - (1-p)*(1-p)*(1-p) // ease out
	dy := (1 - e) * 16 * u
	bar, no, yes := h.snack, h.snackNo, h.snackYes
	bar.y, no.y, yes.y = bar.y+dy, no.y+dy, yes.y+dy
	fillPill(dst, bar, withAlpha(sc.ink, e))
	drawText(dst, h.snackMsg, f.labelBold, bar.x+24*u, bar.cy(), text.AlignStart, withAlpha(sc.ground, e))
	if more := moreWaiting[len(h.reqs)-1]; more != "" {
		x := bar.x + 24*u + textWidth(h.snackMsg, f.labelBold) + 10*u
		drawText(dst, more, f.label, x, bar.cy(), text.AlignStart, withAlpha(sc.ground, 0.6*e))
	}
	if a.over(compactHit(no, u)) {
		fillPill(dst, no, withAlpha(sc.ground, 0.16*e))
	}
	drawText(dst, "No", f.labelBold, no.cx(), no.cy(), text.AlignCenter, withAlpha(sc.ground, e))
	fill := sc.ground
	if a.over(compactHit(yes, u)) {
		fill = sc.tileHi
	}
	fillPill(dst, yes, withAlpha(fill, e))
	drawText(dst, "Let in", f.labelBold, yes.cx(), yes.cy(), text.AlignCenter, withAlpha(sc.ink, e))
}

// roomScene is a joiner's room: the seats, each with its player or bot and state, and Ready along the bottom. The board
// and the room's settings are the host's.
type roomScene struct {
	page
	net *session

	loff  float64
	lkeyN int
	block rect
	rows  [core.MaxPlayers]rect
}

func (r *roomScene) layout(a *app) {
	n := len(r.net.lobby.Seats)
	if !r.frame(a, true) && r.view.offset == r.loff && n == r.lkeyN {
		return
	}
	r.lkeyN = n
	u, bp := a.u, r.bp
	v := r.view.viewport
	pitch := (compactButtonTarget + 8) * u
	h := bp + 40*u + float64(n)*pitch + bp
	y := r.content(h)
	r.loff = r.view.offset
	r.block = rect{v.x, y, v.w, h}
	for i := range n {
		r.rows[i] = rect{v.x + bp, y + bp + 40*u + float64(i)*pitch, v.w - 2*bp, compactButtonHeight * u}
	}
}

func (r *roomScene) update(a *app) error {
	n := r.net
	n.pump(a)
	if n.route(a, r) {
		return nil
	}
	r.layout(a)
	if r.scrolled(a) {
		r.layout(a)
	}
	switch {
	case r.backed(a):
		n.leave(a)
		a.net, a.next = nil, newJoin(a, n.addr != "", "")
	case a.clicked(r.action) && n.got && n.seat >= 0:
		n.link.Send(proto.Ready{On: !n.lobby.Seats[n.seat].Ready})
	}
	return nil
}

func (r *roomScene) draw(dst *ebiten.Image, a *app) {
	r.layout(a)
	sc, f, u, n := a.sc, &a.f, a.u, r.net
	r.page.draw(dst, a, "Room")
	label, style, ok := "Joining", btnInk, n.got && n.seat >= 0
	if ok {
		label = "Ready"
		if n.lobby.Seats[n.seat].Ready {
			label, style = "Not ready", btnTile
		}
	}
	drawButton(dst, a, r.action, label, "", style, ok)
	b := r.body(dst)
	fillRound(b, r.block, 24*u, sc.tile)
	drawText(b, "Players", f.heading, r.block.x+r.bp, r.block.y+r.bp+10*u, text.AlignStart, sc.ink)
	for i, st := range n.lobby.Seats {
		row := r.rows[i]
		fillPill(b, row, sc.ground)
		seatDot(b, sc, i, row.x+24*u, row.cy(), 8*u, false, a.set.Marks)
		name := n.seatLabel(i)
		if i == n.seat {
			name += " (you)"
		}
		drawText(b, name, f.labelBold, row.x+seatChoiceX*u, row.cy(), text.AlignStart, sc.ink)
		right := row.right() - 16*u
		if st.State == proto.SeatAI {
			drawPips(b, 1+int(st.AI), right, row.cy(), u, sc.ink)
			continue
		}
		drawChip(b, a, seatState(st), right, row.cy(), chipHeight*u)
	}
}

// seat is how Setup shows seat i in a room: its label (" (you)" on the host's), its type, and once the room is open
// the state of a human seat.
func (h *hostRoom) seat(a *app, i int) (label string, opt int, state chip) {
	n := h.net
	if n == nil || !n.got || i >= len(n.lobby.Seats) {
		opt = a.set.seats[i]
		label = a.set.label(a.set.seats, i)
		if i == firstHuman(a.set.seats) {
			label += " (you)"
		}
		return label, opt, chip{}
	}
	st := n.lobby.Seats[i]
	label = n.seatLabel(i)
	if i == n.seat {
		label += " (you)"
	}
	if st.State == proto.SeatAI {
		return label, 1 + int(st.AI), chip{}
	}
	return label, seatHuman, seatState(st)
}

// seatType is seat i's type as seat shows it: the lobby's once the room is open, Setup's before.
func (h *hostRoom) seatType(a *app, i int) int {
	n := h.net
	switch {
	case n == nil || !n.got || i >= len(n.lobby.Seats):
		return a.set.seats[i]
	case n.lobby.Seats[i].State == proto.SeatAI:
		return 1 + int(n.lobby.Seats[i].AI)
	}
	return seatHuman
}

// firstHuman is the seat the host takes when the room opens: its first human one.
func firstHuman(seats []int) int {
	for i, s := range seats {
		if s == seatHuman {
			return i
		}
	}
	return -1
}
