package platform

import (
	"fmt"
	"strings"
	"syscall/js"

	"orbitrix/core"
)

// File controls only bridge the system picker and download facility. The game
// continues to draw its entire import/export interface in Ebitengine.
var PickMapFile = pickMapFile
var ExportMapFile = exportMapFile

var mapImport = func() string {
	loc := js.Global().Get("location")
	fragment := loc.Get("hash").String()
	code, ok := strings.CutPrefix(fragment, "#map=")
	if !ok {
		return ""
	}
	// Keep even malformed codes for the import preview to explain the problem.
	js.Global().Get("history").Call("replaceState", nil, "", loc.Get("pathname").String()+loc.Get("search").String())
	return code
}()

// MapImport is stable across calls, like Invite: the URL is cleared only once.
func MapImport() string { return mapImport }

func pickMapFile(done func([]byte, error)) (opened bool) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".orbitrixmap,application/octet-stream")
	input.Get("style").Set("display", "none")
	doc.Get("body").Call("appendChild", input)
	var callbacks []js.Func
	var reader js.Value
	finished := false
	finish := func(data []byte, err error) {
		if finished {
			return
		}
		finished = true
		input.Set("onchange", nil)
		input.Set("oncancel", nil)
		if reader.Truthy() {
			reader.Set("onload", nil)
			reader.Set("onerror", nil)
			reader.Set("onabort", nil)
		}
		input.Call("remove")
		for _, fn := range callbacks {
			fn.Release()
		}
		done(data, err)
	}
	defer func() {
		if recover() != nil {
			finish(nil, fmt.Errorf("import map: browser file picker is unavailable"))
			opened = false
		}
	}()
	bind := func(target js.Value, event string, callback func(js.Value, []js.Value) any) {
		fn := js.FuncOf(callback)
		callbacks = append(callbacks, fn)
		target.Set(event, fn)
	}
	bind(input, "oncancel", func(js.Value, []js.Value) any {
		finish(nil, nil)
		return nil
	})
	bind(input, "onchange", func(js.Value, []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			finish(nil, nil)
			return nil
		}
		file := files.Index(0)
		if file.Get("size").Float() > core.MaxMapBytes {
			finish(nil, fmt.Errorf("import map: file is too large"))
			return nil
		}
		reader = js.Global().Get("FileReader").New()
		bind(reader, "onload", func(js.Value, []js.Value) any {
			array := js.Global().Get("Uint8Array").New(reader.Get("result"))
			if array.Length() > core.MaxMapBytes {
				finish(nil, fmt.Errorf("import map: file is too large"))
				return nil
			}
			data := make([]byte, array.Length())
			js.CopyBytesToGo(data, array)
			finish(data, nil)
			return nil
		})
		bind(reader, "onerror", func(js.Value, []js.Value) any {
			finish(nil, fmt.Errorf("import map: browser could not read the file"))
			return nil
		})
		bind(reader, "onabort", func(js.Value, []js.Value) any {
			finish(nil, nil)
			return nil
		})
		reader.Call("readAsArrayBuffer", file)
		return nil
	})
	input.Call("click")
	return true
}

func exportMapFile(name string, data []byte, done func(string, error)) {
	var err error
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("export map: browser download is unavailable")
		}
		done("Downloads", err)
	}()
	if len(data) > core.MaxMapBytes {
		err = fmt.Errorf("export map: file is too large")
		return
	}
	name = strings.TrimSpace(strings.TrimSuffix(name, ".orbitrixmap"))
	if name == "" {
		name = "Untitled map"
	}
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\x00", "").Replace(name) + ".orbitrixmap"
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	parts := js.Global().Get("Array").New()
	parts.Call("push", array)
	options := js.Global().Get("Object").New()
	options.Set("type", "application/octet-stream")
	blob := js.Global().Get("Blob").New(parts, options)
	urls := js.Global().Get("URL")
	url := urls.Call("createObjectURL", blob).String()
	anchor := js.Global().Get("document").Call("createElement", "a")
	anchor.Set("href", url)
	anchor.Set("download", name)
	anchor.Get("style").Set("display", "none")
	js.Global().Get("document").Get("body").Call("appendChild", anchor)
	anchor.Call("click")
	anchor.Call("remove")
	// Revocation in a later task lets the browser begin the download first.
	var revoke js.Func
	revoke = js.FuncOf(func(js.Value, []js.Value) any {
		urls.Call("revokeObjectURL", url)
		revoke.Release()
		return nil
	})
	js.Global().Call("setTimeout", revoke, 1000)
}

func ReadMapFile(string) ([]byte, error) {
	return nil, fmt.Errorf("import map: choose a file using Import file")
}
