# Orbitrix

A turn-based chain-reaction orb game for 2–12 players, written in Go with Ebitengine. Targets Windows, Android and
the web (WASM), with an optional online server and LAN play.

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

Run the desktop client directly with `go run ./cmd/orbitrix`.

## Layout

| Path | Contents |
|---|---|
| `core/` | Pure game rules (board, moves, wave resolution, win/turn order, seeded RNG). No Ebitengine, net or OS imports. |
| `core/ai/` | The eight AI personalities. |
| `proto/` | Binary wire protocol. |
| `client/` | Ebitengine game: scenes, rendering, input, audio, settings. |
| `cmd/orbitrix/` | Desktop and web entry point. |
| `mobile/`, `android/` | `ebitenmobile` binding and the Gradle wrapper project. |
| `web/` | Page that loads `game.wasm`. |
| `server/`, `cmd/orbitrix-server/` | Online server; `server/hub` is the shared room engine, `server/lan` the LAN host. |
| `deploy/` | Dockerfile, docker-compose, Cloudflare Tunnel config. |
| `docs/decisions.md` | Choices made where the brief left room. |
