# Deployment

From a fresh server to a public site, here `https://orbitrix.example.com`. The container serves the website and signaling only; games run between the players. Cloudflare's free plan covers everything except TURN egress past 1,000 GB a month (step 6). Cloudflare renames dashboard menus now and then: where a path below has moved, search the dashboard for the setting's name.

## 1. Server

Any Linux machine with Docker Engine and its Compose plugin, amd64 or arm64. Make a folder for Orbitrix and copy `deploy/docker-compose.yml` and `deploy/.env.example` into it:

```sh
mkdir -p ~/orbitrix/downloads && cd ~/orbitrix
cp .env.example .env && chmod 600 .env
docker compose pull
```

The `eaglestelle/orbitrix-server` image on Docker Hub is published for both platforms with every release. To run an image you built yourself, push it from the build machine with `go tool mage push`, or send only the server's platform: `docker save --platform linux/arm64 eaglestelle/orbitrix-server | ssh <server> docker load` (`linux/amd64` for an x86 server).

Compose runs the image read-only, with no capabilities, and publishes port 1212 on `127.0.0.1` only.

## 2. Secrets (`.env`)

| Variable | Value |
|---|---|
| `PUBLIC_ORIGIN` | `https://orbitrix.example.com`. Browsers may open `/signal` only from this site. |
| `HOST_PASSWORD` | A random password of at least 20 characters from a password manager. Hosts type it once and their client saves it. Empty: anyone may host, with STUN only and at most 3 rooms per IP. |
| `CF_TURN_KEY_ID`, `CF_TURN_API_TOKEN` | Cloudflare dashboard > Realtime > TURN Server > Create: the key's ID and its API token. |
| `LISTEN` | Listen address inside the container, default `:1212`. |

TURN needs all three of the password, the key ID and the token. Without the key, or with a shorter password, the server falls back to STUN, which costs nothing but leaves some players behind CGNAT unable to connect. Never put these values in a client build or a log.

```sh
docker compose up -d
docker compose logs                    # no "HOST_PASSWORD" or "TURN" warning should appear
curl http://127.0.0.1:1212/healthz     # ok
```

## 3. Cloudflare in front

1. Add the domain to Cloudflare (free plan) and move its nameservers there.
2. Reach the server through a Cloudflare Tunnel: Zero Trust > Networks > Tunnels > Create a tunnel (Cloudflared), install the connector on the server with the command the dashboard shows, then add a public hostname `orbitrix.example.com` pointing at service `HTTP`, `localhost:1212`. The tunnel connects outwards, so the server needs no open port and nothing reaches Orbitrix except through Cloudflare. If cloudflared runs in a container, give it host networking, because the compose port listens on the host's `127.0.0.1` only.

   A reverse proxy works instead, as long as it accepts connections only from [Cloudflare's IP ranges](https://www.cloudflare.com/ips/) and passes `CF-Connecting-IP` on. The server believes that header from any private or loopback peer, so a request that went around Cloudflare could name any address and slip every per-IP limit.
3. SSL/TLS > Edge Certificates: Always Use HTTPS on. Network: WebSockets on (the default).
4. Caching > Configuration: Browser Cache TTL "Respect Existing Headers", so the site's own cache headers reach browsers.

Open `https://orbitrix.example.com/play`: the game loads.

## 4. Cache rules

Caching > Cache Rules. Cloudflare caches `.js`, `.ttf`, `.svg`, `.exe`, `.apk` and `.zip` on its own but not `.wasm` or pages. With these two rules a flood on the website is answered at the edge and barely reaches the server.

| Rule | When incoming requests match (Edit expression) | Then |
|---|---|---|
| Game files | `ends_with(http.request.uri.path, ".wasm") or ends_with(http.request.uri.path, ".js") or ends_with(http.request.uri.path, ".ttf") or ends_with(http.request.uri.path, ".svg")` | Eligible for cache. Edge TTL: use cache-control header if present (the files are content-hashed and marked immutable for a year). |
| Pages | `http.request.uri.path in {"/" "/play"} or starts_with(http.request.uri.path, "/join/")` | Eligible for cache. Edge TTL: ignore cache-control header, 1 minute. Browser TTL: respect origin. |

Check: request the `game.<hash>.wasm` named in `/play`'s source twice. The second answer has `cf-cache-status: HIT`.

## 5. Security rules

Security > WAF > Custom rules (the free plan allows five). Two rules, both with the action Block.

Site only. Lets through only the site's own paths, GET and HEAD, `/signal` only as a WebSocket upgrade, and no query strings outside `/signal`, so cache-busting URLs never reach the server:

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

A web build that adds another kind of file needs its extension in this rule and in the Game files cache rule.

No Tor or datacenters on signaling. Blocks Tor and the large clouds (AWS, Google, Azure, Hetzner, OVH, DigitalOcean, Linode, Vultr) from `/signal` only; the website stays open to everyone. Cloudflare's own network (13335) stays allowed, because WARP and iCloud Private Relay users come from it. A friend on a VPN hosted in one of these clouds can't reach `/signal` while the rule is on; switch it off in the dashboard if that matters.

```
http.request.uri.path eq "/signal" and (ip.src.continent eq "T1"
  or ip.src.asnum in {16509 14618 15169 396982 8075 24940 213230 16276 14061 63949 20473})
```

Security > WAF > Rate limiting rules (one on the free plan): Signal connections, when URI Path equals `/signal`, counted per IP, 30 requests per 10 seconds, action Block for 10 seconds. A real client opens one connection per session, plus reconnects with backoff.

Leave Security > Bots > Bot Fight Mode off: it can't skip one path and would challenge the native clients' `/signal` connections. Leave I'm Under Attack mode off too, since native clients can't answer its challenge. For the same reason there is no Turnstile or CAPTCHA anywhere.

## 6. Billing alert

TURN is the only metered service: 1,000 GB of egress a month free (shared with Cloudflare's SFU), then $0.05/GB, with no hard spend cap. Under Manage Account > Billing > Budget alerts, set a small threshold such as $1. Realtime > TURN Server shows the egress used. The server already limits who gets TURN credentials: the host password, approved joiners only, at most one per human seat plus four rejoins per room, a 6-hour lifetime, and revocation when a seat or room closes.

## 7. Publish the downloads

Native clients play online through the server stamped into them at build time, and Android only updates an app signed with the same key, so published builds need both. The [release workflow](#releases) builds them that way. Copy the files from the release into the server's `downloads/` folder:

```sh
cd ~/orbitrix/downloads
for f in orbitrix.exe orbitrix.apk orbitrix-mac.zip orbitrix-linux.zip; do
  curl -fLO "https://github.com/EagleStelle/orbitrix/releases/latest/download/$f"
done
```

The landing page offers each file once it is there. Cloudflare keeps serving the old one for up to 5 minutes; Caching > Purge Cache skips that.

To build them by hand instead (bash, then PowerShell):

```sh
ORBITRIX_SERVER=https://orbitrix.example.com ORBITRIX_KEYSTORE=/path/to/orbitrix-release.p12 ORBITRIX_KEYSTORE_PASSWORD=… go tool mage desktop mac linux android
scp dist/orbitrix.exe dist/orbitrix.apk dist/orbitrix-mac.zip dist/orbitrix-linux.zip <server>:orbitrix/downloads/
```

```powershell
$env:ORBITRIX_SERVER="https://orbitrix.example.com"; $env:ORBITRIX_KEYSTORE="C:\path\to\orbitrix-release.p12"; $env:ORBITRIX_KEYSTORE_PASSWORD="…"; go tool mage desktop mac linux android
scp dist/orbitrix.exe dist/orbitrix.apk dist/orbitrix-mac.zip dist/orbitrix-linux.zip <server>:orbitrix/downloads/
```

## Releases

Pushing a tag such as `v1.0.0` runs `.github/workflows/release.yml`:

1. The whole CI workflow runs first.
2. The tag must match `version` in `magefile.go` and `versionName` in `android/build.gradle.kts`.
3. The Windows, macOS, Linux and Android clients are built with the release key and `ORBITRIX_SERVER`, and published as a GitHub release with `SHA256SUMS.txt`. The release notes are the Full Changelog link to the previous tag.
4. The server image is pushed to Docker Hub as `eaglestelle/orbitrix-server:<version>` and `:latest`.

The repository needs these settings (Settings > Secrets and variables > Actions):

| Name | Kind | Value |
|---|---|---|
| `ORBITRIX_KEYSTORE_BASE64` | secret | The release keystore, base64 (`base64 -w0 orbitrix-release.p12`) |
| `ORBITRIX_KEYSTORE_PASSWORD` | secret | Its password |
| `DOCKERHUB_USERNAME` | secret | The Docker Hub account that owns `eaglestelle/orbitrix-server` |
| `DOCKERHUB_TOKEN` | secret | A Docker Hub access token with read and write access |
| `ORBITRIX_SERVER` | variable | `https://orbitrix.example.com` |

A missing value stops the release before anything is published. To cut a release, raise `version` in `magefile.go`, `versionName` and `versionCode` in `android/build.gradle.kts`, then:

```sh
git tag v1.0.1
git push origin v1.0.1
```

## Updating

- New server version: `docker compose pull && docker compose up -d`. Rooms live in memory, so they are lost, but running games go on over WebRTC and hosts open a new room on their own. Purge Cloudflare's cache afterwards, so no cached page points at old game files.
- New protocol version: when `proto.Version` or `signal.Version` changes, older native clients say "Update Orbitrix" instead of joining. Publish new downloads (step 7) along with the server.
- Rotating the host password: change `HOST_PASSWORD` in `.env` and run `docker compose up -d`. Hosts are asked for the new one the next time they open a room.

## Production checklist

Every abuse and cost guard, and how to see it working. Most are built into the server; the rest are the steps above.

| Guard | Check |
|---|---|
| Port on 127.0.0.1 only | `docker compose ps` shows `127.0.0.1:1212->1212`; `curl http://<server-lan-ip>:1212/healthz` from another machine fails. |
| Only Cloudflare reaches the server | Tunnel, or a proxy allowlist, as in step 3. |
| Host password | `docker compose logs` has no `HOST_PASSWORD` warning. Hosting from a fresh client asks for it, and asks again after a wrong one. |
| TURN, and STUN fallback | Settings > Network > Force relay on two devices on different networks (a phone on mobile data and a PC), then play an online game. |
| TURN credential limits, 50 rooms, 3 pending approvals, gone hosts' rooms closed | Built in. A room's code dies when its host leaves. |
| Origin check | `PUBLIC_ORIGIN` is exactly the site's address. |
| Edge cache | Step 4's check: `cf-cache-status: HIT`. |
| Site-only rule | `curl -X POST https://orbitrix.example.com/`, `curl "https://orbitrix.example.com/play?x=1"`, `curl https://orbitrix.example.com/signal` and `curl https://orbitrix.example.com/nope` each answer 403 from Cloudflare. |
| Signal rate limit | `for i in $(seq 40); do curl -s -o /dev/null -w "%{http_code} " -H "Upgrade: websocket" https://orbitrix.example.com/signal; done` ends in 429s. |
| Tor and datacenter rule | Security > Events lists its blocks. |
| Proof of work, socket caps (2,000, 100 per IP), message limits, 5-second first message | Built in; they rise or apply only under a flood. |
| Bot Fight Mode and I'm Under Attack off | Step 5. |
| Budget alert | Step 6. |
| No secrets in logs | `docker compose logs` names no password, room secret, rejoin token or TURN credential. |
| Real phone | Install the APK from the landing page on a real Android phone. Play a local game; a Same Wi-Fi game with a PC; an online game on mobile data, joining and hosting, with Force relay on and off; and the web game at `/play` in the phone's browser. |
