//go:build !js

package signal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"orbitrix/core"
	"orbitrix/proto"
	"orbitrix/server/signal/pow"
)

// Waits. Tests shorten the first two through Server's fields.
const (
	failDelay    = time.Second      // before answering a wrong password, room code or host secret
	hostGrace    = 60 * time.Second // a room outlives its host's dropped socket this long
	helloTimeout = 5 * time.Second  // from connecting to the first message
	authTimeout  = 30 * time.Second // from connecting to a correct password or a live code
	pingEvery    = 30 * time.Second
)

const (
	maxRooms        = 50
	maxOpenRooms    = 3 // rooms one IP may hold when hosting is open
	maxSockets      = 2000
	maxSocketsPerIP = 100
	maxFails        = 3 // wrong passwords, codes or secrets before the socket closes
	maxPending      = 3 // joiners waiting for the host's answer, per room
	maxHumans       = 12
	rejoinAllowance = 4 // live credentials a room may hold beyond one per human seat
	minPassword     = 20
	sockRate        = 20 // messages a second per socket, sustained
	sockBurst       = 40
	roomRate        = 50 // messages a second from all of a room's joiners
	roomBurst       = 100
	outQueue        = 32
	challengeTTL    = 30 * time.Second
	tokenLen        = 16 // bytes in a hub rejoin token

	// Proof-of-work difficulty follows the anonymous connection rate over a sliding window, globally and per IP. It
	// starts at minBits once a rate passes its free allowance and rises 2 bits each time the rate doubles, up to
	// maxBits: about 0.04 s of hashing on a desktop and an estimated 2 s for the web build on a phone.
	loadWindow = 10 * time.Second
	freeGlobal = 200 // connections per window
	freePerIP  = 20
	minBits    = 8
	maxBits    = 18
)

// DefaultSTUN is what iceServers holds when no TURN is configured.
var DefaultSTUN = []string{"stun:stun.cloudflare.com:3478"}

// Config is the signaling service's setup, from the server's environment.
type Config struct {
	Password string   // HOST_PASSWORD: empty opens hosting to anyone; under 20 characters keeps TURN off
	Origin   string   // PUBLIC_ORIGIN: a browser's WebSocket must come from it (or from the host it connected to)
	TURN     TURN     // nil gives STUN-only iceServers
	STUN     []string // nil means DefaultSTUN; empty means none (tests on one machine)
	Log      *slog.Logger
}

// Server is the /signal WebSocket endpoint. Its state lives only in memory.
type Server struct {
	cfg     Config
	log     *slog.Logger
	digest  [32]byte
	open    bool // no password: anyone hosts, with STUN only
	stunICE json.RawMessage
	origins []string
	pow     *pow.Issuer
	load    load

	failDelay, hostGrace time.Duration

	mu       sync.Mutex
	rooms    map[string]*room
	sockets  int
	perIP    map[string]int
	nextPeer uint64
}

type state int

const (
	stHello    state = iota // waiting for hello
	stPoW                   // waiting for the proof-of-work answer
	stAuth                  // waiting for host.create, host.reclaim, join or rejoin
	stHost                  // a room's host
	stPending               // a joiner waiting for the host's answer
	stApproved              // an approved joiner exchanging descriptions with the host
)

// sock is one WebSocket. Everything but c and lim is guarded by Server.mu.
type sock struct {
	s      *Server
	c      *websocket.Conn
	ip     string
	lim    *rate.Limiter
	out    chan []byte
	closed bool
	state  state
	fails  int
	room   *room
	id     string // joiners: the peer id the host knows them by
	name   string
	token  string
}

type room struct {
	code, secret string
	hostIP       string
	humans       int
	host         *sock // nil while the host is reconnecting
	gone         *time.Timer
	public       bool   // listed for anyone
	listed       Listed // how the list shows it, as the host last said
	closed       bool
	joiners      map[string]*sock  // pending and approved, by peer id
	creds        map[string]string // live joiner credentials: peer id → TURN username ("" without TURN)
	hostCred     string
	issuing      int // credentials being issued right now
	lim          *rate.Limiter
}

// New returns the service. TURN is used only with a password of at least 20 characters and a TURN key; anything less
// falls back to STUN, which costs nothing, and is logged. Without a password anyone may host.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, log: cfg.Log, pow: pow.NewIssuer(), rooms: map[string]*room{}, perIP: map[string]int{},
		failDelay: failDelay, hostGrace: hostGrace, digest: sha256.Sum256([]byte(cfg.Password))}
	if s.log == nil {
		s.log = slog.Default()
	}
	switch {
	case cfg.Password == "":
		s.open, s.cfg.TURN = true, nil
		s.log.Warn("HOST_PASSWORD is unset: anyone can host, with STUN only")
	case len(cfg.Password) < minPassword:
		s.cfg.TURN = nil
		s.log.Warn("HOST_PASSWORD is shorter than 20 characters: hosting needs it, but with STUN only")
	case cfg.TURN == nil:
		s.log.Info("no TURN key configured: iceServers is STUN only")
	}
	stun, list := cfg.STUN, []iceServer{}
	if stun == nil {
		stun = DefaultSTUN
	}
	if len(stun) > 0 {
		list = append(list, iceServer{URLs: stun})
	}
	s.stunICE, _ = json.Marshal(list) // can't fail
	if u, err := url.Parse(cfg.Origin); err == nil && u.Host != "" {
		s.origins = []string{u.Host}
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.admit(ip) {
		w.Header().Set("Retry-After", "10")
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	defer s.unadmit(ip)
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.origins})
	if err != nil {
		return // Accept has answered, 403 for a foreign Origin
	}
	c.SetReadLimit(MaxMessage)
	k := &sock{s: s, c: c, ip: ip, lim: rate.NewLimiter(sockRate, sockBurst), out: make(chan []byte, outQueue)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go k.write()
	go k.ping(ctx)

	now := time.Now()
	bits := s.load.add(ip, now)
	s.mu.Lock()
	k.send(Msg{T: "challenge", V: Version, C: s.pow.Issue(bits, challengeTTL, now), Bits: bits, Open: s.open})
	s.mu.Unlock()
	k.run(now)
	s.drop(k)
}

// admit counts a new socket, unless the server or this IP is at its cap.
func (s *Server) admit(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sockets >= maxSockets || s.perIP[ip] >= maxSocketsPerIP {
		return false
	}
	s.sockets++
	s.perIP[ip]++
	return true
}

func (s *Server) unadmit(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets--
	if s.perIP[ip]--; s.perIP[ip] == 0 {
		delete(s.perIP, ip)
	}
}

// clientIP is the caller's address. CF-Connecting-IP is believed only from a private or loopback peer: the proxy in
// front of the server, which publishes its port on 127.0.0.1 only. IPv6 callers count by their /64, which one
// subscriber usually holds whole.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if a.IsLoopback() || a.IsPrivate() {
		if cf, err := netip.ParseAddr(r.Header.Get("CF-Connecting-IP")); err == nil {
			a = cf
		}
	}
	if a = a.Unmap(); a.Is6() {
		p, _ := a.Prefix(64)
		return p.String()
	}
	return a.String()
}

// run reads the socket until it closes or breaks a rule. Until a correct password or live code, the socket has
// helloTimeout for its first message and authTimeout for the rest.
func (k *sock) run(start time.Time) {
	for {
		ctx, cancel := context.Background(), context.CancelFunc(func() {})
		if st := k.get(); st == stHello {
			ctx, cancel = context.WithDeadline(ctx, start.Add(helloTimeout))
		} else if st < stHost {
			ctx, cancel = context.WithDeadline(ctx, start.Add(authTimeout))
		}
		typ, b, err := k.c.Read(ctx)
		cancel()
		if err != nil || typ != websocket.MessageText || !k.lim.Allow() {
			return
		}
		var m Msg
		if json.Unmarshal(b, &m) != nil || !k.handle(m) {
			return
		}
	}
}

func (k *sock) get() state {
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	return k.state
}

func (k *sock) set(st state) {
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	k.state = st
}

// handle runs one message and reports whether the socket stays open. A message the state doesn't allow closes it.
func (k *sock) handle(m Msg) bool {
	switch k.get() {
	case stHello:
		if m.T != "hello" {
			return false
		}
		if m.V != Version {
			k.reply(Msg{T: "error", Error: "version"})
			return false
		}
		k.set(stPoW)
		return true
	case stPoW:
		if m.T != "pow" || k.s.pow.Check(m.C, m.N, time.Now()) != nil {
			return false
		}
		k.set(stAuth)
		return true
	case stAuth:
		switch m.T {
		case "host.create":
			return k.create(m)
		case "host.reclaim":
			return k.reclaim(m)
		case "list":
			k.list()
			return true
		case "join", "rejoin":
			return k.join(m)
		}
	case stHost:
		if m.T == "approve" {
			return k.approve(m.Peer)
		}
		return k.hostMsg(m)
	case stPending, stApproved:
		return k.joinerMsg(m)
	}
	return false
}

// fail answers a wrong password, code or secret after failDelay, which slows only this socket, and closes the
// socket on the third.
func (k *sock) fail() bool {
	time.Sleep(k.s.failDelay)
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	k.fails++
	k.send(Msg{T: "error", Error: "refused"})
	return k.fails < maxFails
}

func (k *sock) create(m Msg) bool {
	s := k.s
	if !s.passwordOK(m.Password) {
		return k.fail()
	}
	if m.Humans < 1 || m.Humans > maxHumans {
		return false
	}
	s.mu.Lock()
	if len(s.rooms) >= maxRooms || s.open && s.roomsFrom(k.ip) >= maxOpenRooms {
		k.send(Msg{T: "error", Error: "busy"})
		s.mu.Unlock()
		return false
	}
	code := normCode(m.Code)
	if len(code) != proto.CodeLen || strings.IndexFunc(code, func(r rune) bool { return !strings.ContainsRune(proto.CodeAlphabet, r) }) >= 0 || s.rooms[code] != nil {
		code = s.newCode()
	}
	rm := &room{code: code, secret: random(32), hostIP: k.ip, humans: m.Humans, public: m.Public, host: k,
		joiners: map[string]*sock{},
		creds:   map[string]string{}, lim: rate.NewLimiter(roomRate, roomBurst), issuing: 1}
	s.rooms[rm.code] = rm
	k.room, k.state = rm, stHost
	s.mu.Unlock()

	user, ice := s.issue()
	s.mu.Lock()
	defer s.mu.Unlock()
	rm.issuing--
	rm.hostCred = user
	if rm.closed { // the host closed it while the credential was on its way
		s.revoke(user)
		return false
	}
	k.send(Msg{T: "room", Code: rm.code, Secret: rm.secret, ICE: ice})
	s.log.Info("room opened", "rooms", len(s.rooms))
	return true
}

func (s *Server) passwordOK(pw string) bool {
	d := sha256.Sum256([]byte(pw))
	return s.open || subtle.ConstantTimeCompare(d[:], s.digest[:]) == 1
}

// roomsFrom counts the rooms hosted from ip. Server.mu must be held.
func (s *Server) roomsFrom(ip string) int {
	n := 0
	for _, rm := range s.rooms {
		if rm.hostIP == ip {
			n++
		}
	}
	return n
}

// reclaim gives a host back its room after its socket dropped. Joiners still waiting are asked about again.
func (k *sock) reclaim(m Msg) bool {
	s := k.s
	s.mu.Lock()
	rm := s.rooms[normCode(m.Code)]
	ok := rm != nil && subtle.ConstantTimeCompare([]byte(m.Secret), []byte(rm.secret)) == 1
	if ok {
		if old := rm.host; old != nil {
			old.end("") // a socket that hasn't noticed it is dead yet
		}
		if rm.gone != nil {
			rm.gone.Stop()
		}
		rm.host, k.room, k.state = k, rm, stHost
		k.send(Msg{T: "room", Code: rm.code, Secret: rm.secret})
		for id, j := range rm.joiners {
			if j.state == stPending {
				k.send(Msg{T: "request", Peer: id, Name: j.name, Token: j.token})
			}
		}
	}
	s.mu.Unlock()
	if !ok {
		return k.fail()
	}
	return true
}

// join asks a room's host to let k in. A rejoin carries its hub token, so the host approves it without asking. A
// wrong code and a full queue look the same to the caller.
func (k *sock) join(m Msg) bool {
	rejoin := m.T == "rejoin"
	if !validName(m.Name) || rejoin != (m.Token != "") || rejoin && !validToken(m.Token) {
		return false
	}
	s := k.s
	s.mu.Lock()
	rm := s.rooms[normCode(m.Code)]
	ok := rm != nil && rm.pending() < maxPending
	if ok {
		s.nextPeer++
		k.id, k.name, k.token = "p"+strconv.FormatUint(s.nextPeer, 36), m.Name, m.Token
		k.room, k.state = rm, stPending
		rm.joiners[k.id] = k
		k.send(Msg{T: "wait"})
		if rm.host != nil {
			rm.host.send(Msg{T: "request", Peer: k.id, Name: k.name, Token: k.token})
		}
	}
	s.mu.Unlock()
	if !ok {
		return k.fail()
	}
	return true
}

// list answers with the public rooms whose host is connected and has described them, in code order. Joining one is
// a join like any other: the host still answers it.
func (k *sock) list() {
	s := k.s
	s.mu.Lock()
	defer s.mu.Unlock()
	var rooms []Listed
	for _, rm := range s.rooms {
		if rm.public && rm.host != nil && !rm.closed && rm.listed.Cols != 0 {
			rooms = append(rooms, rm.listed)
		}
	}
	slices.SortFunc(rooms, func(a, b Listed) int { return strings.Compare(a.Code, b.Code) })
	k.send(Msg{T: "rooms", Rooms: rooms})
}

func (rm *room) pending() int {
	n := 0
	for _, j := range rm.joiners {
		if j.state == stPending {
			n++
		}
	}
	return n
}

// approve issues the joiner its credential and iceServers. Only now may descriptions pass between them.
func (k *sock) approve(id string) bool {
	s := k.s
	s.mu.Lock()
	rm := k.room
	j := rm.joiners[id]
	if j == nil || j.state != stPending {
		s.mu.Unlock()
		return true // already gone or approved
	}
	if 1+len(rm.creds)+rm.issuing >= rm.humans+rejoinAllowance {
		k.send(Msg{T: "error", Peer: id, Error: "limit"})
		s.refuse(j)
		s.mu.Unlock()
		return true
	}
	j.state = stApproved
	rm.issuing++
	s.mu.Unlock()

	user, ice := s.issue()
	s.mu.Lock()
	defer s.mu.Unlock()
	rm.issuing--
	switch {
	case rm.closed || rm.joiners[id] != j: // the room closed or the joiner left meanwhile
		s.revoke(user)
	default:
		rm.creds[id] = user
		j.send(Msg{T: "approved", ICE: ice})
		if rm.host != nil {
			rm.host.send(Msg{T: "approved", Peer: id})
		}
	}
	return true
}

func (k *sock) hostMsg(m Msg) bool {
	s := k.s
	s.mu.Lock()
	defer s.mu.Unlock()
	rm := k.room
	switch m.T {
	case "ping":
		k.send(Msg{T: "pong"})
	case "deny":
		if j := rm.joiners[m.Peer]; j != nil && j.state == stPending {
			s.refuse(j)
		}
	case "update":
		l := m.Listed
		if m.Humans < 1 || m.Humans > maxHumans || l == nil || l.Cols < core.MinSize || l.Cols > core.MaxSize ||
			l.Rows < core.MinSize || l.Rows > core.MaxSize || l.Open < 0 || l.Open > maxHumans {
			return false
		}
		rm.public, rm.humans, rm.listed = m.Public, m.Humans, *l
		rm.listed.Code = rm.code
	case "signal":
		if j := rm.joiners[m.Peer]; j != nil && j.state == stApproved && m.SDP != "" {
			j.send(Msg{T: "signal", SDP: m.SDP})
		}
	case "revoke": // the host's hub let go of the player: a kick, a leave or the end of the seat's grace
		if user, ok := rm.creds[m.Peer]; ok {
			delete(rm.creds, m.Peer)
			s.revoke(user)
		}
	case "close":
		s.closeRoom(rm)
		return false
	default:
		return false
	}
	return true
}

func (k *sock) joinerMsg(m Msg) bool {
	s := k.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if !k.room.lim.Allow() {
		return false
	}
	switch {
	case m.T == "ping":
		k.send(Msg{T: "pong"})
	case m.T == "signal" && k.state == stApproved && m.SDP != "":
		if h := k.room.host; h != nil {
			h.send(Msg{T: "signal", Peer: k.id, SDP: m.SDP})
		}
	default:
		return false
	}
	return true
}

// refuse turns a joiner away with nothing but a generic error.
func (s *Server) refuse(j *sock) {
	delete(j.room.joiners, j.id)
	j.end("refused")
}

// drop forgets a closed socket. A host's room waits hostGrace for it to reclaim; a joiner's host hears it is gone.
// An approved joiner's credential stays: it now carries the joiner's game until the host revokes it.
func (s *Server) drop(k *sock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k.end("")
	rm := k.room
	if rm == nil || rm.closed {
		return
	}
	if rm.host == k {
		rm.host = nil
		rm.gone = time.AfterFunc(s.hostGrace, func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if rm.host == nil {
				s.closeRoom(rm)
			}
		})
		return
	}
	if rm.joiners[k.id] == k {
		delete(rm.joiners, k.id)
		if rm.host != nil {
			rm.host.send(Msg{T: "gone", Peer: k.id})
		}
	}
}

// closeRoom ends a room and revokes every credential it holds.
func (s *Server) closeRoom(rm *room) {
	if rm.closed {
		return
	}
	rm.closed = true
	delete(s.rooms, rm.code)
	if rm.gone != nil {
		rm.gone.Stop()
	}
	for _, j := range rm.joiners {
		j.end("closed")
	}
	s.revoke(rm.hostCred)
	for _, user := range rm.creds {
		s.revoke(user)
	}
	s.log.Info("room closed", "rooms", len(s.rooms))
}

// issue returns a new TURN credential and its iceServers. Without TURN, or when Cloudflare fails, it falls back to the
// STUN-only list, so a room never fails for want of a relay.
func (s *Server) issue() (user string, ice json.RawMessage) {
	if s.cfg.TURN == nil {
		return "", s.stunICE
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := s.cfg.TURN.Issue(ctx)
	if err != nil {
		s.log.Warn("TURN credential failed: falling back to STUN", "err", err) // the error never names a credential
		return "", s.stunICE
	}
	return c.Username, c.ICEServers
}

// revoke revokes a TURN credential in the background.
func (s *Server) revoke(user string) {
	if user == "" || s.cfg.TURN == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.cfg.TURN.Revoke(ctx, user); err != nil {
			s.log.Warn("TURN revoke failed", "err", err) // the error never names the credential
		}
	}()
}

// send queues m for the socket; Server.mu must be held. A socket that can't keep up is closed.
func (k *sock) send(m Msg) {
	if k.closed {
		return
	}
	b, _ := json.Marshal(m) // can't fail
	select {
	case k.out <- b:
	default:
		k.closed = true
		close(k.out)
	}
}

// reply is send for a caller that doesn't hold Server.mu.
func (k *sock) reply(m Msg) {
	k.s.mu.Lock()
	defer k.s.mu.Unlock()
	k.send(m)
}

// end closes the socket after its queue, and errMsg if given, are sent. Server.mu must be held.
func (k *sock) end(errMsg string) {
	if errMsg != "" {
		k.send(Msg{T: "error", Error: errMsg})
	}
	if !k.closed {
		k.closed = true
		close(k.out)
	}
}

func (k *sock) write() {
	for b := range k.out {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := k.c.Write(ctx, websocket.MessageText, b)
		cancel()
		if err != nil {
			k.c.CloseNow()
			for range k.out { // until end closes the queue
			}
			return
		}
	}
	k.c.Close(websocket.StatusNormalClosure, "")
}

// ping checks that the other end still answers. Cloudflare drops idle WebSockets after about 100 s; hosts also send
// their own pings, which browsers can do where WebSocket pings can't be sent.
func (k *sock) ping(ctx context.Context) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := k.c.Ping(pctx)
		cancel()
		if err != nil {
			k.c.CloseNow()
			return
		}
	}
}

// newCode returns an unused room code. Server.mu must be held.
func (s *Server) newCode() string {
	for {
		if code := proto.NewCode(); s.rooms[code] == nil {
			return code
		}
	}
}

func normCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails
	return base64.RawURLEncoding.EncodeToString(b)
}

func validName(name string) bool {
	if name == "" || len(name) > 24 || !utf8.ValidString(name) {
		return false
	}
	for _, c := range name {
		if !unicode.IsPrint(c) {
			return false
		}
	}
	return true
}

func validToken(t string) bool {
	b, err := hex.DecodeString(t)
	return err == nil && len(b) == tokenLen
}

// load counts connections for the proof-of-work difficulty.
type load struct {
	mu           sync.Mutex
	start        time.Time // of the current window
	cur, prev    int
	curIP, prev2 map[string]int
}

// add counts a connection from ip and returns the difficulty its challenge gets.
func (l *load) add(ip string, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if el := now.Sub(l.start); el >= 2*loadWindow || l.curIP == nil {
		l.start, l.cur, l.prev, l.curIP, l.prev2 = now, 0, 0, map[string]int{}, nil
	} else if el >= loadWindow {
		l.start, l.cur, l.prev, l.curIP, l.prev2 = l.start.Add(loadWindow), 0, l.cur, map[string]int{}, l.curIP
	}
	l.cur++
	l.curIP[ip]++
	// The previous window counts for the part of it still inside the sliding window.
	f := 1 - float64(now.Sub(l.start))/float64(loadWindow)
	global := float64(l.cur) + f*float64(l.prev)
	perIP := float64(l.curIP[ip]) + f*float64(l.prev2[ip])
	return max(bitsFor(global, freeGlobal), bitsFor(perIP, freePerIP))
}

func bitsFor(n, free float64) int {
	if n <= free {
		return 0
	}
	return min(maxBits, minBits+int(2*math.Log2(n/free)))
}
