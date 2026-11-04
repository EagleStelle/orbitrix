package online

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/signal"
	"orbitrix/server/transport/rtc"
)

// ErrRefused is signaling's one answer for a wrong code, a full approval queue and a host's no: the
// server tells them apart for nobody.
var ErrRefused = errors.New("online: refused")

// ErrConnect means approval succeeded but the connection to the host could not be opened.
var ErrConnect = errors.New("online: connection to host failed")

// Join asks the host of room code to let the player in, and returns the open connection to the host's hub once they
// have. A player who held a seat passes its rejoin token, which the host approves without asking. waiting receives
// true when the host is asked and false on approval. Signaling is closed again before Join returns.
func Join(ctx context.Context, url, code, name string, token proto.Token, relay bool, waiting func(bool)) (conn hub.Conn, err error) {
	k, err := signal.Dial(ctx, url)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	approved := false
	defer func() {
		if approved && err != nil {
			err = fmt.Errorf("%w: %w", ErrConnect, err)
		}
	}()
	m := signal.Msg{T: "join", Code: code, Name: name}
	if token != (proto.Token{}) {
		m.T, m.Token = "rejoin", hex.EncodeToString(token[:])
	}
	if err := k.Send(ctx, m); err != nil {
		return nil, err
	}
	var ice []byte
	for {
		m, err := k.Receive(ctx)
		switch {
		case err != nil:
			return nil, err
		case m.T == "wait":
			waiting(true)
		case m.T == "approved":
			ice = m.ICE
			approved = true
			waiting(false)
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, connectWait)
			defer cancel()
		case m.T == "signal":
			p, answer, err := rtc.Answer(ctx, ice, m.SDP, relay)
			if err != nil {
				return nil, err
			}
			if err := k.Send(ctx, signal.Msg{T: "signal", SDP: answer}); err != nil {
				p.Close()
				return nil, err
			}
			return p.Conn(ctx)
		case m.T == "error" && (m.Error == "refused" || m.Error == "closed"):
			return nil, ErrRefused
		default:
			return nil, fmt.Errorf("online: join: unexpected %q %q", m.T, m.Error)
		}
	}
}
