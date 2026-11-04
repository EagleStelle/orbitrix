# Decisions

Choices made where the brief or the prototype left room, and why. The specs themselves live in `CLAUDE.md`,
`DESIGN.md` and `PRODUCT.md`; this file keeps only what they don't say.

## Tooling (Phase 0)

- **Module path `orbitrix`.** Nothing outside the repo imports it.
- **Go 1.27.3 and Ebitengine v2.10.5 pinned** (latest stable on 2026-10-30). Older toolchains switch through
  `GOTOOLCHAIN=auto`; CI reads the version from `go.mod`.
- **Mage for builds** (`magefile.go`, run as `go tool mage`). One set of targets in Go replaced the hand-synced
  `build.sh` and `build.ps1`: Windows has no `make`, the stack is Go only, and Mage has no dependencies and needs no
  install. `GOOS=… go tool mage` can't cross-compile (`go tool` would build Mage itself
  for that OS), so `desktop` always targets Windows, the desktop build that ships. `all` runs every target.
- **CI builds with `CGO_ENABLED=0`.** Only the Android/iOS bindings need cgo.
- **No README** (user request). Docs live in `PRODUCT.md`.

## Rules engine (Phase 1)

- **A move is a cell index** (`c*rows + r`), so the AI loops over plain ints; `Geometry.Index`/`Coords` convert.
- **Players may not outnumber cells.** Nothing explodes in the first round, so an extra player could have no legal
  first move. `NewGame` rejects it and Setup disables Start ("Board too small").
- **The win check runs after the placement and after every wave**, once two players have moved. A win stops the
  chain on the spot, so a finished board can hold cells at or over capacity.
- **A wave applies in two passes:** every exploding cell gives, then every neighbour receives, taking the owner the
  sender had at the start of the wave. The result doesn't depend on cell order, and a cell explodes at most once a
  wave.
- **Wave cap 10,000.** If it is ever hit, the cells left over capacity resolve in the next move's first wave. It never
  hangs and never creates or destroys orbs.
- **Counts stay below 2×capacity**, so `uint8` can't overflow; `Restore` rejects snapshots that break this.
- **RNG is splitmix64** with Lemire's unbiased `Intn`, for the same bits on every platform.
- **State hash is 64-bit FNV-1a** over the whole game state. A test pins the empty 9×9 hash so format changes are
  deliberate.

## AI (Phase 2)

- **Dropdown order, easiest to hardest:** Jester, Mason, Warden, Gambler, Nomad, Raider, Spy, Prodigy, Tactician,
  Voyager, Hunter, Sovereign, by measured win rates (`go run ./cmd/aitourney`, `TestMatches`). Strategy names are the
  API and the saved seat values, so reorders and new bots never break saved settings.
- **`ai.Bot.Choose(game, strategy, seed, budget)` is deterministic.** Keep one `Bot` per game or room and use it
  sequentially (it reuses scratch). Unknown strategies play like Jester.
- **Budget is work units, not time:** one per simulation, search node or MCTS iteration, plus one per 512 exploded
  cells (remainders carry across the move). Prodigy also pays one per 16 cells it recomputes (a full pass costs about
  0.73 µs a cell) and one per 1,024 cells of its whole-board passes (about 10 ns a cell). Sovereign is priced in
  ticks of about 1.2 ns, fitted to counted work (a scanned cell 1, a played move 384 plus one per 16 cells, a
  re-patterned cell 54, an evaluation 384), at 9,728 ticks a unit.
- **`DefaultBudget` 1,200** keeps every bot's worst move at about 15.6 ms on the development desktop (Voyager, late
  40×40), inside the 150 ms phone target with room to spare. Only Sovereign gets 4× (`Strategy.Budget()`, user
  choice) where it thinks on its own thread: 51–59 ms on the desktop, not yet measured on a phone. In the browser,
  where thinking holds up drawing, every bot on a board bigger than Floor gets `DefaultBudget × 234 / cells`
  (`aiBudget`; 175 on 40×40).
- **Terminal outcomes beat positional scores.** Elimination is a loss, players yet to place are alive, and MCTS
  rewards are 1 for the winner and 0 for everyone else.
- **All randomness comes from `core.RNG`.** Stable sorts keep board order where a bot has no random tie-break.
- **Shared survival protection** applies only to a first placement or a position of at most one cell and two orbs, so
  every bot otherwise keeps its own risk appetite. Jester skips it.

### Heuristic bots

The prototype's personalities, reworked where its version was blind or slow:

- **Warden** explodes its own primed corner only when that captures (re-popping corners kept it off its edges), and
  primes defensively only where no enemy neighbour is primed (the enemy would pop it first).
- **Raider** scores every move from its neighbourhood in O(1) and simulates its 16 best explosions. The prototype
  simulated the first 16 in board order, so on big boards it never saw captures away from the top-left corner. Its
  comeback target, distance penalty, gated opening and last-cell retreat were dropped: they didn't win games, or
  survival protection covers them.
- **Spy** stays a heuristic. `openingThreats` marks the cells enemy chains could reach next turn (−2 lead per own orb
  there), and it gains 1 lead per orb of each enemy neighbour it would explode first. It reads at most 8 explosions,
  best lead first. Reply search was rejected because it would make Spy a search bot.
- **Nomad** (user request) is Mason's sparse opposite. It spreads single orbs over open ground, never inside
  `openingThreats`, and races enemy neighbours: 40 per enemy orb won, or per own orb lost. It simulates only its two
  most promising captures.
- **Gambler** moved below Nomad and Raider (2026-11-02) once their reworks overtook it.

### Search and learned bots

New, not from the prototype; they use real search or learning instead of more heuristics.

- **Voyager:** MCTS with UCT 0.25, up to 24 root and 10 deeper moves, and four-ply rollouts that sample four cells a
  ply. The most visited root move wins.
- **Tactician:** iterative-deepening alpha-beta where it maximises and every opponent minimises, with 24 root and 8
  inner candidates and up to 16 plies. The last complete depth answers when work runs out.
- **Hunter:** Tactician plus Best-Reply Search (Schadd & Winands 2011). Between its moves only the strongest reply
  from any one opponent is searched, so the cost doesn't grow with the player count. It adds two plies of capture
  quiescence (four captures a node; deeper blew up on big boards) and weights the leader 3× as its prey.
- **Prodigy:** AlphaZero-style PUCT (exploration 0.4, 32 root and 8 inner children, no first-play urgency) over a
  1,893-weight graph network, which plays every board size, rotation and reflection alike. It first tries an outright
  win with its primed cells, using at most half the budget. Evaluation is incremental: at layer l it recomputes only
  cells within l steps of a change. Weights are generated Go source (`prodigy_weights.go`), not an asset. Training:
  `go test -tags train -run TestTrainProdigy`. No run has yet beaten the kept weights, so the gate saves only on a
  strict improvement.
- **Sovereign** (NNUE-style): iterative-deepening alpha-beta with a transposition table (cleared every move, so a
  move depends only on its position), killer moves, Best-Reply Search and capture quiescence, over an int16 network
  (`sovereign_weights.go`) whose per-player accumulators update incrementally. Nothing in its evaluation is
  hand-written: candidates come from its policy, captures from the rules and leaf values from the network. Integer
  inference gives identical moves on every platform. It is trained supervised, like Stockfish's NNUE, on bot games,
  then on its own search's labels: `go test -tags train -run TestTrainSovereign`. The positions so far are in
  `%TEMP%\orbitrix-sovereign`, where the next run continues. It is the strongest bot in every duel and table measured
  except twelve players on 9×9, where Voyager and Prodigy lead.

## Client (Phase 3)

- **The UI is the Party Mat design** (`DESIGN.md`), not the prototype's look, which the user rejected as bland. The
  user picks every visual choice from rendered samples.
- **Menus work like most games** (user request). Menu or `Esc` opens the pause menu. "New game" was dropped as
  redundant with Restart and Main menu. Settings opened from pause returns to the paused game.
- **Little text, few keys** (user request): no key hints or subtext; only Esc, M, Space and F3.
- **Only the mover's tiles light up** (user request), so the turn reads at a glance.
- **Colorblind mode is off by default.** Its marks live in the orb atlas, so marked orbs still draw in one batch.
- **The board is four batched `DrawTriangles32` calls** (tiles, orbs, burst rings, flying orbs), with nothing cached
  per move. It draws unclipped, so bursts at the edge spill onto the mat as they do on Home.
- **Orb motion is the prototype's `draw_orbs_on_surface` run at its 80 px cell and scaled to the board** (user
  request), so every board shows the same picture. The prototype's fixed pixel constants crowded small cells.
- **Animation replays core's waves** from the old position. A test checks the replay ends equal to the game.
- **Chain pacing** (user choice: accelerate and skip): the first five waves take 14 ticks each, then each wave is 10%
  faster, down to 2.5 ticks, and a long chain is capped near 8 s. Space or a click finishes it within 0.4 s.
- **AI moves run on a goroutine** against a copy of the game, seeded per game plus the move number. The 300–600 ms
  delay comes from the presentation RNG. The Home demo uses budget 120.
- **The turn display lags the rules on purpose** until the mover's chain has played out.
- **Seats:** new seats take the next AI in dropdown order. A fresh install starts with P1 Human and shuffled AIs.
- **Sound:** C major pentatonic, C4–A6, each note a sine plus soft 2nd and 3rd harmonics with a 6 ms attack and
  exponential decay, and 15 voices. A placement plays the seat's own note, and a wave a 1–5 note chord rooted higher
  as the chain goes on.
- **Settings are JSON**, written through a temp file: `%AppData%\Orbitrix\settings.json` on Windows, the files folder
  on Android, `localStorage` on web. Missing fields keep their defaults.
- **Performance check:** `ORBITRIX_PERF_SECONDS=60 go test -tags perf -run TestPerf -v ./client`
  (`ORBITRIX_PERF_MARKS=1` turns marks on, `ORBITRIX_PERF_NOVSYNC=1` measures headroom). A case fails when its p99
  frame gap is over 25 ms, and an idle screen when it redraws more than about twice a second.

## Drawing cost (2026-10-31)

- **Shapes are white sprites, nine-sliced and tinted per draw** (`queueRound`, `drawShapes`). `vector` renders only
  sprites and cached images, never the screen, because each `vector` path costs several GPU passes and a
  render-target switch that phone GPUs can't afford. Setup went from 3.3–3.7 ms a frame to 0.6–0.7 ms.
- **Labels cache their glyphs** (`drawText`), so text is neither reshaped nor allocated every frame.
- **UI scale is capped at 2** (user choice). Phones at 2.6–3.5× draw about 40% fewer pixels, for slightly softer
  text.
- **`Draw` returns early when no `Update` ran**, so displays above 60 Hz don't draw duplicate frames. Still screens
  (Setup, Settings, a paused game, results) redraw only on input or change, plus every 30 ticks as a backstop.
  Not on Android, where skipped frames show stale buffers and flicker: there every screen draws once per tick.
- **The board clock stops during pause**, so resuming doesn't jump ahead.

## Web and Android (Phase 4)

- **One client everywhere** (user request). There is no separate mobile or web UI; layouts respond to the safe area.
- **Wide boards:** Android turns the phone to landscape for a game with more columns than rows (user request; either
  way up, even with auto-rotate off) and hands rotation back elsewhere. Browsers and desktop windows can't lock
  orientation, so they draw the board a quarter turn instead (user choice).
- **Touch:** a finger clicks on release if it moved under 10 px; a drag scrolls. Ebitengine polls touches once a
  frame, so the Android activity holds back a lift that comes under 50 ms after the press, and quick taps register.
- **Android:** arm64 and x86_64 only; a plain Gradle project with one Java activity and no AndroidX; immersive full
  screen (user choice). Back acts as Esc, and on Home it sends the app to the background. Config changes are handled
  in place, so rotating never restarts the game. AGP's built-in Kotlin is off, because it added 0.6 MB of
  kotlin-stdlib. The release APK is signed with the debug key until Phase 8.
- **Web:** fullscreen isn't restored at start, since browsers allow it only after a click. `cmd/webdist` stamps the
  wasm size into the loader and writes `.gz` and `.br` copies; `server.Static` serves the best copy the browser
  accepts and makes every file revalidate (`wasm_exec.js` and `game.wasm` must come from the same build). Brotli's
  dependency is used only by `cmd/webdist`.
- **Open:** subsetting the fonts would save some of their 1.4 MB.

## Layout (2026-10-31 to 2026-11-01)

- **Board presets: three shapes by six tiers** (user request; the user named and grouped them). Floor (13×18) is the
  largest board that keeps 24 px cells, the prototype's smallest cell and WCAG 2.5.8's minimum target, on a 360×780
  phone held upright. Landscape presets are the portrait ones turned, because a phone browser draws them upright.
  Tiers grow about 1.2× a side. Boards saved with the old presets load as Custom.
- **The game rail takes the width the board can't use** (user request), and never gets narrower than it was.
- **The seat table picks the layout that gives the biggest wheel** (`pickWheel`): one or two columns, with or without
  type names. Text and pills never shrink to grow the wheel (user correction). Equal wheels prefer names, then fewer
  columns. Upright on a phone, the bar under the board takes the height the board leaves, prefers the bigger wheel
  over type names (user choice), and centres in any spare height (user choice).

## Multiplayer (2026-10-31 and 2026-11-02, before Phase 5)

- **Player-hosted games** (user decision). The host runs `server/hub` and is authoritative; the Docker container only
  serves the website and signaling. SQLite, accounts, invites, bans, server-side rooms and spectators were dropped
  before any of them had code.
- **Online hosting needs `HOST_PASSWORD`** (user decision), because the URL is public and TURN is the only metered
  service. This replaced the brief's "no passwords" line.
- **No lockouts** (user requirement). CGNAT puts many people behind one IP, so a correct password or room code always
  passes. Brute force fails on the password's length, and wrong attempts only slow their own socket.
- **Rate limiting in two layers** (user request): Cloudflare's edge (DDoS protection, cache, WAF rules, one `/signal`
  rate rule) and origin token buckets keyed by trust. Bot Fight Mode is off because it would challenge native
  clients. Known gap: a flood from a friend's exact carrier IP blocks new `/signal` connections from that IP while it
  lasts. Closing it needs Cloudflare Business.
- **Bot defences** (reviewed at the user's request). Adopted: a Tor and datacenter-ASN rule on `/signal`, adaptive
  proof of work and a strict message order. Rejected: TLS and HTTP/2 fingerprints (Enterprise only), JS
  fingerprinting, Turnstile and CAPTCHAs (native clients can't run them), reputation feeds (paid) and behavioural
  scores (they would flag game clients).

## Seat inks 9–12 (2026-11-02)

- **Violet, Plum/Mint and Maroon replace Violet, Lime and Magenta** (user choice, from rendered samples). The old four
  extras caused every near-collision under simulated colour blindness: dark Yellow/Lime looked almost the same, and
  so did the two pinks, Mauve and Magenta.
- **How they were picked:** each was the colour furthest from every other ink, the mat and the tile under normal,
  red-weak, green-weak and blue-weak vision. Each also had to reach 3:1 contrast against the tile and the mat, and
  not be a second shade of another ink. No pair is now closer than the Okabe–Ito set's own closest pair
  (Orange/Yellow). Brown stayed, since changing it brought in crimson reds for a gain only under rare blue-weak
  vision.
- **Seat 11 changes hue between modes**, like Charcoal/Chalk. Players are shown as P1–P12, so ink names appear only
  in the docs.
- **Limit:** twelve inks can't all be easy to tell apart, so Colorblind mode's shape marks remain the full fix for
  games with nine or more players.

## Multiplayer engine (Phase 5)

- **Skip passes a seat's turns** (user choice). `core.Game.Pass` ends the turn without placing an orb; a player who
  passes before placing anything is out. The win check now waits until every player has had a turn. In play that is
  the brief's "once two have played", since nothing can explode in the first round; it differs only after a pass,
  which must not end the game before someone has moved.
- **Close reasons travel as `proto.Bye`**, the host's last message, so every transport carries them alike.
  `hub.Conn` is only Send, Receive and Close.
- **Wire format:** a version byte, a type byte, then fixed big-endian fields; strings are length-prefixed (names up to
  24 bytes). Encodings are canonical (a fuzz test checks that whatever decodes re-encodes to the same bytes). A game
  starts with `Snapshot{Seq: 0}`, so there is no separate start message, and a pass is `MoveApplied` with C = R = 255.
- **Hub rules:** Hello comes first, within 5 s. Messages are at most 128 bytes, 20 a second (burst 40), and a player
  with 64 messages queued is dropped. Late or illegal moves are ignored rather than punished, because they can race a
  turn timeout. Start needs every human seat held, connected and ready. A dropped player's seat is held 60 s, in the
  lobby too; Leave and Kick free it at once. A seat freed mid-game waits for the host's Replace or Skip. When the turn
  timer runs out, a random legal move (Jester) is played. If only skipped seats are left alive, the room returns to
  the lobby. The host's own player joins through `loopback` like everyone else.
- **Every connection the hub lets go of is reported** (`Config.OnClose`), a drop included, so the host revokes its
  TURN credential at once and a rejoin gets a new one. The credential cap therefore counts live credentials: the
  host's and the joiners' together at most the human seats plus 4.
- **WebRTC without trickle ICE** (shortcut): each side gathers its candidates before sending its description, so a
  join is one offer and one answer. TURN URLs on port 53 are dropped, as Cloudflare advises for that case.
- **Signaling answers alike** for a wrong code, a locked room, a full approval queue and a denial ("refused"). A
  rejoin carries the hub token and passes a lock. Joins that arrive while the host reconnects wait and are asked
  about again after `host.reclaim`. An approved joiner's credential outlives its signaling socket, because it carries
  the game until the host revokes it.
- **Proof of work:** HMAC-signed challenges with a bounded cache (100,000) of answered ones. A socket must answer the
  challenge it was given, so challenges saved from a quiet moment can't skip the work a flood raises. Difficulty follows
  anonymous connections over a sliding 10 s window: free up to 200 globally and 20 per IP, then 8 bits, plus 2 bits
  each time the rate doubles, capped at 18. 18 bits take about 37 ms on the development desktop; the web build on a
  phone is estimated at about 2 s and not yet measured.
- **Client IP:** `CF-Connecting-IP` is believed only from a private or loopback peer: the self-hoster's proxy, since
  the container publishes its port on 127.0.0.1 only. IPv6 callers are counted by their /64. The proxy must take
  connections only from Cloudflare's IP ranges, or a request sent around Cloudflare names its own address.
- **Peer ids are random**, not counted: a host keeps its joiners' ids across a server restart and revokes TURN
  credentials by them, so a restarted server's counter would hand an old id to a new joiner and revoke theirs.
- **Where "never blocked" ends:** the per-IP socket cap (100). The flood test keeps its bots under it; a flood that
  fills it sheds the honest socket from that IP too, as the edge rule does.
- **Website:** `/` is the landing page, mirroring Home in HTML (user choice). Its wordmark and PLAY use Unbounded,
  served from the build; its quiet labels use the system font, so Archivo isn't downloaded. `/play` and
  `/join/<code>` serve the game's loader. `cmd/webdist` renames `game.wasm`, `wasm_exec.js` and the font after their
  content hash and the pages refer to them by absolute path, so a LAN host can serve the same build at its root.
  Hashed files are cached for a year; pages revalidate. Downloads are offered only when the file is in `-downloads`,
  and cached for 5 minutes, so the edge serves them (a no-cache kept Cloudflare from storing them at all).
- **Server port 1212** (user request: not 8000, 8080 or another common port, yet easy to remember): "twelve twelve",
  for the twelve seats. Windows reserves nothing there, and it stays clear of the LAN hub's 7777.
- **Multi-stage Dockerfile** (user decision, replacing an earlier package-only one). A Go stage runs
  `go tool mage web` and cross-compiles the server for the target platform (the stage runs on the build machine's,
  so an ARM image needs no emulation); the final stage is distroless static (non-root; compose runs it read-only
  with no capabilities). The build context is an allowlist (`deploy/Dockerfile.dockerignore`), so `.env` and local
  files never reach it. Compose runs the `orbitrix-server` image and never builds.
- **Downloads stay out of the image** (user decision): compose mounts `deploy/downloads` read-only, so publishing a
  new `.exe` or `.apk` needs no image rebuild.
- **No tunnel in compose** (user decision). Self-hosters choose how to expose the server, so compose bundles no
  `cloudflared` and publishes the port on 127.0.0.1 only, for their own reverse proxy. `compose.dev.yml` only sets a
  localhost origin.
- **TURN only when everything is in place** (user decision): a `HOST_PASSWORD` of at least 20 characters, both
  `CF_TURN_*` values and a working Cloudflare call. Anything less falls back to STUN, which costs nothing, and is
  logged: an unset password opens hosting to anyone, a shorter one still guards hosting, one key value alone is
  ignored, and a failed Cloudflare call gives that host or joiner STUN instead of an error. With hosting open, the
  challenge says so (`open`), so clients skip the password prompt, and one IP may hold at most 3 rooms. This replaces
  the earlier rule that hosting needs a password, and a short-lived published default password.
- **Race detector in CI:** the Windows development box has no cgo, so CI runs `go test -race` on the server,
  protocol and rules packages.

## Design review before online play (Phase 6)

- **Review:** the impeccable critique of the Phase 3–4 screens scored 24/40 (`.impeccable/critique/`). The user took
  every fix but the first:
  - **Seats keep their type as their name** (user choice), not P# tags, even though DESIGN.md had specified them.
    Players will name themselves later, as in .io games; until then a human is "Human" and a bot its AI's name.
  - **Restart and Main menu ask for a second tap**, in place: the pill turns solid ink and reads "Restart?" for 3 s.
  - **A tap the mover can't make flashes that tile's outline** in the mover's ink, instead of doing nothing.
  - **AI difficulty shows as five pips**: the twelve AIs in their measured order, five even steps.
  - **Results:** a solid winner's dot, so it can't read as an out seat's ring, and the biggest chain only from ×2.

## Online play in the client (Phase 6)

- **Ways to play** (user choice): PLAY swaps itself and its buttons for This Device, Online (and Same Wi-Fi in Phase
  7), then Back, on Home itself. "Local multiplayer" was avoided because games use it for couch co-op; This Device
  covers playing alone against bots as well as passing one device round.
- **One Online page** (user choice): hosting (Set up a room) above joining (six code slots), create before join as
  on Meet, Zoom and Discord, in one centred column (at most 640 px) on every target. A `/join/<code>` address opens
  it with the code filled in, as does a seat this player may rejoin.
- **The device's own keyboard, not a drawn one** (user choice, replacing the drawn keypad and keyboard): Ebitengine
  v2.10's `exp/textinput` opens the on-screen keyboard on Android (a hidden edit text inside EbitenView) and in phone
  browsers (a hidden textarea), and takes physical typing on desktop. A browser opens its keyboard only during a
  touch, so a field takes the focus when the pointer goes down on it, not on release. The Android activity sets the
  hidden edit text's input type per field (`platform.Keyboard`): capitals without suggestions for a code, the password
  variation with no personalised learning for the host password, keeping Ebitengine's multi-line flag so Return
  still reaches the game. M types instead of muting while a field has the focus.
- **The lobby is laid out like Setup** (user choice): a Room block and a Players block, with the action along the
  bottom.
- **Native builds learn their server at build time** (user choice): `ORBITRIX_SERVER` is stamped in by
  `magefile.go`; without it a build offers no Online. The desktop build also reads it at start, for local testing. The web
  build uses the address it was loaded from.
- **Players send the name "Human"** until they can name themselves. A seat's own row reads "(you)", so online players
  can find their colour.
- **The host is ready as soon as the room opens.** Start fills open seats with bots (Setup's default AI for that
  seat), so a host never waits for a friend who didn't come.
- **Join requests are answered for the host when no seat is open**: they are refused rather than shown.
- **Seat left:** the host is asked at once, in a dialog, whether a bot plays the seat or its turns are skipped. Kick
  is in the lobby only.
- **Moves are paced on each screen, not on the host:** the hub waits 400 ms before an AI move (so a browser host
  draws between thoughts), and each client plays a bot's move 300–600 ms after the last one finished, as locally.
  Moves that arrive while a chain plays wait their turn.
- **Hash check:** each `MoveApplied` is played on the client's board and its hash compared; any mismatch, gap or
  move that doesn't fit drops what is queued and asks for a `Snapshot`.
- **Online pause** is only a menu: the game goes on for everyone else, and moves wait until it closes. Esc on an
  online game's result does nothing (the host's Esc would close the room for everyone).
- **The hub ignores room codes online** (`Config.Code` empty): signaling already admits only approved players, and a
  host whose room the server lost gets a new code that seated players' Hellos would no longer match.
- **Rejoin:** a joiner saves the room code and the hub's token on Welcome and forgets them when the room ends or they
  leave; a dropped connection keeps them and offers Rejoin. A rejoin is approved by the host's hub without asking.
- **Closing the desktop window while hosting** closes the room, then waits half a second for the Bye messages
  (shortcut). A browser tab asks before closing (`beforeunload`) and holds a screen wake lock; Android keeps the
  screen on while hosting.
- **Share link:** Android opens the share sheet, browsers their share sheet or the clipboard, Windows the clipboard
  ("Copied").
- **Android needs `-checklinkname=0`:** pion's interface lookup there (`github.com/wlynxg/anet`) links to a `net`
  internal that Go's linker refuses otherwise. The app now asks for `INTERNET`.
- **Size:** pion added 7.3 MB to the APK (now 18.1 MB) and 1.7 MB brotli to the web build (33.6 MB raw, 5.6 MB
  brotli).

## LAN play and one Room page (Phase 7)

- **Same Wi-Fi lists rooms; private ones take a code** (user decisions, in two steps). The first build joined by
  code with an Address row for join by IP; the user dropped the address ("modern apps don't need them"), then the
  code for ordinary rooms ("a join code on a LAN is illogical"), then asked for public and private rooms on both
  kinds. A public LAN room is listed by its host device's name and joined with a tap, no approval; a private one is
  never listed and its code finds it among the rooms heard. There is no join by IP.
- **Online public rooms are listed, and the host still lets people in** (user choice). The host's `update{public,
  humans, listed}` tells signaling whether the room is public, its human seats (which bound its TURN credentials) and
  how the list shows it; an anonymous `list` after the proof of work returns the public rooms with their codes. Rows
  show the board's preset and size, not a device name, since the list is open to the internet. Private rooms are
  never listed. Every online join, public or private, waits for Let in, so strangers never get a seat or TURN without
  the host's yes.
- **Defaults under Settings, Network:** Same Wi-Fi rooms start public and Online rooms private (user choice); the
  Room page's Room button changes it for one room. The Online tab became Network.
- **Lock is gone** (user request), in the client and in signaling: public/private and Kick cover it. "Keep this tab
  open" is gone too; the web host keeps its wake lock and `beforeunload` question.
- **One Room page** (superseded by "The room is Setup" below; user request; impeccable surface round, seed d38a7a29, the user locked "Board-led sheet" over
  "the seat table is the page" and "single-column ticket"). Set-up and lobby share the page; Open room becomes Start
  in place. Every setting is a button opening a dialog: the mini mat is the Board button; Players, Room
  (Public/Private) and each seat have their own. Kick moved into the seat dialog (Remove player), so seat rows stay
  one control each. The invite code with Share sits in the host's top block, under the setting buttons, the same on a
  phone and a desktop; Share sends the join link online and the code on Same Wi-Fi. Joiners see only the seats.
  This Device's Setup is unchanged.
- **Settings stay live until Start** (user choice): `hub.Room.Configure` changes the board and seats in the lobby.
  A seat that goes away or turns into a bot lets its player go as a kick; the host's own seat can't. Seats keep one
  backing array (capacity 12) so grace timers never point at a stale seat.
- **Pause menu:** Main menu leaves at once (user request); Restart still asks twice.
- **No browser LAN play** (user decision): the desktop no longer embeds the web build, and the web build never
  offers Same Wi-Fi. `lan.Host` keeps a stop closure rather than its `http.Server`, and the Room page calls `lan.Open`
  behind a constant `runtime.GOOS` check, so the web build carries no HTTP server: it grew 0.2 MB over Phase 6,
  not 2.8 MB.
- **Discovery by subnet broadcast,** not multicast: every private IPv4 network the host is on gets a 1 s
  announcement (magic, version, room id, name, open seats, playing/private flags, port). Broadcast needs no group
  joins, which Android 11+ makes awkward, and home routers pass it. `github.com/wlynxg/anet` (already in through
  pion) reads interfaces where Android 11+ refuses Go's own lookup. The random room id makes a host heard on two
  networks one room. Joiners hold Android's multicast lock while the page is open.
- **Private interfaces only:** the host listens on each private IPv4 address (7777, then up to 15 more ports), not
  0.0.0.0. Detecting a public network is left to the OS: Windows' firewall prompt allows private networks by default.
  Known limit: a host whose address changes mid-session (new DHCP lease) stops being reachable there.
- **Windows firewall notice before Same Wi-Fi** (moved from Open room): listening for rooms brings up Windows'
  prompt for joiners too, so the first Same Wi-Fi tap on Windows says so once, with Continue and Back.
- **A quiet host is noticed:** `ws` pings its peer every 10 s and drops it after 10 s without a pong; a LAN joiner
  then sees "Host disconnected" and retries every 2 s for the 60 s seat grace with its rejoin token (and code), and
  is back in its seat when the host returns. Browsers can't ping, so only native sides check.
- **No network security config on Android:** it governs Java's HTTP stacks; Go dials its own sockets, and an
  emulator joined a desktop host over plain `ws://` without one.
- **Room codes live in `proto`** (`CodeLen`, `CodeAlphabet`, `NewCode`), shared by signaling, the LAN host and the
  client's code field.
- **Verified:** unit and scene tests (a LAN room played through, a dropped joiner winning its seat back, the Room page
  driven by clicks through its dialogs, a public online room listed); on the API 36 emulator, the room list heard a
  PC room through an emulator redirect, a tap joined it and a game ran, and an Android-hosted room seated a PC client.
  Not yet: two real devices on one Wi-Fi without internet.
- **Join requests are a snackbar** (user request), one at a time like Material's queue, above the action on every layout; request rows left the Players block. A private room's code tiles show empty until the room opens. The Join block has two labelled groups, the list and Join by code; Same Wi-Fi and Online were already one scene (`joinScene`).
- **The room is Setup** (user direction, replacing the Board-led sheet): the host's room page is This Device's Setup
  scene itself (`setupScene` with a `hostRoom`), so Players, Board and the seat dropdown are the same code on both.
  What a room adds sits outside the blocks: a round key button beside the action opens the Room dialog (Public or
  Private; when Private is selected, the code in the join page's tiles and an icon-only copy button), and join
  requests come as a snackbar. Access changes are a draft until Confirm; Cancel or dismiss discards them. The Board,
  Players and seat dialogs were dropped; joiners keep a seats-only page.
- **Remove player** (user request): any seat, bot or human, from the seat dropdown, on both Setups.
  `hub.Room.RemoveSeat` lets the seat's player go, moves the seats after it up with their players and sends those a
  Welcome with their new seat. Seat holds now take room-wide numbers, so a grace timer whose seat moved does nothing,
  and a moved away seat starts its grace again. It is drawn in a danger ink (user request), the one colour outside
  the player inks.
- **Players name themselves on Home** (user request): one name per device, saved as typed (`settings.Name`;
  `settings.name()` falls back to "Human"). It sits in a welcome plaque above PLAY, "Welcome, Ana!" after PopCap's
  greeting (user choice, after rendered mockups of a plaque, a name tag and a captioned box): bare words in the
  buttons' label face, the name in ink between the dim greeting and its mark, no fill and no border (user
  direction: a pill read as a button). It hugs its line (at least the buttons' width), and a tap gives it the
  keyboard, with a caret after the name. Nothing shrinks for it (user direction): on a short landscape phone only the
  column's gaps tighten, and when the ways to play need its room they take it until Back. It labels this
  player's first human seat in local Setup, the game rail and Results, and is the name a host's own Hello and a
  joiner's Hello and signaling `join` carry, so the hub's lobby and the Let in snackbar show it. Text fields now cap
  bytes, not runes: a name takes at most 16 (within `proto.MaxName`'s 24, which signaling enforces), short enough
  for a seat pill; Setup measures the names it shows and lays out again when the lobby changes.
- **Swap position** (user request): the seat dropdown's option above Remove player turns the dropdown into the seats
  this one may trade with; a tap trades them, choices and all. Two humans never trade, on this device or in a room
  (user decision): a human seat lists only the bots, judged by the lobby's seat types once a room is open, and
  `hub.Room.SwapSeats` refuses two human seats. In an open room the hub moves the players and sends each a Welcome
  with their new seat, as Remove player does (`Room.moved`). Swap position and Remove player are always listed (user
  request), greyed out where they can't act: no seat to trade with, or two players left.
