package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStatic(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":   {Data: []byte("page")},
		"game.wasm":    {Data: []byte("plain")},
		"game.wasm.gz": {Data: []byte("gzipped")},
		"game.wasm.br": {Data: []byte("brotli")},
	}
	h := Static(fsys)
	for _, tc := range []struct {
		path, accept, body, encoding string
	}{
		{"/game.wasm", "gzip, deflate, br", "brotli", "br"},
		{"/game.wasm", "gzip", "gzipped", "gzip"},
		{"/game.wasm", "br;q=0, gzip;q=0.5", "gzipped", "gzip"},
		{"/game.wasm", "br;q=0.0, gzip;q=0", "plain", ""},
		{"/game.wasm", "", "plain", ""},
		{"/", "br", "page", ""}, // no compressed copy of the page
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if tc.accept != "" {
			req.Header.Set("Accept-Encoding", tc.accept)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Body.String(); got != tc.body || rec.Header().Get("Content-Encoding") != tc.encoding {
			t.Errorf("%s with %q: got %q encoded %q, want %q encoded %q", tc.path, tc.accept, got,
				rec.Header().Get("Content-Encoding"), tc.body, tc.encoding)
		}
		if tc.path == "/game.wasm" && rec.Header().Get("Content-Type") != "application/wasm" {
			t.Errorf("%s with %q: Content-Type %q", tc.path, tc.accept, rec.Header().Get("Content-Type"))
		}
	}
}
