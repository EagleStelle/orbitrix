//go:build !js

package online

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
)

func TestPack(t *testing.T) {
	id := [4]byte{1, 2, 3, 4}
	sdp := "v=0\r\na=candidate:1 1 udp 2130706431 192.168.1.5 50000 typ host\r\n" +
		"a=candidate:2 1 tcp 1518280447 192.168.1.5 9 typ host tcptype active\r\n" +
		"a=candidate:3 1 UDP 1694498815 203.0.113.9 50001 typ srflx raddr 0.0.0.0 rport 0\r\na=end-of-candidates\r\n"
	want := strings.Replace(sdp, "a=candidate:2 1 tcp 1518280447 192.168.1.5 9 typ host tcptype active\r\n", "", 1)
	gotID, got, err := unpack(pack(id, sdp))
	if err != nil || gotID != id || got != want {
		t.Fatalf("round trip: %v %v %q", gotID, err, got)
	}
	big := pack(id, strings.Repeat("a", maxDescription+1))
	for name, s := range map[string]string{"empty": "", "not base64": "!!!", "short": "AQID", "not deflate": "AQIDBP____8",
		"too big": big, "long link": strings.Repeat("A", 32<<10+1)} {
		if _, _, err := unpack(s); !errors.Is(err, ErrInvite) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDirect lets a joiner into a server-free room by an invite and its reply, then shows the used invite gone, a
// stale reply dropped, and the next invite a different one.
func TestDirect(t *testing.T) {
	saved := iceServers
	iceServers = nil // local addresses only: no STUN server in tests
	defer func() { iceServers = saved }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d, err := OpenDirect(hub.Config{Cols: 5, Rows: 5, Seats: []hub.Seat{{Human: true}, {Human: true}, {AI: ai.Mason}}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Me.Send(proto.Hello{Name: "Host"})
	invite := ready(t, d)

	p, reply, err := Reply(ctx, invite)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Reply(ctx, "garbage"); !errors.Is(err, ErrInvite) {
		t.Fatalf("a garbled invite: %v", err)
	}
	d.Take(reply)
	c, err := p.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Send(proto.Append(nil, proto.Hello{Name: "Ana"})); err != nil {
		t.Fatal(err)
	}
	for {
		b, err := c.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if m, _ := proto.Decode(b); m != nil {
			if w, ok := m.(proto.Welcome); ok {
				if w.Seat != 1 {
					t.Fatalf("joined seat %d, want 1", w.Seat)
				}
				break
			}
		}
	}

	d.Take(reply) // used up: nothing to answer
	if d.Connecting() {
		t.Fatal("a used invite's reply was taken again")
	}
	if next := ready(t, d); next == invite {
		t.Fatal("the used invite was offered again")
	}
}

// ready waits for d's open invite.
func ready(t *testing.T, d *Direct) string {
	t.Helper()
	for start := time.Now(); ; time.Sleep(10 * time.Millisecond) {
		if s := d.Invite(); s != "" {
			return s
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("no invite within 10 s")
		}
	}
}
