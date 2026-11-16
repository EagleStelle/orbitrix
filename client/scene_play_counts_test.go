package client

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"orbitrix/client/internal/ui"
	"orbitrix/core"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestOrbCountsFitIndicators(t *testing.T) {
	dst := ebiten.NewImage(1, 1)
	defer dst.Deallocate()
	for _, size := range [][2]float64{{640, 320}, {780, 360}, {1040, 640}, {1920, 1080}, {320, 640}, {360, 740}} {
		for _, u := range []float64{1, 2.625} {
			for players := 2; players <= 12; players++ {
				for _, marked := range []bool{false, true} {
					t.Run(fmt.Sprintf("%gx%g/u%g/%dp/marks%t", size[0], size[1], u, players, marked), func(t *testing.T) {
						a := withCutout(setupTestApp(size[0], size[1], u, players))
						a.set.Marks = marked
						p, err := newPlay(a, 40, 40, a.set.seats)
						if err != nil {
							t.Fatal(err)
						}
						p.layout(a)
						wheel, dot, board := p.spinR, p.tokX, p.b.Bounds()
						for _, value := range []string{"0", "999", "1000", "9999", "10000", "99999", "88888", "11111"} {
							var boxes []ui.Rect
							for player := range players {
								p.orbS[player] = value
								x, y, r, n, half := p.spinX, p.spinY, p.spinR, players, math.Pi*p.share[player]
								if a.portrait() {
									x, y, r, n, half = p.tokX, p.tokY, p.tokR, 0, 0
								}
								p.drawOrbCount(dst, a, player, x, y, r, half)
								c := p.orbLabels[player]
								if c.Key.Value != value || c.Scale <= 0 || c.Scale > 1 || c.Bounds.W <= 0 || c.Bounds.H <= 0 {
									t.Fatalf("count %q must be preserved and visible: %+v", value, c)
								}
								if c.Scale*c.Key.Face.Size < 8*u {
									t.Fatalf("count %q falls below 8 logical px: %.2f", value, c.Scale*c.Key.Face.Size/u)
								}
								for _, corner := range [][2]float64{{c.Bounds.X, c.Bounds.Y}, {c.Bounds.Right(), c.Bounds.Y},
									{c.Bounds.X, c.Bounds.Bottom()}, {c.Bounds.Right(), c.Bounds.Bottom()}} {
									dx, dy := corner[0]-x, corner[1]-y
									margin := 2 * u
									if n > 0 {
										margin = 8 * u
										angle := math.Remainder(math.Atan2(dy, dx)-p.wedgeAngle(player), 2*math.Pi)
										if math.Abs(angle) >= math.Pi/float64(players) {
											t.Fatalf("%q leaves player %d's slice: %+v", value, player, c.Bounds)
										}
									} else if marked && corner[1] < p.tokY-5*u-0.001 {
										t.Fatal("portrait count must clear the colour-blind mark badge")
									}
									if math.Hypot(dx, dy) > r-margin+0.001 {
										t.Fatalf("%q leaves its circular indicator: %+v", value, c.Bounds)
									}
								}
								if !a.portrait() {
									for _, other := range boxes {
										if overlaps(c.Bounds, other) {
											t.Fatal("adjacent slice counts must not overlap")
										}
									}
									boxes = append(boxes, c.Bounds)
								}
								p.drawOrbCount(dst, a, player, x, y, r, half)
								if p.orbLabels[player] != c {
									t.Fatal("unchanged count geometry must be reused")
								}
							}
						}
						if p.spinR != wheel || p.tokX != dot || p.b.Bounds() != board {
							t.Fatal("long counts must not resize or move the existing layout")
						}
					})
				}
			}
		}
	}
}

func TestIndicatorCountsFollowAnimatedBoard(t *testing.T) {
	a := setupTestApp(390, 844, 1, 2)
	p, err := newPlay(a, 40, 40, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.layout(a)
	dst := ebiten.NewImage(1, 1)
	defer dst.Deallocate()
	for _, count := range []int{0, 999, 9999, 99999, 0} {
		clear(p.b.State.Count)
		for i, left := 0, count; left > 0; i++ {
			n := min(left, 255)
			p.b.State.Owner[i], p.b.State.Count[i] = 0, uint8(n)
			left -= n
		}
		p.tickRail()
		p.drawOrbCount(dst, a, 0, p.tokX, p.tokY, p.tokR, 0)
		if p.orbS[0] != strconv.Itoa(count) || p.orbLabels[0].Key.Value != p.orbS[0] {
			t.Fatalf("indicator and player list must follow the shown board: %d, %q", count, p.orbLabels[0].Key.Value)
		}
	}
}

func TestOrbCountsReuseWheelAndSkipEliminatedPlayers(t *testing.T) {
	a := setupTestApp(780, 360, 1, 12)
	p, err := newPlay(a, 9, 9, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	p.tickRail()
	p.layout(a)
	dst := ebiten.NewImage(1, 1)
	defer dst.Deallocate()
	p.drawSpinner(dst, a)
	wheel, pointer := p.wheel, p.pointer
	for i := range p.seats {
		p.orbS[i] = "99999"
	}
	a.set.Marks = true
	p.drawSpinner(dst, a)
	if p.wheel != wheel || p.pointer != pointer || p.orbLabels[11].Key.Value != "99999" {
		t.Fatal("count and mark changes must update without rebuilding the cached wheel")
	}
	p.alive[3], p.orbS[3] = false, "12345"
	p.drawSpinner(dst, a)
	if p.orbLabels[3].Key.Value != "99999" {
		t.Fatal("eliminated players must not draw a count on their empty slice")
	}
}

// The wheel is a pie: a cell teammates share counts for each colour's own orbs, slices follow the counts, every
// player still in keeps one and an out player has none.
func TestWheelSlicesFollowOrbs(t *testing.T) {
	a := setupTestApp(1040, 640, 1, 4)
	a.set.Teams, a.set.Team = 2, []uint8{0, 0, 1, 1}
	p, err := newPlay(a, 5, 5, a.set.seats)
	if err != nil {
		t.Fatal(err)
	}
	s := p.b.State
	stride := len(s.Mix) / len(s.Count)
	s.Owner[0], s.Count[0], s.Mix[0], s.Mix[1] = 0, 3, 2, 1 // seats 0 and 1 share a cell
	s.Owner[1], s.Count[1], s.Mix[stride+2] = 2, 1, 1
	p.alive[3] = false
	p.tickRail()
	if p.orbs != [core.MaxPlayers]int{2, 1, 1} {
		t.Fatalf("a shared cell must count for each colour in it: %v", p.orbs[:4])
	}
	sh := p.share
	if sh[0] <= sh[1] || sh[1] != sh[2] || sh[2] <= 0 || sh[3] != 0 || math.Abs(sh[0]+sh[1]+sh[2]-1) > 1e-9 {
		t.Fatalf("slices must follow the orbs and fill the wheel: %v", sh[:4])
	}
	if math.Abs(p.wedgeAngle(2)+math.Pi*sh[2]-1.5*math.Pi) > 1e-9 {
		t.Fatal("the last slice must end where the first begins")
	}
}
