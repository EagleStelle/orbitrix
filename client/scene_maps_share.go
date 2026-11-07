package client

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
)

type mapShareScene struct {
	page
	back             scene
	mapData          core.MapDefinition
	data             []byte
	code, link, scan string
	qr               ui.QR
	box, thumb       ui.Rect
	buttons          [3]ui.Rect
	status           mapNotice
	statusY          float64
	done             chan mapFileResult
	copyDone         chan bool
	waiting          bool
	autoExport       bool
}

func newMapShare(a *app, back scene, m core.MapDefinition, export bool) *mapShareScene {
	s := &mapShareScene{back: back, mapData: m, done: make(chan mapFileResult, 1), copyDone: make(chan bool, 2), autoExport: export}
	var err error
	s.data, err = core.EncodeMap(m)
	if err != nil {
		s.status.set(err.Error())
		return s
	}
	s.code, _ = core.MapCode(m)
	base := strings.TrimRight(platform.Origin(), "/")
	if base == "" {
		base = strings.TrimRight(platform.Address(), "/")
	}
	if base != "" {
		s.link = base + "/play#map=" + s.code
		s.scan = s.link
	} else {
		s.scan = s.code
		s.status.set("This build has no website address. Copy the code or export a file; a QR reader can also copy this code.")
	}
	if len(s.scan) > 1200 {
		s.scan = ""
		s.status.set("This map is too detailed for a readable QR. Copy its code or export a file.")
	}
	s.qr.Set(s.scan)
	return s
}

func (s *mapShareScene) exportFile() {
	if s.waiting || len(s.data) == 0 {
		return
	}
	s.waiting = true
	platform.ExportMapFile(s.mapData.Name, s.data, func(name string, err error) { s.done <- mapFileResult{name: name, err: err} })
}

func (s *mapShareScene) layout(a *app) {
	s.frame(a, false)
	v, u := s.view.Viewport, a.u
	size := min(360*u, v.W)
	h := size + 24*u + 3*52*u + s.status.layout(a, v.W)
	y := s.content(h)
	s.box = ui.Rect{X: v.Cx() - size/2, Y: y, W: size, H: size}
	y = s.box.Bottom() + 24*u
	for i := range s.buttons {
		s.buttons[i] = ui.Rect{X: v.X, Y: y + float64(i)*52*u, W: v.W, H: 40 * u}
	}
	s.statusY = y + 3*52*u
}

func (s *mapShareScene) update(a *app) error {
	if s.autoExport {
		s.autoExport = false
		s.exportFile()
	}
	select {
	case result := <-s.done:
		s.waiting = false
		if result.err != nil {
			s.status.set(result.err.Error())
		} else {
			s.status.set("Exported map: " + result.name)
		}
	default:
	}
	select {
	case copied := <-s.copyDone:
		if copied {
			s.status.set("Copied. The map data is included; no map server is needed.")
		} else {
			s.status.set("Clipboard unavailable. Export the map as a file instead.")
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
	if !s.inBody(a) {
		return nil
	}
	for i, value := range []string{s.code, s.link} {
		if value != "" && a.clicked(ui.CompactHit(s.buttons[i], a.u)) {
			platform.Copy(value, func(ok bool) {
				select {
				case s.copyDone <- ok:
				default:
				}
			})
		}
	}
	if !s.waiting && a.clicked(ui.CompactHit(s.buttons[2], a.u)) {
		s.exportFile()
	}
	return nil
}

func (s *mapShareScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	s.page.draw(dst, a, "Share map")
	b := s.body(dst)
	if s.scan != "" {
		s.qr.Draw(b, s.box)
	} else {
		drawMapThumb(b, a, s.mapData, s.box)
	}
	ui.DrawButton(b, a.ui(), s.buttons[0], "Copy code", "", ui.ButtonTile, s.code != "")
	ui.DrawButton(b, a.ui(), s.buttons[1], "Copy link", "", ui.ButtonTile, s.link != "")
	ui.DrawButton(b, a.ui(), s.buttons[2], "Export file", "", ui.ButtonTile, len(s.data) > 0 && !s.waiting)
	s.status.draw(b, a, s.view.Viewport.X, s.statusY)
}
