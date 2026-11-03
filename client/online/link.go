// Package online is a client's side of an online game: hosting a room (signaling, the hub and a WebRTC connection per
// joiner) or joining one. It draws nothing; the client's scenes poll it from the game loop.
package online

import (
	"sync/atomic"

	"orbitrix/proto"
	"orbitrix/server/hub"
)

// Link is a player's connection to a room's hub: messages are decoded on their own goroutine and wait in a queue for
// the game loop, which never blocks on the network.
type Link struct {
	c    hub.Conn
	in   chan proto.Msg
	done atomic.Bool
}

// NewLink starts reading c.
func NewLink(c hub.Conn) *Link {
	l := &Link{c: c, in: make(chan proto.Msg, 256)}
	go l.read()
	return l
}

func (l *Link) read() {
	defer close(l.in)
	for {
		b, err := l.c.Receive()
		if err != nil {
			return
		}
		m, err := proto.Decode(b)
		if err != nil {
			l.c.Close() // the host speaks another version, or something broke: nothing after this can be trusted
			return
		}
		l.in <- m
	}
}

// Poll returns the next message, if one has arrived. ok is false when none is waiting; Closed tells whether more can
// come.
func (l *Link) Poll() (m proto.Msg, ok bool) {
	select {
	case m, ok = <-l.in:
		if !ok {
			l.done.Store(true)
		}
		return m, ok
	default:
		return nil, false
	}
}

// Closed reports that the connection has ended and every message it carried has been polled.
func (l *Link) Closed() bool { return l.done.Load() }

// Send sends m. A failed send shows up as the connection closing.
func (l *Link) Send(m proto.Msg) {
	if l.c.Send(proto.Append(nil, m)) != nil {
		l.c.Close()
	}
}

// Close ends the connection.
func (l *Link) Close() { l.c.Close() }
