package client

import (
	"context"
	"errors"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
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

// moreWaiting is the snackbar's count of the requests behind the one it shows.
var moreWaiting = [maxRequests]string{"", "+1", "+2"}

// hostRoom is the Same Wi-Fi or Online room a player hosts from Setup, which keeps its Players and Board blocks as
// This Device has them. Its action reads Open room, then Start, with an invite button beside it that opens the Invite
// players dialog: access, QR, code, Copy code and Copy link together; the first Open room shows it too. Join requests
// show as a snackbar.
type hostRoom struct {
	lan, private bool
	net          *session // nil until the room opens
	opening      bool
	opened       chan openResult
	note         string // why the room didn't open, shown on the action for noteT ticks
	noteT        int
	reqs         []online.Request
	snackPeer    string
	snack        snackbar
	open         bool // the Invite players dialog is showing
	draft        bool // the dialog's Private, until Confirm
	inviteCode   string
	invite       inviteView
	replyT       int // ticks, for looking out for a server-free room's replies now and then

	// Layout in device px.
	opts     [2]ui.Rect // Public, Private
	confirmB ui.Rect
}

type openResult struct {
	h      *online.Host
	lan    *lan.Host
	direct *online.Direct
	code   string // a private LAN room's
	err    error
}

// newRoomSetup opens Setup for a room this player is about to host, public or private as Settings says. A browser's
// Same Wi-Fi room is server-free, and only its invites let anyone in.
func newRoomSetup(a *app, lanRoom bool) *setupScene {
	private := !a.set.OnlinePublic
	if lanRoom {
		private = a.set.WifiPrivate && runtime.GOOS != "js"
	}
	return &setupScene{menu: -1, room: &hostRoom{lan: lanRoom, private: private, opened: make(chan openResult, 1)}}
}

// newRoom shows the room n is in: Setup with the room for its host, back from a game; the seats for a joiner.
func newRoom(n *session) scene {
	if n.room != nil {
		return &setupScene{menu: -1, room: &hostRoom{lan: n.wifi(), private: n.private, net: n,
			opened: make(chan openResult, 1)}}
	}
	return &roomScene{net: n}
}

// direct reports a server-free room: a browser's Same Wi-Fi room.
func (h *hostRoom) direct() bool { return h.lan && runtime.GOOS == "js" }

// invitation keeps the QR link separate from the written code copied by every platform.
func (h *hostRoom) invitation() (link, code string) {
	n := h.net
	if n == nil || h.open && h.draft != h.private {
		return "", ""
	}
	if n.direct != nil {
		code = n.direct.Invite()
		if code != "" {
			link = platform.Address() + "/play#i=" + code
		}
		return link, code
	}
	code = h.code()
	if n.host != nil {
		if n.host.Offline() {
			return "", code
		}
		return joinLink(code), code
	}
	if n.lan != nil && n.lan.Web {
		link = "http://" + n.addr + "/join/" + code
	}
	return link, code
}

// code is the room's short invite code, prepared before opening when private.
func (h *hostRoom) code() string {
	draft := h.open && h.draft
	switch n := h.net; {
	case n == nil:
		return h.inviteCode
	case n.host != nil:
		return n.host.Code()
	case n.lan != nil && !h.private && !draft:
		return n.lan.Code
	case draft && n.code == "":
		return h.inviteCode
	case h.private || draft:
		return n.code
	}
	return ""
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

// openDialog shows the Invite players dialog with the room's access as its draft.
func (h *hostRoom) openDialog() {
	h.open, h.draft = true, h.private
	if h.draft && h.code() == "" {
		h.inviteCode = proto.NewCode()
	}
}

// layoutDialog uses the shared invitation surface for both access and joining.
func (h *hostRoom) layoutDialog(a *app) {
	h.invite.settings = true
	link, _ := h.invitation()
	h.invite.qr.Set(link)
	h.invite.layout(a)
	h.opts, h.confirmB = h.invite.opts, h.invite.done
}

// pre runs the room before Setup's own update: the open session's messages and its scene changes, an Open room
// under way, the join requests. It reports whether the scene changes.
func (h *hostRoom) pre(a *app, s *setupScene) bool {
	if h.net == nil {
		return h.pollOpen(a, s)
	}
	h.net.pump(a)
	if h.net.route(a, s) {
		return true
	}
	h.requests()
	if d := h.net.direct; d != nil {
		if h.replyT++; h.replyT%15 == 0 { // a quarter second: localStorage is no place to read every frame
			if reply := platform.Reply(); reply != "" {
				d.Take(reply)
			}
		}
	}
	return false
}

// requests takes in the online host's waiting joiners once the lobby is known, turning them away when no seat is open.
func (h *hostRoom) requests() {
	o := h.net.host
	if o == nil || !h.net.got {
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
		h.snackPeer = h.reqs[0].Peer
		h.snack.message = h.reqs[0].Name + " wants to join"
		h.snack.actions, h.snack.ticks = [2]string{"No", "Let in"}, 0
	}
	if len(h.reqs) > 0 {
		h.snack.more = moreWaiting[len(h.reqs)-1]
	}
}

// answerSnack takes a tap on the snackbar: Let in or No for the request on it. It reports whether the bar took the
// pointer, which then reaches nothing under it.
func (h *hostRoom) answerSnack(a *app) bool {
	if len(h.reqs) == 0 {
		return false
	}
	o, q := h.net.host, h.reqs[0]
	action, over := h.snack.update(a)
	switch action {
	case 1:
		o.Approve(q.Peer)
	case 0:
		o.Deny(q.Peer)
	}
	return over
}

// Access remains a draft until Confirm; no unconfirmed QR or code is shared.
func (h *hostRoom) updateDialog(a *app) {
	h.layoutDialog(a)
	link, code := h.invitation()
	if h.invite.update(a, link, code) {
		h.open = false
		return
	}
	switch {
	case a.clicked(h.confirmB):
		if h.net == nil {
			if h.canAct(a) {
				h.private = h.draft
				h.openRoom(a, a.set.Password)
			}
		} else if h.private != h.draft {
			h.private = h.draft
			h.apply(a)
		} else {
			h.open = false
		}
	case !h.direct() && a.clicked(h.opts[0]):
		h.draft = false
	case !h.direct() && a.clicked(h.opts[1]):
		h.draft = true
		if h.code() == "" {
			h.inviteCode = proto.NewCode()
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
			switch o := <-h.opened; {
			case o.h != nil:
				o.h.Close()
			case o.lan != nil:
				o.lan.Close()
			case o.direct != nil:
				o.direct.Close()
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
	if mapCanStart(a.set) {
		cfg := hubConfig(a)
		if err := n.room.SetMap(cfg.Map); err != nil {
			log.Printf("room map: %v", err)
			return
		}
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
	case !mapCanStart(a.set):
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
		h.openDialog() // the invite shows as soon as the room opens
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
	case !mapCanStart(a.set):
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
func seatState(st proto.Seat) ui.Chip {
	switch st.State {
	case proto.SeatOpen:
		return ui.Chip{Word: "Open", Kind: ui.ChipOff}
	case proto.SeatAway:
		return ui.Chip{Word: "Away", Kind: ui.ChipOff}
	case proto.SeatHere:
		if st.Ready {
			return ui.Chip{Word: "Ready", Kind: ui.ChipOn}
		}
		return ui.Chip{Word: "Not ready", Kind: ui.ChipDanger}
	}
	return ui.Chip{}
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
	if h.lan && runtime.GOOS != "js" { // a browser never hosts on the LAN: the constant keeps lan.Open out of its build
		go func() {
			l, err := lan.Open(cfg, name)
			h.opened <- openResult{lan: l, code: cfg.Code, err: err}
		}()
		return
	}
	if h.lan { // a browser's: server-free, and only invites let anyone in
		go func() {
			d, err := online.OpenDirect(cfg)
			h.opened <- openResult{direct: d, err: err}
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
	case o.err == nil && o.direct != nil:
		n := hostSession(o.direct.Room, o.direct.Me, a.set.name())
		n.direct = o.direct
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

// drawRoomButton draws the round button beside the action that opens the Invite players dialog: a person with a plus.
func drawRoomButton(dst *ebiten.Image, a *app, r ui.Rect) {
	u := a.u
	hot := a.over(r)
	ink := ui.DrawButtonSurface(dst, a.ui(), r, ui.ButtonTile, true, hot)
	fill, _ := ui.ButtonColors(a.sc, ui.ButtonTile, true, hot)
	cx, cy := r.Cx(), r.Cy()
	ui.Ring(dst, cx-4*u, cy-4.5*u, 4.5*u, 2.25*u, ink)
	ui.Ring( // the head
		dst, cx-4*u, cy+10*u, 8*u, 2.25*u, ink)
	ui.FillRound( // the shoulders: a ring's top half,
		dst, ui.Rect{X: cx - 13*u, Y: cy + 10*u, W: 18 * u, H: 9 * u}, 0, fill)
	ui. // its lower half covered
		Line(dst, cx+8*u, cy-6*u, cx+8*u, cy+2*u, 2.25*u, ink)
	ui.Line( // the plus
		dst, cx+4*u, cy-2*u, cx+12*u, cy-2*u, 2.25*u, ink)
}

// drawDialog keeps access and the active invitation in one room surface.
func (h *hostRoom) drawDialog(dst *ebiten.Image, a *app) {
	h.layoutDialog(a)
	sc, f := a.sc, &a.f
	link, code := h.invitation()
	// The action stays short beside the copy buttons; why the room can't open shows where the QR goes.
	action, enabled := "Done", true
	switch {
	case h.net == nil && h.opening:
		action, enabled = "Opening", false
	case h.net == nil:
		action, enabled = "Open room", h.canAct(a)
	case h.private != h.draft:
		action = "Confirm"
	}
	ui.FillRound(dst, ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, 0, ui.WithAlpha(sc.Ground, 0.8))
	h.invite.draw(dst, a, "Invite players", link, code, action, enabled)
	if h.direct() {
		r := h.opts[0]
		r.W = h.opts[1].Right() - r.X
		ui.FillPill(dst, r, sc.Ground)
		ui.DrawText(dst, "Invite only", f.LabelBold, r.Cx(), r.Cy(), text.AlignCenter, sc.Dim)
	} else {
		for i, o := range h.opts {
			style := ui.ButtonGround
			if h.draft == (i == 1) {
				style = ui.ButtonChosen
			}
			ui.DrawButton(dst, a.ui(), o, accessNames[i], "", style, true)
		}
	}
	if link == "" {
		state := "Preparing invite"
		switch {
		case h.noteT > 0:
			state = h.note
		case h.opening:
			state = "Opening room"
		case h.net == nil && !h.canAct(a):
			state = h.actionLabel(a) // Board too small, Needs a human seat
		case h.net == nil:
			state = "Room not open"
		case h.private != h.draft:
			state = "Confirm to update invite"
		case h.net.lan != nil && !h.net.lan.Web:
			state = "Browser game not included"
		case h.hostNote() != "":
			state = h.hostNote()
		}
		h.invite.drawState(dst, a, state)
	}
}

// drawCopyButton draws a round copy button: three tiles for the code, or two chain rings for the link; a check for a
// moment once it has copied.
func drawCopyButton(dst *ebiten.Image, a *app, r ui.Rect, link, on, copied bool) {
	u := a.u
	ink := ui.DrawButtonSurface(dst, a.ui(), r, ui.ButtonGround, on, a.over(ui.CompactHit(r, u)))
	cx, cy := r.Cx(), r.Cy()
	switch {
	case copied:
		ui.Line(dst, cx-6*u, cy, cx-2*u, cy+4*u, 2.25*u, ink)
		ui.Line(dst, cx-2*u, cy+4*u, cx+6*u, cy-4*u, 2.25*u, ink)
	case link:
		var g ebiten.GeoM
		g.Rotate(-math.Pi / 4)
		g.Translate(cx, cy)
		for _, x := range [2]float64{-11, -2} {
			ui.DrawShapes(dst, ui.QueueRound(ui.Rect{X: x * u, Y: -4.5 * u, W: 13 * u, H: 9 * u}, 4.5*u, 2.25*u, &g, ink))
		}
	default:
		for i := range 3 {
			ui.StrokeRound(dst, ui.Rect{X: cx + (float64(i)*9-13)*u, Y: cy - 4*u, W: 8 * u, H: 8 * u}, 2*u, 2*u, ink)
		}
	}
}

// roomScene is a joiner's room: the seats, each with its player or bot and state, and Ready along the bottom. The board
// and the room's settings are the host's.
type roomScene struct {
	page
	net *session

	loff  float64
	lkeyN int
	block ui.Rect
	rows  [core.MaxPlayers]ui.Rect
}

func (r *roomScene) layout(a *app) {
	n := len(r.net.lobby.Seats)
	if !r.frame(a, true) && r.view.Offset == r.loff && n == r.lkeyN {
		return
	}
	r.lkeyN = n
	u, bp := a.u, r.bp
	v := r.view.Viewport
	pitch := (ui.CompactButtonTarget + 8) * u
	h := bp + 40*u + float64(n)*pitch + bp
	y := r.content(h)
	r.loff = r.view.Offset
	r.block = ui.Rect{X: v.X, Y: y, W: v.W, H: h}
	for i := range n {
		r.rows[i] = ui.Rect{X: v.X + bp, Y: y + bp + 40*u + float64(i)*pitch, W: v.W - 2*bp, H: ui.CompactButtonHeight * u}
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
		a.net, a.next = nil, newJoin(a, n.wifi(), "")
	case a.clicked(r.action) && n.got && n.seat >= 0:
		n.link.Send(proto.Ready{On: !n.lobby.Seats[n.seat].Ready})
	}
	return nil
}

func (r *roomScene) draw(dst *ebiten.Image, a *app) {
	r.layout(a)
	sc, f, u, n := a.sc, &a.f, a.u, r.net
	r.page.draw(dst, a, "Room")
	label, style, ok := "Joining", ui.ButtonInk, n.got && n.seat >= 0
	if ok {
		label = "Ready"
		if n.lobby.Seats[n.seat].Ready {
			label, style = "Not ready", ui.ButtonTile
		}
	}
	ui.DrawButton(dst, a.ui(), r.action, label, "", style, ok)
	b := r.body(dst)
	ui.FillRound(b, r.block, 24*u, sc.Tile)
	ui.DrawText(b, "Players", f.Heading, r.block.X+r.bp, r.block.Y+r.bp+10*u, text.AlignStart, sc.Ink)
	for i, st := range n.lobby.Seats {
		row := r.rows[i]
		ui.FillPill(b, row, sc.Ground)
		ui.SeatDot(b, sc, i, row.X+24*u, row.Cy(), 8*u, false, a.set.Marks)
		name := n.playerLabel(i)
		ui.DrawText(b, name, f.LabelBold, row.X+seatChoiceX*u, row.Cy(), text.AlignStart, sc.Ink)
		right := row.Right() - 16*u
		if st.State == proto.SeatAI {
			drawPips(b, 1+int(st.AI), right, row.Cy(), u, sc.Ink)
			continue
		}
		ui.DrawChip(b, a.ui(), seatState(st), right, row.Cy(), ui.ChipHeight*u)
	}
}

// seat is how Setup shows seat i in a room: its label (" (you)" on the host's), its type, and once the room is open
// the state of a human seat.
func (h *hostRoom) seat(a *app, i int) (label string, opt int, state ui.Chip) {
	n := h.net
	if n == nil || !n.got || i >= len(n.lobby.Seats) {
		opt = a.set.seats[i]
		label = a.set.label(a.set.seats, i)
		if i == firstHuman(a.set.seats) {
			label += selfSeatSuffix
		}
		return label, opt, ui.Chip{}
	}
	st := n.lobby.Seats[i]
	label = n.playerLabel(i)
	if st.State == proto.SeatAI {
		return label, 1 + int(st.AI), ui.Chip{}
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
