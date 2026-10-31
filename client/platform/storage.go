//go:build !js

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// CanExit reports whether the app may quit itself; storage_js.go says a browser tab can't.
const CanExit = true

// dataDir replaces the user config folder where the OS has none for apps (Android); see SetDataDir.
var dataDir string

// SetDataDir sets the folder that holds Orbitrix's files. The Android wrapper calls it before the game starts.
func SetDataDir(d string) { dataDir = d }

// dir is the per-user folder that holds Orbitrix's files.
func dir() (string, error) {
	if dataDir != "" {
		return dataDir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "Orbitrix"), nil
}

// Load reads a saved file by name.
func Load(name string) ([]byte, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(d, name))
}

// Save writes a file by name, through a temporary file so a crash never leaves it half-written.
func Save(name string, data []byte) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", d, err)
	}
	tmp := filepath.Join(d, name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, filepath.Join(d, name)); err != nil {
		return fmt.Errorf("replace %s: %w", name, err)
	}
	return nil
}
