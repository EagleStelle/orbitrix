package client

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"orbitrix/core"
)

func setupTestApp(w, h, u float64, players int) *app {
	a := &app{w: w * u, h: h * u, u: u, f: newFonts(u), sc: &schemes[themeLight],
		set: &settings{Cols: 9, Rows: 9, Players: players, seats: []int{seatHuman}}}
	a.set.fitSeats()
	return a
}

func containsRect(outer, inner rect) bool {
	const epsilon = 0.001
	return inner.w > 0 && inner.h > 0 && inner.x >= outer.x-epsilon && inner.y >= outer.y-epsilon &&
		inner.right() <= outer.right()+epsilon && inner.bottom() <= outer.bottom()+epsilon
}

func TestSetupLayout(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {1440, 900}, {1881, 851}, {760, 640}, {640, 640}, {540, 844}, {390, 844}, {320, 640}, {844, 390}} {
		for _, u := range []float64{1, 1.5, 2} {
			for players := core.MinPlayers; players <= core.MaxPlayers; players++ {
				t.Run(fmt.Sprintf("%.0fx%.0f/u%g/%dp", size[0], size[1], u, players), func(t *testing.T) {
					a, s := setupTestApp(size[0], size[1], u, players), newSetup()
					s.layout(a)
					if s.start.x != s.boardB.x || s.start.w != s.boardB.w || s.start.bottom() != a.h-s.start.x {
						t.Fatal("Start must fill the content width and stay at the bottom margin")
					}
					if s.players.bottom() >= s.boardB.y || s.viewport.bottom() >= s.start.y {
						t.Fatal("Board, roster, and pinned action must not overlap")
					}
					if !s.showPreview || !containsRect(s.boardB, s.previewPanel) || !containsRect(s.previewPanel, s.preview) {
						t.Fatal("board preview is hidden or outside its block")
					}
					for i, r := range s.presetPills {
						if r.h != s.back.h || compactHit(r, u).h != 44*u || !containsRect(s.boardB, compactHit(r, u)) {
							t.Fatalf("preset %d must match Back's height, retain its hit area, and fit Board", i)
						}
					}
					presetBottom := s.presetPills[len(s.presetPills)-1].bottom()
					if s.colsCtl.y <= presetBottom || s.rowsCtl.y <= presetBottom {
						t.Fatal("Columns and Rows must sit below the preset buttons")
					}
					if s.colsCtl.x != s.presetPills[0].x || s.rowsCtl.right() != s.presetPills[1].right() {
						t.Fatal("presets and dimensions must share their group edges")
					}
					for _, ctl := range []struct {
						label              string
						group, minus, plus rect
					}{{"Columns", s.colsCtl, s.minusCol, s.plusCol}, {"Rows", s.rowsCtl, s.minusRow, s.plusRow}} {
						if ctl.group.h != 44*u || ctl.group.cy() != ctl.minus.cy() || ctl.group.cy() != ctl.plus.cy() {
							t.Fatal("dimension labels and steppers must share one compact row")
						}
						if ctl.group.x+textWidth(ctl.label, a.f.labelBold)+8*u > setupStepHit(ctl.minus, u).x+0.001 {
							t.Fatalf("%s label overlaps its stepper hit area", ctl.label)
						}
					}
					if size[0] < 600 && (s.rowsCtl.y <= s.colsCtl.bottom() || s.rowsCtl.x != s.colsCtl.x || s.rowsCtl.w != s.colsCtl.w) {
						t.Fatal("phone dimensions must stack as full-width rows")
					}
					if size[0] >= 1040 && s.rowsCtl.y != s.colsCtl.y {
						t.Fatal("desktop dimensions must sit side by side")
					}
					for _, r := range []rect{s.minusCol, s.plusCol, s.minusRow, s.plusRow} {
						if r.h != s.back.h || setupStepHit(r, u).w != 44*u || setupStepHit(r, u).h != 44*u || !containsRect(s.boardB, setupStepHit(r, u)) {
							t.Fatal("dimension stepper must stay compact with a full touch area inside Board")
						}
					}
					columns := map[float64]bool{}
					for i := range players {
						columns[s.seatTy[i].x] = true
						if s.seatTy[i].h != s.back.h || s.seatTy[i].h != 40*u || s.seatTy[i].h != s.presetPills[0].h || s.seatHit[i].h != 44*u || !containsRect(s.players, s.seatHit[i]) {
							t.Fatalf("seat %d must match Back's 40px height and retain a 44px target", i)
						}
						if !containsRect(s.seatHit[i], s.seatTy[i]) {
							t.Fatal("seat hit area does not cover its visual pill")
						}
					}
					if len(columns) > 4 {
						t.Fatal("Players must have at most four columns")
					}
					if math.Abs(s.seatTy[len(columns)-1].right()-(s.players.right()-s.blockPad)) > 0.001 {
						t.Fatal("player columns must fill the available width through the right padding")
					}
					for i := range players {
						for _, label := range seatLabels {
							if s.seatTag[i].right()+8*u+textWidth(label, a.f.labelBold) > s.seatTy[i].right()-40*u {
								t.Fatalf("seat %d cannot fit %s beside its chevron", i, label)
							}
						}
					}
					for _, r := range []rect{s.minusP, s.plusP} {
						if !containsRect(s.players, setupStepHit(r, u)) || setupStepHit(r, u).bottom() > s.seatHit[0].y+0.001 {
							t.Fatal("player count control overlaps the roster or its block")
						}
					}
					if s.minusP.cy() != s.plusP.cy() || s.minusP.right() >= s.plusP.x {
						t.Fatal("player count must be bracketed by minus and plus")
					}
					if size[0] >= 1040 && players == 2 && (s.maxScroll != 0 || !containsRect(s.viewport, s.boardB)) {
						t.Fatal("default desktop setup requires scrolling")
					}
					start := s.start
					s.scroll = s.maxScroll
					s.layout(a)
					if s.start != start || s.boardB.bottom() > s.viewport.bottom()+0.001 {
						t.Fatal("scrolling must reveal Board without moving Start")
					}
				})
			}
		}
	}
}

func TestSetupPlayerColumnBreakpoints(t *testing.T) {
	for _, tc := range []struct {
		width float64
		cols  int
	}{{390, 1}, {640, 2}, {760, 3}, {1040, 4}, {1440, 4}} {
		a, s := setupTestApp(tc.width, 844, 1, 12), newSetup()
		s.layout(a)
		columns := map[float64]bool{}
		for i := range a.set.Players {
			columns[s.seatTy[i].x] = true
		}
		if len(columns) != tc.cols {
			t.Errorf("width %v: %d columns, want %d", tc.width, len(columns), tc.cols)
		}
		if tc.cols == 1 && s.seatTy[0].w != s.players.w-2*s.blockPad {
			t.Error("single-column mobile player buttons must fill the available width")
		}
	}
}

func TestSetupMenuBounds(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {320, 640}, {844, 390}} {
		a, s := setupTestApp(size[0], size[1], 1, 12), newSetup()
		s.layout(a)
		for seat := range a.set.Players {
			s.menu = seat
			panel, first := s.menuRects(a)
			if !containsRect(rect{0, 0, a.w, a.h}, panel) {
				t.Fatalf("menu clips at %v, seat %d: %+v", size, seat, panel)
			}
			for i := range seatLabels {
				opt := setupMenuOption(panel, first, i, a.u)
				if opt.h != compactButtonTarget || !containsRect(panel, opt) {
					t.Fatalf("option %d clips at %v or loses its touch area", i, size)
				}
			}
		}
	}
}

func TestSetupCountChangesPreserveSeats(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s := setupTestApp(1040, 640, 1, 2), newSetup()
	a.set.seats = []int{2, seatHuman}
	want := slices.Clone(a.set.seats)
	s.layout(a)
	twoHeight := s.players.h
	s.setPlayers(a, 12)
	s.layout(a)
	if !slices.Equal(a.set.seats[:2], want) || s.players.h <= twoHeight || s.plusP.cy() != s.headY {
		t.Fatal("adding players changed choices, failed to reflow, or moved plus out of the heading")
	}
	s.setPlayers(a, 2)
	s.layout(a)
	if !slices.Equal(a.set.seats, want) || s.players.h != twoHeight || s.seatHit[2].w != 0 || s.plusP.cy() != s.headY {
		t.Fatal("removing players left stale seats or lost the heading stepper")
	}
}
