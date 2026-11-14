package client

import (
	"testing"

	"orbitrix/client/internal/ui"
)

// Where falls back to This Device when the saved choice is one this build cannot offer, and Setup opens there:
// a room not yet open for Same Wi-Fi or Online, none on This Device.
func TestWhere(t *testing.T) {
	old := origin
	defer func() { origin = old }()
	for _, c := range []struct {
		name          string
		origin        string
		saved, want   int
		room, lanRoom bool
	}{
		{"this device", "", whereDevice, whereDevice, false, false},
		{"same wi-fi", "", whereWifi, whereWifi, true, true},
		{"online without a server", "", whereOnline, whereDevice, false, false},
		{"online with a server", "https://orbitrix.example.com", whereOnline, whereOnline, true, false},
		{"a value from a newer build", "", 7, whereDevice, false, false},
	} {
		origin = c.origin
		a := setupTestApp(1040, 640, 1, 2)
		a.set.Where = c.saved
		if got := a.set.where(); got != c.want {
			t.Errorf("%s: where %d, want %d", c.name, got, c.want)
		}
		s := newSetup(a)
		if (s.room != nil) != c.room || c.room && s.room.lan != c.lanRoom {
			t.Errorf("%s: Setup opened with the wrong room", c.name)
		}
	}
}

// The Where row swaps the room Setup would open, keeps seats and board, and is held while a room opens.
func TestSetupWhereRow(t *testing.T) {
	old := origin
	origin = "https://orbitrix.example.com"
	defer func() { origin = old }()
	a := setupTestApp(1040, 640, 1, 4)
	a.set.Firewall = true
	a.set.Cols, a.set.Rows = 7, 10
	s := newSetup(a)
	s.open = foldRules
	s.layout(a)
	click := func(row, i int) {
		t.Helper()
		s.layout(a)
		r := s.game.tiles[row][i]
		a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
		if err := s.update(a); err != nil {
			t.Fatal(err)
		}
		a.in = input{}
		s.layout(a)
	}
	click(0, whereOnline)
	if s.room == nil || s.room.lan || a.set.Where != whereOnline {
		t.Fatal("Online must prepare an online room")
	}
	click(0, whereWifi)
	if s.room == nil || !s.room.lan || a.set.Players != 4 || a.set.Cols != 7 || a.set.Rows != 10 {
		t.Fatal("Same Wi-Fi must prepare a LAN room and keep the seats and board")
	}
	click(1, 1) // Real-time is not built
	click(2, 0) // nor Drop
	click(2, 2) // nor Fog of war
	if a.next != nil || a.set.Where != whereWifi {
		t.Fatal("unbuilt options must do nothing")
	}
	s.room.opening = true
	click(0, whereDevice)
	if s.room == nil || a.set.Where != whereWifi {
		t.Fatal("the Where row must hold while a room opens")
	}
	s.room.opening = false
	click(0, whereDevice)
	if s.room != nil || a.set.Where != whereDevice {
		t.Fatal("This Device must drop the room")
	}
}

// Rules and Players fold to their headings: neither is open at first, and opening one closes the other.
func TestSetupFold(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	s := newSetup(a)
	tap := func(head *ui.Rect) {
		t.Helper()
		s.layout(a)
		s.scroll = 1e9 // to the end, where an open Rules card pushes Players
		s.layout(a)
		a.in = input{x: head.X + 8, y: head.Cy(), clicked: true}
		if err := s.update(a); err != nil {
			t.Fatal(err)
		}
		a.in = input{}
		s.layout(a)
	}
	s.layout(a)
	shut := s.players.H
	if s.open != foldNone || s.seatHit[0].W != 0 || s.game.strip.Viewport.H != 0 {
		t.Fatal("neither card may be open at first")
	}
	tap(&s.rulesHead)
	if s.open != foldRules || s.game.strip.Viewport.H == 0 || s.game.tiles[0][0].W == 0 {
		t.Fatal("a tap on Rules must open it")
	}
	tap(&s.playersHead)
	if s.open != foldPlayers || s.game.strip.Viewport.H != 0 || s.seatHit[3].W == 0 || s.players.H <= shut {
		t.Fatal("opening Players must close Rules and show the seats")
	}
	tap(&s.playersHead)
	if s.open != foldNone || s.players.H != shut || s.seatHit[0].W != 0 {
		t.Fatal("a second tap must close Players")
	}
	a.in = input{x: s.plusP.Cx(), y: s.plusP.Cy(), clicked: true}
	if err := s.update(a); err != nil || a.set.Players != 5 || s.open != foldNone {
		t.Fatal("the count must still change on a collapsed Players card, without opening it")
	}
}
