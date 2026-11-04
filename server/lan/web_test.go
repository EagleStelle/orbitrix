//go:build !js

package lan

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func gz(s string) []byte {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	_, _ = z.Write([]byte(s))
	_ = z.Close()
	return b.Bytes()
}

// TestWeb serves a build laid out as magefile.go copies it in: the page, marked as a LAN host's, at the game's
// addresses, and the hashed files as the gzip they are kept in.
func TestWeb(t *testing.T) {
	mux := http.NewServeMux()
	if routeWeb(mux, fstest.MapFS{"game.0123abcd.wasm.gz": {Data: gz("wasm")}}) {
		t.Fatal("a build without index.html must serve no game")
	}
	build := fstest.MapFS{
		"index.html.gz":         {Data: gz(`<!DOCTYPE html>` + "\n" + pageTag + "\n<title>Orbitrix</title>")},
		"game.0123abcd.wasm.gz": {Data: gz("wasm")},
	}
	if !routeWeb(mux, build) {
		t.Fatal("the build's game isn't served")
	}
	for _, path := range []string{"/", "/play", "/join/ABCDEF"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, lanTag) || strings.Contains(body, pageTag) {
			t.Errorf("%s: %d %q", path, rec.Code, body)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/game.0123abcd.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "gzip" || !bytes.Equal(rec.Body.Bytes(), gz("wasm")) {
		t.Errorf("game.wasm: %d %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/landing.html", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("landing.html: %d", rec.Code)
	}
}

// TestPageTag keeps the loader page's opening tag the one the LAN host marks.
func TestPageTag(t *testing.T) {
	b, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(pageTag)) {
		t.Fatalf("web/index.html lacks %s", pageTag)
	}
}

// A native build must carry every file referenced by its bundled page.
func TestEmbeddedWeb(t *testing.T) {
	if _, err := fs.Stat(webBuild, "index.html.gz"); errors.Is(err, fs.ErrNotExist) {
		t.Skip("run a native Mage build to bundle the web game")
	}
	page, err := lanPage(webBuild)
	if err != nil {
		t.Fatal(err)
	}
	assets := regexp.MustCompile(`"/([^"/]+\.(wasm|js|ttf|svg))"`).FindAllSubmatch(page, -1)
	if len(assets) != 3 {
		t.Fatalf("expected three bundled assets, got %d", len(assets))
	}
	for _, asset := range assets {
		if _, err := fs.Stat(webBuild, string(asset[1])+".gz"); err != nil {
			t.Fatal(err)
		}
	}
}
