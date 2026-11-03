// Package loopback is an in-process connection: the host's own player talks to the hub through it.
package loopback

import (
	"errors"
	"sync"
)

// ErrClosed is returned once either end has closed.
var ErrClosed = errors.New("loopback: closed")

const queue = 64

// Conn is one end of a pipe.
type Conn struct {
	in   <-chan []byte
	out  chan<- []byte
	done chan struct{} // shared by both ends
	once *sync.Once
}

// Pipe returns the two ends of a new connection. Closing either closes both.
func Pipe() (a, b *Conn) {
	ab, ba := make(chan []byte, queue), make(chan []byte, queue)
	done, once := make(chan struct{}), new(sync.Once)
	return &Conn{in: ba, out: ab, done: done, once: once}, &Conn{in: ab, out: ba, done: done, once: once}
}

// Send copies msg to the other end, waiting while its queue is full.
func (c *Conn) Send(msg []byte) error {
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	select {
	case c.out <- append([]byte(nil), msg...):
		return nil
	case <-c.done:
		return ErrClosed
	}
}

// Receive returns the next message. Messages sent before a close are still delivered.
func (c *Conn) Receive() ([]byte, error) {
	select {
	case m := <-c.in:
		return m, nil
	case <-c.done:
		select {
		case m := <-c.in:
			return m, nil
		default:
			return nil, ErrClosed
		}
	}
}

// Close closes both ends.
func (c *Conn) Close() error {
	c.once.Do(func() { close(c.done) })
	return nil
}
