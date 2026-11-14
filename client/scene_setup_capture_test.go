//go:build capture

package client

import (
	"context"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/client/online"
	"orbitrix/core"
	"orbitrix/proto"
	"orbitrix/server/lan"
	"orbitrix/server/signal"
	"orbitrix/server/transport/loopback"
)

type setupCaptureCase struct {
	name        string
	w, h        float64
	players     int
	cols, rows  int
	dark, tail  bool
	menu, small bool
	player      string // the player's name
	focus       bool   // the name takes typing
	play        bool
	hover       string
	progress    float64 // fraction of page or player-list scroll
	count       int     // synthetic indicator stress count; 0 keeps the actual empty-board values
	marks       bool
	turn        int
	screen      string // "home", "settings", "pause" or "results"; empty draws Setup, or the game with play
}

type setupCaptureGame struct {
	dir   string
	cases []setupCaptureCase
	n     int
	err   error
}

func (g *setupCaptureGame) Layout(int, int) (int, int) { return 1440, 900 }
func (g *setupCaptureGame) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.n == len(g.cases) {
		return ebiten.Termination
	}
	return nil
}

func (g *setupCaptureGame) Draw(*ebiten.Image) {
	if g.n >= len(g.cases) || g.err != nil {
		return
	}
	c := g.cases[g.n]
	a, s := setupTestApp(c.w, c.h, 1, c.players), &setupScene{menu: -1, open: foldPlayers}
	if c.dark {
		a.sc, a.set.Marks = &ui.Palettes[ui.Dark], true
	}
	a.set.Marks = a.set.Marks || c.marks
	a.set.Name = c.player
	a.set.seats[1], _ = parseSeat("raider")
	if c.cols > 0 {
		a.set.Cols, a.set.Rows = c.cols, c.rows
	}
	s.layout(a)
	s.scroll = c.progress * s.maxScroll
	if c.tail {
		s.scroll = s.maxScroll
	}
	if c.menu {
		s.menu = 1
	}
	if c.small {
		a.set.Cols, a.set.Rows = 3, 3
	}
	var hover ui.Rect
	switch c.hover {
	case "player":
		hover = s.seatTy[1]
	case "preset":
		hover = s.presetPills[shapePortrait][0]
	case "stepper":
		hover = s.plusP
	}
	if hover.W > 0 {
		a.in.x, a.in.y = hover.Cx(), hover.Cy()
	}
	out := ebiten.NewImage(int(c.w), int(c.h))
	defer out.Deallocate()
	out.Fill(a.sc.Ground)
	var overlay scene
	switch c.screen {
	case "home":
		h := newHome()
		h.layout(a)
		h.name.focused = c.focus
		for range 120 {
			h.demo.tick()
		}
		h.draw(out, a)
	case "settings":
		m := newSettingsMenu(newHome())
		m.layout(a)
		m.draw(out, a)
	case "modes":
		a.set.Where = whereWifi
		s := newSetup(a)
		switch c.hover {
		case "rules":
			s.open = foldRules
		case "players":
			s.open = foldPlayers
		}
		s.layout(a)
		s.draw(out, a)
	case "online", "wifi":
		// Built by hand, so the capture neither listens on the network nor asks a server for its rooms.
		o := &joinScene{lan: c.screen == "wifi", canJoin: true, code: codeField()}
		o.code.set("K7Q")
		o.rows = []joinRow{{"Doormat 7 × 10", roomWord(2, false), "", "AB12CD", true},
			{"Floor 13 × 18", roomWord(0, true), "", "EF34GH", false}}
		if o.lan {
			o.rows = []joinRow{{"DESKTOP-7QF3K2L", roomWord(1, false), "192.168.1.20:7777", "", true},
				{"Pixel 8", roomWord(0, false), "192.168.1.31:7777", "", false}}
		}
		if c.hover == "waiting" || c.hover == "connecting" {
			o.code.set("K7QX3M")
			o.joining, o.code.focused = "K7QX3M", false
			o.waiting.Store(c.hover == "waiting")
			o.snack.ticks = snackIn
		}
		if c.hover == "empty" {
			o.rows = nil
		}
		o.layout(a)
		if c.progress > 0 {
			o.view.Offset = c.progress * o.view.MaxScroll()
			o.layout(a)
		}
		o.draw(out, a)
		overlay = o
	case "room-setup", "room-invite":
		s := newRoomSetup(a, false)
		if c.screen == "room-invite" {
			s.room.openDialog()
		}
		s.layout(a)
		s.draw(out, a)
		overlay = s
	case "room-dialog", "room-dialog-long":
		s = newRoomSetup(a, true)
		s.room.net = &session{lan: &lan.Host{Web: true, Code: "K7QX3M"}, addr: "192.168.1.20:7777", code: "ABCDEF"}
		s.room.private = c.hover == "private"
		s.room.openDialog()
		if c.hover == "snack" {
			s.room.reqs = []online.Request{{Peer: "p1", Name: "Human"}}
			s.room.snack = snackbar{message: "Human wants to join", actions: [2]string{"No", "Let in"}, ticks: snackIn}
		}
		s.layout(a)
		if c.screen == "room-dialog-long" {
			s.room.open = false
			s.draw(out, a)
			v := &s.room.invite
			v.settings = true
			long := strings.Repeat("Ab3_xY7-", 90)
			ui.FillRound(out, ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, 0, ui.WithAlpha(a.sc.Ground, 0.8))
			v.draw(out, a, "Invite players", "https://orbitrix.example.com/play#i="+long, long, "Done", true)
			r := v.opts[0]
			r.W = v.opts[1].Right() - r.X
			ui.FillPill(out, r, a.sc.Ground)
			ui.DrawText(out, "Invite only", a.f.LabelBold, r.Cx(), r.Cy(), text.AlignCenter, a.sc.Dim)
		} else {
			s.draw(out, a)
		}
		overlay = s
	case "room-host", "room-joiner", "room-seat", "room-invite-open":
		n := captureSession(c.screen != "room-joiner")
		n.lobby.Seats = []proto.Seat{{State: proto.SeatHere, Ready: true, Name: "Human"}, {State: proto.SeatHere, Name: "Human"},
			{State: proto.SeatOpen}, {State: proto.SeatAI, AI: 6}}
		if c.hover == "ready" {
			n.lobby.Seats[1].Ready = true
		}
		a.set.seats = []int{seatHuman, seatHuman, seatHuman, 1 + 6}
		if c.screen == "room-joiner" {
			n.seat = 1
		}
		r := newRoom(n)
		if s, ok := r.(*setupScene); ok {
			if c.screen == "room-host" {
				s.room.reqs = []online.Request{{Peer: "p1", Name: "Human"}}
				if c.hover == "two" {
					s.room.reqs = append(s.room.reqs, online.Request{Peer: "p2", Name: "Human"})
				}
				s.room.snackPeer = "p1"
				s.room.snack = snackbar{message: "Human wants to join", more: moreWaiting[len(s.room.reqs)-1],
					actions: [2]string{"No", "Let in"}, ticks: snackIn}
			}
			s.layout(a)
			if c.screen == "room-invite-open" {
				s.room.openDialog()
				s.room.draft = true // Private chosen: its code shows
			}
			if c.screen == "room-seat" {
				s.menu = 1
			}
		}
		r.draw(out, a)
		overlay = r
	case "password":
		p := newPassword(newRoomSetup(a, false), c.hover == "wrong")
		p.pw.set("correct-horse")
		p.layout(a)
		p.draw(out, a)
	case "pause-online", "gone-prompt":
		n := captureSession(true)
		n.lobby.Seats = []proto.Seat{{State: proto.SeatHere, Ready: true, Name: "Human"}, {State: proto.SeatHere, Name: "Human"},
			{State: proto.SeatOpen}, {State: proto.SeatAI, AI: 6}}
		n.lobby.Seats[2] = proto.Seat{State: proto.SeatGone, Name: "Human"}
		g, _ := core.NewGame(9, 9, 4)
		p := newPlayOn(a, g, n.seats(), n.labels(), pauseOnline)
		p.net = n
		p.paused = c.screen == "pause-online"
		p.askGone()
		p.tickRail()
		p.layout(a)
		p.draw(out, a)
	case "gone":
		g := newGone(&session{end: "Connection lost", lost: true, code: "K7QX3M"})
		g.layout(a)
		g.draw(out, a)
	case "settings-online":
		m := newSettingsMenu(newHome())
		m.tab = tabNetwork
		m.layout(a)
		m.draw(out, a)
	case "pause", "results":
		p, err := newPlay(a, a.set.Cols, a.set.Rows, a.set.seats)
		if err != nil {
			g.err = err
			return
		}
		for c.screen == "results" && !p.game.Over() {
			_ = p.game.Simulate(p.game.LegalMoves(nil)[0])
		}
		p.b = boardview.New(p.game)
		p.settle()
		p.paused, p.overT = c.screen == "pause", wipeTicks+60
		if c.hover == "confirm" {
			p.confirm, p.confirmT = pauseRestart, confirmTicks
		}
		if c.hover == "flash" {
			p.paused = false
			_ = p.game.Simulate(0)
			p.b = boardview.New(p.game)
			p.settle()
			p.flash, p.flashT = 0, flashTicks
		}
		p.tickRail()
		p.layout(a)
		p.draw(out, a)
	}
	if c.screen == "" && c.play {
		p, err := newPlay(a, a.set.Cols, a.set.Rows, a.set.seats)
		if err != nil {
			g.err = err
			return
		}
		p.tickRail()
		if c.count > 0 {
			for i := range p.seats {
				p.orbs[i], p.orbS[i] = c.count-i, strconv.Itoa(c.count-i)
			}
		}
		if c.hover == "out" { // every third seat out, so the Out chips show
			for i := 2; i < len(p.seats); i += 3 {
				p.alive[i] = false
			}
		}
		p.shown, p.spin = c.turn, p.wedgeAngle(c.turn)
		p.chain, p.chainHold, p.chainText = 88, calloutHold, "Chain ×88"
		p.layout(a)
		p.seatView.Offset = c.progress * p.seatView.MaxScroll()
		if c.tail {
			p.seatView.Offset = p.seatView.MaxScroll()
		}
		p.layoutScrolled(a)
		p.draw(out, a)
	} else if c.screen == "" {
		s.draw(out, a)
	}
	drawSnackbarOverlay(out, a, overlay)
	pixels := image.NewRGBA(image.Rect(0, 0, int(c.w), int(c.h)))
	out.ReadPixels(pixels.Pix)
	f, err := os.Create(filepath.Join(g.dir, c.name+".png"))
	if err == nil {
		err = png.Encode(f, pixels)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	g.err = err
	g.n++
}

func TestCaptureSetup(t *testing.T) {
	origin = "https://orbitrix.example.com" // so the online entries show
	dir := os.Getenv("ORBITRIX_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set ORBITRIX_CAPTURE_DIR to capture setup viewports")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &setupCaptureGame{dir: dir, cases: []setupCaptureCase{
		{name: "fix-snack-room-desktop", w: 1040, h: 640, players: 4, screen: "room-host", hover: "ready"},
		{name: "fix-snack-room-mobile", w: 390, h: 844, players: 4, screen: "room-host", hover: "ready", dark: true},
		{name: "fix-snack-room-landscape", w: 844, h: 390, players: 4, screen: "room-host", hover: "ready"},
		{name: "fix-snack-dialog-desktop", w: 1040, h: 640, players: 3, screen: "room-dialog", hover: "snack"},
		{name: "fix-snack-dialog-mobile", w: 390, h: 844, players: 3, screen: "room-dialog", hover: "snack", dark: true},
		{name: "fix-snack-dialog-landscape", w: 844, h: 390, players: 3, screen: "room-dialog", hover: "snack"},
		{name: "fix-snack-joining-desktop", w: 1040, h: 640, players: 2, screen: "online", hover: "connecting"},
		{name: "fix-snack-waiting-desktop", w: 1040, h: 640, players: 2, screen: "online", hover: "waiting"},
		{name: "fix-snack-waiting-mobile", w: 390, h: 844, players: 2, screen: "online", hover: "waiting", dark: true},
		{name: "fix-snack-waiting-landscape", w: 844, h: 390, players: 2, screen: "online", hover: "waiting"},
		{name: "fix-snack-waiting-landscape-scrolled", w: 844, h: 390, players: 2, screen: "online", hover: "waiting", progress: 1},
		{name: "layout-room-desktop", w: 1040, h: 640, players: 3, screen: "room-dialog"},
		{name: "layout-room-mobile", w: 390, h: 844, players: 3, screen: "room-dialog", hover: "private"},
		{name: "layout-room-mobile-dark", w: 390, h: 844, players: 3, screen: "room-dialog", hover: "private", dark: true},
		{name: "layout-room-landscape", w: 844, h: 390, players: 3, screen: "room-dialog"},
		{name: "layout-room-short", w: 640, h: 320, players: 3, screen: "room-dialog"},
		{name: "layout-room-long-mobile", w: 390, h: 844, players: 3, screen: "room-dialog-long"},
		{name: "layout-room-long-landscape", w: 780, h: 360, players: 3, screen: "room-dialog-long", dark: true},
		{name: "p7-room-setup", w: 1440, h: 900, players: 4, screen: "room-setup"},
		{name: "p7-room-setup-mobile", w: 390, h: 844, players: 4, screen: "room-setup", dark: true},
		{name: "p7-room-host", w: 1440, h: 900, players: 4, screen: "room-host"},
		{name: "p7-room-host-1040", w: 1040, h: 640, players: 4, screen: "room-host", hover: "two"},
		{name: "p7-room-host-landscape", w: 844, h: 390, players: 4, screen: "room-host", hover: "two", dark: true},
		{name: "p7-room-host-mobile", w: 390, h: 844, players: 4, screen: "room-host", dark: true},
		{name: "p7-room-joiner", w: 1040, h: 640, players: 4, screen: "room-joiner"},
		{name: "p7-room-joiner-mobile", w: 390, h: 844, players: 4, screen: "room-joiner"},
		{name: "p7-room-invite-mobile", w: 390, h: 844, players: 4, screen: "room-invite", dark: true},
		{name: "p7-room-invite-mobile-light", w: 390, h: 844, players: 4, screen: "room-invite"},
		{name: "p7-room-invite-open", w: 1040, h: 640, players: 4, screen: "room-invite-open"},
		{name: "p7-room-invite-open-dark", w: 1040, h: 640, players: 4, screen: "room-invite-open", dark: true},
		{name: "p7-room-seat", w: 1040, h: 640, players: 4, screen: "room-seat"},
		{name: "p7-room-seat-mobile", w: 390, h: 844, players: 4, screen: "room-seat"},
		{name: "p7-join-wifi", w: 1440, h: 900, players: 2, screen: "wifi"},
		{name: "p7-join-wifi-mobile-empty", w: 390, h: 844, players: 2, screen: "wifi", hover: "empty", dark: true},
		{name: "p7-join-online-mobile", w: 390, h: 844, players: 2, screen: "online"},
		{name: "p7-join-online-waiting", w: 1040, h: 640, players: 2, screen: "online", hover: "waiting"},
		{name: "p7-settings-network", w: 1040, h: 640, players: 2, screen: "settings-online"},
		{name: "home-desktop", w: 1440, h: 900, players: 4, screen: "home"},
		{name: "home-dark-1040", w: 1040, h: 640, players: 4, screen: "home", dark: true},
		{name: "home-mobile", w: 390, h: 844, players: 4, screen: "home"},
		{name: "home-landscape", w: 844, h: 390, players: 4, screen: "home"},
		{name: "home-named-780", w: 780, h: 360, players: 4, screen: "home", player: "Ana"},
		{name: "home-typing-mobile", w: 390, h: 844, players: 4, screen: "home", player: "Bartholomew", focus: true},
		{name: "home-modes-780", w: 780, h: 360, players: 4, screen: "modes", player: "Ana"},
		{name: "settings-desktop", w: 1440, h: 900, players: 2, screen: "settings"},
		{name: "settings-mobile-dark", w: 390, h: 844, players: 2, screen: "settings", dark: true},
		{name: "pause-desktop", w: 1440, h: 900, players: 4, screen: "pause"},
		{name: "pause-mobile", w: 390, h: 844, players: 4, screen: "pause", dark: true},
		{name: "results-desktop", w: 1440, h: 900, players: 4, screen: "results"},
		{name: "results-mobile", w: 390, h: 844, players: 4, screen: "results", dark: true},
		{name: "results-landscape", w: 844, h: 390, players: 4, screen: "results"},
		{name: "p6-home-modes", w: 1440, h: 900, players: 4, screen: "modes"},
		{name: "fold-rules", w: 1440, h: 900, players: 4, screen: "modes", hover: "rules"},
		{name: "fold-players-12", w: 1040, h: 640, players: 12, screen: "modes", hover: "players"},
		{name: "fold-players-12-short", w: 844, h: 390, players: 12, screen: "modes", hover: "players", dark: true},
		{name: "fold-players", w: 1040, h: 640, players: 4, screen: "modes", hover: "players", dark: true},
		{name: "fold-rules-mobile", w: 390, h: 844, players: 4, screen: "modes", hover: "rules"},
		{name: "p6-home-modes-mobile", w: 390, h: 844, players: 4, screen: "modes", dark: true},
		{name: "p6-online", w: 1440, h: 900, players: 2, screen: "online"},
		{name: "p6-online-1040-waiting", w: 1040, h: 640, players: 2, screen: "online", hover: "waiting"},
		{name: "p6-online-mobile", w: 390, h: 844, players: 2, screen: "online", dark: true},
		{name: "p6-online-landscape", w: 844, h: 390, players: 2, screen: "online"},
		{name: "p6-password", w: 1040, h: 640, players: 2, screen: "password", hover: "wrong"},
		{name: "p6-password-mobile", w: 390, h: 844, players: 2, screen: "password"},
		{name: "p6-pause-online", w: 1040, h: 640, players: 4, screen: "pause-online"},
		{name: "p6-gone-prompt", w: 1040, h: 640, players: 4, screen: "gone-prompt"},
		{name: "p6-gone", w: 1040, h: 640, players: 2, screen: "gone"},
		{name: "p6-gone-mobile", w: 390, h: 844, players: 2, screen: "gone", dark: true},
		{name: "p6-settings-online", w: 1040, h: 640, players: 2, screen: "settings-online"},
		{name: "fix-pause-confirm", w: 1040, h: 640, players: 4, screen: "pause", hover: "confirm"},
		{name: "fix-pause-confirm-mobile", w: 390, h: 844, players: 4, screen: "pause", hover: "confirm", dark: true},
		{name: "fix-flash", w: 1040, h: 640, players: 2, screen: "pause", hover: "flash"},
		{name: "fix-results-marks", w: 1040, h: 640, players: 4, screen: "results", marks: true},
		{name: "fix-results-dark", w: 1040, h: 640, players: 3, screen: "results", dark: true},
		{name: "desktop", w: 1440, h: 900, players: 2},
		{name: "setup-all-shapes", w: 1440, h: 1080, players: 2},
		{name: "setup-rules-upright", w: 1080, h: 1200, players: 2},
		{name: "setup-map-upright-end", w: 390, h: 844, players: 2, tail: true},
		{name: "setup-rules-upright-dark", w: 1080, h: 1200, players: 2, dark: true},
		{name: "setup-desktop-12p", w: 1440, h: 900, players: 12},
		{name: "setup-preview-background", w: 1881, h: 851, players: 12, dark: true},
		{name: "setup-preview-quarter", w: 1881, h: 851, players: 12, dark: true, progress: 0.25},
		{name: "setup-preview-mid", w: 1881, h: 851, players: 12, dark: true, progress: 0.5},
		{name: "setup-preview-three-quarters", w: 1881, h: 851, players: 12, dark: true, progress: 0.75},
		{name: "setup-preview-background-bottom", w: 1881, h: 851, players: 12, dark: true, tail: true},
		{name: "setup-1146-12p", w: 1146, h: 640, players: 12},
		{name: "setup-tablet-12p", w: 760, h: 844, players: 12},
		{name: "user-1040", w: 1040, h: 640, players: 2},
		{name: "setup-dimensions-desktop", w: 1040, h: 640, players: 2, cols: 17, rows: 10, dark: true},
		{name: "setup-dimensions-mobile", w: 390, h: 844, players: 2, cols: 17, rows: 10, dark: true},
		{name: "setup-1040-12p", w: 1040, h: 640, players: 12},
		{name: "setup-1040-12p-bottom", w: 1040, h: 640, players: 12, tail: true},
		{name: "setup-dark-12p", w: 1040, h: 640, players: 12, dark: true},
		{name: "mobile", w: 390, h: 844, players: 2},
		{name: "setup-mobile-12p", w: 390, h: 844, players: 12, dark: true},
		{name: "setup-mobile-12p-bottom", w: 390, h: 844, players: 12, dark: true, tail: true},
		{name: "setup-landscape", w: 844, h: 390, players: 12},
		{name: "setup-landscape-mid", w: 844, h: 390, players: 12, progress: 0.5},
		{name: "setup-menu", w: 1040, h: 640, players: 2, menu: true},
		{name: "setup-menu-4", w: 1040, h: 640, players: 4, menu: true},
		{name: "setup-menu-2", w: 1040, h: 640, players: 2, menu: true},
		{name: "setup-board-too-small", w: 1040, h: 640, players: 12, small: true, tail: true},
		{name: "setup-player-hover", w: 1040, h: 640, players: 2, hover: "player"},
		{name: "setup-preset-hover", w: 1040, h: 640, players: 2, hover: "preset"},
		{name: "setup-stepper-hover", w: 1040, h: 640, players: 2, hover: "stepper"},
		{name: "setup-dark-player-hover", w: 1040, h: 640, players: 4, dark: true, hover: "player"},
		{name: "setup-dark-preset-hover", w: 1040, h: 640, players: 4, dark: true, hover: "preset"},
		{name: "chip-play-desktop", w: 1440, h: 900, players: 6, play: true, hover: "out"},
		{name: "chip-play-12p-dark", w: 1040, h: 640, players: 12, play: true, hover: "out", dark: true, tail: true},
		{name: "chip-play-landscape-short", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, hover: "out", tail: true},
		{name: "chip-play-portrait", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, hover: "out", dark: true},
		{name: "play-desktop", w: 1440, h: 900, players: 2, play: true},
		{name: "play-desktop-12p", w: 1040, h: 640, players: 12, play: true},
		{name: "play-wide-12p", w: 1920, h: 1080, players: 12, play: true, dark: true},
		{name: "play-responsive-8p", w: 1919, h: 1004, players: 8, play: true},
		{name: "play-responsive-12p", w: 1919, h: 1004, players: 12, play: true, dark: true},
		{name: "play-responsive-narrow", w: 1340, h: 1004, players: 12, play: true},
		{name: "play-mobile", w: 390, h: 844, players: 2, play: true},
		{name: "play-mobile-12p", w: 360, h: 740, players: 12, play: true, dark: true},
		{name: "play-mobile-8p", w: 390, h: 844, players: 8, play: true},
		{name: "play-mobile-placemat", w: 390, h: 844, players: 4, cols: 6, rows: 9, play: true},
		{name: "play-mobile-portrait-6p", w: 390, h: 844, players: 6, cols: 13, rows: 18, play: true},
		{name: "play-mobile-portrait-12p", w: 412, h: 915, players: 12, cols: 13, rows: 18, play: true, dark: true},
		{name: "play-mobile-portrait-limited", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true},
		{name: "play-mobile-portrait-scrolled", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, tail: true},
		{name: "play-mobile-tall-board", w: 360, h: 740, players: 12, cols: 3, rows: 40, play: true, dark: true},
		{name: "play-mobile-tall-board-players-scrolled", w: 360, h: 740, players: 12, cols: 3, rows: 40, play: true, dark: true, tail: true},
		{name: "play-tablet-turned", w: 768, h: 1024, players: 6, cols: 30, rows: 18, play: true, dark: true},
		{name: "play-tablet-turned-short", w: 768, h: 800, players: 12, cols: 30, rows: 18, play: true},
		{name: "play-landscape", w: 844, h: 390, players: 12, cols: 30, rows: 18, play: true},
		{name: "play-landscape-short", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, dark: true},
		{name: "play-landscape-scrolled", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, dark: true, tail: true},
		{name: "counts-landscape-3", w: 844, h: 390, players: 12, cols: 30, rows: 18, play: true, count: 999},
		{name: "counts-landscape-5", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, count: 99999, dark: true},
		{name: "counts-landscape-small-5", w: 640, h: 320, players: 12, cols: 30, rows: 18, play: true, count: 99999, marks: true},
		{name: "counts-desktop-5", w: 1440, h: 900, players: 12, play: true, count: 99999},
		{name: "counts-portrait-3", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 999},
		{name: "counts-portrait-4", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 9999, marks: true},
		{name: "counts-portrait-5", w: 320, h: 640, players: 12, cols: 13, rows: 18, play: true, count: 99999},
		{name: "counts-portrait-dark-5", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 99999, dark: true},
		{name: "counts-portrait-blue-5", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 99999, marks: true, turn: 1},
	}}
	ebiten.SetWindowSize(160, 100)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowPosition(-32000, -32000)
	if only := os.Getenv("ORBITRIX_CAPTURE_ONLY"); only != "" { // a name prefix, to capture a few cases
		g.cases = slices.DeleteFunc(g.cases, func(c setupCaptureCase) bool { return !strings.HasPrefix(c.name, only) })
	}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
}

// captureSession is an online room for the lobby and online game captures: a real one on a local signaling server
// when host, so the host's controls show.
func captureSession(host bool) *session {
	origin = "https://orbitrix.example.com"
	if !host {
		mine, _ := loopback.Pipe()
		n := newSession(online.NewLink(mine))
		n.code, n.got = "K7QX3M", true
		return n
	}
	srv := httptest.NewServer(signal.New(signal.Config{STUN: []string{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := online.Open(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), "", false, false, hubConfig(setupTestApp(800, 600, 1, 4)))
	if err != nil {
		panic(err)
	}
	n := &session{room: h.Room, host: h, link: h.Me, seat: 0, got: true}
	return n
}
