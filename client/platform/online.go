//go:build !js

package platform

import "os"

// origin is the website and signaling server native builds talk to, stamped in by magefile.go from
// ORBITRIX_SERVER (-ldflags "-X orbitrix/client/platform.origin=https://...").
var origin string

// HasWebRTC reports whether this device can make direct peer connections.
const HasWebRTC = true

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

// Address is where the game's page was loaded from (scheme://host), which server-free rooms' links point at, or ""
// outside a browser.
func Address() string { return "" }

// LANHost is the LAN host (host:port) that served the game's page, or "". Only the web build has one.
func LANHost() string { return "" }

// Invite is the server-free room's invite in the link the game was opened with, or "". Only the web build has one.
func Invite() string { return "" }

// Reply takes a joiner's reply to the server-free room hosted here, left by a reply link opened on this device, or
// "". Only the web build gets one.
func Reply() string { return "" }

// Hosting keeps the device awake while this player hosts a room (on), and lets it sleep again (off). The
// mobile wrapper sets it.
var Hosting = func(on bool) {}

// Copy puts text on the clipboard. Android supplies its system clipboard through the mobile wrapper.
var Copy = func(text string, done func(bool)) { done(copyText(text)) }
