package client

import (
	"context"
	"errors"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/proto"
	"orbitrix/server/hub"
)

const replyWait = 10 * time.Minute

// replyScene uses the same invite view as every host, while waiting for the reply's QR to reach the host.
type replyScene struct {
	cancel     context.CancelFunc
	events     chan replyEvent
	invite     inviteView
	link, code string
	failed     bool
	problem    string
	lines      []string
}

type replyEvent struct {
	reply string
	c     hub.Conn
	err   error
}

func newReply(invite string) *replyScene {
	ctx, cancel := context.WithTimeout(context.Background(), replyWait)
	s := &replyScene{cancel: cancel, events: make(chan replyEvent, 2)}
	if !platform.HasWebRTC {
		cancel()
		s.events <- replyEvent{err: errors.New("browser does not support direct connections")}
		close(s.events)
		return s
	}
	go func() {
		defer close(s.events)
		p, reply, err := online.Reply(ctx, invite)
		if err != nil {
			s.events <- replyEvent{err: err}
			return
		}
		s.events <- replyEvent{reply: reply}
		c, err := p.Conn(ctx)
		if err != nil {
			s.events <- replyEvent{err: err}
			return
		}
		s.events <- replyEvent{c: c}
	}()
	return s
}

func (s *replyScene) update(a *app) error {
	link, code := s.link, s.code
	if s.failed {
		link, code = "", ""
	}
	if s.invite.update(a, link, code) || a.clicked(s.invite.done) {
		s.cancel()
		go func() {
			for e := range s.events {
				if e.c != nil {
					e.c.Close()
				}
			}
		}()
		a.next = newHome()
		return nil
	}
	select {
	case e, ok := <-s.events:
		if !ok {
			break
		}
		switch {
		case e.err != nil:
			s.failed = true
			s.problem = "Could not connect to the host. Keep both games open and try a new invite."
			if errors.Is(e.err, online.ErrInvite) {
				s.problem = "This invite is not valid. Ask the host for a new link."
			}
			if !platform.HasWebRTC {
				s.problem = "This browser cannot connect players. Open the invite in another browser."
			}
			s.lines = wrap(s.problem, a.f.label, s.invite.box.w)
		case e.c != nil:
			s.cancel()
			n := newSession(online.NewLink(e.c))
			n.invited = true
			n.link.Send(proto.Hello{Name: a.set.name()})
			a.net, a.next = n, newRoom(n)
			return nil
		default:
			s.code, s.link = e.reply, platform.Address()+"/play#r="+e.reply
		}
	default:
	}
	return nil
}

func (s *replyScene) draw(dst *ebiten.Image, a *app) {
	link, code := s.link, s.code
	if s.failed {
		link, code = "", ""
	}
	s.invite.draw(dst, a, "Reply", link, code, "Done", true)
	if s.failed {
		s.lines = wrap(s.problem, a.f.label, s.invite.box.w)
		for i, line := range s.lines {
			y := s.invite.box.cy() + (float64(i)-float64(len(s.lines)-1)/2)*24*a.u
			drawText(dst, line, a.f.label, s.invite.box.cx(), y, text.AlignCenter, a.sc.ink)
		}
	}
}
