//go:build mage

// Orbitrix build targets. Run them with `go tool mage <target>` from any shell; `go tool mage -l` lists them. Output
// goes to dist/.
package main

import (
	"archive/zip"
	"bytes"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"orbitrix/internal/appicon"
)

const out = "dist"

// version is the release version, matching versionName in android/build.gradle.kts. The release workflow refuses a
// tag that differs from either.
const version = "1.0.0"

// ldflags strips debug info and stamps ORBITRIX_SERVER (https://your.site), the server native clients play online
// through; without it they offer no Online. The web build ignores the stamp and uses the server it was loaded from.
var ldflags = "-s -w -X orbitrix/client/platform.origin=" + os.Getenv("ORBITRIX_SERVER")

// All runs every target: Check, Desktop, Mac, Linux, Web, Android and Image.
func All() {
	mg.SerialDeps(Check, Desktop, Mac, Linux, Web, Android, Image)
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
	icon, err := appicon.WindowsResource()
	if err != nil {
		return err
	}
	const resource = "cmd/orbitrix/icon_windows_amd64.syso"
	if err := os.WriteFile(resource, icon, 0o644); err != nil {
		return err
	}
	defer os.Remove(resource)
	return sh.RunWithV(map[string]string{"GOOS": "windows", "GOARCH": "amd64"},
		"go", "build", "-trimpath", "-ldflags="+ldflags, "-o", filepath.Join(out, "orbitrix.exe"), "./cmd/orbitrix")
}

// infoPlist describes Orbitrix.app to macOS. The local network line is the prompt macOS 15 and later show before Same
// Wi-Fi play.
const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>orbitrix</string>
	<key>CFBundleIdentifier</key><string>com.orbitrix</string>
	<key>CFBundleName</key><string>Orbitrix</string>
	<key>CFBundleIconFile</key><string>orbitrix.icns</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>` + version + `</string>
	<key>CFBundleVersion</key><string>` + version + `</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSLocalNetworkUsageDescription</key><string>Orbitrix finds and hosts Same Wi-Fi rooms.</string>
</dict>
</plist>
`

// Mac builds dist/orbitrix-mac.zip from any OS: Orbitrix.app, one binary for Apple silicon and Intel Macs. It carries
// only the Go linker's ad-hoc signature, so macOS makes the player allow it once in Privacy & Security.
func Mac() error {
	var slices [][]byte
	for _, arch := range []string{"arm64", "amd64"} {
		bin, err := desktopBinary("darwin", arch)
		if err != nil {
			return err
		}
		slices = append(slices, bin)
	}
	fat, err := universal(slices)
	if err != nil {
		return err
	}
	if f, err := macho.NewFatFile(bytes.NewReader(fat)); err != nil || len(f.Arches) != len(slices) {
		return fmt.Errorf("mac: universal binary doesn't read back: %v", err)
	}
	icon, err := appicon.ICNS()
	if err != nil {
		return err
	}
	return zipFiles(filepath.Join(out, "orbitrix-mac.zip"), map[string][]byte{
		"Orbitrix.app/Contents/Info.plist":              []byte(infoPlist),
		"Orbitrix.app/Contents/MacOS/orbitrix":          fat,
		"Orbitrix.app/Contents/Resources/orbitrix.icns": icon,
	})
}

// Linux builds dist/orbitrix-linux.zip (amd64) from any OS. Ebitengine loads X11 (or XWayland), OpenGL and ALSA or
// PulseAudio at run time, so the binary needs no cgo and runs on any desktop distribution.
func Linux() error {
	bin, err := desktopBinary("linux", "amd64")
	if err != nil {
		return err
	}
	icon, err := appicon.PNG(256)
	if err != nil {
		return err
	}
	return zipFiles(filepath.Join(out, "orbitrix-linux.zip"), map[string][]byte{
		"orbitrix": bin, "orbitrix.png": icon,
		"orbitrix.desktop": []byte("[Desktop Entry]\nType=Application\nName=Orbitrix\nExec=orbitrix\nIcon=orbitrix\nTerminal=false\nCategories=Game;BoardGame;\nStartupWMClass=Orbitrix\n"),
	})
}

// desktopBinary builds the desktop client for goos/goarch without cgo and returns it.
func desktopBinary(goos, goarch string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "orbitrix")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, "orbitrix")
	if err := sh.RunWithV(map[string]string{"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"},
		"go", "build", "-trimpath", "-ldflags="+ldflags, "-o", bin, "./cmd/orbitrix"); err != nil {
		return nil, err
	}
	return os.ReadFile(bin)
}

// universal joins Mach-O binaries into one fat binary, as Apple's lipo does: a big-endian header listing each
// architecture's slice, then the slices, each at a 16 KiB boundary.
func universal(bins [][]byte) ([]byte, error) {
	const align = 14
	fat := make([]byte, 1<<align)
	binary.BigEndian.PutUint32(fat, macho.MagicFat)
	binary.BigEndian.PutUint32(fat[4:], uint32(len(bins)))
	for i, b := range bins {
		f, err := macho.NewFile(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		fat = append(fat, make([]byte, -len(fat)&(1<<align-1))...)
		for j, v := range []uint32{uint32(f.Cpu), f.SubCpu, uint32(len(fat)), uint32(len(b)), align} {
			binary.BigEndian.PutUint32(fat[8+20*i+4*j:], v)
		}
		fat = append(fat, b...)
	}
	return fat, nil
}

// zipFiles writes the files to a zip, the executable one (the one with no extension) marked so, which macOS and Linux
// keep when they unpack it.
func zipFiles(name string, files map[string][]byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	for path, data := range files {
		h := &zip.FileHeader{Name: path, Method: zip.Deflate, Modified: time.Now()}
		h.SetMode(0o644)
		if filepath.Ext(path) == "" {
			h.SetMode(0o755)
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	return f.Close()
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
	// gomobile writes the .aar without creating its folder, which a fresh checkout lacks.
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
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
// linux/amd64 and linux/arm64, tagged latest and with the version, labelled with the commit it was built from and
// when, and loads it into Docker. Needs Docker with its containerd image store (Docker Desktop's default) and git.
func Image() error {
	// CI only checks that both platforms build: loading them takes the containerd image store, which a runner may lack.
	if os.Getenv("CI") != "" {
		return image()
	}
	return image("--load")
}

// Push builds the image as Image does and pushes both platforms to Docker Hub. Needs `docker login` first and a
// buildx builder that pushes (the docker-container driver, or Docker's containerd image store).
func Push() error {
	return image("--push")
}

func image(output ...string) error {
	rev, err := sh.Output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	const name = "eaglestelle/orbitrix-server"
	args := append([]string{"buildx", "build", "--platform", "linux/amd64,linux/arm64",
		"-t", name + ":latest", "-t", name + ":" + version, "-f", "deploy/Dockerfile",
		"--build-arg", "VERSION=" + version, "--build-arg", "REVISION=" + rev,
		"--build-arg", "CREATED=" + time.Now().UTC().Format(time.RFC3339)}, output...)
	return sh.RunV("docker", append(args, ".")...)
}
