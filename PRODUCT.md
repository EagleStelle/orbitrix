# Orbitrix

A turn-based chain-reaction orb game for 2–12 players, written in Go with Ebitengine. Targets Windows, Android and
the web (WASM), with an optional online server and LAN play.

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
  no internet, and online through an invite-only server. Devices are mixed: Windows PCs, Android phones (APK
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
- **AI personalities:** eight bots with distinct play styles (jester, gambler, sentinel, conqueror, spy, architect,
  voyager, tactician) that can take any seat or be played against alone.
- **Play together anywhere:** same device, LAN with no internet, or online. Windows, Android and browser clients
  share one room, one rules engine and one protocol.
- **Tactile feel:** orbs that orbit while idle and turn frantic just before exploding, with pentatonic sound
  generated in code.

## Operating Context

- Local games on one device, passing it between players or playing alone against AIs.
- LAN games: one Windows or Android device hosts; others join through discovery, by IP, or (in a browser) at
  `http://<host-ip>:7777`.
- Online games through the invite-only Docker server: invite codes, room codes, lobbies, rematches, spectators.
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
| [JDK](https://adoptium.net/) 17+ | Android | Used by Gradle and `ebitenmobile`. |
| [Android SDK](https://developer.android.com/studio) + NDK | Android | Set `ANDROID_HOME`; install an NDK through the SDK manager (`ANDROID_NDK_HOME` if it isn't found). |
| Docker | server deployment | Only for running the online server in a container. |

Desktop builds on Windows and Linux are pure Go (`CGO_ENABLED=0`). Only the Android/iOS bindings need cgo, through
`ebitenmobile`.

## Building

Every script target works the same from bash and PowerShell. Output goes to `dist/`.

```sh
bash scripts/build.sh [check|desktop|web|server|all]     # bash / Git Bash / CI
```

```powershell
.\scripts\build.ps1 [check|desktop|web|server|all]       # PowerShell
```

| Target | Output |
|---|---|
| `check` | gofmt, `go vet ./...`, `go test ./...` |
| `desktop` | `dist/orbitrix[.exe]` (cross-compile with `GOOS=windows`) |
| `web` | `dist/web/` — `game.wasm`, `wasm_exec.js`, `index.html` |
| `server` | `dist/orbitrix-server[.exe]` |
| `all` | all of the above (default) |

Run the desktop client directly with `go run ./cmd/orbitrix`. F3 toggles the frame-rate overlay (`-fps` starts
with it on).

The AI tournament plays every pair of AI strategies and prints win rates and move times:

```sh
go run ./cmd/aitourney -games 20 -size 9x9      # -seed, -workers also available
```

## Layout

| Path | Contents |
|---|---|
| `core/` | Pure game rules (board, moves, wave resolution, win/turn order, seeded RNG). No Ebitengine, net or OS imports. |
| `core/ai/` | The eight AI personalities. |
| `cmd/aitourney/` | Headless AI-vs-AI tournament. |
| `proto/` | Binary wire protocol. |
| `client/` | Ebitengine game: scenes, rendering, input, audio, settings. `client/fonts/` holds the two OFL fonts and their licences. |
| `cmd/orbitrix/` | Desktop and web entry point. |
| `mobile/`, `android/` | `ebitenmobile` binding and the Gradle wrapper project. |
| `web/` | Page that loads `game.wasm`. |
| `server/`, `cmd/orbitrix-server/` | Online server; `server/hub` is the shared room engine, `server/lan` the LAN host. |
| `deploy/` | Dockerfile, docker-compose, Cloudflare Tunnel config. |
| `docs/decisions.md` | Choices made where the brief left room. |
