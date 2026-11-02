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

- **Reference prototype:** `prototype/main.py` is a local, gitignored reference for rules and personality behaviour.
- **AI dropdown order, easiest to hardest:** Jester, Mason, Warden, Gambler, Nomad, Raider, Spy, Prodigy,
  Tactician, Voyager, Hunter. Strategy names are also the accepted API names and saved seat values, so settings saved
  before Hunter, Prodigy and Nomad existed, or before a reorder, still load. Gambler moved down from the top of the
  heuristic bots on 2026-11-02: the Spy and Raider reworks overtook it, Raider beats it two-player on every size
  (74/70/64% on 9×9/20×20/40×40, 400/200/200 games) and Nomad on 9×9 (70%).
- **Deterministic AI API:** `ai.Bot.Choose(game, strategy, seed, budget)` returns a legal cell for the player to
  move. The result depends on those inputs, so saved positions and seeded games are reproducible. A `Bot` reuses
  scratch buffers; keep one per game or room and use it sequentially. Unknown strategies play like Jester.
- **Shared rules engine:** search uses exact `core.Simulate` results on scratch copies. A heuristic move that cannot
  explode has a known outcome of one added orb and needs no simulation.
- **Bounded work:** all strategies use the same native budget of 1,200 units. Each full heuristic simulation,
  Tactician or Hunter search node or Voyager iteration costs one unit, and a Prodigy network evaluation one plus one
  per 16 cells it recomputes through all four layers (≈0.73 µs a cell on a desktop) and one per 1,024 cells for its
  whole-board passes (≈10 ns a cell), plus one per 512 cell explosions. Explosion remainders
  carry across the move so many small chains count fully. Work units keep the search deterministic and account for
  the cost of long chains. Browser clients scale the same limit down on large boards to keep drawing responsive.
- **Distinct heuristic personalities:** Gambler, Warden, Raider, Spy and Mason retain their own scoring,
  candidate limits and seeded randomness. Shared survival protection applies only to the first placement or a
  position with at most one cell and two orbs, so normal risk preferences determine the rest of their play.
  Eligibility is checked once per move. The strategy proposes a cell, then the survival check may choose a safe
  foothold at least three steps from enemies and outside their primed chains. One work unit is reserved to assess
  an explosive counterattack with the rules engine.
- **Warden departs from the prototype in two places:** it explodes its own primed corner only when that captures
  orbs, because re-popping corners on every quiet turn kept it off its edges, and a defensive prime may have no
  primed enemy neighbour (the prototype allowed one, which let the enemy pop it first). Exploding before priming
  across every enemy made it as strong as Spy at the time, so it was left out.
- **Raider scores every move from its neighbourhood and simulates its 16 best explosions.** The prototype simulated
  the first 16 promising moves in board order, so on big boards it never saw a capture away from the top-left corner.
  Its scoring keeps the prototype's aggression (stacking against enemies, the pull of enemy fill within two steps, a
  bonus for wedging between enemy groups, captures, preying on the weakest) and avoids suicidal cells, but each part
  costs O(1) per move: the enemy fill is spread once per move instead of gathered per move, the flank test looks at
  the cell diagonal between two enemy neighbours instead of labelling every enemy group, and an explosion that can't
  chain or capture is scored without simulating. Dropped as not worth their cost: the comeback cluster target, the
  distance-to-our-network penalty, the distance-gated opening and the last-cell retreat (shared survival protection
  covers it; without it Raider won more). It is 233 lines against the port's 394.
- **Spy stays heuristic but reads danger off the board.** Each shortlist group starts at a random move (board order
  had the same top-left bias). Without simulating replies, `openingThreats` marks the cells enemy chains could reach
  next turn: Spy loses 2 lead per orb of its own there. Placements share one reading; explosions read their own
  board, at most 8, best lead first, until no lead can beat the best score. A two-ply reply search was tried and
  rejected for being search; so were 4 or 2 reads (3–7 points of win rate lost), skipping the simulation of
  explosions that can't capture (no faster) and capping explosions at half the shortlist (5 points lost on 20×20).
  The shortlist stops filling at 48 and the best three are kept without sorting every candidate, which changes no
  move.
- **Spy infiltrates like a spy:** out of enemy reach, it gains 1 lead per orb of each enemy neighbour it would explode
  before, the race Nomad runs. It used to count only cells one short of exploding at 0.5; the race and the higher
  weight together won 4–9 more points of free-for-all games. A penalty for races it would lose cost win rate.
- **Nomad is the sparse opposite of Mason.** It spreads single orbs over open ground, 30 points per step from its
  nearest cell up to 3, never where `openingThreats` says an enemy chain can reach next turn. Each enemy neighbour is
  a race: our cell explodes after cap−count−1 more of our moves, theirs after cap−count of theirs, and they move first.
  A won race scores 40 per enemy orb, a lost one costs 40 per orb we'd leave. Stacking scores low, so it builds only
  to win a race or on a crowded board. Of the explosions that can capture, it simulates the 2 with the most enemy
  orbs next to them and takes the one that captures most (60 a captured orb). With 4 simulations it outscored Spy on
  20×20; 2 bring it level with Raider at two thirds of Raider's cost.
- **Heuristic bot strength** (2026-11-02, random first placements, seed 7, `nomad` in the pool): two-player win rate
  against the other five, 9×9/20×20/40×40 (400/200/40 games a pair): Mason 28/17/16%, Warden 36/18/17%, Gambler
  39/64/73%, Nomad 62/63/65%, Raider 52/70/68%, Spy 84/69/63%. Five players, 9×9/20×20 (500/300 games): Mason 11/5%,
  Warden 13/11%, Gambler 12/20%, Nomad 30/18%, Raider 15/30%, Spy 40/35%. Twelve players, two seats each,
  9×9/20×20/40×40 (300/120/36 games): Mason 4/4/4%, Warden 5/3/0%, Gambler 7/7/11%, Nomad 11/13/14%, Raider
  7/11/10%, Spy 15/12/11%. Against the same five bots as before the Spy changes, Spy went from 84/72/58% to 87/69/59% in
  two-player games and from 41/32% to 45/39% in five-player games. Time per move in real games on a desktop,
  9×9/20×20/40×40: Nomad 3.4/14/57 µs, Raider 4.2/21/85 µs, Spy 9.5/44/166 µs (12.4/48/172 before).
- **Jester chooses uniformly among legal moves.** Its random personality bypasses shared survival protection.
- **Voyager uses Monte Carlo tree search:** UCT selection, expansion, stochastic rollouts and mean-reward
  backpropagation. Its shortlist contains up to 24 root moves and 10 deeper moves to give large-board searches
  useful depth. Rollouts cover four plies, each comparing four sampled legal cells, with UCT exploration 0.25.
  Cutoff rewards value material, territory, readiness, corners and exposure in turn order. The most visited root
  move is selected. Each search starts from the current board to preserve independence from earlier bot calls.
- **Tactician uses alpha-beta minimax with iterative deepening:** its own turns maximize its score and every
  opponent minimizes it, following the rules engine's turn order and eliminations. Search considers up to 24 root
  candidates, eight inner candidates and sixteen plies. A completed depth supplies the answer if work runs out;
  fully evaluated one-ply moves provide the initial fallback.
- **Hunter uses Best-Reply Search** (Schadd & Winands 2011) on Tactician's iterative-deepening alpha-beta, root,
  candidates and evaluation. Between two of its own moves only the single most damaging reply from any one opponent
  is searched, played out of turn (`Game.Turn` set on a scratch copy), so a round costs one opponent ply at any
  player count. Each opponent offers `max(3, 16/opponents)` candidates. Leaves extend through captures
  (quiescence: two plies, the four biggest hauls per node, stand-pat allowed); four plies with four captures
  blew up on big boards and lost to Voyager on 20×20. Its prey is the opponent with the most orbs at the root,
  weighted three times the field in the evaluation, so it goes after the leader. With two players the best reply
  and the prey weighting both match Tactician, so a duel Hunter is Tactician plus quiescence. Measured with random first placements (wins out of games played): two
  players 9×9 60/100 against Tactician and 58/100 against Voyager, 20×20 25/40 against Tactician and 26/40 against
  Voyager, 40×40 11/16 against Voyager; four players 9×13 (with Tactician, Voyager, Gambler) 54/120 wins against
  Voyager's 34. Three players on 9×9 favour Voyager (65/150 against Hunter's 40), the cost of chasing the leader.
  Placed after Voyager, as the hardest.
- **Prodigy plays AlphaZero-style:** PUCT search (exploration 0.4; an unvisited child takes its parent's network
  value; 32 root and 8 inner children, the network's top cells) guided by a 1,893-weight network: four graph
  convolutions of 16 channels over each cell and the sum of its orthogonal neighbours (one weight for all four
  directions, so every board size, rotation and reflection plays alike), a per-cell policy logit and a logistic
  value head over the mean-pooled channels plus the mover's orb share, 1/alive players and Voyager's position
  share. The value replaces rollouts; other players split the rest of the winning chance (max^n backup, any
  player count). Before searching it tries its primed cells next to enemies for an outright win with the rules
  engine, using at most half the budget. Weights are generated Go source (`prodigy_weights.go`), not an asset.
- **Prodigy evaluates incrementally.** A cell's output depends only on cells up to four steps away, so in each move
  the first position each player is to move in gets a full pass and becomes that player's base; every later
  position recomputes, at layer l, only the cells within l steps of a cell that differs from the base, and reads the
  rest from it (a test checks this against a full pass). The convolution keeps eight output channels in registers
  and skips zero inputs; Go has no SIMD, so a full pass stays near the scalar limit at ≈0.73 µs a cell. Evaluations
  per move went from about 200/46/11 to 340–440/210–330/120–220 on 9×9/20×20/40×40 (late/mid game), at 14.6–16.8 ms
  a move on a desktop against 17–20 ms before; Voyager's worst case is 17.1 ms. No allocations after the first move.
  At the browser's 40×40 budget (175 units) one full pass costs about 103, so Prodigy there plays its policy.
- **Prodigy's search settings were re-tuned on the same weights** (100 two-player 9×9 games each against Voyager
  and Hunter): exploration 1.5 to 0.4 (0.3–0.7 alike, 2.5 much worse), first-play urgency 0.1 to none (0.3 much
  worse), root children 16 to 32 (24–48 alike, 10 worse); 5 or 12 inner children did worse than 8. Wins went from
  31 and 21 to 59 and 54.
- **Prodigy's training** (`go test -tags train -run TestTrainProdigy`, build-tagged like the client's perf test)
  imitates Tactician, Hunter and Voyager when starting from zero weights. Each generation then plays 192 games on
  9×9, 20×20 and 40×40, a third each, biggest first so no long game runs alone at the end, with 2–12 players (10%
  with 9–12): half self-play, half league games against those three bots, whose moves become one-hot policy
  samples. Dirichlet(1) root noise at 25% and exploration 1.5 (the setting the weights were trained with) widen
  the visit shares. Like KataGo's playout cap randomisation, a quarter of Prodigy's moves search the full budget
  and become samples, the rest an eighth. The value target averages the winner with the search's root value, at
  most 100 evenly spaced positions per game, AdamW. Every ten generations it plays 96 duels against Hunter and
  Voyager on all three boards and 16 tables on 9×9 and 20×20 (four players with Hunter, Voyager and Tactician;
  twelve with every other bot and a second Hunter), with fixed random first placements, and writes the weights only
  when they beat the best score so far. A finite-difference test checks backpropagation.
- **Training has not yet improved on the kept weights** (2026-11-02). They came from the first 75-minute run on
  5×5 to 9×13. Every continuation lowered the score: all boards 0.328 to 0.078 in five generations, and to 0.125 in
  three with or without league games; 9×9 alone 0.625 to 0.39–0.53 in three to six generations at learning rate
  1e-3 or 1e-4, and with the original trainer's settings. They look like a lucky peak of a noisy process, further
  selected by the search tuning, so the gate was doubled and saves only on a strict improvement. A 32-unit hidden
  value layer was tried and dropped: its fresh head reached only 0.06 in ten generations. A long run is the
  remaining test; it costs about 3.5 minutes a generation on a 16-thread desktop.
- **Prodigy's strength** (2026-11-02, `TestProdigyMatches`, random first placements, two players unless noted;
  wins out of games): 9×9 (100 games a bot): Jester, Mason, Warden, Gambler, Nomad, Raider 100, Spy 99, Tactician
  66, Voyager 59, Hunter 54; four players 12/25 wins, the most per seat (Voyager 4/10, Hunter 3/10); twelve players
  1/25, behind Nomad 6/28, Voyager 6/27 and Hunter 5/27. 20×20 (40 a bot): the heuristic bots 40 except Spy 31,
  Tactician 10, Voyager 9, Hunter 3; four players 3/10, twelve 0/10. 40×40 (20 a bot): Jester, Mason, Warden 20,
  Gambler and Raider 16, Spy 15, Nomad 13, Tactician 3, Voyager 2, Hunter 0; four and twelve players 0/5 each. At
  the browser's budget (20 games a bot) against Tactician, Voyager and Hunter: 20×20 4, 2, 2; 40×40 2, 0, 1. It is
  the strongest bot on 9×9 duels and four-player tables only; the network has seen too little of big boards and crowded tables. Placed after the
  heuristic bots and before Tactician.
- **Terminal outcomes outrank positional scores:** both searches use the rules engine's winner and player-liveness
  state. Elimination is a loss even while other players continue. Opponents who have not placed their first orb
  are still alive. MCTS terminal rewards are exactly one for the winner and zero for everyone else.
- **Bounded scratch memory:** board copies, move lists, queues and search buffers are reused. Survival chain
  estimates use a fixed local byte array to count incoming orbs, including converging branches, without allocating
  a retained buffer on the bot. Each threatened cell propagates once, keeping the opening estimate bounded.
- **Nearest-cell distances use multi-source breadth-first search.** A single grid traversal gives Manhattan
  distances in O(cells), supporting the 40×40 board limit.
- **Seeded randomness:** personality noise, rollout sampling and tie-breaking use `core.RNG`. Stable score ordering
  preserves board order where a strategy has no random tie-break.

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
- **Orb motion** is ported exactly from the prototype's `draw_orbs_on_surface`, but always runs at its largest cell
  (80 px) and is scaled to the board's cell (user request: fixed orb-to-cell ratio). The prototype's fixed pixel
  constants (8 px minimum radius, 12 px orbit margin, separations) made orbs a third of a 24 px cell and stacked
  them on the centre on big boards; now every board shows the 80 px picture: orb radius 17/80 ≈ 21% of the cell,
  orbit up to 11/80. The orbit radius is still smaller than the minimum separation, so orbs in a full cell overlap,
  as they did there.
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
- **Presets read cols × rows**, matching the Columns and Rows steppers. (The Small 9 × 5 … Giant 30 × 18 set was
  replaced on 2026-10-31; see Board presets below.)
- **Sound**: C major pentatonic, C4–A6, each note a sine plus soft 2nd and 3rd harmonics with a 6 ms attack and
  exponential decay, rendered once at startup. One player per note (15 voices, under the 16 limit). Placing an orb
  plays the seat's own note; each wave plays a chord of 1–5 notes by its size, rooted higher as the chain goes on.
  Sounds closer than 100 ms are dropped. Mute is saved.
- **Settings** are JSON in the user config folder (`%AppData%\Orbitrix\settings.json` on Windows), written through
  a temporary file: board size, players, seats (by name), theme, fullscreen, colorblind mode, sound and volume.
  Missing fields keep their defaults. Web and Android storage come in Phase 4.
- **UI scale** is the monitor's device scale factor, capped at 2 since "UI drawn from sprites"; the game draws at
  that resolution. The minimum window
  is 1040×640; when the height runs short, the rail's seat list goes to two compact columns; Setup keeps its
  control sizes and scrolls above Start.
- **Fonts add about 1.4 MB** (Unbounded 0.78 MB, Archivo 0.66 MB, both variable). Subsetting them is a possible
  Phase 4 size saving, without Python tooling.
- **FPS overlay** (frame rate and the slowest frame of the last second) is off by default; F3 toggles it and `-fps`
  starts with it on.
- **Performance meets the Phase 3 bar.** `ORBITRIX_PERF_SECONDS=60 go test -tags perf -run TestPerf -v ./client`
  (its first case; the test now covers more screens, see "UI drawn from sprites") fills a 40×40 board for twelve AIs to two-thirds of its capacity, then plays on in a 1920×1080 window
  with vsync and records every frame (`ORBITRIX_PERF_MARKS=1` turns Colorblind mode on, `ORBITRIX_PERF_NOVSYNC=1`
  measures headroom). On the development desktop at 60 Hz with marks on it held 59.7 FPS over 3,582 frames through
  chains of up to 471 waves: frame gap p50 16.7 ms and p99 17.5 ms, with no frame over 25 ms after the first. Draw
  took 6.5 ms at p99; without vsync it ran at about 200 FPS. One earlier run had a single 1.4 s stall that did not
  reproduce.
- **Allocations:** the client's own update and draw code does not allocate per frame. The drawing helpers take
  `color.RGBA`, not `color.Color`, because boxing a colour into an interface allocates. About 200 allocations a frame
  remain inside Ebitengine, mostly `text/v2` shaping: a few GC cycles a minute and no
  dropped frames on desktop. If Android shows GC hitches, caching drawn labels as images removes the text share.
- **One Back/Menu component** (polish pass): Back on Setup and Settings and Menu on the game rail share the compact
  40 px face, 44 px target, glyph and label offsets; Settings tabs are quiet pills like the other buttons on the
  mat; Results buttons are 52 px like the pause menu (user choice); hover always matches the click target.

## Phase 4

- **One client everywhere** (user request: real reuse, mobile = desktop made responsive). Web and Android run the
  same `client` package and scenes as the desktop; there is no mobile or web UI of its own. `cmd/orbitrix` is also
  the WASM entry point, and `mobile` only starts `client.New()`. Layouts respond to the size they get, measured in
  the safe area: portrait means taller than wide, and compact margins apply below 640 px tall or 600 px wide.
- **Portrait game** (user choice): the board fills the top; under it the spinner with the Chain ×N callout below it
  sits left of the seat table, and a full-width Menu is anchored at the bottom in thumb reach. This replaced an
  earlier top bar (Menu left, callout right) after phone testing (user).
- **Phone type sizes** (user choice): upright screens and screens under 640 px tall set the game's seat rows at the
  15 px label size (13 px when the table splits into two columns) and Chain ×N at Unbounded 800 15 px in a 32 px pill
  sized for "Chain ×88"; the desktop rail keeps 18 px rows and the 20 px callout in its 180×40 pill.
- **Portrait Home** (user choice): wordmark and seat dots on top, live mat in the middle, PLAY and the buttons at the
  bottom.
- **Wide boards rotate the phone** (user request, replacing the turned board on Android): a game whose board has
  more columns than rows asks Android for landscape (`SCREEN_ORIENTATION_SENSOR_LANDSCAPE`, either way up, even with
  auto-rotate off), and every other screen hands rotation back (`UNSPECIFIED`, the phone's own setting). The Go side
  decides per scene (`wantsLandscape`: a game, or Settings opened from its pause menu) and calls `platform.Rotate`,
  which the activity implements. Browsers can only lock orientation in fullscreen and iPhone Safari not at all, so web
  and desktop keep the turned board (user choice): a board wider than tall in an area taller than wide is drawn a
  quarter turn clockwise, which always gives bigger tiles; only its drawing and hit-testing turn.
- **One rail for every platform, as wide as the spinner** (user request). Desktop, browsers and phones on their side
  share `layoutRail`: Menu, spinner, callout and seat table are all the spinner's width, and that width is the least
  the seat table needs. The table takes the first of three set-outs that fits the height (names in one column,
  numbers in one column, numbers in two columns); widths are measured from the text actually shown (`seatTableWidth`,
  which `drawRail` follows), with orb counts sized to the digits of the most orbs the board holds at rest (sum of
  capacity − 1). The seat number always shows (user requirement). Phones skip type names and float the callout over
  the spinner. That can't hold for every case: two columns of numbered rows need about 180–195 px, and a spinner that
  wide plus six 20 px rows doesn't fit under about 400 px of height, so ten or more players on such phones get a
  spinner up to 50 px narrower than the rail (144 vs 195 px on a 780×360 phone with 12 players on 30×18). The
  emulator's 914×411 fits every case at full width. Results with 2 players: 222 px rail on desktop (was 280–340), 116
  px on a phone on its side; the board gets the rest.
- **Small screens:** pause buttons go two by two and the result buttons side by side when the screen is too short.
  Layout tests cover 1040×640 to 1920×1080, 844×390, 780×360, 390×844, 360×740, 412×915 and 768×1024 at scales 1 and
  2.625 with a cutout, for 2–12 players and 9×9 and 30×18 boards: every part inside the safe area, no overlaps, every
  seat row at least as wide as its content, the seat number always shown, and the rail's parts the spinner's width.
- **Input bug fixed from Phase 3:** Ebitengine reports pointer positions on the screen `Layout` returns, which is
  already in device pixels, but the client multiplied them by the UI scale again. Invisible at 100% scaling; on a
  2.6× phone or a hi-DPI browser every click landed far off.
- **Touch:** the first finger is the pointer. It clicks on release and only if it moved less than 10 px; a longer
  drag scrolls Setup. The volume slider starts dragging on press. A lifted finger moves the pointer off screen so no
  hover sticks; the mouse takes over again as soon as it moves. Ebitengine polls touches once a frame, so a tap
  shorter than a frame was never seen (`adb shell input tap` always, and quick taps on a slow phone). The Android
  activity now holds back a lift that comes less than 50 ms after the press (a frame at 20 FPS) until the touch has
  lasted 50 ms, so `input tap` and quick taps work.
- **Android:** `ebitenmobile bind` (pinned as a Go tool in `go.mod`) builds `dist/orbitrix.aar` for arm64 (phones)
  and x86_64 (emulators, Chromebooks); 32-bit ARM is left out. A plain Gradle project (AGP 9.4.1, Gradle 9.8.2,
  compileSdk/targetSdk 37, minSdk 24, no AndroidX) wraps it with one Java activity: immersive full screen (user
  choice; a swipe shows the bars for a moment), drawing under the cutout and reporting its insets as the safe area,
  the back button as Esc (`OnBackInvokedCallback` from Android 13, the key before), suspend/resume, and the rest of
  the config changes handled in place so rotating never restarts the game. Back on Home moves the app to the
  background, as Android expects of a root screen. Settings live in the app's files folder, passed in at start.
  Settings has no Fullscreen row on phones. The release APK is signed with the debug key so it installs directly (a
  store release needs a real keystore), and its native code is stored compressed for a smaller file to pass around.
- **Web:** settings live in `localStorage` (`orbitrix/settings.json`). Fullscreen is not restored at start (browsers
  only allow it after a click) and the Fullscreen toggle shows the real state, since a browser also leaves fullscreen
  on its own Esc. Ebitengine resumes audio after the first gesture by itself. `web/index.html` shows the twelve seat
  dots filling with their inks as `game.wasm` streams in, in the saved theme; `cmd/webdist` writes the uncompressed
  size into the page (a compressed download's length is not the wasm's) and saves `.gz` and `.br` copies of each
  file at the best levels. `server.Static` sends the best copy the browser accepts and makes every file revalidate
  (`wasm_exec.js` and `game.wasm` must come from the same build); `orbitrix-server -web dist/web` serves it now, and
  Phase 5 and the LAN host reuse it. Brotli needs `github.com/andybalholm/brotli`, used only by `cmd/webdist`.
- **Sizes** (Go 1.27.3, Ebitengine v2.10.5, stripped): `game.wasm` 22.2 MB, 5.5 MB gzipped, 3.9 MB brotli.
  `orbitrix.apk` 10.8 MB: the game's native code is 5.2 MB for arm64 and 5.6 MB for x86_64, the Java 18 KB. AGP 9's
  built-in Kotlin is off (`android.builtInKotlin=false`), since it added kotlin-stdlib (0.6 MB) to a Java-only APK.
  The two fonts add 1.4 MB to each build before compression; subsetting them is still open.
- **Verified** on the API 36 emulator (1080×2400, 420 dpi, cutout): the APK (Home, Setup, a game on a turned
  30×18 board, back to pause, landscape rail, Settings, settings kept across restarts, back on Home) and the web
  build in the emulator's Chrome over touch (Setup drag-scroll, a game, settings kept across reloads); the web build
  at desktop size in headless Edge.

## Multiplayer plan change (2026-10-31, before Phase 5)

- **Player-hosted games instead of a central game server** (user decision). The hosting player's client runs
  `server/hub` for online and LAN games alike, and is authoritative. Players connect to the host over WebRTC data
  channels (`github.com/pion/webrtc/v4`, native and `js/wasm`), with STUN at `stun.cloudflare.com:3478` and
  Cloudflare Realtime TURN as the relay when CGNAT or strict NATs block a direct connection. Any client can host,
  web included (the hub runs in the tab's wasm).
- **The Docker container on the NAS only serves the website and signaling** (`/signal`, versioned JSON, in-memory
  state). No game traffic or game logic goes through it.
- **The hub talks only to a `Conn` interface**, with `ws` (LAN), `rtc` (online) and `loopback` (the host's own
  player) implementations in `server/transport`. LAN keeps its plan and uses `ws`.
- **Friends-only access without accounts:** a private room code, host approval of each new joiner, lock and kick,
  and per-player rejoin tokens issued by the host's hub. The code dies when the host leaves. Short-lived TURN
  credentials go only to hosts and approved joiners.
- **Dropped:** SQLite, invites, accounts, device tokens, the admin CLI, bans, server-side rooms, authoritative moves
  and AI seats on the server, spectators, the 200-room load test and the Cloudflare Access option. None of them had
  code yet; only package comments, the `.gitignore` SQLite entries and doc text referred to them.
- **Phases 5–8 rewritten** in CLAUDE.md: Phase 5 transports, hub, signaling and Docker; Phase 6 online screens and
  host safeguards; Phase 7 LAN on `ws`; Phase 8 NAS deploy, signed Android builds and downloads.

## Host password and abuse guards (2026-11-02, before Phase 5)

- **Online hosting needs a host password** (user decision). `HOST_PASSWORD` is one shared secret in the server's env,
  with no accounts and no database. Without it, anyone who found the public URL could create rooms and get TURN
  credentials, and TURN is the only metered Cloudflare service. If the variable is unset, hosting is refused.
  Joining still needs only the room code and host approval, and LAN needs no password. This replaces the brief's
  earlier "no accounts, passwords or invite codes" line.
- **No lockouts** (user requirement, non-negotiable). CGNAT puts many people behind one public IP, so a stranger's
  failures must never lock a friend out. A correct password or room code always passes. Brute force fails on the
  password's length (random, 20+ characters, enforced at startup). Wrong attempts only slow their own socket and
  close it after 3 failures, and per-IP caps exist only against raw floods. This replaces the first draft's limit of
  5 failed passwords per IP per 10 minutes and its per-IP room cap.
- **Abuse and cost guards written into CLAUDE.md §6:** session-length TURN TTLs, a per-room credential cap,
  revocation on kick, leave, grace expiry and room close, room and pending-approval caps, socket limits, a
  tunnel-only container with no published port, an Origin check, no secrets in logs, and a Cloudflare billing
  alert.
- **Rate limiting in two layers** (user request: standard practice, bots must neither flood signaling nor slow the
  site). At the edge, Cloudflare's free plan gives automatic DDoS protection, Cache Rules that serve the website
  from the edge, WAF custom rules that allow only known paths and methods, and one rate limiting rule on
  `/signal`. Bot Fight Mode is off because it can't be skipped for a path and would challenge native clients. At the
  origin, token buckets are keyed by trust (anonymous per IP, authenticated per room), with socket caps, load
  shedding and `http.Server` timeouts. The no-lockout rule still holds at the origin. The one gap left is a bot
  flooding from a friend's exact carrier IP: the edge then blocks new `/signal` connections from that IP while the
  flood lasts. Closing it needs per-client counting (Cloudflare Business or Enterprise).
- **Bot defences reviewed** (the user asked which industry techniques fit).
  - **Adopted:**
    - A Cloudflare WAF rule that keeps Tor and large datacenter ASNs off `/signal`.
    - Adaptive proof of work in our own signaling protocol. It is the only challenge that works for native and web
      clients alike, and it slows floods without blocking anyone.
    - A strict signaling message order.
  - **Rejected:**
    - TLS (JA3/JA4) and HTTP/2 fingerprints. TLS ends at Cloudflare, and fingerprint rules need Enterprise Bot
      Management.
    - JS device fingerprinting, Turnstile and CAPTCHAs, which native clients can't run.
    - Reputation feeds, which are paid.
    - Behavioural bot scores, which would flag game clients that reach `/signal` without loading a page.
- **Signaling reconnect:** the host pings every 20–30 s, because Cloudflare drops idle WebSockets after about 100 s,
  and reclaims its room with `hostSecret` after a drop. The game itself doesn't depend on signaling.
- **Joiners get `iceServers` only after approval.** The brief already ordered approval before the SDP/ICE exchange;
  this makes the TURN side explicit.

## Board presets (2026-10-31)

- **Three shapes, six sizes each** (user request; names and grouping chosen by the user). Portrait, Balanced and
  Landscape stack vertically as plain headings, each above its own 2×3 grid of size tiers. All eighteen presets
  appear in the scrollable list; the shape tabs and their switching implementation are removed. The preview outlines
  only the selected board's shape's presets. The
  tiers have the same name in every shape:

  | Tier | Portrait | Balanced | Landscape |
  |---|---|---|---|
  | Coaster | 5 × 7 | 5 × 5 | 7 × 5 |
  | Placemat | 6 × 9 | 6 × 6 | 9 × 6 |
  | Doormat | 7 × 10 | 7 × 7 | 10 × 7 |
  | Rug | 9 × 13 | 9 × 9 | 13 × 9 |
  | Carpet | 11 × 15 | 11 × 11 | 15 × 11 |
  | Floor | 13 × 18 | 13 × 13 | 18 × 13 |

- **Floor is the largest board that works on every target.** The floor for a cell is 24 px: the prototype's smallest
  cell (`calculate_cell_size`, kept in `cellSize`) and WCAG 2.5.8's minimum touch target. The tightest common screen
  is a 360×780 phone (360 px is the narrowest and 780 the shortest of StatCounter's most common phone sizes in 2026).
  Upright, with a cutout inset, the board gets 328 px across and about 430–485 px down, depending on the seat bar
  (six seats in one column take the most). 328 / 24 allows 13 columns and 430 / 24 about 18 rows. A landscape board
  plays on its side in the Android app (about 505–600 × 328 px beside the rail), but a phone browser turns it upright
  into the same 328 × ~430 px, so landscape presets are the portrait ones turned, and Balanced stops at 13 × 13. A
  phone browser held sideways with its address bar showing has less height (about 21 px cells on Floor); fullscreen
  or holding it upright avoids that. Desktop (1040×640 minimum window) gives every preset 30 px or more.
- **The ladder:** short sides 5, 6, 7, 9, 11, 13, each tier about 1.2× the last (about 1.45× the cells, a geometric
  series from 5 × 5 up to 13 × 13), and long sides about 1.4× the short. Placemat Portrait is 6 × 9, the board the
  original Chain Reaction is most often played on. The default 9 × 9 is Rug Balanced.
- **Choose shape and size together.** Clicking a preset sets both dimensions directly; only that exact preset in
  its shape section is outlined as chosen. Sections have 24 px headings, an 8 px gap, and three 44 px target rows
  with 8 px gaps (180 px per section), with 24 px between sections. The preview background follows and fills the
  visible part of its desktop column: its top and bottom stay inside both the Board body and the scroll viewport
  above Start. The miniature scales with that visible background inside its 12 px padding, preserving its aspect
  ratio and nested outlines. On narrow screens the preview remains above the preset sections.
  Overflow scrolls the page above the pinned Start button, including at the minimum desktop window size.
- **Custom:** the Columns and Rows steppers still allow any size from 3 to 40. A size that matches no preset chooses
  no preset pill and the Board heading reads "Custom" before its dimensions; a preset size shows the preset's name
  there. Boards saved with the old presets load as custom sizes.
- Sources: [StatCounter mobile screen resolutions](https://gs.statcounter.com/screen-resolution-stats/mobile/worldwide),
  [StatCounter desktop screen resolutions](https://gs.statcounter.com/screen-resolution-stats/desktop/worldwide),
  [WCAG 2.5.8 target size (axe rule)](https://dequeuniversity.com/rules/axe/4.8/target-size),
  [Chain Reaction game, usual 9 × 6 board](https://cic.du.ac.in/userfiles/downloads/ProjectBooklets/Chain%20Reaction%20Game.pdf).

## Game rail takes the slack (2026-10-31)

- **The rail fills the width the board can't use** (user request: a wide board left white space on both sides of
  it). The board's shape usually fills the height first, which left a gap left and right of it; now the board keeps
  the left margin, sits one margin from the rail, and the rail grows by the rest. The rail never gets narrower than
  before, so a board that fills the width lays out as it did.
- The spinner grows with the rail, centred, and the callout follows its width. Menu and the seat table span the full
  rail. The responsive table choice below replaces the earlier sizing of rows from the rail's minimum width.

## Responsive seat columns (2026-10-31)

- **Fit the table to the actual rail** (user request: eight players in a single tall column left a wide rail with
  an unnecessarily small wheel; also account for twelve players). After the board takes its space, compare one and
  two columns, with type names when they fit, and choose the largest wheel. Equal wheel sizes prefer names, then
  fewer columns. A second column of names uses spare rail width without taking more room from the board.
- **Keep the original row appearance** (user correction). Desktop rows keep 18 px body text, a 40 px pitch with
  36 px pill faces, and the same dots, padding and spacing in both column layouts. Column selection alone gives
  the wheel more space; neither text nor pills shrink to enlarge it. Phones on their side keep their existing sizing
  and column rules (the upright bar follows below). Seat numbers and orb counts always show. Chain ×N always matches
  the wheel's width and centre.
- Regression coverage includes eight and twelve players at 1919×1004, UI scales 1 and 2.625, and resizing the same
  scene to 1340 px wide and back. For a 9×9 board the wide view uses two named columns and wheels 632 and 552 logical
  px wide respectively; the board stays 940 px wide. The narrower view uses one named column and a 304 px wheel.
  Layout checks enforce the original desktop text and pill sizes for every player count from two to twelve.

## Portrait bar fills the height (2026-11-01)

- **The rail's rule, upright** (user request: square and turned boards left 70–250 px of empty height on phones,
  split above and below the board, beside a wheel stuck at 116 px). The bar under the board starts at its old least
  height, so no board shrinks; the board keeps the top margin and the bar takes every px of height it cannot use.
  The table then picks, among the rail's four set-outs, the one that leaves the largest wheel beside it (`pickWheel`,
  shared with the desktop rail). One column keeps 36 px rows at the label size and two keep 32 px rows at the small
  size, so text never shrinks to grow the wheel; seven or more seats return to one column when it fits.
- **Largest wheel over type names** (user choice). Names cost wheel width beside the table, so on phones they show
  only when the bar's height, not its width, limits the wheel. On a 390×844 phone with a 9×9 board the wheel grows
  from 116 to 236 px for four or eight players (163 px for twelve).
- **Spare height centres the bar** (user choice). When the wheel reaches the width first, the spinner column and the
  table stay centred between the board and Menu, splitting what is left evenly.
- A tall board on a tablet held upright still leaves width beside it: the bar sits under the board, so it can only
  use spare height.

## UI drawn from sprites (2026-10-31)

- **Why:** every frame filled each button, block, dot and icon with anti-aliased `vector` paths. Ebitengine 2.10
  draws each such path 8 times into a stencil texture twice its width, and every text draw between two paths flushes
  that work with a render-target switch, which tile-based phone GPUs pay for heavily. The stencil texture also only
  grows and is cleared in full on every flush, so one full-screen fill (the pause veil, the results flood) slowed
  every later screen. At 1080×2400 on the development desktop Setup took 3.3–3.7 ms a frame, 88% of it in `vector`;
  the Game rail 0.75 ms and Home 0.4–0.7 ms. Scrolling Setup with a 40×40 board took 27.6 ms a frame (36 FPS),
  because the preview resized with the scroll and re-filled one path of 1,600 rounded tiles each step.
- **Sprites:** shapes now come from small white sprites that `vector` renders once and the cache in `theme.go` keeps
  (`queueRound`, `drawShapes`). A rounded rectangle, pill, circle, border or ring is a nine-slice of a rounded-square
  sprite: corners keep the sprite's scale, edges and middle stretch, tinted per draw in one `DrawTriangles32`. A
  stroke with round caps is a turned pill, so icons (arrow, chevrons, hamburger, plus and minus) are a few strokes.
  Seat marks are sprites per seat and size. The spinner's wheel and pointer are images redrawn only when a seat goes
  out, the layout or the theme changes; the pointer turns as it is drawn. The Setup preview queues its tiles into
  one batch. `vector` now only renders sprites and cached images, never the screen.
- **Bounds:** corners above 64 px are rounded up to multiples of 8, and above 512 px the sprite scales up (only the
  results flood, which becomes a plain fill once it covers the screen). The cache starts over after 256 sprites, as
  resizing a window keeps making sizes. With no full-screen `vector` fills left, `screenFit` no longer halves the
  width. A pill or circle skips its empty middle cells (a circle is four quads).
- **2× cap** (user choice): `Layout` caps device px per logical px at `maxScale` = 2. Phones at 2.6–3.5× draw a
  smaller frame that the OS or browser scales up: a 1080×2400 phone fills 823×1829 px, about 40% fewer pixels, for
  slightly softer text. Desktops at 2× or less are unchanged.
- **Draw only after an Update:** displays above 60 Hz call `Draw` more often than the 60 Hz `Update`. The screen is
  no longer cleared every frame, and `Draw` returns at once when no `Update` ran (and the size did not change) since
  the last frame, so the screen keeps it. A 120 Hz phone draws half the frames it did; a 60 Hz display is unchanged.
- **Still screens:** Setup, Settings, paused games and finished results are drawn only when the pointer moves, presses,
  lifts or scrolls, a key the game handles is pressed, the screen changes or resizes, or the FPS overlay is on; every
  30 ticks they redraw anyway, a backstop for a frame the platform lost (an Android app back from the background).
  Idle on Setup, the real game loop drew 7 frames in 3 s instead of about 180.
- **Pause and results:** game orb motion uses the board's 60 Hz clock, so pausing freezes it and resuming does not
  jump ahead. Repeated draws at the same time do not advance smoothing. Once the results wipe covers the screen,
  drawing skips the hidden board and rail; the last frame of the wipe still draws before the screen becomes idle.
- **Labels:** `drawText` keeps each label's glyphs from `text.AppendGlyphs`, laid out around (0, 0) as `text.Draw`
  lays them out before moving them into place, so text draws pixel-identical without shaping or allocating every
  frame (the cache starts over after 512 labels). Tiles, orbs, sprites, marks and the spinner pointer skip
  Ebitengine's mipmap check, as they draw at about their own size; burst rings shrink and keep mipmaps.
  Allocations per frame in the frame-time check fell from 170 to 70; drawing CPU fell about 23% and text CPU about
  half (desktop A/B). On the emulator, still and animated screens now cost the same CPU (about 43%): what is left is
  Ebitengine presenting each frame at 60 Hz, not drawing it.
- **Result** (same desktop, 1080×2400): Setup 0.6–0.7 ms, Game 0.3–0.4 ms, Home 0.24 ms, Setup with a 40×40 board
  1.4 ms; scrolling it at 1440×900 1.8 ms. At the capped 823×1829: Setup 0.62 ms, Game 0.27 ms, Home 0.23 ms (this
  GPU is not fill-bound at phone sizes; phone GPUs are).
- **Frame-time check:** `ORBITRIX_PERF_SECONDS=10 go test -tags perf -run TestPerf -v ./client` runs each screen for
  that many seconds in one window: the dense 40×40 game at 1920×1080 and on a 1080×2400 phone (at the 2× cap), the
  paused game, finished results, idle Setup and Settings, Setup with twelve players and a 40×40 board scrolling up
  and down at 1440×900 and on the phone, and Home on both. Cases use the production `Game.Update`/`Game.Draw` loop
  and an initialized animation clock. A case fails when its p99 frame gap is over 25 ms; idle cases also fail if
  they redraw more than about twice a second. Draw timings count scene redraws, not skipped draws.
- **Android emulator check (2026-11-01):** Android 16 x86_64, 1080×2400 at 420 dpi, drawing at 822×1828 with the
  production 2× cap. A temporary build used the production game loop, seeded a dense 40×40 board with twelve AIs
  and colorblind marks, and measured each screen for ten seconds after two seconds of warmup. Home, scrolling
  Setup, dense play, pause, resumed play and opaque results held 59.9–60.1 FPS; p99 frame gaps stayed below 19.3 ms.
  Dense play/resume draw p99 was 2.73/2.79 ms, scrolling Setup 1.75 ms. Pause and results each redrew 20 times in ten
  seconds, and the board clock stayed at 216 throughout pause before advancing to 936 after resume. This checks
  the Android runtime and rendering path, not a physical phone's GPU speed or thermal limits.
- **Web AI budget:** Go in a browser has one thread, so an AI that is thinking holds up drawing. On web builds, boards
  bigger than the largest preset (Floor, 234 cells) get `ai.DefaultBudget × 234 / cells` work units per move (175 on
  40×40); preset boards keep the full budget.
