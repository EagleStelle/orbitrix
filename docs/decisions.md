# Technical decisions

Decisions that future changes have to respect, and why. How things work is in [architecture.md](architecture.md).

## Build and release

- `GOOS=… go tool mage` would build Mage itself for that OS, so each Mage target sets its own `GOOS`.
- Desktop builds keep cgo off: Ebitengine 2.10 loads Cocoa/Metal, X11, OpenGL and the sound libraries at run time through purego, so every desktop target cross-compiles from any OS. A dependency that needs cgo would break that. Only the Android binding uses cgo.
- The version lives in two places, `version` in `magefile.go` and `versionName` in `android/build.gradle.kts`, and the release workflow refuses a tag that differs from either. `versionCode` must rise with every release, or phones won't update.
- Release file names carry no version, so `releases/latest/download/<file>` stays stable and the files drop straight into the server's downloads folder.
- `all` runs every target except `push`, so a local build never publishes or needs credentials.
- The race detector runs only in CI: the Windows development machine has no cgo.

## Rules engine

- Map capacity is an explosion threshold; outgoing degree counts transfers. Reservoirs add one to the threshold
  without adding a transfer, so blasts preserve the reserve and total orbs. Spillways add one to the threshold and
  one lost transfer into their own pit; a Spillway cannot also be a Link, so its targets fit `MaxConnections`.
- Latch responds to direct accepted placements and orb arrivals, never to exploding by itself. Latch stays
  pressed after its first input; Toggle flips on direct placement into an empty cell and once per receiving or
  exploding wave, regardless of arrival count. Direct placement into an occupied Toggle does not flip it by itself;
  Hold is pressed exactly while its count is nonzero. Circuits accept any mixture and number of switches and at least
  one Gate or Door. Every input reverses independent circuit polarity; simultaneous inputs cancel in pairs.
  Another switch transition rearms a Latch without an additional reversal. Hold press and release both reverse.
  Outputs have individual initial open/closed states; polarity reverses them. Closing a Door preserves trapped orbs
  and ownership until reopening, rather than destroying or relocating them.
- Map topology is immutable per wave. A direct placement applies switch/Gate effects before selecting the first
  exploding cells (terrain event wave −1). Each wave uses its starting connections, then applies aggregate wall
  damage and switch effects after transfers and updates capacity before the next wave is selected. Search copies
  install their own replacement topology, and presentation replays explicit pressed/released and open/closed events
  at the same boundary. This prevents iteration order or animation timing from changing the rules.
- Configured initial output states give every initially playable Floor capacity of at least two, including a Reservoir's or Spillway's extra orb, and
  Breakable Walls require two initial Floor neighbors, keeping first placements non-explosive. Connectivity counts
  destroyed Breakable Walls and Gates whose controller shares a Floor group with either endpoint. Reachable Gates
  merge groups until no more circuits can connect them; permanently isolated Floor and circular Gate dependencies
  remain invalid.
- Map files carry a bounded, versioned binary payload and checksum. QR links embed it in a URL fragment; there is
  no map lookup service. Invalid gameplay drafts remain portable, but malformed data is rejected.
- Portable exports remain format 2 with `orbitrix-map2.` codes. The two-byte cell and six-byte edge layouts keep
  existing switch tags; Door uses the next tag, cell bit 3 and edge bit 7 store initial open state. Backend circuit
  types stay separate from passive features. Format-1/2 maps and temporary format-3 circuit-build exports remain
  importable without rewriting stored maps. New exports always use format 2.
  Match snapshots carry durability, `Pressed`, and independent `CircuitOn` polarity, all covered by the hash.
  Protocol 3 and map format 2 were already introduced after public release v1.0.0 (protocol 1, no custom maps), so
  this circuit change reuses those unreleased identifiers. Compare against the last public release before bumping:
  reuse an existing post-release bump; bump when the public release's identifier would otherwise be reused for an
  incompatible change. All peers must run the same development build; equal unreleased identifiers do not prove
  compatibility. Portable-map import compatibility is separate.

- Players may not outnumber cells. Nothing explodes in the first round, so an extra player could have no legal first move. `NewGame` rejects it and Setup shows "Board too small".
- The win check waits until every player has had a turn, not just two. In normal play that is the same thing, since nothing can explode in the first round, but it keeps a passed turn from ending a game before anyone has moved.
- A win stops the chain on the spot, so a finished board can hold cells over capacity. The 10,000-wave cap likewise leaves over-capacity cells to the next move's first wave.
- Counts stay below twice the capacity, so `uint8` can't overflow; `Restore` rejects snapshots that break this.
- A test pins the empty 9×9 state hash. Changing the hash or the game state's layout breaks desync detection between versions, so it must go with a protocol version bump.

## Bots

- Saved settings store strategy names, so bots can be reordered or added without breaking saved seats. Unknown names play like Jester.
- Budgets count work, never time, so a move is the same on every device. `DefaultBudget` 1,200 keeps the worst move near 16 ms on the development desktop (Voyager, late 40×40), inside the 150 ms phone target. Only Sovereign gets 4×, where it thinks on its own thread. In the browser, where thinking holds up drawing, bots on boards bigger than Floor get `DefaultBudget × 234 / cells`. A new bot needs its work priced in these units.
- A move must depend only on (position, seed, budget). Sovereign clears its transposition table every move for this reason, and its network runs in integers so every platform computes the same moves.
- Shared survival protection covers only a first placement or a position of at most one cell and two orbs, so each bot otherwise keeps its own risk appetite.
- Spy stays a heuristic. Reply search would turn it into another search bot.
- Network weights are generated Go source, rewritten by training only when the new weights win, so the repo holds no data files.
- Sovereign uses Tactician search on custom maps until its network can represent map topology; its existing
  rectangular pattern table cannot represent every map neighborhood or a fifth Link connection. Prodigy fully
  refreshes its network evaluation on custom maps so topology changes cannot leave cached features stale.

## Client

- `vector` draws only into sprites and cached images, never the screen: each path costs several GPU passes and a render-target switch that phone GPUs can't afford. Shapes are white sprites, nine-sliced and tinted per draw.
- The UI scale is capped at 2, so phones at 2.6 to 3.5× draw about 40% fewer pixels for slightly softer text.
- `Draw` returns early when no `Update` ran, and still screens redraw only on input, on change and every 30 ticks. Not on Android, where skipped frames show stale buffers and flicker.
- Settings are JSON written through a temp file, and missing fields keep their defaults, so new settings need no migration.
- Android turns the phone to landscape for a board with more columns than rows. Browsers and desktop windows can't lock orientation, so they draw such a board a quarter turn instead.
- Floor (13×18) is the largest preset because it is the largest board that keeps 24 px cells (WCAG 2.5.8's minimum target) on a 360-px phone held upright.
- The Android activity holds back a touch lift under 50 ms after the press, because Ebitengine polls touches once a frame and quick taps were lost.
- A browser opens its keyboard only during a touch, so a text field takes the focus on pointer down, not on release.
- Palette, editor canvas, thumbnails and match boards share one map-art renderer. Relationship colors are eight
  muted inks per theme, repeated with unique anchored eight-bit notch patterns for identities 1–255. Patterns stay
  visible without Colorblind mode and gain weight with it. This limited exception to player-only hue identifies
  Link pairs and switch/Gate circuits without group numbers; it is not a general chrome accent palette. Shape also
  distinguishes portals, the divided Reservoir tank, Latch, Toggle, Hold, and Gate state.
- The editor keeps a compact desktop list, category-based short-landscape list, and portrait icon-and-label shelf.
  The inspector shows functional help and the pair/circuit swatch without duplicating the selected name or icon.
  Physical-phone touch painting and camera-to-import behavior remain unverified on hardware.

## Platforms

- Android builds need `-checklinkname=0`: pion's interface lookup (`github.com/wlynxg/anet`) links to a `net` internal the linker otherwise refuses.
- Android needs no network security config: it governs Java's HTTP stacks, and Go dials its own sockets.
- AGP's built-in Kotlin stays off, and there is no AndroidX: Kotlin alone added 0.6 MB of stdlib to a one-activity Java app.
- The release key is PKCS12 with alias `orbitrix` and the key password equal to the store password (`keytool`'s PKCS12 default). Without it the build falls back to the debug key, which a release-signed APK can't update.
- Web files are renamed after their content hash and kept only as `.gz` and `.br`; the server unzips the `.gz` for a client that takes neither. `wasm_exec.js` must come from the Go version that built `game.wasm`.
- Mage joins the macOS arm64 and amd64 binaries into a universal one itself (16 KiB-aligned slices, as `lipo` writes them), so the Mac build needs no Mac. The arm64 slice keeps the Go linker's ad-hoc signature, which Apple silicon requires.
- Zips store the executable bit in their Unix mode, which a bare binary download would lose.
- No native iOS: it needs a Mac with Xcode, an Apple Developer account and Apple's multicast entitlement for Same Wi-Fi. iPhones use the web build.

## Multiplayer

- The host is authoritative and the server never carries game traffic. Accounts, databases, server-side rooms and spectators are out of scope.
- No lockouts: CGNAT puts many people behind one IP, so a correct password or room code always passes, and wrong attempts only slow their own socket. Brute force fails on the password's length (20 characters at least). The per-IP socket cap (100) is the one place an honest client can be refused, and a flood from a friend's exact carrier IP blocks new `/signal` connections from it at the edge; fixing that needs Cloudflare Business.
- Rejected bot defences: TLS and HTTP/2 fingerprints (Enterprise only), JS fingerprinting, Turnstile and CAPTCHAs (native clients can't run them), paid reputation feeds, and behavioural scores, which would flag game clients.
- A socket must answer the proof-of-work challenge it was given, so challenges saved from a quiet moment can't skip the work a flood raises. Difficulty is capped at 18 bits so the web build on a phone solves it in about 2 s.
- `CF-Connecting-IP` is believed only from a private or loopback peer, since the container publishes on 127.0.0.1 only behind the self-hoster's proxy.
- TURN needs a password of at least 20 characters, both `CF_TURN_*` values and a working Cloudflare call; anything less falls back to STUN and is logged. With no password, hosting is open, STUN only, at most 3 rooms per IP.
- Every connection the hub lets go of, a drop included, is reported through `Config.OnClose`, so the host revokes its TURN credential at once. The credential cap counts live credentials.
- Signaling answers a wrong code, a full approval queue and a denial with the same "refused", so codes can't be probed. Peer ids are random, so a restarted server can't hand an old id to a new joiner and revoke theirs.
- The hub ignores room codes online: signaling already admits only approved players, and a reclaimed room may have a new code.
- Late or illegal moves are ignored rather than punished, since they can race a turn timeout.
- A pass (Skip) ends a turn without an orb, and a player who passes before ever placing is out. If only skipped seats are left alive, the room returns to the lobby.
- Bot moves are paced on each screen, not on the host, so every client shows them 300 to 600 ms apart. The hub still waits 400 ms before a bot move so a browser host can draw.
- WebRTC skips trickle ICE (shortcut): each side gathers its candidates first, so a join is one offer and one answer. TURN URLs on port 53 are dropped, as Cloudflare advises.
- Closing the desktop window while hosting closes the room and waits half a second for the Bye messages (shortcut).
- Online public rooms are listed by board, not device name, because the list is open to the internet. Every online join, public or private, waits for the host.

## Same Wi-Fi

- Everyone on the network is trusted: no approval. Native clients find rooms by code among the rooms heard; a public code is derived from the announcement ID and grants no privacy. Private admission codes remain independent secrets. Browser joiners use the native host's address or QR link.
- Discovery uses subnet broadcast, not multicast: it needs no group joins, which Android 11 and later make awkward. `wlynxg/anet` reads interfaces where Android 11+ refuses Go's own lookup.
- The host listens only on private IPv4 addresses, never 0.0.0.0, and leaves public-network detection to the OS firewall. A host whose address changes mid-session stops being reachable there.
- Native hosts serve the embedded web game over plain HTTP on their LAN listener, so browser joiners can reach `/ws` at the same origin without mixed content. The local page has no secure-context clipboard or wake-lock guarantee. Mage bundles gzip assets; a browser build carries no embedded game or listener.
- Browser hosts use WebRTC invite/reply links rather than a listener or UDP discovery. The page still needs loading; signaling and TURN are skipped, while public STUN may help. Replies must open in the host's browser at the same origin, with one hosting tab per origin. Each invite admits one player; reconnecting needs a new invite.
- Every platform shares one Invite players dialog: access, QR, code, Copy code, Copy link and the action, without subtext or a written link. The QR always encodes the join link; Copy code copies only the code and Copy link only the link. Browser-only codes carry the full connection description; they cannot become short lookup codes without discovery or a server.

## Rooms and seats

- The host's room page is This Device's Setup scene, so local and room setup share one implementation.
- Seats keep one backing array of capacity 12, and seat holds use room-wide numbers, so a grace timer never acts on a seat that moved.
- Two humans never swap seats; `hub.Room.SwapSeats` refuses it as well as the UI.
- A player name is at most 16 bytes, short enough for a seat pill and within `proto.MaxName`'s 24.

## Server and deployment

- Downloads are cached for 5 minutes rather than `no-cache`, which kept Cloudflare from storing them at all. They stay out of the image, mounted read-only, so publishing a client needs no image rebuild.
- The final image is scratch, so there is no `/etc/mime.types`: the server registers `.ttf` itself, and any new file type needs the same.
- The build context is an allowlist (`deploy/Dockerfile.dockerignore`), so `.env` and local files never reach it. OCI labels come last, so a new version or commit never invalidates a cached layer.
- Compose runs the published image and never builds, publishes on 127.0.0.1 only, and bundles no tunnel: self-hosters choose their own exposure.
- The free plan's WAF has no regex, so the "site only" rule lists exact paths and file extensions. A web build that adds a file type needs it there and in the cache rule.
- Port 1212: not a common port, and clear of the LAN hub's 7777.
