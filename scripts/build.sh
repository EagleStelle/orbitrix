#!/usr/bin/env bash
# Build Orbitrix targets into dist/.
# Usage: scripts/build.sh [check|desktop|web|server|all]   (default: all)
# Cross-compile the desktop client with e.g. GOOS=windows scripts/build.sh desktop
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist

check() {
	unformatted=$(gofmt -l .)
	if [ -n "$unformatted" ]; then
		echo "gofmt needed:" >&2
		echo "$unformatted" >&2
		exit 1
	fi
	go vet ./...
	go test ./...
}

desktop() {
	go build -trimpath -o "$out/orbitrix$(go env GOEXE)" ./cmd/orbitrix
}

web() {
	mkdir -p "$out/web"
	GOOS=js GOARCH=wasm go build -trimpath -o "$out/web/game.wasm" ./cmd/orbitrix
	# wasm_exec.js must match the Go version that built game.wasm. -f because the
	# toolchain's copy is read-only, so a previous copy can't be opened for writing.
	cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/index.html "$out/web/"
}

server() {
	go build -trimpath -o "$out/orbitrix-server$(go env GOEXE)" ./cmd/orbitrix-server
}

case "${1:-all}" in
	check | desktop | web | server) "$1" ;;
	all) check; desktop; web; server ;;
	*) echo "usage: $0 [check|desktop|web|server|all]" >&2; exit 2 ;;
esac
