//go:build perf

package client

import (
	"os"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"orbitrix/core"
	"orbitrix/core/ai"
)

// perfGame plays twelve AIs on a 40×40 board, filled to late-game density first, in a real window, and records how
// long each frame and each Update/Draw took. It checks the Phase 3 bar: a steady 60 FPS on a 40×40 board.
type perfGame struct {
	a       *app
	p       *playScene
	end     time.Time
	last    time.Time
	gaps    []time.Duration // between consecutive Draws
	upd     []time.Duration
	drw     []time.Duration
	biggest int
	games   int
	moves   int // AI moves played after the fill
}

func (g *perfGame) Layout(int, int) (int, int) { return 1920, 1080 }

func (g *perfGame) Update() error {
	if time.Now().After(g.end) {
		return ebiten.Termination
	}
	if g.p == nil || g.p.over {
		if g.p != nil {
			g.biggest = max(g.biggest, g.p.biggest)
			g.moves += g.p.game.Moves
		}
		g.p = denseGame(g.a)
		g.last = time.Time{} // the fill is the harness's own work, not a frame
		g.games++
		g.moves -= g.p.game.Moves
	}
	t := time.Now()
	err := g.p.update(g.a)
	g.upd = append(g.upd, time.Since(t))
	return err
}

func (g *perfGame) Draw(screen *ebiten.Image) {
	now := time.Now()
	if !g.last.IsZero() {
		g.gaps = append(g.gaps, now.Sub(g.last))
	}
	g.last = now
	screen.Fill(&g.a.sc.ground)
	g.p.draw(screen, g.a)
	g.drw = append(g.drw, time.Since(now))
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
	p.b.sync(p.game)
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

func TestPerf40x40(t *testing.T) {
	secs, _ := strconv.Atoi(os.Getenv("ORBITRIX_PERF_SECONDS"))
	if secs == 0 {
		t.Skip("set ORBITRIX_PERF_SECONDS to run the 40×40 frame-time check")
	}
	a := setupTestApp(1920, 1080, 1, 2)
	a.set.Marks = os.Getenv("ORBITRIX_PERF_MARKS") != ""
	a.rng = core.NewRNG(7)
	n := secs * 1000 // room for 1000 FPS, so recording never allocates mid-run
	g := &perfGame{a: a, end: time.Now().Add(time.Duration(secs) * time.Second),
		gaps: make([]time.Duration, 0, n), upd: make([]time.Duration, 0, n), drw: make([]time.Duration, 0, n)}
	ebiten.SetVsyncEnabled(os.Getenv("ORBITRIX_PERF_NOVSYNC") == "")
	ebiten.SetWindowSize(1920, 1080)
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&m1)
	g.biggest = max(g.biggest, g.p.biggest)
	g.moves += g.p.game.Moves
	slow := 0
	for i, d := range g.gaps {
		if d > 25*time.Millisecond {
			slow++
			t.Logf("slow frame %d of %d: %s", i+1, len(g.gaps), ms(d))
		}
	}
	t.Logf("frames %d in %ds (%.1f FPS), games %d, AI moves %d, biggest chain ×%d", len(g.gaps)+1, secs,
		float64(len(g.gaps)+1)/float64(secs), g.games, g.moves, g.biggest)
	t.Logf("frame gap p50 %s p99 %s max %s, gaps over 25 ms: %d", ms(percentile(g.gaps, 0.5)),
		ms(percentile(g.gaps, 0.99)), ms(slices.Max(g.gaps)), slow)
	t.Logf("update p99 %s max %s; draw p99 %s max %s", ms(percentile(g.upd, 0.99)), ms(slices.Max(g.upd)),
		ms(percentile(g.drw, 0.99)), ms(slices.Max(g.drw)))
	t.Logf("allocations per frame %.1f (includes Ebitengine and the AI goroutine), GC cycles %d",
		float64(m1.Mallocs-m0.Mallocs)/float64(len(g.drw)), m1.NumGC-m0.NumGC)
}
