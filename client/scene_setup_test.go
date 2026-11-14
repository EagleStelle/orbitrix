package client

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

func setupTestApp(w, h, u float64, players int) *app {
	a := &app{w: w * u, h: h * u, u: u, f: ui.NewFonts(u), sc: &ui.Palettes[ui.Light], epoch: time.Now(),
		set: &settings{Cols: 9, Rows: 9, Players: players, seats: []int{seatHuman}}}
	a.set.fitSeats()
	return a
}

func containsRect(outer, inner ui.Rect) bool {
	const epsilon = 0.001
	return inner.W > 0 && inner.H > 0 && inner.X >= outer.X-epsilon && inner.Y >= outer.Y-epsilon &&
		inner.Right() <= outer.Right()+epsilon && inner.Bottom() <= outer.Bottom()+epsilon
}

func TestSetupLayout(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {1440, 900}, {1881, 851}, {760, 640}, {640, 640}, {540, 844}, {390, 844}, {320, 640}, {844, 390}} {
		for _, u := range []float64{1, 1.5, 2} {
			for players := core.MinPlayers; players <= core.MaxPlayers; players++ {
				t.Run(fmt.Sprintf("%.0fx%.0f/u%g/%dp", size[0], size[1], u, players), func(t *testing.T) {
					a, s := setupTestApp(size[0], size[1], u, players), &setupScene{menu: -1, open: foldPlayers}
					s.layout(a)
					if s.start.X != s.players.X || math.Abs(s.start.Right()-s.boardB.Right()) > 0.001 || s.start.Bottom() != a.h-s.start.X {
						t.Fatal("Start must fill the content width and stay at the bottom margin")
					}
					if overlaps(s.players, s.boardB) || s.viewport.Bottom() >= s.start.Y {
						t.Fatal("Board, roster, and pinned action must not overlap")
					}
					if s.previewPanel.H > 0 && !containsRect(s.boardB, s.previewPanel) || s.showPreview && !containsRect(s.previewPanel, s.preview) {
						t.Fatal("board preview is outside its block")
					}
					if s.presetPills[0][0].X > s.previewPanel.X {
						if s.previewPanel.H > 0 && !containsRect(s.viewport, s.previewPanel) {
							t.Fatal("the left preview must stay inside the scroll viewport")
						}
						if s.preview != s.previewPanel.Inset(12*u) {
							t.Fatal("the miniature must scale within the full background with its existing padding")
						}
					}
					presetBottom := s.previewPanel.Y
					for g, group := range s.presetPills {
						label := s.shapeLabels[g]
						if label.Y < presetBottom || label.X < s.boardB.X || label.Right() > s.boardB.Right() {
							t.Fatalf("%s must stack below the previous preset group", shapeNames[g])
						}
						for i, r := range group {
							if r.H != s.back.H || ui.CompactHit(r, u).H != 44*u || ui.CompactHit(r, u).Y < label.Bottom() {
								t.Fatalf("%s preset %d must fit below its heading with a full hit area", shapeNames[g], i)
							}
						}
						presetBottom = ui.CompactHit(group[len(group)-1], u).Bottom()
					}
					if s.colsCtl.Y <= presetBottom || s.rowsCtl.Y <= presetBottom {
						t.Fatal("Columns and Rows must sit below the preset buttons")
					}
					if s.colsCtl.X != s.presetPills[0][0].X || s.rowsCtl.Right() != s.presetPills[0][1].Right() {
						t.Fatal("presets and dimensions must share their group edges")
					}
					for _, ctl := range []struct {
						label              string
						group, minus, plus ui.Rect
					}{{"Columns", s.colsCtl, s.minusCol, s.plusCol}, {"Rows", s.rowsCtl, s.minusRow, s.plusRow}} {
						if ctl.group.H != 44*u || ctl.group.Cy() != ctl.minus.Cy() || ctl.group.Cy() != ctl.plus.Cy() {
							t.Fatal("dimension labels and steppers must share one compact row")
						}
						if ctl.group.X+ui.TextWidth(ctl.label, a.f.LabelBold)+8*u > setupStepHit(ctl.minus, u).X+0.001 {
							t.Fatalf("%s label overlaps its stepper hit area", ctl.label)
						}
					}
					if size[0] < 600 && (s.rowsCtl.Y <= s.colsCtl.Bottom() || s.rowsCtl.X != s.colsCtl.X || s.rowsCtl.W != s.colsCtl.W) {
						t.Fatal("phone dimensions must stack as full-width rows")
					}
					for _, r := range []ui.Rect{s.minusCol, s.plusCol, s.minusRow, s.plusRow} {
						if r.H != s.back.H || setupStepHit(r, u).W != 44*u || setupStepHit(r, u).H != 44*u || setupStepHit(r, u).X < s.boardB.X || setupStepHit(r, u).Right() > s.boardB.Right() {
							t.Fatal("dimension stepper must stay compact with a full touch area inside Board")
						}
					}
					columns := map[float64]bool{}
					for i := range players {
						columns[s.seatTy[i].X] = true
						if s.seatTy[i].H != s.back.H || s.seatTy[i].H != 40*u || s.seatTy[i].H != s.presetPills[0][0].H || s.seatHit[i].H != 44*u || s.seatHit[i].X < s.players.X || s.seatHit[i].Right() > s.players.Right() {
							t.Fatalf("seat %d must match Back's 40px height and retain a 44px target", i)
						}
						if !containsRect(s.seatHit[i], s.seatTy[i]) {
							t.Fatal("seat hit area does not cover its visual pill")
						}
					}
					if len(columns) > 4 {
						t.Fatal("Players must have at most four columns")
					}
					if math.Abs(s.seatTy[len(columns)-1].Right()-(s.players.Right()-s.blockPad)) > 0.001 {
						t.Fatal("player columns must fill the available width through the right padding")
					}
					for i := range players {
						for _, label := range seatLabels {
							if s.seatTy[i].X+seatChoiceX*u+ui.TextWidth(label, a.f.LabelBold) > s.seatTy[i].Right()-40*u {
								t.Fatalf("seat %d cannot fit %s beside its chevron", i, label)
							}
						}
					}
					for _, r := range []ui.Rect{s.minusP, s.plusP} {
						if !containsRect(s.players, setupStepHit(r, u)) || setupStepHit(r, u).Bottom() > s.seatHit[0].Y+0.001 {
							t.Fatal("player count control overlaps the roster or its block")
						}
					}
					if s.minusP.Cy() != s.plusP.Cy() || s.minusP.Right() >= s.plusP.X {
						t.Fatal("player count must be bracketed by minus and plus")
					}
					start := s.start
					s.scroll = s.maxScroll
					s.layout(a)
					if s.start != start || s.boardB.Bottom() > s.viewport.Bottom()+0.001 {
						t.Fatal("scrolling must reveal Board without moving Start")
					}
				})
			}
		}
	}
}

func TestSetupPresetSelection(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {844, 390}} {
		a, s := setupTestApp(size[0], size[1], 1, 2), &setupScene{menu: -1, open: foldPlayers}
		s.layout(a)
		for g := range shapeNames {
			for tier := range boardSizes {
				s.scroll = s.maxScroll // the page down to Map, then the Map body to the preset
				s.layout(a)
				s.pickScroll.Reveal(ui.CompactHit(s.presetPills[g][tier], a.u))
				s.lv = ui.Rect{}
				s.layout(a)
				r := s.presetPills[g][tier]
				if !containsRect(s.viewport, ui.CompactHit(r, a.u)) || !containsRect(s.pickScroll.Viewport, ui.CompactHit(r, a.u)) {
					t.Fatalf("%v: %s preset %d is unreachable by scrolling", size, shapeNames[g], tier)
				}
				a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
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
		a, s := setupTestApp(size[0], size[1], 1, 12), &setupScene{menu: -1, open: foldPlayers}
		s.layout(a)
		s.scroll = s.maxScroll
		s.layout(a)
		s.pickScroll.Offset = s.pickScroll.MaxScroll() // and the Map body to its end
		s.lv = ui.Rect{}
		s.layout(a)
		for _, step := range []struct {
			button     ui.Rect
			cols, rows int
		}{{s.plusCol, 10, 9}, {s.plusRow, 10, 10}, {s.minusCol, 9, 10}, {s.minusRow, 9, 9}} {
			if !containsRect(s.viewport, setupStepHit(step.button, a.u)) {
				t.Fatal("dimension stepper is unreachable at the end of scrolling")
			}
			a.in = input{x: step.button.Cx(), y: step.button.Cy(), clicked: true}
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
	}{{390, 1}, {640, 2}, {760, 3}, {1040, 1}, {1440, 1}} { // a screen on its side gives Players one column, left of Map
		a, s := setupTestApp(tc.width, 844, 1, 12), &setupScene{menu: -1, open: foldPlayers}
		s.layout(a)
		columns := map[float64]bool{}
		for i := range a.set.Players {
			columns[s.seatTy[i].X] = true
		}
		if len(columns) != tc.cols {
			t.Errorf("width %v: %d columns, want %d", tc.width, len(columns), tc.cols)
		}
		if tc.cols == 1 && s.seatTy[0].W != s.players.W-2*s.blockPad {
			t.Error("single-column mobile player buttons must fill the available width")
		}
	}
}

func TestSetupMenuBounds(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {390, 844}, {320, 640}, {844, 390}} {
		a, s := setupTestApp(size[0], size[1], 1, 12), &setupScene{menu: -1, open: foldPlayers}
		s.layout(a)
		for seat := range a.set.Players {
			s.menu = seat
			panel, first := s.menuRects(a)
			if !containsRect(ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, panel) {
				t.Fatalf("menu clips at %v, seat %d: %+v", size, seat, panel)
			}
			for i := range seatLabels {
				opt := setupMenuOption(panel, first, i, a.u)
				if opt.H != ui.CompactButtonTarget || !containsRect(panel, opt) {
					t.Fatalf("option %d clips at %v or loses its touch area", i, size)
				}
			}
		}
	}
}

func TestSetupCountChangesPreserveSeats(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s := setupTestApp(1040, 640, 1, 2), &setupScene{menu: -1, open: foldPlayers}
	a.set.seats = []int{2, seatHuman}
	want := slices.Clone(a.set.seats)
	s.layout(a)
	twoHeight := s.players.H
	s.setPlayers(a, 12)
	s.layout(a)
	if !slices.Equal(a.set.seats[:2], want) || s.players.H <= twoHeight || s.plusP.Cy() != s.headY {
		t.Fatal("adding players changed choices, failed to reflow, or moved plus out of the heading")
	}
	s.setPlayers(a, 2)
	s.layout(a)
	if !slices.Equal(a.set.seats, want) || s.players.H != twoHeight || s.seatHit[2].W != 0 || s.plusP.Cy() != s.headY {
		t.Fatal("removing players left stale seats or lost the heading stepper")
	}
}

// TestRemoveSeat takes a middle seat out of Setup: the seats after it move up with their choices, and two seats stay.
func TestRemoveSeat(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 3)
	a.set.seats = []int{seatHuman, 1 + 2, 1 + 5}
	s := newSetup(a)
	s.layout(a)
	s.menu = 1
	if s.menuCount(a) != len(seatLabels)+2 {
		t.Fatal("three seats offer Swap position and Remove player")
	}
	s.removeSeat(a, 1)
	if a.set.Players != 2 || len(a.set.seats) != 2 || a.set.seats[1] != 1+5 || s.menu != -1 {
		t.Fatalf("after removing seat 1: %d players, seats %v", a.set.Players, a.set.seats)
	}
	s.menu = 0
	if s.menuCount(a) != len(seatLabels)+2 || s.canRemove(a) {
		t.Fatal("two seats still list Remove player, greyed out: they can't lose another")
	}
}

// TestSwapSeat moves this player through a bot seat's menu: Swap position trades their seat with that one at once,
// and is greyed out on their own seat and on another human's.
func TestSwapSeat(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	a.set.seats = []int{seatHuman, 1 + 2, 1 + 5, seatHuman}
	a.set.Name = "Ana"
	s := newSetup(a)
	s.layout(a)
	pick := func(o int) {
		panel, first := s.menuRects(a)
		r := setupMenuOption(panel, first, o, a.u)
		a.in = input{x: r.Cx(), y: r.Cy(), clicked: true}
		if err := s.update(a); err != nil {
			t.Fatal(err)
		}
		a.in = input{x: offScreen, y: offScreen}
	}
	for _, i := range []int{0, 3} { // this player's own seat, then another human's
		s.menu = i
		pick(len(seatLabels)) // Swap position
		if s.canSwap(a) || s.menu != i || !slices.Equal(a.set.seats, []int{seatHuman, 1 + 2, 1 + 5, seatHuman}) {
			t.Fatalf("Swap position on seat %d must be greyed out and do nothing: %v", i, a.set.seats)
		}
	}
	s.menu = 2
	pick(len(seatLabels))
	if !slices.Equal(a.set.seats, []int{1 + 5, 1 + 2, seatHuman, seatHuman}) || s.menu != -1 {
		t.Fatalf("after this player took seat 2: %v", a.set.seats)
	}
	if l, _, _ := s.seat(a, 2); l != "Ana" {
		t.Fatalf("the player's name must follow their seat, got %q", l)
	}
}

// On a screen on its side nothing scrolls the page while Rules is closed: Map is exactly as tall as the screen
// shows, and a full Players card stops at the bottom and scrolls its seats inside itself.
func TestSetupCardsScrollInside(t *testing.T) {
	for _, size := range [][2]float64{{1040, 640}, {844, 390}, {1440, 900}} {
		a, s := setupTestApp(size[0], size[1], 1, 12), &setupScene{menu: -1, open: foldPlayers}
		s.layout(a)
		if s.maxScroll != 0 || s.boardB.Y != s.viewport.Y || s.boardB.H != s.viewport.H {
			t.Fatalf("%v: Map must fill the screen's height exactly and the page must not scroll (max %g)", size, s.maxScroll)
		}
		if s.players.Bottom() > s.viewport.Bottom()+0.001 || s.seatScroll.MaxScroll() == 0 {
			t.Fatalf("%v: twelve seats must scroll inside a Players card that ends on the screen", size)
		}
		last := s.seatHit[11]
		a.in = input{x: last.Cx(), y: s.seatScroll.Viewport.Bottom() + 4, clicked: true}
		if err := s.update(a); err != nil || s.menu >= 0 {
			t.Fatalf("%v: a seat scrolled out of the card must not take a click", size)
		}
		s.seatScroll.Reveal(last)
		s.lv = ui.Rect{}
		s.layout(a)
		if last = s.seatHit[11]; !containsRect(s.seatScroll.Viewport, last) {
			t.Fatalf("%v: the last seat is unreachable by scrolling the card", size)
		}
		a.in = input{x: last.Cx(), y: last.Cy(), clicked: true}
		if err := s.update(a); err != nil || s.menu != 11 {
			t.Fatalf("%v: the last seat must open its menu once scrolled to", size)
		}
	}
}
