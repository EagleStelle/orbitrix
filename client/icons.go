package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/client/internal/ui"
)

// Icons for round icon buttons, drawn from lines in an 18 px box.
const (
	iconNew    = iota // a plus
	iconImport        // an arrow into a tray
	iconEdit          // a pencil
	iconCopy          // two sheets
	iconShare         // three joined points
	iconExport        // an arrow out of a tray
	iconDelete        // a bin
	iconUse           // a check

	// The Rules card's options, drawn bolder and cached by gameBlock.renderIcons.
	iconDevice   // a phone
	iconWifi     // signal arcs over a dot
	iconOnline   // a globe
	iconDrop     // an orb falling to a floor
	iconCombine  // two orbs merging
	iconTurns    // two opposite arrows
	iconRealtime // a lightning bolt
	iconFog      // staggered fog lines
	iconTiles    // dealt cards
	iconEnd
)

const ruleIconCount = iconEnd - iconDevice

// iconButton draws a round button holding icon alone; a disabled one is dimmed and ignores hover.
func iconButton(dst *ebiten.Image, a *app, r ui.Rect, icon int, style ui.ButtonStyle, enabled bool) {
	hot := a.over(ui.CompactHit(r, a.u))
	ink := ui.DrawButtonSurface(dst, a.ui(), r, style, enabled, hot)
	fill, _ := ui.ButtonColors(a.sc, style, enabled, hot)
	drawIcon(dst, icon, r.Cx(), r.Cy(), a.u, ink, fill)
}

// drawIcon draws icon centred at (x, y) in ink; ground is what it sits on, for the shapes that cover others.
func drawIcon(dst *ebiten.Image, icon int, x, y, u float64, ink, ground color.RGBA) {
	w := 2.25 * u
	if icon >= iconDevice {
		w = 2.75 * u // the Rules options are drawn larger, and bolder
	}
	line := func(x0, y0, x1, y1 float64) { ui.Line(dst, x+x0*u, y+y0*u, x+x1*u, y+y1*u, w, ink) }
	box := func(bx, by, s float64) { // a rounded square outline
		ui.StrokeRound(dst, ui.Rect{X: x + bx*u, Y: y + by*u, W: s * u, H: s * u}, 2*u, w, ink)
	}
	tray := func() {
		line(-8, 3, -8, 8)
		line(-8, 8, 8, 8)
		line(8, 8, 8, 3)
	}
	switch icon {
	case iconNew:
		line(-8, 0, 8, 0)
		line(0, -8, 0, 8)
	case iconImport:
		line(0, -9, 0, 2)
		line(-4, -2, 0, 2)
		line(4, -2, 0, 2)
		tray()
	case iconExport:
		line(0, 3, 0, -9)
		line(-4, -5, 0, -9)
		line(4, -5, 0, -9)
		tray()
	case iconEdit:
		ui.Line(dst, x-3*u, y+3*u, x+6*u, y-6*u, 5.5*u, ink) // the shaft
		line(-8.5, 8.5, -7, 7)                               // its point
	case iconCopy:
		box(-9, -9, 12)
		ui.FillRound(dst, ui.Rect{X: x - 4.5*u, Y: y - 4.5*u, W: 15 * u, H: 15 * u}, 3*u, ground)
		box(-3, -3, 12)
	case iconShare:
		line(-6, 0, 6, -7)
		line(-6, 0, 6, 7)
		for _, p := range [3][2]float64{{-6, 0}, {6, -7}, {6, 7}} {
			ui.FillCircle(dst, x+p[0]*u, y+p[1]*u, 3.5*u, ink)
		}
	case iconDelete:
		line(-8, -6, 8, -6)
		line(-3, -9, 3, -9)
		line(-6, -6, -5, 9)
		line(6, -6, 5, 9)
		line(-5, 9, 5, 9)
		line(0, -2, 0, 5)
	case iconDevice:
		ui.StrokeRound(dst, ui.Rect{X: x - 8*u, Y: y - 13*u, W: 16 * u, H: 26 * u}, 4*u, w, ink)
		line(-2, 8.5, 2, 8.5)
	case iconWifi:
		ui.FillCircle(dst, x, y+10*u, 2.8*u, ink)
		for _, r := range [3]float64{7, 13.5, 20} {
			iconPath.Reset()
			iconPath.Arc(float32(x), float32(y+10*u), float32(r*u), -3*math.Pi/4, -math.Pi/4, vector.Clockwise)
			ui.StrokePath(dst, &iconPath, w, ink)
		}
	case iconOnline:
		ui.Ring(dst, x, y, 12*u, w, ink)
		line(-11, 0, 11, 0)
		ui.StrokeRound(dst, ui.Rect{X: x - 5.5*u, Y: y - 12*u, W: 11 * u, H: 24 * u}, 5.5*u, w, ink)
	case iconDrop:
		ui.FillCircle(dst, x, y-9*u, 4.5*u, ink)
		line(0, -1, 0, 7)
		line(-4, 3, 0, 7)
		line(4, 3, 0, 7)
		line(-11, 12.5, 11, 12.5)
	case iconCombine:
		ui.Ring(dst, x-5.5*u, y, 7.5*u, w, ink)
		ui.Ring(dst, x+5.5*u, y, 7.5*u, w, ink)
	case iconTurns:
		line(-10, -5.5, 10, -5.5)
		line(6, -9.5, 10, -5.5)
		line(6, -1.5, 10, -5.5)
		line(10, 5.5, -10, 5.5)
		line(-6, 1.5, -10, 5.5)
		line(-6, 9.5, -10, 5.5)
	case iconRealtime:
		iconPath.Reset()
		for i, p := range [6][2]float64{{4, -14}, {-9, 2}, {-1, 2}, {-4, 14}, {9, -2}, {1, -2}} {
			if px, py := float32(x+p[0]*u), float32(y+p[1]*u); i == 0 {
				iconPath.MoveTo(px, py)
			} else {
				iconPath.LineTo(px, py)
			}
		}
		iconPath.Close()
		ui.FillPath(dst, &iconPath, ink)
	case iconFog:
		line(-11, -7, 5, -7)
		line(-5, 0, 11, 0)
		line(-11, 7, 3, 7)
	case iconTiles:
		line(-11, 8, -11, -9) // the card behind
		line(-8, -12, 3, -12)
		ui.StrokeRound(dst, ui.Rect{X: x - 5*u, Y: y - 7*u, W: 16 * u, H: 20 * u}, 4*u, w, ink)
		ui.FillRound(dst, ui.Rect{X: x, Y: y, W: 6 * u, H: 6 * u}, 1.5*u, ink)
	case iconUse:
		ui.Line(dst, x-8*u, y+u, x-2.5*u, y+6.5*u, 2.75*u, ink)
		ui.Line(dst, x-2.5*u, y+6.5*u, x+8*u, y-5.5*u, 2.75*u, ink)
	}
}

// ruleIconSide is the side of a Rules option's icon, in logical px.
const ruleIconSide = 36

var iconPath vector.Path

// renderIcons caches the option icons at scale u: some are arcs and filled shapes, which only a path draws.
func (g *gameBlock) renderIcons(u float64) {
	g.iconSide = ruleIconSide * u
	white := color.RGBA{255, 255, 255, 255}
	for i := range g.icons {
		if g.icons[i] != nil {
			g.icons[i].Deallocate()
		}
		g.icons[i] = ebiten.NewImage(int(g.iconSide)+1, int(g.iconSide)+1)
		drawIcon(g.icons[i], iconDevice+i, g.iconSide/2, g.iconSide/2, u, white, white)
	}
}
