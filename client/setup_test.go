package client

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"orbitrix/core"
)

func setupTestApp(w, h, u float64, players int) *app {
	a := &app{w: w * u, h: h * u, u: u, f: newFonts(u), sc: &schemes[themeLight], epoch: time.Now(),
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
					if s.previewPanel.h > 0 && !containsRect(s.boardB, s.previewPanel) || s.showPreview && !containsRect(s.previewPanel, s.preview) {
						t.Fatal("board preview is outside its block")
					}
					if s.presetPills[0][0].x > s.previewPanel.x {
						if s.previewPanel.h > 0 && !containsRect(s.viewport, s.previewPanel) {
							t.Fatal("the left preview must stay inside the scroll viewport")
						}
						if s.preview != s.previewPanel.inset(12*u) {
							t.Fatal("the miniature must scale within the full background with its existing padding")
						}
					}
					presetBottom := s.previewPanel.y
					for g, group := range s.presetPills {
						label := s.shapeLabels[g]
						if label.y < presetBottom || !containsRect(s.boardB, label) {
							t.Fatalf("%s must stack below the previous preset group", shapeNames[g])
						}
						for i, r := range group {
							if r.h != s.back.h || compactHit(r, u).h != 44*u || !containsRect(s.boardB, compactHit(r, u)) || compactHit(r, u).y < label.bottom() {
								t.Fatalf("%s preset %d must fit below its heading with a full hit area", shapeNames[g], i)
							}
						}
						presetBottom = compactHit(group[len(group)-1], u).bottom()
					}
					if s.colsCtl.y <= presetBottom || s.rowsCtl.y <= presetBottom {
						t.Fatal("Columns and Rows must sit below the preset buttons")
					}
					if s.colsCtl.x != s.presetPills[0][0].x || s.rowsCtl.right() != s.presetPills[0][1].right() {
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
						if s.seatTy[i].h != s.back.h || s.seatTy[i].h != 40*u || s.seatTy[i].h != s.presetPills[0][0].h || s.seatHit[i].h != 44*u || !containsRect(s.players, s.seatHit[i]) {
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
							if s.seatTy[i].x+seatChoiceX*u+textWidth(label, a.f.labelBold) > s.seatTy[i].right()-40*u {
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

func TestSetupPreviewFollowsVisibleBoard(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {1881, 851}, {844, 390}, {760, 844}} {
		for _, u := range []float64{1, 2.625} {
			for _, players := range []int{2, 12} {
				a, s := withCutout(setupTestApp(size[0], size[1], u, players)), newSetup()
				s.layout(a)
				start, first, limit := s.start, s.presetPills[0][0], s.maxScroll
				for _, progress := range []float64{0, 0.25, 0.5, 0.75, 1, 0} {
					s.scroll = progress * limit
					s.layout(a)
					bodyTop, bodyBottom := s.boardB.y+s.blockPad+32*u, s.boardB.bottom()-s.blockPad
					panel := s.previewPanel
					if bodyTop >= s.viewport.bottom() {
						if panel.h != 0 || s.showPreview {
							t.Fatal("a Board below the viewport must not draw a miniature")
						}
						continue
					}
					if !containsRect(s.viewport, panel) || !containsRect(s.boardB, panel) {
						t.Fatalf("preview %+v leaves its visible Board area at %v / u%g / %dp / scroll %g", panel, size, u, players, s.scroll)
					}
					if math.Abs(panel.y-max(bodyTop, s.viewport.y)) > 0.001 || math.Abs(panel.bottom()-min(bodyBottom, s.viewport.bottom())) > 0.001 {
						t.Fatal("the preview must fill all of the visible Board column as the page scrolls")
					}
					if s.start != start || s.maxScroll != limit || math.Abs(s.presetPills[0][0].y-(first.y-s.scroll)) > 0.001 {
						t.Fatal("following the preview must preserve normal page scrolling and the pinned Start")
					}
				}
			}
		}
	}
}

func TestSetupPresetSelection(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {844, 390}} {
		a, s := setupTestApp(size[0], size[1], 1, 2), newSetup()
		s.layout(a)
		for g := range shapeNames {
			for tier := range boardSizes {
				s.scroll += s.presetPills[g][tier].cy() - s.viewport.cy()
				s.layout(a)
				r := s.presetPills[g][tier]
				if !containsRect(s.viewport, compactHit(r, a.u)) {
					t.Fatalf("%v: %s preset %d is unreachable by scrolling", size, shapeNames[g], tier)
				}
				a.in = input{x: r.cx(), y: r.cy(), clicked: true}
				if err := s.update(a); err != nil {
					t.Fatal(err)
				}
				cols, rows := presetSize(g, tier)
				if a.set.Cols != cols || a.set.Rows != rows {
					t.Fatalf("%v: %s preset %d selected %dx%d, want %dx%d", size, shapeNames[g], tier, a.set.Cols, a.set.Rows, cols, rows)
				}
			}
		}
	}
}

func TestSetupDimensionsAtScrollEnd(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {844, 390}} {
		a, s := setupTestApp(size[0], size[1], 1, 12), newSetup()
		s.layout(a)
		s.scroll = s.maxScroll
		s.layout(a)
		for _, step := range []struct {
			button     rect
			cols, rows int
		}{{s.plusCol, 10, 9}, {s.plusRow, 10, 10}, {s.minusCol, 9, 10}, {s.minusRow, 9, 9}} {
			if !containsRect(s.viewport, setupStepHit(step.button, a.u)) {
				t.Fatal("dimension stepper is unreachable at the end of scrolling")
			}
			a.in = input{x: step.button.cx(), y: step.button.cy(), clicked: true}
			if err := s.update(a); err != nil {
				t.Fatal(err)
			}
			if a.set.Cols != step.cols || a.set.Rows != step.rows {
				t.Fatalf("%v: stepper set %dx%d, want %dx%d", size, a.set.Cols, a.set.Rows, step.cols, step.rows)
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

// TestRemoveSeat takes a middle seat out of Setup: the seats after it move up with their choices, and two seats stay.
func TestRemoveSeat(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 3)
	a.set.seats = []int{seatHuman, 1 + 2, 1 + 5}
	s := newSetup()
	s.layout(a)
	s.menu = 1
	if s.menuCount(a) != len(seatLabels)+1 {
		t.Fatal("three seats offer Remove player")
	}
	s.removeSeat(a, 1)
	if a.set.Players != 2 || len(a.set.seats) != 2 || a.set.seats[1] != 1+5 || s.menu != -1 {
		t.Fatalf("after removing seat 1: %d players, seats %v", a.set.Players, a.set.seats)
	}
	s.menu = 0
	if s.menuCount(a) != len(seatLabels) {
		t.Fatal("two seats can't lose another")
	}
}
