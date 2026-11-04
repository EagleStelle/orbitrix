package appicon

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/xml"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestImage(t *testing.T) {
	img := Image(192)
	for _, p := range []struct {
		x, y int
		want color.RGBA
	}{
		{96, 96, color.RGBA{0xD5, 0x5E, 0x00, 0xFF}},
		{168, 96, color.RGBA{0xD5, 0x5E, 0x00, 0xFF}},
		{126, 126, color.RGBA{0x17, 0x16, 0x15, 0xFF}},
		{143, 143, color.RGBA{0x17, 0x16, 0x15, 0xFF}},
		{136, 136, color.RGBA{}},
		{0, 0, color.RGBA{}},
	} {
		if got := img.RGBAAt(p.x, p.y); got != p.want {
			t.Errorf("pixel (%d, %d) = %v, want %v", p.x, p.y, got, p.want)
		}
	}
	for y := range 192 {
		for x := range 192 {
			if img.RGBAAt(x, y) != img.RGBAAt(191-x, y) || img.RGBAAt(x, y) != img.RGBAAt(y, x) {
				t.Fatalf("logo lost its symmetry at (%d, %d)", x, y)
			}
		}
	}
}

func TestICNS(t *testing.T) {
	data, err := ICNS()
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:])) != len(data) {
		t.Fatal("invalid ICNS header")
	}
	data = data[8:]
	for i, size := range []int{128, 256, 512, 1024} {
		if len(data) < 8 || string(data[:4]) != []string{"ic07", "ic08", "ic09", "ic10"}[i] {
			t.Fatalf("missing %dpx icon", size)
		}
		n := int(binary.BigEndian.Uint32(data[4:]))
		if n < 8 || n > len(data) {
			t.Fatal("invalid ICNS chunk length")
		}
		img, err := png.Decode(bytes.NewReader(data[8:n]))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			t.Fatalf("%dpx icon: %v", size, img.Bounds())
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("%dpx icon background is not transparent", size)
		}
		data = data[n:]
	}
	if len(data) != 0 {
		t.Fatal("unexpected trailing ICNS data")
	}
}

func TestAndroidLauncherTransparency(t *testing.T) {
	var legacy struct {
		Items []struct {
			Drawable string `xml:"drawable,attr"`
		} `xml:"item"`
	}
	data, err := os.ReadFile("../../android/src/main/res/drawable/ic_launcher.xml")
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if len(legacy.Items) != 1 || legacy.Items[0].Drawable != "@drawable/ic_launcher_foreground" {
		t.Fatal("legacy launcher must contain only the transparent logo")
	}
	var adaptive struct {
		Background struct {
			Drawable string `xml:"drawable,attr"`
		} `xml:"background"`
	}
	data, err = os.ReadFile("../../android/src/main/res/drawable-v26/ic_launcher.xml")
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(data, &adaptive); err != nil {
		t.Fatal(err)
	}
	if adaptive.Background.Drawable != "@android:color/transparent" {
		t.Fatal("adaptive launcher background must be transparent")
	}
}

// Exercise the real Go linker, so a valid-looking COFF file cannot silently lose its icon.
func TestWindowsResourceLinks(t *testing.T) {
	resource, err := WindowsResource()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"go.mod":                  []byte("module icontest\n"),
		"main.go":                 []byte("package main\nfunc main() {}\n"),
		"icon_windows_amd64.syso": resource,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "-o", "icon.exe", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("link icon: %v\n%s", err, output)
	}
	f, err := pe.Open(filepath.Join(dir, "icon.exe"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	section := f.Section(".rsrc")
	if section == nil {
		t.Fatal("executable has no resources")
	}
	data, err := section.Data()
	if err != nil {
		t.Fatal(err)
	}
	u32 := func(offset uint32) uint32 { return binary.LittleEndian.Uint32(data[offset:]) }
	// Walk both resource trees by ID, then follow the linker's relocated data RVA.
	for i, wantType := range []uint32{3, 14} {
		entry := uint32(16 + 8*i)
		if u32(entry) != wantType {
			t.Fatalf("resource type = %d, want %d", u32(entry), wantType)
		}
		name := u32(entry+4) & 0x7fffffff
		if u32(name+16) != 1 {
			t.Fatal("resource ID is not 1")
		}
		lang := u32(name+20) & 0x7fffffff
		leaf := u32(lang + 20)
		offset, length := u32(leaf)-section.VirtualAddress, u32(leaf+4)
		if uint64(offset)+uint64(length) > uint64(len(data)) {
			t.Fatal("resource RVA is outside .rsrc")
		}
		payload := data[offset : offset+length]
		if wantType == 3 {
			img, err := png.Decode(bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
				t.Fatalf("embedded icon: %v", img.Bounds())
			}
			if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
				t.Fatal("Windows icon background is not transparent")
			}
		} else if len(payload) != 20 || binary.LittleEndian.Uint16(payload[2:]) != 1 ||
			binary.LittleEndian.Uint16(payload[4:]) != 1 || binary.LittleEndian.Uint16(payload[18:]) != 1 {
			t.Fatal("invalid icon group or missing icon reference")
		}
	}
}
