package platform

import (
	"fmt"
	"io/fs"
	"syscall/js"
)

// CanExit reports whether the app may quit itself. A browser tab can't.
const CanExit = false

// The browser keeps each file as one localStorage entry under this prefix.
const storagePrefix = "orbitrix/"

// Load reads a saved file by name.
func Load(name string) (data []byte, err error) {
	defer catch(&err)
	v := js.Global().Get("localStorage").Call("getItem", storagePrefix+name)
	if v.IsNull() {
		return nil, fs.ErrNotExist
	}
	return []byte(v.String()), nil
}

// Save writes a file by name.
func Save(name string, data []byte) (err error) {
	defer catch(&err)
	js.Global().Get("localStorage").Call("setItem", storagePrefix+name, string(data))
	return nil
}

// catch turns a JavaScript exception (storage disabled, quota full) into an error.
func catch(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("localStorage: %v", r)
	}
}
