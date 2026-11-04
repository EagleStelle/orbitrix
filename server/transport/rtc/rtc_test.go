//go:build !js

package rtc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"orbitrix/server/transport/rtc"
)

// A silent alternate relay must not prevent the peers' working addresses from being exchanged.
func TestPairWithUnresponsiveTURN(t *testing.T) {
	sink, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	ice, _ := json.Marshal([]map[string]any{{"urls": []string{"turn:" + sink.LocalAddr().String() + "?transport=udp"},
		"username": "test", "credential": "test"}})
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	host, offer, err := rtc.Offer(ctx, ice, false)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	joiner, answer, err := rtc.Answer(ctx, ice, offer, false)
	if err != nil {
		t.Fatal(err)
	}
	defer joiner.Close()
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
	if err := h.Send([]byte("joined")); err != nil {
		t.Fatal(err)
	}
	if got, err := j.Receive(); err != nil || string(got) != "joined" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGatherFailure(t *testing.T) {
	sink, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	ice, _ := json.Marshal([]map[string]any{{"urls": []string{"turn:" + sink.LocalAddr().String() + "?transport=udp"},
		"username": "test", "credential": "test"}})
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if canceled {
				cancel()
			}
			p, sdp, err := rtc.Offer(ctx, ice, true)
			if p != nil || sdp != "" || err == nil {
				t.Fatalf("relay-only gathering without a relay: %v, %q, %v", p, sdp, err)
			}
			if canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
		})
	}
}

// TestPair connects two peers in one process, with no ICE servers (host candidates only), and checks both
// directions and that a close reaches the other side after the message sent before it.
func TestPair(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host, offer, err := rtc.Offer(ctx, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	joiner, answer, err := rtc.Answer(ctx, nil, offer, false)
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
