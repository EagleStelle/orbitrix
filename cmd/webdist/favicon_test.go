package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite docs/logo.svg and docs/logo-dark.svg")

// TestDocsLogos keeps the README's logos equal to the drawn logo. After changing the logo, rewrite them with
// `go test ./cmd/webdist -run TestDocsLogos -update`.
func TestDocsLogos(t *testing.T) {
	for path, want := range map[string][]byte{"../../docs/logo.svg": favicon(), "../../docs/logo-dark.svg": darkLogo()} {
		if *update {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the drawn logo: run go test ./cmd/webdist -run TestDocsLogos -update", path)
		}
	}
}

func TestFaviconIsWellFormedSVG(t *testing.T) {
	d := xml.NewDecoder(bytes.NewReader(favicon()))
	shapes := map[string]int{}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if el, ok := tok.(xml.StartElement); ok {
			shapes[el.Name.Local]++
		}
	}
	if shapes["svg"] != 1 || shapes["path"] != 4 || shapes["circle"] != 5 {
		t.Errorf("elements = %v, want one svg, four corner paths and five orbs", shapes)
	}
}
