package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/platform"
	"orbitrix/core"
)

const maxLibraryMaps, maxLibraryBytes = 128, 4 << 20

type mapLibrary struct {
	Version int                  `json:"version"`
	Maps    []core.MapDefinition `json:"maps"`
}

// mapStore only commits its in-memory list after the persistent write succeeds.
// A damaged library remains read-only instead of being replaced by an empty one.
type mapStore struct {
	maps    []core.MapDefinition
	loadErr error
	write   func(string, []byte) error
}

func decodeMapLibrary(b []byte) ([]core.MapDefinition, error) {
	if len(b) > maxLibraryBytes {
		return nil, fmt.Errorf("map library exceeds 4 MiB")
	}
	var saved mapLibrary
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&saved); err != nil {
		return nil, fmt.Errorf("read map library: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("read map library: trailing contents")
	}
	if saved.Version != 1 || len(saved.Maps) > maxLibraryMaps {
		return nil, fmt.Errorf("unsupported map library version or too many maps")
	}
	for i, m := range saved.Maps {
		if err := m.ValidateStructure(); err != nil {
			return nil, fmt.Errorf("read map %d: %w", i+1, err)
		}
	}
	return saved.Maps, nil
}

func encodeMapLibrary(maps []core.MapDefinition) ([]byte, error) {
	if len(maps) > maxLibraryMaps {
		return nil, fmt.Errorf("map library is full; export and remove a map first")
	}
	for _, m := range maps {
		if err := m.ValidateStructure(); err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(mapLibrary{Version: 1, Maps: maps})
	if err != nil {
		return nil, fmt.Errorf("save map library: %w", err)
	}
	if len(b) > maxLibraryBytes {
		return nil, fmt.Errorf("map library exceeds 4 MiB; export and remove a map first")
	}
	return b, nil
}

func loadMapStore(read func(string) ([]byte, error), write func(string, []byte) error) *mapStore {
	s := &mapStore{write: write}
	b, err := read("maps.json")
	if errors.Is(err, fs.ErrNotExist) {
		s.maps = core.StarterMaps()
		return s
	}
	if err != nil {
		s.loadErr = fmt.Errorf("could not open saved maps: %w", err)
		return s
	}
	s.maps, s.loadErr = decodeMapLibrary(b)
	return s
}

func (s *mapStore) commit(maps []core.MapDefinition) error {
	if s.loadErr != nil {
		return fmt.Errorf("saved map library could not be read; it was left untouched: %w", s.loadErr)
	}
	b, err := encodeMapLibrary(maps)
	if err != nil {
		return err
	}
	if err := s.write("maps.json", b); err != nil {
		return fmt.Errorf("save maps: %w", err)
	}
	s.maps = maps
	return nil
}

// put appends at -1, or replaces one existing entry without aliasing its draft.
func (s *mapStore) put(index int, m core.MapDefinition) error {
	if index < -1 || index >= len(s.maps) {
		return fmt.Errorf("saved map no longer exists")
	}
	maps := append([]core.MapDefinition(nil), s.maps...)
	if index < 0 {
		maps = append(maps, m.Clone())
	} else {
		maps[index] = m.Clone()
	}
	return s.commit(maps)
}

func (s *mapStore) remove(index int) error {
	if index < 0 || index >= len(s.maps) {
		return fmt.Errorf("saved map no longer exists")
	}
	maps := make([]core.MapDefinition, 0, len(s.maps)-1)
	maps = append(maps, s.maps[:index]...)
	maps = append(maps, s.maps[index+1:]...)
	return s.commit(maps)
}

func mapCanStart(s *settings) bool {
	if s.Map == nil {
		return s.Players <= s.Cols*s.Rows
	}
	if s.mapChecked != s.Map {
		s.mapChecked = s.Map
		s.mapOK = s.Map.ValidateStructure() == nil && len(s.Map.Validate()) == 0
	}
	return s.mapOK && s.Players <= s.Map.FloorCount()
}

func (s *setupScene) mapButton(a *app) rect {
	r := s.previewPanel
	return rect{r.x + 8*a.u, r.bottom() - 48*a.u, max(1, r.w-16*a.u), 40 * a.u}
}

type mapNotice struct {
	message string
	width   float64
	lines   []string
}

func (n *mapNotice) set(message string) { n.message, n.width, n.lines = message, -1, nil }

func (n *mapNotice) layout(a *app, width float64) float64 {
	if n.message == "" {
		return 0
	}
	if n.width != width {
		n.width, n.lines = width, wrap(n.message, a.f.label, width)
	}
	return float64(len(n.lines))*24*a.u + 12*a.u
}

func (n *mapNotice) draw(dst *ebiten.Image, a *app, x, y float64) {
	for i, line := range n.lines {
		drawText(dst, line, a.f.label, x, y+(float64(i)+0.5)*24*a.u, text.AlignStart, a.sc.ink)
	}
}

type libraryEntry struct {
	issues     []core.MapIssue
	floors     int
	status     string
	name       string
	nameShown  string
	nameWidth  float64
	nameScale  float64
	thumb      *ebiten.Image
	thumbMode  *scheme
	thumbSize  int
	thumbMarks bool
}

type mapsScene struct {
	page
	back        scene
	pick        func(*core.MapDefinition)
	store       *mapStore
	entries     []libraryEntry
	selected    int
	rows        []rect
	actions     [7]rect
	toolbar     [2]rect
	recovery    [2]rect
	draft       *core.MapDefinition
	reloadDraft bool
	confirm     *askDialog
	deleteIndex int
	discard     bool
	notice      mapNotice
	noticeY     float64
	importText  string
}

func newMaps(a *app, back scene, pick func(*core.MapDefinition)) *mapsScene {
	s := &mapsScene{back: back, pick: pick, selected: -1, store: loadMapStore(platform.Load, platform.Save)}
	if s.store.loadErr != nil {
		s.notice.set("Your saved maps could not be read. The existing library will remain untouched. " + s.store.loadErr.Error())
	}
	s.refresh()
	s.readDraft()
	return s
}

func (s *mapsScene) refresh() {
	for _, entry := range s.entries {
		if entry.thumb != nil {
			entry.thumb.Deallocate()
		}
	}
	s.entries = make([]libraryEntry, len(s.store.maps))
	s.rows = make([]rect, len(s.store.maps))
	for i, m := range s.store.maps {
		e := &s.entries[i]
		e.issues, e.name = m.Validate(), m.Name
		e.floors = m.FloorCount()
		if e.name == "" {
			e.name = "Untitled map"
		}
		e.status = numbers[m.Cols] + " × " + numbers[m.Rows]
		if len(e.issues) > 0 {
			e.status += " · Draft"
		} else {
			e.status += " · Ready"
		}
	}
	if s.selected >= len(s.entries) {
		s.selected = len(s.entries) - 1
	}
	if len(s.entries) == 0 && s.store.loadErr == nil {
		s.notice.set("No maps saved yet. Create a map or import one.")
	}
}

func (s *mapsScene) readDraft() {
	s.reloadDraft, s.draft = false, nil
	if b, err := platform.Load("map-draft.orbitrixmap"); err == nil && len(b) > 0 {
		if m, err := core.DecodeMap(b); err == nil {
			s.draft = &m
		} else {
			s.notice.set("The recovered draft could not be read. Your map library is unchanged.")
		}
	}
}

func (s *mapsScene) saveAt(index int) func(core.MapDefinition) error {
	return func(m core.MapDefinition) error {
		if err := s.store.put(index, m); err != nil {
			return err
		}
		if index < 0 {
			index = len(s.store.maps) - 1
			s.selected = index
		}
		s.refresh()
		s.reloadDraft = true
		s.notice.set("Map saved on this device.")
		return nil
	}
}

func (s *mapsScene) edit(a *app, m core.MapDefinition, index int) {
	s.reloadDraft = true
	a.next = newMapEditor(a, s, m.Clone(), s.saveAt(index))
}

func (s *mapsScene) choose(a *app, m *core.MapDefinition) {
	if m != nil {
		copy := m.Clone()
		m = &copy
	}
	if s.pick != nil {
		s.pick(m)
	} else {
		a.set.Map = m
		if m != nil {
			a.set.Cols, a.set.Rows = m.Cols, m.Rows
		}
		a.set.save()
	}
	a.next = s.back
}

func (s *mapsScene) layout(a *app) {
	s.frame(a, true)
	v, u := s.view.viewport, a.u
	cols := max(1, min(3, int(v.w/(132*u))))
	buttonW := (v.w - float64(cols-1)*8*u) / float64(cols)
	actionRows := (len(s.actions) + cols - 1) / cols
	h := 56 * u
	if s.draft != nil {
		h += 56 * u
	}
	noticeH := s.notice.layout(a, v.w)
	h += noticeH + float64(len(s.rows))*112*u
	if s.selected >= 0 {
		h += float64(actionRows)*52*u + 12*u
	}
	y := s.content(h)
	s.toolbar = [2]rect{{v.x, y, (v.w - 8*u) / 2, 40 * u}, {v.cx() + 4*u, y, (v.w - 8*u) / 2, 40 * u}}
	y += 56 * u
	s.recovery = [2]rect{}
	if s.draft != nil {
		s.recovery = [2]rect{{v.x, y, (v.w - 8*u) / 2, 40 * u}, {v.cx() + 4*u, y, (v.w - 8*u) / 2, 40 * u}}
		y += 56 * u
	}
	s.noticeY = y
	y += noticeH
	for i := range s.rows {
		s.rows[i] = rect{v.x, y, v.w, 100 * u}
		y += 112 * u
		if i == s.selected {
			for j := range s.actions {
				s.actions[j] = rect{v.x + float64(j%cols)*(buttonW+8*u), y + float64(j/cols)*52*u, buttonW, 40 * u}
			}
			y += float64(actionRows)*52*u + 12*u
		}
	}
}

func (s *mapsScene) update(a *app) error {
	if s.reloadDraft {
		s.readDraft()
	}
	if s.importText != "" {
		text := s.importText
		s.importText = ""
		i := newMapImport(a, s, s.saveAt(-1))
		i.input.set(text)
		i.previewCode(text)
		a.next = i
		return nil
	}
	s.layout(a)
	if s.confirm != nil {
		yes, back := s.confirm.update(a)
		if yes {
			if s.discard {
				if err := platform.Save("map-draft.orbitrixmap", nil); err != nil {
					s.notice.set(err.Error())
				} else {
					s.draft = nil
				}
			} else if err := s.store.remove(s.deleteIndex); err != nil {
				s.notice.set(err.Error())
			} else {
				s.selected = -1
				s.refresh()
			}
		}
		if yes || back {
			s.confirm = nil
		}
		return nil
	}
	if s.scrolled(a) {
		s.layout(a)
	}
	if s.backed(a) {
		a.next = s.back
		return nil
	}
	if a.clicked(s.action) {
		s.edit(a, core.NewMap(a.set.Cols, a.set.Rows), -1)
		return nil
	}
	if !s.inBody(a) {
		return nil
	}
	switch {
	case a.clicked(compactHit(s.toolbar[0], a.u)):
		a.next = newMapImport(a, s, s.saveAt(-1))
	case a.clicked(compactHit(s.toolbar[1], a.u)):
		s.choose(a, nil)
	case s.draft != nil && a.clicked(compactHit(s.recovery[0], a.u)):
		s.edit(a, *s.draft, -1)
	case s.draft != nil && a.clicked(compactHit(s.recovery[1], a.u)):
		s.discard = true
		s.confirm = &askDialog{text: "Discard the recovered draft?", yes: "Discard draft"}
	default:
		for i, row := range s.rows {
			if a.clicked(row) {
				s.selected = i
				return nil
			}
		}
		if s.selected < 0 {
			return nil
		}
		m := s.store.maps[s.selected]
		for action, r := range s.actions {
			if !a.clicked(compactHit(r, a.u)) {
				continue
			}
			switch action {
			case 0:
				if len(s.entries[s.selected].issues) == 0 && a.set.Players <= m.FloorCount() {
					s.choose(a, &m)
				}
			case 1, 2:
				s.edit(a, m, s.selected)
			case 3:
				m.Name = mapCopyName(m.Name)
				if err := s.store.put(-1, m); err != nil {
					s.notice.set(err.Error())
				} else {
					s.selected = len(s.store.maps) - 1
					s.refresh()
				}
			case 4:
				s.deleteIndex, s.discard = s.selected, false
				s.confirm = &askDialog{text: "Delete this saved map?", yes: "Delete map"}
			case 5, 6:
				a.next = newMapShare(a, s, m, action == 5)
			}
			break
		}
	}
	return nil
}

func mapCopyName(name string) string {
	for len(name) > 57 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name + " (copy)"
}

func (s *mapsScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	s.page.draw(dst, a, "Maps")
	b, u := s.body(dst), a.u
	drawButton(b, a, s.toolbar[0], "Import", "", btnTile, true)
	drawButton(b, a, s.toolbar[1], "Classic board", "", btnTile, true)
	if s.draft != nil {
		drawButton(b, a, s.recovery[0], "Recover draft", "", btnTile, true)
		drawButton(b, a, s.recovery[1], "Discard draft", "", btnTile, true)
	}
	s.notice.draw(b, a, s.view.viewport.x, s.noticeY)
	for i, r := range s.rows {
		if r.bottom() < s.view.viewport.y || r.y > s.view.viewport.bottom() {
			continue
		}
		e, m := &s.entries[i], s.store.maps[i]
		fillRound(b, r, 16*u, a.sc.tile)
		if s.selected == i {
			strokeRound(b, r, 16*u, 2.5*u, a.sc.ink)
		}
		thumb := rect{r.x + 12*u, r.y + 12*u, 76 * u, 76 * u}
		if e.thumb == nil || e.thumbMode != a.sc || e.thumbSize != int(thumb.w) || e.thumbMarks != a.set.Marks {
			if e.thumb != nil {
				e.thumb.Deallocate()
			}
			e.thumbSize, e.thumbMode = int(thumb.w), a.sc
			e.thumbMarks = a.set.Marks
			e.thumb = ebiten.NewImage(max(1, e.thumbSize), max(1, e.thumbSize))
			drawMapThumb(e.thumb, a, m, rect{0, 0, thumb.w, thumb.h})
		}
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(thumb.x, thumb.y)
		b.DrawImage(e.thumb, &op)
		x, width := thumb.right()+12*u, r.right()-thumb.right()-24*u
		if e.nameWidth != width || e.nameScale != u {
			e.nameWidth, e.nameScale, e.nameShown = width, u, e.name
			if textWidth(e.nameShown, a.f.heading) > width {
				for len(e.nameShown) > 0 && textWidth(e.nameShown+"…", a.f.heading) > width {
					_, size := utf8.DecodeLastRuneInString(e.nameShown)
					e.nameShown = e.nameShown[:len(e.nameShown)-size]
				}
				e.nameShown += "…"
			}
		}
		drawText(b, e.nameShown, a.f.heading, x, r.y+36*u, text.AlignStart, a.sc.ink)
		status := e.status
		if len(e.issues) == 0 && a.set.Players > e.floors {
			status = "Too small for these players"
		}
		drawText(b, status, a.f.label, x, r.y+66*u, text.AlignStart, a.sc.dim)
	}
	if s.selected >= 0 {
		labels := [...]string{"Use map", "Edit", "Rename", "Duplicate", "Delete", "Export", "Share"}
		for i, label := range labels {
			enabled := true
			if i == 0 {
				enabled = len(s.entries[s.selected].issues) == 0 && a.set.Players <= s.store.maps[s.selected].FloorCount()
			}
			drawButton(b, a, s.actions[i], label, "", btnTile, enabled)
		}
	}
	drawButton(dst, a, s.action, "New map", "", btnInk, true)
	if s.confirm != nil {
		s.confirm.draw(dst, a)
	}
}

// Libraries, setup and sharing preview the same map pieces as the playing board.
func drawMapThumb(dst *ebiten.Image, a *app, m core.MapDefinition, r rect) {
	if m.ValidateStructure() != nil {
		return
	}
	cell := min(r.w/float64(m.Cols), r.h/float64(m.Rows))
	ox, oy := r.cx()-float64(m.Cols)*cell/2, r.cy()-float64(m.Rows)*cell/2
	drawMapArt(dst, a.sc, mapArtState{Map: &m}, mapArtTransform{X: ox, Y: oy, Cell: cell}, true, a.set.Marks)
}

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
	box, file rect
	preview   rect
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
	v, u := s.view.viewport, a.u
	thumbH := min(280*u, v.w)
	h := 28*u + 52*u + 12*u + 40*u + 24*u + s.status.layout(a, v.w)
	if s.mapData != nil {
		h += thumbH + 48*u
	}
	y := s.content(h)
	s.box = rect{v.x, y + 28*u, v.w, 52 * u}
	s.file = rect{v.x, s.box.bottom() + 12*u, v.w, 40 * u}
	s.statusY = s.file.bottom() + 16*u
	s.preview = rect{v.x, s.statusY + s.status.layout(a, v.w), v.w, thumbH}
	s.input.caret = image.Rect(int(s.box.x), int(s.box.y), int(s.box.right()), int(s.box.bottom()))
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
	if !s.waiting && s.inBody(a) && a.clicked(compactHit(s.file, a.u)) {
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
	drawText(b, "Share code, link, or file path", a.f.labelBold, s.box.x, s.box.y-14*u, text.AlignStart, a.sc.ink)
	fillPill(b, s.box, a.sc.tile)
	if s.drawn != s.input.edits {
		s.drawn = s.input.edits
		runes := s.input.shown[max(0, len(s.input.shown)-100):]
		for len(runes) > 0 && textWidth(string(runes), a.f.label) > s.box.w-32*u {
			runes = runes[1:]
		}
		s.shown = string(runes)
	}
	drawText(b, s.shown, a.f.label, s.box.x+16*u, s.box.cy(), text.AlignStart, a.sc.ink)
	if s.input.focused {
		strokeRound(b, s.box, s.box.h/2, 2.5*u, a.sc.ink)
	}
	drawButton(b, a, s.file, "Choose file", "", btnTile, !s.waiting)
	s.status.draw(b, a, s.view.viewport.x, s.statusY)
	if s.mapData != nil {
		drawMapThumb(b, a, *s.mapData, s.preview)
		drawText(b, s.mapData.Name, a.f.heading, s.preview.cx(), s.preview.bottom()+24*u, text.AlignCenter, a.sc.ink)
	}
	label := "Preview"
	if s.mapData != nil {
		label = "Save map"
	}
	drawButton(dst, a, s.action, label, "", btnInk, !s.waiting && (len(s.input.shown) > 0 || s.mapData != nil))
}

type mapShareScene struct {
	page
	back             scene
	mapData          core.MapDefinition
	data             []byte
	code, link, scan string
	qr               qrCode
	box, thumb       rect
	buttons          [3]rect
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
	s.qr.set(s.scan)
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
	v, u := s.view.viewport, a.u
	size := min(360*u, v.w)
	h := size + 24*u + 3*52*u + s.status.layout(a, v.w)
	y := s.content(h)
	s.box = rect{v.cx() - size/2, y, size, size}
	y = s.box.bottom() + 24*u
	for i := range s.buttons {
		s.buttons[i] = rect{v.x, y + float64(i)*52*u, v.w, 40 * u}
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
		if value != "" && a.clicked(compactHit(s.buttons[i], a.u)) {
			platform.Copy(value, func(ok bool) {
				select {
				case s.copyDone <- ok:
				default:
				}
			})
		}
	}
	if !s.waiting && a.clicked(compactHit(s.buttons[2], a.u)) {
		s.exportFile()
	}
	return nil
}

func (s *mapShareScene) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	s.page.draw(dst, a, "Share map")
	b := s.body(dst)
	if s.scan != "" {
		s.qr.draw(b, s.box)
	} else {
		drawMapThumb(b, a, s.mapData, s.box)
	}
	drawButton(b, a, s.buttons[0], "Copy code", "", btnTile, s.code != "")
	drawButton(b, a, s.buttons[1], "Copy link", "", btnTile, s.link != "")
	drawButton(b, a, s.buttons[2], "Export file", "", btnTile, len(s.data) > 0 && !s.waiting)
	s.status.draw(b, a, s.view.viewport.x, s.statusY)
}
