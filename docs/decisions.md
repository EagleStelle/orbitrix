# Decisions

Choices made where the brief or the prototype left room, and why. The specs themselves live in `CLAUDE.md`,
`DESIGN.md` and `PRODUCT.md`; this file keeps only what they don't say.

## Tooling (Phase 0)

- **Module path `orbitrix`.** Nothing outside the repo imports it.
- **Go 1.27.3 and Ebitengine v2.10.5 pinned** (latest stable on 2026-10-30). Older toolchains switch through
  `GOTOOLCHAIN=auto`; CI reads the version from `go.mod`.
- **Build scripts, not a Makefile**, since Windows has no `make`: `scripts/build.sh` and `scripts/build.ps1` share
  target names.
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
