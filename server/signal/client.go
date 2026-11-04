package signal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coder/websocket"

	"orbitrix/server/signal/pow"
)

// ErrVersion means the server speaks another signaling version: this client, or the server, needs updating.
var ErrVersion = errors.New("signal: other version")

// Client is a game client's signaling connection, already past hello and the proof of work.
type Client struct {
	c *websocket.Conn

	// Open reports that this server lets anyone host without a password (with STUN only), so the client doesn't ask
	// for one.
	Open bool
}

// Dial connects to url (wss://host/signal) and answers the server's challenge.
func Dial(ctx context.Context, url string) (*Client, error) {
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("signal: dial: %w", err)
	}
	c.SetReadLimit(MaxMessage)
	k := &Client{c: c}
	ch, err := k.Receive(ctx)
	switch {
	case err != nil:
	case ch.T != "challenge":
		err = fmt.Errorf("signal: expected challenge, got %q", ch.T)
	case ch.V != Version:
		err = fmt.Errorf("%w: server %d, client %d", ErrVersion, ch.V, Version)
	}
	if err == nil {
		err = k.Send(ctx, Msg{T: "hello", V: Version})
	}
	if err == nil {
		err = k.Send(ctx, Msg{T: "pow", C: ch.C, N: pow.Solve(ch.C, ch.Bits)})
	}
	if err != nil {
		c.CloseNow()
		return nil, err
	}
	k.Open = ch.Open
	return k, nil
}

func (k *Client) Send(ctx context.Context, m Msg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("signal: %w", err)
	}
	if err := k.c.Write(ctx, websocket.MessageText, b); err != nil {
		return fmt.Errorf("signal: send %s: %w", m.T, err)
	}
	return nil
}

func (k *Client) Receive(ctx context.Context) (Msg, error) {
	var m Msg
	_, b, err := k.c.Read(ctx)
	if err != nil {
		return m, fmt.Errorf("signal: receive: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("signal: receive: %w", err)
	}
	return m, nil
}

func (k *Client) Close() error { return k.c.Close(websocket.StatusNormalClosure, "") }
