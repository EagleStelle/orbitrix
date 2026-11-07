//go:build !js

package platform

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"orbitrix/core"
)

// PickMapFile is supplied by platforms with a system document picker. Desktop
// clients fall back to entering a path; Android installs its picker bridge.
var PickMapFile func(done func([]byte, error)) bool

// ExportMapFile saves a uniquely named portable file and reports its location.
// Android replaces this with its system document creation bridge.
var ExportMapFile = exportMapFile

func exportMapFile(name string, data []byte, done func(string, error)) {
	path, err := writeMapExport(name, data)
	done(path, err)
}

func writeMapExport(name string, data []byte) (string, error) {
	if len(data) > core.MaxMapBytes {
		return "", fmt.Errorf("export map: file is too large")
	}
	d, err := dir()
	if err != nil {
		return "", err
	}
	folder := filepath.Join(d, "exports")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fmt.Errorf("export map: %w", err)
	}
	var safe strings.Builder
	for _, r := range strings.TrimSuffix(filepath.Base(name), ".orbitrixmap") {
		if safe.Len() >= 48 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			safe.WriteRune(r)
		case r == ' ':
			safe.WriteByte('-')
		}
	}
	base := strings.Trim(safe.String(), "-_")
	if base == "" {
		base = "untitled"
	}
	f, err := os.CreateTemp(folder, "map-"+base+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("export map: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return "", fmt.Errorf("export map: %w", writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("export map: %w", closeErr)
	}
	path := strings.TrimSuffix(tmp, ".tmp") + ".orbitrixmap"
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("export map: %w", err)
	}
	return path, nil
}

// ReadMapFile bounds the read before the portable decoder sees untrusted data.
func ReadMapFile(path string) ([]byte, error) {
	f, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("import map: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, core.MaxMapBytes+1))
	if err != nil {
		return nil, fmt.Errorf("import map: %w", err)
	}
	if len(b) > core.MaxMapBytes {
		return nil, fmt.Errorf("import map: file is too large")
	}
	return b, nil
}

func MapImport() string { return "" }
