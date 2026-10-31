package platform

import (
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
