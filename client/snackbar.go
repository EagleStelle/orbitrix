package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

const snackH, snackIn, snackGap = 56, 12, 12

// snackbar is a screen overlay. Its position never depends on the page's content or scroll offset.
type snackbar struct {
	message, more string
	actions       [2]string
	ticks         int
	avoid         rect // a fixed action row to keep clear, or zero
}

func (s *snackbar) frame(a *app) (bar rect, buttons [2]rect, opacity float64) {
	v, u := a.safe(), a.u
	w := min(v.w-32*u, 560*u)
	limit := v.bottom()
	if s.avoid.h > 0 {
		limit = min(limit, s.avoid.y)
	}
	bar = rect{v.cx() - w/2, limit - (snackH+snackGap)*u, w, snackH * u}
	buttons[1] = rect{bar.right() - 104*u, bar.cy() - 20*u, 96 * u, 40 * u}
	buttons[0] = rect{buttons[1].x - 68*u, buttons[1].y, 64 * u, 40 * u}
	p := min(1, float64(s.ticks)/snackIn)
	opacity = 1 - (1-p)*(1-p)*(1-p)
	dy := min((1-opacity)*16*u, limit-bar.bottom())
	bar.y += dy
	for i := range buttons {
		buttons[i].y += dy
	}
	return
}

// update reports the chosen action, or -1, and consumes the pointer over the overlay.
func (s *snackbar) update(a *app) (action int, over bool) {
	s.ticks = min(snackIn, s.ticks+1)
	bar, buttons, _ := s.frame(a)
	action = -1
	for i, label := range s.actions {
		if label != "" && a.clicked(compactHit(buttons[i], a.u)) {
			action = i
		}
	}
	return action, a.over(bar)
}

func (s *snackbar) draw(dst *ebiten.Image, a *app) {
	bar, buttons, opacity := s.frame(a)
	sc, f, u := a.sc, &a.f, a.u
	fillPill(dst, bar, withAlpha(sc.ink, opacity))
	drawText(dst, s.message, f.small, bar.x+24*u, bar.cy(), text.AlignStart, withAlpha(sc.ground, opacity))
	if s.more != "" {
		x := bar.x + 24*u + textWidth(s.message, f.small) + 10*u
		drawText(dst, s.more, f.small, x, bar.cy(), text.AlignStart, withAlpha(sc.ground, 0.6*opacity))
	}
	for i, label := range s.actions {
		if label == "" {
			continue
		}
		b, ink := buttons[i], sc.ground
		if i == 1 {
			fill := sc.ground
			if a.over(compactHit(b, u)) {
				fill = sc.tileHi
			}
			fillPill(dst, b, withAlpha(fill, opacity))
			ink = sc.ink
		} else if a.over(compactHit(b, u)) {
			fillPill(dst, b, withAlpha(sc.ground, 0.16*opacity))
		}
		drawText(dst, label, f.labelBold, b.cx(), b.cy(), text.AlignCenter, withAlpha(ink, opacity))
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
