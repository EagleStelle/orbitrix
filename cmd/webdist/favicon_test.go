package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"testing"
)

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
