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
  player-violet: "#7B4FD6"
  player-brown: "#8B5A2B"
  player-lime: "#9BCB00"
  player-magenta: "#E8418C"
  player-vermilion-night: "#E86A1A"
  player-blue-night: "#3A9BE0"
  player-green-night: "#1FB88C"
  player-orange-night: "#F0AA1A"
  player-mauve-night: "#DD8DB8"
  player-sky-night: "#7CC6F0"
  player-yellow-night: "#F0E442"
  player-chalk-night: "#F4F4F4"
  player-violet-night: "#9C7BF0"
  player-brown-night: "#C98B4E"
  player-lime-night: "#B4E61E"
  player-magenta-night: "#FF5FA8"
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
    height: "32px"
---

# Design System: Orbitrix

## Overview

**Creative North Star: "The Party Mat"**

Orbitrix is a printed vinyl floor mat for a loud room of friends. The board is a sheet of printed rounded-square tiles with gutters; the players' twelve inks are the only colour on it; every control is plain ink on the vinyl. Everything is drawn in code as flat, solid fills. Nothing has a bevel, a glow, a gradient or a cast shadow. The category's neon-orbs-on-black look and the purple party-app shell are both rejected.

The system is built to be read from the couch. Type is large: seat rows on the desktop game rail use 18 px body; a phone held in the hand steps down to the 15 px label size. The display face is a wide, heavy geometric (Unbounded) and is used only for things people shout: the wordmark, PLAY, Start, Chain ×N and the winner's title. Archivo handles everything else. There are two modes, a light vinyl mat (the default) and a warm charcoal mat at night. They share every rule, and only the values change.

Colour signals ownership and turn, never decoration. A player's tiles light up only when it is their turn. A chain callout appears in the mover's ink, and the winner's ink floods the screen at the end. Colorblind mode adds each player's shape mark everywhere that player's colour appears.

**Key Characteristics:**
- Flat solid fills only: no 3D, no shadow depth, no gradients, no neon.
- Colour belongs to the players. Chrome is ink (light mode) or chalk (dark mode) on the mat.
- Solid ink is reserved for the one primary action on a screen.
- Pills, dots and rounded-square tiles. There are no sharp rectangles in the chrome.
- Labels are plain and short. There are no key hints on buttons and no explanatory subtext.
- Menus work like most games: one Menu button opens a pause dialog, and Settings is tabbed (Video, Audio).

## Colors

The palette is two quiet neutral sets (a light vinyl mat and a night mat) that carry twelve saturated, colour-blind-tuned player inks. The neutrals lean faintly warm and never blue: no blue tint in the ground, tiles, ink or dim of either mode. Blue appears only as the Blue and Sky player inks.

### Primary
- **Press Ink** (`ink`, light mode) / **Chalk Ink** (`chalk`, dark mode): all text, outlines, the PLAY dot, the Start and Resume buttons, toggles that are on, slider fills, the spinner pointer and orb-count badges. Hover lifts them to **Worn Press Ink** / **Worn Chalk Ink**.

### Secondary
- **The twelve player inks**, in fixed seat order: Vermilion, Blue, Green, Orange, Mauve, Sky, Yellow, Charcoal (Chalk in dark mode), Violet, Brown, Lime, Magenta. The first eight follow Okabe–Ito. Each mode has its own tuned set (the `-night` tokens). They are used only for orbs, seat dots, spinner wedges, the 30% turn tint on tiles, burst rings, the chain callout and the winner flood.

### Neutral
- **Vinyl White** (`mat-vinyl`) / **Charcoal Mat** (`mat-night`): the ground of every screen. It is also the fill of quiet controls that sit on a tile block, and the rim colour around every orb.
- **Printed Tile Grey** (`tile-print`) / **Night Tile** (`tile-night`): empty board tiles, Setup blocks, the Settings panel, the pause dialog, quiet buttons on the mat, and the current-player row highlight. Hover uses the **Pressed** variant.
- **Faded Ink** (`dim`) / **Faded Chalk** (`dim-night`): secondary text, players who are out (an empty ring), dashed outlines for the board presets, toggles that are off, and disabled labels.
- **Mark White** (`mark-white`): shape marks and the chain-callout text on dark player inks. `inkOn` picks Press Ink or Mark White, whichever has more contrast with the fill behind it.

### Named Rules
**The Players Own Colour Rule.** Hue appears only where it identifies a player. Chrome never borrows a player ink for emphasis, hover or decoration.

**The Turn Light Rule.** Only the player to move gets lit tiles (tile mixed 30% toward their ink). Every other owned tile stays printed blank, so ownership is shown by the orbs on it, not by the tile.
## Typography

**Display Font:** Unbounded (variable, embedded, OFL)
**Body Font:** Archivo (variable, embedded, OFL)

**Character:** Unbounded is wide and heavy, like poster lettering on a party mat. Archivo is a sturdy grotesque that stays readable at couch distance. All sizes are logical px and scale together with the UI scale.

### Hierarchy
- **Display** (Unbounded 800, wordmark capped at 60 px): the ORBITRIX wordmark on Home. The same face also sets PLAY (34% of the dot radius) and the winner's title (56 to 120 px, scaling with the window).
- **Headline** (Unbounded 700, 26 px): screen titles ("Set up the game", "Settings", "Paused").
- **Title** (Unbounded 700, 17 px): block headings (Players, Board) and the labels of primary buttons.
- **Callout** (Unbounded 800, 20 px): "Chain ×N" in the mover's ink. **Callout compact** (Unbounded 800, 15 px) sets it on phones (portrait, or under 640 px tall).
- **Number** (Unbounded 700, 20 px): stepper values and the board dimensions.
- **Body** (Archivo 500 or 700, 18 px): seat rows on the desktop game rail, Settings row labels and the volume value, and the results line.
- **Label** (Archivo 700 or 500, 15 px): pill labels, seat tags, dropdown options, and the game's seat rows on phones (or in two columns on the desktop rail).
- **Small** (Archivo 500 or 700, 13 px): preset dimensions after a preset name, the game's two-column seat rows on phones, and the F3 frame-rate overlay.

### Named Rules
**The Shout Rule.** Unbounded is used only for words the room says out loud and for numbers. Everything you read quietly is set in Archivo.

## Layout

Layout is computed in code from the window size and the UI scale `u`. There is no grid framework. Outside Setup, pages use a 40 px margin (Settings), Home a 64 px one, and the game screen 32 px on every side. Spacing repeats in steps of 8, 12, 24, 28, 40 and 64 px. Rows are 8–12 px apart, groups 24–28 px apart, and the outer margins are 40–64 px.

One layout serves every target (desktop, browser, Android): there is no separate mobile UI, only the same screens responding to the size they get. Everything is laid out inside the **safe area**, the screen minus the display cutout on Android; only full-screen fills (the mat, the pause veil, the winner's flood) reach past it. **Portrait** means the safe area is taller than wide, whatever the device. Screens shorter than 640 px or narrower than 600 px use the compact margins (16 px) and tighter gaps; touch targets never shrink below 44 px.

- **Home:** a wordmark column on the left (at least 420 px, or 36% of the width). The column holds the wordmark, the twelve seat dots spread exactly across its width, the PLAY dot (radius 64 to 150 px) and a stack of quiet pills (Settings, then Exit where the platform allows it), each as wide as the PLAY dot up to the shared 240 px quiet-button maximum. The rest of the screen is a live AI-vs-AI demo mat. There is no tagline and no facts line. On a screen too short for the column (a phone on its side) the gaps tighten and the PLAY dot shrinks (to 40 px radius at least). **Portrait:** the column opens around the mat: the wordmark (24 px side margins) and seat dots at the top, the live mat filling the middle, and the PLAY dot (radius up to 11% of the height) with the button stack at the bottom, within thumb reach.
- **Setup:** a Back pill and a headline across the top, then the active Players block above the full-width Board block. Start spans the content width (52 px tall), anchored to the bottom margin outside the scrolling content. Outer margins and block padding are 20 logical px, reducing to 16 px below a 1200 px window width; gutters reduce from 16 to 12 px at the same breakpoint. The roster and board options scroll above Start whenever their full-size controls exceed the viewport.
- **Setup roster:** show active seats only, in one to four columns. The minimum column width starts at 220 px and grows to fit the measured longest strategy, P12 tag, padding and chevron. Player columns divide the full body width equally after subtracting the existing 12 px or 16 px gutters; pills fill their column without a width cap. A short final row keeps the same grid-column widths, and a single column fills the body width. With twelve players, window widths of 1040, 760, 640 and 390 px fit four, three, two and one columns respectively; the four-column pills at 1040 px are about 235 px wide. The heading groups minus, the player count, and plus; the controls stop at two and twelve players. There is no Add player chip. Row pitch is 52 px (44 px target plus 8 px gap). Each seat combines its ink dot, player number, Human/AI choice and chevron in one 40 px pill, inset 2 px vertically in its 44 px hit row. Setup groups use the compact Back/Menu sizing role and shared fill hover; Home and pause-menu quiet buttons retain their larger 52 px faces. Changing the count preserves existing Human/AI choices via `fitSeats`; new seats take AI defaults.
- **Setup reflow:** the mini-mat preview remains part of the Board block at every size, with the chosen shape's presets as nested outlines; the frame spans that shape's largest preset (Floor) or the chosen board, whichever is bigger. Portrait, Balanced and Landscape are three vertically stacked sections with plain headings, each followed by six preset pills in two columns and three rows. There are no shape tabs: all eighteen presets are in the scrolling list, and choosing one sets both its shape and dimensions. Each section is 180 px tall (24 px heading, 8 px below it, then 44 px targets with 8 px row gaps), with 24 px between sections; visible button faces remain 40 px tall. The Board heading reads the preset's name, or "Custom", before the dimensions. Presets sit above the Columns/Rows controllers with a gap between them. Presets and dimensions share one aligned control group, using the full body width when stacked; beside the preview, its width aims for 42% of the body up to 420 px, with the measured inline-pair minimum taking precedence. When the preview's 200 px minimum width, the control group's minimum width and the gutter fit, its background fills only the visible part of the left Board column. Its top and bottom follow that column's intersection with the scroll viewport above Start as the page scrolls. The miniature scales with the visible background inside 12 px padding, preserving its aspect ratio and nested outlines. Otherwise the preview sits above the entire group. Each dimension group is one 44 px-high inline row with its label and −/value/+ controls vertically centred. Its minimum width includes the wider Columns/Rows label, an 8 px gap, two 44 px targets and the widest cached numeric label with 12 px padding. The groups share one line when the pair and gutter fit; below a 600 px viewport width or when the pair cannot fit its own control-group width, full-width Rows sits below full-width Columns with the existing 12 px or 16 px gutter. Overflow wheel-scrolls (or drags, on touch) within a clipped viewport above Start; the same layout is the phone layout.
- **Settings:** a Back pill and a headline, tab pills (124 × 44 px), and one panel that runs down to the bottom of the page. Rows are 76 px tall, separated by hairlines. Compact screens use 16 px margins and padding and 56 px rows. Phones have no Fullscreen row: they are always full screen.
- **Game (desktop, browser, phone on its side):** one computation for every platform. The board fills the column left of a rail, margin to margin; the rail stacks Menu, the spinner, the chain callout and the seat table. Its minimum width fits the existing one-column table or numbered table, Menu and the callout, then it takes every px of width the board cannot use. After fitting the board, the desktop table compares one and two columns, with and without type names, within the actual rail width and chooses the largest wheel while preserving text size, pill height, dots and insets. Ties preserve type names, then prefer fewer columns. Two columns of type names use spare rail width without shrinking the board. Every set-out shows the seat number (P1–P12), dot and orb count; widths are measured from real text and the board's maximum orb-count digits. Menu and the table span the rail. The spinner grows to the rail's width or the available height, whichever is smaller; the chain callout always matches the wheel's width and centre. Desktop: 32 px margins, 24 px under Menu, the callout in its own slot 20 px under the spinner, 24 px above the table, and a fixed 40 px row pitch with 36 px pill faces in either column layout. Rows never shrink to grow the wheel. Below 640 px of height, the existing phone layout keeps 16 px margins, 12 px gaps clear of the pointer, no type names, rows 20–32 px, and the callout across the spinner's middle.
- **Seat rows:** inset, dot, space, number, type, orbs right-aligned. Desktop in either column layout: 14 px insets, 12 px dots, 12 px space, body size (18 px), and at least 12 px before the orbs. Column changes preserve these sizes; two columns have an 8 px gap. Phone one column: 10 px insets, 10 px dots, 8 px space, label size. The existing phone two-column layout packs tighter: 6 px insets, 8 px dots, 4 px space, at least 6 px before the orbs, small size.
- **Game, portrait:** the board fills the top, 16 px margins and gaps. Under it a bar holds the spinner (88 to 140 px, at most 32% of the width, never narrower than the callout) with the chain callout, exactly the spinner's width, 12 px below it (clear of the pointer's tip) on the left, and the seat table beside them, centred against each other. Menu spans the full width at the bottom margin, in thumb reach. Up to six seats get one column of 36 px rows at the label size, with type names when they fit; more get two columns of 32 px rows at the small size (number and orbs). A crowded table on a narrow phone takes what it lacks from the gap before it, down to 8 px. The compact callout is 32 px tall and at least wide enough for "Chain ×88" (three digits still fit inside).
- **Orientation:** on Android a game whose board has more columns than rows turns the phone to landscape (either way up), Settings from its pause menu included, and leaving the game hands rotation back to the phone. Where the app can't turn the screen (browsers, desktop windows), such a board in an area taller than wide is drawn a quarter turn instead (see Board).
- **Pause and Results on small screens:** the pause buttons narrow to fit the width and go two by two when one column is too tall. The winner's title shrinks so "P12 wins!" fits the width, and on a screen too short for the column the winner's dot shrinks (24 px radius at least) and the result buttons stand side by side; the results line drops to the label size when the body size would not fit.
- **Board:** tiles have a 7% gutter on each side of every cell, and the board is scaled to fill its area with the outer tiles' edges on the area's edges (no extra inner margin). A board whose shape differs from its area is centred, so the leftover space splits evenly on both sides of the slack axis, on the game screen and on Home alike. Where the app can't turn the screen itself (browsers, desktop windows), a board wider than tall in an area taller than wide is drawn a quarter turn clockwise, so its long side runs down the screen and its tiles stay big enough to tap; only the drawing turns.
- **Web loader:** before the game starts, the page shows the twelve seat dots (16 px, 12 px apart) on the mat, in the saved theme; each fills with its ink as another twelfth of the game arrives. No text, unless loading fails.

## Elevation & Depth

The system is flat. There are no shadows, bevels or gradients anywhere. Depth comes from layering fills: mat, then tile or block, then a ground-coloured control on the block, then ink. Orbs get a rim in the mat colour so that overlapping orbs read as separate flat pieces. This is a cut-out edge, not a shadow. The pause dialog sits on a flat 80% veil of the mat colour. Motion uses opacity and size, never blur: burst rings expand and fade, the chain callout fades out over 20 ticks, and the winner's ink floods out from the board centre as an eased circle (40 ticks).

### Named Rules
**The Flat Vinyl Rule.** If a layer needs to stand out, give it a different flat fill or an ink outline. Never use a shadow, a glow or a gradient.

## Shapes

The shape language is pills, dots and rounded squares. Every button, tag, toggle, slider track, callout and badge is a full pill. Primary actions on Home are big circles (the PLAY dot), and every seat is a dot. Board tiles are rounded squares with a corner radius of 20% of the cell. Containers use a generous fixed radius: 24 px for blocks and the Settings panel, 28 px for the pause dialog, 18 px for dropdown menus, 16 px for the preview panel and 14 px for hovered settings rows. Outlines come in two weights: 2.5 px means chosen or selected, and 1.5 px marks a dim edge. Quiet buttons change fill on hover without adding a border; chosen borders remain persistent. Dashed outlines mean a potential slot (the board presets in the preview). Icons are drawn as stroked paths with round caps (a back arrow, chevrons, the hamburger, plus and minus); there are no glyph icons.

**The Twelve Marks.** Each seat has one shape mark, drawn in Press Ink or Mark White, whichever contrasts more with the seat colour: dot, bar, plus, cross, triangle, square, diamond, stem, pair, ring, chevron, frame.

## Components

### Buttons
Buttons are plain pills: solid where they act, quiet where they offer.
- **Shape:** full pill (`rounded.pill`), 40 or 52 px tall.
- **Primary (`button-primary`):** solid ink fill with the label in the mat colour, set in Unbounded 700. There is one per screen: Start on Setup, Resume in the pause dialog. On hover the fill lifts to `ink-hover`. When disabled, it becomes a tile fill with a dim label.
- **Setup Start (`setup-start`):** the primary style with its own 52 px height and full content width, pinned below the scrolling viewport. It reads "Board too small" and is disabled when players outnumber cells. Other primary buttons keep their existing dimensions.
- **Compact Setup controls:** Back and the visible button faces inside Players/Board use the compact role (40 px), with 44 px hit targets. Keep this distinct from the 52 px Home/pause-menu quiet-button role and the 52 px Start footer; do not restore the larger menu height inside Setup groups.
- **Quiet on the mat (`button-quiet`):** tile fill with an ink label in Archivo 700. Used for Home's Settings/Exit, Back, Menu and unselected tabs. Hover changes the fill to the pressed tile colour.
- **Quiet on a block (`button-on-block`):** mat fill on a tile block. Hover changes the fill to the pressed tile colour, exactly like Home Settings/Exit, Back and the game Menu; it adds no outline. Disabled controls ignore hover and use tile fill with dim labels.
- **Chosen (`button-chosen`):** mat fill with a persistent 2.5 px ink outline. Hover uses the same pressed tile fill as other quiet buttons and preserves the existing border. Used for the selected tab, the selected preset and the selected Light/Dark option.
- **On the winner's ink (Results):** Play again is filled with the contrasting ink. Main menu and Exit are 2.5 px outlines in that ink. All three are 220 × 52 px, 12 px apart, like the pause menu.
- **PLAY dot:** a solid ink circle with PLAY in the mat colour.

### Cards / Containers
- **Corner Style:** 24 px for blocks and the panel, 28 px for the dialog.
- **Background:** tile fill on the mat. There is no border and no shadow.
- **Internal Padding:** 24–28 px; Setup blocks use `setup-block` (20 px), or `setup-block-compact` (16 px) below the compact window breakpoint.

### Inputs / Fields
- **Setup seat picker (`setup-seat-picker`):** a combined mat-coloured pill on the Players block (40 px face inside a 44 px hit row), using the shared quiet-button surface and Archivo 700 at 15 px. Human seats and an open picker keep the common 2.5 px chosen ink outline; hover changes the fill to pressed tile and turns the player tag from dim to full ink. The 16 px colour dot sits 16 px from the left, the player tag at 44 px, the left-aligned choice after the measured P12 tag width (at least 30 px) plus 8 px, and the chevron 20 px from the right. The dropdown is a mat panel (18 px radius, 2 px ink outline) with 44 px option rows and 40 px visible faces using the same shared surfaces; the selected option is solid ink and other hover uses pressed tile fill without an outline. Options use two columns when the 408 px single-column list (nine target rows plus panel padding) would not fit the window height.
- **Setup player count (`setup-player-count`):** minus/count/plus stay together in the Players heading. The 40 px circular faces sit inside 44×44 px hit areas and use the shared Ground style: mat fill, pressed tile fill on hover, and tile fill with dim marks and no hover when disabled. The appropriate control is disabled at two or twelve players.
- **Setup presets (`setup-preset`):** shared mat-coloured surfaces with compact 40 px faces inside 44 px hit rows, centred Archivo 700 labels at 15 px and 8 px target-row gaps; hover uses pressed tile fill and the chosen preset keeps its 2.5 px ink outline. Plain Portrait, Balanced and Landscape headings separate the stacked groups. Only the preset matching both the board's shape and dimensions is chosen; a custom size chooses no preset. These use the compact Back/Menu role while Home/pause-menu quiet buttons stay 52 px, without changing the legacy `seat-picker` token.
- **Stepper:** − and + drawn as strokes in 40 px circular shared Ground faces inside 44×44 px hit areas, with the value set in the Number style between them. Disabled controls use tile fill and dim marks with no hover.
- **Toggle:** on is an ink track with a mat-coloured knob on the right. Off is a mat track with a dim outline and a dim knob on the left. Hover turns the outline to ink.
- **Slider:** a mat-coloured track with an ink fill and a 12 px ink knob. Everything turns dim while sound is muted.

### Navigation
- **Back and Menu** are one compact component: a tile pill with a 40 px face in a 44 px target, the stroked glyph centred 22 px from its left edge and the label at 42 px. Back is 112 px wide on Setup and Settings (arrow only, 44 px wide, on pages narrower than 540 px); Menu spans the game rail, and the full width at the bottom of the portrait game.
- **Touch:** a finger acts on release, and only if it has not dragged more than 10 px, so a drag scrolls Setup without pressing what it started on; the volume knob follows a finger from the moment it lands. A lifted finger leaves no hover behind. Android's back button acts as Esc, and on Home it sends the app to the background.
- **Menu pill** on the game rail (hamburger + "Menu") opens the pause dialog: Resume (primary), Restart, Settings, Main menu.
- **Settings tabs:** Video and Audio, as pills (the chosen style for the active tab, tile fill otherwise).
- **Keys:** only Esc, M, Space and F3 are bound. No key is ever printed on a button.

### Spinner (signature)
A pie with one wedge per seat in that seat's ink, split by 3 px lines in the mat colour. Seats that are out are drawn as blank tile. There is a mat-coloured hub and an ink pointer aimed at the player to move. In Colorblind mode each living wedge carries its mark.

### Chain Callout (signature)
A pill in the mover's ink reading "Chain ×N" in Unbounded 800, in `inkOn` contrast, always exactly the spinner's width. It appears for chains of 2 or more and fades out. On a phone on its side it floats across the spinner's middle.

### Board, Orbs and Bursts (signature)
Board tiles are rounded squares, printed blank or lit 30% toward the mover's ink. Orbs are flat discs in their owner's ink with a rim in the mat colour, plus their mark in Colorblind mode, all drawn in one batch. A burst is a flat ring in the exploding player's ink that expands over 18 ticks and fades. The hover tile gets an outline in the mover's ink. A cell holding more than four orbs gets an ink count badge.

### Winner Flood
The winner's ink expands from the board centre to cover the screen. On top of it sit the winner's dot ringed in the contrasting ink, the title in the Display face, a body line, and the result buttons.

## Do's and Don'ts

### Do:
- **Do** keep every fill flat and solid. Change the fill or add an ink outline to separate layers; quiet-button hover uses pressed tile fill and chosen controls keep a 2.5 px ink outline.
- **Do** keep hue for players: orbs, seat dots, wedges, turn tint (30%), bursts, the chain callout and the winner flood.
- **Do** use solid ink for exactly one primary action per screen, and quiet tile or mat pills for everything else.
- **Do** pick the text and mark colour on a player ink with `inkOn` (Press Ink or Mark White).
- **Do** add the seat's shape mark everywhere its colour appears when Colorblind mode is on.
- **Do** define every value for both modes, light and dark.
- **Do** ask the user before making any new visual decision.

### Don't:
- **Don't** use 3D, shadow depth, gradients, glows or neon.
- **Don't** colour chrome with a player ink, and don't add an accent colour outside the twelve.
- **Don't** print key hints on buttons or add explanatory subtext under labels.
- **Don't** bind keys beyond Esc, M, Space and F3.
- **Don't** build menus that differ from game convention: one Menu button opens the pause dialog, and Settings is tabbed.
- **Don't** use glyph or emoji icons. Draw icons as stroked paths.
