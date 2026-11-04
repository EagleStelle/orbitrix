package platform

import (
	"strings"
	"syscall/js"
)

// HasWebRTC reports whether this browser provides direct peer connections.
var HasWebRTC = js.Global().Get("RTCPeerConnection").Type() == js.TypeFunction

// lanPage reports that a LAN host served the page, marking it so (server/lan's web.go).
var lanPage = js.Global().Get("document").Get("documentElement").Call("hasAttribute", "data-lan").Bool()

// Origin is the address the page was loaded from: the web build always plays against its own server. A LAN host's
// page has none.
func Origin() string {
	if lanPage {
		return ""
	}
	return Address()
}

// Address is where the page was loaded from (scheme://host).
func Address() string { return js.Global().Get("location").Get("origin").String() }

// LANHost is the LAN host (host:port) that served the page, or "".
func LANHost() string {
	if !lanPage {
		return ""
	}
	return js.Global().Get("location").Get("host").String()
}

// invite is the one in the address (#i=...), taken out of it so a reload doesn't answer it again.
var invite = func() string {
	loc := js.Global().Get("location")
	s, ok := strings.CutPrefix(loc.Get("hash").String(), "#i=")
	if ok {
		js.Global().Get("history").Call("replaceState", nil, "", loc.Get("pathname").String())
	}
	return s
}()

// Invite is the server-free room's invite in the address the page was opened with, or "".
func Invite() string { return invite }

// replyFile is where index.html leaves a reply link's reply (#r=...) for the tab hosting the room.
// shortcut: one hosting tab per origin shares this slot; key replies by invite if multiple simultaneous rooms are needed.
const replyFile = "reply"

// Reply takes a joiner's reply to the server-free room hosted in this tab, left by a reply link opened in another
// tab, or "".
func Reply() (reply string) {
	var err error
	defer catch(&err) // storage disabled: no reply ever arrives
	ls := js.Global().Get("localStorage")
	v := ls.Call("getItem", storagePrefix+replyFile)
	if v.Type() != js.TypeString {
		return ""
	}
	ls.Call("removeItem", storagePrefix+replyFile)
	return v.String()
}

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

// Copy copies text without opening a share sheet. The selection fallback also works on local HTTP pages.
func Copy(text string, done func(bool)) {
	cb := js.Global().Get("navigator").Get("clipboard")
	if !cb.Truthy() {
		done(copySelection(text))
		return
	}
	var success, failure js.Func
	release := func() { success.Release(); failure.Release() }
	success = js.FuncOf(func(js.Value, []js.Value) any { release(); done(true); return nil })
	failure = js.FuncOf(func(js.Value, []js.Value) any { release(); done(copySelection(text)); return nil })
	defer func() {
		if recover() != nil {
			release()
			done(copySelection(text))
		}
	}()
	cb.Call("writeText", text).Call("then", success, failure)
}

func copySelection(text string) (copied bool) {
	var err error
	defer catch(&err)
	w := js.Global()
	doc := w.Get("document")
	active := doc.Get("activeElement")
	box := doc.Call("createElement", "textarea")
	box.Set("value", text)
	box.Call("setAttribute", "readonly", "")
	box.Get("style").Set("position", "fixed")
	box.Get("style").Set("left", "-9999px")
	doc.Get("body").Call("appendChild", box)
	defer box.Call("remove")
	box.Call("select")
	copied = doc.Call("execCommand", "copy").Bool()
	if active.Truthy() {
		active.Call("focus")
	}
	return copied
}
