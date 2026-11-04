package online

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"orbitrix/server/hub"
	"orbitrix/server/signal"
	"orbitrix/server/transport/loopback"
	"orbitrix/server/transport/rtc"
)

// ErrInvite means a link carries no invite or reply this build can read.
var ErrInvite = errors.New("online: not an invite")

// iceServers is the STUN server alone: it lets two devices on one Wi-Fi meet when the network keeps them from each
// other's local addresses. A server-free room has no TURN.
var iceServers, _ = json.Marshal([]map[string][]string{{"urls": signal.DefaultSTUN}}) // never fails

const (
	prepareWait    = 15 * time.Second // to gather an invite's candidates
	retryAfter     = 5 * time.Second  // before preparing again after an invite didn't come together
	maxDescription = 16 << 10         // bytes of session description an invite or reply may unpack to
)

// Direct runs a server-free room in this process (a browser's Same Wi-Fi room): the hub, the host's own player on a
// loopback connection, and one open invite at a time. Players come in by invite links with no signaling: the invite
// carries the host's side of a WebRTC connection, the joiner's reply carries theirs back, and the players carry each
// between their devices, as a QR code or a shared link. Each invite lets one player in.
type Direct struct {
	Room *hub.Room
	Me   *Link // the host's own player

	mu         sync.Mutex
	peer       *rtc.Peer // the open invite's side of its connection
	id         [4]byte   // the open invite's, which its reply carries back
	invite     string    // the open invite, packed; "" while one is being prepared
	preparing  bool
	retry      time.Time // no invite is prepared before this, after one failed
	connecting int       // replies taken whose channels aren't open yet
	closed     bool
}

// OpenDirect starts a server-free room's hub with cfg.
func OpenDirect(cfg hub.Config) (*Direct, error) {
	room, err := hub.New(cfg)
	if err != nil {
		return nil, err
	}
	mine, theirs := loopback.Pipe()
	room.Add(theirs)
	return &Direct{Room: room, Me: NewLink(mine)}, nil
}

// Invite is the open invite, packed for a link. When there is none it starts preparing one and returns "" until it
// is ready, which takes the time to gather the host's addresses.
func (d *Direct) Invite() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.invite == "" && !d.preparing && !d.closed && time.Now().After(d.retry) {
		d.preparing = true
		go d.prepare()
	}
	return d.invite
}

func (d *Direct) prepare() {
	ctx, cancel := context.WithTimeout(context.Background(), prepareWait)
	defer cancel()
	p, sdp, err := rtc.Offer(ctx, iceServers, false)
	var id [4]byte
	_, _ = rand.Read(id[:]) // never fails
	d.mu.Lock()
	defer d.mu.Unlock()
	d.preparing = false
	switch {
	case err != nil:
		log.Printf("invite: %v", err)
		d.retry = time.Now().Add(retryAfter)
	case d.closed:
		p.Close()
	default:
		d.peer, d.id, d.invite = p, id, pack(id, sdp)
	}
}

// Take lets in the joiner whose reply answers the open invite, which is then used up; the next call to Invite
// prepares another. A reply to any other invite is dropped, and its joiner waits in vain.
func (d *Direct) Take(reply string) {
	id, sdp, err := unpack(reply)
	if err != nil {
		return
	}
	d.mu.Lock()
	p := d.peer
	if p == nil || id != d.id {
		d.mu.Unlock()
		return
	}
	d.peer, d.invite = nil, ""
	d.connecting++
	d.mu.Unlock()
	go func() { // off the game loop: in the browser, every step waits on a promise
		defer func() {
			d.mu.Lock()
			d.connecting--
			d.mu.Unlock()
		}()
		if err := p.Accept(sdp); err != nil { // Accept closes p
			log.Printf("reply: %v", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), connectWait)
		defer cancel()
		if c, err := p.Conn(ctx); err == nil {
			d.Room.Add(c) // a room closed meanwhile closes c
		}
	}()
}

// Connecting reports whether a joiner's reply has been taken and their connection isn't open yet.
func (d *Direct) Connecting() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connecting > 0
}

// Close ends the room for everyone and drops the open invite.
func (d *Direct) Close() {
	d.mu.Lock()
	p := d.peer
	d.closed, d.peer, d.invite = true, nil, ""
	d.mu.Unlock()
	if p != nil {
		p.Close()
	}
	d.Room.Close()
}

// Reply answers an invite: it returns the joiner's side of the connection, which opens once the host takes the
// reply, and the reply to carry back to the host.
func Reply(ctx context.Context, invite string) (*rtc.Peer, string, error) {
	id, sdp, err := unpack(invite)
	if err != nil {
		return nil, "", err
	}
	p, answer, err := rtc.Answer(ctx, iceServers, sdp, false)
	if err != nil {
		return nil, "", err
	}
	return p, pack(id, answer), nil
}

// pack writes an invite or a reply for a link: the invite's id, then the session description deflated, in URL-safe
// base64. TCP candidates are left out: devices on one Wi-Fi meet over UDP, and the link stays short enough for a QR
// code a phone reads off another phone's screen.
func pack(id [4]byte, sdp string) string {
	var b bytes.Buffer
	b.Write(id[:])
	w, _ := flate.NewWriter(&b, flate.BestCompression) // only fails for an invalid level
	for line := range strings.Lines(sdp) {
		// a=candidate:<foundation> <component> <transport> ...
		if f := strings.Fields(line); len(f) > 2 && strings.HasPrefix(f[0], "a=candidate:") && !strings.EqualFold(f[2], "udp") {
			continue
		}
		_, _ = io.WriteString(w, line) // a bytes.Buffer never fails
	}
	_ = w.Close()
	return base64.RawURLEncoding.EncodeToString(b.Bytes())
}

// unpack reads what pack wrote.
func unpack(s string) (id [4]byte, sdp string, err error) {
	if len(s) > 32<<10 {
		return id, "", ErrInvite
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) < len(id) {
		return id, "", ErrInvite
	}
	copy(id[:], b)
	z := flate.NewReader(bytes.NewReader(b[len(id):]))
	defer z.Close()
	out, err := io.ReadAll(io.LimitReader(z, maxDescription+1))
	if err != nil || len(out) > maxDescription {
		return id, "", ErrInvite
	}
	return id, string(out), nil
}
