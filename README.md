# <picture><source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg"><img src="docs/logo.svg" alt="" width="32"></picture> Orbitrix

[![CI](https://github.com/EagleStelle/orbitrix/actions/workflows/ci.yml/badge.svg)](https://github.com/EagleStelle/orbitrix/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/EagleStelle/orbitrix?sort=semver)](https://github.com/EagleStelle/orbitrix/releases/latest)
[![License: GPL v3](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27.3-00ADD8?logo=go&logoColor=white)
![Ebitengine](https://img.shields.io/badge/Ebitengine-v2.10.5-DB5F24)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Android%20%7C%20Web%20%7C%20macOS%20%7C%20Linux-informational)
![WebRTC](https://img.shields.io/badge/pion%2Fwebrtc-v4-333333?logo=webrtc&logoColor=white)
[![Docker](https://img.shields.io/docker/v/eaglestelle/orbitrix-server?sort=semver&label=docker&logo=docker&logoColor=white)](https://hub.docker.com/r/eaglestelle/orbitrix-server)

A chain-reaction orb game for 2 to 12 players, on one device, over the same Wi-Fi, or online.

Players take turns placing orbs on a grid. A cell that fills to its capacity (2 in a corner, 3 on an edge, 4 inside) explodes, sends one orb to each neighbour and takes those neighbours over, and a full neighbour explodes in turn. One orb can set off a chain across the whole board. The last player with orbs left wins.

Orbitrix is written in Go with Ebitengine. One client runs on Windows, Android, macOS, Linux and in the browser, and every one of them can sit in the same room. Empty seats take one of twelve bots, from a random mover to a search bot with a trained integer network. Boards run from 3×3 to 40×40, and chains on the largest boards still resolve exactly and play back wave by wave.

There is no game server. The player who hosts runs the room in their own client, and the others connect to them: over WebSocket on the local network, or over WebRTC online. The one server, a small Docker image, serves the website and the signaling that introduces players to a host. No game traffic passes through it.

Every graphic is drawn in code and every sound is synthesised in code. The only embedded files are two OFL fonts.

## Features

- 2 to 12 players on boards from 3×3 to 40×40, with eighteen presets in portrait, square and landscape shapes
- Twelve bots, listed easiest to hardest: Jester, Mason, Warden, Gambler, Nomad, Raider, Spy, Prodigy, Tactician, Voyager, Hunter and Sovereign
- This Device: play alone against bots, or pass one phone or PC around
- Same Wi-Fi: native LAN rooms without internet, browser joining by QR, and browser rooms using invite/reply links
- Online: any player can host, a browser tab included; public rooms are listed, private ones take a six-letter code or a `/join/<CODE>` link, and the host lets each player in
- Rejoin after a dropped connection, with the seat held for 60 seconds
- Exact wave-by-wave chain playback that speeds up on long chains, with Space to skip to the end
- Pentatonic sound generated at startup: a note per orb, a chord per wave
- Light and dark themes, and a colorblind mode that adds a shape mark to each player's orbs and seat
- A landing page with downloads, the web game at `/play`, and a Docker image for self-hosting

## Quick start

With [Go](https://go.dev/dl/) 1.27.3 or newer:

```sh
git clone https://github.com/EagleStelle/orbitrix.git
cd orbitrix
go run ./cmd/orbitrix
```

The desktop client opens on Home. Press PLAY, then This Device, pick the seats and a board, and press Start.

To play the web build locally:

```sh
go tool mage web
go run ./cmd/orbitrix-server -web dist/web
```

Then open `http://localhost:1212/play`. With the server running, `ORBITRIX_SERVER=http://localhost:1212 go run ./cmd/orbitrix` gives the desktop client an Online page that hosts and joins through it.

Ready-made builds for Windows, Android, macOS and Linux are on the [releases page](https://github.com/EagleStelle/orbitrix/releases/latest).

## Technologies used

Game

- Go 1.27 and Ebitengine v2.10: `vector` shapes, `text/v2` with the Unbounded and Archivo fonts, `audio` playing PCM generated in code, `exp/textinput` for the device's own keyboard
- A pure Go rules engine and AI package with no platform imports, so every client and host resolves moves bit for bit alike
- `ebitenmobile` for the Android binding, wrapped by a Gradle project with one Java activity; `js/wasm` for the browser

Networking

- `github.com/pion/webrtc/v4` data channels between online players, with STUN and Cloudflare Realtime TURN as the relay
- `github.com/coder/websocket` for signaling and for Same Wi-Fi rooms
- A compact binary wire protocol with a version byte, the same on every transport
- UDP broadcast for finding Same Wi-Fi rooms
- `golang.org/x/time/rate` token buckets and an HMAC-signed proof of work guarding the signaling service

Build and deploy

- Mage (`go tool mage`), pinned in `go.mod` as a tool, so the same targets run from bash and PowerShell
- A multi-stage Dockerfile built with buildx for linux/amd64 and linux/arm64, ending in a scratch image of about 25 MB
- GitHub Actions for CI and tagged releases, and Dependabot for weekly dependency updates

## Quick installation

1. Install Go 1.27.3 or newer. For Android builds, also install JDK 17+ and the Android SDK with an NDK.
2. Run `go tool mage -l` to list the build targets, then build the ones you need, for example `go tool mage desktop web`.
3. Find the output in `dist/`.

[docs/installation.md](docs/installation.md) covers every target, the environment variables and how to check a build.

## Architecture (summary)

`core` holds the rules: flat arrays for counts and owners, precomputed neighbour and capacity tables, and a resolver that returns every wave of a chain so the client can animate it. `core/ai` builds the twelve bots on top, each a pure function of the position, a seed and a work budget. The Ebitengine client in `client` draws the board in four batched calls and replays waves from the old position. For multiplayer, the hosting client runs `server/hub`, the authoritative room engine, which talks to players only through a `Conn`: `loopback` for the host's own player, `ws` on the LAN and `rtc` online. Players send moves; the hub validates and resolves them and broadcasts each one with a state hash, and every client resolves it again to animate it. `server/signal` pairs hosts and joiners online and issues TURN credentials. See [docs/architecture.md](docs/architecture.md).

## Project structure

```
orbitrix/
├── core/            Game rules: board, moves, waves, win and turn order, seeded RNG, hashing
│   └── ai/          The twelve bots, shared search helpers and network weights
├── proto/           Binary wire protocol and room codes
├── client/          Ebitengine game: scenes, board rendering, input, audio, settings
│   ├── online/      Hosting and joining online rooms
│   ├── platform/    Storage, clipboard, keyboard and other per-platform code
│   └── fonts/       Unbounded and Archivo with their OFL licences
├── server/          Website: landing page, /play, /join, downloads, /signal
│   ├── hub/         The room engine every host runs
│   ├── transport/   loopback, ws and rtc connections for the hub
│   ├── signal/      Signaling service and its proof of work
│   └── lan/         Same Wi-Fi discovery, host and join
├── cmd/             orbitrix, orbitrix-server, aitourney and webdist commands
├── mobile/          ebitenmobile binding package
├── android/         Gradle project that wraps the Android library in an APK
├── web/             The game's loader page and the landing page
├── deploy/          Dockerfile, compose files and .env.example
└── magefile.go      Build targets
```

See [docs/structure.md](docs/structure.md) for every package and what belongs in it.

## Documentation

| Page | Contents |
|---|---|
| [Usage guide](docs/usage.md) | Playing locally, on Same Wi-Fi and online, the bots, settings and keys |
| [Maps and Map Editor](docs/maps.md) | Custom terrain, Links, Reservoirs, Switches, Gates, local maps and sharing |
| [Installation](docs/installation.md) | Toolchain, build targets and checking that a build works |
| [Deployment](docs/deployment.md) | Running the server, putting Cloudflare in front, and publishing releases |
| [Architecture](docs/architecture.md) | How a move travels from a tap to every screen, and how each layer is built |
| [Structure](docs/structure.md) | Every package and folder, and what belongs in each |
| [Technical decisions](docs/decisions.md) | Trade-offs and the reasons behind them |
| [API](docs/api.md) | HTTP routes, the signaling protocol, the game protocol and LAN discovery |
| [Testing](docs/testing.md) | Unit, property, fuzz, integration and performance tests, and the bot tournament |

## Support and contributing

Report bugs and ask questions in [GitHub Issues](https://github.com/EagleStelle/orbitrix/issues). Pull requests are welcome. Read [docs/architecture.md](docs/architecture.md) and [docs/testing.md](docs/testing.md) first, and run `go tool mage check` before opening one. A change to `proto.Version` or `signal.Version` makes older clients ask to update, so it needs a release of the clients and the server together.

## License

Orbitrix is licensed under the [GNU General Public License v3.0](LICENSE). The bundled Unbounded and Archivo fonts are under the SIL Open Font License ([client/fonts/OFL-Unbounded.txt](client/fonts/OFL-Unbounded.txt), [client/fonts/OFL-Archivo.txt](client/fonts/OFL-Archivo.txt)).
