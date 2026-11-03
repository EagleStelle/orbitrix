package platform

import (
	"strings"
	"syscall/js"
)

// Origin is the address the page was loaded from: the web build always plays against its own server.
func Origin() string { return js.Global().Get("location").Get("origin").String() }

// JoinCode is the room code in a /join/<code> address, or "".
func JoinCode() string {
	code, ok := strings.CutPrefix(js.Global().Get("location").Get("pathname").String(), "/join/")
	if !ok {
		return ""
	}
	return strings.ToUpper(strings.Trim(code, "/"))
}

var (
	wakeLock     js.Value
	beforeUnload = js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("preventDefault") // the browser asks before closing a tab that hosts a room
		args[0].Set("returnValue", "")
		return nil
	})
)

// Hosting keeps the screen awake and asks before the tab closes while this player hosts an online room.
func Hosting(on bool) {
	w := js.Global()
	if !on {
		w.Call("removeEventListener", "beforeunload", beforeUnload)
		if wakeLock.Truthy() {
			wakeLock.Call("release")
			wakeLock = js.Undefined()
		}
		return
	}
	w.Call("addEventListener", "beforeunload", beforeUnload)
	if wl := w.Get("navigator").Get("wakeLock"); wl.Truthy() {
		var then js.Func
		then = js.FuncOf(func(this js.Value, args []js.Value) any {
			wakeLock = args[0]
			then.Release()
			return nil
		})
		// shortcut: a wake lock ends when the tab is hidden and isn't taken again on return; re-request it on
		// visibilitychange if hosts' screens still sleep.
		wl.Call("request", "screen").Call("then", then)
	}
}

// Share hands text to the browser's share sheet, or copies it where there is none, and reports whether it copied.
func Share(text string) (copied bool) {
	nav := js.Global().Get("navigator")
	if nav.Get("share").Truthy() {
		nav.Call("share", map[string]any{"url": text})
		return false
	}
	if cb := nav.Get("clipboard"); cb.Truthy() {
		cb.Call("writeText", text)
		return true
	}
	return false
}
