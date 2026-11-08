# Architecture

Orbitrix is one Go module that builds four things: the game client (desktop, Android and WebAssembly from the same code), the room engine every host runs inside its client, the website and signaling server, and a few command-line tools. The rules sit at the bottom and import nothing platform-specific, so every client and every host resolves a move to the same bits.

```
             client (Ebitengine)            cmd/orbitrix-server
      scenes · board · audio · settings     website · /signal
             │              │                      │
        core/ai          client/online ──────── server/signal
             │              │      (SDP, TURN)
             └──── core ◄── server/hub ◄── server/transport/{loopback, ws, rtc}
                    ▲            ▲
                    │          proto (binary wire format)
               server/lan (UDP discovery, ws host)
```

## Components

### `core`: the rules

- A board is `cols × rows` cells in flat arrays: `[]uint8` counts and `[]int8` owners, indexed `c*rows + r`. `Geometry` precomputes each cell's capacity and neighbours, so the hot loops have no bounds logic and no maps.
- Custom map definitions are copied on entry and retained as immutable designs. Geometry separates outgoing degree
  from capacity and supports a fifth connection for Links; Reservoirs add one reserve to capacity. Each circuit has
  any number and mixture of Latch, Toggle and Hold controlling Gates and whole-cell Doors. `CircuitKind` is separate
  from passive `Feature`; `core/circuit.go` owns circuit inputs. `Pressed` records individual switches; `CircuitOn`
  records independent polarity. Outputs reverse their individual starting states with that polarity. Direct placement applies switch effects before the first exploding cells are
  selected. Each wave transfers on its starting topology, then aggregates wall damage and switch effects before
  rebuilding connections/capacities and selecting the next wave. Gate opening, Gate closure, and destruction install
  replacement topology safely isolated from AI copies. Terrain events record placement at wave −1 and subsequent
  changes at their wave boundaries. See [maps.md](maps.md).
- `Apply(cell)` places an orb and resolves the chain in waves, returning every wave in reused buffers. A wave runs in two passes: every cell at capacity gives, then every neighbour receives, taking the sender's owner from the start of the wave. The result doesn't depend on cell order.
- The win check runs after the placement and after every wave, once every player has had a turn, and stops the chain on the spot. A safety cap of 10,000 waves leaves any cells still over capacity to the next move's first wave.
- `Simulate` and `CopyFrom` give the bots an allocation-free search path on fixed topology; custom maps allocate replacement geometry when terrain changes. `Snapshot` and `Restore` (validated) carry a whole game over the network, including map durability, `Pressed`, and `CircuitOn`. `Hash` (64-bit FNV-1a) includes the map, initial output states and runtime state to detect desync. Closed Doors preserve any trapped orbs until reopening.
- All randomness comes from `core.RNG`, a splitmix64 generator.

Worst-case chains resolve in about 5 µs on 9×9, 56 µs on 20×20 and 440 µs on 40×40.

### `core/ai`: the bots

`ai.Bot.Choose(game, strategy, seed, budget)` returns a move. It is a pure function of its inputs: the budget counts work units (one per simulation or search node, plus one per 512 exploded cells), never time, so a move is the same on a phone and a desktop. One `Bot` per game reuses its scratch memory.

Each strategy has its own file. The seven heuristic bots (Jester to Spy) score candidate moves by simulating them. Voyager runs Monte Carlo tree search, Tactician iterative-deepening alpha-beta, and Hunter alpha-beta with Best-Reply Search and capture quiescence. Prodigy runs PUCT over a small graph network trained by self-play, recomputing only the cells a move changed. Sovereign runs alpha-beta with Best-Reply Search, a transposition table and quiescence over an int16 network whose accumulators update incrementally; integer inference keeps its moves identical on every platform. Both networks are generated Go source, not data files.

All twelve strategies accept custom maps. Sovereign uses the existing Tactician search on them because its trained rectangular pattern table does not cover map neighborhoods or the fifth Link connection. Prodigy refreshes its full network evaluation on custom maps rather than retaining topology-dependent cached features.

`DefaultBudget` (1,200) keeps every bot's worst move around 16 ms on a desktop at 40×40, inside the 150 ms target for a mid-range phone. Sovereign alone gets four times that where it thinks on its own thread. In the browser, where thinking holds up drawing, bots on boards bigger than Floor get a budget scaled down by cell count.

### `client`: the game

An Ebitengine game with one scene at a time: Home, Setup (which is also the host's room page), the Maps library, import/share views, Map Editor, the join page, the host password page, the game and the "room ended" page. Everything is laid out in code from the safe area and a UI scale capped at 2.

- Drawing: shapes are white sprites, nine-sliced and tinted per draw. The board is four batched `DrawTriangles32` calls (tiles, orbs, burst rings, flying orbs) from one atlas, and labels cache their glyphs, so a frame allocates nothing.
- Animation: a move is applied to the game at once, then its waves replay from a copy of the old position. Bursts last about 18 ticks and orbs fly to neighbours over about 14. Orb motion is the prototype's, run at its 80 px cell and scaled to the board.
- Map art: `client/internal/boardview/map.go` draws the same flat terrain, devices, relationship patterns and edges for palette icons, editor canvas, thumbnails and the match board. Callers cache their rendered layers; the shared renderer retains no map-sized image or buffer. Adjacent walls and edge posts join using local neighbor lookups. Editor map rendering is bounded to 48 px per cell and scaled for zoom, so zoom does not allocate an oversized target. Match animation replays placement terrain events before the first explosion and wave events after the corresponding transfers.
- Map editing/storage: editor drafts are cloned, strokes share one undo checkpoint, and recovery is saved separately from the library. Test uses a private Setup/settings copy and returns to the same editor draft. Import validates structure before preview and saves only on an explicit action; gameplay-invalid drafts remain editable. The local library uses `client/platform` storage, while portable files/codes use format 2 with a bounded legacy format-1 import path.
- Bots think on a goroutine against a copy of the game; the 300 to 600 ms delay before a bot's move is presentation only.
- Audio: C major pentatonic notes, C4 to A6, generated at startup as PCM. Links use glass chimes; Latch, Toggle and
  Hold use distinct keyed/reed/mallet voices, with lower release notes for Toggle and Hold. Wall damage uses a woody
  knock; destruction uses a falling three-note crumble. Startup synthesis matches each cue's loudest 50 ms RMS
  window to 0.18 full scale, capped at a 0.65 peak. Orb chords and map cues share burst power equally, so large
  chords cannot bury individual effects. Duplicate cues coalesce, with a global 100 ms minimum gap and at most
  15 playing voices. Rate-limited map cues wait for the next burst; the game update drains final-wave cues too.
- Settings: JSON written through a temporary file, through `client/platform`.
- Client ownership: `scene_*.go` holds screens and orchestration; `client/internal/ui` owns shared presentation primitives and controls, and `client/internal/boardview` owns board drawing and wave replay. Shared UI takes presentation/input values instead of application state. `client/internal/sound` owns synthesis and playback; `client/internal/mapstore` owns map library validation and transactional writes through injected storage functions. See [the client guide](../client/README.md).
- Screens that don't change redraw only on input (and every 30 ticks as a backstop), except on Android, where skipped frames flicker.

### `server/hub`: the room engine

Every multiplayer room, online or on the LAN, is a `hub.Room` inside the host's client. The host is authoritative. The hub talks to players only through `Conn` (send, receive, close), and has no idea which transport is behind it:

- `loopback`: an in-process pipe for the host's own player, who joins like everyone else.
- `ws`: WebSocket, for Same Wi-Fi rooms, with a ping every 10 s.
- `rtc`: a pion WebRTC data channel, ordered and reliable, for online rooms.

The hub runs the lobby (ready, seat changes, Swap and Remove, settings that stay live until Start), the game, bot seats, rejoin tokens with a 60 s seat grace, Kick, Replace and Skip, an optional turn timer, and rematch. It enforces message size and rate limits on every `Conn`.

### `client/online` and `server/signal`: online rooms

`client/online.Host` creates a room through signaling, keeps its socket alive with pings, reconnects and reclaims the room with its secret if the socket drops, and opens a WebRTC connection per approved joiner. `client/online.Join` asks to join, waits for approval, connects, and leaves signaling once the data channel opens.

`server/signal` is a state machine per socket: hello, proof of work, then create, reclaim, list, join or rejoin, then only the messages that state allows. It holds rooms in memory, relays SDP, and creates Cloudflare TURN credentials for a verified host and approved joiners, revoking them when the hub reports a seat or room closing.

### `server/lan`: Same Wi-Fi rooms

`lan.Host` runs the hub behind `ws` on each private IPv4 address and announces the room by UDP broadcast every second. `lan.Find` listens for announcements; `lan.Join` connects to a room it heard. There is no signaling, TURN or approval on the LAN: a public room lets anyone in, a private one needs its code.

Native Mage builds embed the web game's gzip files. The LAN host serves that game at `/`, `/play` and `/join/<code>`, marking its page so browsers join over the same origin's `/ws`. `lan.Watch` polls `/room` instead of listening to UDP. The shared Invite players dialog shows the join link as a QR beside the code, with Copy code and Copy link. Public LAN codes identify announcements; private codes authenticate admission.

`client/online.Direct` hosts browser Same Wi-Fi rooms without signaling. Each invite carries one compressed WebRTC offer; the joiner returns an answer through a QR link opened on the host's device. The loader hands replies to the hosting tab through local storage. These connections use STUN but no TURN; invites are single-use, and disconnected players need new ones.

### `server`: the website

`server.Site` serves the landing page, the game at `/play` and `/join/<code>`, the downloads, the web build's content-hashed files with precompressed copies, and mounts `/signal`. `cmd/orbitrix-server` configures it from flags and environment variables. It carries no game traffic.

## How a move travels

On one device:

1. A tap on a legal cell calls `core.Game.Apply`, which returns the waves and records terrain events for custom maps.
2. The board replays from the old position: placement effects first, then each wave's transfers and terrain effects,
   while the turn display waits for the chain to finish. Link cues start with transfers across the pair; switch and
   wall cues follow the recorded placement or landing events, including effects on the final wave.

In a room:

1. The player's client sends `Move{C, R}` through its `Conn`.
2. The host's hub checks the move is the mover's and legal, applies it with `core`, and broadcasts `MoveApplied{Seq, Player, C, R, Hash}` to every player, the host's own included.
3. Each client applies the move with its own `core`, compares the hash, and animates the waves. A mismatch or a gap sends `Resync`, and the hub answers with a `Snapshot`.
4. Bot seats run in the hub, which waits 400 ms before a bot's move so a browser host can draw between thoughts. Each client still plays bot moves 300 to 600 ms apart.

## Patterns

- Pure core: `core` and `core/ai` import no Ebitengine, net or OS packages, which keeps them deterministic, testable without a window, and identical on every target.
- Transport-agnostic hub: one engine for online and LAN play, behind a three-method `Conn`, so a new transport needs no change to the rules of a room.
- Host authority with client replay: clients recompute ordinary moves from small messages, and a state hash catches any divergence. Lobby and snapshot messages also carry the chosen map and, for snapshots, runtime terrain state; incoming player messages remain bounded to 128 bytes. The binary game protocol remains version 3, already bumped after public release v1.0.0; peers must use the same development build.
- Build tags for platforms: `client/platform` holds the storage, clipboard, keyboard and server-address code for each target, so the rest of the client has no platform branches.
- Budgets instead of clocks: AI cost is counted, which makes bots reproducible in tests and fair across devices.

## Security model

The server's URL is public and TURN is the only metered service, so access is layered:

- Online hosting needs `HOST_PASSWORD` when set, compared as SHA-256 digests in constant time. Every joiner needs the host's approval, public room or not.
- TURN credentials go only to a verified host and approved joiners: one per human seat plus 4, a 6-hour lifetime, revoked on kick, leave, seat-grace expiry and room close.
- Signaling limits by trust: anonymous traffic per IP and per socket, authenticated traffic per room and per socket, so an anonymous flood never drains a real room's budget. A wrong secret only slows its own socket, and a correct one is never blocked.
- An adaptive proof of work slows floods instead of blocking them, with socket caps, message size and rate limits, a 5 s first-message deadline and ping/pong.
- The container publishes its port on `127.0.0.1` only and believes `CF-Connecting-IP` only from private or loopback peers. Cloudflare's edge rules (cache, WAF allowlist, Tor and datacenter block, rate limit) are described in [deployment.md](deployment.md).
- No password, room secret, rejoin token or TURN credential is ever logged, and no secret is built into a client.
