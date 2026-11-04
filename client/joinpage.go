package client

import (
	"context"
	hexenc "encoding/hex"
	"errors"
	"image"
	"log"
	"runtime"
	"slices"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/lan"
	"orbitrix/server/signal"
)

// joinScene is the Same Wi-Fi and Online pages: one centred column, a Host block (Set up a room, which opens the
// Room page) above a Join block. Join lists the public rooms, heard on the network or listed by the server, and a tap
// joins one (online, once its host lets the player in). Under the list, the code's six slots, typed on the device's
// own keyboard, join a private room. On Android the multicast lock lets the Wi-Fi pass the hosts' announcements on
// while the page is open. A browser's Same Wi-Fi page hears only the LAN host that served it, and explains why nearby
// rooms cannot be found: a browser's server-free rooms are joined by invite links.
type joinScene struct {
	page
	lan         bool
	canJoin     bool // nearby rooms and short codes work here; the Join block is always visible
	hostBlocked bool
	joinNote    string
	joinLines   []string
	statusLines []string
	lstatus     string
	code        *field
	finder      *lan.Finder   // Same Wi-Fi; nil when another program has the discovery port
	lister      *online.Rooms // Online
	heard       []lan.Room
	listed      []signal.Listed
	scratch     struct {
		heard  []lan.Room
		listed []signal.Listed
	}
	rows    []joinRow
	joining string      // the room being joined (its address or code), or ""
	waiting atomic.Bool // the join request is in front of the host
	status  string      // why the last try failed, or ""
	cancel  context.CancelFunc
	online  chan joinResult
	local   chan lanResult

	loff               float64
	lrows              int
	hostB, host, joinB rect
	rowR               []rect
	emptyY, codeY      float64
	listY, divY        float64 // the list's label, and the hairline between the two ways to join
	slots              [proto.CodeLen]rect
	slotRow            rect // the slots: pressing them opens the keyboard
	slotFace           *text.GoTextFace
	statusY            float64
	join               rect
}

// joinRow is a public room's row: its name, its state, and where a tap goes.
type joinRow struct {
	label      string
	state      chip
	addr, code string // a LAN room's host:port, or an online room's code
	ok         bool   // a tap joins: a seat is open, or the room holds this player's
}

type joinResult struct {
	c     hub.Conn
	err   error
	code  string
	token proto.Token
}

type lanResult struct {
	c          hub.Conn
	w          proto.Welcome
	addr, code string
	err        error
}

// newJoin opens the Same Wi-Fi (lanRoom) or Online page with code filled in: from a /join/ link, or the room this
// player may rejoin. With a mouse there is a physical keyboard, so the code takes typing straight away; a touch screen
// opens its keyboard when the player presses the slots.
func newJoin(a *app, lanRoom bool, code string) *joinScene {
	if r := a.set.Rejoin; code == "" && r != nil && (r.Addr != "") == lanRoom {
		code = r.Code
	}
	host := platform.LANHost()
	s := &joinScene{lan: lanRoom, canJoin: !lanRoom || runtime.GOOS != "js" || host != "", code: codeField(),
		online: make(chan joinResult, 1), local: make(chan lanResult, 1)}
	s.code.set(code)
	if runtime.GOOS == "js" {
		s.hostBlocked = !platform.HasWebRTC
		if lanRoom {
			s.joinNote = "Browsers cannot search your Wi-Fi for rooms. Ask the host for an invite link."
			if host != "" {
				s.joinNote = "Browsers cannot search your Wi-Fi for rooms. Only this host's room appears here."
			}
		} else if s.hostBlocked {
			s.canJoin = false
			s.joinNote = "This browser can't connect players. Use another browser or the app to join."
		}
		if lanRoom && host == "" && s.hostBlocked {
			s.joinNote = "This browser cannot connect to browser hosts. Open a link from a host using the app, or use another browser."
		}
	}
	s.code.focused = !a.in.finger && s.canJoin
	switch {
	case !lanRoom:
		s.lister = online.ListRooms(signalURL())
		return s
	case runtime.GOOS == "js": // a page can't hear broadcasts: it asks the host that served it, if one did
		if host != "" {
			s.finder = lan.Watch(host)
		}
		return s
	}
	platform.Discover(true)
	var err error
	if s.finder, err = lan.Find(); err != nil {
		log.Print(err) // the list stays empty; codes still reach a saved host
		platform.Discover(false)
	}
	return s
}

// refresh takes in the rooms heard or listed, and rebuilds the rows only when they changed.
func (s *joinScene) refresh(a *app) {
	switch {
	case s.finder != nil:
		s.scratch.heard = s.finder.Rooms(s.scratch.heard[:0])
		if slices.Equal(s.scratch.heard, s.heard) {
			return
		}
		s.heard = append(s.heard[:0], s.scratch.heard...)
	case s.lister != nil:
		s.scratch.listed = s.lister.Get(s.scratch.listed[:0])
		if slices.Equal(s.scratch.listed, s.listed) {
			return
		}
		s.listed = append(s.listed[:0], s.scratch.listed...)
	default:
		return
	}
	r := a.set.Rejoin
	s.rows = s.rows[:0]
	for _, h := range s.heard {
		if h.Private {
			continue // a private room is joined by its code
		}
		name := h.Name
		if name == "" {
			name = "Room"
		}
		mine := r != nil && r.Addr == h.Addr
		s.rows = append(s.rows, joinRow{name, roomWord(int(h.Open), h.Playing), h.Addr, "",
			h.Open > 0 && !h.Playing || mine})
	}
	for _, l := range s.listed {
		mine := r != nil && r.Addr == "" && r.Code == l.Code
		s.rows = append(s.rows, joinRow{boardName(l.Cols, l.Rows), roomWord(l.Open, l.Playing), "", l.Code,
			l.Open > 0 && !l.Playing || mine})
	}
}

// roomWord is a listed room's state.
func roomWord(open int, playing bool) chip {
	switch {
	case playing:
		return chip{"Playing", chipBusy}
	case open == 0:
		return chip{"Full", chipOff}
	}
	return chip{"Open", chipOn}
}

// boardName is a board size as Setup names it: its preset and dimensions, or Custom.
func boardName(cols, rows int) string {
	name := "Custom"
	if t := presetOf(cols, rows); t >= 0 {
		name = boardSizes[t].name
	}
	return name + " " + numbers[cols] + " × " + numbers[rows]
}

func (s *joinScene) layout(a *app) {
	if !s.frame(a, false) && s.view.offset == s.loff && len(s.rows) == s.lrows && s.status == s.lstatus {
		return
	}
	s.lrows = len(s.rows)
	s.lstatus = s.status
	u, bp := a.u, s.bp
	v := s.view.viewport
	w := min(v.w, 640*u)
	x := v.cx() - w/2
	pitch := (compactButtonTarget + 8) * u
	slot := codeSlotSize(u, w-2*bp)
	listH := float64(max(1, len(s.rows))) * pitch
	statusH := 44 * u
	if s.status != "" {
		s.statusLines = wrap(s.status, a.f.label, w-2*bp)
		statusH = max(statusH, float64(len(s.statusLines))*24*u+16*u)
	}
	hostH := bp + 32*u + 52*u + bp
	joinNoteH := 0.0
	if s.joinNote != "" {
		s.joinLines = wrap(s.joinNote, a.f.label, w-2*bp)
		joinNoteH = float64(len(s.joinLines))*24*u + 12*u
	}
	// Two groups: the listed rooms under their label, a hairline, then Join by code.
	joinH := bp + 32*u + 28*u + listH + 4*u + 20*u + 28*u + slot + statusH + 52*u + bp
	if !s.canJoin {
		joinH = bp + 32*u + bp
	}
	joinH += joinNoteH
	y := s.centred(hostH + s.gap + joinH)
	s.loff = s.view.offset
	s.hostB = rect{x, y, w, hostH}
	s.host = rect{x + bp, y + bp + 32*u, w - 2*bp, 52 * u}
	s.joinB = rect{x, s.hostB.bottom() + s.gap, w, joinH}
	if !s.canJoin {
		return
	}

	s.listY = s.joinB.y + bp + 32*u + joinNoteH + 10*u
	top := s.joinB.y + bp + 32*u + joinNoteH + 28*u
	s.rowR = s.rowR[:0]
	for i := range s.rows {
		s.rowR = append(s.rowR, rect{x + bp, top + float64(i)*pitch, w - 2*bp, compactButtonHeight * u})
	}
	s.emptyY = top + compactButtonHeight*u/2
	s.divY = top + listH + 4*u
	s.codeY = s.divY + 20*u + 10*u
	rowW := proto.CodeLen*slot + (proto.CodeLen-1)*8*u
	s.slotRow = rect{s.joinB.cx() - rowW/2, s.divY + 20*u + 28*u, rowW, slot}
	for i := range s.slots {
		s.slots[i] = rect{s.slotRow.x + float64(i)*(slot+8*u), s.slotRow.y, slot, slot}
	}
	s.code.caret = image.Rect(int(s.slotRow.x), int(s.slotRow.y), int(s.slotRow.right()), int(s.slotRow.bottom()))
	s.slotFace = newFace(srcDisplay, slot*0.5, 700)
	s.statusY = s.slotRow.bottom() + 22*u
	s.join = rect{x + bp, s.slotRow.bottom() + statusH, w - 2*bp, 52 * u}
}

func (s *joinScene) update(a *app) error {
	defer s.leaving(a)
	s.refresh(a)
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	in := s.inBody(a)
	if s.joining != "" {
		select {
		case r := <-s.online:
			s.joinedOnline(a, r)
		case r := <-s.local:
			s.joinedLAN(a, r)
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
	if in && !s.hostBlocked && a.clicked(s.host) {
		a.next = newRoomSetup(a, s.lan)
		return nil
	}
	if !s.canJoin {
		return nil
	}
	if in {
		for i, row := range s.rows {
			if row.ok && a.clicked(compactHit(s.rowR[i], a.u)) {
				s.start(a, row.addr, row.code, false)
				return nil
			}
		}
		s.code.tap(a, s.slotRow)
	}
	if s.code.update(a) || in && a.clicked(s.join) {
		if code := s.code.value(); len(code) == proto.CodeLen {
			s.start(a, "", code, true)
		}
	}
	return nil
}

// leaving stops listening for rooms once the page hands over to another.
func (s *joinScene) leaving(a *app) {
	if a.next == nil {
		return
	}
	if s.finder != nil {
		s.finder.Close()
		s.finder = nil
		platform.Discover(false)
	}
	if s.lister != nil {
		s.lister.Close()
		s.lister = nil
	}
}

// start asks a room's host for a seat, off the game loop: the LAN host at addr, or online the room with code. A code
// typed for Same Wi-Fi is tried on every private room heard. A player coming back to the room they left carries its
// rejoin token, and the host lets them straight back in.
func (s *joinScene) start(a *app, addr, code string, typed bool) {
	r := a.set.Rejoin
	mine := r != nil && (r.Addr != "") == s.lan && (typed || !s.lan) && r.Code == code || // the same code
		r != nil && s.lan && !typed && r.Addr == addr // the same public LAN host
	var token proto.Token
	if mine {
		_, _ = hexenc.Decode(token[:], []byte(r.Token)) // a damaged token joins as someone new
	}
	addrs := []string{addr}
	if s.lan && typed {
		addrs = addrs[:0]
		if mine {
			addrs = append(addrs, r.Addr)
		}
		for _, h := range s.heard {
			if (h.Private || lan.PublicCode(h.ID) == code) && !slices.Contains(addrs, h.Addr) {
				addrs = append(addrs, h.Addr)
			}
		}
		if len(addrs) == 0 {
			s.status = byeWhy(proto.ReasonBadCode)
			return
		}
	}
	s.code.focus(a, false)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.joining, s.status = cancel, addr+code, ""
	s.waiting.Store(false)
	name := a.set.name()
	if s.lan {
		go func() {
			joinLAN(ctx, addrs, name, code, token, s.local)
			cancel()
		}()
		return
	}
	relay, url := a.set.Relay, signalURL()
	go func() {
		c, err := online.Join(ctx, url, code, name, token, relay, s.waiting.Store)
		cancel()
		s.online <- joinResult{c: c, err: err, code: code, token: token}
	}()
}

// joinLAN asks the hosts at addrs in turn for a seat, with code for a private room, and puts the first seat it gets on
// result, or the most telling refusal: a wrong code only when no host said anything else. It blocks: run it off the
// game loop.
func joinLAN(ctx context.Context, addrs []string, name, code string, token proto.Token, result chan<- lanResult) {
	err := error(lan.Refused{Reason: proto.ReasonBadCode})
	for _, addr := range addrs {
		c, w, e := lan.Join(ctx, addr, proto.Hello{Name: name, Code: code, Token: token})
		if e == nil {
			result <- lanResult{c, w, addr, code, nil}
			return
		}
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
		if r := (lan.Refused{}); !errors.As(e, &r) || r.Reason != proto.ReasonBadCode {
			err = e
		}
	}
	result <- lanResult{code: code, err: err}
}

func (s *joinScene) joinedLAN(a *app, r lanResult) {
	s.joining = ""
	s.code.focused = !a.in.finger
	var refused lan.Refused
	switch {
	case errors.Is(r.err, context.Canceled):
	case errors.As(r.err, &refused):
		s.status = byeWhy(refused.Reason)
	case r.err != nil:
		s.status = "Can't reach the host"
	default:
		n := lanSession(a, r.c, r.w, r.addr, r.code)
		a.net, a.next = n, newRoom(n)
	}
}

func (s *joinScene) joinedOnline(a *app, r joinResult) {
	s.joining = ""
	s.code.focused = !a.in.finger
	switch {
	case errors.Is(r.err, context.Canceled):
	case errors.Is(r.err, online.ErrRefused):
		s.status = "Couldn't join. Check the code or ask the host."
	case errors.Is(r.err, signal.ErrVersion):
		s.status = byeWhy(proto.ReasonVersion)
	case errors.Is(r.err, online.ErrConnect):
		s.status = "Couldn't connect to the host. Try again or use another network."
	case r.err != nil:
		s.status = "Can't reach online rooms. Check your internet connection and try again."
	default:
		n := newSession(online.NewLink(r.c))
		n.code = r.code
		n.link.Send(proto.Hello{Name: a.set.name(), Token: r.token})
		a.net, a.next = n, newRoom(n)
	}
}

func (s *joinScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	title := "Online"
	if s.lan {
		title = "Same Wi-Fi"
	}
	s.page.draw(dst, a, title)
	b := s.body(dst)
	fillRound(b, s.hostB, 24*u, sc.tile)
	drawText(b, "Host", f.heading, s.hostB.x+s.bp, s.hostB.y+s.bp+10*u, text.AlignStart, sc.ink)
	if s.hostBlocked {
		drawButton(b, a, s.host, "This browser can't host", "", btnTile, false)
	} else if s.joining != "" {
		drawButton(b, a, s.host, "Set up a room", "", btnTile, false)
	} else {
		drawButton(b, a, s.host, "Set up a room", "", btnGround, true)
	}
	fillRound(b, s.joinB, 24*u, sc.tile)
	drawText(b, "Join", f.heading, s.joinB.x+s.bp, s.joinB.y+s.bp+10*u, text.AlignStart, sc.ink)
	for i, line := range s.joinLines {
		drawText(b, line, f.label, s.joinB.x+s.bp, s.joinB.y+s.bp+32*u+(float64(i)+0.5)*24*u, text.AlignStart, sc.dim)
	}
	if !s.canJoin {
		return
	}
	listLabel := "Public rooms"
	if s.lan {
		listLabel = "Rooms nearby"
	}
	drawText(b, listLabel, f.labelBold, s.joinB.x+s.bp, s.listY, text.AlignStart, sc.ink)
	if len(s.rows) == 0 {
		empty := "No public rooms"
		if s.lan {
			empty = "Looking for rooms"
		}
		drawText(b, empty, f.label, s.joinB.cx(), s.emptyY, text.AlignCenter, sc.dim)
	}
	for i, row := range s.rows {
		rr := s.rowR[i]
		ok := s.joining == "" && row.ok
		ink := drawButtonSurface(b, a, rr, btnGround, ok, ok && a.over(compactHit(rr, u)))
		state := row.state
		if s.joining == row.addr+row.code {
			state = chip{"Joining", chipOn}
		}
		drawText(b, row.label, f.labelBold, rr.x+16*u, rr.cy(), text.AlignStart, ink)
		ch := chipHeight * u
		drawChip(b, a, state, rr.right()-(rr.h-ch)/2, rr.cy(), ch) // concentric with the row's end
	}

	// The code's slots are printed tiles, like the board's; while the code takes typing, the next one is outlined.
	fillRound(b, rect{s.joinB.x + s.bp, s.divY, s.joinB.w - 2*s.bp, 1.5 * u}, 0, mix(sc.tile, sc.dim, 0.3))
	drawText(b, "Join by code", f.labelBold, s.joinB.x+s.bp, s.codeY, text.AlignStart, sc.ink)
	code := s.code.shown
	drawCodeSlots(b, a, s.slots[:], s.slotFace, code, s.code.focused)
	if s.joining != "" {
		status := "Connecting to the host"
		if s.waiting.Load() {
			status = "Waiting for the host"
		}
		drawText(b, status, f.label, s.joinB.cx(), s.statusY, text.AlignCenter, sc.dim)
	} else if s.status != "" {
		for i, line := range s.statusLines {
			drawText(b, line, f.label, s.joinB.cx(), s.statusY+float64(i)*24*u, text.AlignCenter, sc.ink)
		}
	}
	switch {
	case s.joining != "":
		drawButton(b, a, s.join, "Cancel", "", btnGround, true)
	case len(code) == proto.CodeLen:
		drawButton(b, a, s.join, "Join", "", btnInk, true)
	default:
		drawButton(b, a, s.join, "Join", "", btnTile, false) // the mat's disabled fill would vanish on the block
	}
}

// drawCodeSlots draws a room code as the printed tiles it is typed into, one per character, like the board's; while
// caret is set, the next tile is outlined. The Invite players dialog shows its code in the same tiles, read-only.
func drawCodeSlots(dst *ebiten.Image, a *app, slots []rect, face *text.GoTextFace, code []rune, caret bool) {
	for i, r := range slots {
		fillRound(dst, r, r.w*tileRadius, a.sc.ground)
		if i < len(code) {
			drawText(dst, string(code[i]), face, r.cx(), r.cy(), text.AlignCenter, a.sc.ink)
		} else if i == len(code) && caret {
			strokeRound(dst, r, r.w*tileRadius, 2.5*a.u, a.sc.ink)
		}
	}
}
