package lan

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/transport/loopback"
	"orbitrix/server/transport/ws"
)

// Port is where a LAN host listens first. When another program has it, the host takes the next free one.
const Port = 7777

// ErrNoNetwork means the device is on no private network for players to reach it through.
var ErrNoNetwork = errors.New("lan: not on a local network")

// Host runs a LAN room in this process: the hub, a WebSocket listener (/ws) on each private network the device is
// on, and the announcements that let joiners find it. A public room has no code: everyone on the network may join
// and the host can kick. A private one is announced as private and takes its code. The host's own player talks to the
// hub through Me.
type Host struct {
	Room *hub.Room
	Me   hub.Conn
	Addr string // host:port, on the network the other players most likely share

	// stop ends the listener and the announcements. A func, not the http.Server: a Host reachable by reflection would
	// keep all of the server's methods, TLS included, in builds that never host (the browser's).
	stop    func()
	private atomic.Bool
}

// Open starts a room for cfg, announced under name, and listens for players on private networks only. A cfg.Code
// makes the room private.
func Open(cfg hub.Config, name string) (*Host, error) {
	nets := networks()
	if len(nets) == 0 {
		return nil, ErrNoNetwork
	}
	room, err := hub.New(cfg)
	if err != nil {
		return nil, err
	}
	lns, port, err := listen(nets)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if c, err := ws.Accept(w, r); err == nil { // on failure Accept has answered the request
			room.Add(c)
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 8 << 10}
	for _, ln := range lns {
		go srv.Serve(ln) // returns ErrServerClosed once Close has run
	}
	h := &Host{Room: room, Addr: net.JoinHostPort(preferred(nets).String(), strconv.Itoa(port))}
	h.private.Store(cfg.Code != "")
	mine, theirs := loopback.Pipe()
	room.Add(theirs)
	h.Me = mine
	var id [4]byte
	_, _ = rand.Read(id[:]) // never fails
	ctx, cancel := context.WithCancel(context.Background())
	h.stop = func() {
		cancel()
		srv.Close() // the players' WebSockets were handed to the room, which closes them
	}
	go announce(ctx, func() Info {
		i := info(room.Lobby(), binary.BigEndian.Uint32(id[:]), name, port)
		i.Private = h.private.Load()
		return i
	})
	return h, nil
}

// SetCode makes the room private with code, or public with "". Players already in keep their seats.
func (h *Host) SetCode(code string) {
	h.Room.SetCode(code)
	h.private.Store(code != "")
}

// Close ends the room for everyone and stops listening.
func (h *Host) Close() {
	h.Room.Close() // says Bye and closes the players' WebSockets
	h.stop()
}

func info(l proto.Lobby, id uint32, name string, port int) Info {
	i := Info{ID: id, Name: name, Playing: l.Playing, Port: uint16(port)}
	for _, s := range l.Seats {
		if s.State == proto.SeatOpen {
			i.Open++
		}
	}
	return i
}

// listen opens one port on every network, trying Port first.
func listen(nets []*net.IPNet) ([]net.Listener, int, error) {
	var err error
	for port := Port; port < Port+16; port++ {
		var lns []net.Listener
		for _, n := range nets {
			var ln net.Listener
			if ln, err = net.Listen("tcp4", net.JoinHostPort(n.IP.String(), strconv.Itoa(port))); err != nil {
				break
			}
			lns = append(lns, ln)
		}
		if err == nil {
			return lns, port, nil
		}
		for _, ln := range lns {
			ln.Close()
		}
	}
	return nil, 0, fmt.Errorf("lan: listen: %w", err)
}

// preferred is the device's address on the network its traffic leaves by: the Wi-Fi or Ethernet the other players
// share, rather than a virtual adapter's.
func preferred(nets []*net.IPNet) net.IP {
	if c, err := net.Dial("udp4", "192.0.2.1:9"); err == nil { // a UDP dial only looks up the route: nothing is sent
		ip := c.LocalAddr().(*net.UDPAddr).IP
		c.Close()
		for _, n := range nets {
			if n.IP.Equal(ip) {
				return n.IP
			}
		}
	}
	return nets[0].IP
}
