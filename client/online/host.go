package online

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/signal"
	"orbitrix/server/transport/loopback"
	"orbitrix/server/transport/rtc"
)

var (
	// ErrPassword means the server wants a host password and none was given.
	ErrPassword = errors.New("online: host password needed")
	// ErrBusy means the server has no room for another room.
	ErrBusy = errors.New("online: server busy")
)

const (
	pingEvery   = 25 * time.Second // Cloudflare closes WebSockets idle for about 100 s
	connectWait = 30 * time.Second // from an approved joiner's offer to their open data channel
	backoffMin  = time.Second
	backoffMax  = 30 * time.Second
)

// Request is a joiner waiting for the host's answer.
type Request struct{ Peer, Name string }

// Host runs an online room in this process: the hub, the host's own player on a loopback connection, and signaling,
// which it keeps connected for the whole session so joiners can reach it.
type Host struct {
	Room *hub.Room
	Me   *Link // the host's own player

	url, password string
	relay, public bool
	humans        int
	code          string
	ctx           context.Context
	cancel        context.CancelFunc

	mu      sync.Mutex
	k       *signal.Client // nil while reconnecting
	secret  string
	ice     json.RawMessage
	reqs    []Request
	peers   map[string]*rtc.Peer // approved joiners whose channel isn't open yet
	update  *signal.Msg          // the room as the host last described it, sent again after a reconnect
	offline bool                 // signaling is down: the game goes on, but nobody new can join
}

// Open creates a room on the signaling server at url and starts its hub with cfg. A public room is listed for anyone
// once Update describes it.
// It returns ErrPassword when the server wants a password and
// password is empty, ErrRefused when the password is wrong, and ErrBusy when the server is full.
func Open(ctx context.Context, url, password string, relay, public bool, cfg hub.Config) (*Host, error) {
	humans := 0
	for _, s := range cfg.Seats {
		if s.Human {
			humans++
		}
	}
	h := &Host{url: url, password: password, relay: relay, public: public, humans: humans, code: cfg.Code, peers: map[string]*rtc.Peer{}}
	k, err := h.create(ctx)
	if err != nil {
		return nil, err
	}
	cfg.Code = ""
	cfg.OnClose = h.closed
	if h.Room, err = hub.New(cfg); err != nil {
		k.Close()
		return nil, err
	}
	mine, theirs := loopback.Pipe()
	h.Room.Add(theirs)
	h.Me = NewLink(mine)
	h.ctx, h.cancel = context.WithCancel(context.Background())
	h.k = k // so sends work at once, before run takes over the socket
	go h.run(k)
	go h.ping()
	return h, nil
}

// create dials signaling and opens a new room on it.
func (h *Host) create(ctx context.Context) (*signal.Client, error) {
	k, err := signal.Dial(ctx, h.url)
	if err != nil {
		return nil, err
	}
	if !k.Open && h.password == "" {
		k.Close()
		return nil, ErrPassword
	}
	m := signal.Msg{T: "host.create", Humans: h.humans, Public: h.public, Code: h.code}
	if !k.Open {
		m.Password = h.password
	}
	if err := k.Send(ctx, m); err != nil {
		k.Close()
		return nil, err
	}
	r, err := k.Receive(ctx)
	switch {
	case err != nil:
	case r.T == "room":
		h.mu.Lock()
		h.code, h.secret, h.ice = r.Code, r.Secret, r.ICE
		h.mu.Unlock()
		return k, nil
	case r.Error == "refused":
		err = ErrRefused
	case r.Error == "busy":
		err = ErrBusy
	default:
		err = fmt.Errorf("online: host: unexpected %q %q", r.T, r.Error)
	}
	k.Close()
	return nil, err
}

// Code is the room's code. It changes if the server lost the room (a restart) and the host made a new one.
func (h *Host) Code() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.code
}

// Offline reports that signaling is down: the game goes on, but nobody can join until it is back.
func (h *Host) Offline() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.offline
}

// Requests appends the joiners waiting for an answer to dst, oldest first.
func (h *Host) Requests(dst []Request) []Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append(dst, h.reqs...)
}

// Approve lets a waiting joiner in; Deny turns them away.
func (h *Host) Approve(peer string) { h.answer(peer, "approve") }
func (h *Host) Deny(peer string)    { h.answer(peer, "deny") }

func (h *Host) answer(peer, t string) {
	h.mu.Lock()
	h.drop(peer)
	h.mu.Unlock()
	h.send(signal.Msg{T: t, Peer: peer})
}

// Update tells signaling how the room stands: whether it is public, how many human seats it has (which bounds its
// TURN credentials) and how the list shows it.
func (h *Host) Update(public bool, humans int, l signal.Listed) {
	m := signal.Msg{T: "update", Public: public, Humans: humans, Listed: &l}
	h.mu.Lock()
	h.update = &m
	h.mu.Unlock()
	h.send(m)
}

// Close ends the room for everyone and leaves signaling.
func (h *Host) Close() {
	h.Room.Close()
	h.send(signal.Msg{T: "close"})
	h.cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.k != nil {
		h.k.Close()
		h.k = nil
	}
}

func (h *Host) send(m signal.Msg) {
	h.mu.Lock()
	k := h.k
	h.mu.Unlock()
	if k != nil {
		_ = k.Send(h.ctx, m) // a dead socket shows up in run, which reconnects
	}
}

// drop forgets a request. h.mu must be held.
func (h *Host) drop(peer string) {
	for i, r := range h.reqs {
		if r.Peer == peer {
			h.reqs = append(h.reqs[:i], h.reqs[i+1:]...)
			return
		}
	}
}

// closed runs when the hub lets go of a connection: the joiner's TURN credential goes with it.
func (h *Host) closed(c hub.Conn) {
	if rc, ok := c.(*rtc.Conn); ok && rc.Peer != "" {
		h.send(signal.Msg{T: "revoke", Peer: rc.Peer})
	}
}

func (h *Host) ping() {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-t.C:
			h.send(signal.Msg{T: "ping"})
		}
	}
}

// run serves signaling for the whole session, reconnecting whenever the socket drops.
func (h *Host) run(k *signal.Client) {
	for {
		h.mu.Lock()
		h.k, h.offline = k, false
		h.mu.Unlock()
		h.serve(k)
		h.mu.Lock()
		h.k, h.offline, h.reqs = nil, true, h.reqs[:0]
		h.mu.Unlock()
		k.Close()
		if k = h.reconnect(); k == nil {
			return
		}
	}
}

// reconnect dials signaling again with backoff and jitter, reclaims the room, or opens a new one (with a new code)
// if the server lost it. It returns nil once the host has closed.
func (h *Host) reconnect() *signal.Client {
	wait := backoffMin
	for {
		select {
		case <-h.ctx.Done():
			return nil
		case <-time.After(wait/2 + time.Duration(time.Now().UnixNano()%int64(wait/2+1))): // jitter, so hosts a restart dropped don't return together
		}
		wait = min(2*wait, backoffMax)
		k, err := h.reclaim()
		if errors.Is(err, ErrRefused) {
			k, err = h.create(h.ctx)
		}
		if err == nil {
			h.mu.Lock()
			u := h.update
			h.mu.Unlock()
			if u != nil {
				_ = k.Send(h.ctx, *u) // a new room starts private and undescribed
			}
			return k
		}
	}
}

func (h *Host) reclaim() (*signal.Client, error) {
	k, err := signal.Dial(h.ctx, h.url)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	m := signal.Msg{T: "host.reclaim", Code: h.code, Secret: h.secret}
	h.mu.Unlock()
	if err = k.Send(h.ctx, m); err == nil {
		var r signal.Msg
		if r, err = k.Receive(h.ctx); err == nil && r.T == "room" {
			return k, nil
		} else if err == nil {
			err = ErrRefused
		}
	}
	k.Close()
	return nil, err
}

// serve answers one signaling socket until it drops.
func (h *Host) serve(k *signal.Client) {
	for {
		m, err := k.Receive(h.ctx)
		if err != nil {
			return
		}
		switch m.T {
		case "request":
			h.request(m)
		case "approved":
			go h.offer(m.Peer)
		case "signal":
			h.accept(m.Peer, m.SDP)
		case "gone", "error":
			h.mu.Lock()
			h.drop(m.Peer)
			h.mu.Unlock()
		}
	}
}

// request lets a returning player straight back in; anyone else waits for the host's answer.
func (h *Host) request(m signal.Msg) {
	var t proto.Token
	if b, err := hex.DecodeString(m.Token); err == nil && len(b) == len(t) {
		copy(t[:], b)
		if h.Room.Rejoins(t) {
			h.send(signal.Msg{T: "approve", Peer: m.Peer})
			return
		}
	}
	h.mu.Lock()
	h.reqs = append(h.reqs, Request{m.Peer, m.Name})
	h.mu.Unlock()
}

// offer opens a data channel for an approved joiner and sends them its description.
func (h *Host) offer(peer string) {
	h.mu.Lock()
	ice := h.ice
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(h.ctx, connectWait)
	p, sdp, err := rtc.Offer(ctx, ice, h.relay)
	if err != nil {
		cancel()
		h.send(signal.Msg{T: "revoke", Peer: peer})
		return
	}
	h.mu.Lock()
	h.peers[peer] = p
	h.mu.Unlock()
	h.send(signal.Msg{T: "signal", Peer: peer, SDP: sdp})
	go func() {
		defer cancel()
		c, err := p.Conn(ctx)
		h.mu.Lock()
		delete(h.peers, peer)
		h.mu.Unlock()
		if err != nil {
			h.send(signal.Msg{T: "revoke", Peer: peer})
			return
		}
		c.Peer = peer
		h.Room.Add(c)
	}()
}

func (h *Host) accept(peer, sdp string) {
	h.mu.Lock()
	p := h.peers[peer]
	h.mu.Unlock()
	if p != nil {
		_ = p.Accept(sdp) // on failure the channel never opens, and offer revokes the credential when it gives up
	}
}
