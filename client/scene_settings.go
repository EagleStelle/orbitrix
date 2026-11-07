package client

import (
	"math"
	"runtime"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
)

// Settings tabs and their rows. New groups get a tab; new options get a row.
var settingsTabs = [...]string{"Video", "Audio", "Network"}

const (
	tabVideo = iota
	tabAudio
	tabNetwork // last, so a build that plays with nobody else drops it (tabCount)
)

// tabCount is how many tabs show: Network only where the game plays on Same Wi-Fi or online.
func tabCount() int {
	if len(settingRows[tabNetwork]) == 0 {
		return tabNetwork
	}
	return len(settingsTabs)
}

type rowKind int

const (
	rowToggle rowKind = iota
	rowSlider
	rowTheme
	rowAccess // Public or Private, the access a new room starts with
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
	optRelay
	optWifi
	optOnline
)

var settingRows = [len(settingsTabs)][]settingRow{
	// Phones are always full screen, so they have no Fullscreen row.
	tabVideo: slices.DeleteFunc([]settingRow{{"Theme", rowTheme, optTheme}, {"Fullscreen", rowToggle, optFullscreen},
		{"Colorblind mode", rowToggle, optMarks}}, func(r settingRow) bool {
		return r.id == optFullscreen && !platform.CanFullscreen
	}),
	tabAudio: {{"Sound", rowToggle, optSound}, {"Volume", rowSlider, optVolume}},
	tabNetwork: slices.DeleteFunc([]settingRow{{"Same Wi-Fi rooms", rowAccess, optWifi},
		{"Online rooms", rowAccess, optOnline}, {"Force relay", rowToggle, optRelay}}, func(r settingRow) bool {
		return r.id == optWifi && runtime.GOOS == "js" || r.id != optWifi && origin == ""
	}),
}

// settingsMenu is the Settings page: tabs over one block of rows, each a label and its control. It returns to the
// screen that opened it.
type settingsMenu struct {
	back scene
	tab  int
	drag bool // dragging the volume slider

	// Layout in device px, recomputed when the screen size or tab changes.
	lv             ui.Rect
	lt             int
	backBtn        ui.Rect
	titleX, titleY float64
	tabs           [len(settingsTabs)]ui.Rect
	panel          ui.Rect
	rows           [3]ui.Rect

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
	if v.W < 600*u || v.H < 640*u {
		pad, bp, tabGap, panelGap, rowH = 16*u, 16*u, 16*u, 12*u, 56*u
	}
	w := v.W - 2*pad // full width, like a game's settings screen
	x := v.X + pad
	s.backBtn = ui.BackButton(x, v.Y+pad, w, u)
	s.titleX, s.titleY = s.backBtn.Right()+16*u, s.backBtn.Cy()

	ty := s.backBtn.Bottom() + tabGap
	for i := range tabCount() {
		s.tabs[i] = ui.Rect{X: x + float64(i)*136*u, Y: ty, W: 124 * u, H: 44 * u}
	}
	n := len(settingRows[s.tab])
	top := ty + 44*u + panelGap
	s.panel = ui.Rect{X: x, Y: top, W: w, H: max(float64(n)*rowH+2*bp, v.Bottom()-pad-top)} // down to the foot of the page
	for i := range n {
		s.rows[i] = ui.Rect{X: x + bp, Y: s.panel.Y + bp + float64(i)*rowH, W: w - 2*bp, H: rowH}
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
	case optRelay:
		return a.set.Relay
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
	case optRelay:
		a.set.Relay = !a.set.Relay
	}
	a.set.save()
}

// sliderTrack is the volume slider's track inside row r.
func (s *settingsMenu) sliderTrack(a *app, r ui.Rect) ui.Rect {
	u := a.u
	w := min(480*u, r.W*0.4)
	return ui.Rect{X: r.Right() - 64*u - w, Y: r.Cy() - 4*u, W: w, H: 8 * u}
}

// private reads whether new rooms of the kind row id names start private.
func (s *settingsMenu) private(a *app, id int) bool {
	if id == optWifi {
		return a.set.WifiPrivate
	}
	return !a.set.OnlinePublic
}

// themeOptions are the Light and Dark choices inside row r.
func (s *settingsMenu) themeOptions(a *app, r ui.Rect) [2]ui.Rect {
	u := a.u
	w, h := 104*u, ui.CompactButtonHeight*u
	return [2]ui.Rect{{X: r.Right() - 2*w - 8*u, Y: r.Cy() - h/2, W: w, H: h}, {X: r.Right() - w, Y: r.Cy() - h/2, W: w, H: h}}
}

func (s *settingsMenu) update(a *app) error {
	s.layout(a)
	if a.net != nil {
		a.net.pump(a) // an online game goes on meanwhile; the game scene plays what came in on return
	}
	if a.in.esc || a.clicked(ui.CompactHit(s.backBtn, a.u)) {
		s.close(a)
		return nil
	}
	for i, t := range s.tabs[:tabCount()] {
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
				if a.clicked(ui.CompactHit(o, a.u)) && a.sc != &ui.Palettes[t] {
					a.toggleTheme()
				}
			}
		case rowAccess:
			for t, o := range s.themeOptions(a, r) {
				if a.clicked(ui.CompactHit(o, a.u)) {
					if row.id == optWifi {
						a.set.WifiPrivate = t == 1
					} else {
						a.set.OnlinePublic = t == 0
					}
					a.set.save()
				}
			}
		case rowSlider:
			tr := s.sliderTrack(a, r)
			hit := ui.Rect{X: tr.X - 14*a.u, Y: r.Y, W: tr.W + 28*a.u, H: r.H}
			if a.hover(hit) && a.in.pressed { // on press, so a finger can drag the knob
				s.drag = true
			}
			if s.drag {
				a.pointer = true
				v := int(math.Round((a.in.x-tr.X)/tr.W*20)) * 5 // steps of 5%
				a.set.Volume = max(0, min(100, v))
				a.snd.Volume = float64(a.set.Volume) / 100
				if !a.in.down {
					s.drag = false
					a.set.save()
					a.snd.Place(0, 0) // a sample note at the new volume
				}
			}
		}
	}
	return nil
}

func (s *settingsMenu) draw(dst *ebiten.Image, a *app) {
	s.layout(a)
	sc, f, u := a.sc, &a.f, a.u
	ui.DrawBack(dst, a.ui(), s.backBtn)
	ui.DrawText(dst, "Settings", f.Title, s.titleX, s.titleY, text.AlignStart, sc.Ink)

	// The chosen tab is outlined in ink; the others are quiet pills on the mat.
	for i, t := range s.tabs[:tabCount()] {
		style := ui.ButtonTile
		if i == s.tab {
			style = ui.ButtonChosen
		}
		ui.DrawButton(dst, a.ui(), t, settingsTabs[i], "", style, true)
	}
	ui.FillRound(dst, s.panel, 24*u, sc.Tile)
	for i, row := range settingRows[s.tab] {
		r := s.rows[i]
		// The hovered row lights up across the full width so the eye can follow it from label to control.
		if a.over(r) {
			ui.FillRound(dst, ui.Rect{X: r.X - 12*u, Y: r.Y + 4*u, W: r.W + 24*u, H: r.H - 8*u}, 14*u, sc.TileHi)
		} else if i > 0 && !a.over(s.rows[i-1]) {
			ui.FillRound(dst, ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 1.5 * u}, 0, ui.Mix(sc.Tile, sc.Dim, 0.3))
		}
		ui.DrawText(dst, row.label, f.BodyBold, r.X, r.Cy(), text.AlignStart, sc.Ink)
		switch row.kind {
		case rowToggle:
			drawToggle(dst, a, ui.Rect{X: r.Right() - 60*u, Y: r.Cy() - 17*u, W: 60 * u, H: 34 * u}, s.get(a, row.id), a.over(r))
		case rowTheme:
			opts := s.themeOptions(a, r)
			for t, o := range opts {
				style := ui.ButtonGround
				if a.sc == &ui.Palettes[t] {
					style = ui.ButtonChosen
				}
				ui.DrawButton(dst, a.ui(), o, [2]string{"Light", "Dark"}[t], "", style, true)
			}
		case rowAccess:
			for t, o := range s.themeOptions(a, r) {
				style := ui.ButtonGround
				if s.private(a, row.id) == (t == 1) {
					style = ui.ButtonChosen
				}
				ui.DrawButton(dst, a.ui(), o, accessNames[t], "", style, true)
			}
		case rowSlider:
			s.drawSlider(dst, a, r)
		}
	}
}

// drawToggle draws an on/off switch: an ink track with the knob right when on; when off, a ground track with a dim
// outline (so its edge reads on the panel) and the knob left. Hover outlines it in ink.
func drawToggle(dst *ebiten.Image, a *app, r ui.Rect, on, hot bool) {
	sc, u := a.sc, a.u
	if on {
		ui.FillPill(dst, r, sc.Ink)
		ui.FillCircle(dst, r.Right()-r.H/2, r.Cy(), r.H/2-5*u, sc.Ground)
		return
	}
	edge := sc.Dim
	if hot {
		edge = sc.Ink
	}
	ui.FillPill(dst, r, sc.Ground)
	ui.StrokeRound(dst, r, r.H/2, 1.5*u, edge)
	ui.FillCircle(dst, r.X+r.H/2, r.Cy(), r.H/2-6*u, sc.Dim)
}

// drawSlider draws the volume slider, in dim ink while sound is off.
func (s *settingsMenu) drawSlider(dst *ebiten.Image, a *app, r ui.Rect) {
	sc, u := a.sc, a.u
	ink := sc.Ink
	if a.set.Muted {
		ink = sc.Dim
	}
	tr := s.sliderTrack(a, r)
	v := float64(a.set.Volume) / 100
	ui.FillPill(dst, tr, sc.Ground)
	if v > 0 {
		ui.FillPill(dst, ui.Rect{X: tr.X, Y: tr.Y, W: max(tr.H, tr.W*v), H: tr.H}, ink)
	}
	ui.FillCircle(dst, tr.X+tr.W*v, tr.Cy(), 12*u, ink)
	if s.volN != a.set.Volume {
		s.volN, s.volText = a.set.Volume, strconv.Itoa(a.set.Volume)+"%"
	}
	ui.DrawText(dst, s.volText, a.f.BodyBold, r.Right(), r.Cy(), text.AlignEnd, ink)
}
