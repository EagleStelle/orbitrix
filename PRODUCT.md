# Orbitrix

A turn-based chain-reaction orb game for 2–12 players, in Go with Ebitengine, on Windows, Android, the web (WASM),
macOS and Linux, with online and LAN games hosted by one of the players. No native iOS: iPhones play in the browser.

<!-- impeccable:product-schema 1 -->

## Platform

web

One custom-drawn game look on every target. Ebitengine draws all UI in Go: no HTML, CSS or JavaScript UI, no DOM,
no native widgets, on any platform including the web build. "web" here means "not a native OS design language". The
only HTML is `web/index.html` (the loader for `game.wasm`) and `web/landing.html` (the landing page at `/`); neither
is game UI. Platform needs still apply: touch targets and a portrait layout on Android, safe areas, Android back as
Esc, keyboard shortcuts on desktop and web.

## Users

- Now: the developer, friends and family. Pass-and-play on one device, LAN sessions with no internet, and online
  rooms one of them hosts, on mixed devices (Windows PCs, Android phones with the APK, browsers) in the same room.
- Later, possibly: a public audience. Nothing is committed, but no work may make a public release impossible.

## Product Purpose

Players take turns placing orbs. A cell at capacity explodes into its neighbours and captures them, which can set off
chains across the board. The last player with orbs wins. Success: friends start a game on whatever devices they have,
fill empty seats with AI, and enjoy following a chain reaction as it plays out.

## Positioning

- Huge, chaotic games: up to 12 players on boards up to 40×40, with long chains that resolve exactly and play back
  wave by wave.
- AI personalities: twelve bots with distinct styles, easiest to hardest: Jester, Mason, Warden, Gambler, Nomad,
  Raider, Spy, Prodigy, Tactician, Voyager, Hunter, Sovereign. Any seat, or solo play against them.
- Play together anywhere: same device, LAN without internet, or online. Every client shares one room, one rules
  engine and one protocol.
- Tactile feel: orbs orbit while idle and turn frantic just before exploding, with pentatonic sound generated in code.
  Links chime, switches click in distinct voices, and breakable walls knock or crumble, all in C major pentatonic.

## Operating Context

- Local games on one device, passed between players or alone against AIs.
- Same Wi-Fi: native devices find a host by its broadcast or private code; browsers load the game from the native
  host's QR/address and join there. Browser hosts use manual WebRTC invite/reply links, without signaling or TURN.
- Online: any client hosts, a browser tab included, with the server's host password when it has one. Public rooms are
  listed; private ones are reached by code or a `/join/<CODE>` link. The host approves each player and can kick.
  Players connect to the host over WebRTC, through Cloudflare TURN when networks block a direct connection. The
  server only serves the website and signaling; no game traffic passes through it.
- Codes, names and passwords are typed on the device's own keyboard, physical or on-screen.
- Every platform uses the same Invite players dialog, shown on the first Open room and from the invite button: access,
  the QR (the join link), the code, Copy code, Copy link and the action. No subtext, no written link. Each copy button
  copies only its own value. Browser-only rooms use longer connection codes instead of a server lookup.

## Capabilities and Constraints

- Custom maps and Map Editor: Floor, Void, Wall, Breakable wall, Barriers, Gates, Doors, Links, Reservoirs, Spillways, and three
  switch types: Latch (rearmed by another switch), Toggle (flips on empty-cell placement or receiving/exploding waves), and Hold (pressed while occupied).
  Preserve the established map design and change it only as explicitly requested; the authoritative map-device rules are in `DESIGN.md`, shared across palette, editor, thumbnails and match. Links retain the separated lower/upper U paths with rounded internal bends derived from the outer frame, unchanged tips, width and pair ink, and no right-angle bends. Latch and Hold buttons are outlined OFF/released and solid circuit-color fill ON/pressed. The Reservoir reserve pocket stays square; a Spillway shares its frame and ink logic but has diagonal corner channels into a Pit-ink pocket.
  Device outer frames share the exact Floor tile bounds and corner radius, with no surrounding Floor gap. All three switches share inner gaps and corner-radius geometry: Latch and Hold have identical inner rectangles; Toggle uses the full shared inner rectangle and gaps, split into two equal adjacent halves by a center seam: left solid circuit-color fill means OFF, right solid fill means ON, with the opposite half outlined. Hold brackets are clipped from the same rounded frame. Empty Reservoirs are neutral; occupied Reservoir outlines use the occupying player's exact ink and follow the displayed animation wave. Four symmetrical cardinal channel strokes connect the central pocket toward the basin, stopping two stroke widths before its unchanged frame; they share its stroke and neutral/owner ink. Links and switches retain pair/circuit inks.
  Breakable walls show a connected, tapered negative-space fracture rooted at the center: three main splits and two forks, visibly identifying breakability at initial durability 2. At durability 1 the same whole connected network grows 1.24x, longer and wider toward the edges. Use branching fractures rather than repetitive zigzags; no random texture, detached flecks or circles. Fracture cuts are structural damage, not additional device-icon stroke weights. Wall fill and joins stay unchanged; destruction creates empty Floor.
  Circuit groups any mixture and number of switches with Gates and Doors; each output has an editable open/closed starting state. Maps are stored locally, with file/code/QR portability,
  editable draft recovery, test play, and automatic host-to-player distribution. See `docs/maps.md`.
  Closed Doors preserve trapped orbs and their owner: they appear frozen in Floor ink without Colorblind marks, beneath a locked icon in the opposite theme's Floor color. Reopening restores their owner's normal appearance, motion and control.

- Rules, screens, AI behaviour and multiplayer design are specified in `CLAUDE.md`, the source of truth.
- Every graphic is drawn and every sound synthesised in code. No image or audio files in the game; the only
  embedded assets are the OFL fonts Unbounded and Archivo in `client/fonts/` with their licences. The README's logos
  in `docs/` are generated from the same logo code.
- Target: 60 FPS on a 40×40 board with 12 players in a long chain, on a mid-range Android phone and desktop Chrome.
- Released as 1.0.0 through GitHub releases (Windows, Android, macOS, Linux) and the website's downloads; the server
  image is on Docker Hub. Store listings are undecided.
- Developer docs: `README.md` and `docs/` (usage, installation, deployment, architecture, structure, decisions, API,
  testing). Licence: GPL-3.0.

## Brand Commitments

- Name: Orbitrix.
- Visual world: Party Mat, a printed party floor mat. The same twelve orb inks identify players, Link pairs and
  switch circuits. Map art stays identical with Colorblind enabled or disabled; no group numbers are printed. General
  controls stay ink on vinyl. Flat only: no 3D, no shadow depth, no gradients, no neon. Nothing from the prototype's
  UI. The design system is in `DESIGN.md`.
- Light mode (bright vinyl, the default) and dark mode (warm charcoal), switchable and saved. No blue tint in either
  mode's neutrals; blue is confined to the twelve orb inks.
- The 12 player inks (colour-blind tuned, first eight from Okabe–Ito) and their order are fixed, each with a shape
  mark: Vermilion (dot), Blue (bar), Green (plus), Orange (cross), Mauve (triangle), Sky (square), Yellow (diamond),
  Charcoal/Chalk (stem), Violet (pair), Brown (ring), Plum/Mint (chevron), Maroon (frame).
- Type: Unbounded for display, Archivo for UI.
- Orb motion (idle orbit, frantic before exploding, per-cell variation) is the prototype's exactly and stays binding;
  only how orbs look follows the design system. Cells are rounded square tiles with gutters.
- Labels plain and short ("Play", "Start", "Restart", "Settings", "Main menu", "Play again"). No key hints on
  buttons. Host and Join groups stay visible with no descriptions; only the browser's Join group may explain a
  limitation and how to continue, using plain words without jargon or center dots.

## Evidence on Hand

- `prototype/main.py`: the pygame prototype, reference for rules, AI and orb motion only. Gitignored; the original
  lives outside the repo.
- `go run ./cmd/aitourney`: AI-versus-AI win rates and move times.
- `docs/decisions.md`: constraints and trade-offs future work must respect, AI budget sizing included.
- No players outside the friend group, no testimonials, screenshots, store listings or usage data. Do not invent any.

## Product Principles

1. Chaos stays readable. However large the board or player count, people follow what happened: chains animate wave
   by wave, and AI moves are delayed on purpose.
2. Every seat is equal. Human, AI, local, LAN and online players fill the same seat under the same deterministic
   rules.
3. Feel is part of the game. Orb motion, sound and a steady frame rate are features, not decoration.
4. Friends first, public possible. Build for a trusted group today without blocking strangers later.

## Accessibility & Inclusion

- Players are told apart by colour. The first eight inks follow Okabe–Ito; Colorblind mode (Settings > Video) adds
  each player's shape mark to their orbs, seat dots and spinner wedge, which seats 9–12 rely on more.
- Map relationships use twelve groups with the exact orb colors. Devices, terrain, Gates and relationship swatches keep identical normal art and colors with Colorblind enabled or disabled. Player and orb marks elsewhere stay unchanged. Neither state adds map identity stamps. Device silhouettes and
  pressed/released forms also communicate function and state without relying on hue.
- Input works with touch on Android and mouse and keyboard on desktop and web.
- No accessibility standard has been set.
