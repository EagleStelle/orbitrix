package online

import (
	"context"
	"sync"
	"time"

	"orbitrix/server/signal"
)

// listEvery is how often the Online page asks for the public rooms.
const listEvery = 3 * time.Second

// Rooms keeps the server's list of public rooms, asked for every few seconds on one signaling socket while the Online
// page is open. A dropped socket is dialled again with backoff, and the list stays up until a dial fails: the server
// ends every socket that hasn't hosted or joined within 30 s, so a page left open redials twice a minute.
type Rooms struct {
	mu     sync.Mutex
	rooms  []signal.Listed
	cancel context.CancelFunc
}

// ListRooms starts listing the public rooms on the signaling server at url, until Close.
func ListRooms(url string) *Rooms {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Rooms{cancel: cancel}
	go r.run(ctx, url)
	return r
}

func (r *Rooms) run(ctx context.Context, url string) {
	wait := backoffMin
	for ctx.Err() == nil {
		if k, err := signal.Dial(ctx, url); err == nil {
			wait = backoffMin
			r.poll(ctx, k)
			k.Close()
		} else {
			r.set(nil)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, backoffMax)
	}
}

func (r *Rooms) poll(ctx context.Context, k *signal.Client) {
	for {
		if k.Send(ctx, signal.Msg{T: "list"}) != nil {
			return
		}
		m, err := k.Receive(ctx)
		if err != nil || m.T != "rooms" {
			return
		}
		r.set(m.Rooms)
		select {
		case <-ctx.Done():
			return
		case <-time.After(listEvery):
		}
	}
}

func (r *Rooms) set(l []signal.Listed) {
	r.mu.Lock()
	r.rooms = l
	r.mu.Unlock()
}

// Get appends the rooms last listed to dst.
func (r *Rooms) Get(dst []signal.Listed) []signal.Listed {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append(dst, r.rooms...)
}

// Close stops listing.
func (r *Rooms) Close() { r.cancel() }
