package client

import (
	"fmt"
	"testing"

	"orbitrix/core"
)

// Window and phone sizes in logical px: desktop, a phone on its side, upright phones and a tablet.
var layoutSizes = [][2]float64{{1040, 640}, {1920, 1080}, {844, 390}, {780, 360}, {390, 844}, {360, 740}, {412, 915},
	{768, 1024}}

// withCutout gives a a camera cutout on the screen's short edge: the top when upright, the left on its side.
func withCutout(a *app) *app {
	if a.h > a.w {
		a.ins[1] = 24 * a.u
	} else {
		a.ins[0] = 24 * a.u
	}
	return a
}

func overlaps(a, b rect) bool {
	return a.x < b.right() && b.x < a.right() && a.y < b.bottom() && b.y < a.bottom()
}

// checkParts fails unless every part sits inside v and no two parts overlap.
func checkParts(t *testing.T, v rect, parts map[string]rect) {
	t.Helper()
	for name, r := range parts {
		if !containsRect(v, r) {
			t.Fatalf("%s %+v leaves the safe area %+v", name, r, v)
		}
		for other, o := range parts {
			if name < other && overlaps(r, o) {
				t.Fatalf("%s %+v overlaps %s %+v", name, r, other, o)
			}
		}
	}
}

func TestPlayLayout(t *testing.T) {
	for _, size := range layoutSizes {
		for _, u := range []float64{1, 2.625} {
			for players := core.MinPlayers; players <= core.MaxPlayers; players++ {
				for _, board := range [][2]int{{9, 9}, {30, 18}} {
					t.Run(fmt.Sprintf("%.0fx%.0f/u%g/%dp/%dx%d", size[0], size[1], u, players, board[0], board[1]), func(t *testing.T) {
						a := withCutout(setupTestApp(size[0], size[1], u, players))
						p, err := newPlay(a, board[0], board[1], a.set.seats)
						if err != nil {
							t.Fatal(err)
						}
						p.layout(a)
						v := a.safe()
						parts := map[string]rect{
							"menu":    compactHit(p.menu, u),
							"callout": p.callout,
							"board":   p.b.bounds(),
							"spinner": {p.spinX - p.spinR, p.spinY - p.spinR, 2 * p.spinR, 2 * p.spinR},
						}
						for i := range players {
							parts[seatNames[i]] = p.rowRect[i]
						}
						checkParts(t, v, parts)
						if size == [2]float64{390, 844} && board[0] > board[1] && !p.b.turned {
							t.Fatal("a wide board must turn on an upright phone")
						}

						if !containsRect(v, p.pausePanel) {
							t.Fatalf("pause panel %+v leaves the screen", p.pausePanel)
						}
						pause := map[string]rect{}
						for i, r := range p.pauseBtn {
							pause[pauseItems[i]] = r
						}
						checkParts(t, p.pausePanel, pause)

						results := map[string]rect{"again": p.again, "home": p.toHome, "exit": p.exit,
							"dot": {p.winX - p.winDotR, p.winDotY - p.winDotR, 2 * p.winDotR, 2 * p.winDotR}}
						checkParts(t, v, results)
						if w := textWidth(winTitles[players-1], p.winFace); w > v.w {
							t.Fatalf("title %.0f px wide on a %.0f px screen", w, v.w)
						}
					})
				}
			}
		}
	}
}

func TestHomeLayout(t *testing.T) {
	for _, size := range layoutSizes {
		for _, u := range []float64{1, 2.625} {
			a := withCutout(setupTestApp(size[0], size[1], u, 2))
			s := newHome()
			s.layout(a)
			v := a.safe()
			parts := map[string]rect{
				"wordmark": {s.x, s.logoY - s.logo.Size/2, s.w, s.logo.Size},
				"dots":     {s.x, s.dotsY - s.dotR, s.w, 2 * s.dotR},
				"play":     s.playRect(),
				"mat":      s.demo.b.bounds(),
			}
			for _, it := range s.items {
				parts[it.label] = it.r
			}
			t.Run(fmt.Sprintf("%.0fx%.0f/u%g", size[0], size[1], u), func(t *testing.T) { checkParts(t, v, parts) })
		}
	}
}

// TestBoardTurn checks that a turned board maps every cell to its own tile and back, inside the board's bounds.
func TestBoardTurn(t *testing.T) {
	for _, area := range []rect{{0, 0, 800, 500}, {0, 0, 360, 640}} {
		g, err := core.NewGame(7, 4, 2)
		if err != nil {
			t.Fatal(err)
		}
		b := newBoard(g)
		b.layout(area, 1)
		if b.turned != (area.h > area.w) {
			t.Fatalf("area %+v: turned = %v", area, b.turned)
		}
		bounds := b.bounds()
		for i := range g.Geo.Cells() {
			x, y := b.centre(i)
			if got := b.cellAt(x, y); got != i {
				t.Fatalf("area %+v: cell %d draws at (%.0f, %.0f), which reads back as %d", area, i, x, y, got)
			}
			if !bounds.contains(x, y) {
				t.Fatalf("area %+v: cell %d outside the board", area, i)
			}
		}
	}
}
