package client

import (
	"fmt"
	"math"
	"strconv"
	"testing"

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
						wheel, dot, board := p.spinR, p.tokX, p.b.bounds()
						for _, value := range []string{"0", "999", "1000", "9999", "10000", "99999", "88888", "11111"} {
							var boxes []rect
							for player := range players {
								p.orbS[player] = value
								x, y, r, n := p.spinX, p.spinY, p.spinR, players
								if a.portrait() {
									x, y, r, n = p.tokX, p.tokY, p.tokR, 0
								}
								p.drawOrbCount(dst, a, player, x, y, r, n)
								c := p.orbLabels[player]
								if c.key.value != value || c.scale <= 0 || c.scale > 1 || c.bounds.w <= 0 || c.bounds.h <= 0 {
									t.Fatalf("count %q must be preserved and visible: %+v", value, c)
								}
								if c.scale*c.key.face.Size < 8*u {
									t.Fatalf("count %q falls below 8 logical px: %.2f", value, c.scale*c.key.face.Size/u)
								}
								for _, corner := range [][2]float64{{c.bounds.x, c.bounds.y}, {c.bounds.right(), c.bounds.y},
									{c.bounds.x, c.bounds.bottom()}, {c.bounds.right(), c.bounds.bottom()}} {
									dx, dy := corner[0]-x, corner[1]-y
									margin := 2 * u
									if n > 0 {
										margin = 8 * u
										angle := math.Remainder(math.Atan2(dy, dx)-p.wedgeAngle(player), 2*math.Pi)
										if math.Abs(angle) >= math.Pi/float64(players) {
											t.Fatalf("%q leaves player %d's slice: %+v", value, player, c.bounds)
										}
									} else if marked && corner[1] < p.tokY-5*u-0.001 {
										t.Fatal("portrait count must clear the colour-blind mark badge")
									}
									if math.Hypot(dx, dy) > r-margin+0.001 {
										t.Fatalf("%q leaves its circular indicator: %+v", value, c.bounds)
									}
								}
								if !a.portrait() {
									for _, other := range boxes {
										if overlaps(c.bounds, other) {
											t.Fatal("adjacent slice counts must not overlap")
										}
									}
									boxes = append(boxes, c.bounds)
								}
								p.drawOrbCount(dst, a, player, x, y, r, n)
								if p.orbLabels[player] != c {
									t.Fatal("unchanged count geometry must be reused")
								}
							}
						}
						if p.spinR != wheel || p.tokX != dot || p.b.bounds() != board {
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
		clear(p.b.v.count)
		for i, left := 0, count; left > 0; i++ {
			n := min(left, 255)
			p.b.v.owner[i], p.b.v.count[i] = 0, uint8(n)
			left -= n
		}
		p.tickRail()
		p.drawOrbCount(dst, a, 0, p.tokX, p.tokY, p.tokR, 0)
		if p.orbS[0] != strconv.Itoa(count) || p.orbLabels[0].key.value != p.orbS[0] {
			t.Fatalf("indicator and player list must follow the shown board: %d, %q", count, p.orbLabels[0].key.value)
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
	if p.wheel != wheel || p.pointer != pointer || p.orbLabels[11].key.value != "99999" {
		t.Fatal("count and mark changes must update without rebuilding the cached wheel")
	}
	p.alive[3], p.orbS[3] = false, "12345"
	p.drawSpinner(dst, a)
	if p.orbLabels[3].key.value != "99999" {
		t.Fatal("eliminated players must not draw a count on their empty slice")
	}
}
