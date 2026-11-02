# Orbitrix

A turn-based chain-reaction orb game for 2–12 players, written in Go with Ebitengine. Targets Windows, Android and
the web (WASM), with online and LAN games hosted by one of the players.

<!-- impeccable:product-schema 1 -->

## Platform

web

One custom-drawn game look on every target. The UI never uses web technology: no HTML, CSS or JavaScript UI, on
any platform including the web build. Ebitengine draws all of it in Go (no DOM, no native widgets), so the value
above means "not a native OS design language", not "HTML/CSS". The only HTML is `web/index.html`, the bootstrap
page that loads `game.wasm` and shows its loading progress; it is not game UI. Platform needs still apply where they
exist: touch targets and a portrait layout on Android, safe areas, Android back acting as Esc, keyboard shortcuts on
desktop and web.

## Users

- **Now:** the developer and their friends and family. They play pass-and-play on one device, at LAN sessions with
  no internet, and online in rooms one of them hosts. Devices are mixed: Windows PCs, Android phones (APK
  installed directly) and browsers, often in the same room.
- **Later (possible):** a public audience of players who don't know the developer. Nothing is committed yet, but
  future work must not make a public release impossible.

## Product Purpose

Orbitrix is a turn-based chain-reaction orb game for 2–12 players. Players take turns placing orbs. A cell that
reaches its capacity explodes into its neighbours and captures them, which can set off chains across the board. The
last player with orbs on the board wins. Success means a group of friends can start a game on whatever devices they
have, fill empty seats with AI, and enjoy following a chain reaction as it plays out.

## Positioning

- **Huge, chaotic games:** up to 12 players on boards up to 40×40, with long cascading chains that still resolve
  exactly and play back wave by wave.
- **AI personalities:** twelve bots with distinct play styles, listed easiest to hardest: Jester, Mason,
  Warden, Gambler, Nomad, Raider, Spy, Prodigy, Tactician, Voyager, Hunter, Sovereign. They can take any seat or be
  played against alone.
- **Play together anywhere:** same device, LAN with no internet, or online. Windows, Android and browser clients
  share one room, one rules engine and one protocol.
- **Tactile feel:** orbs that orbit while idle and turn frantic just before exploding, with pentatonic sound
  generated in code.

## Operating Context

- Local games on one device, passing it between players or playing alone against AIs.
- LAN games: one Windows or Android device hosts; others join through discovery, by IP, or (in a browser) at
  `http://<host-ip>:7777`.
- Online games hosted from any player's client, a browser tab included, by a player who has the server's host
  password. The host shares a room code or
  `/join/<CODE>` link privately, approves each player, and can lock the room or kick. Players connect to the host
  over WebRTC, through a Cloudflare TURN relay when their networks block a direct connection. The Docker container
  on the developer's NAS only serves the website and signaling; no game traffic passes through it.
- Codes and names are typed on a keypad drawn in the game on every platform, because mobile has no on-screen
  keyboard.

## Capabilities and Constraints

- Rules, screens, AI behaviour and multiplayer design are specified in `CLAUDE.md`, which is the source of truth.
- Every graphic is drawn in code and every sound is synthesised in code. There are no image or audio files. The
  only embedded assets are two open-licence (OFL) fonts, Unbounded and Archivo, in `client/fonts/` with their
  licences.
- Target: 60 FPS on a 40×40 board with 12 players in a long chain, on a mid-range Android phone and in desktop
  Chrome.
- Undecided: whether, when and through which channel (stores or otherwise) a public release happens.

## Brand Commitments

- Name: Orbitrix.
- Visual world: **Party Mat**, a printed party floor mat. Colour belongs to the players and nothing else; every
  control is ink on vinyl. Flat only: no 3D, no shadow depth, no gradients, no neon. Nothing is taken from the
  prototype's UI. The design system is recorded in `DESIGN.md`.
- Two modes: light (bright vinyl mat, the default) and dark (warm charcoal mat), switchable and saved. No blue tint
  in either mode's neutrals; blue appears only as the Blue and Sky player inks.
- The 12 player inks (colour-blind tuned, first eight from the Okabe–Ito set) and their order are fixed, each with a
  shape mark: Vermilion (dot), Blue (bar), Green (plus), Orange (cross), Mauve (triangle), Sky (square), Yellow
  (diamond), Charcoal/Chalk (stem), Violet (pair), Brown (ring), Lime (chevron), Magenta (frame).
- Type: Unbounded for display, Archivo for UI.
- Orb motion (idle orbit, frantic before exploding, per-cell variation) is ported exactly from the prototype and
  stays binding; only how orbs look follows the design system. Board cells are rounded square tiles with gutters.
- Labels stay plain and short ("Play", "Start", "Restart", "Settings", "Main menu", "Play again"). No key hints on
  buttons and no explanatory subtext.

## Evidence on Hand

- `prototype/main.py`: the pygame prototype, used as reference for rules, AI and orb motion only (its UI was
  replaced). It is gitignored in this repo; the original lives on SynologyDrive.
- `go run ./cmd/aitourney`: AI-versus-AI win rates and move times.
- Benchmarks and design choices: `docs/decisions.md`.
- No players outside the friend group, no testimonials, no screenshots or store listings, and no usage data exist.
  Do not invent any of them.

## Product Principles

1. **Chaos stays readable.** However large the board or the player count, people must be able to follow what
   happened: chains animate wave by wave, and AI moves are delayed on purpose.
2. **Every seat is equal.** A human, an AI, a local player, a LAN player and an online player all fill the same seat
   and play under the same deterministic rules.
3. **Feel is part of the game.** Orb motion, sound and a steady frame rate are product features, not decoration.
4. **Friends first, public possible.** Build for a trusted group today, without baking in assumptions that would
   block strangers later.

## Accessibility & Inclusion

- Players are told apart by colour. The first eight inks follow the Okabe–Ito colour-blind-safe set; Colorblind
  mode (Settings → Video) adds each player's shape mark to their orbs, seat dots and spinner wedge, which seats
  9–12 rely on more.
- Input must work with touch on Android and with mouse and keyboard on desktop and web.
- No accessibility standard has been set.

## Prerequisites

| Tool | Needed for | Notes |
|---|---|---|
| [Go](https://go.dev/dl/) 1.27.3+ | everything | The exact version is pinned in `go.mod`; older toolchains download it automatically (`GOTOOLCHAIN=auto`). |
| [JDK](https://adoptium.net/) 17+ | Android | Used by Gradle (the wrapper in `android/` fetches Gradle 9.8.2 and the Android Gradle Plugin itself). |
| [Android SDK](https://developer.android.com/studio) + NDK | Android | The scripts use `ANDROID_HOME`, or `%LOCALAPPDATA%\Android\Sdk` / `~/Android/Sdk` when it isn't set. Install an NDK with the SDK manager (`sdkmanager "ndk;30.0.16248370"`). |
| Docker | server deployment | Only for running the website and signaling server in a container. |

Desktop builds on Windows and Linux are pure Go (`CGO_ENABLED=0`). Only the Android/iOS bindings need cgo, through
`ebitenmobile`, which is pinned in `go.mod` as a tool (`go tool ebitenmobile`).

## Building

Every script target works the same from bash and PowerShell. Output goes to `dist/`.

```sh
bash scripts/build.sh [check|desktop|web|server|android|all]     # bash / Git Bash / CI
```

```powershell
.\scripts\build.ps1 [check|desktop|web|server|android|all]       # PowerShell
```

| Target | Output |
|---|---|
| `check` | gofmt, `go vet ./...`, `go test ./...` |
| `desktop` | `dist/orbitrix[.exe]` (cross-compile with `GOOS=windows`) |
| `web` | `dist/web/`: `game.wasm`, `wasm_exec.js`, `index.html`, each with `.gz` and `.br` copies (`cmd/webdist`) |
| `server` | `dist/orbitrix-server[.exe]` |
| `android` | `dist/orbitrix.aar` (the Go game, arm64 + x86_64) and `dist/orbitrix.apk` (installable) |
| `all` | check, desktop, web and server (default); `android` is separate because it needs the SDK |

Run the desktop client directly with `go run ./cmd/orbitrix`. F3 toggles the frame-rate overlay (`-fps` starts
with it on).

Play the web build locally after a `web` build: `go run ./cmd/orbitrix-server -web dist/web`, then open
`http://localhost:8080`. The server sends the precompressed copies to browsers that accept them.

Install the Android build on a connected phone or emulator with `adb install -r dist/orbitrix.apk`. It is signed
with the debug key, so it installs directly (no store).

The AI tournament plays every pair of AI strategies and prints win rates and move times:

```sh
go run ./cmd/aitourney -games 20 -size 9x9      # -seed, -workers also available
```

Prodigy's network is trained by self-play and by games against Tactician, Hunter and Voyager, and stored as
generated Go source in `core/ai/prodigy_weights.go`. Training continues from the current weights and rewrites that
file only when the weights beat the best score so far in duels against Hunter and Voyager and in four- and
twelve-player tables. It uses every core; a generation takes about 3.5 minutes on a 16-thread desktop (bash, then
PowerShell):

```sh
ORBITRIX_TRAIN_MINUTES=480 go test -tags train -run TestTrainProdigy -timeout 0 -v ./core/ai
```

```powershell
$env:ORBITRIX_TRAIN_MINUTES=480; go test -tags train -run TestTrainProdigy -timeout 0 -v ./core/ai
```

Sovereign's network is trained supervised, NNUE style. Each iteration plays games on 9×9, 20×20 and 40×40 with
Hunter, Voyager, Tactician, Prodigy and Sovereign seats, labels the positions with the results and Sovereign's own
deeper search, trains a fresh network on every position collected so far, and rewrites
`core/ai/sovereign_weights.go` only when the new network wins at least 55% of the games either network wins, in duels
and free-for-all tables against the current one. Positions accumulate in `%TEMP%\orbitrix-sovereign` (or
`ORBITRIX_SOV_DATA`), so a new run continues from all earlier data. An iteration of the default 960 games takes
about 1.5 hours; the other `ORBITRIX_SOV_*` settings are listed at the top of `core/ai/sovereign_train_test.go`:

```sh
ORBITRIX_TRAIN_MINUTES=480 go test -tags train -run TestTrainSovereign -timeout 0 -v ./core/ai
```

```powershell
$env:ORBITRIX_TRAIN_MINUTES=480; go test -tags train -run TestTrainSovereign -timeout 0 -v ./core/ai
```

A bot's matches against every other bot on the benchmark boards, with random first placements
(`ORBITRIX_MATCH_BOT` picks the bot, default Prodigy; `ORBITRIX_MATCH_SIZES`, `ORBITRIX_MATCH_FOES`,
`ORBITRIX_MATCH_DUELS=0` and `ORBITRIX_MATCH_WEB=1` narrow the run):

```sh
ORBITRIX_MATCH_BOT=sovereign ORBITRIX_MATCH_GAMES=100 go test -tags train -run TestMatches -timeout 0 -v ./core/ai
```

```powershell
$env:ORBITRIX_MATCH_BOT="sovereign"; $env:ORBITRIX_MATCH_GAMES=100; go test -tags train -run TestMatches -timeout 0 -v ./core/ai
```

## Layout

| Path | Contents |
|---|---|
| `core/` | Pure game rules (board, moves, wave resolution, win/turn order, seeded RNG). No Ebitengine, net or OS imports. |
| `core/ai/` | The twelve AI personalities, and Prodigy's and Sovereign's training and match harness (build tag `train`). |
| `cmd/aitourney/` | Headless AI-vs-AI tournament. |
| `cmd/webdist/` | Finishes a web build: loading-progress size and precompressed copies. |
| `proto/` | Binary wire protocol. |
| `client/` | Ebitengine game: scenes, rendering, input, audio, settings. `client/fonts/` holds the two OFL fonts and their licences. |
| `cmd/orbitrix/` | Desktop and web entry point. |
| `mobile/`, `android/` | `ebitenmobile` binding and the Gradle project that wraps it (one activity in Java). |
| `web/` | Page that loads `game.wasm`. |
| `server/`, `cmd/orbitrix-server/` | Website and signaling server; `server/hub` is the room engine every host runs, `server/lan` the LAN host. |
| `deploy/` | Dockerfile, docker-compose, Cloudflare Tunnel config. |
| `docs/decisions.md` | Choices made where the brief left room. |
