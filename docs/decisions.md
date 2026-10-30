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

## Phase 2

- **The prototype is a local, gitignored copy at `prototype/main.py`**, reference only; it is never pushed.
- **API:** `ai.Bot.Choose(game, strategy, seed, budget)` returns a cell for the player to move. A `Bot` keeps
  scratch buffers, so keep one per game or room; it isn't safe for concurrent use. Unknown strategies play like
  Jester, as the prototype's fallback did. Strategy order follows the prototype's `AI_TYPES`.
- **No move history.** The prototype's Game has no `move_history`, `last_move`, last mover or per-player move
  counts, so Sentinel's and Conqueror's `getattr` lookups always fell through to their board-only fallbacks. The
  port keeps only those fallbacks, which keeps every AI a pure function of the board.
- **Simulations are exact** (`core.Simulate` on a scratch copy). A move that can't explode is scored without
  simulating (the mover gains one orb), so only explosive moves cost anything.
- **Budget = work units per move**, replacing every wall-clock limit. A full simulation (Tactician: a search node)
  costs 1 unit plus 1 per 512 cells its chains explode (remainders carry over within the move, so many small
  chains count as fully as one big one); `core.Game.Exploded` reports the count (not part of the hashed or
  snapshotted state). A flat simulation count wasn't enough: late-game 40×40 chains average ~1,300
  explosions (max 13,000+) per move, so 1,500 Tactician nodes took 118 ms on a desktop. Measured on a desktop: an
  explosion costs ~21 ns and a Tactician evaluation of a 40×40 board ~10 µs, so 512 explosions ≈ one evaluation
  and a unit costs at most ~21 µs. `ai.DefaultBudget` = 1200 units therefore caps a move at ~25 ms on a desktop,
  which leaves 5–6× headroom for the 150 ms phone target. When the budget runs out, a strategy behaves as the
  prototype did when `simulate_after_move` returned `None` (Tactician: as when time ran out, scoring the node as a
  leaf and expanding no further). The prototype's own candidate limits (Sentinel 8, Conqueror 16, Spy 48,
  Architect 25, Tactician's branch/depth table) are kept as personality.
- **Dead branches dropped.** Legal moves are never enemy cells, so every "attack an enemy cell" branch is
  unreachable (Sentinel's options A and D and its enemy-edge scoring, Gambler's attack risk, Architect's enemy
  candidates, Conqueror's enemy scoring and in-cluster bonus, Tactician's attack scoring).
  Orbs are conserved and only the mover's colour spreads, so a move's gain is always 1 + the enemy's loss and no
  opponent ever grows: negative-gain penalties and Architect's trap bonus (`opp > self`)
  never fire. Sentinel's option B simulated but played the move whatever the result, so it no
  longer simulates.
- **Bugs fixed:**
  - Gambler never incremented `adjacent_enemies`, so exposing a nearly full cell added no risk.
  - Architect's `limit and a or b or c` applied the simulation limit to the first condition only.
  - Tactician: its own resolver destroyed leftover orbs and stopped after 100 waves (core rules now); minimax
    used `(player + 1) % players`, so eliminated players kept moving (core turn order now); it alternated
    max/min regardless of who moved, so with 3+ players some opponents played as allies (only our turns maximize
    now); move shortlists for opponents used the root player's enemies, so opponents saw their own cells as
    threats (enemies are now everyone but the mover); and it "won" on its first move whenever only its orbs were on
    the board (real win check now).
- **Voyager is real MCTS** (user request), replacing the prototype's one-ply "MCTS-inspired" scoring:
  - UCT selection (exploration 0.5; rewards in [0, 1]). With several players each node scores the player who moved
    into it (max^n style); a finished game rewards 1 to the winner, otherwise every player's orb share.
  - Each node considers a shortlist of the best moves by a cheap prior (24 at the root, 10 deeper), so the tree
    grows deep instead of 1,600 wide on 40×40. Unvisited children go in prior order, then UCT.
  - Rollouts: 8 plies, each the best of 4 random legal cells (explode next to an enemy > explode > build > claim).
  - An iteration costs one budget unit (board copy + shortlist scan ≈ one unit of work on 40×40) plus the chain
    charge, so 1,200 iterations at the default budget. The final move is the most visited root child. A root
    child that wins on the spot is played at once.
  - No tree reuse between moves: it would make a move depend on the Bot's history, not just the position.
  - Speed: an exact bound skips the shortlist's neighbour scan for cells that can't make the list (−24% late
    game), lookup tables replace UCT's log/sqrt/divide, rollouts take four 16-bit samples per random draw, and a
    salted Fibonacci hash breaks shortlist ties instead of a random draw per cell. 1.0–1.5 ms per move on 9×9,
    2.2–3.6 ms on 40×40 (benchmark positions), no allocations once warm.
- **Randomness** goes through the seeded `core.RNG`. Python's `sort(key=(…, random()))` tie-breaks became a
  uniform choice among the tied moves (same distribution); stable sorts keep Python's tie order elsewhere.
- **Distances** (Sentinel's buffer, Conqueror's network/enemy/comeback distances) come from one breadth-first
  search from all source cells, which equals the prototype's all-pairs Manhattan minimum in O(cells).
- **Tournament (desktop, `-workers 4`):** worst single move on 40×40 21.0 ms (Voyager; p99 14.8 ms, avg 6.8 ms),
  Tactician 19.5 ms, Gambler 17.6 ms; everything else ≤ 6.1 ms. Overall win rates on 9×9 / 20×20 / 40×40:
  Voyager 98 / 100 / 100 %, Tactician 81 / 43 / 50 %, Gambler 42 / 74 / 86 %, Spy 58 / 57 / 50 %, Conqueror
  46 / 47 / 50 %, Sentinel 39 / 39 / 43 %, Architect 27 / 24 / 14 %, Jester 10 / 16 / 7 %. Windows' clock ticks in
  ~0.5 ms steps, so sub-millisecond p99s read as 0 or 0.5; outliers of ~10 ms even for Jester are scheduler noise.
- **Faithful quirks kept:** Tactician scores the whole search with the root position's density; Spy picks among its
  top three 12% of the time; a Gambler on a hot streak with no big bet falls back to a random explosive cell;
  Conqueror's opening mode ignores wins.
