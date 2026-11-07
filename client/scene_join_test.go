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
					if s.hostB.W <= 0 || s.joinB.W <= 0 || overlaps(s.hostB, s.joinB) {
						t.Fatal("Host and Join must both be laid out")
					}
					if !containsRect(s.hostB, s.host) || len(s.joinLines) == 0 {
						t.Fatal("groups must contain their action and the browser Join explanation")
					}
					if s.hostB.H != 2*s.bp+32*a.u+52*a.u {
						t.Fatal("Host must have no description space")
					}
					for _, lines := range [][]string{s.joinLines, s.statusLines} {
						for _, line := range lines {
							if ui.TextWidth(line, a.f.Label) > s.hostB.W-2*s.bp {
								t.Fatalf("explanation overflows: %q", line)
							}
						}
					}
					if canJoin && !containsRect(s.joinB, s.join) {
						t.Fatal("Join must stay inside its group")
					}
					if blocked {
						a.in = input{x: s.host.Cx(), y: s.host.Cy(), clicked: true}
						if err := s.update(a); err != nil || a.next != nil {
							t.Fatal("unavailable hosting must not open Setup")
						}
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
