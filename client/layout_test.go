package client

import (
	"fmt"
	"math"
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
						spinner := rect{p.spinX - p.spinR, p.spinY - p.spinR, 2 * p.spinR, 2 * p.spinR}
						parts := map[string]rect{"menu": compactHit(p.menu, u), "board": p.b.bounds(), "spinner": spinner}
						overlay := !a.portrait() && p.compact // a phone on its side floats the callout over the spinner
						if overlay {
							if !containsRect(spinner, p.callout) {
								t.Fatalf("callout %+v must float inside the spinner %+v", p.callout, spinner)
							}
						} else {
							parts["callout"] = p.callout
						}
						for i := range players {
							parts[seatNames[i]] = p.rowRect[i]
						}
						checkParts(t, v, parts)
						if size == [2]float64{390, 844} && board[0] > board[1] && !p.b.turned {
							t.Fatal("a wide board must turn on an upright phone")
						}

						// The spinner sets the width: the callout everywhere, and Menu and the seat table on the rail.
						d := 2 * p.spinR
						if math.Abs(p.callout.w-d) > 0.01 || math.Abs(p.callout.cx()-p.spinX) > 0.01 {
							t.Fatalf("callout %+v must be the spinner's width (%.1f) and centre", p.callout, d)
						}
						table, cols := p.rowRect[0], float64(p.table.cols)
						need := (p.seatTableWidth(a) - (cols-1)*8*u) / cols
						for _, r := range p.rowRect[:players] {
							table.w = max(table.w, r.right()-table.x)
							if r.w < need-0.01 {
								t.Fatalf("seat row %+v is narrower than its content (%.1f)", r, need)
							}
						}
						if !p.table.tags {
							t.Fatal("the seat numbers must always show")
						}
						// On the rail the spinner is only ever narrower than Menu and the table when the table is at its
						// last style and its least row height.
						squeezed := p.table == railStyles[len(railStyles)-1] && p.rowRect[0].h+4*u <= 24*u
						if !a.portrait() && (math.Abs(p.menu.w-table.w) > 0.01 || table.x != p.menu.x ||
							math.Abs(p.menu.cx()-p.spinX) > 0.01 || d > p.menu.w+0.01 ||
							d < p.menu.w-0.01 && !squeezed) {
							t.Fatalf("rail: Menu %+v and seat table %+v must be the spinner's width (%.1f) and column",
								p.menu, table, d)
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

func TestWantsLandscape(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	for _, tc := range []struct {
		cols, rows int
		want       bool
	}{{30, 18, true}, {9, 9, false}, {5, 9, false}} {
		p, err := newPlay(a, tc.cols, tc.rows, a.set.seats)
		if err != nil {
			t.Fatal(err)
		}
		if wantsLandscape(p) != tc.want || wantsLandscape(newSettingsMenu(p)) != tc.want {
			t.Errorf("%dx%d game: wantsLandscape != %v", tc.cols, tc.rows, tc.want)
		}
	}
	if wantsLandscape(newHome()) || wantsLandscape(newSetup()) || wantsLandscape(newSettingsMenu(newHome())) {
		t.Error("only a game rotates the screen")
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
