package server

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// encodings are the precompressed siblings cmd/webdist writes, best first.
var encodings = [...]struct{ token, ext string }{{"br", ".br"}, {"gzip", ".gz"}}

// Static serves the web build in fsys. When the client accepts it, a file's precompressed .br or .gz sibling is sent
// instead, so nothing is compressed per request. Every response must be revalidated: wasm_exec.js and game.wasm have
// to come from the same build.
func Static(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Vary", "Accept-Encoding")
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		for _, enc := range encodings {
			if accepts(r, enc.token) && serveEncoded(w, r, fsys, name, enc.token, enc.ext) {
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

// serveEncoded sends name's precompressed sibling, if there is one.
func serveEncoded(w http.ResponseWriter, r *http.Request, fsys fs.FS, name, token, ext string) bool {
	f, err := fsys.Open(name + ext)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	rs, seekable := f.(io.ReadSeeker)
	if err != nil || !seekable || st.IsDir() {
		return false
	}
	ctype := mime.TypeByExtension(path.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Encoding", token)
	http.ServeContent(w, r, name, st.ModTime(), rs)
	return true
}

// accepts reports whether the request's Accept-Encoding lists enc with a quality above zero.
func accepts(r *http.Request, enc string) bool {
	for _, part := range strings.Split(strings.Join(r.Header.Values("Accept-Encoding"), ","), ",") {
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), enc) {
			continue
		}
		q, isQ := strings.CutPrefix(strings.TrimSpace(params), "q=")
		return !isQ || strings.Trim(q, "0.") != "" // q=0, q=0.0 and the like refuse it
	}
	return false
}
