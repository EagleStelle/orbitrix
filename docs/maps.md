# Maps and Map Editor

Open **Maps** from Home or from the Board preview in Setup. Choose a saved map, or choose **Classic board** to use the existing rectangular presets. Maps are saved on this device, never uploaded to a catalogue. Browser storage belongs to the current browser and site; export files for backups.

The library starts with Crossroads, Long Distance, and Pressure Points. Select a map to edit, rename, duplicate, delete, export, share, or use it. Invalid designs remain editable drafts; they cannot start a match.

## Terms and tools

A **map** is the saved design. A **board** is that design in a match. A **cell** is one grid position, with base **terrain** and optionally one **feature**. An **edge** is the boundary between adjacent cells; a **connection** is a route an explosion actually uses. **Capacity** is the orb count that triggers an explosion.

| Element | Behavior |
|---|---|
| Floor | Playable cell. Capacity equals its open connections. |
| Wall | Permanent obstruction occupying a cell. |
| Void | Missing surface, outside the playable area. Blocks connections like Wall. |
| Breakable wall | Starts with two durability. Each adjacent exploding cell deals one damage per wave; destruction creates empty Floor. |
| Link | A pair of distant Floor cells are extra neighbors. Each endpoint sends one orb to the other when exploding. Arrivals land normally. |
| Reservoir | Capacity is one above the number of connections. Explosions send one orb per connection, retaining the reserve. |
| Latch | Presses on the first accepted placement or orb arrival and stays pressed for the rest of the match. |
| Toggle | Flips when an orb is placed into its empty cell, or once per wave when orbs pop into or out of it. Adding orbs without a pop does not flip it. |
| Hold | Pressed while its cell holds at least one orb; releases when empty. |
| Barrier | Permanently blocks the connection across an edge between Floor cells. |
| Gate | Starts closed. Opens while its circuit's switch is pressed, and closes when that switch releases. |

The palette groups **Terrain** (Floor, Wall, Void, Barrier), **Structures** (Breakable wall, Gate), and **Devices** (Link, Reservoir, Latch, Toggle, Hold). Terrain stays unchanged during play; Structures can change through damage or switch control. Desktop shows a compact icon-and-label list. Short landscape uses a category picker and one list; portrait uses category tabs and a compact icon-and-label shelf. The selected tool stays outlined. Its inspector shows only a short functional description, plus a relationship swatch when relevant; it does not repeat the tool's name or icon.

Edit the map name in the header, beside **Test**, **Save**, and **More** (the three-dot button). Undo, Redo, Erase, Pan, zoom out, Fit, and zoom in remain beside the canvas. Drag to paint one undoable stroke. Edge tools trace the drag between grid corners: horizontal and vertical passes stay on that line, and a held drag can turn corners without wrapping each Floor cell. A tap places one edge. Erase removes an edge obstruction when it hits one; otherwise it restores Floor. Pan and zoom make large maps reachable; Fit resets both.

Link placement takes two distant Floor endpoints. The inspector's **Pair color** or **Circuit color** swatch opens a single page of twelve group colors, taken directly from the orb palette in the same order. Link pairs and switch circuits use separate identity sets within those twelve colors. Map art and relationship swatches stay identical with Colorblind enabled or disabled. Neither state adds black/white stamps, notches, or identity patterns; player and orb marks elsewhere stay unchanged. A contextual line helps find a Link's partner during placement or hover.

**More** contains map dimensions, **Show capacities**, and **Preview pressed switches**. The preview shows every switch pressed and its Gates open, and capacities follow that preview; it does not change the saved design or the closed starting state. The **Issues** button cycles validation messages; affected cells are outlined. Invalid drafts can be saved, but Test remains disabled until they are valid.

Save commits the draft to the library. Unsaved work is recovered separately after editing stops; **Recover draft** resumes it. Leaving an edited map asks before discarding changes. **Test** opens the familiar player/bot setup with private preferences. Escape/Android Back returns from test play to the unchanged editor draft.

## Valid maps

Dimensions are 3–40 cells per side. With every Gate closed and Links included, each initial Floor must have capacity of at least two. A Reservoir makes Floor with one open connection valid by raising its capacity to two; Floor with no open connections remains invalid even with a Reservoir. Every Breakable Wall must border at least two initial Floor cells. These checks preserve the opening round.

All initial Floor must eventually connect, counting Breakable Walls as future Floor. Gates may separate Floor groups when their matching switch is reachable from at least one side without opening that Gate first. Reachable Gates can connect further switches and Gates; circular dependencies are invalid. Links also connect Floor groups. Wall, Void, and Barriers remain permanent obstructions, so regions enclosed only by them are invalid.

Links need exactly two distinct, nonadjacent Floor endpoints. Features require Floor. Barriers and Gates require an orthogonal Floor edge, with only one element on that edge. Each circuit needs exactly one controller (Latch, Toggle, or Hold) and at least one Gate. The editor refuses a second controller in a circuit. Player count cannot exceed initial Floor count.

Playable maps use group identities 1–12. Older maps with higher group identities remain saveable, shareable drafts, with the affected elements highlighted; reassign those elements to one of the twelve colors before starting a match. Imports do not silently recolor or discard them.

## Shared map art

Palette, canvas, library thumbnail, and match board share the same component art and the map-device rules in DESIGN.md. Devices use one outline weight, `max(0.8 px, cell size * 0.04)`, Wall-colored (`sc.dim`) neutral strokes, no circles, and no incidental detail. Solid fills differ from Floor and Void. All device outer frames share exact Floor tile bounds, `r.inset(cell * tileGutter)`, and radius `cell * tileRadius`, with no surrounding Floor gap. All three switches share inner inset `body.w * 0.1875` and inner radius `max(0, outerRadius - inset)`; Latch and Hold have identical inner rectangles.

Links have a rounded-square colored rim and dark opening using the exact Floor corner radius, with two separated U paths reproducing the large lower arc and small upper arc. Their internal bends inherit the outer rounded geometry with inset-derived radius `min(halfSpan, max(strokeWidth, frameRadius - (body.w / 2 - halfSpan)))`, at least the stroke width within the path span; no right-angle bends. Arrangement, tips, width and pair ink stay unchanged. The opening uses `mix(ink, dim, 0.18)` in light mode and `mix(ground, dim, 0.18)` in dark mode. The two paths always remain visible with Colorblind enabled or disabled. Latch retains its rounded-square housing and button with normalized strokes, outlined OFF/released and solid circuit-color fill ON/pressed with unchanged geometry. Toggle uses the same rounded outer frame and full shared inner rectangle and gaps as the other switches. Its button has two equal adjacent halves and a center seam: left solid circuit-color fill means OFF, right solid fill means ON, with the opposite half outlined. The selected-half fill is clipped to the rounded inner geometry; the full inner outline and vertical seam use the shared stroke width. Hold is top-down: four corner brackets clipped from the shared rounded frame surround a central plate identical to Latch's inner rectangle, outlined when released and filled in circuit color when pressed. Reservoir is a top-down square well with the exact Floor radius on its outer outline and one unchanged square inset reserve pocket representing the one orb retained after exploding, without a machine housing or side view. Both Reservoir outlines are neutral Wall ink when empty, use the occupying player's exact ink while occupied, and return to neutral when empty. Ownership follows the displayed animation wave. Four symmetrical cardinal channel strokes extend from the central square pocket toward the outer basin, stopping two stroke widths before the frame, with the same shared stroke and neutral/owner ink. Frame and radius stay unchanged. Links and switches retain their pair/circuit inks.

Devices, terrain, Gates and relationship swatches keep identical normal art in both Colorblind states, with no map group marks or alternate symbols. Player and orb symbols elsewhere are unchanged. Wall fill and joins and structural Gate/Barrier thickness remain unchanged.

Breakable walls show a connected, tapered negative-space fracture rooted at the center: three main splits and two forks, visibly identifying breakability at initial durability 2. At durability 1 the same whole connected network grows 1.24x, longer and wider toward the edges. Use branching fractures rather than repetitive zigzags; no random texture, detached flecks or circles. Fracture cuts are structural damage, not additional device-icon stroke weights. Wall fill and joins stay unchanged; destruction creates empty Floor.

## Sharing and files

Exports use `.orbitrixmap`, portable format version 2, with zlib compression, a 64 KiB bound on both file and decompressed payload, and a checksum. Share codes start with `orbitrix-map2.`. Files, share codes, and QR payloads carry the same map definition. Files and pasted codes work offline. Imports show a preview and require **Save**; they never replace another map automatically.

Legacy format-1 files and `orbitrix-map1.` codes still import. Their one-shot Switch is shown as Latch and follows the current arrival/placement rule. A legacy map with multiple controllers in one circuit remains an invalid, editable draft. Importing or opening it never silently removes controllers, repairs it, or overwrites the stored design; any correction requires an explicit edit and Save. New exports use format 2.

In browsers and native builds configured with `ORBITRIX_SERVER`, QR sharing uses `/play#map=…`. The complete map is in the URL fragment and is not sent to the website. The link targets the game's import preview; the phone-camera handoff still needs device verification. Loading the web application may require connectivity. Large payloads use file/code sharing instead of an impractically dense QR. Native development builds without a configured website provide code/file sharing; their raw code QR requires a scanner capable of copying text.

Browser imports use a file picker and exports download a file. Android uses the system document picker. Other native builds export into the app data directory's `exports` folder and accept a pasted file path for import.

In a room the host sends the selected map automatically. Joining does not add it to your library. Use **Save map** in the match menu to preview and save a copy.

## Rules and compatibility

Explosions preserve every orb: each accepted placement adds exactly one, and terrain neither creates nor consumes them. A direct accepted placement updates its switch and circuit immediately, before choosing the first exploding cells. This is recorded as terrain events at wave −1.

Each explosion wave uses the connections open at its start. All transfers finish before wall damage and switch effects apply together. Receiving one or more orbs activates Latch once in that wave; exploding by itself never activates Latch. Toggle flips on direct placement into an empty cell, and once when receiving or exploding in a wave, including when both happen together; adding to an occupied Toggle without a pop does not flip it. Hold is pressed exactly when its post-transfer count is above zero. Gates follow the resulting controller state, including closure. Connections and capacities are rebuilt before selecting the next wave, so a closing Gate can leave a neighboring cell ready to explode. Victory is checked after the current wave's effects.

Map definitions contain no match state. Snapshots and resync carry wall durability, per-cell `Pressed` state, and `OpenGates`; the state hash includes the design and these values. Runtime topology is isolated between AI search copies. Animation replays placement events before the first explosion and wave events after the corresponding transfers.

Map sounds follow the same displayed steps: a glass chime when an exploding Link sends an orb across its pair,
distinct clicks when Latch, Toggle or Hold changes state, a lower note when Toggle or Hold releases, and separate
woody damage and destruction cues for breakable walls. Repeated arrivals on an unchanged switch stay silent.
All cues use the orb sounds' C major pentatonic scale, saved mute and volume, shared rate limit and voice cap.
Rate-limited map cues coalesce for the next burst instead of disappearing, including after a chain's final wave.

The game protocol is version 3. Update clients and the server together. The portable map format has its own version and legacy import path; it is separate from protocol compatibility. Classic rectangular gameplay remains available.

All twelve bots remain legal and deterministic on maps. Sovereign's existing network encodes rectangular neighborhoods with at most four neighbors; on custom maps it uses the existing Tactician search until a map-aware network is trained. Prodigy uses a full network evaluation for custom maps so topology changes cannot leave cached features stale.

Physical-phone touch painting and camera-to-import behavior still need device verification; desktop captures and automated checks do not establish those results.

In the editor, click the map title to rename it and press Enter to finish. On a keyboard, Ctrl/Cmd+Z undoes, Ctrl/Cmd+Shift+Z or Ctrl+Y redoes, and E selects Erase. Editor shortcuts are inactive while renaming or using a dialog. Validation messages appear automatically in the bottom snackbar.

The editor toolbar groups capacity and pressed-switch preview toggles separately from Test and Save. Map size stays in the options menu. Erase highlights the cell or edge that will be removed. Validation outlines faulty Barriers and Gates themselves, using the same edge outline as Erase; floor issues outline their cells.

Hold the middle mouse button on the editor canvas to pan, or hold the right mouse button to erase. Releasing restores the previously selected tool; each right-button erase stroke is undoable. Touch controls continue to use the visible Pan and Erase tools.

Keyboard view controls: press M to toggle Move; press M again to restore the previous tool. Use +/= or numpad + to zoom in, - or numpad - to zoom out, and 0 (also Ctrl/Cmd+0 or numpad 0) to fit and recenter the map. View shortcuts are inactive while typing a title or using a dialog.

Ordinary editor snackbar messages dismiss automatically after five seconds. Validation issues stay visible until resolved.
