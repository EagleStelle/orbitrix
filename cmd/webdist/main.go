// Command webdist finishes a web build in a folder (default dist/web): it writes game.wasm's size into index.html for
// the loading dots, then saves a gzip and a brotli copy of each file beside it, so a server sends them as they are.
package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/andybalholm/brotli"
)

// sizeMark is where index.html takes game.wasm's size.
const sizeMark = "/*WASM_SIZE*/0"

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
	index := filepath.Join(dir, "index.html")
	page, err := os.ReadFile(index)
	if err != nil {
		return err
	}
	if !bytes.Contains(page, []byte(sizeMark)) {
		return fmt.Errorf("%s: no %s to fill in", index, sizeMark)
	}
	page = bytes.Replace(page, []byte(sizeMark), []byte("/*WASM_SIZE*/"+strconv.FormatInt(wasm.Size(), 10)), 1)
	if err := os.WriteFile(index, page, 0o644); err != nil {
		return err
	}
	for _, name := range []string{"index.html", "wasm_exec.js", "game.wasm"} {
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
