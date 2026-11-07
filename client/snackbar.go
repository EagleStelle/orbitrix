package client

import (
	"orbitrix/client/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

const snackH, snackIn, snackGap = 56, 12, 12
const snackDuration = 5 * 60

// snackbar is a screen overlay. Its position never depends on the page's content or scroll offset.
type snackbar struct {
	message, more string
	actions       [2]string
	ticks         int
	timeout, age  int     // timeout 0 keeps the snackbar visible until its owner dismisses it
	avoid         ui.Rect // a fixed action row to keep clear, or zero
}

func (s *snackbar) frame(a *app) (bar ui.Rect, buttons [2]ui.Rect, opacity float64) {
	v, u := a.safe(), a.u
	w := min(v.W-32*u, 560*u)
	limit := v.Bottom()
	if s.avoid.H > 0 {
		limit = min(limit, s.avoid.Y)
	}
	height := snackH * u
	if s.actions == [2]string{} && s.more == "" {
		height = max(height, float64(len(wrap(s.message, a.f.Small, max(1, w-48*u))))*18*u+24*u)
	}
	bar = ui.Rect{X: v.Cx() - w/2, Y: limit - height - snackGap*u, W: w, H: height}
	buttons[1] = ui.Rect{X: bar.Right() - 104*u, Y: bar.Cy() - 20*u, W: 96 * u, H: 40 * u}
	buttons[0] = ui.Rect{X: buttons[1].X - 68*u, Y: buttons[1].Y, W: 64 * u, H: 40 * u}
	p := min(1, float64(s.ticks)/snackIn)
	opacity = 1 - (1-p)*(1-p)*(1-p)
	dy := min((1-opacity)*16*u, limit-bar.Bottom())
	bar.Y += dy
	for i := range buttons {
		buttons[i].Y += dy
	}
	return
}

// update reports the chosen action, or -1, and consumes the pointer over the overlay.
func (s *snackbar) update(a *app) (action int, over bool) {
	if s.timeout > 0 {
		s.age = min(s.timeout, s.age+1)
	}
	if s.expired() {
		return -1, false
	}
	s.ticks = min(snackIn, s.ticks+1)
	bar, buttons, _ := s.frame(a)
	action = -1
	for i, label := range s.actions {
		if label != "" && a.clicked(ui.CompactHit(buttons[i], a.u)) {
			action = i
		}
	}
	return action, a.over(bar)
}

func (s *snackbar) expired() bool { return s.timeout > 0 && s.age >= s.timeout }

func (s *snackbar) draw(dst *ebiten.Image, a *app) {
	if s.expired() {
		return
	}
	bar, buttons, opacity := s.frame(a)
	sc, f, u := a.sc, &a.f, a.u
	ui.FillPill(dst, bar, ui.WithAlpha(sc.Ink, opacity))
	if s.actions == [2]string{} && s.more == "" {
		lines := wrap(s.message, f.Small, max(1, bar.W-48*u))
		for i, l := range lines {
			ui.DrawText(dst, l, f.Small, bar.X+24*u, bar.Cy()+(float64(i)-float64(len(lines)-1)/2)*18*u, text.AlignStart, ui.WithAlpha(sc.Ground, opacity))
		}
	} else {
		ui.DrawText(dst, s.message, f.Small, bar.X+24*u, bar.Cy(), text.AlignStart, ui.WithAlpha(sc.Ground, opacity))
	}
	if s.more != "" {
		x := bar.X + 24*u + ui.TextWidth(s.message, f.Small) + 10*u
		ui.DrawText(dst, s.more, f.Small, x, bar.Cy(), text.AlignStart, ui.WithAlpha(sc.Ground, 0.6*opacity))
	}
	for i, label := range s.actions {
		if label == "" {
			continue
		}
		b, ink := buttons[i], sc.Ground
		if i == 1 {
			fill := sc.Ground
			if a.over(ui.CompactHit(b, u)) {
				fill = sc.TileHi
			}
			ui.FillPill(dst, b, ui.WithAlpha(fill, opacity))
			ink = sc.Ink
		} else if a.over(ui.CompactHit(b, u)) {
			ui.FillPill(dst, b, ui.WithAlpha(sc.Ground, 0.16*opacity))
		}
		ui.DrawText(dst, label, f.LabelBold, b.Cx(), b.Cy(), text.AlignCenter, ui.WithAlpha(ink, opacity))
	}
}

// drawSnackbarOverlay runs after the scene, including its dialogs, has finished drawing.
func drawSnackbarOverlay(dst *ebiten.Image, a *app, current scene) {
	if bar := sceneSnackbar(current); bar != nil {
		bar.draw(dst, a)
	}
}

func sceneSnackbar(current scene) *snackbar {
	if source, ok := current.(interface{ snackbar() *snackbar }); ok {
		return source.snackbar()
	}
	return nil
}
