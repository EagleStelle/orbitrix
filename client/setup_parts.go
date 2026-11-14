package client

import (
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
	"orbitrix/core"
)

// Where a game is played, in the order the Rules card's Play on group offers them.
const (
	whereDevice = iota
	whereWifi
	whereOnline
)

// gameRows are the Rules card's groups: what each offers, as a label and an icon, and which options are built (a
// bit per option). The rest are listed dimmed until their rules exist.
var gameRows = [...]struct {
	label string
	opts  []string
	icons []int
	built uint8
}{
	{"Play on", []string{"This Device", "Same Wi-Fi", "Online"}, []int{iconDevice, iconWifi, iconOnline}, 0b111},
	{"Mode", []string{"Turn-based", "Real-time"}, []int{iconTurns, iconRealtime}, 0b01},
	{"Add-ons", []string{"Drop", "Combine", "Fog of war", "Tiles"}, []int{iconDrop, iconCombine, iconFog, iconTiles}, 0},
}

// The Rules card's option tiles are squares, ruleTile px a side and ruleGap apart; its groups stack ruleRowGap apart.
const (
	ruleTile   = 92
	ruleGap    = 8
	ruleRowGap = 12
)

// gameBlock is the Rules card a host's Setup and a joiner's room share: Play on, Mode and Add-ons as groups of
// square option tiles, each an icon over its label, stacked one group a row. The rows scroll sideways where the card
// is narrower than its widest group.
type gameBlock struct {
	r     ui.Rect
	bp    float64
	strip ui.Scroll                 // the groups' rows, scrolling sideways together
	tiles [len(gameRows)][4]ui.Rect // by group and option
	ly    [len(gameRows)]float64    // each group's label
	join  ui.Rect                   // the heading's Join a room button

	icons    [ruleIconCount]*ebiten.Image // the option icons in white, tinted when drawn
	iconSide float64
}

// layout places the card at (x, y), open or collapsed to its heading, and returns its height, which y does not
// change.
func (g *gameBlock) layout(a *app, x, y, w, bp float64, open bool) float64 {
	u := a.u
	side := ruleTile * u
	pitch := 24*u + side + ruleRowGap*u // a group's label, its tiles, then the gap to the next group
	widest := 0.0
	for _, row := range gameRows {
		widest = max(widest, float64(len(row.opts))*(side+ruleGap*u)-ruleGap*u)
	}
	g.strip.Horizontal = true
	vh := float64(len(gameRows))*pitch - (ruleRowGap-6)*u
	if !open {
		vh = 0 // collapsed to its heading: no tile takes a click
	}
	g.strip.Set(ui.Rect{X: x + bp, Y: y + bp + 50*u, W: w - 2*bp, H: vh}, widest)
	for i, row := range gameRows {
		ty := g.strip.Viewport.Y + float64(i)*pitch
		g.ly[i] = ty + 10*u
		for o := range row.opts {
			g.tiles[i][o] = ui.Rect{X: g.strip.Viewport.X - g.strip.Offset + float64(o)*(side+ruleGap*u), Y: ty + 24*u, W: side, H: side}
		}
	}
	h := 2*bp + 50*u + vh // the first label's text starts 12 px under the heading's button
	if !open {
		h = 2*bp + 44*u
	}
	g.r, g.bp = ui.Rect{X: x, Y: y, W: w, H: h}, bp
	jw := ui.TextWidth("Join a room", a.f.LabelBold) + 40*u
	g.join = ui.Rect{X: x + w - bp - foldRoom*u - jw, Y: y + bp + (22-ui.CompactButtonHeight/2)*u, W: jw, H: ui.CompactButtonHeight * u}
	return h
}

// scrolled follows a sideways drag or the wheel over the strip and reports whether it moved, so the page lays out
// again.
func (g *gameBlock) scrolled(a *app) bool { return g.strip.Update(a.ui()) }

// hit is the option of group i clicked inside the strip, or -1.
func (g *gameBlock) hit(a *app, i int) int {
	if !g.strip.Viewport.Contains(a.in.x, a.in.y) {
		return -1
	}
	for o := range gameRows[i].opts {
		if a.clicked(g.tiles[i][o]) {
			return o
		}
	}
	return -1
}

// draw draws the card with where chosen. Options in whereOff are dimmed; locked dims every group, for a joiner.
// Its caller draws the heading's Join a room button over it. A foldable card shows its chevron; collapsed (not open),
// only its heading is drawn.
func (g *gameBlock) draw(dst *ebiten.Image, a *app, where int, whereOff uint8, locked, open, foldable bool) {
	sc, u := a.sc, a.u
	ui.FillRound(dst, g.r, 24*u, sc.Tile)
	ui.DrawText(dst, "Rules", a.f.Heading, g.r.X+g.bp, g.r.Y+g.bp+22*u, text.AlignStart, sc.Ink)
	if foldable {
		foldChevron(dst, a, g.r.Right()-g.bp, g.r.Y+g.bp+22*u, open)
	}
	if !open {
		return
	}
	if g.iconSide != ruleIconSide*u {
		g.renderIcons(u)
	}
	clip := g.strip.Clip(dst)
	over := g.strip.Viewport.Contains(a.in.x, a.in.y)
	for i, row := range gameRows {
		ui.DrawText(clip, row.label, a.f.SmallBold, g.strip.Viewport.X, g.ly[i], text.AlignStart, sc.Dim)
		on, off := uint8(1), ^row.built
		if i == 0 {
			on, off = 1<<where, whereOff
		}
		if i == len(gameRows)-1 {
			on = 0 // add-ons are each on or off, any number together, and none is built
		}
		if locked {
			off = 0xff
		}
		for o, label := range row.opts {
			r, enabled := g.tiles[i][o], off&(1<<o) == 0
			fill, ink := ui.ButtonColors(sc, ui.ButtonGround, enabled, over && a.over(r))
			if !enabled {
				fill = ui.Mix(sc.Tile, sc.Ground, 0.4) // a dimmed tile keeps its square
			}
			ui.FillRound(clip, r, 16*u, fill)
			if on&(1<<o) != 0 {
				ui.StrokeRound(clip, r, 16*u, 2.5*u, sc.Ink)
			}
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(r.Cx()-g.iconSide/2, r.Y+14*u)
			ui.Tint(&op.ColorScale, ink)
			clip.DrawImage(g.icons[row.icons[o]-iconDevice], &op)
			ui.DrawText(clip, label, a.f.SmallBold, r.Cx(), r.Bottom()-18*u, text.AlignCenter, ink)
		}
	}
	g.strip.DrawScrollbar(dst, a.ui())
}

// sizePicker is the board size control Setup's Board block and the Map Editor share: the presets by shape, two a
// row, above the Columns and Rows steppers.
type sizePicker struct {
	shapeLabels       [len(shapeNames)]ui.Rect
	presetPills       [len(shapeNames)][len(boardSizes)]ui.Rect
	colsCtl, rowsCtl  ui.Rect
	minusCol, plusCol ui.Rect
	minusRow, plusRow ui.Rect
}

// numberW is the room a stepper keeps for its value: the widest number, padded.
func numberW(a *app) float64 {
	w := 0.0
	for _, number := range ui.Numbers {
		w = max(w, ui.TextWidth(number, a.f.Number))
	}
	return w + 12*a.u
}

// stepRects are a stepper's − and + faces, ending at right and centred on cy.
func stepRects(a *app, right, cy float64) (minus, plus ui.Rect) {
	u := a.u
	face := ui.CompactButtonHeight * u
	plus = ui.Rect{X: right - 42*u, Y: cy - face/2, W: face, H: face}
	return ui.Rect{X: plus.X - numberW(a) - ui.CompactButtonTarget*u, Y: plus.Y, W: face, H: face}, plus
}

// sizeDimensionMin is the least width one dimension row needs: its label, both targets and the widest value.
func sizeDimensionMin(a *app) float64 {
	return max(ui.TextWidth("Columns", a.f.LabelBold), ui.TextWidth("Rows", a.f.LabelBold)) + 8*a.u + 2*ui.CompactButtonTarget*a.u + numberW(a)
}

// sizePresetH is the presets' height: per shape a heading, then three rows of two.
func sizePresetH(u float64) float64 {
	groupH := (32 + 3*ui.CompactButtonTarget + 16) * u
	return float64(len(shapeNames))*groupH + float64(len(shapeNames)-1)*24*u
}

// sizePickerH is the picker's height; stack puts Rows under Columns.
func sizePickerH(u, gap float64, stack bool) float64 {
	h := sizePresetH(u) + gap + ui.CompactButtonTarget*u
	if stack {
		h += ui.CompactButtonTarget*u + gap
	}
	return h
}

// layout places the picker in the w-wide column at (x, y).
func (p *sizePicker) layout(a *app, x, y, w, gap float64, stack bool) {
	u := a.u
	face, target := ui.CompactButtonHeight*u, ui.CompactButtonTarget*u
	pitch := target + 8*u
	groupH := 32*u + 3*target + 16*u
	pw := (w - 8*u) / 2
	for g := range p.presetPills {
		gy := y + float64(g)*(groupH+24*u)
		p.shapeLabels[g] = ui.Rect{X: x, Y: gy, W: w, H: 24 * u}
		for t := range p.presetPills[g] {
			p.presetPills[g][t] = ui.Rect{X: x + float64(t%2)*(pw+8*u), Y: gy + 32*u + float64(t/2)*pitch + (target-face)/2, W: pw, H: face}
		}
	}
	cw := (w - gap) / 2
	p.colsCtl = ui.Rect{X: x, Y: y + sizePresetH(u) + gap, W: cw, H: target}
	p.rowsCtl = ui.Rect{X: x + cw + gap, Y: p.colsCtl.Y, W: cw, H: target}
	if stack {
		p.colsCtl.W = w
		p.rowsCtl = ui.Rect{X: x, Y: p.colsCtl.Bottom() + gap, W: w, H: target}
	}
	p.minusCol, p.plusCol = stepRects(a, p.colsCtl.Right(), p.colsCtl.Cy())
	p.minusRow, p.plusRow = stepRects(a, p.rowsCtl.Right(), p.rowsCtl.Cy())
}

// pick is the size a click asks for, and whether it changes cols × rows.
func (p *sizePicker) pick(a *app, cols, rows int) (int, int, bool) {
	u := a.u
	c, r := cols, rows
	switch {
	case a.clicked(setupStepHit(p.minusCol, u)):
		c--
	case a.clicked(setupStepHit(p.plusCol, u)):
		c++
	case a.clicked(setupStepHit(p.minusRow, u)):
		r--
	case a.clicked(setupStepHit(p.plusRow, u)):
		r++
	default:
		for g, group := range p.presetPills {
			for t, pill := range group {
				if a.clicked(ui.CompactHit(pill, u)) {
					c, r = presetSize(g, t)
				}
			}
		}
	}
	c, r = max(core.MinSize, min(core.MaxSize, c)), max(core.MinSize, min(core.MaxSize, r))
	return c, r, c != cols || r != rows
}

// draw draws the picker with cols × rows chosen.
func (p *sizePicker) draw(dst *ebiten.Image, a *app, cols, rows int) {
	sc := a.sc
	g, tier := shapeOf(cols, rows), presetOf(cols, rows)
	for shape, group := range p.presetPills {
		label := p.shapeLabels[shape]
		ui.DrawText(dst, shapeNames[shape], a.f.LabelBold, label.X, label.Cy(), text.AlignStart, sc.Ink)
		for t, r := range group {
			style := ui.ButtonGround
			if shape == g && t == tier {
				style = ui.ButtonChosen
			}
			ui.DrawButton(dst, a.ui(), r, boardSizes[t].name, presetDims[shape][t], style, true)
		}
	}
	ui.DrawText(dst, "Columns", a.f.LabelBold, p.colsCtl.X, p.colsCtl.Cy(), text.AlignStart, sc.Ink)
	stepper(dst, a, cols, core.MinSize, core.MaxSize, p.minusCol, p.plusCol)
	ui.DrawText(dst, "Rows", a.f.LabelBold, p.rowsCtl.X, p.rowsCtl.Cy(), text.AlignStart, sc.Ink)
	stepper(dst, a, rows, core.MinSize, core.MaxSize, p.minusRow, p.plusRow)
}

// ellipsize is s cut to fit width in face, ending in an ellipsis when it was cut.
func ellipsize(s string, face text.Face, width float64) string {
	if ui.TextWidth(s, face) <= width {
		return s
	}
	for len(s) > 0 && ui.TextWidth(s+"…", face) > width {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s + "…"
}

// sizePresetW is the width a preset pill needs to hug its widest name and dimensions.
func sizePresetW(a *app) float64 {
	w := 0.0
	for g, dims := range presetDims {
		for t := range dims {
			w = max(w, ui.TextWidth(boardSizes[t].name, a.f.LabelBold)+8*a.u+ui.TextWidth(presetDims[g][t], a.f.Small))
		}
	}
	return w + 40*a.u
}

// Setup's Rules and Players cards fold to their headings; at most one is open, and none is at first.
const (
	foldNone = iota
	foldRules
	foldPlayers
)

// foldRoom is the room a folding card's chevron takes at the right end of its heading; heading buttons stand left of
// it.
const foldRoom = 32

// foldChevron draws a folding card's chevron at the very right of its heading, which ends at right and is centred on
// y: pointing down while the card is collapsed, up while it is open.
func foldChevron(dst *ebiten.Image, a *app, right, y float64, open bool) {
	u := a.u
	tip := 3 * u
	if open {
		tip = -tip
	}
	ui.Chevron(dst, right-10*u, y, 6*u, tip, 2.5*u, a.sc.Ink)
}
