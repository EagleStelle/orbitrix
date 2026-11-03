package platform

import (
	"os"
	"runtime"
	"sync/atomic"
)

// CanFullscreen reports whether the player can switch fullscreen on and off. Phones are always full screen.
const CanFullscreen = runtime.GOOS != "android" && runtime.GOOS != "ios"

// The mobile wrapper reports the back button and the screen's safe area from its UI thread; the game reads them
// on its own goroutine.
var (
	back atomic.Bool
	safe [4]atomic.Int32 // left, top, right, bottom insets in device px
)

// Leave sends the app to the background, as Android's back button does on the root screen. The mobile wrapper
// sets it; elsewhere it does nothing.
var Leave = func() {}

// Rotate holds the screen in landscape (true) or lets it turn freely again (false). The mobile wrapper sets it; it
// stays nil where the app can't turn the screen itself (desktop, web).
var Rotate func(landscape bool)

// Keyboard sets up the on-screen keyboard for the field taking the focus: a password's (secret), which the keyboard
// neither suggests nor learns, or a room code's (capitals, no suggestions). The mobile wrapper sets it; it stays nil
// elsewhere.
var Keyboard func(secret bool)

// Discover holds Android's multicast lock while the player looks for LAN rooms (on), without which the Wi-Fi drops
// the hosts' broadcasts. The mobile wrapper sets it; elsewhere it does nothing.
var Discover = func(on bool) {}

// DeviceName is the name this device goes by, which other players see its LAN rooms listed under. The mobile wrapper
// sets it on Android, whose host name is "localhost".
var DeviceName = func() string {
	name, _ := os.Hostname()
	return name
}

// PressBack reports a press of Android's back button.
func PressBack() { back.Store(true) }

// TakeBack reports whether back was pressed since the last call.
func TakeBack() bool { return back.Swap(false) }

// SetSafeArea sets the insets, in device px, that the display's cutout takes from each edge.
func SetSafeArea(left, top, right, bottom int) {
	for i, v := range [4]int{left, top, right, bottom} {
		safe[i].Store(int32(v))
	}
}

// SafeArea returns the insets set by SetSafeArea: zero on desktop and web.
func SafeArea() (left, top, right, bottom float64) {
	return float64(safe[0].Load()), float64(safe[1].Load()), float64(safe[2].Load()), float64(safe[3].Load())
}
