---
name: Orbitrix
description: A chain-reaction orb game drawn as a printed party floor mat; colour belongs to the players.
colors:
  mat-vinyl: "#F4F3F0"
  tile-print: "#E0DED9"
  tile-print-hover: "#D1CEC8"
  ink: "#171615"
  ink-hover: "#3B3936"
  dim: "#65615B"
  mat-night: "#1C1A18"
  tile-night: "#2D2A27"
  tile-night-hover: "#3B3733"
  chalk: "#F5F2EE"
  chalk-hover: "#D4CEC6"
  dim-night: "#A39C92"
  mark-white: "#FFFFFF"
  player-vermilion: "#D55E00"
  player-blue: "#0072B2"
  player-green: "#009E73"
  player-orange: "#E69F00"
  player-mauve: "#CC79A7"
  player-sky: "#56B4E9"
  player-yellow: "#E8D21D"
  player-charcoal: "#2E2C2A"
  player-violet: "#901CFC"
  player-brown: "#8B5A2B"
  player-plum: "#440048"
  player-maroon: "#641404"
  player-vermilion-night: "#E86A1A"
  player-blue-night: "#3A9BE0"
  player-green-night: "#1FB88C"
  player-orange-night: "#F0AA1A"
  player-mauve-night: "#DD8DB8"
  player-sky-night: "#7CC6F0"
  player-yellow-night: "#F0E442"
  player-chalk-night: "#F4F4F4"
  player-violet-night: "#844CFC"
  player-brown-night: "#C98B4E"
  player-mint-night: "#B4FC84"
  player-maroon-night: "#B85450"
  danger: "#C21D3A"
  danger-night: "#FF6B7A"
typography:
  display:
    fontFamily: "Unbounded"
    fontSize: "60px"
    fontWeight: 800
    lineHeight: 1
  headline:
    fontFamily: "Unbounded"
    fontSize: "26px"
    fontWeight: 700
    lineHeight: 1
  title:
    fontFamily: "Unbounded"
    fontSize: "17px"
    fontWeight: 700
    lineHeight: 1
  callout:
    fontFamily: "Unbounded"
    fontSize: "20px"
    fontWeight: 800
    lineHeight: 1
  callout-compact:
    fontFamily: "Unbounded"
    fontSize: "15px"
    fontWeight: 800
    lineHeight: 1
  callout-tight:
    fontFamily: "Unbounded"
    fontSize: "13px"
    fontWeight: 800
    lineHeight: 1
  number:
    fontFamily: "Unbounded"
    fontSize: "20px"
    fontWeight: 700
    lineHeight: 1
  body:
    fontFamily: "Archivo"
    fontSize: "18px"
    fontWeight: 500
    lineHeight: 1.2
  label:
    fontFamily: "Archivo"
    fontSize: "15px"
    fontWeight: 700
    lineHeight: 1.2
  small:
    fontFamily: "Archivo"
    fontSize: "13px"
    fontWeight: 500
    lineHeight: 1.2
rounded:
  block: "24px"
  dialog: "28px"
  menu: "18px"
  panel: "16px"
  row: "14px"
  pill: "9999px"
spacing:
  xs: "8px"
  sm: "12px"
  md: "24px"
  lg: "28px"
  xl: "40px"
  xxl: "64px"
  setup-outer: "20px"
  setup-compact: "16px"
  setup-gutter: "16px"
  setup-gutter-compact: "12px"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.mat-vinyl}"
    typography: "{typography.title}"
    rounded: "{rounded.pill}"
    height: "52px"
  button-primary-hover:
    backgroundColor: "{colors.ink-hover}"
    textColor: "{colors.mat-vinyl}"
  button-quiet:
    backgroundColor: "{colors.tile-print}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "52px"
  button-quiet-hover:
    backgroundColor: "{colors.tile-print-hover}"
    textColor: "{colors.ink}"
  button-on-block:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "52px"
  button-on-block-hover:
    backgroundColor: "{colors.tile-print-hover}"
    textColor: "{colors.ink}"
  button-chosen:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
  button-chosen-hover:
    backgroundColor: "{colors.tile-print-hover}"
    textColor: "{colors.ink}"
  setup-block:
    backgroundColor: "{colors.tile-print}"
    textColor: "{colors.ink}"
    rounded: "{rounded.block}"
    padding: "{spacing.setup-outer}"
  setup-block-compact:
    backgroundColor: "{colors.tile-print}"
    textColor: "{colors.ink}"
    rounded: "{rounded.block}"
    padding: "{spacing.setup-compact}"
  setup-start:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.mat-vinyl}"
    typography: "{typography.title}"
    rounded: "{rounded.pill}"
    width: "100%"
    height: "52px"
  setup-seat-picker:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "40px"
  setup-seat-hit-row:
    height: "44px"
  setup-player-count:
    typography: "{typography.number}"
    height: "40px"
  setup-preset:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "40px"
  setup-menu-option:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "40px"
  pause-dialog:
    backgroundColor: "{colors.tile-print}"
    textColor: "{colors.ink}"
    rounded: "{rounded.dialog}"
    padding: "28px"
    width: "356px"
  seat-picker:
    backgroundColor: "{colors.mat-vinyl}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    height: "48px"
  chain-callout:
    textColor: "{colors.ink}"
    typography: "{typography.callout}"
    rounded: "{rounded.pill}"
    width: "180px"
    height: "40px"
  chain-callout-compact:
    textColor: "{colors.ink}"
    typography: "{typography.callout-compact}"
    rounded: "{rounded.pill}"
    height: "40px"
  chain-callout-tight:
    textColor: "{colors.ink}"
    typography: "{typography.callout-tight}"
    rounded: "{rounded.pill}"
    height: "28px"
  status-chip:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.mat-vinyl}"
    typography: "{typography.small}"
    rounded: "{rounded.pill}"
    height: "24px"
  status-chip-busy:
    backgroundColor: "{colors.dim}"
    textColor: "{colors.mat-vinyl}"
  status-chip-off:
    textColor: "{colors.dim}"
  status-chip-danger:
    textColor: "{colors.danger}"
---

# Design System: Orbitrix

## Overview

**Creative North Star: "The Party Mat"**

A printed vinyl floor mat for a loud room of friends. The board is a sheet of printed rounded-square tiles with gutters; the twelve player inks are the only colour on it; every control is plain ink on the vinyl. Everything is drawn in code as flat solid fills: no bevel, glow, gradient or cast shadow. Rejected: the category's neon-orbs-on-black and the purple party-app shell.

Built to be read from the couch. Type is large: desktop rail seat rows use 18 px body, a phone in the hand steps down to the 15 px label. Unbounded, a wide heavy geometric, is only for what people shout: the wordmark, PLAY, Start, Chain ×N, the winner's title. Archivo sets everything else. Two modes, a light vinyl mat (default) and a warm charcoal night mat, share every rule; only values change.

Colour signals ownership and turn, never decoration. Only the mover's tiles light up, the chain callout is in the mover's ink, and the winner's ink floods the screen. Colorblind mode adds each player's shape mark wherever their colour appears.

**Key Characteristics:**
- Flat solid fills only: no 3D, no shadow depth, no gradients, no neon.
- Colour belongs to the players. Chrome is ink (light) or chalk (dark) on the mat.
- Solid ink only for the one primary action on a screen.
- Pills, dots and rounded-square tiles; no sharp rectangles in the chrome.
- Plain, short labels; no key hints on buttons, no explanatory subtext.
- Menus as in most games: one Menu button opens a pause dialog; Settings is tabbed (Video, Audio, Network).

## Colors

Two quiet neutral sets (light vinyl, night mat) carry twelve saturated, colour-blind-tuned player inks. Neutrals lean faintly warm, never blue, in ground, tiles, ink and dim of both modes. Blue appears only as the Blue and Sky inks.

### Primary
- **Press Ink** (`ink`, light) / **Chalk Ink** (`chalk`, dark): all text, outlines, the PLAY dot, Start and Resume, toggles that are on, slider fills, the spinner pointer, orb-count badges. Hover: **Worn Press Ink** / **Worn Chalk Ink**.

### Secondary
- **The twelve player inks**, in fixed seat order: Vermilion, Blue, Green, Orange, Mauve, Sky, Yellow, Charcoal (Chalk in dark), Violet, Brown, Plum (Mint in dark), Maroon. The first eight are Okabe–Ito. Violet, Plum/Mint and Maroon were searched under simulated protan, deutan and tritan vision: no pair is closer than Okabe–Ito's own closest (Orange/Yellow), each clears 3:1 against tile and mat, none is a second shade of another. Each mode has its own tuned set (`-night` tokens). Used only for orbs, seat dots, spinner wedges, the 30% turn tint, burst rings, the chain callout and the winner flood.

### Neutral
- **Vinyl White** (`mat-vinyl`) / **Charcoal Mat** (`mat-night`): every screen's ground, the fill of quiet controls on a tile block, and the rim around every orb.
- **Printed Tile Grey** (`tile-print`) / **Night Tile** (`tile-night`): empty tiles, Setup blocks, the Settings panel, the pause dialog, quiet buttons on the mat, the current-player row highlight. Hover uses the **Pressed** variant.
- **Faded Ink** (`dim`) / **Faded Chalk** (`dim-night`): secondary text, out players (an empty ring), dashed preset outlines, toggles that are off, disabled labels.
- **Mark White** (`mark-white`): shape marks and chain-callout text on dark inks. `inkOn` picks Press Ink or Mark White, whichever contrasts more with the fill.

### Named Rules
**The Players Own Colour Rule.** Hue appears only where it identifies a player. Chrome never borrows a player ink for emphasis, hover or decoration.

**The Turn Light Rule.** Only the player to move gets lit tiles (tile mixed 30% toward their ink). Every other owned tile stays printed blank; ownership shows through the orbs.

## Typography

**Display Font:** Unbounded (variable, embedded, OFL)
**Body Font:** Archivo (variable, embedded, OFL)

**Character:** Unbounded is wide and heavy, poster lettering on a party mat. Archivo is a sturdy grotesque, readable at couch distance. Sizes are logical px and scale with the UI scale.

### Hierarchy
- **Display** (Unbounded 800, wordmark capped at 60 px): the ORBITRIX wordmark. The same face sets PLAY (34% of the dot radius) and the winner's title (56 to 120 px, scaling with the window).
- **Headline** (Unbounded 700, 26 px): screen titles ("Set up the game", "Settings", "Paused").
- **Title** (Unbounded 700, 17 px): block headings (Players, Board) and primary button labels.
- **Callout** (Unbounded 800, 20 px): "Chain ×N" in the mover's ink. **Callout compact** (15 px) in portrait; **Callout tight** (13 px) in landscape under 640 px tall, with less vertical padding to give the wheel room.
- **Number** (Unbounded 700, 20 px): stepper values and board dimensions.
- **Body** (Archivo 500 or 700, 18 px): desktop rail seat rows, Settings row labels, the volume value, the results line.
- **Label** (Archivo 700 or 500, 15 px): pill labels, seat tags, dropdown options, phone seat rows (and two-column rows on the desktop rail).
- **Small** (Archivo 500 or 700, 13 px): preset dimensions after a preset name, two-column phone seat rows, the F3 overlay.

### Named Rules
**The Shout Rule.** Unbounded only for words the room says out loud and for numbers. Everything read quietly is Archivo.

## Layout

Computed in code from the window size and UI scale `u`; no grid framework. Margins: Settings 40 px, Home 64 px, the game 32 px each side. Spacing steps: 8, 12, 24, 28, 40, 64 px. Rows 8–12 px apart, groups 24–28 px, outer margins 40–64 px.

One layout for every target (desktop, browser, Android), responding to its size; no separate mobile UI. Everything sits inside the **safe area** (the screen minus Android's cutout); only full-screen fills (mat, pause veil, winner flood) reach past it. **Portrait** = safe area taller than wide. Under 640 px tall or 600 px wide: compact 16 px margins and tighter gaps. Touch targets never go below 44 px.

- **Home:** a wordmark column on the left (at least 420 px, or 36% of the width): the wordmark, the twelve seat dots spread exactly across its width, the PLAY dot (radius 64 to 150 px), and quiet pills (Settings, then Exit where the platform allows), each as wide as the PLAY dot up to 240 px. The rest is a live AI-vs-AI demo mat. No tagline or facts line. On a screen too short for the column (a phone on its side) gaps tighten and the PLAY dot shrinks, to 40 px radius at least. **Portrait:** wordmark (24 px side margins) and seat dots on top, the mat filling the middle, the PLAY dot (radius up to 11% of the height) and buttons at the bottom within thumb reach.
- **Welcome plaque:** above PLAY, "Welcome, Ana!" after PopCap's greeting: bare words in the buttons' label face, the name in ink between the dim greeting and its mark, no fill, no border (a pill reads as a button). It hugs its line (at least the buttons' width); a tap gives it the keyboard with a caret after the name; "Human" stays dim until a name is given. Nothing shrinks for it on short landscape phones: only the column's gaps tighten, and the ways to play take its room while they show.
- **Home, choosing:** PLAY swaps the dot and its buttons, in place, for the ways to play: solid ink pills (This Device; Same Wi-Fi in every native build, never in a browser; Online where the build has a server) and a quiet Back, 52 px tall, at least 220 px wide, centred where the dot and stack stood. No "local multiplayer" label (it reads as couch co-op). With only This Device, PLAY opens Setup directly. On Windows the first Same Wi-Fi tap opens a pause-style dialog: "Windows will ask to let Orbitrix on your network. Allow it." in Heading, then Continue (ink) above Back.
- **Join pages (Same Wi-Fi, Online):** Setup's frame (Back, headline, 20/16 px margins), one centred column (at most 640 px) on every target: a Host block (Set up a room, a quiet pill on the block) above a Join block of two groups, each under a Label Bold heading, split by the 1.5 px Settings hairline. "Rooms nearby" (Same Wi-Fi) or "Public rooms" (Online) lists public rooms as 40 px quiet pills: name in Label Bold on the left (host device's name on Same Wi-Fi, board preset and size online), status chip on the right (Open, Full or Playing; Joining while joining it), concentric with the row's end. A tap joins; a room with no seat for this player is disabled. Empty: "Looking for rooms" or "No public rooms", dim, in the first row's place. Then "Join by code": six code slots (printed tiles; the next one outlined while typing), a status line (Connecting, Waiting for the host, or why the last try failed) and Join, which reads Cancel during a join.
- **Room:** the host's room is This Device's Setup (Players, Board, the same seat dropdown and presets), titled "Set up the room", then "Room" once open. Start narrows to leave 52 px plus the gutter for a key button at its right: a 52 px round quiet button on the mat (tile fill, pressed tile on hover) with a drawn key, for access and code. Held seats show their state where an AI shows pips; the host's own seat has no dropdown or chevron. The action reads Open room (Opening, Board too small, Needs a human seat, or why it failed), then Start, Start with bots or Waiting for players. A joiner sees only a Players block (40 px pills: dot, label with " (you)" on theirs, state or pips) and Ready or Not ready (quiet).
- **Room dialog:** the pause dialog style (tile panel, 28 px radius and padding, 80% mat veil, Title "Room"). Public and Private side by side as 52 px pills, the draft outlined. Private shows the code at once in the join page's six read-only tiles across the inner width; a saved LAN code is reused across games and access changes. Confirm (ink) beside a 52 px round copy button along the foot (ground fill on the tile panel, pressed tile on hover, two 12 px sheets, a check for 2 s after copying). Public: a full-width Confirm, no copy button. "Reconnecting to the server" in Label dim. The choice applies only on Confirm; Esc or a tap outside drops it. The panel's top stays where the private dialog's is, so choosing never moves buttons under the pointer.
- **Seat dropdown, Swap position and Remove player:** in Setup, local and in a room. Swap position sits above Remove player after a hairline; a tap moves this player (the seat carrying their name, the host's in a room) straight into the dropdown's seat, which takes theirs. It is greyed out on their own seat and on another human's: two humans never trade. Remove player is the last option, in the danger ink (light C21D3A, dark FF6B7A) with a 14% danger hover. It takes the seat out, bot or human; later seats move up with their choices (and players in an open room). Greyed out at two players. The danger ink is the one colour outside the player inks, used only here and on the Danger status chip.
- **Join request snackbar:** an online host's requests, one at a time, oldest first, as a 56 px pill centred just above the action: the page's inverse (ink fill, mat-coloured words, flipping with the theme), the body's width on a phone and at most 560 px from 600 px wide. "Human wants to join" in Label Bold, a dim "+N" when more wait, a quiet No (16% mat-coloured fill on hover) and Let in as a 96 × 40 px mat-coloured pill. Each rises 16 px and fades in over 12 ticks (ease-out). While shown, the page gains room at its foot.
- **Host password:** Setup's frame; one block (at most 640 px, centred both ways while it fits) with the masked field (a mat pill, bullets, the end showing on overflow; chosen outline and a 2 px ink caret with focus), a Show/Hide pill, and "Wrong password" after a refusal. Focused on open. Open room is the bottom action.
- **Room ended:** the reason centred in Headline ("The host closed the room", "The host removed you", "Connection lost", …), then Rejoin (primary, only after a dropped connection) and Main menu (quiet), 240 × 52 px.
- **Setup:** Back pill and headline on top, the Players block above the full-width Board block, and Start across the content width (52 px), anchored to the bottom margin outside the scrolling content. Margins and block padding 20 px, 16 px below a 1200 px window width; gutters 16 px, 12 px below the same breakpoint. Roster and board options scroll (wheel or drag) above Start whenever full-size controls exceed the viewport; nothing squeezes.
- **Setup roster:** active seats only, one to four columns. Minimum column width starts at 220 px and grows to fit the longest strategy, its pips, padding and chevron. Columns split the body width equally after the 12/16 px gutters; pills fill their column, no width cap; a short final row keeps the grid widths; one column fills the body. With twelve players, 1040, 760, 640 and 390 px windows fit four, three, two and one columns (four-column pills at 1040 px are about 235 px). The heading groups −, count, + (2 to 12); no Add player chip. Row pitch 52 px (44 px target plus 8 px gap). Each seat is one 40 px pill (ink dot, Human/AI choice, an AI's pips, chevron), inset 2 px in its 44 px hit row. Setup uses the compact Back/Menu role and the shared fill hover; Home and pause-menu quiet buttons keep 52 px faces. Count changes keep existing choices (`fitSeats`); new seats take AI defaults.
- **Setup board:** the mini-mat preview stays in the Board block at every size, with the chosen shape's presets as nested outlines; its frame spans that shape's largest preset (Floor) or the chosen board, whichever is bigger. Portrait, Balanced and Landscape are stacked sections with plain headings, each six preset pills in two columns and three rows (Coaster, Placemat, Doormat, Rug, Carpet, Floor). No shape tabs: all eighteen presets scroll, and choosing one sets shape and size. Each section is 180 px (24 px heading, 8 px gap, 44 px targets with 8 px row gaps), 24 px apart; faces stay 40 px. The Board heading reads the preset's name, or "Custom", before the dimensions. Presets sit above the Columns/Rows controls in one aligned group, full body width when stacked. Beside the preview the group aims for 42% of the body up to 420 px, its measured minimum taking precedence. When the preview's 200 px minimum, the group's minimum and the gutter fit, the preview's background fills only the visible part of the left Board column, its top and bottom following that column's intersection with the scroll viewport, and the miniature scales inside 12 px padding keeping its aspect and outlines. Otherwise the preview sits above the group. Each dimension group is one 44 px row: label, −, value, + vertically centred; its minimum width is the wider of the Columns/Rows labels, an 8 px gap, two 44 px targets and the widest cached number with 12 px padding. The pair shares a line when it fits; below 600 px viewport width, or when it can't fit, full-width Rows sits under full-width Columns with the 12/16 px gutter. The same layout is the phone layout.
- **Settings:** Back pill and headline, tab pills (124 × 44 px), one panel down to the bottom of the page. Rows 76 px with hairlines; compact screens use 16 px margins and padding and 56 px rows. Phones have no Fullscreen row.
- **Game (desktop, browser, phone on its side):** the board fills the left column; the rail stacks Menu, turn wheel, chain chip, player list. The wheel fills 100% of the rail width, sharing width and centre with Menu, the chip and the player viewport. Rail width is bounded by vertical capacity so the wheel, controls, pointer-safe gaps and one player row fit; extra width stays with the board. The wheel never shrinks for the roster. Players scroll inside a clipped viewport below the chip. One or two columns are chosen from actual text widths (two for more than six seats only when they fit), comparing layouts for the largest wheel: equal wheels prefer type names, then fewer columns. Text and pills never shrink to grow the wheel. Desktop: 32 px margins, 24 px below Menu, 20 px below the wheel, 24 px before the list, 40 px faces, 40 px pitch, 36 px pills. Landscape under 640 px tall: 16 px margins, 12 px gaps, 28 px Menu and chip faces, 13 px chain text, 32 px pitch; Menu keeps its 44 px target.
- **Seat rows:** inset, dot, space, type, right-aligned orb count or the Out chip (24 px; 20 px on 32 px pitch). Desktop: 14 px insets, 12 px dots, 12 px space, 18 px body, at least 12 px before the count. Compact one-column: 10 px insets, 10 px dots, 8 px space, label type. Compact two-column: 6 px insets, 8 px dots, 4 px space, at least 6 px before the count, small type. Columns 8 px apart; faces 4 px shorter than the pitch. Overflow clips and scrolls with a proportional indicator; wheel and touch drag share one scroller, and a drag stays with the viewport it started in. A turn change reveals the current player; manual scrolling stays put until the turn or geometry changes. Long counts fit the number column without crowding the type or changing the layout.
- **Game, portrait:** the board keeps at least 16 px margins and fills the width with square cells while its full height fits above the players and controls; otherwise it scales down uniformly, centred, every row visible, no board scrolling. Board and player list anchor together above the fixed bottom controls with 16 px gaps; spare height goes above the board. The player viewport is no taller than its content: one column at 36 px pitch, or above six seats when text fits, two at 32 px. Only the list scrolls; board cells take no input through the list or footer. The bottom row holds the chain chip, a current-player dot showing its exact orb count, and Menu, 8 px apart; the dot sits at the row's exact centre with equal-width chip and Menu areas. No wheel and no repeated type label in portrait. In Colorblind mode the dot's shape sits in a small badge at its top edge so the number stays readable. Menu and chip keep 40 px faces; Menu keeps its 44 px target.
- **Orientation:** on Android a board with more columns than rows turns the phone to landscape (either way up), its pause Settings included; leaving the game hands rotation back. Where the app can't turn the screen (browsers, desktop windows), such a board in an area taller than wide is drawn a quarter turn (see Board).
- **Pause and Results on small screens:** pause buttons narrow to fit and go two by two when one column is too tall. The winner's title shrinks so the longest seat label plus " wins!" fits; on a screen too short the winner's dot shrinks (24 px radius at least) and the result buttons stand side by side; the results line drops to label size when body won't fit.
- **Board:** a 7% gutter on each side of every cell; the board scales to fill its area with the outer tiles on the area's edges. A board whose shape differs from its area is centred, the slack split evenly, on the game screen and Home alike. Where the app can't turn the screen, a board wider than tall in an area taller than wide is drawn a quarter turn clockwise, long side down the screen, tiles big enough to tap; only the drawing turns.
- **Web loader:** before the game starts, the twelve seat dots (16 px, 12 px apart) on the mat in the saved theme, each filling with its ink as another twelfth of the game arrives. No text unless loading fails.

## Elevation & Depth

Flat. No shadows, bevels or gradients. Depth comes from layered fills: mat, then tile or block, then a ground-coloured control on the block, then ink. Orbs get a rim in the mat colour so overlapping orbs read as separate flat pieces: a cut-out edge, not a shadow. The pause dialog sits on a flat 80% mat veil. Motion uses opacity and size, never blur: burst rings expand and fade, the chain callout fades over 20 ticks, the winner's ink floods out from the board centre as an eased circle (40 ticks).

### Named Rules
**The Flat Vinyl Rule.** A layer that must stand out gets a different flat fill or an ink outline. Never a shadow, glow or gradient.

## Shapes

Pills, dots and rounded squares. Every button, tag, toggle, slider track, callout and badge is a full pill. Home's primary actions are big circles (the PLAY dot), and every seat is a dot. Board tiles are rounded squares with a 20% corner radius. Containers: 24 px blocks and the Settings panel, 28 px pause dialog, 18 px dropdowns, 16 px preview panel, 14 px hovered settings rows. Outlines: 2.5 px means chosen, 1.5 px a dim edge. Quiet buttons change fill on hover with no border; chosen borders persist. Dashed outlines mean a potential slot (presets in the preview). Icons are stroked paths with round caps (back arrow, chevrons, hamburger, plus, minus, key, copy); no glyph icons.

**The Twelve Marks.** One shape mark per seat, in Press Ink or Mark White by contrast: dot, bar, plus, cross, triangle, square, diamond, stem, pair, ring, chevron, frame.

## Components

### Buttons
Plain pills: solid where they act, quiet where they offer.
- **Shape:** full pill (`rounded.pill`), 40 or 52 px tall.
- **Primary (`button-primary`):** solid ink, mat-coloured label in Unbounded 700. One per screen (Start on Setup, Resume in the pause dialog). Hover lifts to `ink-hover`. Disabled: tile fill, dim label.
- **Setup Start (`setup-start`):** primary style, 52 px, full content width, pinned below the scroll viewport. Reads "Board too small" and disables when players outnumber cells.
- **Compact Setup controls:** Back and the faces inside Players/Board use the compact 40 px role in 44 px targets, distinct from the 52 px Home/pause quiet buttons and the 52 px Start footer.
- **Quiet on the mat (`button-quiet`):** tile fill, ink label in Archivo 700: Home's Settings/Exit, Back, Menu, unselected tabs. Hover: pressed tile fill.
- **Quiet on a block (`button-on-block`):** mat fill on a tile block; hover pressed tile, like Home Settings/Exit, Back and Menu, with no outline. Disabled: tile fill, dim label, no hover.
- **Chosen (`button-chosen`):** mat fill with a persistent 2.5 px ink outline; hover pressed tile, border kept. The selected tab, preset and Light/Dark option.
- **On the winner's ink (Results):** Play again filled in the contrasting ink; Main menu and Exit as 2.5 px outlines in it. All 220 × 52 px, 12 px apart. Exit hidden on web; online only the host gets Play again (back to the lobby).
- **Second tap:** Restart turns solid ink reading "Restart?" for 3 s before it acts.
- **PLAY dot:** solid ink circle, PLAY in the mat colour.

### Cards / Containers
- **Corner Style:** 24 px blocks and panel, 28 px dialog.
- **Background:** tile fill on the mat; no border, no shadow.
- **Internal Padding:** 24–28 px; Setup blocks `setup-block` (20 px) or `setup-block-compact` (16 px) below the compact breakpoint.

### Inputs / Fields
- **Setup seat picker (`setup-seat-picker`):** one mat-coloured 40 px pill in a 44 px hit row on the Players block, Archivo 700 at 15 px. Human seats and an open picker keep the 2.5 px chosen outline; hover pressed tile. 16 px colour dot 16 px from the left, the choice left-aligned at 44 px, an AI's five pips ending 36 px from the right, chevron 20 px from the right. No number tags: a seat is called by its type ("Human", "Raider"), or by its player's name. The dropdown is a mat panel (18 px radius, 2 px ink outline) with 44 px rows and 40 px faces; the selected option is solid ink, others hover pressed tile without outline; every AI option ends in its pips 14 px from the right. Two columns when the 408 px single column (nine rows plus padding) won't fit the window height.
- **Setup player count (`setup-player-count`):** −, count, + together in the Players heading; 40 px circular faces in 44×44 px targets, shared Ground style (mat fill, pressed tile on hover, tile fill with dim marks and no hover when disabled). Disabled at two or twelve.
- **Setup presets (`setup-preset`):** mat-coloured 40 px faces in 44 px rows, centred Archivo 700 at 15 px, 8 px row gaps; hover pressed tile; the chosen preset keeps its 2.5 px outline. Only the preset matching both shape and size is chosen; a custom size chooses none.
- **Stepper:** stroked − and + in 40 px circular Ground faces in 44×44 px targets, value in Number between. Disabled: tile fill, dim marks, no hover.
- **Toggle:** on is an ink track with a mat knob on the right; off a mat track with a dim outline and dim knob on the left. Hover turns the outline ink.
- **Slider:** mat track, ink fill, 12 px ink knob. All dim while muted.
- **Text fields:** no drawn keyboard; fields take the device's own (physical, or the on-screen one Android and phone browsers show, via Ebitengine's `exp/textinput`). Focus shows as the 2.5 px chosen outline. Pressing a field focuses it; on touch, a tap elsewhere or closing the keyboard unfocuses. With a mouse the room code takes typing as soon as the page opens; on touch its keyboard opens when the slots are pressed. M types instead of muting while a field has focus. On Android the code keyboard offers capitals without suggestions; the password keyboard neither suggests nor learns.
- **Difficulty pips:** five 5 px dots 9 px apart, the strength filled in the label's ink, the rest at 25%: Jester, Mason, Warden 1; Gambler, Nomad 2; Raider, Spy, Prodigy 3; Tactician, Voyager 4; Hunter, Sovereign 5. Human shows none.
- **Status chip (`status-chip`):** a state word ending a row, a 24 px pill (4 px inside the row's face on rows too short), Archivo 700 at 13 px, padded 40% of its height each side, first letter capitalised. State is coded by weight, so it reads the same in Colorblind mode, both themes and on mat, tile or pressed tile; its one hue is the danger ink. **On** (solid ink, mat word): the way is open: Open (a room), Ready, Joining. **Busy** (Faded Ink fill, mat word): Playing. **Off** (1.5 px dim outline, dim word): nobody is in it or can get in: Full, Away, Open on an unheld room seat. **Danger** (1.5 px danger outline and word): Out, and Not ready, which holds the start. Used on game seat rows, join-page room rows and Room seat rows.
- **Seat left (host, online):** pause-dialog style titled "<seat> left", with Hand to a bot (primary) and Skip their turns.

### Navigation
- **Back and Menu:** compact tile pills, 40 px face in a 44 px target, glyph centred 22 px from the left edge, label at 42 px. On short landscape rails Menu and the chain chip use the tight 28 px face; Menu keeps its 44 px target. Back is 112 px on Setup and Settings (arrow only, 44 px, under 540 px wide); Menu spans the landscape rail or the footer width beside the portrait turn dot.
- **Touch:** a finger acts on release, only if it dragged under 10 px, so a drag scrolls without pressing; the volume knob follows a finger from the moment it lands. A lifted finger leaves no hover. Android back acts as Esc; on Home it sends the app to the background.
- **Menu pill** (hamburger + "Menu") opens the pause dialog: Resume (primary), Restart (second tap), Settings, Main menu (leaves at once). Online the dialog is titled Menu (the game goes on) with Resume, Settings, Main menu; the host's Main menu closes the room.
- **Settings tabs:** Video, Audio and Network (Same Wi-Fi rooms and Online rooms as Public/Private pairs in the Light/Dark style, chosen one outlined; Force relay in builds that play online; the tab is dropped where neither kind of room exists), as pills (chosen style for the active tab, tile fill otherwise).
- **Keys:** only Esc, M, Space and F3. No key is ever printed on a button.

### Spinner (signature)
Landscape only: a pie with one wedge per seat in its ink, split by 3 px mat-coloured lines; out seats are blank tile. Its diameter equals the rail width. A mat-coloured hub and an ink pointer at the rim aimed at the mover, with a 2 px mat-coloured outline so it shows over every wedge in both modes; the tip stays within its 12 px clearance beyond the wheel. Each living wedge shows its exact orb count, upright Archivo bold, in the outer band; Colorblind marks take a separate inner band; the pointer stays clear of both. Out wedges show neither. Portrait uses the centred current-player dot beside Menu with the same exact count. Both fit values through five digits with the same cached glyphs and uniform scaling, no abbreviation or truncation; shorter values grow back to the regular size. Counts follow the displayed board during chain playback without rebuilding the cached wheel. The player list keeps the larger count reference.

### Chain Callout (signature)
A pill in the mover's ink reading "Chain ×N" in Unbounded 800, `inkOn` text. In landscape it matches the wheel's width and centre and sits below it with pointer clearance, never over it. In portrait it fills the footer left of the turn dot. Its height matches Menu: 40 px on desktop and in portrait, 28 px in short landscape. Shown for chains of 2 or more, then fades out.

### Board, Orbs and Bursts (signature)
Tiles are rounded squares, printed blank or lit 30% toward the mover's ink. Orbs are flat discs in their owner's ink with a mat-coloured rim (plus their mark in Colorblind mode), all in one batch. A burst is a flat ring in the exploding player's ink, expanding over 18 ticks and fading. The hovered tile gets an outline in the mover's ink. A tap the mover can't make (someone else's tile, or not their turn) flashes that tile's outline in the mover's ink, fading over half a second. A cell with more than four orbs gets an ink count badge.

### Winner Flood
The winner's ink expands from the board centre to cover the screen. On it: the winner's dot, solid in the contrasting ink (its mark in the flood ink in Colorblind mode, so it never reads as an out seat's ring), the title in Display, a body line (move count, then the biggest chain from ×2), and the result buttons.

## Do's and Don'ts

### Do:
- **Do** keep every fill flat and solid; separate layers with a different fill or an ink outline. Quiet hover is pressed tile; chosen keeps a 2.5 px ink outline.
- **Do** keep hue for players: orbs, seat dots, wedges, the 30% turn tint, bursts, the chain callout, the winner flood.
- **Do** use solid ink for exactly one primary action per screen, quiet tile or mat pills for the rest.
- **Do** pick text and mark colour on a player ink with `inkOn`.
- **Do** add the seat's shape mark wherever its colour appears in Colorblind mode.
- **Do** define every value for both modes.
- **Do** ask the user before making any new visual decision.

### Don't:
- **Don't** use 3D, shadow depth, gradients, glows or neon.
- **Don't** colour chrome with a player ink, or add an accent colour outside the twelve (the danger ink is the one exception, for Remove player and the Danger chip).
- **Don't** print key hints on buttons or add explanatory subtext.
- **Don't** bind keys beyond Esc, M, Space and F3.
- **Don't** build menus that differ from game convention.
- **Don't** use glyph or emoji icons; draw icons as stroked paths.
