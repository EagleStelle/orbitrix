package lan

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Watch hears the one host at addr (host:port) by asking it how its room stands (/room) as often as hosts announce.
// It is how a browser finds the room on a page a host served: a page can't hear broadcasts.
func Watch(addr string) *Finder {
	ctx, cancel := context.WithCancel(context.Background())
	f := &Finder{stop: cancel, seen: map[uint32]heard{}}
	go f.watch(ctx, addr)
	return f
}

func (f *Finder) watch(ctx context.Context, addr string) {
	client := http.Client{Timeout: forgetAfter}
	for {
		if info, ok := ask(ctx, &client, addr); ok {
			f.hear(Room{addr, info})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(announceEvery):
		}
	}
}

// ask fetches one announcement from the host at addr.
func ask(ctx context.Context, client *http.Client, addr string) (Info, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/room", nil)
	if err != nil {
		return Info{}, false
	}
	res, err := client.Do(req)
	if err != nil {
		return Info{}, false
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 512)) // an announcement fits a UDP packet's 512
	if err != nil || res.StatusCode != http.StatusOK {
		return Info{}, false
	}
	return parse(b)
}
