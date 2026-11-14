package client

import (
	"fmt"
	"math"
	"testing"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
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

func overlaps(a, b ui.Rect) bool {
	return a.X < b.Right() && b.X < a.Right() && a.Y < b.Bottom() && b.Y < a.Bottom()
}

// checkParts fails unless every part sits inside v and no two parts overlap.
func checkParts(t *testing.T, v ui.Rect, parts map[string]ui.Rect) {
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
	for _, size := range append(layoutSizes, [2]float64{320, 640}, [2]float64{640, 320}, [2]float64{540, 600}, [2]float64{768, 800}) {
		for _, u := range []float64{1, 2.625} {
			for players := core.MinPlayers; players <= core.MaxPlayers; players++ {
				for _, board := range [][2]int{{9, 9}, {30, 18}, {5, 7}, {6, 9}, {13, 18}, {3, 40}, {40, 3}} {
					t.Run(fmt.Sprintf("%.0fx%.0f/u%g/%dp/%dx%d", size[0], size[1], u, players, board[0], board[1]), func(t *testing.T) {
						a := withCutout(setupTestApp(size[0], size[1], u, players))
						p, err := newPlay(a, board[0], board[1], a.set.seats)
						if err != nil {
							t.Fatal(err)
						}
						p.layout(a)
						v := a.safe()
						wantHeight := float64(ui.CompactButtonHeight) * u
						if p.compact && !a.portrait() {
							wantHeight = ui.CompactButtonTightHeight * u
						}
						if math.Abs(p.callout.H-p.menu.H) > 0.01 || math.Abs(p.callout.H-wantHeight) > 0.01 || p.menu.W < menuWidth(a)-0.01 {
							t.Fatalf("chip %+v and Menu %+v must share the control height and fit their text", p.callout, p.menu)
						}
						parts := map[string]ui.Rect{"menu": ui.CompactHit(p.menu, u), "board viewport": p.boardView.Viewport,
							"players viewport": p.seatView.Viewport, "callout": p.callout}
						face, _ := p.calloutStyle(a)
						if p.callout.W < ui.TextWidth("Chain ×88", face)+16*u-0.01 {
							t.Fatal("chain chip must fit its label")
						}
						if a.portrait() {
							parts["mover dot"] = ui.Rect{X: p.tokX - p.tokR, Y: p.tokY - p.tokR, W: 2 * p.tokR, H: 2 * p.tokR}
							if p.spinR != 0 || p.tokR <= 0 || math.Abs(p.tokX-v.Cx()) > 0.01 ||
								math.Abs(p.tokY-p.menu.Cy()) > 0.01 || p.tokX+p.tokR >= p.menu.X || math.Abs(p.callout.W-p.menu.W) > 0.01 {
								t.Fatal("portrait must show only the turn dot beside Menu")
							}
							b := p.b.Bounds()
							cols, rows := p.b.ScreenSize()
							maxHeight := p.menu.Y - (v.Y + 16*u) - 32*u - p.seatRowPitch(a)
							wantWidth := min(v.W-32*u, maxHeight*(float64(cols)-2*boardview.TileGutter)/(float64(rows)-2*boardview.TileGutter))
							if math.Abs(b.W-wantWidth) > 0.01 || math.Abs(b.Cx()-v.Cx()) > 1 {
								t.Fatalf("portrait board %+v must fill the available width unless constrained by screen height", b)
							}
							if !containsRect(v, b) || !containsRect(p.boardView.Viewport.Inset(-1), b) || p.boardView.MaxScroll() != 0 {
								t.Fatalf("the entire portrait board %+v must fit on screen without scrolling or clipping", b)
							}
							if math.Abs(b.H/b.W-(float64(rows)-2*boardview.TileGutter)/(float64(cols)-2*boardview.TileGutter)) > 0.001 {
								t.Fatal("portrait boards must retain square cells")
							}
							if board[0] > board[1] && !p.b.Turned {
								t.Fatal("wide boards must turn on upright browser screens")
							}
							if math.Abs(p.menu.Bottom()-(v.Bottom()-16*u)) > 0.01 || math.Abs(p.menu.Cy()-p.callout.Cy()) > 0.01 {
								t.Fatal("portrait controls must share the bottom row")
							}
							boardView, seats := p.boardView.Viewport, p.seatView.Viewport
							if math.Abs(boardView.Bottom()+16*u-seats.Y) > 0.01 || math.Abs(seats.Bottom()+16*u-p.menu.Y) > 0.01 ||
								seats.H > p.seatTableHeight(a)+0.01 {
								t.Fatal("portrait board and players must anchor above the bottom controls without spare space below the list")
							}
						} else {
							d := 2 * p.spinR
							spinner := ui.Rect{X: p.spinX - p.spinR, Y: p.spinY - p.spinR, W: d, H: d}
							parts["spinner"] = spinner
							if p.tokR != 0 || math.Abs(d-p.menu.W) > 0.01 || math.Abs(d-p.callout.W) > 0.01 ||
								math.Abs(d-p.seatView.Viewport.W) > 0.01 || math.Abs(p.spinX-p.menu.Cx()) > 0.01 ||
								p.callout.Y < spinner.Bottom()+12*u-0.01 || p.seatView.Viewport.Y < p.callout.Bottom()+12*u-0.01 {
								t.Fatal("landscape must keep a full-width wheel, chip, then scrolling players")
							}
						}
						checkParts(t, v, parts)
						cols := float64(p.tableCols)
						need := (p.seatTableWidth(a) - (cols-1)*8*u) / cols
						for _, r := range p.rowRect[:players] {
							if r.W < need-0.01 || math.Abs(r.H-(p.seatRowPitch(a)-4*u)) > 0.01 {
								t.Fatalf("seat row %+v must fit its text without compressing its height", r)
							}
						}
						if p.seatView.Offset < 0 || p.seatView.Offset > p.seatView.MaxScroll()+0.01 {
							t.Fatal("player scroll must stay in bounds")
						}
						if !containsRect(v, p.pausePanel) {
							t.Fatalf("pause panel %+v leaves the screen", p.pausePanel)
						}
						pause := map[string]ui.Rect{}
						for k, i := range p.items {
							pause[pauseItems[i]] = p.pauseBtn[k]
						}
						checkParts(t, p.pausePanel, pause)
						results := map[string]ui.Rect{"again": p.again, "home": p.toHome, "exit": p.exit,
							"dot": {X: p.winX - p.winDotR, Y: p.winDotY - p.winDotR, W: 2 * p.winDotR, H: 2 * p.winDotR}}
						checkParts(t, v, results)
						for _, l := range p.labels {
							if ui.TextWidth(l+" wins!", p.winFace) > v.W {
								t.Fatal("result title must fit the screen")
							}
						}
					})
				}
			}
		}
	}
}

func TestPlayerOverflowAndTurnReveal(t *testing.T) {
	for _, size := range [][2]float64{{780, 360}, {360, 740}} {
		for _, u := range []float64{1, 2.625} {
			a := withCutout(setupTestApp(size[0], size[1], u, 12))
			p, err := newPlay(a, 13, 18, a.set.seats)
			if err != nil {
				t.Fatal(err)
			}
			p.layout(a)
			if p.seatView.MaxScroll() <= 0 {
				t.Fatal("tight player lists must overflow instead of squeezing the wheel or board")
			}
			diameter, boardWidth := 2*p.spinR, p.b.Bounds().W
			v := p.seatView.Viewport
			a.in = input{x: v.Cx(), y: v.Cy(), wheel: -10000}
			p.updateScroll(a)
			if p.seatView.Offset != p.seatView.MaxScroll() || !overlaps(p.rowRect[11], v) {
				t.Fatal("wheel scrolling must reach the final player and clamp at the end")
			}
			a.in = input{x: p.menu.Cx(), y: p.menu.Cy(), wheel: 10000}
			p.updateScroll(a)
			if p.seatView.Offset != p.seatView.MaxScroll() {
				t.Fatal("scrolling outside the player viewport must not move it")
			}
			a.in = input{x: v.Cx(), y: v.Cy(), wheel: 10000}
			p.updateScroll(a)
			p.shown = 11
			p.layout(a)
			if !containsRect(v, p.rowRect[11]) {
				t.Fatal("a turn change must reveal the active player's complete row")
			}
			p.updateScroll(a)
			p.layout(a)
			if p.seatView.Offset != 0 {
				t.Fatal("manual scrolling must remain possible after revealing the turn")
			}
			if diameter != 2*p.spinR || boardWidth != p.b.Bounds().W {
				t.Fatal("scrolling must not resize the board or wheel")
			}
			for _, height := range []float64{1440, size[1]} {
				a.h = height * u
				p.layout(a)
				if p.seatView.Offset > p.seatView.MaxScroll() {
					t.Fatal("resizing must clamp the scroll offset")
				}
				if a.portrait() && (p.spinR != 0 || p.tokR <= 0) || !a.portrait() && (p.tokR != 0 || math.Abs(2*p.spinR-p.menu.W) > 0.01) {
					t.Fatal("rotating must switch between the full-width wheel and portrait dot")
				}
			}
		}
	}
}

func TestPortraitBoardFitAndDragOwnership(t *testing.T) {
	for _, u := range []float64{1, 2.625} {
		a := withCutout(setupTestApp(360, 740, u, 12))
		p, err := newPlay(a, 3, 40, a.set.seats)
		if err != nil {
			t.Fatal(err)
		}
		p.layout(a)
		if p.boardView.MaxScroll() != 0 || p.seatView.MaxScroll() <= 0 {
			t.Fatal("an extreme board must fit entirely while the player list can scroll")
		}
		b, s := p.boardView.Viewport, p.seatView.Viewport
		tiles, orbs := p.b.TileImg, p.b.OrbImg
		a.in = input{x: b.Cx(), y: b.Cy(), finger: true, down: true, pressed: true}
		p.updateScroll(a)
		a.in = input{x: s.Cx(), y: s.Cy(), finger: true, down: true, dragY: -60 * u}
		p.updateScroll(a)
		if p.boardView.Offset != 0 || p.seatView.Offset != 0 {
			t.Fatal("a drag that starts on the fitted board must not move it or the players")
		}
		if p.cellAt(s.Cx(), s.Cy()) != -1 || p.cellAt(p.menu.Cx(), p.menu.Cy()) != -1 {
			t.Fatal("board cells must not respond through the player list or Menu")
		}
		a.in = input{x: s.Cx(), y: s.Cy(), finger: true, down: true, pressed: true}
		p.updateScroll(a)
		a.in = input{x: b.Cx(), y: b.Cy(), finger: true, down: true, dragY: -40 * u}
		p.updateScroll(a)
		if p.boardView.Offset != 0 || p.seatView.Offset != 40*u {
			t.Fatal("a drag that starts on the players must not scroll the board")
		}
		a.in = input{x: b.Cx(), y: b.Cy(), wheel: -10000}
		p.updateScroll(a)
		if p.boardView.Offset != 0 {
			t.Fatal("wheel scrolling must not move the fitted board")
		}
		for cell := range p.game.Geo.Cells() {
			x, y := p.b.Centre(cell)
			if !b.Contains(x, y) || p.cellAt(x, y) != cell {
				t.Fatalf("fitted board cell %d must remain visible and tappable", cell)
			}
		}
		if p.b.TileImg != tiles || p.b.OrbImg != orbs {
			t.Fatal("scrolling must reuse the board images")
		}
	}
}

func TestOrbAtlasFitsGPUWithoutShrinkingOrbs(t *testing.T) {
	for _, c := range []struct {
		draw, side float64
		limit      int
	}{{100, 100, 4096}, {600, 292, 4096}, {600, 292, 0}, {600, 146, 2048}, {400, 400, 16384}} {
		if got := boardview.OrbAtlasSide(c.draw, c.limit); got != c.side {
			t.Fatalf("%g px orb on a %d px GPU: source side %g, want %g", c.draw, c.limit, got, c.side)
		}
	}
	b := &boardview.Board{OrbSize: 292, OrbDrawSize: 600, Palette: &ui.Palettes[ui.Light]}
	b.Orb(1000, 1000, 0)
	if b.Verts[0].DstX != 700 || b.Verts[3].DstX != 1300 || b.Verts[1].SrcX != 292 {
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
			parts := map[string]ui.Rect{
				"wordmark": {X: s.x, Y: s.logoY - s.logo.Size/2, W: s.w, H: s.logo.Size},
				"dots":     {X: s.x, Y: s.dotsY - s.dotR, W: s.w, H: 2 * s.dotR},
				"play":     s.playRect(),
				"name":     s.bar,
				"mat":      s.demo.b.Bounds(),
			}
			for _, it := range s.items {
				parts[it.label] = it.r
				if it.r.Right() > s.x+s.w+0.001 || it.r.X < s.x {
					t.Fatalf("%s leaves the column", it.label)
				}
			}
			if s.bar.H != 2*s.playR || s.playX-s.playR != s.x || s.bar.X <= s.playX+s.playR || math.Abs(s.bar.Right()-(s.x+s.w)) > 0.001 {
				t.Fatal("the play button, then the name bar, must share one row as wide as the column, at one height")
			}
			t.Run(fmt.Sprintf("%.0fx%.0f/u%g", size[0], size[1], u), func(t *testing.T) {
				checkParts(t, v, parts)
			})
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
	if wantsLandscape(newHome()) || wantsLandscape(newSetup(a)) || wantsLandscape(newSettingsMenu(newHome())) {
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
		k := ui.ScreenFit(tc.w, tc.h, tc.m)
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
	for _, area := range []ui.Rect{{X: 0, Y: 0, W: 800, H: 500}, {X: 0, Y: 0, W: 360, H: 640}} {
		g, err := core.NewGame(7, 4, 2)
		if err != nil {
			t.Fatal(err)
		}
		b := boardview.New(g)
		b.Layout(area, 1)
		if b.Turned != (area.H > area.W) {
			t.Fatalf("area %+v: turned = %v", area, b.Turned)
		}
		bounds := b.Bounds()
		for i := range g.Geo.Cells() {
			x, y := b.Centre(i)
			if got := b.CellAt(x, y); got != i {
				t.Fatalf("area %+v: cell %d draws at (%.0f, %.0f), which reads back as %d", area, i, x, y, got)
			}
			if !bounds.Contains(x, y) {
				t.Fatalf("area %+v: cell %d outside the board", area, i)
			}
		}
	}
}
