// Command webdist finishes a web build in a folder (default dist/web). It writes game.wasm's size into index.html for
// the loading dots, draws the logo as favicon.svg, renames the build's files after their content hash so the edge can cache them for good, points
// the pages at the new names, and saves a gzip and a brotli copy of each served file beside it, so a server sends
// them as they are. landing.html stays uncompressed: the server renders it as a template.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
)

// sizeMark is where index.html takes game.wasm's size.
const sizeMark = "/*WASM_SIZE*/0"

// hashed are renamed after their content. The pages name each one in double quotes, and get "/<new name>".
var hashed = []string{"game.wasm", "wasm_exec.js", "Unbounded.ttf", "favicon.svg"}

var pages = []string{"index.html", "landing.html"}

// encoders write each file's compressed copies, by extension, at their best (slowest) levels.
var encoders = map[string]func(io.Writer) io.WriteCloser{
	".gz": func(w io.Writer) io.WriteCloser {
		z, _ := gzip.NewWriterLevel(w, gzip.BestCompression) // only fails for an invalid level
		return z
	},
	".br": func(w io.Writer) io.WriteCloser { return brotli.NewWriterLevel(w, brotli.BestCompression) },
}

func main() {
	dir := "dist/web"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(dir); err != nil {
		log.Fatal(err)
	}
}

func run(dir string) error {
	wasm, err := os.Stat(filepath.Join(dir, "game.wasm"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "favicon.svg"), favicon(), 0o644); err != nil {
		return err
	}
	text := map[string][]byte{}
	for _, name := range pages {
		if text[name], err = os.ReadFile(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	if !bytes.Contains(text["index.html"], []byte(sizeMark)) {
		return fmt.Errorf("index.html: no %s to fill in", sizeMark)
	}
	text["index.html"] = bytes.Replace(text["index.html"], []byte(sizeMark),
		[]byte("/*WASM_SIZE*/"+strconv.FormatInt(wasm.Size(), 10)), 1)

	served := []string{"index.html"}
	for _, name := range hashed {
		renamed, err := rename(dir, name)
		if err != nil {
			return err
		}
		ref, found := []byte(`"`+name+`"`), false
		for page, b := range text {
			found = found || bytes.Contains(b, ref)
			text[page] = bytes.ReplaceAll(b, ref, []byte(`"/`+renamed+`"`))
		}
		if !found {
			return fmt.Errorf("no page refers to %q", name)
		}
		served = append(served, renamed)
	}
	for page, b := range text {
		if err := os.WriteFile(filepath.Join(dir, page), b, 0o644); err != nil {
			return err
		}
	}
	for _, name := range served {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		for ext, enc := range encoders {
			if err := compress(filepath.Join(dir, name+ext), data, enc); err != nil {
				return err
			}
		}
	}
	return nil
}

// rename moves name to name.<first 12 hex digits of its SHA-256>.ext and returns the new name.
func rename(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	ext := filepath.Ext(name)
	renamed := strings.TrimSuffix(name, ext) + "." + hex.EncodeToString(sum[:6]) + ext
	return renamed, os.Rename(filepath.Join(dir, name), filepath.Join(dir, renamed))
}

func compress(path string, data []byte, enc func(io.Writer) io.WriteCloser) error {
	var buf bytes.Buffer
	w := enc(&buf)
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("compress %s: %w", path, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("compress %s: %w", path, err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
