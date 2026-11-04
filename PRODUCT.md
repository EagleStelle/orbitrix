# Orbitrix

A turn-based chain-reaction orb game for 2–12 players, written in Go with Ebitengine. Targets Windows, Android and
the web (WASM), plus macOS and Linux, with online and LAN games hosted by one of the players. No native iOS build:
iPhones play in the browser.

<!-- impeccable:product-schema 1 -->

## Platform

web

One custom-drawn game look on every target. The UI never uses web technology: no HTML, CSS or JavaScript UI, on
any platform including the web build. Ebitengine draws all of it in Go (no DOM, no native widgets), so the value
above means "not a native OS design language", not "HTML/CSS". The only HTML is `web/index.html`, the bootstrap
page that loads `game.wasm` and shows its loading progress, and `web/landing.html`, the website's landing page at `/`;
neither is game UI. Platform needs still apply where they
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
- LAN games (Same Wi-Fi): one Windows or Android device hosts and the others, apps on the same network, find it in
  the list of rooms they hear (public) or by its code (private). Browsers don't play on the LAN.
- Online games hosted from any player's client, a browser tab included, by a player who has the server's host
  password. A public room is listed for anyone; a private one is reached by its room code or `/join/<CODE>` link,
  shared privately. Either way the host approves each player, and can kick. Players connect to the host
  over WebRTC, through a Cloudflare TURN relay when their networks block a direct connection. The Docker container
  only serves the website and signaling; no game traffic passes through it.
- Codes and passwords are typed on the device's own keyboard: a physical one, or the on-screen keyboard Android
  and phone browsers show.

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
  (diamond), Charcoal/Chalk (stem), Violet (pair), Brown (ring), Plum/Mint (chevron), Maroon (frame).
- Type: Unbounded for display, Archivo for UI.
- Orb motion (idle orbit, frantic before exploding, per-cell variation) is ported exactly from the prototype and
  stays binding; only how orbs look follows the design system. Board cells are rounded square tiles with gutters.
- Labels stay plain and short ("Play", "Start", "Restart", "Settings", "Main menu", "Play again"). No key hints on
  buttons and no explanatory subtext.

## Evidence on Hand

- `prototype/main.py`: the pygame prototype, used as reference for rules, AI and orb motion only (its UI was
  replaced). It is gitignored in this repo; the original lives outside the repo.
- `go run ./cmd/aitourney`: AI-versus-AI win rates and move times.
- Design choices and AI budget sizing: `docs/decisions.md`.
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
| [Android SDK](https://developer.android.com/studio) + NDK | Android | `go tool mage android` uses `ANDROID_HOME`, or `%LOCALAPPDATA%\Android\Sdk` / `~/Android/Sdk` when it isn't set. Install an NDK with the SDK manager (`sdkmanager "ndk;30.0.16248370"`). |
| Docker | `image` target, server deployment | Builds and runs the website and signaling server image. |

Desktop builds (Windows, macOS, Linux) are pure Go (`CGO_ENABLED=0`) and cross-compile from any OS. Only the Android
binding needs cgo, through `ebitenmobile`. Both `ebitenmobile` and the build tool, [Mage](https://magefile.org), are pinned in `go.mod` as tools,
so `go tool` runs them with nothing to install.

## Building

The targets live in `magefile.go` and run the same from any shell. Output goes to `dist/`.

```sh
go tool mage <target>      # go tool mage -l lists the targets
```

| Target | Output |
|---|---|
| `check` | gofmt, `go vet ./...`, `go test ./...` |
| `desktop` | `dist/orbitrix.exe`, the Windows client, from any OS |
| `mac` | `dist/orbitrix-mac.zip`: `Orbitrix.app` with one universal binary (Apple silicon + Intel), from any OS |
| `linux` | `dist/orbitrix-linux.zip`: the amd64 Linux client, from any OS. It needs X11 or XWayland, OpenGL and ALSA or PulseAudio at run time; Copy uses `wl-copy` or `xclip` when one is installed |
| `web` | `dist/web/`: `game.wasm`, `wasm_exec.js`, the wordmark font and `favicon.svg` (the logo) under content-hashed names, `index.html` (the game's loader) and `landing.html`; each served file kept only as `.gz` and `.br` copies (`cmd/webdist`; the server unzips the `.gz` for a client that takes neither) |
| `android` | `dist/orbitrix.aar` (the Go game, arm64 + x86_64) and `dist/orbitrix.apk` (installable) |
| `image` | the `eaglestelle/orbitrix-server` Docker image for linux/amd64 and linux/arm64, compiled inside Docker by `deploy/Dockerfile` (the web build plus a Linux server), labelled with its commit and build time |
| `all` | every target above, in that order |

Native clients (`desktop`, `mac`, `linux`, `android`) play online through the server named in `ORBITRIX_SERVER` when they are built
(e.g. `https://orbitrix.example.com`); without it they offer no Online. The web build always uses the server it was
loaded from. For testing, the desktop client also reads `ORBITRIX_SERVER` when it starts, so
`ORBITRIX_SERVER=http://localhost:1212 go run ./cmd/orbitrix` plays against a local server.

Run the desktop client directly with `go run ./cmd/orbitrix`. F3 toggles the frame-rate overlay (`-fps` starts
with it on).

Play the web build locally after a `web` build: `go run ./cmd/orbitrix-server -web dist/web`, then open
`http://localhost:1212` (the landing page) or `http://localhost:1212/play` (the game). The server sends the
precompressed copies to browsers that accept them. It reads its settings from the environment:

| Variable | Meaning |
|---|---|
| `LISTEN` | Listen address, default `:1212`. |
| `PUBLIC_ORIGIN` | The site's public address (`https://…`); browsers may open `/signal` only from it. |
| `HOST_PASSWORD` | Empty: anyone can host online, without a password, using STUN only. Set: hosts must enter it. Random, at least 20 characters. |
| `CF_TURN_KEY_ID`, `CF_TURN_API_TOKEN` | Cloudflare Realtime TURN key. TURN needs both and a password of at least 20 characters; otherwise, or when Cloudflare fails, it falls back to STUN. |

`-downloads <folder>` offers `orbitrix.exe`, `orbitrix.apk`, `orbitrix-mac.zip` and `orbitrix-linux.zip` from that
folder on the landing page, each once it is there.

The `eaglestelle/orbitrix-server` image is built in two stages: a Go stage compiles the web build and the server, and
the final image, from scratch, holds only those and the CA roots (about 25 MB per platform). `go tool mage image` builds it with buildx for linux/amd64 and
linux/arm64 at once, natively on the build machine, and loads both into Docker, which takes Docker's containerd image
store (Docker Desktop's default). On a machine without Go: `docker buildx build --platform linux/amd64,linux/arm64
-t eaglestelle/orbitrix-server -f deploy/Dockerfile --load .` from the repo root, which leaves the commit and
build-time labels empty. [Deploying](#deploying) puts it on a server. To try the image locally with hosting open (no
password), at `http://localhost:1212`:

```sh
cd deploy && docker compose -f docker-compose.yml -f compose.dev.yml up
```

Install the Android build on a connected phone or emulator with `adb install -r dist/orbitrix.apk`. Without a release
key (see [Publishing the downloads](#7-publish-the-downloads)) it is signed with the debug key, which is fine for
testing but not for publishing.

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

## Deploying

From a fresh server to a public site, here `https://orbitrix.example.com`. The container serves the website and
signaling only; games run between the players. Cloudflare's free plan covers everything except TURN egress past
1,000 GB a month (see step 6). Cloudflare renames dashboard menus now and then: where a path below has moved, search
the dashboard for the setting's name.

### 1. Server

Any Linux machine with Docker Engine and its Compose plugin, amd64 or arm64. Make a folder for Orbitrix and copy
`deploy/docker-compose.yml` and `deploy/.env.example` into it:

```sh
mkdir -p ~/orbitrix/downloads && cd ~/orbitrix
cp .env.example .env && chmod 600 .env
```

Get the image onto the server. Either push it from the build machine (`docker login`, then
`docker push eaglestelle/orbitrix-server` sends both platforms) and run `docker compose pull` on the server, or send
only the server's platform: `docker save --platform linux/arm64 eaglestelle/orbitrix-server | ssh <server> docker load`
(`linux/amd64` for an x86 server).

### 2. Secrets (`.env`)

| Variable | Value |
|---|---|
| `PUBLIC_ORIGIN` | `https://orbitrix.example.com`. Browsers may open `/signal` only from this site. |
| `HOST_PASSWORD` | A random password of at least 20 characters from a password manager. Hosts type it once; their client saves it. |
| `CF_TURN_KEY_ID`, `CF_TURN_API_TOKEN` | Cloudflare dashboard → Realtime → TURN Server → Create: the key's ID and its API token. |

TURN needs all three: without the key, or with a shorter password, the server falls back to STUN, which costs nothing
but leaves some players behind CGNAT unable to connect. Never put these values in a client build or a log.

```sh
docker compose up -d
docker compose logs     # no "HOST_PASSWORD" or "TURN" warning should appear
curl http://127.0.0.1:1212/healthz     # ok
```

### 3. Cloudflare in front

1. Add the domain to Cloudflare (free plan) and move its nameservers there.
2. Reach the server through a Cloudflare Tunnel: Zero Trust → Networks → Tunnels → Create a tunnel (Cloudflared),
   install the connector on the server with the command the dashboard shows, then add a public hostname
   `orbitrix.example.com` → service `HTTP`, `localhost:1212`. The tunnel connects outwards, so the server needs no open
   port, and nothing reaches Orbitrix except through Cloudflare. If cloudflared runs in a container, give it host
   networking: the compose port listens on the host's `127.0.0.1` only. A reverse proxy works instead, as long as it
   accepts connections only from [Cloudflare's IP ranges](https://www.cloudflare.com/ips/) and passes
   `CF-Connecting-IP` on: the server believes that header from any private or loopback peer, so a request that went
   around Cloudflare could name any address and slip every per-IP limit.
3. SSL/TLS → Edge Certificates: Always Use HTTPS on. Network: WebSockets on (the default).
4. Caching → Configuration: Browser Cache TTL "Respect Existing Headers", so the site's own cache headers reach
   browsers.

Open `https://orbitrix.example.com/play`: the game loads.

### 4. Cache Rules

Caching → Cache Rules. Cloudflare caches `.js`, `.ttf`, `.svg`, `.exe`, `.apk` and `.zip` on its own but not `.wasm` or pages. With
these two rules a flood on the website is answered at the edge and barely reaches the server.

| Rule | When incoming requests match (Edit expression) | Then |
|---|---|---|
| Game files | `ends_with(http.request.uri.path, ".wasm") or ends_with(http.request.uri.path, ".js") or ends_with(http.request.uri.path, ".ttf") or ends_with(http.request.uri.path, ".svg")` | Eligible for cache. Edge TTL: use cache-control header if present (the files are content-hashed and marked immutable for a year). |
| Pages | `http.request.uri.path in {"/" "/play"} or starts_with(http.request.uri.path, "/join/")` | Eligible for cache. Edge TTL: ignore cache-control header, 1 minute. Browser TTL: respect origin. |

Check: request the `game.<hash>.wasm` named in `/play`'s source twice; the second answer has `cf-cache-status: HIT`.

### 5. Security rules

Security → WAF → Custom rules (the free plan allows five). Two rules, both with the action **Block**:

**Site only.** Lets through only the site's own paths, GET and HEAD, `/signal` only as a WebSocket upgrade, and no
query strings outside `/signal`, so cache-busting URLs never reach the server:

```
not (http.request.method in {"GET" "HEAD"})
or (http.request.uri.query ne "" and http.request.uri.path ne "/signal")
or (http.request.uri.path eq "/signal" and not any(lower(http.request.headers["upgrade"][*])[*] eq "websocket"))
or not (
  http.request.uri.path in {"/" "/play" "/signal" "/healthz" "/download/orbitrix.exe" "/download/orbitrix.apk"
    "/download/orbitrix-mac.zip" "/download/orbitrix-linux.zip"}
  or starts_with(http.request.uri.path, "/join/")
  or ends_with(http.request.uri.path, ".wasm")
  or ends_with(http.request.uri.path, ".js")
  or ends_with(http.request.uri.path, ".ttf")
  or ends_with(http.request.uri.path, ".svg")
)
```

A web build that ever adds another kind of file needs its extension in this rule and in the Game files cache rule.

**No Tor or datacenters on signaling.** Blocks Tor and the large clouds (AWS, Google, Azure, Hetzner, OVH,
DigitalOcean, Linode, Vultr) from `/signal` only; the website stays open to everyone. Cloudflare's own network (13335)
stays allowed, because WARP and iCloud Private Relay users come from it. A friend on a VPN hosted in one of these
clouds can't reach `/signal` while the rule is on; switch it off in the dashboard if that matters.

```
http.request.uri.path eq "/signal" and (ip.src.continent eq "T1"
  or ip.src.asnum in {16509 14618 15169 396982 8075 24940 213230 16276 14061 63949 20473})
```

Security → WAF → Rate limiting rules (one on the free plan): **Signal connections**, when URI Path equals `/signal`,
counted per IP, 30 requests per 10 seconds, action Block for 10 seconds. A real client opens one connection per
session, plus reconnects with backoff.

Leave these off: Security → Bots → **Bot Fight Mode** (it can't skip one path and would challenge the native clients'
`/signal` connections) and **I'm Under Attack** mode (native clients can't answer its challenge). No Turnstile or
CAPTCHA anywhere, for the same reason.

### 6. Billing alert

TURN is the only metered service: 1,000 GB of egress a month free (shared with Cloudflare's SFU), then $0.05/GB, with
no hard spend cap. Manage Account → Billing → Budget alerts: set a small threshold (e.g. $1). Realtime → TURN Server
shows the egress used. The server already limits who can get TURN credentials (the host password, approved joiners
only, at most one per human seat plus four rejoins per room, 6-hour lifetime, revoked when a seat or room closes).

### 7. Publish the downloads

Native clients play online through the server stamped into them at build time, and Android only updates an app
signed with the same key, so published builds need both.

Create the release key once, with the JDK's `keytool`, and keep the file and its password in the password manager. A
lost key means every phone has to uninstall Orbitrix (losing its settings) to take the next build.

```sh
keytool -genkeypair -keystore orbitrix-release.p12 -storetype PKCS12 -alias orbitrix -keyalg RSA -keysize 4096 -validity 10000 -dname "CN=Orbitrix"
```

Build the clients with the key and the site's address, then copy them into the server's `downloads/` folder. The
landing page offers each file once it is there; Cloudflare keeps serving the old one for up to 5 minutes (Caching →
Purge Cache to skip that).

```sh
ORBITRIX_SERVER=https://orbitrix.example.com ORBITRIX_KEYSTORE=/path/to/orbitrix-release.p12 ORBITRIX_KEYSTORE_PASSWORD=… go tool mage desktop mac linux android
scp dist/orbitrix.exe dist/orbitrix.apk dist/orbitrix-mac.zip dist/orbitrix-linux.zip <server>:orbitrix/downloads/
```

```powershell
$env:ORBITRIX_SERVER="https://orbitrix.example.com"; $env:ORBITRIX_KEYSTORE="C:\path\to\orbitrix-release.p12"; $env:ORBITRIX_KEYSTORE_PASSWORD="…"; go tool mage desktop mac linux android
scp dist/orbitrix.exe dist/orbitrix.apk dist/orbitrix-mac.zip dist/orbitrix-linux.zip <server>:orbitrix/downloads/
```

A phone holding a debug-signed build must uninstall it once before installing the first release-signed one. Windows
SmartScreen warns about the unsigned `.exe` until it has been downloaded often enough: More info → Run anyway. The Mac
app isn't signed with an Apple Developer ID or notarized, so macOS refuses its first open: System Settings → Privacy &
Security → Open Anyway (once). If macOS calls it damaged instead, run `xattr -dr com.apple.quarantine
/Applications/Orbitrix.app` once. On Linux, unzip and run `./orbitrix`.

### Updating

- **New server version:** get the new image onto the server (step 1), then `docker compose up -d`. Rooms live in
  memory, so they are lost, but running games go on over WebRTC and hosts open a new room on their own. Purge
  Cloudflare's cache afterwards, so no cached page points at old game files.
- **New protocol version:** when `proto.Version` or `signal.Version` changes, older native clients say "Update
  Orbitrix" instead of joining; publish new downloads (step 7) along with the server.
- **Rotating the host password:** change `HOST_PASSWORD` in `.env` and run `docker compose up -d`. Hosts are asked for
  the new one the next time they open a room.

### Production checklist

Every abuse and cost guard, and how to see it working. Most are built into the server; the rest are the steps above.

| Guard | Check |
|---|---|
| Port on 127.0.0.1 only | `docker compose ps` shows `127.0.0.1:1212->1212`; `curl http://<server-lan-ip>:1212/healthz` from another machine fails. |
| Only Cloudflare reaches the server | Tunnel (or proxy allowlist) as in step 3. |
| Host password | `docker compose logs` has no `HOST_PASSWORD` warning. Hosting from a fresh client asks for it, and asks again after a wrong one. |
| TURN, and STUN fallback | Settings → Network → Force relay on two devices on different networks (a phone on mobile data and a PC), then play an online game. |
| TURN credential limits, 50 rooms, 3 pending approvals, gone hosts' rooms closed | Built in. A room's code dies when its host leaves. |
| Origin check | `PUBLIC_ORIGIN` is exactly the site's address. |
| Edge cache | Step 4's check: `cf-cache-status: HIT`. |
| Site-only rule | `curl -X POST https://orbitrix.example.com/`, `curl "https://orbitrix.example.com/play?x=1"`, `curl https://orbitrix.example.com/signal` and `curl https://orbitrix.example.com/nope` each answer 403 from Cloudflare. |
| Signal rate limit | `for i in $(seq 40); do curl -s -o /dev/null -w "%{http_code} " -H "Upgrade: websocket" https://orbitrix.example.com/signal; done` ends in 429s. |
| Tor and datacenter rule | Security → Events lists its blocks. |
| Proof of work, socket caps (2,000, 100 per IP), message limits, 5-second first message | Built in; they rise or apply only under a flood. |
| Bot Fight Mode and I'm Under Attack off | Step 5. |
| Budget alert | Step 6. |
| No secrets in logs | `docker compose logs` names no password, room secret, rejoin token or TURN credential. |
| Real phone | Install the APK from the landing page on a real Android phone. Play a local game; a Same Wi-Fi game with the PC (Windows asks to let Orbitrix on the network once); an online game on mobile data, joining and hosting, with Force relay on and off; and the web game at `/play` in the phone's browser. |

## Layout

| Path | Contents |
|---|---|
| `core/` | Pure game rules (board, moves, wave resolution, win/turn order, seeded RNG). No Ebitengine, net or OS imports. |
| `core/ai/` | The twelve AI personalities, and Prodigy's and Sovereign's training and match harness (build tag `train`). |
| `cmd/aitourney/` | Headless AI-vs-AI tournament. |
| `cmd/webdist/` | Finishes a web build: loading-progress size, the logo as `favicon.svg` and precompressed copies. |
| `proto/` | Binary wire protocol. |
| `client/` | Ebitengine game: scenes, rendering, input, audio, settings. `client/fonts/` holds the two OFL fonts and their licences. |
| `client/online/` | A client's side of an online room: hosting (signaling, the hub, a WebRTC channel per joiner, reconnects) and joining. |
| `cmd/orbitrix/` | Desktop and web entry point. |
| `mobile/`, `android/` | `ebitenmobile` binding and the Gradle project that wraps it (one activity in Java). |
| `web/` | The game's loader page (`index.html`) and the landing page (`landing.html`). |
| `server/`, `cmd/orbitrix-server/` | Website server: landing page, `/play`, `/join/<code>`, downloads, `/signal`. |
| `server/hub/` | The room engine every host runs, online and on LAN: lobby, seats, rejoin, the authoritative game. |
| `server/transport/` | The hub's connections: `ws` (LAN), `rtc` (WebRTC data channel, online), `loopback` (the host's own player). |
| `server/signal/` | Signaling: rooms, join approval, SDP relay, TURN credentials, abuse guards; `pow/` is its proof of work. |
| `server/lan/` | Same Wi-Fi rooms: UDP discovery, the LAN host (hub behind `ws` on private networks), joining. |
| `deploy/` | Multi-stage Dockerfile, docker-compose, local override, `.env.example`. |
| `magefile.go` | Build targets (`go tool mage -l`). |
| `docs/decisions.md` | Choices made where the brief left room. |
