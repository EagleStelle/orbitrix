package signal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"orbitrix/proto"
	"orbitrix/server/signal/pow"
)

const testPassword = "correct horse battery staple 42"

// fakeTURN records what signaling asks of Cloudflare.
type fakeTURN struct {
	mu      sync.Mutex
	issued  []string
	revoked []string
	down    bool // Cloudflare fails every request
}

func (f *fakeTURN) Issue(context.Context) (Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return Credential{}, fmt.Errorf("turn: HTTP 503")
	}
	user := fmt.Sprintf("u%d", len(f.issued))
	f.issued = append(f.issued, user)
	ice := fmt.Sprintf(`[{"urls":["turn:turn.example:3478"],"username":%q,"credential":"x"}]`, user)
	return Credential{Username: user, ICEServers: json.RawMessage(ice)}, nil
}

func (f *fakeTURN) Revoke(_ context.Context, user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = append(f.revoked, user)
	return nil
}

func (f *fakeTURN) counts() (issued int, revoked []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.issued), slices.Sorted(slices.Values(f.revoked))
}

// waitRevoked waits for revocations, which run in the background.
func (f *fakeTURN) waitRevoked(t *testing.T, want ...string) {
	t.Helper()
	slices.Sort(want)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, got := f.counts()
		if slices.Equal(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("revoked %v, want %v", got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// setup starts a server with short waits, a fake TURN and no STUN.
func setup(t *testing.T, password string) (*fakeTURN, string) {
	t.Helper()
	turn := &fakeTURN{}
	s := New(Config{Password: password, Origin: "https://orbitrix.example", TURN: turn, STUN: []string{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s.failDelay, s.hostGrace = 10*time.Millisecond, 100*time.Millisecond
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return turn, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, url string) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.c.CloseNow() })
	return c
}

func send(t *testing.T, c *Client, m Msg) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Send(ctx, m); err != nil {
		t.Fatal(err)
	}
}

// recv returns the next message, or one with T "closed" once the server has closed the socket.
func recv(t *testing.T, c *Client) Msg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := c.Receive(ctx)
	if err != nil {
		if ctx.Err() != nil {
			t.Fatal("no message")
		}
		return Msg{T: "closed"}
	}
	return m
}

func expect(t *testing.T, c *Client, typ string) Msg {
	t.Helper()
	m := recv(t, c)
	if m.T != typ {
		t.Fatalf("got %+v, want %q", m, typ)
	}
	return m
}

func host(t *testing.T, url string, humans int) (*Client, Msg) {
	t.Helper()
	h := dial(t, url)
	send(t, h, Msg{T: "host.create", Password: testPassword, Humans: humans})
	return h, expect(t, h, "room")
}

// joinRoom joins and returns the joiner and the peer id its host was asked about.
func joinRoom(t *testing.T, url string, h *Client, code string) (*Client, string) {
	t.Helper()
	j := dial(t, url)
	send(t, j, Msg{T: "join", Code: code, Name: "Bo"})
	expect(t, j, "wait")
	req := expect(t, h, "request")
	if req.Name != "Bo" || req.Peer == "" {
		t.Fatalf("request %+v", req)
	}
	return j, req.Peer
}

func approve(t *testing.T, h, j *Client, peer string) Msg {
	t.Helper()
	send(t, h, Msg{T: "approve", Peer: peer})
	if m := expect(t, h, "approved"); m.Peer != peer {
		t.Fatalf("approved %q, want %q", m.Peer, peer)
	}
	return expect(t, j, "approved")
}

func TestHostPassword(t *testing.T) {
	turn, url := setup(t, testPassword)
	c := dial(t, url)
	if c.Open {
		t.Fatal("a server with a password says hosting is open")
	}
	for _, pw := range []string{"wrong", "", testPassword + " "} {
		send(t, c, Msg{T: "host.create", Password: pw, Humans: 2})
		if m := recv(t, c); m.T != "error" || m.Code != "" || m.ICE != nil {
			t.Fatalf("password %q: %+v", pw, m)
		}
	}
	if m := recv(t, c); m.T != "closed" {
		t.Fatalf("after three failures: %+v", m)
	}
	if n, _ := turn.counts(); n != 0 {
		t.Fatalf("%d credentials issued for wrong passwords", n)
	}

	// A password too short to resist guessing still guards hosting, but never unlocks TURN.
	short := "only19characters!!!"
	turn, url = setup(t, short)
	c = dial(t, url)
	send(t, c, Msg{T: "host.create", Password: "wrong", Humans: 2})
	expect(t, c, "error")
	send(t, c, Msg{T: "host.create", Password: short, Humans: 2})
	if m := expect(t, c, "room"); string(m.ICE) != "[]" || c.Open {
		t.Fatalf("short server password: %+v", m)
	}
	if n, _ := turn.counts(); n != 0 {
		t.Fatal("short server password: TURN credential issued")
	}
}

// TestTURNDownFallsBackToSTUN: when Cloudflare fails, hosts and joiners still connect, with STUN only.
func TestTURNDownFallsBackToSTUN(t *testing.T) {
	turn, url := setup(t, testPassword)
	turn.down = true
	h, rm := host(t, url, 2)
	j, p := joinRoom(t, url, h, rm.Code)
	for _, ice := range []json.RawMessage{rm.ICE, approve(t, h, j, p).ICE} {
		if string(ice) != "[]" {
			t.Fatalf("iceServers %s", ice)
		}
	}
}

// TestRequestedCode: a host gets the code it asks for (a private room shows it before it opens) unless that code is
// taken or malformed, when it gets a fresh one.
func TestRequestedCode(t *testing.T) {
	_, url := setup(t, testPassword)
	for _, tc := range []struct{ ask, want string }{{"abcdef", "ABCDEF"}, {"ABCDEF", ""}, {"ABC0EF", ""}, {"", ""}} {
		c := dial(t, url)
		send(t, c, Msg{T: "host.create", Password: testPassword, Humans: 2, Code: tc.ask})
		got := expect(t, c, "room").Code
		if tc.want != "" && got != tc.want || tc.want == "" && (got == "ABCDEF" || len(got) != proto.CodeLen) {
			t.Fatalf("asked %q, got %q", tc.ask, got)
		}
	}
}

// TestOpenHosting: with no password, anyone hosts without one, and neither hosts nor joiners get TURN even when a key
// is configured. One IP may hold only a few rooms.
func TestOpenHosting(t *testing.T) {
	turn, url := setup(t, "")
	h := dial(t, url)
	if !h.Open {
		t.Fatal("the challenge doesn't say hosting is open")
	}
	send(t, h, Msg{T: "host.create", Humans: 2})
	rm := expect(t, h, "room")
	j, p := joinRoom(t, url, h, rm.Code)
	for _, ice := range []json.RawMessage{rm.ICE, approve(t, h, j, p).ICE} {
		if string(ice) != "[]" { // setup configures no STUN, so STUN only is an empty list
			t.Fatalf("iceServers %s", ice)
		}
	}
	if n, _ := turn.counts(); n != 0 {
		t.Fatalf("%d TURN credentials issued while hosting is open", n)
	}
	for i := 1; i < maxOpenRooms; i++ { // the first room is already open
		c := dial(t, url)
		send(t, c, Msg{T: "host.create", Humans: 2})
		expect(t, c, "room")
	}
	extra := dial(t, url)
	send(t, extra, Msg{T: "host.create", Humans: 2})
	if m := recv(t, extra); m.T != "error" || m.Error != "busy" {
		t.Fatalf("room %d from one IP: %+v", maxOpenRooms+1, m)
	}
}

func TestOnlyHostAndApprovedJoinersGetCredentials(t *testing.T) {
	turn, url := setup(t, testPassword)
	h, rm := host(t, url, 3)
	if !strings.Contains(string(rm.ICE), `"u0"`) || len(rm.Code) != 6 || rm.Secret == "" {
		t.Fatalf("room %+v", rm)
	}
	j1, p1 := joinRoom(t, url, h, rm.Code)
	if n, _ := turn.counts(); n != 1 {
		t.Fatalf("%d credentials before any approval", n)
	}
	send(t, h, Msg{T: "deny", Peer: p1})
	if m := recv(t, j1); m.T != "error" || m.ICE != nil {
		t.Fatalf("denied joiner got %+v", m)
	}

	j2, p2 := joinRoom(t, url, h, strings.ToLower(rm.Code))
	if m := approve(t, h, j2, p2); !strings.Contains(string(m.ICE), `"u1"`) {
		t.Fatalf("approved joiner got %+v", m)
	}
	// Descriptions pass only between the host and its approved joiner.
	send(t, h, Msg{T: "signal", Peer: p2, SDP: "offer"})
	if m := expect(t, j2, "signal"); m.SDP != "offer" {
		t.Fatalf("relayed %+v", m)
	}
	send(t, j2, Msg{T: "signal", SDP: "answer"})
	if m := expect(t, h, "signal"); m.SDP != "answer" || m.Peer != p2 {
		t.Fatalf("relayed %+v", m)
	}

	bad := dial(t, url)
	send(t, bad, Msg{T: "join", Code: "ZZZZZZ", Name: "Eve"})
	if m := recv(t, bad); m.T != "error" || m.ICE != nil {
		t.Fatalf("wrong code: %+v", m)
	}
	if n, _ := turn.counts(); n != 2 {
		t.Fatalf("%d credentials, want the host's and one approved joiner's", n)
	}
}

func TestFailuresNeverBlockTheCorrectSecret(t *testing.T) {
	_, url := setup(t, testPassword)
	for i := 0; i < 5; i++ { // every socket from this IP fails three times and is closed
		c := dial(t, url)
		for k := 0; k < 3; k++ {
			send(t, c, Msg{T: "host.create", Password: "guess", Humans: 2})
			send(t, c, Msg{T: "join", Code: "AAAAAA", Name: "x"})
		}
	}
	h, rm := host(t, url, 2)
	joinRoom(t, url, h, rm.Code)
}

func TestRevocation(t *testing.T) {
	turn, url := setup(t, testPassword)
	h, rm := host(t, url, 4)
	j1, p1 := joinRoom(t, url, h, rm.Code)
	approve(t, h, j1, p1)
	j2, p2 := joinRoom(t, url, h, rm.Code)
	approve(t, h, j2, p2)
	j1.Close() // a joiner leaves signaling once its data channel is open: the credential stays in use

	send(t, h, Msg{T: "revoke", Peer: p1}) // the hub let go of p1: kick, leave or grace expiry
	turn.waitRevoked(t, "u1")
	send(t, h, Msg{T: "close"})
	turn.waitRevoked(t, "u0", "u1", "u2")

	// A host that never comes back loses its room after the grace period.
	h2, rm2 := host(t, url, 2)
	h2.c.CloseNow()
	turn.waitRevoked(t, "u0", "u1", "u2", "u3")
	j := dial(t, url)
	send(t, j, Msg{T: "join", Code: rm2.Code, Name: "Bo"})
	expect(t, j, "error")
}

// TestPeerIDsAcrossRestarts: a host keeps its joiners' peer ids, and revokes by them, after the server restarts and it
// opens a new room. The new server must never hand out one of those ids again.
func TestPeerIDsAcrossRestarts(t *testing.T) {
	seen := map[string]bool{}
	for range 2 { // a restarted server is a new Server
		_, url := setup(t, testPassword)
		h, rm := host(t, url, 2)
		_, p := joinRoom(t, url, h, rm.Code)
		if seen[p] {
			t.Fatalf("peer id %q handed out again after a restart", p)
		}
		seen[p] = true
	}
}

func TestReclaim(t *testing.T) {
	_, url := setup(t, testPassword)
	h, rm := host(t, url, 2)
	h.c.CloseNow()
	j := dial(t, url) // joins while the host is away: asked once the host is back
	send(t, j, Msg{T: "join", Code: rm.Code, Name: "Bo"})
	expect(t, j, "wait")

	thief := dial(t, url)
	send(t, thief, Msg{T: "host.reclaim", Code: rm.Code, Secret: "nope"})
	expect(t, thief, "error")

	back := dial(t, url)
	send(t, back, Msg{T: "host.reclaim", Code: rm.Code, Secret: rm.Secret})
	expect(t, back, "room")
	if m := expect(t, back, "request"); m.Name != "Bo" {
		t.Fatalf("request %+v", m)
	}
}

func TestPendingCapAndPublicList(t *testing.T) {
	_, url := setup(t, testPassword)
	h, rm := host(t, url, 6)
	for i := 0; i < maxPending; i++ {
		joinRoom(t, url, h, rm.Code)
	}
	extra := dial(t, url)
	send(t, extra, Msg{T: "join", Code: rm.Code, Name: "Bo"})
	expect(t, extra, "error")

	// A private room isn't listed; once the host makes it public, anyone sees it, with its code.
	looker := dial(t, url)
	send(t, looker, Msg{T: "list"})
	if m := expect(t, looker, "rooms"); len(m.Rooms) != 0 {
		t.Fatalf("a private room was listed: %+v", m.Rooms)
	}
	send(t, h, Msg{T: "update", Public: true, Humans: 3, Listed: &Listed{Code: "FAKE00", Cols: 9, Rows: 13, Open: 2}})
	send(t, h, Msg{T: "ping"}) // the update has landed once the pong comes back
	expect(t, h, "pong")
	send(t, looker, Msg{T: "list"})
	if m := expect(t, looker, "rooms"); len(m.Rooms) != 1 ||
		m.Rooms[0] != (Listed{Code: rm.Code, Cols: 9, Rows: 13, Open: 2}) {
		t.Fatalf("listed %+v", m.Rooms)
	}
	// A rejoin carries its hub token, which the host checks.
	back := dial(t, url)
	send(t, back, Msg{T: "rejoin", Code: rm.Code, Name: "Bo", Token: strings.Repeat("ab", tokenLen)})
	expect(t, back, "error") // the approval queue is still full

	bad := Msg{T: "update", Humans: 3, Listed: &Listed{Cols: 99, Rows: 9}}
	send(t, h, bad)
	if _, err := h.Receive(context.Background()); err == nil {
		t.Fatal("a host describing an impossible room must be closed")
	}
}

func TestCredentialLimit(t *testing.T) {
	_, url := setup(t, testPassword)
	h, rm := host(t, url, 1) // the host plus rejoinAllowance joiners at most
	for i := 0; i < rejoinAllowance; i++ {
		j, p := joinRoom(t, url, h, rm.Code)
		approve(t, h, j, p)
	}
	j, p := joinRoom(t, url, h, rm.Code)
	send(t, h, Msg{T: "approve", Peer: p})
	if m := expect(t, h, "error"); m.Peer != p || m.Error != "limit" {
		t.Fatalf("over the limit: %+v", m)
	}
	expect(t, j, "error")
}

// raw opens a socket without the client's handshake and returns it with the server's challenge.
func raw(t *testing.T, url string) (*websocket.Conn, Msg) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	_, b, err := c.Read(ctx)
	var ch Msg
	if err != nil || json.Unmarshal(b, &ch) != nil || ch.T != "challenge" {
		t.Fatalf("challenge: %s %v", b, err)
	}
	return c, ch
}

// closes reports whether the server closes c after the given messages, before answering anything.
func closes(t *testing.T, c *websocket.Conn, msgs ...Msg) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, m := range msgs {
		b, _ := json.Marshal(m)
		if err := c.Write(ctx, websocket.MessageText, b); err != nil {
			return true
		}
	}
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			return ctx.Err() == nil
		}
		if !strings.Contains(string(b), `"error"`) {
			return false
		}
	}
}

func TestMessageOrder(t *testing.T) {
	_, url := setup(t, testPassword)
	hello := Msg{T: "hello", V: Version}
	answer := func(ch Msg) Msg { return Msg{T: "pow", C: ch.C, N: pow.Solve(ch.C, ch.Bits)} }
	for name, msgs := range map[string]func(ch Msg) []Msg{
		"create before hello": func(Msg) []Msg { return []Msg{{T: "host.create", Password: testPassword, Humans: 2}} },
		"join before pow":     func(Msg) []Msg { return []Msg{hello, {T: "join", Code: "AAAAAA", Name: "x"}} },
		"other version":       func(Msg) []Msg { return []Msg{{T: "hello", V: Version + 1}} },
		"unknown type":        func(ch Msg) []Msg { return []Msg{hello, answer(ch), {T: "dance"}} },
		"hello twice":         func(ch Msg) []Msg { return []Msg{hello, hello} },
		"joiner approving":    func(ch Msg) []Msg { return []Msg{hello, answer(ch), {T: "approve", Peer: "p1"}} },
		"forged challenge": func(Msg) []Msg {
			c := pow.NewIssuer().Issue(0, time.Minute, time.Now())
			return []Msg{hello, {T: "pow", C: c}}
		},
		"no name":              func(ch Msg) []Msg { return []Msg{hello, answer(ch), {T: "join", Code: "AAAAAA"}} },
		"rejoin without token": func(ch Msg) []Msg { return []Msg{hello, answer(ch), {T: "rejoin", Code: "AAAAAA", Name: "x"}} },
		// One saved from a quiet moment would skip the work a flood has raised.
		"another socket's challenge": func(Msg) []Msg { _, other := raw(t, url); return []Msg{hello, answer(other)} },
	} {
		c, ch := raw(t, url)
		if !closes(t, c, msgs(ch)...) {
			t.Errorf("%s: socket stayed open", name)
		}
	}

	// A challenge answered on one socket can't be reused on another.
	a, ch := raw(t, url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, m := range []Msg{hello, answer(ch), {T: "join", Code: "AAAAAA", Name: "x"}} {
		b, _ := json.Marshal(m)
		if err := a.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	if _, b, err := a.Read(ctx); err != nil || !strings.Contains(string(b), `"refused"`) {
		t.Fatalf("first use: %s %v", b, err) // the answer counted: the join was looked at
	}
	b, _ := raw(t, url)
	if !closes(t, b, hello, answer(ch)) {
		t.Error("reused challenge accepted")
	}
}

func TestDialOtherVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		b, _ := json.Marshal(Msg{T: "challenge", V: Version + 1, C: "x"})
		_ = c.Write(r.Context(), websocket.MessageText, b)
		_, _, _ = c.Read(r.Context()) // until the client hangs up
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")); !errors.Is(err, ErrVersion) {
		t.Fatalf("Dial: %v, want ErrVersion", err)
	}
}

func TestSocketLimits(t *testing.T) {
	_, url := setup(t, testPassword)
	// A browser from another site is refused; native clients send no Origin.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: %v %v", resp, err)
	}
	ok, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://orbitrix.example"}}})
	if err != nil {
		t.Fatalf("own origin: %v", err)
	}
	ok.CloseNow()

	var open []*websocket.Conn
	defer func() {
		for _, c := range open {
			c.CloseNow()
		}
	}()
	for len(open) < maxSocketsPerIP {
		c, _, err := websocket.Dial(ctx, url, nil)
		if err != nil {
			t.Fatalf("socket %d: %v", len(open), err)
		}
		open = append(open, c)
	}
	_, resp, err = websocket.Dial(ctx, url, nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("over the per-IP cap: %v %v", resp, err)
	}
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct{ remote, cf, want string }{
		{"203.0.113.9:4000", "", "203.0.113.9"},
		{"203.0.113.9:4000", "198.51.100.1", "203.0.113.9"}, // only a proxy on a private address is believed
		{"172.18.0.3:4000", "198.51.100.1", "198.51.100.1"},
		{"127.0.0.1:4000", "2001:db8:1:2:3:4:5:6", "2001:db8:1:2::/64"},
		{"[::ffff:203.0.113.9]:4000", "", "203.0.113.9"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/signal", nil)
		r.RemoteAddr = tc.remote
		if tc.cf != "" {
			r.Header.Set("CF-Connecting-IP", tc.cf)
		}
		if got := clientIP(r); got != tc.want {
			t.Errorf("%s via %q: %s, want %s", tc.remote, tc.cf, got, tc.want)
		}
	}
}

func TestDifficulty(t *testing.T) {
	var l load
	now := time.Unix(1_800_000_000, 0)
	if b := l.add("a", now); b != 0 {
		t.Fatalf("first connection: %d bits", b)
	}
	var b int
	for i := 0; i < 4*freePerIP; i++ {
		b = l.add("flood", now)
	}
	if b != minBits+4 || l.add("calm", now) != 0 {
		t.Fatalf("4× the per-IP allowance: %d bits", b)
	}
	if b := l.add("flood", now.Add(3*loadWindow)); b != 0 {
		t.Fatalf("after the window: %d bits", b)
	}
	if bitsFor(1e9, freeGlobal) != maxBits {
		t.Fatal("difficulty isn't capped")
	}
}
