// Package mobile is the ebitenmobile binding for Android (and later iOS). ebitenmobile bind turns it into the AAR the
// Gradle project in /android wraps; its exported functions become static methods of the Java class Mobile.
package mobile

import (
	"sync"

	ebitenmobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"orbitrix/client"
	"orbitrix/client/platform"
)

// Host is what the Android activity does for the game.
type Host interface {
	// Leave sends the app to the background, as the back button does on Android's root screen.
	Leave()
	// Rotate holds the screen in landscape, or lets it turn freely again.
	Rotate(landscape bool)
}

var start sync.Once

// Start runs the game, saving its settings in dataDir. The activity calls it before showing the game's view; later
// calls (a recreated activity) keep the running game.
func Start(dataDir string, host Host) {
	start.Do(func() {
		platform.SetDataDir(dataDir)
		platform.Leave = host.Leave
		platform.Rotate = host.Rotate
		ebitenmobile.SetGame(client.New())
	})
}

// SetSafeArea reports the screen edges the display's cutout covers, in pixels.
func SetSafeArea(left, top, right, bottom int) { platform.SetSafeArea(left, top, right, bottom) }

// Back reports a press of the back button; the game treats it as Esc.
func Back() { platform.PressBack() }
