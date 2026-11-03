// Command orbitrix-server serves the website and the signaling service; it carries no game traffic.
//
// Configuration comes from the environment: LISTEN (default :1212), PUBLIC_ORIGIN, HOST_PASSWORD, CF_TURN_KEY_ID
// and CF_TURN_API_TOKEN. Without both CF_TURN_* values, iceServers is STUN only.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"orbitrix/server"
	"orbitrix/server/signal"
)

func main() {
	web := flag.String("web", "", "serve the web build from this folder (dist/web after a web build)")
	downloads := flag.String("downloads", "", "offer orbitrix.exe and orbitrix.apk from this folder")
	flag.Parse()

	cfg := signal.Config{Password: os.Getenv("HOST_PASSWORD"), Origin: os.Getenv("PUBLIC_ORIGIN")}
	switch key, token := os.Getenv("CF_TURN_KEY_ID"), os.Getenv("CF_TURN_API_TOKEN"); {
	case key != "" && token != "":
		cfg.TURN = &signal.Cloudflare{KeyID: key, Token: token}
	case key != "" || token != "":
		slog.Warn("only one of CF_TURN_KEY_ID and CF_TURN_API_TOKEN is set: iceServers is STUN only")
	}
	if cfg.Origin == "" {
		slog.Warn("PUBLIC_ORIGIN is unset: browsers may only open /signal from the host they loaded")
	}
	site := server.Site{Downloads: *downloads, Signal: signal.New(cfg)}
	if *web != "" {
		site.Web = os.DirFS(*web)
	}
	h, err := site.Handler()
	if err != nil {
		slog.Error("web build", "err", err)
		os.Exit(1)
	}

	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = ":1212"
	}
	// No read or write timeouts: /signal sockets stay open for a whole session. ReadHeaderTimeout stops slow loris.
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute,
		MaxHeaderBytes: 16 << 10}
	slog.Info("listening", "addr", addr, "web", *web, "downloads", *downloads)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
