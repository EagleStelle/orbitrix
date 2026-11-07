//go:build !js

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orbitrix/core"
)

func TestMapFileExportImport(t *testing.T) {
	previous := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = previous }()
	b, err := core.EncodeMap(core.NewMap(9, 9))
	if err != nil {
		t.Fatal(err)
	}
	first, err := writeMapExport("../../Pressure:Points", b)
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeMapExport("../../Pressure:Points", b)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, filepath.Join(dataDir, "exports")+string(os.PathSeparator)) || !strings.HasSuffix(first, ".orbitrixmap") {
		t.Fatal("export must stay local with unique portable filenames")
	}
	read, err := ReadMapFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.DecodeMap(read); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dataDir, "oversized.orbitrixmap")
	if err := os.WriteFile(big, make([]byte, core.MaxMapBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMapFile(big); err == nil {
		t.Fatal("accepted oversized file")
	}
}
