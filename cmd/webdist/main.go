// Command webdist packages a web build (default dist/web), reusing unchanged compressed files.
// -src reads raw inputs from a separate folder; -gzip-only omits Brotli for native LAN bundles.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"

	"orbitrix/internal/appicon"
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
	src := flag.String("src", "", "raw build folder (default: output folder)")
	gzipOnly := flag.Bool("gzip-only", false, "generate gzip only for native LAN bundles")
	flag.Parse()
	dir := "dist/web"
	if flag.NArg() > 1 {
		log.Fatal("usage: webdist [-src folder] [-gzip-only] [output folder]")
	}
	if flag.NArg() == 1 {
		dir = flag.Arg(0)
	}
	if *src == "" {
		*src = dir
	}
	if err := run(*src, dir, *gzipOnly); err != nil {
		log.Fatal(err)
	}
}

func run(src, dir string, gzipOnly bool) error {
	wasm, err := os.Stat(filepath.Join(src, "game.wasm"))
	if err != nil {
		return err
	}
	text := map[string][]byte{}
	for _, name := range pages {
		if text[name], err = os.ReadFile(filepath.Join(src, name)); err != nil {
			return err
		}
	}
	if !bytes.Contains(text["index.html"], []byte(sizeMark)) {
		return fmt.Errorf("index.html: no %s to fill in", sizeMark)
	}
	text["index.html"] = bytes.Replace(text["index.html"], []byte(sizeMark),
		[]byte("/*WASM_SIZE*/"+strconv.FormatInt(wasm.Size(), 10)), 1)

	served := map[string][]byte{}
	for _, name := range hashed {
		data := appicon.SVG()
		if name != "favicon.svg" {
			data, err = os.ReadFile(filepath.Join(src, name))
			if err != nil {
				return err
			}
		}
		sum := sha256.Sum256(data)
		ext := filepath.Ext(name)
		renamed := strings.TrimSuffix(name, ext) + "." + hex.EncodeToString(sum[:6]) + ext
		ref, found := []byte(`"`+name+`"`), false
		for page, b := range text {
			found = found || bytes.Contains(b, ref)
			text[page] = bytes.ReplaceAll(b, ref, []byte(`"/`+renamed+`"`))
		}
		if !found {
			return fmt.Errorf("no page refers to %q", name)
		}
		served[renamed] = data
	}
	served["index.html"] = text["index.html"]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cachePath := filepath.Clean(dir) + ".cache.json"
	old := map[string]string{}
	if data, err := os.ReadFile(cachePath); err == nil {
		// Missing or damaged cache metadata just means recompressing.
		if json.Unmarshal(data, &old) != nil {
			old = nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	current := map[string]string{"version": compressionVersion()}
	if old["version"] != current["version"] {
		old = nil
	}
	for name, data := range served {
		sum := sha256.Sum256(data)
		key := hex.EncodeToString(sum[:])
		for ext, enc := range encoders {
			if gzipOnly && ext == ".br" {
				continue
			}
			file := name + ext
			path := filepath.Join(dir, file)
			info, err := os.Stat(path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if old[file] != key || info == nil || !info.Mode().IsRegular() {
				if err := compress(path, data, enc); err != nil {
					return err
				}
			}
			current[file] = key
		}
	}
	if err := writeFile(filepath.Join(dir, "landing.html"), text["landing.html"]); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if !entry.IsDir() && (ext == ".gz" || ext == ".br") && current[entry.Name()] == "" {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	if filepath.Clean(src) == filepath.Clean(dir) {
		for _, name := range append([]string{"index.html"}, hashed...) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return writeFile(cachePath, data)
}

func compressionVersion() string {
	version := fmt.Sprintf("%s/gzip=%d/brotli=%d", runtime.Version(), gzip.BestCompression, brotli.BestCompression)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/andybalholm/brotli" {
				if dep.Replace != nil {
					dep = dep.Replace
				}
				version += "/" + dep.Path + "@" + dep.Version + "/" + dep.Sum
			}
		}
	}
	return version
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
	return writeFile(path, buf.Bytes())
}

// A failed write must not leave a partial file that the next build treats as cached.
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".webdist-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
