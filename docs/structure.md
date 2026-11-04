# Structure

```
orbitrix/
├── .github/
│   ├── dependabot.yml          Weekly updates for Go modules, Actions, the Docker base image and Gradle
│   └── workflows/
│       ├── ci.yml              Checks, race tests, desktop, image and Android builds on main and pull requests
│       └── release.yml         Tagged releases: clients to GitHub, the image to Docker Hub
├── core/                       Game rules
│   ├── game.go                 Game, Apply, waves, alive, win and turn order, Pass
│   ├── geometry.go             Capacity and neighbour tables, Index and Coords
│   ├── rng.go                  splitmix64 RNG
│   ├── snapshot.go             Snapshot, Restore and Hash
│   ├── testdata/fuzz/          Saved fuzz inputs
│   └── ai/                     The bots
│       ├── ai.go               Bot, Choose, Strategy, Parse, budgets and the shared helpers (simulate, tally)
│       ├── safety.go           Survival protection for fragile openings
│       ├── jester.go … sovereign.go   One file per strategy
│       ├── prodigy_weights.go, sovereign_weights.go   Trained networks as generated Go source
│       └── *_train_test.go     Training and match harness (build tag train)
├── proto/                      Binary wire protocol, version, room codes
├── client/                     The Ebitengine game
│   ├── game.go                 The app: scene switching, input, the frame loop
│   ├── ui.go, page.go, theme.go   Buttons and drawing helpers, the shared page frame, the two themes
│   ├── home.go, setup.go, room.go, joinpage.go, password.go, gone.go   Home, Setup and the room, join pages, host password, room ended
│   ├── play.go, view.go, board.go, orbs.go, count.go   The game screen, wave replay, board batches, orb motion, orb counts
│   ├── net.go, netplay.go      Online and LAN sessions, and a networked game: hash checks, resync, pacing
│   ├── audio.go                Generated pentatonic notes and voice limits
│   ├── settings.go, settingsmenu.go, textfield.go, ask.go   Saved settings, the Settings screen, text entry, dialogs
│   ├── online/                 Hosting and joining online rooms
│   ├── platform/               Per-platform storage, clipboard, keyboard type, device name, server address
│   └── fonts/                  Unbounded and Archivo, with their OFL licences
├── server/                     Website: landing page, /play, /join, downloads, static files, /signal
│   ├── hub/                    The room engine every host runs
│   ├── transport/
│   │   ├── loopback/           In-process Conn for the host's own player
│   │   ├── ws/                 WebSocket Conn (LAN), with keepalive pings
│   │   └── rtc/                pion WebRTC data channel Conn (online)
│   ├── signal/                 Signaling service, its client, Cloudflare TURN
│   │   └── pow/                Proof-of-work challenges and solver, shared by server and clients
│   └── lan/                    UDP discovery, the LAN host and joining
├── cmd/
│   ├── orbitrix/               Desktop and web entry point
│   ├── orbitrix-server/        Website and signaling server entry point
│   ├── aitourney/              Headless bot tournament
│   └── webdist/                Finishes a web build: hashed names, favicon, size stamp, .gz and .br copies
├── mobile/                     ebitenmobile binding package (Start, SetSafeArea, Back)
├── android/                    Gradle project: one Java activity wrapping dist/orbitrix.aar
├── web/
│   ├── index.html              Loader page for game.wasm, with seat-dot progress
│   └── landing.html            Landing page template
├── deploy/
│   ├── Dockerfile              Multi-stage build: Go stage, then scratch with the server, CA roots and web build
│   ├── Dockerfile.dockerignore Build-context allowlist
│   ├── docker-compose.yml      Runs the published image on 127.0.0.1:1212
│   ├── compose.dev.yml         Local override with a localhost origin
│   └── .env.example            Server settings to copy into .env
├── docs/                       This documentation, and the README's light and dark logos (generated)
├── magefile.go                 Build targets
├── go.mod, go.sum              Module, pinned Go and Ebitengine versions, Mage and ebitenmobile as tools
└── LICENSE                     GPL-3.0
```

`dist/` holds build output and is not tracked.

## Layers

From the bottom up:

1. `core` and `core/ai`: rules and bots. They import only the standard library, so they compile for every target, run without a window and behave identically on every host.
2. `proto`: the wire format, which depends on `core` for snapshots.
3. `server/hub` and `server/transport`: the room engine and its connections. The hub depends on `core`, `core/ai` and `proto`, and on transports only through `Conn`.
4. `server/signal`, `server/lan` and `client/online`: discovery and pairing, online and on the LAN.
5. `client` and `client/platform`: everything the player sees, hears and touches.
6. `cmd/*`, `mobile` and `android`: entry points and packaging.

## What goes where

- Anything that changes the outcome of a move belongs in `core`, never in the client or the hub. `core` must not import Ebitengine, `net`, `os` or `math/rand`'s global source.
- Bot logic belongs in `core/ai`, one file per strategy, with shared helpers in `safety.go`. A bot never reads the clock; its cost is counted in budget units.
- The hub must not depend on HTTP, WebRTC or signaling. A new way to connect players is a new `Conn` in `server/transport`.
- Platform differences belong in `client/platform` behind build tags, not in `runtime.GOOS` checks spread through the client.
- Game messages are binary `proto` messages on every transport. JSON is only for signaling and settings.
- Graphics are drawn in code and sounds are generated. `internal/appicon` shares the logo geometry between the web favicon, desktop window icons, native build icons and Android's launcher vector. Native PNG, ICNS and Windows resources are generated during builds. The README SVGs and Android vector are checked in; `TestDocsLogos` fails when they drift, and `go test ./cmd/webdist -run TestDocsLogos -update` rewrites them. The two fonts in `client/fonts/` are the other assets.
- Build steps go in `magefile.go`, not shell scripts, so they run the same from bash and PowerShell.
