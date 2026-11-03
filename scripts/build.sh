#!/usr/bin/env bash
# Build Orbitrix targets into dist/.
# Usage: scripts/build.sh [check|desktop|web|server|image|android|all]   (default: all; image and android are separate)
# Cross-compile the desktop client with e.g. GOOS=windows scripts/build.sh desktop
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist
# ORBITRIX_SERVER (https://your.site) is the server native clients play online through; without it they offer no Online.
stamp="-X orbitrix/client/platform.origin=${ORBITRIX_SERVER:-}"

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
	go build -trimpath -ldflags="$stamp" -o "$out/orbitrix$(go env GOEXE)" ./cmd/orbitrix
}

web() {
	rm -rf "$out/web" # webdist renames files after their hash, so old builds would pile up
	mkdir -p "$out/web"
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$out/web/game.wasm" ./cmd/orbitrix
	# wasm_exec.js must match the Go version that built game.wasm. -f because the
	# toolchain's copy is read-only, so a previous copy can't be opened for writing.
	cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/index.html web/landing.html client/fonts/Unbounded.ttf "$out/web/"
	go run ./cmd/webdist "$out/web" # loading progress size, hashed names, .gz and .br copies
}

server() {
	go build -trimpath -o "$out/orbitrix-server$(go env GOEXE)" ./cmd/orbitrix-server
}

# image packages the web build and a Linux server binary as the orbitrix-server Docker image. Nothing is compiled in
# Docker.
# shortcut: linux/amd64 only; add an architecture option if the server is ARM.
image() {
	web
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out/orbitrix-server-linux" ./cmd/orbitrix-server
	docker build --platform linux/amd64 -t orbitrix-server -f deploy/Dockerfile "$out"
}

# android binds ./mobile into an AAR and wraps it in an APK. Needs the Android SDK with an NDK and a JDK.
android() {
	if [ -z "${ANDROID_HOME:-}" ]; then
		if [ -n "${LOCALAPPDATA:-}" ]; then
			ANDROID_HOME="$LOCALAPPDATA/Android/Sdk"
		else
			ANDROID_HOME="$HOME/Android/Sdk"
		fi
		export ANDROID_HOME
	fi
	# arm64 for phones, x86_64 for emulators and Chromebooks. -checklinkname=0: pion's Android interface lookup
	# (github.com/wlynxg/anet) links to a net internal that Go's linker otherwise refuses.
	go tool ebitenmobile bind -target android/arm64,android/amd64 -androidapi 24 -javapkg com.orbitrix \
		-trimpath -ldflags="-s -w -checklinkname=0 $stamp" -o "$out/orbitrix.aar" ./mobile
	(cd android && sh ./gradlew -q assembleRelease)
	cp -f android/build/outputs/apk/release/orbitrix-release.apk "$out/orbitrix.apk"
}

case "${1:-all}" in
	check | desktop | web | server | image | android) "$1" ;;
	all) check; desktop; web; server ;;
	*) echo "usage: $0 [check|desktop|web|server|image|android|all]" >&2; exit 2 ;;
esac
