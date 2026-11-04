//go:build mage

// Orbitrix build targets. Run them with `go tool mage <target>` from any shell; `go tool mage -l` lists them. Output
// goes to dist/.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const out = "dist"

// ldflags strips debug info and stamps ORBITRIX_SERVER (https://your.site), the server native clients play online
// through; without it they offer no Online. The web build ignores the stamp and uses the server it was loaded from.
var ldflags = "-s -w -X orbitrix/client/platform.origin=" + os.Getenv("ORBITRIX_SERVER")

// All runs every target: Check, Desktop, Web, Android and Image.
func All() {
	mg.SerialDeps(Check, Desktop, Web, Android, Image)
}

// Check fails on unformatted files, then runs go vet and go test.
func Check() error {
	unformatted, err := sh.Output("gofmt", "-l", ".")
	if err != nil {
		return err
	}
	if unformatted != "" {
		return errors.New("gofmt needed:\n" + unformatted)
	}
	if err := sh.RunV("go", "vet", "./..."); err != nil {
		return err
	}
	return sh.RunV("go", "test", "./...")
}

// Desktop builds the Windows client, dist/orbitrix.exe, from any OS.
func Desktop() error {
	return sh.RunWithV(map[string]string{"GOOS": "windows", "GOARCH": "amd64"},
		"go", "build", "-trimpath", "-ldflags="+ldflags, "-o", filepath.Join(out, "orbitrix.exe"), "./cmd/orbitrix")
}

// Web builds the browser game into dist/web: game.wasm, its loader page and the landing page, under content-hashed
// names with .gz and .br copies.
func Web() error {
	dir := filepath.Join(out, "web")
	// webdist renames files after their hash, so old builds would pile up.
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := sh.RunWithV(map[string]string{"GOOS": "js", "GOARCH": "wasm"},
		"go", "build", "-trimpath", "-ldflags="+ldflags, "-o", filepath.Join(dir, "game.wasm"), "./cmd/orbitrix"); err != nil {
		return err
	}
	goroot, err := sh.Output("go", "env", "GOROOT")
	if err != nil {
		return err
	}
	// wasm_exec.js must match the Go version that built game.wasm.
	for _, f := range []string{filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"), "web/index.html", "web/landing.html", "client/fonts/Unbounded.ttf"} {
		if err := sh.Copy(filepath.Join(dir, filepath.Base(f)), f); err != nil {
			return err
		}
	}
	return sh.RunV("go", "run", "./cmd/webdist", dir)
}

// Android binds ./mobile into dist/orbitrix.aar (arm64 for phones, x86_64 for emulators and Chromebooks) and wraps it
// in dist/orbitrix.apk, signed with the key in ORBITRIX_KEYSTORE (password ORBITRIX_KEYSTORE_PASSWORD), or with the
// debug key without one. Needs the Android SDK with an NDK (ANDROID_HOME, or its default folder) and a JDK.
func Android() error {
	if os.Getenv("ORBITRIX_KEYSTORE") == "" {
		fmt.Println("android: ORBITRIX_KEYSTORE is unset, so the APK is signed with the debug key: don't publish it")
	}
	if os.Getenv("ANDROID_HOME") == "" {
		sdk := filepath.Join(os.Getenv("HOME"), "Android", "Sdk")
		if runtime.GOOS == "windows" {
			sdk = filepath.Join(os.Getenv("LOCALAPPDATA"), "Android", "Sdk")
		}
		os.Setenv("ANDROID_HOME", sdk)
	}
	// -checklinkname=0: pion's Android interface lookup (github.com/wlynxg/anet) links to a net internal that Go's
	// linker otherwise refuses.
	if err := sh.RunV("go", "tool", "ebitenmobile", "bind", "-target", "android/arm64,android/amd64", "-androidapi", "24",
		"-javapkg", "com.orbitrix", "-trimpath", "-ldflags="+ldflags+" -checklinkname=0",
		"-o", filepath.Join(out, "orbitrix.aar"), "./mobile"); err != nil {
		return err
	}
	gradlew := []string{"sh", "android/gradlew"} // through sh: committed from Windows, it lacks the executable bit
	if runtime.GOOS == "windows" {
		gradlew = []string{`android\gradlew.bat`}
	}
	if err := sh.RunV(gradlew[0], append(gradlew[1:], "-p", "android", "-q", "assembleRelease")...); err != nil {
		return err
	}
	return sh.Copy(filepath.Join(out, "orbitrix.apk"), "android/build/outputs/apk/release/orbitrix-release.apk")
}

// Image builds the eaglestelle/orbitrix-server Docker image (website and signaling server) from deploy/Dockerfile for
// linux/amd64 and linux/arm64, labelled with the commit it was built from and when, and loads it into Docker. Needs
// Docker with its containerd image store (Docker Desktop's default) and git.
func Image() error {
	rev, err := sh.Output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	args := []string{"buildx", "build", "--platform", "linux/amd64,linux/arm64", "-t", "eaglestelle/orbitrix-server",
		"-f", "deploy/Dockerfile", "--build-arg", "REVISION=" + rev,
		"--build-arg", "CREATED=" + time.Now().UTC().Format(time.RFC3339)}
	// CI only checks that both platforms build: loading them takes the containerd image store, which a runner may lack.
	if os.Getenv("CI") == "" {
		args = append(args, "--load")
	}
	return sh.RunV("docker", append(args, ".")...)
}
