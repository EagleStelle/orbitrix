# API

Orbitrix has four interfaces: the website's HTTP routes, the signaling protocol at `/signal`, the binary game protocol between a host and its players, and the UDP announcements of Same Wi-Fi rooms.

## HTTP routes

Served by `server.Site` (`server/static.go`), run by `cmd/orbitrix-server`.

| Method and path | Response |
|---|---|
| `GET /` | The landing page: the wordmark, Play in browser, and a download link for each file present in `-downloads` |
| `GET /play` | The game's loader page, which loads `game.<hash>.wasm` |
| `GET /join/{code}` | The same loader; the game reads the code from the address and opens the join page with it filled in |
| `GET /download/{file}` | `orbitrix.exe`, `orbitrix.apk`, `orbitrix-mac.zip` or `orbitrix-linux.zip` from the downloads folder, cached for 5 minutes; 404 for anything else |
| `GET /signal` | WebSocket upgrade into the signaling service |
| `GET /healthz` | `ok` |
| `GET /<file>` | A file of the web build. Content-hashed names are cached for a year as immutable. The best precompressed copy the browser accepts is sent (`br`, then `gzip`); a client that takes neither gets the `.gz` unzipped |

### Server settings

| Flag or variable | Meaning |
|---|---|
| `-web <dir>` | Serve the web build from this folder (`dist/web` after `go tool mage web`) |
| `-downloads <dir>` | Offer the native clients from this folder |
| `LISTEN` | Listen address, default `:1212` |
| `PUBLIC_ORIGIN` | The site's public address; a browser may open `/signal` only from it. Native clients send no `Origin` |
| `HOST_PASSWORD` | Required to host online when set. Empty opens hosting to anyone, STUN only |
| `CF_TURN_KEY_ID`, `CF_TURN_API_TOKEN` | Cloudflare Realtime TURN key. Used only with a host password of at least 20 characters |

The `http.Server` sets `ReadHeaderTimeout` 10 s, `IdleTimeout` 2 min and `MaxHeaderBytes` 16 KiB. When the socket caps (2,000 in total, 100 per IP) are reached, new `/signal` upgrades get `503` with `Retry-After`.

## Signaling (`/signal`)

Versioned JSON, one message per WebSocket text frame, at most 16 KiB. Every message is an object whose `t` field names it (`server/signal/msg.go`). Not part of the game path: once a joiner's data channel opens, it leaves signaling.

Order is enforced. A message that is unknown or out of order closes the socket.

1. The server sends `challenge{v, c, bits, open}` on connect. `open` means hosting needs no password.
2. The client sends `hello{v}`, then `pow{c, n}`: a nonce `n` whose SHA-256 with the challenge has `bits` leading zero bits. `bits` is 0 under normal load and rises with the anonymous connection rate, up to 18. The first message must arrive within 5 s.
3. The client sends one of:

| Message | Answer | Purpose |
|---|---|---|
| `list` | `rooms{rooms}` | The public rooms: `{code, cols, rows, open, playing}` each |
| `host.create{password, humans, public}` | `room{code, secret, ice}` | Create a room. `secret` reclaims it later; `ice` is the host's ICE server list |
| `host.reclaim{code, secret}` | `room{…}` | Take a room back after the host's socket dropped |
| `join{code, name}` | `wait`, then `approved{ice}` or `error` | Ask to join; the host sees `request{peer, name, token}` |
| `rejoin{code, name, token}` | as `join` | Rejoin a held seat; the host's hub approves the token without asking |

After that, a host may send `update{public, humans, listed}`, `approve{peer}`, `deny{peer}`, `signal{peer, sdp}`, `revoke{peer}`, `ping` and `close`, and receives `request`, `signal{peer, sdp}`, `gone{peer}` and `pong`. A joiner sends and receives `signal{sdp}` once approved. Hosts ping every 20 to 30 s, because Cloudflare closes WebSockets idle for about 100 s.

WebRTC uses no trickle ICE: each side gathers its candidates before sending its description, so a join is one offer and one answer.

### Errors

A wrong code, a full approval queue (3 pending per room) and a denial all answer the same `error` ("refused"), so a code can't be probed. A wrong password waits about 1 s before its error, and a socket closes after 3 failures. Nothing is ever locked out: a correct password or live code always passes. A client on another signaling version gets `signal.ErrVersion` and shows "Update Orbitrix".

### TURN credentials

Issued only to a password-verified host and to joiners that host approved, with a 6-hour lifetime. A room holds at most one live credential per human seat plus 4. The host's `revoke` and the room's close revoke them.

## Game protocol (`proto`)

Binary, on every transport: `ws` on the LAN, a WebRTC data channel online and `loopback` for the host's own player. Each message is a version byte (`proto.Version`, currently 3), a type byte, then fixed big-endian fields. Strings are length-prefixed: names up to 24 bytes, codes up to 8. Encodings are canonical, so whatever decodes re-encodes to the same bytes.

| Type | Direction | Fields |
|---|---|---|
| `Hello` | player to host | `Name`, `Code`, `Token` (zero for a new player) |
| `Ready` | player to host | `On` |
| `Move` | player to host | `C`, `R` |
| `Resync` | player to host | asks for a `Snapshot` |
| `Leave` | player to host | gives the seat up, with no grace period |
| `Welcome` | host to player | `Seat`, `Token` for rejoining |
| `Lobby` | host to player | `Cols`, `Rows`, `TurnTimer`, `Playing`, `Seats[]` (`State`, `AI`, `Ready`, `Name`), optional map definition |
| `MoveApplied` | host to player | `Seq`, `Player`, `C`, `R` (255, 255 for a pass), `Hash` after the move |
| `Snapshot` | host to player | `Seq` and the whole game, including optional map definition, `Durability`, `OpenGates`, and per-cell `Pressed`; `Seq` 0 starts a game |
| `Bye` | host to player | the reason the host closed the connection |

Seat states are AI, Open, Here, Away (dropped, inside the 60 s grace), Gone (left mid-game) and Skipped. Close reasons are Closed, Version, BadCode, Full, Kicked, Replaced and Timeout.

A client applies each `MoveApplied` with `core` and compares the hash. A mismatch, a gap in `Seq` or a move that doesn't fit drops what is queued and sends `Resync`.

Map definitions in `Lobby` and `Snapshot` use the bounded portable map encoding, with dimensions matching the board. Protocol decoding requires its current canonical encoding. `Snapshot` terrain state is validated by `core.Restore`: `Pressed` is a 0/1 value for each cell, set only on switch cells; Hold must agree with occupancy, and open Gate circuits must agree with their controller. The hash includes map terrain/features, relationships, wall durability, `Pressed`, and Gate state. Version 3 replaces the earlier spent-switch state with explicit pressed/released state; peers on another game protocol version must update together. Portable map versions and legacy imports are separate; see [maps.md](maps.md).

The hub's limits on every connection: `Hello` within 5 s, inbound player messages of at most 128 bytes, 20 a second with a burst of 40, and a drop after 64 queued messages. Host-to-player lobby and snapshot messages can be larger because they carry maps; WebSocket messages have a 128 KiB read bound. Late or illegal moves are ignored.

## Same Wi-Fi discovery

A host broadcasts an announcement every second to UDP port 7778 on each private IPv4 network it is on. The packet is `ORBX`, the protocol version, a random 4-byte room id, flags (1 playing, 2 private), open human seats, the hub's port and the host device's name. A room not heard for 3 s is dropped from the list.

The hub listens on each private IPv4 address at port 7777, or the next free port up to 15 more, and serves the game protocol over WebSocket at `ws://<host>:<port>/ws`. A private room's `Hello` must carry its code.

Public LAN room codes are the first six characters of the announcement ID encoded in `proto.CodeAlphabet` using base32. A public code identifies the host among heard announcements; it is not an admission secret. Private codes remain randomly generated and checked by the hub.

`GET /room` returns the same binary announcement with `Cache-Control: no-store`; browsers use it instead of UDP discovery. Native Mage builds also serve the bundled web game at `/`, `/play` and `/join/<CODE>` plus its content-hashed assets. Those pages are marked `data-lan` and join the host over the same origin's `/ws`.

Browser-hosted Same Wi-Fi rooms exchange compressed WebRTC descriptions through `/play#i=<invite>` and `/play#r=<reply>` links. These are browser URL fragments, never HTTP query parameters or signaling requests. The reply must open at the hosting tab's origin and browser profile. Each invite is used once, and these rooms provide no TURN or automatic rejoin.
