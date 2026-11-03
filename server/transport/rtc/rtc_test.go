//go:build !js

package rtc_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"orbitrix/server/transport/rtc"
)

// TestPair connects two peers in one process, with no ICE servers (host candidates only), and checks both
// directions and that a close reaches the other side after the message sent before it.
func TestPair(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host, offer, err := rtc.Offer(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	joiner, answer, err := rtc.Answer(ctx, nil, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Accept(answer); err != nil {
		t.Fatal(err)
	}
	h, err := host.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	j, err := joiner.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]interface {
		Send([]byte) error
		Receive() ([]byte, error)
	}{{h, j}, {j, h}} {
		if err := pair[0].Send([]byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
		if got, err := pair[1].Receive(); err != nil || !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Fatalf("got %v, %v", got, err)
		}
	}
	if err := h.Send([]byte("bye")); err != nil {
		t.Fatal(err)
	}
	h.Close()
	if got, err := j.Receive(); err != nil || string(got) != "bye" {
		t.Fatalf("last message: %q, %v", got, err)
	}
	if _, err := j.Receive(); err == nil {
		t.Fatal("joiner still open after the host closed")
	}
}
