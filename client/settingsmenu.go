package client

import (
	"math"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/platform"
)

// Settings tabs and their rows. New groups get a tab; new options get a row.
var settingsTabs = [...]string{"Video", "Audio"}

const (
	tabVideo = iota
	tabAudio
)

type rowKind int

const (
	rowToggle rowKind = iota
	rowSlider
	rowTheme
)

type settingRow struct {
	label string
	kind  rowKind
	id    int // which setting, see settingsMenu.toggle
}

const (
	optSound = iota
	optVolume
	optTheme
	optFullscreen
	optMarks
)

var settingRows = [len(settingsTabs)][]settingRow{
	// Phones are always full screen, so they have no Fullscreen row.
	tabVideo: slices.DeleteFunc([]settingRow{{"Theme", rowTheme, optTheme}, {"Fullscreen", rowToggle, optFullscreen},
		{"Colorblind mode", rowToggle, optMarks}}, func(r settingRow) bool {
		return r.id == optFullscreen && !platform.CanFullscreen
	}),
	tabAudio: {{"Sound", rowToggle, optSound}, {"Volume", rowSlider, optVolume}},
}

// settingsMenu is the Settings page: tabs over one block of rows, each a label and its control. It returns to the
// screen that opened it.
type settingsMenu struct {
	back scene
	tab  int
	drag bool // dragging the volume slider

	// Layout in device px, recomputed when the screen size or tab changes.
	lv             rect
	lt             int
	backBtn        rect
	titleX, titleY float64
	tabs           [len(settingsTabs)]rect
	panel          rect
	rows           [3]rect

	volText string
	volN    int
}

func newSettingsMenu(back scene) *settingsMenu { return &settingsMenu{back: back, volN: -1} }

func (s *settingsMenu) layout(a *app) {
	v := a.safe()
	if v == s.lv && s.tab == s.lt {
		return
	}
	s.lv, s.lt = v, s.tab
	u := a.u
	// The same Back pill and title placement as Setup, inside this page's wider margin. Phones get Setup's compact
	// margins and shorter rows that still clear the touch target.
	pad, bp, tabGap, panelGap, rowH := 40*u, 24*u, 28*u, 20*u, 76*u
	if v.w < 600*u || v.h < 640*u {
		pad, bp, tabGap, panelGap, rowH = 16*u, 16*u, 16*u, 12*u, 56*u
	}
	w := v.w - 2*pad // full width, like a game's settings screen
	x := v.x + pad
	s.backBtn = backButton(x, v.y+pad, w, u)
	s.titleX, s.titleY = s.backBtn.right()+16*u, s.backBtn.cy()

	ty := s.backBtn.bottom() + tabGap
	for i := range s.tabs {
		s.tabs[i] = rect{x + float64(i)*136*u, ty, 124 * u, 44 * u}
	}
	n := len(settingRows[s.tab])
	top := ty + 44*u + panelGap
	s.panel = rect{x, top, w, max(float64(n)*rowH+2*bp, v.bottom()-pad-top)} // down to the foot of the page
	for i := range n {
		s.rows[i] = rect{x + bp, s.panel.y + bp + float64(i)*rowH, w - 2*bp, rowH}
	}
}

func (s *settingsMenu) close(a *app) {
	a.set.save()
	a.next = s.back
}

// get and set read and change a toggle's setting.
func (s *settingsMenu) get(a *app, id int) bool {
	switch id {
	case optSound:
		return !a.set.Muted
	case optFullscreen:
		return ebiten.IsFullscreen() // a browser also leaves fullscreen on its own Esc
	case optMarks:
		return a.set.Marks
	}
	return false
}

func (s *settingsMenu) flip(a *app, id int) {
	switch id {
	case optSound:
		a.toggleMute()
	case optFullscreen:
		a.set.Fullscreen = !ebiten.IsFullscreen()
		ebiten.SetFullscreen(a.set.Fullscreen)
	case optMarks:
		a.set.Marks = !a.set.Marks
	}
	a.set.save()
}

// sliderTrack is the volume slider's track inside row r.
func (s *settingsMenu) sliderTrack(a *app, r rect) rect {
	u := a.u
	w := min(480*u, r.w*0.4)
	return rect{r.right() - 64*u - w, r.cy() - 4*u, w, 8 * u}
}

// themeOptions are the Light and Dark choices inside row r.
func (s *settingsMenu) themeOptions(a *app, r rect) [2]rect {
	u := a.u
	w, h := 104*u, compactButtonHeight*u
	return [2]rect{{r.right() - 2*w - 8*u, r.cy() - h/2, w, h}, {r.right() - w, r.cy() - h/2, w, h}}
}

func (s *settingsMenu) update(a *app) error {
	s.layout(a)
	if a.in.esc || a.clicked(compactHit(s.backBtn, a.u)) {
		s.close(a)
		return nil
	}
	for i, t := range s.tabs {
		if a.clicked(t) {
			s.tab = i
			s.layout(a)
			return nil
		}
	}
	for i, row := range settingRows[s.tab] {
		r := s.rows[i]
		switch row.kind {
		case rowToggle:
			if a.clicked(r) {
				s.flip(a, row.id)
			}
		case rowTheme:
			for t, o := range s.themeOptions(a, r) {
				if a.clicked(compactHit(o, a.u)) && a.sc != &schemes[t] {
					a.toggleTheme()
				}
			}
		case rowSlider:
			tr := s.sliderTrack(a, r)
			hit := rect{tr.x - 14*a.u, r.y, tr.w + 28*a.u, r.h}
			if a.hover(hit) && a.in.pressed { // on press, so a finger can drag the knob
				s.drag = true
			}
			if s.drag {
				a.pointer = true
				v := int(math.Round((a.in.x-tr.x)/tr.w*20)) * 5 // steps of 5%
				a.set.Volume = max(0, min(100, v))
				a.snd.volume = float64(a.set.Volume) / 100
				if !a.in.down {
					s.drag = false
					a.set.save()
					a.snd.place(0) // a sample note at the new volume
				}
			}
		}
	}
	return nil
}

func (s *settingsMenu) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u

	drawBack(dst, a, s.backBtn)
	drawText(dst, "Settings", f.title, s.titleX, s.titleY, text.AlignStart, sc.ink)

	// The chosen tab is outlined in ink; the others are quiet pills on the mat.
	for i, t := range s.tabs {
		style := btnTile
		if i == s.tab {
			style = btnChosen
		}
		drawButton(dst, a, t, settingsTabs[i], "", style, true)
	}

	fillRound(dst, s.panel, 24*u, sc.tile)
	for i, row := range settingRows[s.tab] {
		r := s.rows[i]
		// The hovered row lights up across the full width so the eye can follow it from label to control.
		if a.over(r) {
			fillRound(dst, rect{r.x - 12*u, r.y + 4*u, r.w + 24*u, r.h - 8*u}, 14*u, sc.tileHi)
		} else if i > 0 && !a.over(s.rows[i-1]) {
			fillRound(dst, rect{r.x, r.y, r.w, 1.5 * u}, 0, mix(sc.tile, sc.dim, 0.3))
		}
		drawText(dst, row.label, f.bodyBold, r.x, r.cy(), text.AlignStart, sc.ink)
		switch row.kind {
		case rowToggle:
			drawToggle(dst, a, rect{r.right() - 60*u, r.cy() - 17*u, 60 * u, 34 * u}, s.get(a, row.id), a.over(r))
		case rowTheme:
			opts := s.themeOptions(a, r)
			for t, o := range opts {
				style := btnGround
				if a.sc == &schemes[t] {
					style = btnChosen
				}
				drawButton(dst, a, o, [2]string{"Light", "Dark"}[t], "", style, true)
			}
		case rowSlider:
			s.drawSlider(dst, a, r)
		}
	}
}

// drawToggle draws an on/off switch: an ink track with the knob right when on; when off, a ground track with a dim
// outline (so its edge reads on the panel) and the knob left. Hover outlines it in ink.
func drawToggle(dst *ebiten.Image, a *app, r rect, on, hot bool) {
	sc, u := a.sc, a.u
	if on {
		fillPill(dst, r, sc.ink)
		fillCircle(dst, r.right()-r.h/2, r.cy(), r.h/2-5*u, sc.ground)
		return
	}
	edge := sc.dim
	if hot {
		edge = sc.ink
	}
	fillPill(dst, r, sc.ground)
	strokeRound(dst, r, r.h/2, 1.5*u, edge)
	fillCircle(dst, r.x+r.h/2, r.cy(), r.h/2-6*u, sc.dim)
}

// drawSlider draws the volume slider, in dim ink while sound is off.
func (s *settingsMenu) drawSlider(dst *ebiten.Image, a *app, r rect) {
	sc, u := a.sc, a.u
	ink := sc.ink
	if a.set.Muted {
		ink = sc.dim
	}
	tr := s.sliderTrack(a, r)
	v := float64(a.set.Volume) / 100
	fillPill(dst, tr, sc.ground)
	if v > 0 {
		fillPill(dst, rect{tr.x, tr.y, max(tr.h, tr.w*v), tr.h}, ink)
	}
	fillCircle(dst, tr.x+tr.w*v, tr.cy(), 12*u, ink)
	if s.volN != a.set.Volume {
		s.volN, s.volText = a.set.Volume, strconv.Itoa(a.set.Volume)+"%"
	}
	drawText(dst, s.volText, a.f.bodyBold, r.right(), r.cy(), text.AlignEnd, ink)
}
