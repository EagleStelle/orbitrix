package lan

import (
	"context"
	"errors"
	"fmt"
	"time"

	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/transport/ws"
)

// joinWait is how long a host has to connect and answer a Hello.
const joinWait = 3 * time.Second

// Refused is a host's Bye to a Hello: a full room, another version.
type Refused struct{ Reason proto.Reason }

func (r Refused) Error() string { return fmt.Sprintf("lan: refused (reason %d)", r.Reason) }

// Join sends hello to the host at addr (host:port) and returns the connection to its hub once it seats the player,
// with its Welcome. A host that answers with a Bye gives a Refused.
func Join(ctx context.Context, addr string, hello proto.Hello) (hub.Conn, proto.Welcome, error) {
	ctx, cancel := context.WithTimeout(ctx, joinWait)
	defer cancel()
	c, err := ws.Dial(ctx, "ws://"+addr+"/ws")
	if err != nil {
		return nil, proto.Welcome{}, err
	}
	stop := context.AfterFunc(ctx, func() { c.Close() }) // Receive has no deadline of its own
	if err = c.Send(proto.Append(nil, hello)); err == nil {
		var b []byte
		if b, err = c.Receive(); err == nil {
			var m proto.Msg
			m, err = proto.Decode(b)
			switch m := m.(type) {
			case proto.Welcome:
				if stop() {
					return c, m, nil
				}
				err = ctx.Err()
			case proto.Bye:
				err = Refused{m.Reason}
			case nil:
				if errors.Is(err, proto.ErrVersion) {
					err = Refused{proto.ReasonVersion}
				}
			default:
				err = fmt.Errorf("lan: join %s: unexpected message %d", addr, m.Type())
			}
		}
	}
	stop()
	c.Close()
	return nil, proto.Welcome{}, err
}
