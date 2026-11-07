# Testing

## Running the tests

```sh
go tool mage check                              # gofmt, go vet ./..., go test ./...
go test ./core/...                              # one package tree
go test -race ./server/... ./proto/ ./core/     # race detector (needs cgo)
```

CI runs `check`, then the race detector on the server, protocol and rules packages, then the desktop, image and Android builds. The race detector needs cgo, so on a Windows machine without a C compiler it runs only in CI.

Client tests run Ebitengine scenes headless. They need no window, so they run in CI like the others.

## What the tests cover

| Package | Tests |
|---|---|
| `core` | Table-driven rules cases (capacity, explosions, leftovers, ownership, immediate wins, turn order, passes), property tests over random games (orbs grow by exactly one per move, determinism, a win always ends the game, classic `Apply` allocates nothing), `FuzzGame`, snapshot validation, RNG output, and a pinned state hash for the empty 9×9 board |
| `core/ai` | Every strategy plays legal moves and is deterministic over full games, behaviour cases per bot, the integer network's forward pass, and benchmarks |
| `proto` | Encode and decode of every message, rejection of malformed input, and `FuzzDecode`, which checks that whatever decodes re-encodes to the same bytes (100% coverage) |
| `server/hub` | Lobby, ready, start, rematch, leave, rejoin tokens and the seat grace, kick, replace, skip, Swap and Remove, the turn timer, bot seats and the message limits |
| `server/signal` | The message order, proof of work, password checks, approvals and the pending cap, TURN credential limits and revocation, socket caps and the origin check. `TestOnlineGame` plays a whole game between two pion peers through signaling |
| `server` | Routes, precompressed files and cache headers; `TestFloodFromOneIP` floods `/signal` with wrong passwords from one IP and checks a correct host still gets through |
| `server/transport/*`, `server/lan` | Each `Conn`, keepalive pings, LAN announcements, hosting, HTTP room announcements and bundled browser routes |
| `client/online` | Signaling joins, manual WebRTC invite/reply joins, single-use invites and malformed or oversized link rejection |
| `client` | Layout across window sizes, QR dialogs, abandoned reply cleanup, Setup and seat changes, text fields, count badges, wave replay ending equal to the game, and whole rooms: `TestOnlineRoom`, `TestLANRoom` (a dropped joiner winning its seat back), `TestDirectRoom` and `TestRoomSetup` driving the room page through clicks |
| `cmd/webdist` | The drawn favicon and generated README/Android logo consistency |
| `internal/appicon` | Logo pixels and symmetry, macOS ICNS sizes, Windows icon resources linked into a real executable |

## Benchmarks

The rules engine and every bot are benchmarked on 9×9, 20×20 and 40×40 boards:

```sh
go test -run '^$' -bench . ./core/       # BenchmarkApply, BenchmarkSimulate, BenchmarkMidgame
go test -run '^$' -bench . ./core/ai/    # BenchmarkChoose per strategy and size, BenchmarkProdigyForward
```

## Frame-rate check

A 40×40 board with twelve bots in long chains, measured for a minute:

```sh
ORBITRIX_PERF_SECONDS=60 go test -tags perf -run TestPerf -v ./client
```

A case fails when its 99th-percentile frame gap is over 25 ms, and an idle screen fails when it redraws more than about twice a second. `ORBITRIX_PERF_MARKS=1` turns colorblind marks on, and `ORBITRIX_PERF_NOVSYNC=1` measures headroom without vsync. It opens a window, so it doesn't run in CI.

## Bot tournament and matches

The tournament plays every pair of bots and prints win rates and move times:

```sh
go run ./cmd/aitourney -games 20 -size 9x9      # -seed and -workers also available
```

`TestMatches` plays one bot against every other on the benchmark boards, with random first placements. `ORBITRIX_MATCH_BOT` picks the bot (default Prodigy); `ORBITRIX_MATCH_SIZES`, `ORBITRIX_MATCH_FOES`, `ORBITRIX_MATCH_DUELS=0` and `ORBITRIX_MATCH_WEB=1` narrow the run.

```sh
ORBITRIX_MATCH_BOT=sovereign ORBITRIX_MATCH_GAMES=100 go test -tags train -run TestMatches -timeout 0 -v ./core/ai
```

## Training the learned bots

Both networks are stored as generated Go source, and training rewrites that file only when the new weights win.

Prodigy trains by self-play and by games against Tactician, Hunter and Voyager. It continues from the current weights and saves only when they beat the best score so far in duels against Hunter and Voyager and in four- and twelve-player tables. A generation takes about 3.5 minutes on a 16-thread desktop.

```sh
ORBITRIX_TRAIN_MINUTES=480 go test -tags train -run TestTrainProdigy -timeout 0 -v ./core/ai
```

Sovereign trains supervised, NNUE style. Each iteration plays games on 9×9, 20×20 and 40×40 with Hunter, Voyager, Tactician, Prodigy and Sovereign seats, labels the positions with the results and Sovereign's own deeper search, trains a fresh network on every position collected so far, and saves it only when it wins at least 55% of the decided games against the current one. Positions accumulate in `%TEMP%\orbitrix-sovereign` (or `ORBITRIX_SOV_DATA`), so a new run continues from all earlier data. An iteration of the default 960 games takes about 1.5 hours; the other `ORBITRIX_SOV_*` settings are listed at the top of `core/ai/sovereign_train_test.go`.

```sh
ORBITRIX_TRAIN_MINUTES=480 go test -tags train -run TestTrainSovereign -timeout 0 -v ./core/ai
```

In PowerShell, set each variable first, for example `$env:ORBITRIX_TRAIN_MINUTES=480; go test -tags train -run TestTrainSovereign -timeout 0 -v ./core/ai`.
