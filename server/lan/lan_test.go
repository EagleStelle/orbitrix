package lan

import (
	"context"
	"errors"
	"testing"
	"time"

	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
)

func TestAnnouncement(t *testing.T) {
	in := Info{ID: 0xdeadbeef, Name: "desk", Open: 2, Playing: true, Private: true, Port: 7777}
	b := in.append(nil)
	if out, ok := parse(b); !ok || out != in {
		t.Fatalf("round trip: got %+v %v", out, ok)
	}
	bad := map[string][]byte{
		"empty":       nil,
		"short":       b[:head-1],
		"magic":       append([]byte("XXXX"), b[4:]...),
		"version":     append(append([]byte{}, b[:4]...), append([]byte{proto.Version + 1}, b[5:]...)...),
		"name length": b[:len(b)-1],
		"trailing":    append(append([]byte{}, b...), 0),
		"no port":     Info{Name: "x"}.append(nil),
	}
	for name, b := range bad {
		if _, ok := parse(b); ok {
			t.Errorf("%s: parsed", name)
		}
	}
	long := Info{Name: string(make([]byte, 300)), Port: 1}.append(nil)
	if out, ok := parse(long); !ok || len(out.Name) != proto.MaxName {
		t.Errorf("a long name must be cut to %d bytes, got %d %v", proto.MaxName, len(out.Name), ok)
	}
}

func TestPublicCode(t *testing.T) {
	for id, want := range map[uint32]string{0: "AAAAAA", 0x08000000: "BAAAAA", 0xffffffff: "999999"} {
		if got := PublicCode(id); got != want {
			t.Errorf("%08x: code %q, want %q", id, got, want)
		}
	}
}

// TestHost opens a room on this device's private networks, hears its announcement, joins it, and is turned away once
// its one human seat is taken.
func TestHost(t *testing.T) {
	if len(networks()) == 0 {
		t.Skip("not on a private network")
	}
	h, err := Open(hub.Config{Cols: 5, Rows: 5, Seats: []hub.Seat{{Human: true}, {AI: ai.Jester}}}, "desk")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if f, err := Find(); err != nil {
		t.Logf("discovery not tested: %v", err)
	} else {
		defer f.Close()
		for start := time.Now(); ; time.Sleep(50 * time.Millisecond) {
			rooms := f.Rooms(nil)
			if len(rooms) > 1 {
				t.Fatalf("one host must be one room, however many networks it is heard on: %+v", rooms)
			}
			if len(rooms) == 1 && rooms[0].Name == "desk" && rooms[0].Open == 1 {
				t.Logf("heard %+v after %v", rooms[0], time.Since(start))
				break
			}
			if time.Since(start) > 3*time.Second {
				t.Fatal("no announcement within 3 s")
			}
		}
	}

	watch := Watch(h.Addr) // as a browser on the page this host served finds it
	defer watch.Close()
	for start := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		rooms := watch.Rooms(nil)
		if len(rooms) == 1 && rooms[0].Addr == h.Addr && rooms[0].Name == "desk" && rooms[0].Open == 1 && !rooms[0].Private {
			if PublicCode(rooms[0].ID) != h.Code || len(h.Code) != proto.CodeLen {
				t.Fatal("a public room's displayed code must match its announcement")
			}
			break
		}
		if time.Since(start) > 3*time.Second {
			t.Fatalf("watching the host: %+v", rooms)
		}
	}
	h.SetCode("ABCDEF")
	for start := time.Now(); ; time.Sleep(50 * time.Millisecond) {
		if rooms := watch.Rooms(nil); len(rooms) == 1 && rooms[0].Private {
			break
		}
		if time.Since(start) > 3*time.Second {
			t.Fatal("a watched room turned private still reads as public")
		}
	}
	h.SetCode("")

	if _, _, err := Join(ctx, "127.0.0.1:1", proto.Hello{}); err == nil {
		t.Fatal("nobody there, yet joined")
	}
	c, w, err := Join(ctx, h.Addr, proto.Hello{Name: "Human"})
	if err != nil || w.Seat != 0 {
		t.Fatalf("join: seat %d, %v", w.Seat, err)
	}
	defer c.Close()
	var r Refused
	if _, _, err := Join(ctx, h.Addr, proto.Hello{Name: "Human"}); !errors.As(err, &r) || r.Reason != proto.ReasonFull {
		t.Fatalf("a full room: %v", err)
	}
}
