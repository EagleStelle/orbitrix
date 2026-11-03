// Package signal is the signaling service at /signal: room codes, join approval, the SDP relay and TURN credentials.
// No game traffic passes through it. Messages are versioned JSON, one per WebSocket text frame; every client walks
// the same order: hello, the proof-of-work answer, then host.create, host.reclaim, join or rejoin.
package signal

import "encoding/json"

// Version is the signaling protocol version a client sends in hello.
const Version = 1

// MaxMessage is the largest frame either side accepts. It fits an SDP with its ICE candidates.
const MaxMessage = 16 << 10

// Msg is every signaling message: T names it and the other fields are those it uses.
//
// Client to server: hello{v}, pow{c, n}, host.create{password, humans}, host.reclaim{code, secret},
// join{code, name}, rejoin{code, name, token}, ping; a host's approve{peer}, deny{peer}, lock{on},
// signal{peer, sdp}, revoke{peer}, close; a joiner's signal{sdp}.
//
// Server to client: challenge{v, c, bits, open}, room{code, secret, ice}, wait, approved{ice} (to the joiner) or
// approved{peer} (to the host), request{peer, name, token}, signal{sdp[, peer]}, gone{peer}, pong,
// error{error[, peer]}.
type Msg struct {
	T        string          `json:"t"`
	V        int             `json:"v,omitempty"`
	C        string          `json:"c,omitempty"`    // proof-of-work challenge
	N        uint64          `json:"n,omitempty"`    // its answer
	Bits     int             `json:"bits,omitempty"` // its difficulty
	Password string          `json:"password,omitempty"`
	Humans   int             `json:"humans,omitempty"` // human seats in the room, the host's included
	Code     string          `json:"code,omitempty"`
	Secret   string          `json:"secret,omitempty"` // the host's key to reclaim its room
	Name     string          `json:"name,omitempty"`
	Token    string          `json:"token,omitempty"` // a rejoin token from the host's hub, hex
	Peer     string          `json:"peer,omitempty"`  // a joiner, as the host sees it
	On       bool            `json:"on,omitempty"`
	Open     bool            `json:"open,omitempty"` // in the challenge: hosting needs no password (STUN only)
	SDP      string          `json:"sdp,omitempty"`
	ICE      json.RawMessage `json:"ice,omitempty"` // RTCIceServer list
	Error    string          `json:"error,omitempty"`
}
