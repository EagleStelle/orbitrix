// Command orbitrix-server serves the website and the signaling service; it carries no game traffic. Until Phase 5 it
// answers /healthz and, with -web, serves the web build.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"orbitrix/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	web := flag.String("web", "", "serve the web build from this folder (dist/web after a web build)")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	if *web != "" {
		mux.Handle("GET /", server.Static(os.DirFS(*web)))
	}

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", *addr, "web", *web)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
