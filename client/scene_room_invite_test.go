package client

import (
	"fmt"
	"strings"
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/server/lan"
)

func TestQRLayout(t *testing.T) {
	for _, size := range append(layoutSizes, [2]float64{640, 320}) {
		for _, u := range []float64{1, 2.625} {
			t.Run(fmt.Sprintf("%gx%g/u%g", size[0], size[1], u), func(t *testing.T) {
				a := withCutout(setupTestApp(size[0], size[1], u, 3))
				s := newRoomSetup(a, true)
				s.layout(a)
				checkParts(t, a.safe(), map[string]ui.Rect{"Start": s.start, "Room": s.invite})
				if gap := (s.invite.X - s.start.Right()) / u; gap < 12 || gap > 16 {
					t.Fatal("a room must reserve exactly one button")
				}
				s.room.layoutDialog(a)
				v := &s.room.invite
				v.layout(a)
				if !containsRect(a.safe(), v.panel) {
					t.Fatalf("invite dialog leaves the safe area: %+v", v.panel)
				}
				checkParts(t, v.panel, map[string]ui.Rect{"QR": v.box, "Public": v.opts[0], "Private": v.opts[1], "code": v.code,
					"Done": v.done, "Copy code": v.copies[0], "Copy link": v.copies[1]})
				for _, label := range []string{"Open room", "Opening", "Confirm"} {
					if ui.TextWidth(label, a.f.Heading) > v.done.W-2*16*u {
						t.Fatalf("%q overflows the dialog's action", label)
					}
				}
				if v.box.W < 160*u || v.done.H != ui.QuietButtonHeight*u {
					t.Fatal("QR or controls too small")
				}
				if v.face.Size != v.slot*0.5 {
					t.Fatal("invite tiles must use the Join field's type scale")
				}
			})
		}
	}
}

func TestLANQRLink(t *testing.T) {
	h := &hostRoom{lan: true, net: &session{lan: &lan.Host{Web: true, Code: "GHJKLM"}, addr: "192.168.1.5:7777", code: "ABCDEF"}}
	for _, private := range []bool{false, true} {
		h.private = private
		wantCode := "GHJKLM"
		if private {
			wantCode = "ABCDEF"
		}
		want := "http://192.168.1.5:7777/join/" + wantCode
		if link, code := h.invitation(); link != want || code != wantCode {
			t.Fatalf("private %v: link %q, code %q", private, link, code)
		}
	}
	h.net.lan.Web = false
	if link, _ := h.invitation(); link != "" {
		t.Fatal("a build without the web game offers a browser link")
	}
}

func TestLongInviteTiles(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	var v inviteView
	code := strings.Repeat("Ab3_", 180)
	v.layout(a)
	a.in = input{x: v.code.Cx(), y: v.code.Cy(), wheel: -10000}
	v.update(a, "", code)
	if v.cached != code || len(v.runes) != len(code) || v.offset <= 0 {
		t.Fatal("the full code must remain available while scrolling")
	}
	want := float64(len(code)-6) * (v.slot + 8*a.u)
	if v.offset != want {
		t.Fatalf("scroll stops at %g, want %g", v.offset, want)
	}
	a.in = input{x: v.code.Cx(), y: v.code.Cy(), wheel: 10000}
	v.update(a, "", code)
	if v.offset != 0 {
		t.Fatal("the code must scroll back to its first character")
	}
}

func TestUnconfirmedInviteIsNotShared(t *testing.T) {
	h := &hostRoom{lan: true, open: true, draft: true, net: &session{lan: &lan.Host{Web: true, Code: "ABCDEF"}, addr: "192.168.1.5:7777"}}
	if link, code := h.invitation(); link != "" || code != "" {
		t.Fatal("draft access must not expose an invitation before Confirm")
	}
	h.open = false
	if link, code := h.invitation(); link == "" || code != "ABCDEF" {
		t.Fatal("canceling the draft must restore the active public invite")
	}
}

func TestInviteCopiesCodeAndLink(t *testing.T) {
	saved := platform.Copy
	defer func() { platform.Copy = saved }()
	for _, code := range []string{"ABCDEF", "full_invite_or_reply_code"} {
		link := "https://orbitrix.example.com/join/" + code
		for i, want := range [2]string{code, link} {
			copied := ""
			platform.Copy = func(text string, done func(bool)) { copied = text; done(true) }
			a := setupTestApp(390, 844, 1, 2)
			var v inviteView
			v.layout(a)
			a.in = input{x: v.copies[i].Cx(), y: v.copies[i].Cy(), clicked: true}
			if v.update(a, link, code) || copied != want || v.copiedT[i] == 0 || v.copiedT[1-i] != 0 {
				t.Fatalf("copy button %d copied %q, want %q", i, copied, want)
			}
		}
	}
}
