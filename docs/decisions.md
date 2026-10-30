# Decisions

Choices made where the brief or the prototype left room. Newest last.

## Phase 0

- **Module path is `orbitrix`.** Nothing outside this repo imports it, so it doesn't need to match a hosting URL.
- **Go is pinned with `go 1.27.3` in `go.mod`** (latest stable on 2026-10-30). Older local toolchains switch to it
  automatically through `GOTOOLCHAIN=auto`. CI reads the version from `go.mod`.
- **Ebitengine v2.10.5** (latest stable on 2026-10-30).
- **Build scripts instead of a Makefile.** `make` isn't standard on Windows, so each target lives in
  `scripts/build.sh` and `scripts/build.ps1` with the same target names.
- **CI builds with `CGO_ENABLED=0`.** Ebitengine desktop builds are pure Go on Windows and Linux, and the server
  uses pure-Go SQLite, so cgo is only needed for the Android/iOS bindings.
- **No README.** Project docs (prerequisites, build targets, layout) live in `PRODUCT.md`, at the user's request.

## Phase 1

- **`prototype/main.py` is not in the repo.** Phase 1 follows the rules in CLAUDE.md §1, which are canonical
  anyway. The prototype is needed from Phase 2 (AI port) on.
- **A move is a cell index** (`c*rows + r`), not a `{c, r}` struct. `Geometry.Index`/`Coords` convert, and the AI
  can loop over plain ints.
- **Players may not outnumber cells.** Nothing explodes in the first round, so on a 3×3 board the 10th player
  would have no legal first move. `NewGame` rejects `players > cols*rows`; the start screen must respect it.
- **Win check runs after the placement and after every wave**, once at least two players have had a turn. A
  win stops the chain on the spot, so cells at or over capacity can remain on a finished board.
- **Explosions apply in two passes per wave**: every exploding cell gives up its orbs, then every neighbour
  receives. The result doesn't depend on the order of cells within a wave, and a cell reaches capacity at most
  once per wave, so no de-duplication is needed. Receiving cells take the owner the sender had at the start of
  the wave.
- **Wave limit (10,000).** A chain that never settles spreads to every cell, so a normal single-owner chain
  always ends in a win long before the limit. If the limit is hit anyway, the move ends with cells over capacity
  and the next move resolves them in its first wave together with its own explosion. Those two owners can then
  recapture each other forever, hitting the limit every move. That is acceptable for a safety valve that
  shouldn't trigger: it never hangs and never creates or destroys orbs.
- **Every cell stays below 2×capacity**, since a cell under capacity receives at most `capacity` orbs per wave. So
  `uint8` counts can't overflow, and `Restore` rejects snapshots that break this bound.
- **RNG is splitmix64**, with Lemire's unbiased `Intn`. It's small, fast and gives the same bits on every
  platform.
- **State hash is 64-bit FNV-1a** over the board size, players, turn, move count, played mask, winner and every
  cell's count and owner. A test pins the hash of an empty 9×9 board so format changes are deliberate.
