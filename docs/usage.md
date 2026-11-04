# Usage guide

## Rules

- Each cell holds orbs and belongs to the player whose orbs are in it, or to nobody when empty.
- On your turn you place one orb in an empty cell or one you own. Cells owned by others are off limits; tapping one flashes its outline.
- A cell's capacity is its number of neighbours: 2 in a corner, 3 on an edge, 4 inside. When a cell reaches it, it explodes: it sends one orb to each neighbour, every neighbour it reaches becomes yours, and any orbs left over stay behind.
- Explosions resolve in waves. Every full cell explodes at once, then the cells those orbs filled explode in the next wave, and so on.
- You are out when you have no orbs left after your first turn. Once everyone has had a turn, the moment only one player has orbs on the board, that player wins, even in the middle of a chain.

## Home

Home shows the ORBITRIX wordmark, the twelve seat colours, a live bot game on the mat, and the PLAY dot with Settings and Exit under it. Exit is missing in the browser.

Tap the welcome line above PLAY ("Welcome, Human!" at first) to type your name on the device's keyboard. It labels your seat everywhere: your first human seat on this device, and your seat in any room you join or host.

PLAY swaps itself for the ways to play:

- This Device: alone against bots, or with friends passing one device around.
- Same Wi-Fi: rooms on the local network. Native apps only; never in a browser.
- Online: rooms through the site's server. Present in the web build and in native builds made with `ORBITRIX_SERVER`.
- Back.

In a browser on a site without a server, PLAY goes straight to Setup.

## Setup

- Players: the heading's − and + set the count from 2 to 12. Each seat is Human or one of the twelve bots, with its difficulty shown as five pips. The seat dropdown also has Swap position, which trades this seat with another (two humans never trade), and Remove player, which takes the seat out and moves the seats after it up.
- Board: a mini-board preview, then eighteen presets in Portrait, Balanced and Landscape sections, each with six sizes from Coaster (5×7) to Floor (13×18). Columns and Rows go from 3 to 40; a size that matches no preset reads Custom.
- Start reads "Board too small" when there are more players than cells.

Your choices are saved for next time.

## The game

The board fills the screen beside a rail with Menu, a turn wheel showing each player's orb count, the "Chain ×N" callout and the seat table. Held upright, a phone puts the seats and a current-player dot under the board instead. Only the player to move has their tiles lit in their colour.

Bots wait 300 to 600 ms before moving, so you can follow them. Long chains play back faster and faster; Space or a tap finishes the playback at once.

Menu (or Esc) opens Resume, Restart, Settings and Main menu. Restart asks for a second tap. Online, the menu has Resume, Settings and Main menu, and the game carries on behind it.

When a player wins, their colour floods the screen with the move count and the biggest chain, then Play again, Main menu and Exit.

### Keys

| Key | Action |
|---|---|
| Esc | Pause menu, or back |
| M | Mute (types an M while a text field has the focus) |
| Space | Skip the rest of a chain |
| F3 | Frame-rate overlay |

On Android the back button acts as Esc, and on Home it sends the app to the background.

## The bots

From easiest to hardest:

| Bot | Plays |
|---|---|
| Jester | A random legal move |
| Mason | Builds up dense groups of its own cells |
| Warden | Defensively, keeping a safe buffer and reacting to threats |
| Gambler | Risky moves that shift with its mood: hot streak, desperate, chasing or cautious |
| Nomad | Sparse single orbs spread over open ground, racing enemy neighbours |
| Raider | Aggressively, pressing enemies with its best explosions |
| Spy | Reads where enemy chains can reach and infiltrates where it explodes first |
| Prodigy | AlphaZero-style tree search guided by a self-play-trained network |
| Tactician | Iterative-deepening alpha-beta search |
| Voyager | Monte Carlo tree search |
| Hunter | Best-reply search with capture quiescence, aimed at the leader |
| Sovereign | Alpha-beta and best-reply search over an integer NNUE-style network |

Every bot is deterministic for a given position and seed, and each thinks for a fixed amount of work rather than a fixed time.

## Same Wi-Fi

Every native build can host or join on the local network, with or without internet.

1. One player opens Same Wi-Fi and taps Set up a room. The room page is Setup with a round key button beside the action.
2. The key button opens the Room dialog: Public or Private. A private room shows its six-character code and a copy button. The choice applies on Confirm.
3. Open room starts listening. Joiners find a public room under Rooms nearby, named after the host device, and tap it. A private room is joined by typing its code under Join by code.
4. Seats stay changeable until the host presses Start. Open seats are filled with bots.

On Windows the first Same Wi-Fi tap explains that Windows will ask to let Orbitrix on the network. Allow it for private networks. If the host's device sleeps or the Android app goes to the background, the room pauses and joiners retry for 60 seconds.

## Online

1. The host opens Online and taps Set up a room. When the server has a host password, the host types it once; the client saves it and asks again only if the server refuses it.
2. Open room creates the room. A public room is listed under Public rooms with its board size; a private one is reached by its code, or by the `https://<site>/join/<CODE>` link, which opens the game with the code filled in.
3. Each join request appears as a bar above the action with No and Let in. More waiting requests show as "+N".
4. Start fills open seats with bots.

The game runs on the host's device. If the host leaves, the game ends for everyone. A browser host must keep the tab open; Android keeps the screen on while hosting and warns that leaving the app pauses the game. A player whose connection drops can rejoin the same seat for 60 seconds without being asked about again.

## Settings

| Tab | Options |
|---|---|
| Video | Theme (light or dark), Fullscreen (not on phones), Colorblind mode |
| Audio | Sound, Volume |
| Network | The access new Same Wi-Fi rooms and Online rooms start with (Public or Private), and Force relay in builds that play online |

Settings are saved to `%AppData%\Orbitrix\settings.json` on Windows, the app's files folder on Android and the browser's `localStorage` on the web.

## Common problems

| Problem | Fix |
|---|---|
| No Online button in a native build | The build was made without `ORBITRIX_SERVER`. Use a release download, or rebuild with it set. |
| "Update Orbitrix" | The server speaks a newer protocol than this client. Install the latest release. |
| "Wrong password" when hosting | The server's host password changed. Type the new one. |
| A friend can't connect online | Their network may block direct connections. The server needs TURN configured; Force relay under Settings > Network tests it. |
| No rooms under Rooms nearby | Both devices must be on the same network, and Windows must allow Orbitrix on private networks. Guest networks often block devices from seeing each other. |
| The Linux build exits at once | It needs a sound device (PulseAudio, PipeWire or ALSA) and X11 or XWayland with OpenGL. |
