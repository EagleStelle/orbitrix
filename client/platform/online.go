//go:build !js

package platform

import "os"

// origin is the website and signaling server native builds talk to, stamped in by magefile.go from
// ORBITRIX_SERVER (-ldflags "-X orbitrix/client/platform.origin=https://...").
var origin string

// Origin is the server's address (https://host, no trailing slash), or "" when this build has none and so can't play
// online. On desktop, ORBITRIX_SERVER overrides the stamped one, for testing against a local server.
func Origin() string {
	if o := os.Getenv("ORBITRIX_SERVER"); o != "" {
		return o
	}
	return origin
}

// JoinCode is the room code the app was opened with (a /join/<code> link), or "". Only the web build has one.
func JoinCode() string { return "" }

// Hosting keeps the device awake while this player hosts a room (on), and lets it sleep again (off). The
// mobile wrapper sets it.
var Hosting = func(on bool) {}

// ShareSheet opens the system share sheet with text. The mobile wrapper sets it; elsewhere it stays nil.
var ShareSheet func(text string)

// Share hands text to the share sheet (Android) or copies it to the clipboard (desktop), and reports whether it
// copied, so the caller can say so. Where it can do neither the caller shows the text anyway.
func Share(text string) (copied bool) {
	if ShareSheet != nil {
		ShareSheet(text)
		return false
	}
	return copyText(text)
}
