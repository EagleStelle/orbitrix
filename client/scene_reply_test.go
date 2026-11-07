package client

import (
	"context"
	"testing"
	"time"

	"orbitrix/client/online"
	"orbitrix/server/transport/loopback"
)

func TestReplyBackClosesConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mine, theirs := loopback.Pipe()
	defer theirs.Close()
	s := &replyScene{cancel: cancel, events: make(chan replyEvent, 2)}
	s.events <- replyEvent{c: mine}
	close(s.events)
	a := setupTestApp(390, 844, 1, 2)
	a.in.esc = true
	if err := s.update(a); err != nil || ctx.Err() == nil || a.net != nil {
		t.Fatalf("Back didn't abandon the reply: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := theirs.Receive(); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("abandoned connection is still open")
		}
	case <-time.After(time.Second):
		t.Fatal("abandoned connection wasn't closed")
	}
}

func TestReplyInvalidInvite(t *testing.T) {
	s := &replyScene{events: make(chan replyEvent, 1)}
	s.events <- replyEvent{err: online.ErrInvite}
	close(s.events)
	a := setupTestApp(390, 844, 1, 2)
	if err := s.update(a); err != nil || !s.failed || len(s.lines) == 0 || s.problem != "This invite is not valid. Ask the host for a new link." {
		t.Fatalf("invalid invite must explain recovery: %q, %v", s.problem, err)
	}
}
