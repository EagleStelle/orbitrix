package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// build is laid out like cmd/webdist's output: served files only as .gz and .br.
var build = fstest.MapFS{
	"index.html.gz":         {Data: gz("game page")},
	"index.html.br":         {Data: []byte("game page, brotli")},
	"landing.html":          {Data: []byte(`<p>{{if .Windows}}exe{{end}}{{if .Android}}apk{{end}}</p>`)},
	"game.0123abcd.wasm.gz": {Data: gz("plain")},
	"game.0123abcd.wasm.br": {Data: []byte("brotli")},
}

func gz(s string) []byte {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	_, _ = z.Write([]byte(s))
	_ = z.Close()
	return b.Bytes()
}

func site(t *testing.T, downloads string) http.Handler {
	t.Helper()
	h, err := Site{Web: build, Downloads: downloads}.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(h http.Handler, path, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if accept != "" {
		req.Header.Set("Accept-Encoding", accept)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStatic(t *testing.T) {
	h := site(t, "")
	for _, tc := range []struct {
		path, accept, body, encoding string
	}{
		{"/game.0123abcd.wasm", "gzip, deflate, br", "brotli", "br"},
		{"/game.0123abcd.wasm", "gzip", string(gz("plain")), "gzip"},
		{"/game.0123abcd.wasm", "br;q=0, gzip;q=0.5", string(gz("plain")), "gzip"},
		{"/game.0123abcd.wasm", "br;q=0.0, gzip;q=0", "plain", ""},
		{"/game.0123abcd.wasm", "", "plain", ""},
	} {
		rec := get(h, tc.path, tc.accept)
		if got := rec.Body.String(); got != tc.body || rec.Header().Get("Content-Encoding") != tc.encoding {
			t.Errorf("%s with %q: got %q encoded %q, want %q encoded %q", tc.path, tc.accept, got,
				rec.Header().Get("Content-Encoding"), tc.body, tc.encoding)
		}
		if rec.Header().Get("Content-Type") != "application/wasm" ||
			!strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("%s with %q: headers %v", tc.path, tc.accept, rec.Header())
		}
	}
	// The scratch image has no /etc/mime.types, so the font's type must come from the server itself.
	if got := contentType("Unbounded.0123abcd.ttf"); got != "font/ttf" {
		t.Errorf("ttf Content-Type %q, want font/ttf", got)
	}
}

func TestPages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "orbitrix.apk"), []byte("apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := site(t, dir)
	for _, tc := range []struct {
		path, accept string
		status       int
		body         string
	}{
		{"/", "", 200, "<p>apk</p>"}, // only the download that exists is offered
		{"/play", "br", 200, "game page, brotli"},
		{"/join/K7M2QX", "", 200, "game page"},
		{"/download/orbitrix.apk", "", 200, "apk"},
		{"/download/orbitrix.exe", "", 404, ""},
		{"/download/secret.txt", "", 404, ""},
		{"/index.html", "", 404, ""},
		{"/landing.html", "", 404, ""},
		{"/healthz", "", 200, "ok\n"},
	} {
		rec := get(h, tc.path, tc.accept)
		if rec.Code != tc.status || tc.status == 200 && rec.Body.String() != tc.body {
			t.Errorf("%s: %d %q, want %d %q", tc.path, rec.Code, rec.Body.String(), tc.status, tc.body)
		}
		want := "no-cache"
		if strings.HasPrefix(tc.path, "/download/") {
			want = "public, max-age=300" // the edge serves downloads, which a no-cache would stop
		}
		if tc.status == 200 && tc.path != "/healthz" && rec.Header().Get("Cache-Control") != want {
			t.Errorf("%s: Cache-Control %q, want %q", tc.path, rec.Header().Get("Cache-Control"), want)
		}
	}
}
