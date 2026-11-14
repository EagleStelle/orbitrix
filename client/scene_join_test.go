package client

import (
	"context"
	"fmt"
	"math"
	"testing"

	"orbitrix/client/internal/ui"
)

func TestJoinGroupsVisible(t *testing.T) {
	for _, size := range append(layoutSizes, [2]float64{640, 320}) {
		for _, canJoin := range []bool{false, true} {
			for _, blocked := range []bool{false, true} {
				t.Run(fmt.Sprintf("%gx%g/join%v/blocked%v", size[0], size[1], canJoin, blocked), func(t *testing.T) {
					a := withCutout(setupTestApp(size[0], size[1], 1, 2))
					s := &joinScene{lan: true, canJoin: canJoin, hostBlocked: blocked, code: codeField(),
						joinNote: "Browsers cannot search your Wi-Fi for rooms. Ask the host for an invite link.",
						status:   "No room with that code. Ask the host for a new code."}
					s.layout(a)
					if s.joinB.W <= 0 || len(s.joinLines) == 0 {
						t.Fatal("Join must be laid out with the browser explanation")
					}
					for _, lines := range [][]string{s.joinLines, s.statusLines} {
						for _, line := range lines {
							if ui.TextWidth(line, a.f.Label) > s.joinB.W-2*s.bp {
								t.Fatalf("explanation overflows: %q", line)
							}
						}
					}
					if canJoin && !containsRect(s.joinB, s.join) {
						t.Fatal("Join must stay inside its group")
					}
				})
			}
		}
	}
}

func TestJoinWaitingKeepsButtonInGroup(t *testing.T) {
	for _, size := range append(layoutSizes, [2]float64{640, 320}) {
		for _, waiting := range []bool{false, true} {
			t.Run(fmt.Sprintf("%gx%g/waiting%v", size[0], size[1], waiting), func(t *testing.T) {
				a := withCutout(setupTestApp(size[0], size[1], 1, 2))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				s := &joinScene{canJoin: true, code: codeField(), joining: "K7QX3M", cancel: cancel, rows: make([]joinRow, 20)}
				s.waiting.Store(waiting)
				s.snack.ticks = snackIn
				s.layout(a)
				bar, _, _ := s.snack.frame(a)
				if overlaps(bar, s.join) || !containsRect(s.view.Viewport, s.join) {
					t.Fatal("starting a join must reveal Cancel above the snackbar")
				}
				for _, offset := range []float64{0, s.view.MaxScroll() / 2, s.view.MaxScroll()} {
					s.view.Offset = offset
					s.layout(a)
					got, _, _ := s.snack.frame(a)
					if got != bar || !containsRect(a.safe(), got) {
						t.Fatal("the snackbar must stay fixed regardless of content or scrolling")
					}
					if !containsRect(s.joinB, s.join) || s.join.H != 52*a.u ||
						s.join.X != s.joinB.X+s.bp || s.join.W != s.joinB.W-2*s.bp ||
						s.join.Y != s.slotRow.Bottom()+44*a.u || math.Abs(s.join.Bottom()-(s.joinB.Bottom()-s.bp)) > 0.001 {
						t.Fatal("Join/Cancel must keep its original size, padding and position inside the group")
					}
				}
				if !containsRect(s.view.Viewport, s.join) {
					t.Fatal("Cancel must be reachable at the end of the group's scroll")
				}
				a.in = input{x: s.join.Cx(), y: s.join.Cy(), clicked: true}
				if err := s.update(a); err != nil || ctx.Err() != context.Canceled {
					t.Fatalf("Cancel must stop the join while waiting: %v", err)
				}
			})
		}
	}
}

// The Same Wi-Fi and Online tabs share the column over the Join block, each a full touch target.
func TestJoinTabs(t *testing.T) {
	for _, size := range append(layoutSizes, [2]float64{640, 320}) {
		a := withCutout(setupTestApp(size[0], size[1], 1, 2))
		s := &joinScene{lan: true, canJoin: true, code: codeField()}
		s.layout(a)
		for i, tab := range s.tabs {
			hit := ui.CompactHit(tab, a.u)
			if hit.H < 44*a.u || hit.Bottom() > s.joinB.Y || tab.X < s.joinB.X || tab.Right() > s.joinB.Right()+0.001 {
				t.Fatalf("%v: tab %d %+v must sit over the Join block %+v", size, i, tab, s.joinB)
			}
			if ui.TextWidth(joinTabs[i], a.f.LabelBold) > tab.W-24*a.u {
				t.Fatalf("%v: %q does not fit its tab", size, joinTabs[i])
			}
		}
		if overlaps(s.tabs[0], s.tabs[1]) {
			t.Fatalf("%v: the tabs overlap", size)
		}
	}
}
