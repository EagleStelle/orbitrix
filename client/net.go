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
)

// origin is the server this build plays online through, or "" when it can't play online.
var origin = strings.TrimSuffix(platform.Origin(), "/")

// signalURL is the signaling WebSocket on origin.
func signalURL() string { return "ws" + strings.TrimPrefix(origin, "http") + "/signal" }

// joinLink is the address that opens the game with room code filled in.
func joinLink(code string) string { return origin + "/join/" + code }

// onlineName is what this player is called in a room. Players can't name themselves yet, so it is the seat type.
const onlineName = "Human"

// hostAIDelay paces the host's AI seats, so a browser host's thinking leaves it time to draw between moves.
const hostAIDelay = 400 * time.Millisecond

// session is one online room as this player sees it, kept across the lobby, the game and its results. Scenes call
// pump every update, so the connection never backs up.
type session struct {
	host *online.Host // nil for a joiner
	link *online.Link
	code string // the room code a joiner came in with

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

// hostSession opens an online room on the server for the seats Setup chose.
func hostSession(a *app, h *online.Host) *session {
	s := newSession(h.Me)
	s.host = h
	s.link.Send(proto.Hello{Name: onlineName})
	platform.Hosting(true)
	return s
}

// hubConfig is the room the host's Setup describes.
func hubConfig(a *app) hub.Config {
	cfg := hub.Config{Cols: a.set.Cols, Rows: a.set.Rows, AIDelay: hostAIDelay, Budget: aiBudget}
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
			s.seat, s.token = int(m.Seat), m.Token
			if s.host == nil {
				a.set.Rejoin = &rejoin{Code: s.code, Token: hexenc.EncodeToString(m.Token[:])}
				a.set.save()
			}
		case proto.Lobby:
			s.lobby, s.got = m, true
			s.lobbyN++
			// The host is ready whenever the room is in its lobby: when it opens and after every Play again.
			if s.host != nil && !m.Playing && s.seat >= 0 && !m.Seats[s.seat].Ready {
				s.link.Send(proto.Ready{On: true})
			}
		case proto.Snapshot:
			s.snap, s.moves = &m, s.moves[:0]
		case proto.MoveApplied:
			s.moves = append(s.moves, m)
		case proto.Bye:
			s.end = byeText[min(int(m.Reason), len(byeText)-1)]
		}
	}
	if s.end == "" && s.link.Closed() {
		s.end, s.lost = "Connection lost", s.host == nil
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

// leave ends this player's part in the room: a host closes it for everyone, a joiner gives up the seat.
func (s *session) leave(a *app) {
	if s.host != nil {
		s.host.Close()
		platform.Hosting(false)
	} else {
		s.link.Send(proto.Leave{})
		s.link.Close()
	}
	s.forget(a)
}

// forget drops the saved rejoin once the seat is gone for good.
func (s *session) forget(a *app) {
	if s.host == nil && a.set.Rejoin != nil {
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
		if s.host != nil {
			platform.Hosting(false)
		}
		to = newGone(s)
	case !s.got:
	case s.lobby.Playing && !inGame && s.snap != nil:
		if p := newNetPlay(a, s); p != nil {
			to = p
		}
	case !s.lobby.Playing && inGame:
		to = newLobby(s)
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
