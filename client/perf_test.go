//go:build perf

package client

import (
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/core"
	"orbitrix/core/ai"
)

// perfCase is one screen the frame-time check holds at 60 FPS, at a screen size in logical px and a UI scale.
type perfCase struct {
	name    string
	w, h, u float64
	scroll  bool
	scene   func(a *app) scene
}

// perfStats is what one case recorded.
type perfStats struct {
	gaps, upd, drw        []time.Duration // between consecutive Draws, and each Update and Draw
	games, moves, biggest int             // games, AI moves after the fill, and the longest chain
	idle                  bool
}

// perfGame plays the cases one after another in one window (Ebitengine runs one game per process) and records how
// long each frame and each Update/Draw took.
type perfGame struct {
	cases []perfCase
	stats []perfStats
	secs  int
	i     int
	game  *Game
	end   time.Time
	last  time.Time
	frame int
}

func (g *perfGame) Layout(int, int) (int, int) {
	c := g.cases[min(g.i, len(g.cases)-1)]
	return int(math.Ceil(c.w * c.u)), int(math.Ceil(c.h * c.u))
}

// start sets up case i: twelve seats, a 40×40 board, nothing under the pointer.
func (g *perfGame) start() {
	c := g.cases[g.i]
	g.game = &Game{a: *setupTestApp(c.w, c.h, c.u, core.MaxPlayers), fresh: true}
	a := &g.game.a
	a.set.Cols, a.set.Rows = 40, 40
	a.set.Marks = os.Getenv("ORBITRIX_PERF_MARKS") != ""
	a.rng = core.NewRNG(7)
	a.in.x, a.in.y = offScreen, offScreen
	g.game.scene = c.scene(a)
	g.stats[g.i].idle = still(g.game.scene) && !c.scroll
	g.end = time.Now().Add(time.Duration(g.secs) * time.Second)
	g.last, g.frame = time.Time{}, 0
	if p, ok := g.game.scene.(*playScene); ok {
		g.stats[g.i].moves -= p.game.Moves
	}
}

func (g *perfGame) Update() error {
	if g.game == nil || time.Now().After(g.end) {
		if g.game != nil {
			g.finish()
			g.i++
		}
		if g.i == len(g.cases) {
			return ebiten.Termination
		}
		g.start()
	}
	switch s := g.game.scene.(type) {
	case *playScene:
		if s.over && !still(s) {
			g.finish()
			g.game.scene = denseGame(&g.game.a)
			g.game.fresh = true
			g.last = time.Time{} // the fill is the harness's own work, not a frame
			g.stats[g.i].moves -= g.game.scene.(*playScene).game.Moves
		}
	case *setupScene:
		if g.cases[g.i].scroll {
			// Synthetic scroll input still draws through the production redraw guard.
			s.layout(&g.game.a)
			s.scroll = s.maxScroll * math.Abs(math.Mod(float64(g.frame)/120, 2)-1)
			g.game.fresh = true
		}
	}
	// Mouse movement outside the test must not turn an idle case into a hover test.
	in := &g.game.a.in
	in.mx, in.my = ebiten.CursorPosition()
	in.x, in.y, in.finger = offScreen, offScreen, true
	t := time.Now()
	err := g.game.Update()
	g.stats[g.i].upd = append(g.stats[g.i].upd, time.Since(t))
	return err
}

// finish counts the game being played, if the case plays one.
func (g *perfGame) finish() {
	if p, ok := g.game.scene.(*playScene); ok {
		st := &g.stats[g.i]
		st.games++
		st.moves += p.game.Moves
		st.biggest = max(st.biggest, p.biggest)
	}
}

func (g *perfGame) Draw(screen *ebiten.Image) {
	if g.game == nil {
		return
	}
	now := time.Now()
	st := &g.stats[g.i]
	if !g.last.IsZero() {
		st.gaps = append(st.gaps, now.Sub(g.last))
	}
	g.last = now
	draw := g.game.fresh
	g.game.Draw(screen)
	if draw {
		st.drw = append(st.drw, time.Since(now))
	}
	g.frame++
}

// denseGame starts a 40×40 game for twelve AIs and plays random moves until the board is about two-thirds full.
func denseGame(a *app) *playScene {
	seats := make([]int, core.MaxPlayers)
	for i := range seats {
		seats[i] = 1 + i%int(ai.NumStrategies)
	}
	p, err := newPlay(a, 40, 40, seats)
	if err != nil {
		panic(err)
	}
	var bot ai.Bot
	rng := core.NewRNG(p.seed)
	total := 0
	for i := range p.game.Geo.Cells() {
		total += int(p.game.Geo.Cap[i])
	}
	// Random play ends in a win long before the board fills, so the fill stops at the move before the first player is
	// knocked out.
	prev := p.game.Clone()
	for orbs := 0; orbs < total*2/3; orbs++ {
		prev.CopyFrom(p.game)
		if _, err := p.game.Apply(bot.Choose(p.game, ai.Jester, rng.Uint64(), 1)); err != nil {
			panic(err)
		}
		if p.game.Moves > len(seats) && !allAlive(p.game) {
			p.game.CopyFrom(prev)
			break
		}
	}
	p.b.Sync(p.game)
	p.settle()
	return p
}

func allAlive(g *core.Game) bool {
	for i := range g.Players {
		if !g.Alive(i) {
			return false
		}
	}
	return true
}

func percentile(d []time.Duration, q float64) time.Duration {
	s := slices.Clone(d)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s))*q))]
}

func ms(d time.Duration) string {
	return strconv.FormatFloat(float64(d.Microseconds())/1000, 'f', 2, 64) + " ms"
}

// TestPerf runs every case for ORBITRIX_PERF_SECONDS and fails a case whose p99 frame gap misses 60 FPS by a margin.
func TestPerf(t *testing.T) {
	secs, _ := strconv.Atoi(os.Getenv("ORBITRIX_PERF_SECONDS"))
	if secs == 0 {
		t.Skip("set ORBITRIX_PERF_SECONDS (per screen) to run the frame-time check")
	}
	const phoneDPR = 2.625 // a 1080×2400 phone, drawn at maxScale like Layout does
	phoneW, phoneH, phoneU := 1080/phoneDPR, 2400/phoneDPR, min(phoneDPR, maxScale)
	game := func(a *app) scene { return denseGame(a) }
	circuits := func(a *app) scene {
		m := core.NewMap(40, 40)
		for x := 1; x+1 < m.Cols; x += 2 {
			for y := 1; y+1 < m.Rows; y += 2 {
				group := uint8((x/2*7+y/2)%core.MaxMapGroups + 1)
				m.Cells[x*m.Rows+y] = core.MapCell{Circuit: core.Latch + core.CircuitKind((x/2+y/2)%3), Group: group}
				m.Cells[(x+1)*m.Rows+y] = core.MapCell{Circuit: core.Door, Group: group, InitiallyOpen: true}
				m.Edges = append(m.Edges, core.MapEdge{A: x*m.Rows + y + 1, B: (x+1)*m.Rows + y + 1, Kind: core.Gate, Group: group, InitiallyOpen: true})
			}
		}
		a.set.Map = &m
		return denseGame(a)
	}
	paused := func(a *app) scene { p := denseGame(a); p.paused = true; return p }
	results := func(a *app) scene {
		p := denseGame(a)
		p.game.Winner = 0
		p.settle()
		p.overT = wipeTicks
		return p
	}
	setup := func(*app) scene { return newSetup() }
	settings := func(*app) scene { return newSettingsMenu(newHome()) }
	home := func(*app) scene { return newHome() }
	cases := []perfCase{
		{"game 40×40, 12 AIs", 1920, 1080, 1, false, game},
		{"game 40×40, 12 AIs, phone", phoneW, phoneH, phoneU, false, game},
		{"circuits 40×40, 12 AIs", 1920, 1080, 1, false, circuits},
		{"circuits 40×40, 12 AIs, phone", phoneW, phoneH, phoneU, false, circuits},
		{"game paused, phone", phoneW, phoneH, phoneU, false, paused},
		{"results, phone", phoneW, phoneH, phoneU, false, results},
		{"setup idle, phone", phoneW, phoneH, phoneU, false, setup},
		{"settings idle, phone", phoneW, phoneH, phoneU, false, settings},
		{"setup 12 players 40×40 scrolling", 1440, 900, 1, true, setup},
		{"setup 12 players 40×40 scrolling, phone", phoneW, phoneH, phoneU, true, setup},
		{"home", 1920, 1080, 1, false, home},
		{"home, phone", phoneW, phoneH, phoneU, false, home},
	}
	if filter := os.Getenv("ORBITRIX_PERF_CASE"); filter != "" {
		cases = slices.DeleteFunc(cases, func(c perfCase) bool { return !strings.Contains(c.name, filter) })
		if len(cases) == 0 {
			t.Fatalf("no performance case matches %q", filter)
		}
	}
	n := secs * 1000 // room for 1000 FPS, so recording never allocates mid-run
	g := &perfGame{cases: cases, stats: make([]perfStats, len(cases)), secs: secs}
	for i := range g.stats {
		g.stats[i] = perfStats{gaps: make([]time.Duration, 0, n), upd: make([]time.Duration, 0, n),
			drw: make([]time.Duration, 0, n)}
	}
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetVsyncEnabled(os.Getenv("ORBITRIX_PERF_NOVSYNC") == "")
	ebiten.SetWindowSize(1920, 1080)
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&m1)
	frames := 0
	for i, c := range cases {
		frames += len(g.stats[i].gaps) + 1
		st := g.stats[i]
		if len(st.gaps) == 0 || len(st.drw) == 0 {
			t.Errorf("%s: no frames", c.name)
			continue
		}
		slow := 0
		for _, d := range st.gaps {
			if d > 25*time.Millisecond {
				slow++
			}
		}
		t.Logf("%s: %.1f FPS; frame gap p50 %s p99 %s max %s, over 25 ms: %d; update p99 %s; draw p99 %s", c.name,
			float64(len(st.gaps)+1)/float64(secs), ms(percentile(st.gaps, 0.5)), ms(percentile(st.gaps, 0.99)),
			ms(slices.Max(st.gaps)), slow, ms(percentile(st.upd, 0.99)), ms(percentile(st.drw, 0.99)))
		if st.moves > 0 {
			t.Logf("%s: games %d, AI moves %d, biggest chain ×%d", c.name, st.games, st.moves, st.biggest)
		}
		if p99 := percentile(st.gaps, 0.99); p99 > 25*time.Millisecond {
			t.Errorf("%s: p99 frame gap %s, below 60 FPS", c.name, ms(p99))
		}
		t.Logf("%s: %d scene redraws", c.name, len(st.drw))
		if st.idle && len(st.drw) > secs*60/stillRedraw+2 {
			t.Errorf("%s: idle screen redrew %d times instead of about twice per second", c.name, len(st.drw))
		}
	}
	t.Logf("allocations per frame %.1f (includes Ebitengine, the AI goroutine and the dense-board fills), GC cycles %d",
		float64(m1.Mallocs-m0.Mallocs)/float64(frames), m1.NumGC-m0.NumGC)
}
