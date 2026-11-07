// Package mobile is the ebitenmobile binding for Android. ebitenmobile bind turns it into the AAR the
// Gradle project in /android wraps; its exported functions become static methods of the Java class Mobile.
package mobile

import (
	"errors"
	"sync"

	ebitenmobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"orbitrix/client"
	"orbitrix/client/platform"
	"orbitrix/core"
)

// Host is what the Android activity does for the game.
type Host interface {
	// Leave sends the app to the background, as the back button does on Android's root screen.
	Leave()
	// Rotate holds the screen in landscape, or lets it turn freely again.
	Rotate(landscape bool)
	// KeepScreenOn keeps the screen awake while the player hosts a room, online or on the LAN.
	KeepScreenOn(on bool)
	// Copy puts the invite code on the system clipboard.
	Copy(text string) bool
	// Keyboard sets up the on-screen keyboard for the next field: a password's (secret) or a room code's.
	Keyboard(secret bool)
	// Discover holds the Wi-Fi multicast lock while the player looks for LAN rooms, so hosts' broadcasts get through.
	Discover(on bool)
	// DeviceName is the name the user gave the phone, which other players see its LAN rooms listed under.
	DeviceName() string
	// PickMapFile opens Android's document picker and answers through MapImported.
	PickMapFile()
	// ExportMapFile lets the user choose where to write a portable map, then answers through MapExported.
	ExportMapFile(name string, data []byte)
}

var start sync.Once

var mapFiles struct {
	sync.Mutex
	host     Host
	imported func([]byte, error)
	exported func(string, error)
}

// Start runs the game, saving its settings in dataDir. The activity calls it before showing the game's view; later
// calls (a recreated activity) keep the running game.
func Start(dataDir string, host Host) {
	mapFiles.Lock()
	mapFiles.host = host // recreated activities must open pickers on the current activity
	mapFiles.Unlock()
	start.Do(func() {
		platform.SetDataDir(dataDir)
		platform.Leave = host.Leave
		platform.Rotate = host.Rotate
		platform.Hosting = host.KeepScreenOn
		platform.Copy = func(text string, done func(bool)) { done(host.Copy(text)) }
		platform.Keyboard = host.Keyboard
		platform.Discover = host.Discover
		platform.DeviceName = host.DeviceName
		platform.PickMapFile = pickMapFile
		platform.ExportMapFile = exportMapFile
		ebitenmobile.SetGame(client.New())
	})
}

func pickMapFile(done func([]byte, error)) bool {
	mapFiles.Lock()
	if mapFiles.imported != nil || mapFiles.exported != nil || mapFiles.host == nil {
		mapFiles.Unlock()
		return false
	}
	mapFiles.imported = done
	host := mapFiles.host
	mapFiles.Unlock()
	host.PickMapFile()
	return true
}

func exportMapFile(name string, data []byte, done func(string, error)) {
	if len(data) > core.MaxMapBytes {
		done("", errors.New("map file is too large"))
		return
	}
	mapFiles.Lock()
	if mapFiles.imported != nil || mapFiles.exported != nil || mapFiles.host == nil {
		mapFiles.Unlock()
		done("", errors.New("finish the open file picker first"))
		return
	}
	mapFiles.exported = done
	host := mapFiles.host
	mapFiles.Unlock()
	host.ExportMapFile(name, append([]byte(nil), data...))
}

// MapImported answers the pending document picker. A nonempty message reports cancellation or an I/O error.
func MapImported(data []byte, message string) {
	mapFiles.Lock()
	done := mapFiles.imported
	mapFiles.imported = nil
	mapFiles.Unlock()
	if done == nil {
		return
	}
	if message != "" {
		done(nil, errors.New(message))
	} else if len(data) > core.MaxMapBytes {
		done(nil, errors.New("map file is too large"))
	} else {
		done(append([]byte(nil), data...), nil)
	}
}

// MapExported answers the pending save picker. An empty message means the file was saved.
func MapExported(message string) {
	mapFiles.Lock()
	done := mapFiles.exported
	mapFiles.exported = nil
	mapFiles.Unlock()
	if done == nil {
		return
	}
	if message != "" {
		done("", errors.New(message))
	} else {
		done("File saved", nil)
	}
}

// SetSafeArea reports the screen edges the display's cutout covers, in pixels.
func SetSafeArea(left, top, right, bottom int) { platform.SetSafeArea(left, top, right, bottom) }

// Back reports a press of the back button; the game treats it as Esc.
func Back() { platform.PressBack() }
