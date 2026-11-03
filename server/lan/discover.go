package lan

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wlynxg/anet"

	"orbitrix/proto"
)

// A host announces its room every second to the broadcast address of each private IPv4 network it is on; joiners
// listen on DiscoveryPort. Broadcast rather than multicast: it needs no group joins, which Android 11 and later make
// hard to do per interface, and home routers pass it as readily.
const (
	DiscoveryPort = 7778
	announceEvery = time.Second
	forgetAfter   = 3 * time.Second // a host not heard from for this long has gone
)

// magic starts every announcement, so stray packets on the port are ignored.
var magic = [4]byte{'O', 'R', 'B', 'X'}

// head is an announcement's fixed part: magic, version, id, flags, open seats, port, name length.
const head = 4 + 1 + 4 + 1 + 1 + 2 + 1

// Info is what a host announces about its room.
type Info struct {
	ID      uint32 // random per room: a host on two networks is heard twice, but is one room
	Name    string // the host device's name
	Open    uint8  // human seats nobody holds
	Playing bool
	Private bool   // joining takes the room's code, so the room isn't listed
	Port    uint16 // the hub's WebSocket port
}

// Room is a host heard on the network.
type Room struct {
	Addr string // host:port of its hub
	Info
}

func (i Info) append(b []byte) []byte {
	var flags byte
	if i.Playing {
		flags |= 1
	}
	if i.Private {
		flags |= 2
	}
	name := i.Name[:min(len(i.Name), proto.MaxName)]
	b = append(b, magic[:]...)
	b = append(b, proto.Version)
	b = binary.BigEndian.AppendUint32(b, i.ID)
	b = append(b, flags, i.Open)
	b = binary.BigEndian.AppendUint16(b, i.Port)
	b = append(b, byte(len(name)))
	return append(b, name...)
}

// parse reads an announcement. A host on another protocol version is no room this player could join.
func parse(b []byte) (Info, bool) {
	if len(b) < head || [4]byte(b[:4]) != magic || b[4] != proto.Version || len(b) != head+int(b[head-1]) {
		return Info{}, false
	}
	i := Info{ID: binary.BigEndian.Uint32(b[5:9]), Playing: b[9]&1 != 0, Private: b[9]&2 != 0, Open: b[10], Port: binary.BigEndian.Uint16(b[11:13]),
		Name: strings.ToValidUTF8(string(b[head:]), "")}
	return i, i.Port != 0
}

// networks returns the device's private IPv4 networks. anet reads them where Android 11 and later refuse Go's own
// lookup.
func networks() []*net.IPNet {
	addrs, err := anet.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []*net.IPNet
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.To4() == nil || !n.IP.IsPrivate() {
			continue
		}
		m := n.Mask
		if len(m) == net.IPv6len {
			m = m[12:]
		}
		out = append(out, &net.IPNet{IP: n.IP.To4(), Mask: m})
	}
	return out
}

// broadcast is n's broadcast address, or nil for a network too small to have one.
func broadcast(n *net.IPNet) net.IP {
	if ones, bits := n.Mask.Size(); bits != 32 || ones > 30 {
		return nil
	}
	b := make(net.IP, net.IPv4len)
	for i := range b {
		b[i] = n.IP[i] | ^n.Mask[i]
	}
	return b
}

// announce tells every network the device is on about the room, each second until ctx ends. The networks are read
// again each time, so a host that changes Wi-Fi is heard on the new one.
func announce(ctx context.Context, info func() Info) {
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return // players can still join by address
	}
	defer c.Close()
	t := time.NewTicker(announceEvery)
	defer t.Stop()
	var b []byte
	for {
		b = info().append(b[:0])
		for _, n := range networks() {
			if ip := broadcast(n); ip != nil {
				_, _ = c.WriteToUDP(b, &net.UDPAddr{IP: ip, Port: DiscoveryPort}) // a network that went away misses one
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Finder hears the rooms hosts announce. Android passes broadcasts on only to apps holding a multicast lock, which
// the client takes while it listens.
type Finder struct {
	c    *net.UDPConn
	mu   sync.Mutex
	seen map[uint32]heard // by room ID
}

type heard struct {
	room Room // as last heard
	at   time.Time
}

// Find starts listening for rooms. It fails when another program, or a second Orbitrix on this device, has the port.
func Find() (*Finder, error) {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{Port: DiscoveryPort})
	if err != nil {
		return nil, fmt.Errorf("lan: find: %w", err)
	}
	f := &Finder{c: c, seen: map[uint32]heard{}}
	go f.read()
	return f, nil
}

func (f *Finder) read() {
	b := make([]byte, 512)
	for {
		n, from, err := f.c.ReadFromUDP(b)
		if errors.Is(err, net.ErrClosed) {
			return
		}
		info, ok := parse(b[:n])
		if err != nil || !ok {
			continue
		}
		addr := net.JoinHostPort(from.IP.String(), strconv.Itoa(int(info.Port)))
		f.mu.Lock()
		f.seen[info.ID] = heard{Room{addr, info}, time.Now()}
		f.mu.Unlock()
	}
}

// Rooms appends the rooms heard in the last few seconds to dst, one per host, by name.
func (f *Finder) Rooms(dst []Room) []Room {
	f.mu.Lock()
	defer f.mu.Unlock()
	start := len(dst)
	for id, h := range f.seen {
		if time.Since(h.at) > forgetAfter {
			delete(f.seen, id)
			continue
		}
		dst = append(dst, h.room)
	}
	slices.SortFunc(dst[start:], func(a, b Room) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Addr, b.Addr))
	})
	return dst
}

// Close stops listening.
func (f *Finder) Close() { f.c.Close() }
