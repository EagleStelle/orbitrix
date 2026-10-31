//go:build capture

package client

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

type setupCaptureCase struct {
	name        string
	w, h        float64
	players     int
	cols, rows  int
	dark, tail  bool
	menu, small bool
	play        bool
	hover       string
	progress    float64 // fraction of page or player-list scroll
	count       int     // synthetic indicator stress count; 0 keeps the actual empty-board values
	marks       bool
	turn        int
}

type setupCaptureGame struct {
	dir   string
	cases []setupCaptureCase
	n     int
	err   error
}

func (g *setupCaptureGame) Layout(int, int) (int, int) { return 1440, 900 }
func (g *setupCaptureGame) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.n == len(g.cases) {
		return ebiten.Termination
	}
	return nil
}

func (g *setupCaptureGame) Draw(*ebiten.Image) {
	if g.n >= len(g.cases) || g.err != nil {
		return
	}
	c := g.cases[g.n]
	a, s := setupTestApp(c.w, c.h, 1, c.players), newSetup()
	if c.dark {
		a.sc, a.set.Marks = &schemes[themeDark], true
	}
	a.set.Marks = a.set.Marks || c.marks
	a.set.seats[1], _ = parseSeat("conqueror")
	if c.cols > 0 {
		a.set.Cols, a.set.Rows = c.cols, c.rows
	}
	s.layout(a)
	s.scroll = c.progress * s.maxScroll
	if c.tail {
		s.scroll = s.maxScroll
	}
	if c.menu {
		s.menu = 1
	}
	if c.small {
		a.set.Cols, a.set.Rows = 3, 3
	}
	var hover rect
	switch c.hover {
	case "player":
		hover = s.seatTy[1]
	case "preset":
		hover = s.presetPills[shapePortrait][0]
	case "stepper":
		hover = s.plusP
	}
	if hover.w > 0 {
		a.in.x, a.in.y = hover.cx(), hover.cy()
	}
	out := ebiten.NewImage(int(c.w), int(c.h))
	defer out.Deallocate()
	out.Fill(a.sc.ground)
	if c.play {
		p, err := newPlay(a, a.set.Cols, a.set.Rows, a.set.seats)
		if err != nil {
			g.err = err
			return
		}
		p.tickRail()
		if c.count > 0 {
			for i := range p.seats {
				p.orbs[i], p.orbS[i] = c.count-i, strconv.Itoa(c.count-i)
			}
		}
		p.shown, p.spin = c.turn, p.wedgeAngle(c.turn)
		p.chain, p.chainHold, p.chainText = 88, calloutHold, "Chain ×88"
		p.layout(a)
		p.seatView.offset = c.progress * p.seatView.maxScroll()
		if c.tail {
			p.seatView.offset = p.seatView.maxScroll()
		}
		p.layoutScrolled(a)
		p.draw(out, a)
	} else {
		s.draw(out, a)
	}
	pixels := image.NewRGBA(image.Rect(0, 0, int(c.w), int(c.h)))
	out.ReadPixels(pixels.Pix)
	f, err := os.Create(filepath.Join(g.dir, c.name+".png"))
	if err == nil {
		err = png.Encode(f, pixels)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	g.err = err
	g.n++
}

func TestCaptureSetup(t *testing.T) {
	dir := os.Getenv("ORBITRIX_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set ORBITRIX_CAPTURE_DIR to capture setup viewports")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &setupCaptureGame{dir: dir, cases: []setupCaptureCase{
		{name: "desktop", w: 1440, h: 900, players: 2},
		{name: "setup-all-shapes", w: 1440, h: 1080, players: 2},
		{name: "setup-desktop-12p", w: 1440, h: 900, players: 12},
		{name: "setup-preview-background", w: 1881, h: 851, players: 12, dark: true},
		{name: "setup-preview-quarter", w: 1881, h: 851, players: 12, dark: true, progress: 0.25},
		{name: "setup-preview-mid", w: 1881, h: 851, players: 12, dark: true, progress: 0.5},
		{name: "setup-preview-three-quarters", w: 1881, h: 851, players: 12, dark: true, progress: 0.75},
		{name: "setup-preview-background-bottom", w: 1881, h: 851, players: 12, dark: true, tail: true},
		{name: "setup-1146-12p", w: 1146, h: 640, players: 12},
		{name: "setup-tablet-12p", w: 760, h: 844, players: 12},
		{name: "user-1040", w: 1040, h: 640, players: 2},
		{name: "setup-dimensions-desktop", w: 1040, h: 640, players: 2, cols: 17, rows: 10, dark: true},
		{name: "setup-dimensions-mobile", w: 390, h: 844, players: 2, cols: 17, rows: 10, dark: true},
		{name: "setup-1040-12p", w: 1040, h: 640, players: 12},
		{name: "setup-1040-12p-bottom", w: 1040, h: 640, players: 12, tail: true},
		{name: "setup-dark-12p", w: 1040, h: 640, players: 12, dark: true},
		{name: "mobile", w: 390, h: 844, players: 2},
		{name: "setup-mobile-12p", w: 390, h: 844, players: 12, dark: true},
		{name: "setup-mobile-12p-bottom", w: 390, h: 844, players: 12, dark: true, tail: true},
		{name: "setup-landscape", w: 844, h: 390, players: 12},
		{name: "setup-landscape-mid", w: 844, h: 390, players: 12, progress: 0.5},
		{name: "setup-menu", w: 1040, h: 640, players: 2, menu: true},
		{name: "setup-board-too-small", w: 1040, h: 640, players: 12, small: true, tail: true},
		{name: "setup-player-hover", w: 1040, h: 640, players: 2, hover: "player"},
		{name: "setup-preset-hover", w: 1040, h: 640, players: 2, hover: "preset"},
		{name: "setup-stepper-hover", w: 1040, h: 640, players: 2, hover: "stepper"},
		{name: "setup-dark-player-hover", w: 1040, h: 640, players: 4, dark: true, hover: "player"},
		{name: "setup-dark-preset-hover", w: 1040, h: 640, players: 4, dark: true, hover: "preset"},
		{name: "play-desktop", w: 1440, h: 900, players: 2, play: true},
		{name: "play-desktop-12p", w: 1040, h: 640, players: 12, play: true},
		{name: "play-wide-12p", w: 1920, h: 1080, players: 12, play: true, dark: true},
		{name: "play-responsive-8p", w: 1919, h: 1004, players: 8, play: true},
		{name: "play-responsive-12p", w: 1919, h: 1004, players: 12, play: true, dark: true},
		{name: "play-responsive-narrow", w: 1340, h: 1004, players: 12, play: true},
		{name: "play-mobile", w: 390, h: 844, players: 2, play: true},
		{name: "play-mobile-12p", w: 360, h: 740, players: 12, play: true, dark: true},
		{name: "play-mobile-8p", w: 390, h: 844, players: 8, play: true},
		{name: "play-mobile-placemat", w: 390, h: 844, players: 4, cols: 6, rows: 9, play: true},
		{name: "play-mobile-portrait-6p", w: 390, h: 844, players: 6, cols: 13, rows: 18, play: true},
		{name: "play-mobile-portrait-12p", w: 412, h: 915, players: 12, cols: 13, rows: 18, play: true, dark: true},
		{name: "play-mobile-portrait-limited", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true},
		{name: "play-mobile-portrait-scrolled", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, tail: true},
		{name: "play-mobile-tall-board", w: 360, h: 740, players: 12, cols: 3, rows: 40, play: true, dark: true},
		{name: "play-mobile-tall-board-players-scrolled", w: 360, h: 740, players: 12, cols: 3, rows: 40, play: true, dark: true, tail: true},
		{name: "play-tablet-turned", w: 768, h: 1024, players: 6, cols: 30, rows: 18, play: true, dark: true},
		{name: "play-tablet-turned-short", w: 768, h: 800, players: 12, cols: 30, rows: 18, play: true},
		{name: "play-landscape", w: 844, h: 390, players: 12, cols: 30, rows: 18, play: true},
		{name: "play-landscape-short", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, dark: true},
		{name: "play-landscape-scrolled", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, dark: true, tail: true},
		{name: "counts-landscape-3", w: 844, h: 390, players: 12, cols: 30, rows: 18, play: true, count: 999},
		{name: "counts-landscape-5", w: 780, h: 360, players: 12, cols: 30, rows: 18, play: true, count: 99999, dark: true},
		{name: "counts-landscape-small-5", w: 640, h: 320, players: 12, cols: 30, rows: 18, play: true, count: 99999, marks: true},
		{name: "counts-desktop-5", w: 1440, h: 900, players: 12, play: true, count: 99999},
		{name: "counts-portrait-3", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 999},
		{name: "counts-portrait-4", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 9999, marks: true},
		{name: "counts-portrait-5", w: 320, h: 640, players: 12, cols: 13, rows: 18, play: true, count: 99999},
		{name: "counts-portrait-dark-5", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 99999, dark: true},
		{name: "counts-portrait-blue-5", w: 360, h: 740, players: 12, cols: 13, rows: 18, play: true, count: 99999, marks: true, turn: 1},
	}}
	ebiten.SetWindowSize(160, 100)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowPosition(-32000, -32000)
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
}
