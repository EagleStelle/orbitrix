package client

import (
	"math"
	"testing"
)

func TestSnackbarBottomAnchor(t *testing.T) {
	for _, size := range layoutSizes {
		for _, u := range []float64{1, 2.625} {
			a := withCutout(setupTestApp(size[0], size[1], u, 2))
			a.ins[3] = 24 * u
			bar := snackbar{message: "Waiting for the host", ticks: snackIn}
			bounds, _, _ := bar.frame(a)
			if math.Abs(bounds.bottom()-(a.safe().bottom()-12*u)) > 0.001 {
				t.Fatal("a snackbar without a fixed action row must leave a 12 px gap above the safe-area bottom")
			}
			s := newRoomSetup(a, false)
			s.layout(a)
			for _, ticks := range []int{0, snackIn / 2, snackIn} {
				s.room.snack.ticks = ticks
				bounds, _, _ = s.room.snack.frame(a)
				if !containsRect(a.safe(), bounds) || overlaps(bounds, s.start) || overlaps(bounds, s.invite) {
					t.Fatal("the room snackbar must keep Open room/Start and Invite clear throughout its entrance")
				}
			}
			if math.Abs(bounds.bottom()-(s.start.y-12*u)) > 0.001 {
				t.Fatal("the room snackbar must sit just above the fixed action row")
			}
			s.scroll = s.maxScroll
			s.layout(a)
			got, _, _ := s.room.snack.frame(a)
			if got != bounds {
				t.Fatal("scrolling must not reposition the room snackbar")
			}
		}
	}
}

func TestSnackbarActions(t *testing.T) {
	for _, size := range layoutSizes {
		for _, u := range []float64{1, 2.625} {
			a := withCutout(setupTestApp(size[0], size[1], u, 2))
			bar := snackbar{message: "Human wants to join", actions: [2]string{"No", "Let in"}}
			for _, ticks := range []int{0, snackIn / 2, snackIn} {
				bar.ticks = ticks
				bounds, _, _ := bar.frame(a)
				if !containsRect(a.safe(), bounds) {
					t.Fatal("the overlay must stay in the safe area throughout its entrance")
				}
				for i := range bar.actions {
					_, buttons, _ := bar.frame(a)
					a.in = input{x: buttons[i].cx(), y: buttons[i].cy(), clicked: true}
					if action, over := bar.update(a); action != i || !over {
						t.Fatalf("snackbar action %d must consume its click; got %d, %v", i, action, over)
					}
				}
			}
			bar.actions = [2]string{}
			bounds, _, _ := bar.frame(a)
			a.in = input{x: bounds.cx(), y: bounds.cy(), clicked: true}
			if action, over := bar.update(a); action != -1 || !over {
				t.Fatal("a status-only snackbar must consume the pointer without choosing an action")
			}
		}
	}
}
