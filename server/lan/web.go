//go:build !js

package lan

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"orbitrix/server"
)

// embedded is the web game as cmd/webdist leaves it, copied in by magefile.go before each native build; a build made
// without Mage carries none, and its rooms serve no game. Only the .gz copies come: over plain http browsers take
// gzip, never brotli.
//
//go:embed all:web
var embedded embed.FS

var webBuild, _ = fs.Sub(embedded, "web") // only fails for an invalid name

// pageTag is index.html's opening tag, and lanTag the same tag marked as a LAN host's: the web game reads the mark
// (platform.LANHost) and joins this host instead of playing online.
const pageTag, lanTag = `<html lang="en">`, `<html lang="en" data-lan>`

// routeWeb serves browsers the web game from build, as the website does: the page at /, /play and /join/<code>,
// marked as this host's, and the build's content-hashed files. It reports whether build holds the game.
func routeWeb(mux *http.ServeMux, build fs.FS) bool {
	page, err := lanPage(build)
	if err != nil {
		return false
	}
	game := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	}
	mux.HandleFunc("GET /{$}", game)
	mux.HandleFunc("GET /play", game)
	mux.HandleFunc("GET /join/{code}", game) // the game reads the code from the address
	mux.Handle("GET /", server.Static(build))
	return true
}

// lanPage is build's index.html, marked as a LAN host's.
func lanPage(build fs.FS) ([]byte, error) {
	f, err := build.Open("index.html.gz")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("lan: index.html: %w", err)
	}
	b, err := io.ReadAll(z)
	if err != nil {
		return nil, fmt.Errorf("lan: index.html: %w", err)
	}
	if !bytes.Contains(b, []byte(pageTag)) {
		return nil, fmt.Errorf("lan: index.html lacks %s", pageTag)
	}
	return bytes.Replace(b, []byte(pageTag), []byte(lanTag), 1), nil
}
