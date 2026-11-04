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

## Operating Context

- Local games on one device, passed between players or alone against AIs.
- Same Wi-Fi: a Windows, Android, macOS or Linux device hosts; the others find it in the list of rooms they hear
  (public) or by its code (private). Browsers don't play on the LAN.
- Online: any client hosts, a browser tab included, with the server's host password when it has one. Public rooms are
  listed; private ones are reached by code or a `/join/<CODE>` link. The host approves each player and can kick.
  Players connect to the host over WebRTC, through Cloudflare TURN when networks block a direct connection. The
  server only serves the website and signaling; no game traffic passes through it.
- Codes, names and passwords are typed on the device's own keyboard, physical or on-screen.

## Capabilities and Constraints

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
- Visual world: Party Mat, a printed party floor mat. Colour belongs to the players and nothing else; every control
  is ink on vinyl. Flat only: no 3D, no shadow depth, no gradients, no neon. Nothing from the prototype's UI. The
  design system is in `DESIGN.md`.
- Light mode (bright vinyl, the default) and dark mode (warm charcoal), switchable and saved. No blue tint in either
  mode's neutrals; blue appears only as the Blue and Sky inks.
- The 12 player inks (colour-blind tuned, first eight from Okabe–Ito) and their order are fixed, each with a shape
  mark: Vermilion (dot), Blue (bar), Green (plus), Orange (cross), Mauve (triangle), Sky (square), Yellow (diamond),
  Charcoal/Chalk (stem), Violet (pair), Brown (ring), Plum/Mint (chevron), Maroon (frame).
- Type: Unbounded for display, Archivo for UI.
- Orb motion (idle orbit, frantic before exploding, per-cell variation) is the prototype's exactly and stays binding;
  only how orbs look follows the design system. Cells are rounded square tiles with gutters.
- Labels plain and short ("Play", "Start", "Restart", "Settings", "Main menu", "Play again"). No key hints on
  buttons, no explanatory subtext.

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
- Input works with touch on Android and mouse and keyboard on desktop and web.
- No accessibility standard has been set.
