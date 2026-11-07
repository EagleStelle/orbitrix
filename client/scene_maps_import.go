package client

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
)

type mapFileResult struct {
	data []byte
	name string
	err  error
}

type mapImportScene struct {
	page
	back      scene
	save      func(core.MapDefinition) error
	input     *field
	box, file ui.Rect
	preview   ui.Rect
	mapData   *core.MapDefinition
	status    mapNotice
	statusY   float64
	fileDone  chan mapFileResult
	waiting   bool
	drawn     int
	shown     string
}

func newMapImport(a *app, back scene, save func(core.MapDefinition) error) *mapImportScene {
	return &mapImportScene{back: back, save: save, input: newField(2*core.MaxMapBytes, false, printable), drawn: -1, fileDone: make(chan mapFileResult, 1)}
}

func (s *mapImportScene) accept(m core.MapDefinition) {
	s.mapData = &m
	issues := m.Validate()
	message := "Ready to save on this device."
	if len(issues) > 0 {
		message = "This is a draft. Save it, then edit before playing. " + issues[0].Message
	}
	s.status.set(message)
}

func (s *mapImportScene) previewCode(code string) {
	m, err := core.ParseMapCode(code)
	if err != nil {
		s.mapData = nil
		s.status.set(err.Error())
		return
	}
	s.accept(m)
}

func (s *mapImportScene) layout(a *app) {
	s.frame(a, true)
	v, u := s.view.Viewport, a.u
	thumbH := min(280*u, v.W)
	h := 28*u + 52*u + 12*u + 40*u + 24*u + s.status.layout(a, v.W)
	if s.mapData != nil {
		h += thumbH + 48*u
	}
	y := s.content(h)
	s.box = ui.Rect{X: v.X, Y: y + 28*u, W: v.W, H: 52 * u}
	s.file = ui.Rect{X: v.X, Y: s.box.Bottom() + 12*u, W: v.W, H: 40 * u}
	s.statusY = s.file.Bottom() + 16*u
	s.preview = ui.Rect{X: v.X, Y: s.statusY + s.status.layout(a, v.W), W: v.W, H: thumbH}
	s.input.caret = image.Rect(int(s.box.X), int(s.box.Y), int(s.box.Right()), int(s.box.Bottom()))
}

func (s *mapImportScene) update(a *app) error {
	select {
	case result := <-s.fileDone:
		s.waiting = false
		if result.err == nil && result.data == nil {
			break
		}
		s.mapData = nil
		if result.err != nil {
			s.status.set(result.err.Error())
		} else if m, err := core.DecodeMap(result.data); err != nil {
			s.status.set(err.Error())
		} else {
			s.accept(m)
		}
	default:
	}
	s.layout(a)
	if s.scrolled(a) {
		s.layout(a)
	}
	if s.backed(a) {
		a.next = s.back
		return nil
	}
	if s.inBody(a) {
		s.input.tap(a, s.box)
	}
	before := s.input.edits
	enter := s.input.update(a)
	if before != s.input.edits {
		s.mapData = nil
	}
	if !s.waiting && s.inBody(a) && a.clicked(ui.CompactHit(s.file, a.u)) {
		if platform.PickMapFile != nil && platform.PickMapFile(func(data []byte, err error) { s.fileDone <- mapFileResult{data: data, err: err} }) {
			s.waiting = true
		} else {
			s.status.set("Paste a file path above, then choose Preview.")
		}
	}
	if enter || a.clicked(s.action) {
		if s.mapData != nil {
			if err := s.save(*s.mapData); err != nil {
				s.status.set(err.Error())
			} else {
				a.next = s.back
			}
		} else {
			value := strings.TrimSpace(s.input.value())
			if strings.HasPrefix(value, "orbitrix-map") || strings.Contains(value, "#map=") {
				s.previewCode(value)
			} else if data, err := platform.ReadMapFile(value); err != nil {
				s.status.set(err.Error())
			} else if m, err := core.DecodeMap(data); err != nil {
				s.status.set(err.Error())
			} else {
				s.accept(m)
			}
		}
	}
	return nil
}

func (s *mapImportScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	s.page.draw(dst, a, "Import map")
	b, u := s.body(dst), a.u
	ui.DrawText(b, "Share code, link, or file path", a.f.LabelBold, s.box.X, s.box.Y-14*u, text.AlignStart, a.sc.Ink)
	ui.FillPill(b, s.box, a.sc.Tile)
	if s.drawn != s.input.edits {
		s.drawn = s.input.edits
		runes := s.input.shown[max(0, len(s.input.shown)-100):]
		for len(runes) > 0 && ui.TextWidth(string(runes), a.f.Label) > s.box.W-32*u {
			runes = runes[1:]
		}
		s.shown = string(runes)
	}
	ui.DrawText(b, s.shown, a.f.Label, s.box.X+16*u, s.box.Cy(), text.AlignStart, a.sc.Ink)
	if s.input.focused {
		ui.StrokeRound(b, s.box, s.box.H/2, 2.5*u, a.sc.Ink)
	}
	ui.DrawButton(b, a.ui(), s.file, "Choose file", "", ui.ButtonTile, !s.waiting)
	s.status.draw(b, a, s.view.Viewport.X, s.statusY)
	if s.mapData != nil {
		drawMapThumb(b, a, *s.mapData, s.preview)
		ui.DrawText(b, s.mapData.Name, a.f.Heading, s.preview.Cx(), s.preview.Bottom()+24*u, text.AlignCenter, a.sc.Ink)
	}
	label := "Preview"
	if s.mapData != nil {
		label = "Save map"
	}
	ui.DrawButton(dst, a.ui(), s.action, label, "", ui.ButtonInk, !s.waiting && (len(s.input.shown) > 0 || s.mapData != nil))
}
