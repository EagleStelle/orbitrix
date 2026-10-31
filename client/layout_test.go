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
							if !p.compact && math.Abs(r.h-36*u) > 0.01 {
								t.Fatalf("desktop seat pill height %.1f must stay 36 logical px", r.h/u)
							}
						}
						if name, bold := p.seatFaces(a); !p.compact && (name != a.f.body || bold != a.f.bodyBold) {
							t.Fatal("desktop seat text must stay at body size in both column layouts")
						}
						if !p.table.tags {
							t.Fatal("the seat numbers must always show")
						}
						// On the rail the spinner is only ever narrower than Menu and the table when the column fills the
						// height.
						pad := 32 * u
						if p.compact {
							pad = 16 * u
						}
						bottom := 0.0
						for _, r := range p.rowRect[:players] {
							bottom = max(bottom, r.bottom()+4*u)
						}
						full := math.Abs(bottom-(v.bottom()-pad)) < 0.01
						if !a.portrait() && (math.Abs(p.menu.w-table.w) > 0.01 || table.x != p.menu.x ||
							math.Abs(p.menu.cx()-p.spinX) > 0.01 || d > p.menu.w+0.01 ||
							d < p.menu.w-0.01 && !full) {
							t.Fatalf("rail: Menu %+v and seat table %+v must be the spinner's width (%.1f) and column",
								p.menu, table, d)
						}
						// The rail takes the width the board leaves: the board spans the left margin to the rail's gap.
						if b := p.b.bounds(); !a.portrait() && (math.Abs(b.x-(v.x+pad)) > 1 ||
							math.Abs(b.right()-(p.menu.x-pad)) > 1) {
							t.Fatalf("rail: board %+v must run from the margin to the rail at %.1f", b, p.menu.x)
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

func TestRailColumnsGrowSpinner(t *testing.T) {
	for _, players := range []int{8, 12} {
		for _, u := range []float64{1, 2.625} {
			a := setupTestApp(1919, 1004, u, players)
			p, err := newPlay(a, 9, 9, a.set.seats)
			if err != nil {
				t.Fatal(err)
			}
			// Reuse the scene through resizes: a narrow rail should return to one column when it fits equally well.
			for _, width := range []float64{1919, 1340, 1919} {
				a.w = width * u
				p.layout(a)
				wantCols, wantDiameter := 2, 632.0
				if players == 12 {
					wantDiameter = 552
				}
				if width == 1340 {
					wantCols, wantDiameter = 1, 304
				}
				if p.table.cols != wantCols || !p.table.names || math.Abs(2*p.spinR-wantDiameter*u) > 0.01 {
					t.Fatalf("%dp / width %.0f / u%g: table %+v, wheel %.1f; want %d columns with names and wheel %.1f",
						players, width, u, p.table, 2*p.spinR/u, wantCols, wantDiameter)
				}
				if math.Abs(p.b.bounds().w-940*u) > 0.01 {
					t.Fatal("reflow must use the spare rail width without shrinking the board")
				}
			}
		}
	}
}

func TestOrbAtlasFitsGPUWithoutShrinkingOrbs(t *testing.T) {
	for _, c := range []struct {
		draw, side float64
		limit      int
	}{{100, 100, 4096}, {600, 292, 4096}, {600, 292, 0}, {600, 146, 2048}, {400, 400, 16384}} {
		if got := orbAtlasSide(c.draw, c.limit); got != c.side {
			t.Fatalf("%g px orb on a %d px GPU: source side %g, want %g", c.draw, c.limit, got, c.side)
		}
	}
	b := &board{orbSize: 292, orbDrawSize: 600, sc: &schemes[themeLight]}
	b.orb(1000, 1000, 0)
	if b.verts[0].DstX != 700 || b.verts[3].DstX != 1300 || b.verts[1].SrcX != 292 {
		t.Fatal("a bounded atlas must preserve the orb's physical size while sampling the smaller source")
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

// TestScreenFit checks that a screen too big for the GPU shrinks until it fits the largest image with its padding,
// while screens that already fit keep full resolution.
func TestScreenFit(t *testing.T) {
	for _, tc := range []struct {
		w, h   float64
		m      int
		shrink bool
	}{
		{2560, 1600, 4096, false}, {5120, 2880, 4096, true}, {4096, 2160, 4096, true}, {1080, 2400, 4096, false},
		{3840, 2160, 16384, false}, {7680, 4320, 4096, true}, {20000, 1000, 16384, true}, {1000, 5000, 4096, true},
		{5120, 2880, 0, true},
	} {
		k := screenFit(tc.w, tc.h, tc.m)
		m := tc.m
		if m == 0 {
			m = 4096
		}
		w, h := int(math.Ceil(tc.w*k)), int(math.Ceil(tc.h*k))
		if (k < 1) != tc.shrink || w+1 > m || h+1 > m {
			t.Errorf("%gx%g on a %d px GPU: scale %.3f gives %dx%d", tc.w, tc.h, tc.m, k, w, h)
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
