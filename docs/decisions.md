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

## Phase 3

- **The UI is the Party Mat design system, not the prototype's.** A first pass ported the prototype's look; the user
  rejected it as bland and had every visual choice made from rendered samples: the world (Party Mat: a printed party
  floor mat, flat, no 3D), light and dark modes (light is the default), fonts (Unbounded for display, Archivo for UI,
  both OFL and embedded), the 12 player inks (colour-blind tuned, first eight from the Okabe–Ito set) with a shape
  mark each, and rounded square tiles for cells. Tokens and components are in `DESIGN.md`; the direction contract
  is in `.impeccable/surfaces/client.md`.
- **Screens:** Home (wordmark, seat colours, PLAY dot with Settings and Exit under it, a live four-AI demo mat with no
  lit tiles), Setup (the active Players with a −/count/+ heading, Board with a mini-board preview, nested
  preset outlines and Columns/Rows steppers, Start), Settings (tabs: Video, Audio), Game (mat and rail: Menu, turn spinner, "Chain ×N"
  callout, seat table with orb counts) and Results (the winner's ink floods the screen).
- **Menus like most games** (user request): one Menu button (or `Esc`) opens a pause menu (Resume, Restart,
  Settings, Main menu) and the game stands still behind it. "New game" was dropped from the pause menu and Results
  as redundant with Restart and Main menu. Settings opened from the pause menu returns to the paused game.
- **Little text and few keys** (user request): no key hints on buttons and no explanatory subtext. Keys left:
  `Esc` (pause / back), `M` (mute), `Space` (skip a chain), `F3` (frame rate).
- **Settings page:** tabs of rows, each a label and a control (switch, slider or two-way choice), so new groups and
  options slot in, and the layout is a single column that also suits a phone. Video: Theme, Fullscreen, Colorblind
  mode. Audio: Sound, Volume (0–100% in steps of 5, master volume on every note).
- **Colorblind mode** (off by default) draws each player's shape mark on their orbs, seat dots, spinner wedge and
  winner dot. On the board the twelve marks live in the orb atlas next to the rim and fill discs, so marked orbs
  still draw in one batch.
- **Only the player to move has lit tiles** (user request): their cells take 30% of their ink; every other cell
  stays printed blank, so territory reads by orb colour and the turn reads at a glance.
- **Board drawing:** tiles, orbs, explosions and flying orbs are four batched `DrawTriangles32` calls per frame from
  three small images (a rounded tile, an orb atlas holding a rim disc and a fill disc, and a ring). Nothing is
  cached per move any more, which also removes the per-move layer rebuild the first pass had. Orbs get a rim in the
  mat's colour so overlapping orbs read as separate pieces; explosions are flat rings in the exploding cell's ink.
- **Orb motion** is ported exactly from the prototype's `draw_orbs_on_surface`, in base units (its unscaled cell,
  24–80 px, from `calculate_cell_size`), so sizes, orbits, jitter and separation keep its proportions at any window
  size. At those sizes the orbit radius is smaller than the minimum separation, so orbs in a full cell overlap, as
  they did there.
- **Animation replays core's result.** `core.Game.Apply` resolves the move at once; the client's view starts from
  the old position and replays the recorded waves (launch, then land), which a test checks ends equal to the game.
  Leftover orbs stay, unlike the prototype's animated path.
- **Chain pacing** (user choice: accelerate + skip): the first five waves play at 14 ticks, later waves speed up 10%
  each down to 2.5 ticks, and a long chain speeds up faster so the whole chain takes about 8 s at most. Space or a
  click on the board plays the rest within 0.4 s. Explosion rings last 18/14 of a wave, but at least 6 ticks.
- **AI moves** run on a goroutine against a copy of the game, seeded from a per-game seed plus the move number,
  with `ai.DefaultBudget`. The 300–600 ms delay is drawn from the presentation RNG. The Home demo uses budget 120.
- **Turn display lags the rules on purpose**: the rail, the lit tiles and the spinner show the mover until their
  chain has played out.
- **Player count**: the count stepper (2–12) adds or removes seats at the end and keeps every existing Human/AI
  choice (`fitSeats`); new seats take the next AI in dropdown order. A fresh install starts with player 1 Human and
  the other seats shuffled AIs (the prototype put the human on a random seat). While the players outnumber the
  cells, Start is disabled and reads "Board too small".
- **Presets read cols × rows** (Small 9 × 5 … Giant 30 × 18), matching the Columns and Rows steppers.
- **Sound**: C major pentatonic, C4–A6, each note a sine plus soft 2nd and 3rd harmonics with a 6 ms attack and
  exponential decay, rendered once at startup. One player per note (15 voices, under the 16 limit). Placing an orb
  plays the seat's own note; each wave plays a chord of 1–5 notes by its size, rooted higher as the chain goes on.
  Sounds closer than 100 ms are dropped. Mute is saved.
- **Settings** are JSON in the user config folder (`%AppData%\Orbitrix\settings.json` on Windows), written through
  a temporary file: board size, players, seats (by name), theme, fullscreen, colorblind mode, sound and volume.
  Missing fields keep their defaults. Web and Android storage come in Phase 4.
- **UI scale** is the monitor's device scale factor; the game draws at full device resolution. The minimum window
  is 1040×640; when the height runs short, the rail's seat list goes to two compact columns; Setup keeps its
  control sizes and scrolls above Start.
- **Fonts add about 1.4 MB** (Unbounded 0.78 MB, Archivo 0.66 MB, both variable). Subsetting them is a possible
  Phase 4 size saving, without Python tooling.
- **FPS overlay** (frame rate and the slowest frame of the last second) is off by default; F3 toggles it and `-fps`
  starts with it on.
- **Performance meets the Phase 3 bar.** `ORBITRIX_PERF_SECONDS=60 go test -tags perf -run TestPerf40x40 -v
  ./client` fills a 40×40 board for twelve AIs to two-thirds of its capacity, then plays on in a 1920×1080 window
  with vsync and records every frame (`ORBITRIX_PERF_MARKS=1` turns Colorblind mode on, `ORBITRIX_PERF_NOVSYNC=1`
  measures headroom). On the development desktop at 60 Hz with marks on it held 59.7 FPS over 3,582 frames through
  chains of up to 471 waves: frame gap p50 16.7 ms and p99 17.5 ms, with no frame over 25 ms after the first. Draw
  took 6.5 ms at p99; without vsync it ran at about 200 FPS. One earlier run had a single 1.4 s stall that did not
  reproduce.
- **Allocations:** the client's own update and draw code does not allocate per frame. The drawing helpers take
  `color.RGBA`, not `color.Color`, because boxing a colour into an interface allocates. About 200 allocations a frame
  remain inside Ebitengine, mostly `text/v2` shaping and `vector` path filling: a few GC cycles a minute and no
  dropped frames on desktop. If Android shows GC hitches, caching drawn labels as images removes the text share.
- **One Back/Menu component** (polish pass): Back on Setup and Settings and Menu on the game rail share the compact
  40 px face, 44 px target, glyph and label offsets; Settings tabs are quiet pills like the other buttons on the
  mat; Results buttons are 52 px like the pause menu (user choice); hover always matches the click target.
