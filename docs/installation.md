# Installation

## Prerequisites

| Tool | Needed for | Notes |
|---|---|---|
| [Go](https://go.dev/dl/) 1.27.3+ | everything | The version is pinned in `go.mod`; an older toolchain downloads the right one itself (`GOTOOLCHAIN=auto`). |
| [JDK](https://adoptium.net/) 17+ | Android | Used by Gradle. The wrapper in `android/` fetches Gradle 9.8.2 and the Android Gradle Plugin. |
| [Android SDK](https://developer.android.com/studio) with an NDK | Android | Found through `ANDROID_HOME`, or `%LOCALAPPDATA%\Android\Sdk` / `~/Android/Sdk` when it is unset. Install the NDK with `sdkmanager "ndk;30.0.16248370"`. |
| Docker with buildx | the `image` and `push` targets, running the server | |
| git | the `image` and `push` targets | The image is labelled with the commit. |

Mage and `ebitenmobile` are pinned in `go.mod` as tools, so `go tool` runs them with nothing to install. The desktop builds (Windows, macOS, Linux) are pure Go with cgo off and cross-compile from any OS. Only the Android binding needs cgo.

## Steps

1. Clone the repository:

   ```sh
   git clone https://github.com/EagleStelle/orbitrix.git
   cd orbitrix
   ```

2. Run the desktop client straight from source:

   ```sh
   go run ./cmd/orbitrix        # -fps starts with the frame-rate overlay on (F3 toggles it)
   ```

3. Or build a target into `dist/`:

   ```sh
   go tool mage -l              # lists the targets
   go tool mage <target>
   ```

| Target | Output |
|---|---|
| `check` | Fails on unformatted files, then runs `go vet ./...` and `go test ./...` |
| `desktop` | `dist/orbitrix.exe`, the Windows client |
| `mac` | `dist/orbitrix-mac.zip`: `Orbitrix.app` with one universal binary for Apple silicon and Intel |
| `linux` | `dist/orbitrix-linux.zip`: the amd64 client. At run time it needs X11 or XWayland, OpenGL and ALSA or PulseAudio |
| `web` | `dist/web/`: `game.wasm`, `wasm_exec.js`, the wordmark font and `favicon.svg` under content-hashed names, the loader page and the landing page, each served file kept as `.gz` and `.br` copies |
| `android` | `dist/orbitrix.aar` (arm64 and x86_64) and `dist/orbitrix.apk` |
| `image` | The `eaglestelle/orbitrix-server` image for linux/amd64 and linux/arm64, tagged `latest` and with the version, loaded into Docker |
| `push` | The same image, pushed to Docker Hub (needs `docker login`) |
| `all` | Every target above except `push`, in that order |

### Environment variables

| Variable | Read by | Meaning |
|---|---|---|
| `ORBITRIX_SERVER` | `desktop`, `mac`, `linux`, `android` | The server the native clients play online through, such as `https://orbitrix.example.com`, stamped in at build time. Without it a build offers no Online page. The desktop client also reads it when it starts, for local testing. The web build always uses the server it was loaded from. |
| `ORBITRIX_KEYSTORE`, `ORBITRIX_KEYSTORE_PASSWORD` | `android` | A PKCS12 keystore holding the key `orbitrix`, and its password. Without them the APK is signed with the debug key, which is fine for testing but can't be updated by a release-signed APK. |

Create the release keystore once with the JDK's `keytool`, and keep the file and its password safe. A lost key means every phone must uninstall Orbitrix, losing its settings, before it can take the next build.

```sh
keytool -genkeypair -keystore orbitrix-release.p12 -storetype PKCS12 -alias orbitrix -keyalg RSA -keysize 4096 -validity 10000 -dname "CN=Orbitrix"
```

### Running the server locally

After a `web` build:

```sh
go run ./cmd/orbitrix-server -web dist/web
```

`http://localhost:1212` is the landing page and `http://localhost:1212/play` the game. Hosting online is open to anyone (no `HOST_PASSWORD`) and uses STUN only. The server's settings are listed in [deployment.md](deployment.md#2-secrets-env).

To try the Docker image instead, with hosting open, at `http://localhost:1212`:

```sh
go tool mage image
cd deploy && docker compose -f docker-compose.yml -f compose.dev.yml up
```

Loading a two-platform image into Docker needs its containerd image store, the default in Docker Desktop and in Docker Engine 29 and later. Without Go, build it from the repository root with `docker buildx build --platform linux/amd64,linux/arm64 -t eaglestelle/orbitrix-server -f deploy/Dockerfile --load .`, which leaves the version, commit and build-time labels empty.

### Android

```sh
go tool mage android
adb install -r dist/orbitrix.apk
```

A phone holding a debug-signed build must uninstall it once before it takes a release-signed one.

## Checking it works

- `go tool mage check` passes: formatting, vet and every test.
- `go run ./cmd/orbitrix` opens Home with a live bot game on the mat. PLAY, This Device, Start plays a game.
- With the server running, `curl http://localhost:1212/healthz` prints `ok`, and `/play` loads the game in a browser.
- `go run ./cmd/aitourney -games 4 -size 9x9` plays every pair of bots and prints win rates and move times.

## First launch of a downloaded build

- Windows: SmartScreen warns about the unsigned `.exe` until it has been downloaded often enough. Choose More info, then Run anyway. The first Same Wi-Fi game brings up the firewall prompt; allow private networks.
- macOS: the app has no Developer ID signature or notarization, so macOS refuses the first open. Open System Settings > Privacy & Security > Open Anyway, once. If macOS calls the app damaged, run `xattr -dr com.apple.quarantine /Applications/Orbitrix.app`.
- Linux: unzip and run `./orbitrix`. Copying a room link uses `wl-copy` or `xclip` when one is installed.
- Android: allow installs from the browser or file manager that opened the APK.
