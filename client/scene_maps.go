package client

import (
	"cmp"
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
	dims       string // "cols × rows", its badge
	unsaved    bool   // the recovered draft
	name       string
	nameShown  string
	nameWidth  float64
	nameScale  float64
	thumb      *ebiten.Image
	thumbMode  *ui.Palette
	thumbSize  int
	thumbMarks bool
}

// What the picker has selected besides a saved map's index: nothing, the classic map (the plain board Setup sizes),
// or the recovered draft.
const (
	selNone    = -1
	selClassic = -2
	selDraft   = -3
)

// The bottom bar's buttons, left to right: two for the library, then the selected map's.
const (
	barNew = iota
	barImport
	barEdit
	barCopy
	barShare
	barExport
	barDelete
	barUse
	barCount
)

// unsavedChip marks the recovered draft's tile.
var unsavedChip = ui.Chip{Word: "Unsaved", Kind: ui.ChipOn}

var barIcons = [barCount]int{iconNew, iconImport, iconEdit, iconCopy, iconShare, iconExport, iconDelete, iconUse}

// mapsScene is the map picker Home and Setup share: a grid of map tiles (the classic map first, then a recovered
// draft, then the saved maps) over one bar of round icon buttons. New and Import stand at the bar's left; the
// selected map's actions at its right, dimmed where they can't act.
type mapsScene struct {
	page
	back        scene
	pick        func(*core.MapDefinition)
	store       *mapstore.Store
	entries     []libraryEntry
	selected    int
	rows        []ui.Rect
	classic     libraryEntry // the plain board at Setup's size
	classicMap  core.MapDefinition
	classicR    ui.Rect
	draftE      libraryEntry
	draftR      ui.Rect
	bar         [barCount]ui.Rect
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
	s := &mapsScene{back: back, pick: pick, selected: selClassic, store: mapstore.Load(platform.Load, platform.Save)}
	if s.store.LoadErr != nil {
		s.notice.set("Your saved maps could not be read. The existing library will remain untouched. " + s.store.LoadErr.Error())
	}
	if a.set.Map != nil { // the map in use starts selected
		s.selected = a.set.selectedMapIndex(s.store.Maps)
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
		e.issues, e.name, e.dims = m.Validate(), m.Name, ui.Numbers[m.Cols]+" × "+ui.Numbers[m.Rows]
		if e.name == "" {
			e.name = "Untitled map"
		}
	}
	if s.selected >= len(s.entries) {
		s.selected = len(s.entries) - 1
	}
}

func (s *mapsScene) readDraft() {
	s.reloadDraft, s.draft = false, nil
	if b, err := platform.Load("map-draft.orbitrixmap"); err == nil && len(b) > 0 {
		if m, err := core.DecodeMap(b); err == nil {
			s.draft = &m
			if s.draftE.thumb != nil {
				s.draftE.thumb.Deallocate()
			}
			s.draftE = libraryEntry{name: cmp.Or(m.Name, "Untitled map"), dims: ui.Numbers[m.Cols] + " × " + ui.Numbers[m.Rows], unsaved: true, issues: m.Validate()}
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
	if s.pick == nil { // opened from Home: on to Setup, whose Back returns here
		setup := newSetup(a)
		setup.backTo = s
		a.next = setup
	}
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

// layout sets the library out as a grid of map tiles, each a thumbnail over its name, as many a row as
// stay at least 148 px wide, above the bar of icon buttons.
func (s *mapsScene) layout(a *app) {
	if s.frame(a, true) {
		s.layoutBar(a)
	}
	v, u := s.view.Viewport, a.u
	if s.classicMap.Cols != a.set.Cols || s.classicMap.Rows != a.set.Rows {
		if s.classic.thumb != nil {
			s.classic.thumb.Deallocate()
		}
		s.classicMap = core.NewMap(a.set.Cols, a.set.Rows)
		s.classic = libraryEntry{name: "Classic", dims: ui.Numbers[a.set.Cols] + " × " + ui.Numbers[a.set.Rows]}
	}
	gap := 12 * u
	cols := max(1, int((v.W+gap)/(160*u)))
	tileW := (v.W - float64(cols-1)*gap) / float64(cols)
	tileH := tileW + 28*u
	tiles := 1 + len(s.rows)
	if s.draft != nil {
		tiles++
	}
	noticeH := s.notice.layout(a, v.W)
	y := s.content(noticeH + float64((tiles+cols-1)/cols)*(tileH+gap) - gap)
	s.noticeY = y
	y += noticeH
	n := 0
	place := func() ui.Rect {
		r := ui.Rect{X: v.X + float64(n%cols)*(tileW+gap), Y: y + float64(n/cols)*(tileH+gap), W: tileW, H: tileH}
		n++
		return r
	}
	s.classicR, s.draftR = place(), ui.Rect{}
	if s.draft != nil {
		s.draftR = place()
	}
	for i := range s.rows {
		s.rows[i] = place()
	}
}

// layoutBar sets the icon buttons in the page's action row: the library's two from the left, the selected map's
// six from the right. A screen too narrow for all eight at their smallest moves the library's two to a row below,
// so nothing shrinks further.
func (s *mapsScene) layoutBar(a *app) {
	u, r := a.u, s.action
	size, gap := ui.QuietButtonHeight*u, 8*u
	if barCount*size+(barCount-1)*gap > r.W {
		size, gap = ui.CompactButtonHeight*u, 4*u
	}
	y := r.Cy() - size/2
	top := y // the selected map's row
	if barCount*size+(barCount-1)*gap > r.W {
		top = y - size - 8*u
		s.view.Viewport.H = max(1, s.view.Viewport.H-size-8*u)
	}
	for i := range s.bar {
		s.bar[i] = ui.Rect{X: r.X + float64(i)*(size+gap), Y: y, W: size, H: size}
		if i >= barEdit {
			s.bar[i].X, s.bar[i].Y = r.Right()-float64(barCount-i)*(size+gap)+gap, top
		}
	}
}

// selectedMap is the saved map selected, if one is.
func (s *mapsScene) selectedMap() (core.MapDefinition, bool) {
	if s.selected < 0 || s.selected >= len(s.store.Maps) {
		return core.MapDefinition{}, false
	}
	return s.store.Maps[s.selected], true
}

// can reports whether bar button i acts: New and Import always; the classic map can only be used, a draft only
// edited or discarded, and a saved map used once it is ready and holds the players.
func (s *mapsScene) can(a *app, i int) bool {
	m, saved := s.selectedMap()
	switch {
	case i == barNew || i == barImport:
		return true
	case s.selected == selClassic:
		return i == barUse
	case s.selected == selDraft:
		return s.draft != nil && (i == barEdit || i == barDelete)
	case !saved:
		return false
	case i == barUse:
		return len(s.entries[s.selected].issues) == 0 && a.set.Players <= m.FloorCount()
	}
	return true
}

// act runs bar button i.
func (s *mapsScene) act(a *app, i int) {
	m, _ := s.selectedMap()
	switch i {
	case barNew:
		s.edit(a, core.NewMap(a.set.Cols, a.set.Rows), -1)
	case barImport:
		a.next = newMapImport(a, s, s.saveAt(a, -1))
	case barEdit:
		if s.selected == selDraft {
			s.edit(a, *s.draft, -1)
		} else {
			s.edit(a, m, s.selected)
		}
	case barCopy:
		m.Name = mapCopyName(m.Name)
		if err := s.store.Put(-1, m); err != nil {
			s.notice.set(err.Error())
		} else {
			s.selected = len(s.store.Maps) - 1
			s.refresh()
		}
	case barShare, barExport:
		a.next = newMapShare(a, s, m, i == barExport)
	case barDelete:
		if s.discard = s.selected == selDraft; s.discard {
			s.confirm = &askDialog{text: "Discard the recovered draft?", yes: "Discard draft"}
		} else {
			s.deleteIndex = s.selected
			s.confirm = &askDialog{text: "Delete this saved map?", yes: "Delete map"}
		}
	case barUse:
		if s.selected == selClassic {
			s.choose(a, nil)
		} else {
			s.choose(a, &m)
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
				s.refresh()
			}
			s.selected = selNone
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
	for i, r := range s.bar {
		if s.can(a, i) && a.clicked(ui.CompactHit(r, a.u)) {
			s.act(a, i)
			return nil
		}
	}
	if !s.inBody(a) {
		return nil
	}
	switch {
	case a.clicked(s.classicR):
		s.selected = selClassic
	case s.draft != nil && a.clicked(s.draftR):
		s.selected = selDraft
	default:
		for i, row := range s.rows {
			if a.clicked(row) {
				s.selected = i
			}
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
	b := s.body(dst)
	s.notice.draw(b, a, s.view.Viewport.X, s.noticeY)
	s.drawTile(b, a, s.classicR, &s.classic, s.classicMap, s.selected == selClassic)
	if s.draft != nil {
		s.drawTile(b, a, s.draftR, &s.draftE, *s.draft, s.selected == selDraft)
	}
	for i, r := range s.rows {
		s.drawTile(b, a, r, &s.entries[i], s.store.Maps[i], s.selected == i)
	}
	for i, r := range s.bar {
		style := ui.ButtonTile
		if i == barUse {
			style = ui.ButtonInk
		}
		iconButton(dst, a, r, barIcons[i], style, s.can(a, i))
	}
	if s.confirm != nil {
		s.confirm.draw(dst, a)
	}
}

// drawTile draws one map's tile: its cached thumbnail, badged with its size, over its name; outlined when selected.
func (s *mapsScene) drawTile(b *ebiten.Image, a *app, r ui.Rect, e *libraryEntry, m core.MapDefinition, selected bool) {
	if r.Bottom() < s.view.Viewport.Y || r.Y > s.view.Viewport.Bottom() {
		return
	}
	u := a.u
	ui.FillRound(b, r, 16*u, a.sc.Tile)
	if selected {
		ui.StrokeRound(b, r, 16*u, 2.5*u, a.sc.Ink)
	}
	thumb := ui.Rect{X: r.X + 12*u, Y: r.Y + 12*u, W: r.W - 24*u, H: r.W - 24*u}
	if e.thumb == nil || e.thumbMode != a.sc || e.thumbSize != int(thumb.W) || e.thumbMarks != a.set.Marks {
		if e.thumb != nil {
			e.thumb.Deallocate()
		}
		e.thumbSize, e.thumbMode = int(thumb.W), a.sc
		e.thumbMarks = a.set.Marks
		e.thumb = ebiten.NewImage(max(1, e.thumbSize), max(1, e.thumbSize))
		drawMapThumb(e.thumb, a, m, ui.Rect{X: 8 * u, Y: 8 * u, W: thumb.W - 16*u, H: thumb.H - 16*u})
	}
	ui.FillRound(b, thumb, 10*u, a.sc.Ground) // the mat under the map, as in Setup's preview
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(thumb.X, thumb.Y)
	b.DrawImage(e.thumb, &op)
	if e.nameWidth != thumb.W || e.nameScale != u {
		e.nameWidth, e.nameScale = thumb.W, u
		e.nameShown = ellipsize(e.name, a.f.LabelBold, thumb.W)
	}
	ui.DrawText(b, e.nameShown, a.f.LabelBold, thumb.X, thumb.Bottom()+20*u, text.AlignStart, a.sc.Ink)
	// Its size is a badge on the thumbnail's top right corner; a recovered draft is marked on the left.
	ch := 22 * u
	ui.DrawChip(b, a.ui(), ui.Chip{Word: e.dims, Kind: ui.ChipBusy}, thumb.Right()-6*u, thumb.Y+6*u+ch/2, ch)
	if e.unsaved {
		ui.DrawChip(b, a.ui(), unsavedChip, thumb.X+6*u+ui.ChipWidth(a.ui(), unsavedChip.Word, ch), thumb.Y+6*u+ch/2, ch)
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
