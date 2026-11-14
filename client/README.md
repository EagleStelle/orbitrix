# Client code

The client uses Go 1.27.3 and Ebitengine v2.10.5. Desktop, browser (Go WebAssembly),
and Android entry points all use `client.New()` and `client.Game`.

## Find the code

| Location | Responsibility |
| --- | --- |
| `game.go` | Shared application state, Ebitengine frame loop, screen transitions and layout |
| `input.go` | Mouse, touch, keyboard-back input and hit testing |
| `settings.go` | Saved preferences, seat choices and rejoin details |
| `scene_home.go`, `scene_setup.go`, `scene_settings.go` | Home, Setup (This Device and hosted rooms) and settings screens |
| `setup_parts.go` | Shared Setup parts: the Rules card and its option tiles, the board size picker |
| `icons.go` | Round icon buttons, and the line-drawn icons for them and for the Rules options |
| `scene_play.go` | Match controls, AI scheduling, pause menu and results |
| `scene_join.go`, `scene_room*.go`, `scene_password.go`, `scene_reply.go`, `scene_gone.go` | Joining, lobby, invitations, host password, direct replies and disconnect screens |
| `scene_maps.go` | Map library selection and actions |
| `scene_maps_import.go`, `scene_maps_share.go` | Portable map import, export, links and QR sharing |
| `scene_mapeditor.go`, `scene_mapeditor_view.go` | Editor state, tools, undo and recovery; editor layout and drawing |
| `page.go`, `dialog.go`, `snackbar.go`, `textfield.go` | Application page framing, dialog actions, notification routing and keyboard focus |
| `scene_play_counts.go` | Match-specific count indicators and player-wheel drawing |
| `internal/ui/` | Independent geometry, palettes, fonts, drawing, text caches, controls, scrolling and QR rendering |
| `internal/boardview/` | Board batches, shared terrain art, wave replay, orb motion and count geometry |
| `session.go`, `session_play.go` | Room state, routing, move synchronization and resync |
| `internal/sound/` | Generated pentatonic notes and map instruments, combined playback, volume and shared voice limits |
| `internal/mapstore/` | Map library format, validation and transactional persistence |
| `online/` | WebRTC hosting, joining and direct invite/reply connections |
| `platform/` | Storage, clipboard, device services and browser/native implementations |
| `fonts/` | Font embedding, font files and their OFL licenses |

## Placement rules

Screens and their orchestration share private application state in `package
client`; the `scene_` file prefix is this project's navigation convention.
Reusable presentation code lives in `internal/ui`, and board rendering and
animation live in `internal/boardview`. This uses [Go's supporting-package
convention](https://go.dev/doc/modules/layout); there is no prescribed Go game
folder layout.

Keep tests beside the implementation with the same file prefix. Cross-screen
layout checks live in `scene_layout_test.go`. Capture tests keep the `capture`
build tag, and frame timing checks keep the `perf` build tag. Platform suffixes
and build tags stay in `platform/`.

The dependency direction is `client` to its supporting packages. Shared UI takes
`ui.Context` values containing presentation and pointer data, never `*app`.
`boardview` consumes game state and UI primitives without importing scenes.
Neither package imports `client`. Drawing scratch state stays private to its
owner. Scene-specific count drawing stays beside the play screen.

Shared rules have one owner: `ui.Numbers` caches numeric labels, `field` filters
committed and composing text, `session.playerLabel` marks this device's seat, and
`ui.Scroll` handles both scroll axes. Platform-specific implementations stay
separate because they use different OS/browser services.

Board rendering and orb motion read the displayed geometry from the replay
state; they do not keep geometry copies that need separate synchronization.

Map storage accepts read/write functions; `platform` continues to own actual
filesystem or browser storage. File names, JSON formats and font asset paths are
unchanged. Generated builds and captures belong in the repository root's `dist/`.

## Check and run

Run these commands from the repository root:

```sh
go tool mage check
go tool mage desktop web
go run ./cmd/orbitrix
```

For the browser build:

```sh
go run ./cmd/orbitrix-server -web dist/web
```

Then open `http://localhost:1212/play`. See [testing](../docs/testing.md) for
integration, capture and performance checks.
