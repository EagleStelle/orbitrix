package ui

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	fontdata "orbitrix/client/fonts"
)

// Fonts: Unbounded (display) and Archivo (UI), both OFL, embedded with their licences in fonts/.
var (
	SrcDisplay = mustFont(fontdata.Unbounded)
	SrcUI      = mustFont(fontdata.Archivo)
	tagWeight  = text.MustParseTag("wght")
)

func mustFont(ttf []byte) *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		panic("client: embedded font: " + err.Error())
	}
	return s
}

func NewFace(src *text.GoTextFaceSource, size float64, weight float32) *text.GoTextFace {
	f := &text.GoTextFace{Source: src, Size: size}
	f.SetVariation(tagWeight, weight)
	return f
}

// fonts is the type scale, rebuilt when the UI scale changes. Sizes are logical px.
type Fonts struct {
	Logo, Title, Heading, Callout, CalloutCompact, CalloutTight, Number *text.GoTextFace // Unbounded
	Body, BodyBold, Label, LabelBold, Small, SmallBold                  *text.GoTextFace // Archivo
}

func NewFonts(u float64) Fonts {
	return Fonts{
		Logo:           NewFace(SrcDisplay, 80*u, 800),
		Title:          NewFace(SrcDisplay, 26*u, 700),
		Heading:        NewFace(SrcDisplay, 17*u, 700),
		Callout:        NewFace(SrcDisplay, 20*u, 800),
		CalloutCompact: NewFace(SrcDisplay, 15*u, 800),
		CalloutTight:   NewFace(SrcDisplay, 13*u, 800),
		Number:         NewFace(SrcDisplay, 20*u, 700),
		Body:           NewFace(SrcUI, 18*u, 500),
		BodyBold:       NewFace(SrcUI, 18*u, 700),
		Label:          NewFace(SrcUI, 15*u, 500),
		LabelBold:      NewFace(SrcUI, 15*u, 700),
		Small:          NewFace(SrcUI, 13*u, 500),
		SmallBold:      NewFace(SrcUI, 13*u, 700),
	}
}
