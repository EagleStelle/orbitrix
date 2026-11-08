package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

func rawBuild(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"game.wasm":     "\x00asm" + strings.Repeat("game", 100),
		"wasm_exec.js":  "// wasm loader",
		"Unbounded.ttf": "font data",
		"index.html":    `<html lang="en"><script>/*WASM_SIZE*/0</script><a href="game.wasm"></a><script src="wasm_exec.js"></script><link href="Unbounded.ttf"></html>`,
		"landing.html":  `<html><link href="favicon.svg"></html>`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func unpack(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reader io.Reader
	if filepath.Ext(path) == ".gz" {
		z, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer z.Close()
		reader = z
	} else {
		reader = brotli.NewReader(bytes.NewReader(data))
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

func TestBuildReusesUnchangedFiles(t *testing.T) {
	for _, gzipOnly := range []bool{false, true} {
		t.Run(strconv.FormatBool(gzipOnly), func(t *testing.T) {
			src := rawBuild(t)
			dir := filepath.Join(t.TempDir(), "output")
			if err := run(src, dir, gzipOnly); err != nil {
				t.Fatal(err)
			}
			wasm, err := os.ReadFile(filepath.Join(src, "game.wasm"))
			if err != nil {
				t.Fatal(err)
			}
			oldGame, _ := filepath.Glob(filepath.Join(dir, "game.*.wasm.gz"))
			if len(oldGame) != 1 || !bytes.Equal(unpack(t, oldGame[0]), wasm) {
				t.Fatal("packaged game differs from source")
			}
			page := unpack(t, filepath.Join(dir, "index.html.gz"))
			gameName := strings.TrimSuffix(filepath.Base(oldGame[0]), ".gz")
			if !bytes.Contains(page, []byte(`"/`+gameName+`"`)) || !bytes.Contains(page, []byte("/*WASM_SIZE*/"+strconv.Itoa(len(wasm)))) {
				t.Fatalf("loader has wrong asset name or size: %s", page)
			}
			brFiles, _ := filepath.Glob(filepath.Join(dir, "*.br"))
			if gzipOnly && len(brFiles) != 0 || !gzipOnly && len(brFiles) != 5 {
				t.Fatalf("Brotli files = %d, gzip-only = %v", len(brFiles), gzipOnly)
			}
			if !gzipOnly && !bytes.Equal(unpack(t, strings.TrimSuffix(oldGame[0], ".gz")+".br"), wasm) {
				t.Fatal("Brotli game differs from source")
			}
			stamp := time.Unix(1700000000, 0)
			files, _ := filepath.Glob(filepath.Join(dir, "*"))
			for _, file := range files {
				if err := os.Chtimes(file, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			if err := run(src, dir, gzipOnly); err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				if filepath.Base(file) == "landing.html" {
					continue
				}
				info, err := os.Stat(file)
				if err != nil || !info.ModTime().Equal(stamp) {
					t.Fatalf("unchanged output was rewritten: %s (%v)", file, err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			// Same size and timestamp: only content can reveal the change.
			wasm[len(wasm)-1] ^= 1
			if err := os.WriteFile(filepath.Join(src, "game.wasm"), wasm, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(filepath.Join(src, "game.wasm"), stamp, stamp); err != nil {
				t.Fatal(err)
			}
			if err := run(src, dir, gzipOnly); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(oldGame[0]); !os.IsNotExist(err) {
				t.Fatalf("stale game was kept: %v", err)
			}
			game, _ := filepath.Glob(filepath.Join(dir, "game.*.wasm.gz"))
			if len(game) != 1 || !bytes.Equal(unpack(t, game[0]), wasm) {
				t.Fatal("changed game was not rebuilt")
			}
			fonts, _ := filepath.Glob(filepath.Join(dir, "Unbounded.*.ttf.gz"))
			info, err := os.Stat(fonts[0])
			if err != nil || !info.ModTime().Equal(stamp) {
				t.Fatal("unchanged font was recompressed")
			}
			page = unpack(t, filepath.Join(dir, "index.html.gz"))
			if !bytes.Contains(page, []byte(`"/`+strings.TrimSuffix(filepath.Base(game[0]), ".gz")+`"`)) {
				t.Fatal("loader still points at the stale game")
			}
			if _, err := os.Stat(filepath.Join(dir, ".gitkeep")); err != nil {
				t.Fatal("cleanup removed an unrelated file")
			}
			if err := os.Remove(game[0]); err != nil {
				t.Fatal(err)
			}
			if err := run(src, dir, gzipOnly); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(unpack(t, game[0]), wasm) {
				t.Fatal("missing cached output was not recreated")
			}
		})
	}
}

func TestPackagingModesAndCacheRecovery(t *testing.T) {
	src, dir := rawBuild(t), t.TempDir()
	for _, gzipOnly := range []bool{false, true, false} {
		if err := run(src, dir, gzipOnly); err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.br"))
		if gzipOnly && len(files) != 0 || !gzipOnly && len(files) != 5 {
			t.Fatalf("wrong files after mode change: %v", files)
		}
	}
	data, err := os.ReadFile(dir + ".cache.json")
	if err != nil {
		t.Fatal(err)
	}
	cache := map[string]string{}
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatal(err)
	}
	cache["version"] = "old settings"
	obsolete, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []string{string(obsolete), `{"version":`} {
		if err := os.WriteFile(dir+".cache.json", []byte(metadata), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(1700000000, 0)
		path := filepath.Join(dir, "index.html.gz")
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if err := run(src, dir, true); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.ModTime().Equal(stamp) {
			t.Fatal("obsolete or damaged metadata was reused")
		}
		data, err := os.ReadFile(dir + ".cache.json")
		if err != nil || !json.Valid(data) {
			t.Fatal("cache metadata was not repaired")
		}
	}
	before := unpack(t, filepath.Join(dir, "index.html.gz"))
	if err := os.WriteFile(filepath.Join(src, "index.html"), []byte("missing size marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(src, dir, true); err == nil {
		t.Fatal("invalid loader was accepted")
	}
	if !bytes.Equal(unpack(t, filepath.Join(dir, "index.html.gz")), before) {
		t.Fatal("invalid input damaged the previous build")
	}
}

func TestInPlacePackaging(t *testing.T) {
	dir := rawBuild(t)
	if err := run(dir, dir, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"index.html"}, hashed...) {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("raw input %s was kept: %v", name, err)
		}
	}
	if len(unpack(t, filepath.Join(dir, "index.html.gz"))) == 0 {
		t.Fatal("empty packaged loader")
	}
}
