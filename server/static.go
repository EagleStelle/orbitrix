package server

import (
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// encodings are the precompressed siblings cmd/webdist writes, best first.
var encodings = [...]struct{ token, ext string }{{"br", ".br"}, {"gzip", ".gz"}}

// downloads are the files /download/ offers, by name, and the landing page field that shows each.
var downloads = map[string]string{"orbitrix.exe": "Windows", "orbitrix.apk": "Android"}

// Site is the website. Edge caching depends on its headers: the build's files have content-hashed names and never
// change, pages are revalidated on every load, and downloads keep their names, so they are cached for 5 minutes.
type Site struct {
	Web       fs.FS        // the web build (dist/web); nil serves only /healthz and /signal
	Downloads string       // folder holding orbitrix.exe and orbitrix.apk; "" offers neither
	Signal    http.Handler // the signaling service; nil leaves /signal out
}

// Handler routes / (landing), /play and /join/<code> (the game), /download/<file>, /signal, /healthz and the
// build's files.
func (s Site) Handler() (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	if s.Signal != nil {
		mux.Handle("GET /signal", s.Signal)
	}
	if s.Web == nil {
		return mux, nil
	}
	landing, err := template.ParseFS(s.Web, "landing.html")
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		offer := map[string]bool{}
		for name, field := range downloads {
			offer[field] = s.has(name)
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = landing.Execute(w, offer) // only fails if the client went away
	})
	game := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		serve(w, r, s.Web, "index.html")
	}
	mux.HandleFunc("GET /play", game)
	mux.HandleFunc("GET /join/{code}", game) // the game reads the code from the address
	mux.HandleFunc("GET /download/{file}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		if _, ok := downloads[name]; !ok || !s.has(name) {
			http.NotFound(w, r)
			return
		}
		// Cloudflare stores .exe and .apk by default, unless told no-cache: the edge serves a download flood, and a new
		// build shows within 5 minutes.
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.ServeFile(w, r, filepath.Join(s.Downloads, name))
	})
	mux.Handle("GET /", Static(s.Web))
	return mux, nil
}

func (s Site) has(name string) bool {
	if s.Downloads == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(s.Downloads, name))
	return err == nil && st.Mode().IsRegular()
}

// Static serves the web build's files, which cmd/webdist names by content hash, so they are cached for good. Pages
// are not served here: Site renders and serves them with their own headers.
func Static(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || path.Ext(name) == ".html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		serve(w, r, fsys, name)
	})
}

// serve sends one file, as its precompressed .br or .gz sibling when the client accepts one, so nothing is compressed
// per request.
func serve(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	w.Header().Add("Vary", "Accept-Encoding")
	for _, enc := range encodings {
		if accepts(r, enc.token) && serveEncoded(w, r, fsys, name, enc.token, enc.ext) {
			return
		}
	}
	http.ServeFileFS(w, r, fsys, name)
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
