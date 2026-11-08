package client

import (
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/mapstore"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
)

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

func (s *setupScene) mapButton(a *app) ui.Rect {
	r := s.previewPanel
	return ui.Rect{X: r.X + 8*a.u, Y: r.Bottom() - 48*a.u, W: max(1, r.W-16*a.u), H: 40 * a.u}
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
		n.width, n.lines = width, wrap(n.message, a.f.Label, width)
	}
	return float64(len(n.lines))*24*a.u + 12*a.u
}

func (n *mapNotice) draw(dst *ebiten.Image, a *app, x, y float64) {
	for i, line := range n.lines {
		ui.DrawText(dst, line, a.f.Label, x, y+(float64(i)+0.5)*24*a.u, text.AlignStart, a.sc.Ink)
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
	thumbMode  *ui.Palette
	thumbSize  int
	thumbMarks bool
}

type mapsScene struct {
	page
	back        scene
	pick        func(*core.MapDefinition)
	store       *mapstore.Store
	entries     []libraryEntry
	selected    int
	rows        []ui.Rect
	actions     [7]ui.Rect
	toolbar     [2]ui.Rect
	recovery    [2]ui.Rect
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
	s := &mapsScene{back: back, pick: pick, selected: -1, store: mapstore.Load(platform.Load, platform.Save)}
	if s.store.LoadErr != nil {
		s.notice.set("Your saved maps could not be read. The existing library will remain untouched. " + s.store.LoadErr.Error())
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
	s.entries = make([]libraryEntry, len(s.store.Maps))
	s.rows = make([]ui.Rect, len(s.store.Maps))
	for i, m := range s.store.Maps {
		e := &s.entries[i]
		e.issues, e.name = m.Validate(), m.Name
		e.floors = m.FloorCount()
		if e.name == "" {
			e.name = "Untitled map"
		}
		e.status = ui.Numbers[m.Cols] + " × " + ui.Numbers[m.Rows]
		if len(e.issues) > 0 {
			e.status += " · Draft"
		} else {
			e.status += " · Ready"
		}
	}
	if s.selected >= len(s.entries) {
		s.selected = len(s.entries) - 1
	}
	if len(s.entries) == 0 && s.store.LoadErr == nil {
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

func (s *mapsScene) saveAt(a *app, index int) func(core.MapDefinition) error {
	return func(m core.MapDefinition) error {
		selected := index >= 0 && a.set.selectedMapIndex(s.store.Maps) == index
		if err := s.store.Put(index, m); err != nil {
			return err
		}
		if selected && len(m.Validate()) == 0 {
			s.applyMap(a, &m)
		}
		if index < 0 {
			index = len(s.store.Maps) - 1
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
	a.next = newMapEditor(a, s, m.Clone(), s.saveAt(a, index))
}

func (s *mapsScene) choose(a *app, m *core.MapDefinition) {
	s.applyMap(a, m)
	a.next = s.back
}

func (s *mapsScene) applyMap(a *app, m *core.MapDefinition) {
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
}

func (s *mapsScene) layout(a *app) {
	s.frame(a, true)
	v, u := s.view.Viewport, a.u
	cols := max(1, min(3, int(v.W/(132*u))))
	buttonW := (v.W - float64(cols-1)*8*u) / float64(cols)
	actionRows := (len(s.actions) + cols - 1) / cols
	h := 56 * u
	if s.draft != nil {
		h += 56 * u
	}
	noticeH := s.notice.layout(a, v.W)
	h += noticeH + float64(len(s.rows))*112*u
	if s.selected >= 0 {
		h += float64(actionRows)*52*u + 12*u
	}
	y := s.content(h)
	s.toolbar = [2]ui.Rect{{X: v.X, Y: y, W: (v.W - 8*u) / 2, H: 40 * u}, {X: v.Cx() + 4*u, Y: y, W: (v.W - 8*u) / 2, H: 40 * u}}
	y += 56 * u
	s.recovery = [2]ui.Rect{}
	if s.draft != nil {
		s.recovery = [2]ui.Rect{{X: v.X, Y: y, W: (v.W - 8*u) / 2, H: 40 * u}, {X: v.Cx() + 4*u, Y: y, W: (v.W - 8*u) / 2, H: 40 * u}}
		y += 56 * u
	}
	s.noticeY = y
	y += noticeH
	for i := range s.rows {
		s.rows[i] = ui.Rect{X: v.X, Y: y, W: v.W, H: 100 * u}
		y += 112 * u
		if i == s.selected {
			for j := range s.actions {
				s.actions[j] = ui.Rect{X: v.X + float64(j%cols)*(buttonW+8*u), Y: y + float64(j/cols)*52*u, W: buttonW, H: 40 * u}
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
		i := newMapImport(a, s, s.saveAt(a, -1))
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
			} else if err := s.store.Remove(s.deleteIndex); err != nil {
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
	case a.clicked(ui.CompactHit(s.toolbar[0], a.u)):
		a.next = newMapImport(a, s, s.saveAt(a, -1))
	case a.clicked(ui.CompactHit(s.toolbar[1], a.u)):
		s.choose(a, nil)
	case s.draft != nil && a.clicked(ui.CompactHit(s.recovery[0], a.u)):
		s.edit(a, *s.draft, -1)
	case s.draft != nil && a.clicked(ui.CompactHit(s.recovery[1], a.u)):
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
		m := s.store.Maps[s.selected]
		for action, r := range s.actions {
			if !a.clicked(ui.CompactHit(r, a.u)) {
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
				if err := s.store.Put(-1, m); err != nil {
					s.notice.set(err.Error())
				} else {
					s.selected = len(s.store.Maps) - 1
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
	ui.DrawButton(b, a.ui(), s.toolbar[0], "Import", "", ui.ButtonTile, true)
	ui.DrawButton(b, a.ui(), s.toolbar[1], "Classic board", "", ui.ButtonTile, true)
	if s.draft != nil {
		ui.DrawButton(b, a.ui(), s.recovery[0], "Recover draft", "", ui.ButtonTile, true)
		ui.DrawButton(b, a.ui(), s.recovery[1], "Discard draft", "", ui.ButtonTile, true)
	}
	s.notice.draw(b, a, s.view.Viewport.X, s.noticeY)
	for i, r := range s.rows {
		if r.Bottom() < s.view.Viewport.Y || r.Y > s.view.Viewport.Bottom() {
			continue
		}
		e, m := &s.entries[i], s.store.Maps[i]
		ui.FillRound(b, r, 16*u, a.sc.Tile)
		if s.selected == i {
			ui.StrokeRound(b, r, 16*u, 2.5*u, a.sc.Ink)
		}
		thumb := ui.Rect{X: r.X + 12*u, Y: r.Y + 12*u, W: 76 * u, H: 76 * u}
		if e.thumb == nil || e.thumbMode != a.sc || e.thumbSize != int(thumb.W) || e.thumbMarks != a.set.Marks {
			if e.thumb != nil {
				e.thumb.Deallocate()
			}
			e.thumbSize, e.thumbMode = int(thumb.W), a.sc
			e.thumbMarks = a.set.Marks
			e.thumb = ebiten.NewImage(max(1, e.thumbSize), max(1, e.thumbSize))
			drawMapThumb(e.thumb, a, m, ui.Rect{X: 0, Y: 0, W: thumb.W, H: thumb.H})
		}
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(thumb.X, thumb.Y)
		b.DrawImage(e.thumb, &op)
		x, width := thumb.Right()+12*u, r.Right()-thumb.Right()-24*u
		if e.nameWidth != width || e.nameScale != u {
			e.nameWidth, e.nameScale, e.nameShown = width, u, e.name
			if ui.TextWidth(e.nameShown, a.f.Heading) > width {
				for len(e.nameShown) > 0 && ui.TextWidth(e.nameShown+"…", a.f.Heading) > width {
					_, size := utf8.DecodeLastRuneInString(e.nameShown)
					e.nameShown = e.nameShown[:len(e.nameShown)-size]
				}
				e.nameShown += "…"
			}
		}
		ui.DrawText(b, e.nameShown, a.f.Heading, x, r.Y+36*u, text.AlignStart, a.sc.Ink)
		status := e.status
		if len(e.issues) == 0 && a.set.Players > e.floors {
			status = "Too small for these players"
		}
		ui.DrawText(b, status, a.f.Label, x, r.Y+66*u, text.AlignStart, a.sc.Dim)
	}
	if s.selected >= 0 {
		labels := [...]string{"Use map", "Edit", "Rename", "Duplicate", "Delete", "Export", "Share"}
		for i, label := range labels {
			enabled := true
			if i == 0 {
				enabled = len(s.entries[s.selected].issues) == 0 && a.set.Players <= s.store.Maps[s.selected].FloorCount()
			}
			ui.DrawButton(b, a.ui(), s.actions[i], label, "", ui.ButtonTile, enabled)
		}
	}
	ui.DrawButton(dst, a.ui(), s.action, "New map", "", ui.ButtonInk, true)
	if s.confirm != nil {
		s.confirm.draw(dst, a)
	}
}

// Libraries, setup and sharing preview the same map pieces as the playing board.
func drawMapThumb(dst *ebiten.Image, a *app, m core.MapDefinition, r ui.Rect) {
	if m.ValidateStructure() != nil {
		return
	}
	cell := min(r.W/float64(m.Cols), r.H/float64(m.Rows))
	ox, oy := r.Cx()-float64(m.Cols)*cell/2, r.Cy()-float64(m.Rows)*cell/2
	boardview.DrawMapArt(dst, a.sc, boardview.MapState{Map: &m}, boardview.MapTransform{X: ox, Y: oy, Cell: cell}, true, a.set.Marks)
}
