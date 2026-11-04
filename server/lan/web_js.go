//go:build js

package lan

import (
	"io/fs"
	"net/http"
)

// A browser never hosts on the LAN, so its build carries no web game to serve.
var webBuild fs.FS

func routeWeb(*http.ServeMux, fs.FS) bool { return false }
