package client

import (
	hexenc "encoding/hex"
	"strings"
	"time"

	"orbitrix/client/online"
	"orbitrix/client/platform"
	"orbitrix/core/ai"
	"orbitrix/proto"
	"orbitrix/server/hub"
	"orbitrix/server/lan"
	"orbitrix/server/signal"
)

// origin is the server this build plays online through, or "" when it can't play online.
var origin = strings.TrimSuffix(platform.Origin(), "/")

// signalURL is the signaling WebSocket on origin.
func signalURL() string { return "ws" + strings.TrimPrefix(origin, "http") + "/signal" }

// joinLink is the address that opens the game with room code filled in.
func joinLink(code string) string { return origin + "/join/" + code }

// hostAIDelay paces the host's AI seats, so a browser host's thinking leaves it time to draw between moves.
const hostAIDelay = 400 * time.Millisecond

// session is one online or LAN room as this player sees it, kept across the lobby, the game and its results. Scenes
// call pump every update, so the connection never backs up.
type session struct {
	room    *hub.Room      // the room this player hosts, or nil for a joiner
	host    *online.Host   // an online host's signaling, or nil
	lan     *lan.Host      // a LAN host's listener, or nil
	direct  *online.Direct // a server-free room's invites, for its host, or nil
	invited bool           // this player joined a server-free room by its invite
	link    *online.Link
	code    string     // the room's code: the one a joiner came in with, or a private LAN room's
	addr    string     // a LAN room's host:port, or "" online
	private bool       // the room this player hosts takes its code (LAN) or isn't listed (online)
	desc    signal.Msg // what describe last told signaling

	seat   int // -1 until Welcome
	token  proto.Token
	lobby  proto.Lobby
	got    bool                // a Lobby has arrived
	lobbyN int                 // room updates so far, so the game notices a seat change
	snap   *proto.Snapshot     // a game start, rejoin or resync not yet taken up by the game scene
	moves  []proto.MoveApplied // moves not yet played on this player's board
	end    string              // why the room ended for this player, or ""
	lost   bool                // the connection dropped without a word from the host: a joiner may rejoin
}

func newSession(link *online.Link) *session { return &session{link: link, seat: -1} }

// hostSession seats this player, called name, in the room they host, through me, the hub's in-process connection.
func hostSession(room *hub.Room, me *online.Link, name string) *session {
	s := newSession(me)
	s.room = room
	s.link.Send(proto.Hello{Name: name})
	platform.Hosting(true)
	return s
}

// lanSession is the LAN room at addr this player joined, through c, with the host's Welcome; code is a private
// room's, kept for a rejoin.
func lanSession(a *app, c hub.Conn, w proto.Welcome, addr, code string) *session {
	s := newSession(online.NewLink(c))
	s.addr, s.code = addr, code
	s.welcome(a, w)
	return s
}

// hubConfig is the room the host's Setup describes.
func hubConfig(a *app) hub.Config {
	cfg := hub.Config{Cols: a.set.Cols, Rows: a.set.Rows, Map: a.set.Map, AIDelay: hostAIDelay, Budget: aiBudget}
	for _, opt := range a.set.seats {
		cfg.Seats = append(cfg.Seats, hub.Seat{Human: opt == seatHuman, AI: ai.Strategy(max(0, opt-1))})
	}
	return cfg
}

// pump takes in everything the host has sent.
func (s *session) pump(a *app) {
	for {
		m, ok := s.link.Poll()
		if !ok {
			break
		}
		switch m := m.(type) {
		case proto.Welcome:
			s.welcome(a, m)
		case proto.Lobby:
			s.lobby, s.got = m, true
			s.lobbyN++
			s.describe()
			// The host is ready whenever the room is in its lobby: when it opens and after every Play again.
			if s.room != nil && !m.Playing && s.seat >= 0 && !m.Seats[s.seat].Ready {
				s.link.Send(proto.Ready{On: true})
			}
		case proto.Snapshot:
			s.snap, s.moves = &m, s.moves[:0]
		case proto.MoveApplied:
			s.moves = append(s.moves, m)
		case proto.Bye:
			s.end = byeWhy(m.Reason)
		}
	}
	if s.end == "" && s.link.Closed() {
		s.end, s.lost = "Connection lost", s.room == nil && !s.invited // an invite is used up: no way back in
		if s.lost && s.addr != "" {
			s.end = "Host disconnected" // a LAN host whose device slept or left the app; the joiner retries
		}
	}
}

// describe tells signaling how the online room this player hosts stands: whether it is listed, its human seats (which
// bound its TURN credentials) and how the list shows it. It sends only what changed.
func (s *session) describe() {
	if s.host == nil || !s.got {
		return
	}
	l := signal.Listed{Cols: int(s.lobby.Cols), Rows: int(s.lobby.Rows), Playing: s.lobby.Playing}
	humans := 0
	for _, st := range s.lobby.Seats {
		if st.State != proto.SeatAI {
			humans++
		}
		if st.State == proto.SeatOpen {
			l.Open++
		}
	}
	m := signal.Msg{Public: !s.private, Humans: max(1, humans), Listed: &l}
	if s.desc.Listed != nil && *s.desc.Listed == l && s.desc.Public == m.Public && s.desc.Humans == m.Humans {
		return
	}
	s.desc = m
	s.host.Update(m.Public, m.Humans, l)
}

// wifi reports a Same Wi-Fi room: on the LAN, or server-free.
func (s *session) wifi() bool { return s.addr != "" || s.direct != nil || s.invited }

// welcome takes the seat the host gave this player. A joiner remembers it, to rejoin after the app closes; a
// server-free room's joiner can't, needing a new invite.
func (s *session) welcome(a *app, m proto.Welcome) {
	s.seat, s.token = int(m.Seat), m.Token
	if s.room == nil && !s.invited {
		a.set.Rejoin = &rejoin{Code: s.code, Addr: s.addr, Token: hexenc.EncodeToString(m.Token[:])}
		a.set.save()
	}
}

// byeText says why the host let this player go, by proto.Reason.
var byeText = [...]string{
	proto.ReasonClosed:     "The host closed the room",
	proto.ReasonVersion:    "Update Orbitrix to join this room",
	proto.ReasonBadCode:    "No room with that code",
	proto.ReasonFull:       "The room is full",
	proto.ReasonKicked:     "The host removed you",
	proto.ReasonReplaced:   "You joined from another device",
	proto.ReasonTimeout:    "Connection lost",
	proto.ReasonBadMessage: "Connection lost",
	proto.ReasonRateLimit:  "Connection lost",
}

// byeWhy is byeText for any reason, one from a newer host included.
func byeWhy(r proto.Reason) string { return byeText[min(int(r), len(byeText)-1)] }

// leave ends this player's part in the room: a host closes it for everyone, a joiner gives up the seat.
func (s *session) leave(a *app) {
	switch {
	case s.host != nil:
		s.host.Close()
	case s.lan != nil:
		s.lan.Close()
	case s.direct != nil:
		s.direct.Close()
	default:
		s.link.Send(proto.Leave{})
		s.link.Close()
	}
	if s.room != nil {
		platform.Hosting(false)
	}
	s.forget(a)
}

// forget drops the saved rejoin once the seat is gone for good.
func (s *session) forget(a *app) {
	if s.room == nil && a.set.Rejoin != nil {
		a.set.Rejoin = nil
		a.set.save()
	}
}

// route moves the player to the scene the room is in: the game once it starts, the lobby when it ends, and the
// room's end when the connection does. It reports whether the scene changes.
func (s *session) route(a *app, from scene) bool {
	var to scene
	_, inGame := from.(*playScene)
	switch {
	case s.end != "":
		if !s.lost {
			s.forget(a)
		}
		if s.room != nil {
			platform.Hosting(false)
		}
		to = newGone(s)
	case !s.got:
	case s.lobby.Playing && !inGame && s.snap != nil:
		if p := newNetPlay(a, s); p != nil {
			to = p
		}
	case !s.lobby.Playing && inGame:
		to = newRoom(s)
	}
	if to == nil {
		return false
	}
	a.next, a.net = to, s
	if s.end != "" {
		a.net = nil
	}
	return true
}

// seatLabel is what seat i is called: the player's name, or the seat's type.
func (s *session) seatLabel(i int) string {
	st := s.lobby.Seats[i]
	switch {
	case st.State == proto.SeatAI:
		return seatLabels[1+int(st.AI)]
	case st.Name != "":
		return st.Name
	}
	return seatLabels[seatHuman]
}

// seats is the room's seats as Setup options, for the game scene.
func (s *session) seats() []int {
	out := make([]int, len(s.lobby.Seats))
	for i, st := range s.lobby.Seats {
		if st.State == proto.SeatAI {
			out[i] = 1 + int(st.AI)
		}
	}
	return out
}

// labels is what the game's seat table calls each seat, this player's own marked.
func (s *session) labels() []string {
	out := make([]string, len(s.lobby.Seats))
	for i := range out {
		out[i] = s.seatLabel(i)
		if i == s.seat {
			out[i] += " (you)"
		}
	}
	return out
}
