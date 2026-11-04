// Package rtc carries hub connections over a WebRTC data channel (ordered and reliable), for online games. The host
// offers and each joiner answers; signaling carries the two descriptions between them. It builds natively and for
// the browser, where pion uses the browser's own WebRTC.
package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

// ErrClosed is returned once the connection has closed.
var ErrClosed = errors.New("rtc: closed")

const (
	inQueue     = 256     // received messages waiting for Receive; a hub reads far faster than peers may send
	maxBuffered = 1 << 20 // bytes queued for sending before Send gives up on a stalled peer
	flushWait   = time.Second
	gatherWait  = 5 * time.Second // leave time to exchange descriptions and open the channel
)

// Peer is one side of a connection while it is being set up.
type Peer struct {
	pc   *webrtc.PeerConnection
	open chan *Conn
}

// Offer starts the host's side and returns its description, ICE candidates included, for signaling to send to the
// joiner. iceServers is the list signaling issued, as JSON; relay limits the connection to TURN relays (the Force relay
// setting, iceTransportPolicy=relay).
//
// shortcut: descriptions include the candidates found within gatherWait; trickle candidates if slower networks need
// addresses that arrive later. An unreachable ICE server must not hold up the working addresses.
func Offer(ctx context.Context, iceServers json.RawMessage, relay bool) (*Peer, string, error) {
	p, err := newPeer(iceServers, relay)
	if err != nil {
		return nil, "", err
	}
	dc, err := p.pc.CreateDataChannel("orbitrix", nil) // ordered and reliable by default
	if err != nil {
		return p.fail(fmt.Errorf("rtc: data channel: %w", err))
	}
	p.attach(dc)
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return p.fail(fmt.Errorf("rtc: offer: %w", err))
	}
	return p.describe(ctx, offer)
}

// Accept applies the joiner's answer to the host's side.
func (p *Peer) Accept(answer string) error {
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		p.pc.Close()
		return fmt.Errorf("rtc: answer: %w", err)
	}
	return nil
}

// Answer starts the joiner's side from the host's offer and returns the answer for signaling to send back.
func Answer(ctx context.Context, iceServers json.RawMessage, offer string, relay bool) (*Peer, string, error) {
	p, err := newPeer(iceServers, relay)
	if err != nil {
		return nil, "", err
	}
	p.pc.OnDataChannel(p.attach)
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		return p.fail(fmt.Errorf("rtc: offer: %w", err))
	}
	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return p.fail(fmt.Errorf("rtc: answer: %w", err))
	}
	return p.describe(ctx, answer)
}

// Close abandons a connection still being set up: an invite nobody answered.
func (p *Peer) Close() { p.pc.Close() }

// Conn waits for the data channel to open.
func (p *Peer) Conn(ctx context.Context) (*Conn, error) {
	select {
	case c := <-p.open:
		return c, nil
	case <-ctx.Done():
		p.pc.Close()
		return nil, fmt.Errorf("rtc: connect: %w", ctx.Err())
	}
}

func newPeer(iceServers json.RawMessage, relay bool) (*Peer, error) {
	var cfg webrtc.Configuration
	if relay {
		cfg.ICETransportPolicy = webrtc.ICETransportPolicyRelay
	}
	if len(iceServers) > 0 {
		if err := json.Unmarshal(iceServers, &cfg.ICEServers); err != nil {
			return nil, fmt.Errorf("rtc: ICE servers: %w", err)
		}
	}
	pc, err := webrtc.NewPeerConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("rtc: %w", err)
	}
	return &Peer{pc: pc, open: make(chan *Conn, 1)}, nil
}

func (p *Peer) fail(err error) (*Peer, string, error) {
	p.pc.Close()
	return nil, "", err
}

// describe waits for gathering to finish, or sends the addresses already found after gatherWait.
func (p *Peer) describe(ctx context.Context, d webrtc.SessionDescription) (*Peer, string, error) {
	gathered := webrtc.GatheringCompletePromise(p.pc)
	if err := p.pc.SetLocalDescription(d); err != nil {
		return p.fail(fmt.Errorf("rtc: local description: %w", err))
	}
	timer := time.NewTimer(gatherWait)
	defer timer.Stop()
	select {
	case <-gathered:
	case <-timer.C:
	case <-ctx.Done():
		return p.fail(fmt.Errorf("rtc: gathering: %w", ctx.Err()))
	}
	sdp := p.pc.LocalDescription().SDP
	if !strings.Contains(sdp, "\na=candidate:") {
		return p.fail(errors.New("rtc: no connection addresses found"))
	}
	return p, sdp, nil
}

func (p *Peer) attach(dc *webrtc.DataChannel) {
	c := &Conn{pc: p.pc, dc: dc, in: make(chan []byte, inQueue), done: make(chan struct{})}
	dc.OnOpen(func() { p.open <- c })
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		select {
		case c.in <- m.Data: // pion hands each message its own slice
		default:
			c.Close() // a peer that floods faster than the hub reads
		}
	})
	dc.OnClose(func() { c.Close() })
	p.pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			c.Close()
		}
	})
}

// Conn is a hub connection over an open data channel.
type Conn struct {
	pc   *webrtc.PeerConnection
	dc   *webrtc.DataChannel
	in   chan []byte
	done chan struct{}
	once sync.Once

	// Peer is the signaling id of the joiner at the other end, for the host to name in a revoke.
	Peer string
}

func (c *Conn) Send(msg []byte) error {
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	if c.dc.BufferedAmount() > maxBuffered {
		return errors.New("rtc: peer stopped reading")
	}
	return c.dc.Send(msg)
}

// Receive returns the next message. Messages that arrived before a close are still delivered.
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

// Close gives queued messages up to a second to leave, so a closing Bye arrives, then closes the connection.
func (c *Conn) Close() error {
	c.once.Do(func() {
		close(c.done)
		go func() { // pion callbacks call Close, and must not block on the connection
			for end := time.Now().Add(flushWait); c.dc.BufferedAmount() > 0 && time.Now().Before(end); {
				time.Sleep(10 * time.Millisecond)
			}
			c.pc.Close()
		}()
	})
	return nil
}
