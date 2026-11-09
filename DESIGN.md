---
name: Orbitrix
description: A chain-reaction orb game drawn as a printed party floor mat; colour identifies players and map relationships.
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

A printed vinyl floor mat for a loud room of friends. The board is a sheet of printed rounded-square tiles with gutters; twelve orb inks identify players and map relationships. General controls are plain ink on the vinyl. Everything is drawn in code as flat solid fills: no bevel, glow, gradient or cast shadow. Rejected: the category's neon-orbs-on-black and the purple party-app shell.

Built to be read from the couch. Type is large: desktop rail seat rows use 18 px body, a phone in the hand steps down to the 15 px label. Unbounded, a wide heavy geometric, is only for what people shout: the wordmark, PLAY, Start, Chain ×N, the winner's title. Archivo sets everything else. Two modes, a light vinyl mat (default) and a warm charcoal night mat, share every rule; only values change.

Colour signals ownership, turn, or a map relationship, never decoration. Only the mover's tiles light up, the chain callout is in the mover's ink, and the winner's ink floods the screen. Colorblind mode adds player and orb shape marks elsewhere; all map art stays identical with Colorblind enabled or disabled, without identity stamps.

**Key Characteristics:**
- Flat solid fills only: no 3D, no shadow depth, no gradients, no neon.
- The same twelve orb colors identify players, Link pairs and switch circuits. General chrome is ink (light) or chalk (dark) on the mat.
- Solid ink only for the one primary action on a screen.
- Pills, dots and rounded-square tiles; no sharp rectangles in the chrome.
- Plain, short labels; no key hints on buttons. Host and Join stay visible without descriptions. Only the browser's Join group may explain a limitation and next step, without jargon or center dots.
- Menus as in most games: one Menu button opens a pause dialog; Settings is tabbed (Video, Audio, Network).

## Colors

Two quiet neutral sets (light vinyl, night mat) carry twelve saturated, colour-blind-tuned orb inks. Map groups reuse those exact inks in the same order. Neutrals lean faintly warm, never blue, in ground, tiles, ink and dim of both modes.

### Primary
- **Press Ink** (`ink`, light) / **Chalk Ink** (`chalk`, dark): all text, outlines, the PLAY dot, Start and Resume, toggles that are on, slider fills, the spinner pointer, orb-count badges. Hover: **Worn Press Ink** / **Worn Chalk Ink**.

### Secondary
- **The twelve player inks**, in fixed seat order: Vermilion, Blue, Green, Orange, Mauve, Sky, Yellow, Charcoal (Chalk in dark), Violet, Brown, Plum (Mint in dark), Maroon. The first eight are Okabe–Ito. Violet, Plum/Mint and Maroon were searched under simulated protan, deutan and tritan vision: no pair is closer than Okabe–Ito's own closest (Orange/Yellow), each clears 3:1 against tile and mat, none is a second shade of another. Each mode has its own tuned set (`-night` tokens). Used for orbs, seat dots, spinner wedges, the 30% turn tint, burst rings, the chain callout, the winner flood, and map relationship groups.
- **Map groups:** exactly twelve color choices, taken directly from the twelve player inks in `scheme.players`. Group ink highlights the physical components; their neutral housings, borders, and dark details remain. Map devices, terrain, Gates and relationship swatches keep the same normal art and colors with Colorblind enabled or disabled. Neither mode adds binary notches, dots, or black/white identity decoration. No partner or group numbers are printed. The relationship picker shows all twelve choices on one page.

### Neutral
- **Vinyl White** (`mat-vinyl`) / **Charcoal Mat** (`mat-night`): every screen's ground, the fill of quiet controls on a tile block, and the rim around every orb.
- **Printed Tile Grey** (`tile-print`) / **Night Tile** (`tile-night`): empty tiles, Setup blocks, the Settings panel, the pause dialog, quiet buttons on the mat, the current-player row highlight. Hover uses the **Pressed** variant.
- **Faded Ink** (`dim`) / **Faded Chalk** (`dim-night`): secondary text, out players (an empty ring), dashed preset outlines, toggles that are off, disabled labels.
- **Pit** (`pit`) / **Night Pit** (`pit-night`): map Holes and Spillway pockets, darker than every printed surface.
- **Mark White** (`mark-white`): shape marks and chain-callout text on dark inks. `inkOn` picks Press Ink or Mark White, whichever contrasts more with the fill.

### Named Rules
**The Functional Colour Rule.** The twelve orb inks identify players and map relationships. Map groups keep identical normal art and color in both Colorblind states; player and orb symbols elsewhere stay unchanged. General chrome never borrows an orb ink for emphasis, hover or decoration; the pair/circuit swatch is a functional relationship control.

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

One layout for every target (desktop, browser, Android), responding to its size; no separate mobile UI. Everything sits inside the **safe area** (the screen minus Android's cutout); only full-screen fills (mat, pause veil, winner flood) reach past it. **Portrait** = safe area taller than wide. Under 640 px tall or 600 px wide: compact 16 px margins and tighter gaps. General touch controls use at least 44 px targets; the editor's denser desktop palette is specified separately below.

- **Home:** a wordmark column on the left (at least 420 px, or 36% of the width): the wordmark, the twelve seat dots spread exactly across its width, the PLAY dot (radius 64 to 150 px), and quiet pills (Settings, then Exit where the platform allows), each as wide as the PLAY dot up to 240 px. The rest is a live AI-vs-AI demo mat. No tagline or facts line. On a screen too short for the column (a phone on its side) gaps tighten and the PLAY dot shrinks, to 40 px radius at least. **Portrait:** wordmark (24 px side margins) and seat dots on top, the mat filling the middle, the PLAY dot (radius up to 11% of the height) and buttons at the bottom within thumb reach.
- **Welcome plaque:** above PLAY, "Welcome, Ana!" after PopCap's greeting: bare words in the buttons' label face, the name in ink between the dim greeting and its mark, no fill, no border (a pill reads as a button). It hugs its line (at least the buttons' width); a tap gives it the keyboard with a caret after the name; "Human" stays dim until a name is given. Nothing shrinks for it on short landscape phones: only the column's gaps tighten, and the ways to play take its room while they show.
- **Home, choosing:** PLAY swaps the dot and its buttons, in place, for the ways to play: solid ink pills (This Device; Same Wi-Fi in every native build, never in a browser; Online where the build has a server) and a quiet Back, 52 px tall, at least 220 px wide, centred where the dot and stack stood. No "local multiplayer" label (it reads as couch co-op). With only This Device, PLAY opens Setup directly. On Windows the first Same Wi-Fi tap opens a pause-style dialog: "Windows will ask to let Orbitrix on your network. Allow it." in Heading, then Continue (ink) above Back.
- **Join pages (Same Wi-Fi, Online):** Setup's frame (Back, headline, 20/16 px margins), one centred column (at most 640 px) on every target: a Host block (Set up a room, a quiet pill on the block) above a Join block of two groups, each under a Label Bold heading, split by the 1.5 px Settings hairline. "Rooms nearby" (Same Wi-Fi) or "Public rooms" (Online) lists public rooms as 40 px quiet pills: name in Label Bold on the left (host device's name on Same Wi-Fi, board preset and size online), status chip on the right (Open, Full or Playing; Joining while joining it), concentric with the row's end. A tap joins; a room with no seat for this player is disabled. Empty: "Looking for rooms" or "No public rooms", dim, in the first row's place. Then "Join by code": six code slots (printed tiles; the next one outlined while typing), an error line when the last try failed, and Join inside the Join block below the code slots: a 52 px pill spanning the block's inner width. Join uses the primary ink style when enabled; Cancel uses the quiet mat fill on the block. The button scrolls with its group. Progress belongs on the action itself; when Join reads Cancel, "Connecting to the host" or "Waiting for the host" appears in the shared snackbar overlay, whose screen position is independent of the group and scrolling.
- **Room:** the host's room is This Device's Setup (Players, Board, the same seat dropdown and presets), titled "Set up the room", then "Room" once open. Start narrows to leave 52 px plus the gutter for an invite button at its right: a 52 px round quiet button on the mat (tile fill, pressed tile on hover) with a drawn person and plus, opening the Invite players dialog. Held seats show their state where an AI shows pips; the host's own seat has no dropdown or chevron. The action reads Open room (Opening, Board too small, Needs a human seat, or why it failed), then Start, Start with bots or Waiting for players. A joiner sees only a Players block (40 px pills: dot, label with " (you)" on theirs, state or pips) and Ready or Not ready (quiet).
- **Invite players dialog:** one dialog behind the invite button, the same on every platform and room type, and shown by itself on the first Open room. The pause dialog style (tile panel, 28 px radius and padding, 80% mat veil, Title "Invite players"), then Public and Private side by side as 52 px pills, the draft outlined ("Invite only", dim, on a browser's server-free room). Then the QR, which is always the join link and never written out, the code in the Join field's read-only tiles, and one row: the ink action (Open room or Opening before the room opens, Confirm while access is a draft, else Done) with two 52 px round quiet buttons after it, Copy code (three tiles) and Copy link (two chain rings). Each copies only its own value and shows a check only after copying succeeds. Access applies only on Confirm; Esc or a tap outside drops it. A saved private LAN code is reused across games and access changes. Where the QR goes, dim words say why there is none yet (Room not open, Opening room, Confirm to update invite, why the room didn't open). Long browser-only codes scroll in the same tiles, and Copy code keeps the entire value. Short landscape screens place the QR beside the same controls. Reply uses the same view, without access.
- **Seat dropdown, Swap position and Remove player:** in Setup, local and in a room. Swap position sits above Remove player after a hairline; a tap moves this player (the seat carrying their name, the host's in a room) straight into the dropdown's seat, which takes theirs. It is greyed out on their own seat and on another human's: two humans never trade. Remove player is the last option, in the danger ink (light C21D3A, dark FF6B7A) with a 14% danger hover. It takes the seat out, bot or human; later seats move up with their choices (and players in an open room). Greyed out at two players. The danger ink remains reserved for this action and the Danger status chip; orb colors also identify functional map groups.
- **Snackbars:** one shared component (`client/snackbar.go`) owns screen positioning, typography, entrance animation, optional actions and pointer input. Both connecting/waiting messages and an online host's join requests use it. The 56 px pill is centred in the safe area, at most 560 px wide with 16 px minimum side margins, anchored 12 px above its bottom edge by default. The room screen supplies its fixed Open room/Start row as an exclusion: the same component places the snackbar 12 px above that row, keeping both the action and Invite usable. Content, scrolling and dialogs never reposition it. The page's inverse (ink fill, mat-coloured words, flipping with the theme), Small (13 px Archivo) text, left-aligned 24 px from the edge. Join requests show "Human wants to join", a dim "+N" when more wait, a quiet No (16% mat-coloured fill on hover) and Let in as a 96 ? 40 px mat-coloured pill. Every snackbar fades in over 12 ticks (ease-out), rising up to 16 px when space allows; its entrance stays inside the safe area and clear of the fixed action row. Host scroll content reserves space at its foot while a request is shown. The root game draws the overlay last, above every screen, dialog and dropdown; it takes pointer input before the page.
- **Host password:** Setup's frame; one block (at most 640 px, centred both ways while it fits) with the masked field (a mat pill, bullets, the end showing on overflow; chosen outline and a 2 px ink caret with focus), a Show/Hide pill, and "Wrong password" after a refusal. Focused on open. Open room is the bottom action.
- **Room ended:** the reason centred in Headline ("The host closed the room", "The host removed you", "Connection lost", …), then Rejoin (primary, only after a dropped connection) and Main menu (quiet), 240 × 52 px.
- **Setup:** Back pill and headline on top, the Players block above the full-width Board block, and Start across the content width (52 px), anchored to the bottom margin outside the scrolling content. Margins and block padding 20 px, 16 px below a 1200 px window width; gutters 16 px, 12 px below the same breakpoint. Roster and board options scroll (wheel or drag) above Start whenever full-size controls exceed the viewport; nothing squeezes.
- **Setup roster:** active seats only, one to four columns. Minimum column width starts at 220 px and grows to fit the longest strategy, its pips, padding and chevron. Columns split the body width equally after the 12/16 px gutters; pills fill their column, no width cap; a short final row keeps the grid widths; one column fills the body. With twelve players, 1040, 760, 640 and 390 px windows fit four, three, two and one columns (four-column pills at 1040 px are about 235 px). The heading groups −, count, + (2 to 12); no Add player chip. Row pitch 52 px (44 px target plus 8 px gap). Each seat is one 40 px pill (ink dot, Human/AI choice, an AI's pips, chevron), inset 2 px in its 44 px hit row. Setup uses the compact Back/Menu role and the shared fill hover; Home and pause-menu quiet buttons keep 52 px faces. Count changes keep existing choices (`fitSeats`); new seats take AI defaults.
- **Setup board:** the mini-mat preview stays in the Board block at every size, with the chosen shape's presets as nested outlines; its frame spans that shape's largest preset (Floor) or the chosen board, whichever is bigger. Portrait, Balanced and Landscape are stacked sections with plain headings, each six preset pills in two columns and three rows (Coaster, Placemat, Doormat, Rug, Carpet, Floor). No shape tabs: all eighteen presets scroll, and choosing one sets shape and size. Each section is 180 px (24 px heading, 8 px gap, 44 px targets with 8 px row gaps), 24 px apart; faces stay 40 px. The Board heading reads the preset's name, or "Custom", before the dimensions. Presets sit above the Columns/Rows controls in one aligned group, full body width when stacked. Beside the preview the group aims for 42% of the body up to 420 px, its measured minimum taking precedence. When the preview's 200 px minimum, the group's minimum and the gutter fit, the preview's background fills only the visible part of the left Board column, its top and bottom following that column's intersection with the scroll viewport, and the miniature scales inside 12 px padding keeping its aspect and outlines. Otherwise the preview sits above the group. Each dimension group is one 44 px row: label, −, value, + vertically centred; its minimum width is the wider of the Columns/Rows labels, an 8 px gap, two 44 px targets and the widest cached number with 12 px padding. The pair shares a line when it fits; below 600 px viewport width, or when it can't fit, full-width Rows sits under full-width Columns with the 12/16 px gutter. The same layout is the phone layout.
- **Settings:** Back pill and headline, tab pills (124 × 44 px), one panel down to the bottom of the page. Rows 76 px with hairlines; compact screens use 16 px margins and padding and 56 px rows. Phones have no Fullscreen row.
- **Map Editor:** 16 px safe-area insets, a 44 px header with Back, the editable map name, Test, Save, and More. A 196 px right dock on the wide desktop layout contains all three labeled palette groups in 36 px rows; wide means at least 880 px of inset width and 460 px of inset height, measured at UI scale. Short landscape uses a 176 px dock, a category picker, and 44 px palette rows. Portrait puts the 44 px utility controls above the canvas and a bottom dock with a 152 px base height, plus 48 px while Gate or Door is selected, bounded to 50% of the body; category tabs sit above 52 px icon-and-label shelf items. The dock inspector contains a short description and a functional pair/circuit color swatch when relevant, without repeating the selected name or component icon. Gate and Door reserve a 44 px starting-state control in an additional 48 px row above the inspector; their nonportrait help area grows by 26 px. Utility controls use 44 px targets. Palette scrolling and clipping preserve the canvas area. These dimensions describe this editor, not a new global density scale.
- **Game (desktop, browser, phone on its side):** the board fills the left column; the rail stacks Menu, turn wheel, chain chip, player list. The wheel fills 100% of the rail width, sharing width and centre with Menu, the chip and the player viewport. Rail width is bounded by vertical capacity so the wheel, controls, pointer-safe gaps and one player row fit; extra width stays with the board. The wheel never shrinks for the roster. Players scroll inside a clipped viewport below the chip. One or two columns are chosen from actual text widths (two for more than six seats only when they fit), comparing layouts for the largest wheel: equal wheels prefer type names, then fewer columns. Text and pills never shrink to grow the wheel. Desktop: 32 px margins, 24 px below Menu, 20 px below the wheel, 24 px before the list, 40 px faces, 40 px pitch, 36 px pills. Landscape under 640 px tall: 16 px margins, 12 px gaps, 28 px Menu and chip faces, 13 px chain text, 32 px pitch; Menu keeps its 44 px target.
- **Seat rows:** inset, dot, space, type, right-aligned orb count or the Out chip (24 px; 20 px on 32 px pitch). Desktop: 14 px insets, 12 px dots, 12 px space, 18 px body, at least 12 px before the count. Compact one-column: 10 px insets, 10 px dots, 8 px space, label type. Compact two-column: 6 px insets, 8 px dots, 4 px space, at least 6 px before the count, small type. Columns 8 px apart; faces 4 px shorter than the pitch. Overflow clips and scrolls with a proportional indicator; wheel and touch drag share one scroller, and a drag stays with the viewport it started in. A turn change reveals the current player; manual scrolling stays put until the turn or geometry changes. Long counts fit the number column without crowding the type or changing the layout.
- **Game, portrait:** the board keeps at least 16 px margins and fills the width with square cells while its full height fits above the players and controls; otherwise it scales down uniformly, centred, every row visible, no board scrolling. Board and player list anchor together above the fixed bottom controls with 16 px gaps; spare height goes above the board. The player viewport is no taller than its content: one column at 36 px pitch, or above six seats when text fits, two at 32 px. Only the list scrolls; board cells take no input through the list or footer. The bottom row holds the chain chip, a current-player dot showing its exact orb count, and Menu, 8 px apart; the dot sits at the row's exact centre with equal-width chip and Menu areas. No wheel and no repeated type label in portrait. In Colorblind mode the dot's shape sits in a small badge at its top edge so the number stays readable. Menu and chip keep 40 px faces; Menu keeps its 44 px target.
- **Orientation:** on Android a board with more columns than rows turns the phone to landscape (either way up), its pause Settings included; leaving the game hands rotation back. Where the app can't turn the screen (browsers, desktop windows), such a board in an area taller than wide is drawn a quarter turn (see Board).
- **Pause and Results on small screens:** pause buttons narrow to fit and go two by two when one column is too tall. The winner's title shrinks so the longest seat label plus " wins!" fits; on a screen too short the winner's dot shrinks (24 px radius at least) and the result buttons stand side by side; the results line drops to label size when body won't fit.
- **Board:** a 7% gutter on each side of every cell; the board scales to fill its area with the outer tiles on the area's edges. A board whose shape differs from its area is centred, the slack split evenly, on the game screen and Home alike. Where the app can't turn the screen, a board wider than tall in an area taller than wide is drawn a quarter turn clockwise, long side down the screen, tiles big enough to tap; only the drawing turns.
- **Web loader:** before the game starts, the twelve seat dots (16 px, 12 px apart) on the mat in the saved theme, each filling with its ink as another twelfth of the game arrives. No text unless loading fails.
- **Landing page (`/`):** Portrait keeps Home's column (wordmark, seat dots, PLAY dot), then the downloads as store-style tiles in a 2×2 grid, the wordmark's width (372 px at least, the full width on a phone; one column under 352 px), 12 px apart. The portrait PLAY dot is as wide as the grid, PLAY at 17% of its diameter; short portrait screens scroll. Landscape keeps the wordmark and seat dots above a row: PLAY on the left, the 2×2 downloads on the right. The circle fits the available width and height; short landscape screens use tighter outer spacing. An odd last tile sits centred at one column's width. Each tile is 64 px tall, tile fill with pressed-tile hover, 16 px corners: the platform's official mark (Simple Icons, filled in ink, 26 px), then "Download for" in Small over the platform name in Title. All platforms use the same tile style. The page's exceptions to the system: filled brand marks instead of stroked icons, 16 px tiles instead of pills, and the "Download for" line.

## Elevation & Depth

Flat. No shadows, bevels or gradients. Depth comes from layered fills: mat, then tile or block, then a ground-coloured control on the block, then ink. Orbs get a rim in the mat colour so overlapping orbs read as separate flat pieces: a cut-out edge, not a shadow. The pause dialog sits on a flat 80% mat veil. Motion uses opacity and size, never blur: burst rings expand and fade, the chain callout fades over 20 ticks, the winner's ink floods out from the board centre as an eased circle (40 ticks).

### Named Rules
**The Flat Vinyl Rule.** A layer that must stand out gets a different flat fill or an ink outline. Never a shadow, glow or gradient.

## Shapes

Pills, dots and rounded squares. General buttons, tags, toggles, slider tracks, callouts and badges are full pills. Home's primary actions are big circles (the PLAY dot), and every seat is a dot. Board tiles are rounded squares with a 20% corner radius. Containers: 24 px blocks and the Settings panel, 28 px pause dialog, 18 px dropdowns, 16 px preview panel, 14 px hovered settings rows. Outlines: 2.5 px means chosen, 1.5 px a dim edge. Quiet buttons change fill on hover with no border; chosen borders persist. The map editor uses compact 8 px rounded controls with 2 px selected outlines, 10 px canvas/dock corners, and 16 px options panels; these are local treatments. Dashed outlines mean a potential slot (presets in the preview). Icons are stroked paths with round caps (back arrow, chevrons, hamburger, plus, minus, key, copy); no glyph icons.

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
- **Maps and Map Editor:** New map uses the primary ink pill. The editor's Save is solid ink and its other controls use quiet neutral fills, Archivo labels, and the editor's local rounded shape. The palette is persistent, with the active tool outlined. More opens dimensions, Show capacities, and Preview pressed switches; active options keep their outline.
- **On the winner's ink (Results):** Play again filled in the contrasting ink; Main menu and Exit as 2.5 px outlines in it. All 220 × 52 px, 12 px apart. Exit hidden on web; online only the host gets Play again (back to the lobby).
- **Second tap:** Restart turns solid ink reading "Restart?" for 3 s before it acts.
- **PLAY dot:** solid ink circle, PLAY in the mat colour.

### Cards / Containers
- **Corner Style:** 24 px blocks and panel, 28 px dialog.
- **Background:** tile fill on the mat; no border, no shadow.
- **Internal Padding:** 24–28 px; Setup blocks `setup-block` (20 px) or `setup-block-compact` (16 px) below the compact breakpoint.
- **Maps library:** tile-filled rounded rows contain mini-mat thumbnails using the shared terrain/device art, a name, dimensions and Ready or Draft status. The selected row keeps the chosen ink outline. The editor's canvas is a rounded mat surface with tile-filled Floor cells.

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
- **Map tools:** Terrain contains Floor, Void, Hole, Wall, Barrier, Breakable Wall and Breakable Barrier; Devices contains Link, Portal, Reservoir and Spillway; Circuit contains Latch, Toggle, Hold, Gate and Door. Keep the full Breakable Wall and Breakable Barrier labels (two lines in portrait). The inspector describes function once, without repeating the selected name or icon. Its pair/circuit swatch opens one page with twelve orb colors and Done. A circuit accepts any mixture and number of switches plus Gates and Doors. Gate and Door expose a 44 px **Starts closed / Starts open** button above the inspector; repainting applies the setting, with Undo/Redo. Portrait reserves one additional 48 px row for that control while an output tool is selected. Capacities uses corner badges on passive-feature and circuit cells.

### Navigation
- **Back and Menu:** compact tile pills, 40 px face in a 44 px target, glyph centred 22 px from the left edge, label at 42 px. On short landscape rails Menu and the chain chip use the tight 28 px face; Menu keeps its 44 px target. Back is 112 px on Setup and Settings (arrow only, 44 px, under 540 px wide); Menu spans the landscape rail or the footer width beside the portrait turn dot.
- **Touch:** a finger acts on release, only if it dragged under 10 px, so a drag scrolls without pressing; the volume knob follows a finger from the moment it lands. A lifted finger leaves no hover. Android back acts as Esc; on Home it sends the app to the background.
- **Menu pill** (hamburger + "Menu") opens the pause dialog: Resume (primary), Restart (second tap), Settings, Main menu (leaves at once). Online the dialog is titled Menu (the game goes on) with Resume, Settings, Main menu; the host's Main menu closes the room.
- **Settings tabs:** Video, Audio and Network (Same Wi-Fi rooms and Online rooms as Public/Private pairs in the Light/Dark style, chosen one outlined; Force relay in builds that play online; the tab is dropped where neither kind of room exists), as pills (chosen style for the active tab, tile fill otherwise).
- **Keys:** only Esc, M, Space and F3. No key is ever printed on a button.
- **Editor workflow:** Undo, Redo, Erase, Pan, zoom out, Fit and zoom in stay beside the canvas; Erase or Pan is outlined while active. The editable map name, Test, Save and three-dot More control stay in the header. Test opens the existing Setup flow for a valid draft and returns to the editor. Recover draft and Discard draft are quiet library actions; leaving unsaved edits uses the existing confirmation dialog. Show capacities and Preview pressed switches stay in More. The pressed preview reverses the relevant Gates and Doors from their saved starting states and updates shown capacities without changing the saved map.

### Spinner (signature)
Landscape only: a pie with one wedge per seat in its ink, split by 3 px mat-coloured lines; out seats are blank tile. Its diameter equals the rail width. A mat-coloured hub and an ink pointer at the rim aimed at the mover, with a 2 px mat-coloured outline so it shows over every wedge in both modes; the tip stays within its 12 px clearance beyond the wheel. Each living wedge shows its exact orb count, upright Archivo bold, in the outer band; Colorblind marks take a separate inner band; the pointer stays clear of both. Out wedges show neither. Portrait uses the centred current-player dot beside Menu with the same exact count. Both fit values through five digits with the same cached glyphs and uniform scaling, no abbreviation or truncation; shorter values grow back to the regular size. Counts follow the displayed board during chain playback without rebuilding the cached wheel. The player list keeps the larger count reference.

### Chain Callout (signature)
A pill in the mover's ink reading "Chain ×N" in Unbounded 800, `inkOn` text. In landscape it matches the wheel's width and centre and sits below it with pointer clearance, never over it. In portrait it fills the footer left of the turn dot. Its height matches Menu: 40 px on desktop and in portrait, 28 px in short landscape. Shown for chains of 2 or more, then fades out.

### Board, Orbs and Bursts (signature)
Tiles are rounded squares, printed blank or lit 30% toward the mover's ink. Orbs are flat discs in their owner's ink with a mat-coloured rim (plus their mark in Colorblind mode), all in one batch. A burst is a flat ring in the exploding player's ink, expanding over 18 ticks and fading. The hovered tile gets an outline in the mover's ink. A tap the mover can't make (someone else's tile, or not their turn) flashes that tile's outline in the mover's ink, fading over half a second. A cell with more than four orbs gets an ink count badge.

Map art is shared by palette, editor canvas, library thumbnail and match board. Void leaves a gap. Hole fills the Floor footprint with Pit ink and joins neighboring Holes like walls; orbs reach its centre, then shrink, darken and fade as they fall. Wall is a plain solid block. Breakable walls show a connected, tapered negative-space fracture rooted at the center: three main splits and two forks, visibly identifying breakability at initial durability 2. At durability 1 the same whole connected network grows 1.24x, longer and wider toward the edges. Use branching fractures rather than repetitive zigzags; no random texture, detached flecks or circles. Fracture cuts are structural damage, not additional device-icon stroke weights. Wall fill and joins stay unchanged; destruction creates empty Floor. Adjacent walls join across gutters, and edge obstructions connect to nearby wall corners.

#### Map device system (authoritative)

Preserve this established map design; make only explicitly requested refinements. These rules apply to the shared art in the palette, editor canvas, library thumbnails and match board, in both light and dark themes. They govern map art without changing player/orb identity or general chrome.

- Device outlines and functional strokes use `max(0.8 px, cell size * 0.04)`. Neutral strokes use the exact Wall ink (`sc.dim`): #65615B in light mode, #A39C92 in dark mode.
- Device outer frames share the exact Floor tile geometry: bounds `r.inset(cell * tileGutter)` and corner radius `cell * tileRadius`, with no surrounding Floor gap. Hold corner brackets are clipped sections of that same rounded frame. All three switches share inner inset `body.w * 0.1875` and inner radius `max(0, outerRadius - inset)`; Latch and Hold use the identical inner rectangle. Toggle now uses that full shared inner rectangle, including its horizontal extent and gaps.
- Device fills must differ from both Floor and Void. Prefer open outlines; the dark Link opening uses `mix(ink, dim, 0.18)` in light mode and `mix(ground, dim, 0.18)` in dark mode. Relationship strokes retain the twelve exact orb colors.
- No circular shapes in device art. Map art has no Colorblind variants or group marks; player and orb marks elsewhere stay unchanged.
- Every line communicates a boundary, motion, storage or state. No identity ticks, decorative dots, slats, or incidental detail.
- Link: rounded-square colored rim and dark opening with the exact Floor corner radius, with two separated U paths reproducing the large lower arc and small upper arc, with rounded internal bends matching the outer frame. Preserve their arrangement, tips, stroke width and pair ink. Each bent path inherits the outer rounded geometry through an inset-derived radius: `min(halfSpan, max(strokeWidth, frameRadius - (body.w / 2 - halfSpan)))`. The radius is at least the stroke width within the path span; no right-angle bends inside the rounded Link frame. The two paths remain visible in both Colorblind states.
- Latch: retain the rounded-square housing and shared inset button, outlined when OFF/released and solid circuit-color fill when ON/pressed, with unchanged geometry.
- Toggle: rounded-square outer frame using the exact Floor radius. The button fills the full shared inner rectangle and comprises two equal adjacent halves with a center seam. OFF fills the left half solid circuit ink; ON fills the right half solid circuit ink. The opposite half stays outlined. Clip each selected-half fill to the shared rounded inner geometry, outline the full inner rectangle, and draw the vertical seam with the shared stroke width. Preserve all shared gaps.
- Hold: top-down view with four corner brackets clipped from the shared rounded frame surrounding a central plate matching Latch's inner rectangle and radius. The plate is outlined when released and filled in circuit color when pressed.
- Reservoir: top-down square well with the exact Floor radius on its outer outline and one unchanged square inset reserve pocket, representing the one orb retained after an explosion. Both outlines are neutral Wall ink when empty and use the occupying player's exact ink while occupied, returning to neutral when empty. Ownership follows the displayed animation wave. Four symmetrical cardinal channel strokes extend from the central square pocket toward the outer basin, stopping two stroke widths before the frame; they use the same shared stroke and neutral/owner ink. Frame and radius stay unchanged. No machine housing or side view. Links and switches retain their pair/circuit inks.
- Spillway: Reservoir's exact frame, radius, stroke and neutral/owner ink logic, with its own silhouette: four diagonal channels run from the frame's corners, stopping two stroke widths short of the rounded corner, into a square pocket (inset 30% of the body) filled with Pit ink, where its extra orb is lost. The X reads apart from Reservoir's +. That orb reaches the pocket and falls like one sent into a Hole, with the same cue.
- Gate and both Barrier kinds retain their structural edge thickness; these are physical obstructions, rather than device-icon outlines. Gates retract toward the endpoints when open. Breakable Barriers use the same ink as Breakable Wall, `mix(dim, tile, 0.23)`, with a transverse negative-space fracture, widening after one hit; destruction removes the edge and its wall joins.
- Door: closed is a solid circuit-colored square using the exact Floor tile bounds and radius, with a contrasting locked icon chosen by `inkOn`. Open removes that square and shows only a neutral Wall-ink unlocked icon on normal Floor; no outer frame, panels or leaves remain. Both icons use the same centered square canvas as the shared device inner rectangle (`body.w * 0.625`). The slim lock body is inset horizontally by 8% of that canvas on each side and occupies its lower 54%; the shackle is inset by 20% of the body's path width on each side. Body and shackle use one directly drawn vector outline with the shared `max(0.8 px, cell * 0.04)` stroke and matching true quarter-round bends of one stroke width, with no blur or sprite resampling. Opening raises only the shackle's free right end by 2.5 stroke widths; the body, shackle alignment and overall bounds stay fixed. Palette, editor, thumbnails and match use the same art; both themes and Colorblind states retain it.

Orbs trapped inside a closed Door stay at their exact last displayed positions, without orbiting, jitter, breathing or smoothing. Their fill uses the normal Floor ink (`sc.tile`) and they have no Colorblind marks. The same locked icon draws above them in the opposite theme's exact Floor color: #E0DED9 in dark mode, #2D2A27 in light mode. It retains a solid circuit-color keyline a quarter stroke wide on either side for separation; no blur. Empty locked Doors retain `inkOn(circuitColor)`. This presentation follows the displayed circuit wave; saved counts and ownership remain intact. Reopening restores the original owner's orb ink, optional marks, normal motion and access.

Both Colorblind states use identical normal map art: devices, terrain, Gates and relationship swatches have no identity stamps or group numbers. Link guidance remains contextual during placement or hover.

### Winner Flood
The winner's ink expands from the board centre to cover the screen. On it: the winner's dot, solid in the contrasting ink (its mark in the flood ink in Colorblind mode, so it never reads as an out seat's ring), the title in Display, a body line (move count, then the biggest chain from ×2), and the result buttons.

## Do's and Don'ts

### Do:
- **Do** keep every fill flat and solid; separate layers with a different fill or an ink outline. Quiet hover is pressed tile; chosen keeps a 2.5 px ink outline.
- **Do** reuse the twelve orb colors for map relationships; keep map art identical with Colorblind enabled or disabled; retain exact player and orb marks elsewhere.
- **Do** use solid ink for exactly one primary action per screen, quiet tile or mat pills for the rest.
- **Do** pick text and mark colour on a player ink with `inkOn`.
- **Do** add the seat's shape mark to player and orb displays in Colorblind mode; map art stays identical in both states.
- **Do** keep normal map art free of identity stamps and use the shared component art on every map surface.
- **Do** define every value for both modes.
- **Do** ask the user before making any new visual decision.

### Don't:
- **Don't** use 3D, shadow depth, gradients, glows or neon.
- **Don't** colour general chrome with orb inks. The danger ink remains reserved for Remove player and the Danger chip.
- **Don't** print key hints on buttons or add explanatory subtext outside the browser's Join group and the editor's functional tool description.
- **Don't** use printed partner/group numbers, duplicate the selected component in the inspector, or replace the shared map art with unrelated palette symbols.
- **Don't** bind keys beyond Esc, M, Space and F3.
- **Don't** build menus that differ from game convention.
- **Don't** use glyph or emoji icons; draw icons as stroked paths.
