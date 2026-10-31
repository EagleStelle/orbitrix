package client

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
)

// AI moves wait aiDelayMin..aiDelayMax ticks (300–600 ms) so people can follow them. Presentation only: the
// move itself depends on the position, strategy, seed and budget alone.
const (
	aiDelayMin  = 18
	aiDelayMax  = 36
	calloutHold = 90 // ticks a finished chain's callout stays up
	wipeTicks   = 40 // the winner's ink flooding the screen
)

// winTitles are "P1 wins!" for each seat.
var winTitles = func() (w [core.MaxPlayers]string) {
	for i := range w {
		w[i] = seatNames[i] + " wins!"
	}
	return w
}()

// The pause menu, in order.
var pauseItems = [...]string{"Resume", "Restart", "Settings", "Main menu"}

const (
	pauseResume = iota
	pauseRestart
	pauseSettings
	pauseMainMenu
)

// playScene is a local game: the mat on the left, the rail on the right (Menu, turn spinner, chain callout, seat
// table), a pause menu over it, and the results flood at the end.
type playScene struct {
	cols, rows int
	seats      []int
	game       *core.Game
	b          *board
	seed       uint64 // per game; AI seeds derive from it

	bot     ai.Bot
	scratch *core.Game // the AI thinks on this copy, off the update goroutine
	aiMove  chan int
	aiBusy  bool
	aiDelay int

	shown int // player whose turn is shown: the mover until their chain has played out
	alive [core.MaxPlayers]bool
	orbs  [core.MaxPlayers]int
	orbS  [core.MaxPlayers]string
	spin  float64 // spinner pointer angle, chasing the shown player's wedge

	chain, chainHold int // waves shown in the current or last chain, and ticks left to show it
	chainOwner       int
	chainText        string
	chainN           int
	biggest          int // longest chain this game, in waves

	paused  bool
	over    bool
	overT   int // ticks since the game ended
	results string

	// Layout in device px, recomputed when the screen size changes.
	lv                          rect
	menu                        rect
	spinX, spinY, spinR         float64
	callout                     rect
	rowRect                     [core.MaxPlayers]rect
	twoCol                      bool
	pausePanel                  rect
	pauseBtn                    [len(pauseItems)]rect
	again, toHome               rect
	exit                        rect
	winFace                     *text.GoTextFace
	winX                        float64
	winTitleY, winDotY, winDotR float64
	statsY                      float64
}

func newPlay(a *app, cols, rows int, seats []int) (*playScene, error) {
	g, err := core.NewGame(cols, rows, len(seats))
	if err != nil {
		return nil, fmt.Errorf("new game: %w", err)
	}
	p := &playScene{
		cols: cols, rows: rows, seats: append([]int(nil), seats...), game: g, b: newBoard(g),
		seed: a.rng.Uint64(), scratch: g.Clone(), aiMove: make(chan int, 1),
	}
	p.settle()
	p.spin = p.wedgeAngle(p.shown)
	return p, nil
}

// settle runs when a move has finished playing: the turn passes and the rail catches up.
func (p *playScene) settle() {
	p.shown = p.game.Turn
	for i := range p.game.Players {
		p.alive[i] = p.game.Alive(i)
	}
	if p.game.Over() && !p.over {
		p.over, p.shown, p.paused = true, int(p.game.Winner), false
		p.results = seatLabels[p.seats[p.shown]] + "  ·  " + strconv.Itoa(p.game.Moves) + " moves  ·  biggest chain ×" +
			strconv.Itoa(p.biggest)
	}
}

func (p *playScene) move(a *app, cell int) {
	mover := p.game.Turn
	waves, err := p.game.Apply(cell)
	if err != nil {
		log.Printf("move: %v", err) // callers check Legal, so this is a bug
		return
	}
	a.snd.place(mover)
	p.b.start(cell, int8(mover), waves, a.snd)
	p.biggest = max(p.biggest, len(waves))
	if len(waves) >= 2 {
		p.chainOwner, p.chainHold = mover, 0
	}
	if !p.b.playing {
		p.settle()
	}
}

func (p *playScene) restartGame(a *app) {
	if np, err := newPlay(a, p.cols, p.rows, p.seats); err == nil {
		a.next = np
	}
}

func (p *playScene) update(a *app) error {
	p.layout(a)
	if p.paused {
		return p.updatePause(a)
	}
	if a.in.esc {
		if p.over && !p.b.playing {
			a.next = newHome()
		} else {
			p.paused = true
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		p.b.fastForward()
	}

	wasPlaying := p.b.playing
	p.b.tick(a.snd)
	p.tickRail()
	if wasPlaying && !p.b.playing {
		p.settle()
	}

	if p.over && !p.b.playing {
		p.overT++
		switch {
		case p.resultsAlpha() == 0: // the buttons have not faded in yet
		case a.clicked(p.again):
			p.restartGame(a)
		case a.clicked(p.toHome):
			a.next = newHome()
		case platform.CanExit && a.clicked(p.exit):
			return ebiten.Termination
		}
		return nil
	}

	if a.clicked(compactHit(p.menu, a.u)) {
		p.paused = true
		return nil
	}
	if p.b.playing {
		if a.in.clicked && p.b.area.contains(a.in.x, a.in.y) {
			p.b.fastForward()
		}
		return nil
	}
	if p.seats[p.game.Turn] == seatHuman {
		if cell := p.b.cellAt(a.in.x, a.in.y); cell >= 0 && p.game.Legal(cell) {
			a.pointer = true
			if a.in.clicked {
				p.move(a, cell)
			}
		}
		return nil
	}
	p.thinkAI(a)
	return nil
}

// updatePause runs the pause menu. The game stands still behind it: no animation ticks, and an AI move that is
// ready waits.
func (p *playScene) updatePause(a *app) error {
	if a.in.esc {
		p.paused = false
		return nil
	}
	for i, r := range p.pauseBtn {
		if !a.clicked(r) {
			continue
		}
		switch i {
		case pauseResume:
			p.paused = false
		case pauseRestart:
			p.restartGame(a)
		case pauseSettings:
			a.next = newSettingsMenu(p)
		case pauseMainMenu:
			a.next = newHome()
		}
		return nil
	}
	if a.in.clicked && !p.pausePanel.contains(a.in.x, a.in.y) {
		p.paused = false
	}
	return nil
}

// tickRail advances the spinner, the chain callout and the orb counts.
func (p *playScene) tickRail() {
	// The pointer always turns forward to the next player, easing in.
	d := math.Mod(p.wedgeAngle(p.shown)-p.spin, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	if d > 2*math.Pi-1e-3 || d < 1e-3 {
		p.spin = p.wedgeAngle(p.shown)
	} else {
		p.spin += d * 0.16
	}

	if p.b.playing && len(p.b.waves) >= 2 {
		p.chain, p.chainHold = p.b.wave+1, calloutHold
	} else if p.chainHold > 0 {
		p.chainHold--
	}
	if p.chain != p.chainN {
		p.chainN, p.chainText = p.chain, "Chain ×"+strconv.Itoa(p.chain)
	}

	var orbs [core.MaxPlayers]int
	for i, o := range p.b.v.owner {
		if o != core.NoOwner {
			orbs[o] += int(p.b.v.count[i])
		}
	}
	for i := range p.seats {
		if orbs[i] != p.orbs[i] || p.orbS[i] == "" {
			p.orbs[i], p.orbS[i] = orbs[i], strconv.Itoa(orbs[i])
		}
	}
}

// wedgeAngle is the middle of seat i's wedge on the spinner, clockwise from the top.
func (p *playScene) wedgeAngle(i int) float64 {
	return -math.Pi/2 + (float64(i)+0.5)*2*math.Pi/float64(len(p.seats))
}

// thinkAI starts the AI on a copy of the game, then plays its move once both the move and the delay are ready.
func (p *playScene) thinkAI(a *app) {
	if !p.aiBusy {
		p.scratch.CopyFrom(p.game)
		s := ai.Strategy(p.seats[p.game.Turn] - 1)
		rng := core.NewRNG(p.seed + uint64(p.game.Moves))
		seed := rng.Uint64()
		p.aiBusy, p.aiDelay = true, aiDelayMin+a.rng.Intn(aiDelayMax-aiDelayMin+1)
		g, bot, out := p.scratch, &p.bot, p.aiMove
		go func() { out <- bot.Choose(g, s, seed, ai.DefaultBudget) }()
		return
	}
	if p.aiDelay > 0 {
		p.aiDelay--
		return
	}
	select {
	case cell := <-p.aiMove:
		p.aiBusy = false
		p.move(a, cell)
	default:
	}
}

func (p *playScene) layout(a *app) {
	v := a.safe()
	if v == p.lv {
		return
	}
	p.lv = v
	if a.portrait() {
		p.layoutBars(a, v)
	} else {
		p.layoutRail(a, v)
	}
	p.layoutPause(a, v)
	p.layoutResults(a, v)
}

// layoutRail puts the board on the left and the rail on the right: Menu across the rail's top, then the spinner, the
// chain callout and the seat table. A short screen (a phone on its side) gets smaller margins, gaps and spinner.
func (p *playScene) layoutRail(a *app, v rect) {
	u := a.u
	pad, menuGap, calloutGap, listGap, minSpin := 32*u, 24*u, 20*u, 24*u, 96*u
	if v.h < 640*u {
		pad, menuGap, calloutGap, listGap, minSpin = 16*u, 12*u, 8*u, 12*u, 72*u
	}
	rw := max(280*u, min(340*u, v.w*0.26))
	r := rect{v.right() - pad - rw, v.y + pad, rw, v.h - 2*pad}
	// The board is centred in the whole column left of the rail, margin to margin. Menu spans the rail and the rest of
	// the rail starts one gap under it.
	p.b.layout(rect{v.x + pad, v.y + pad, r.x - v.x - 2*pad, v.h - 2*pad}, u)
	p.menu = rect{r.x, r.y, rw, compactButtonHeight * u}
	top := p.menu.bottom() + menuGap

	// Top down under Menu: the spinner, the chain callout slot, the seat table. The spinner shrinks first when space
	// runs out; if one column of seats still won't fit, they go into two compact columns without the type names.
	avail := r.bottom() - top
	above := calloutGap + 40*u + listGap // spinner to list, excluding the spinner itself
	rows, rowH := len(p.seats), 40*u
	spinner := func() float64 { return max(minSpin, min(240*u, rw*0.8, avail-above-float64(rows)*rowH)) }
	d := spinner()
	p.twoCol = rows > 3 && avail-above-d < float64(rows)*30*u
	if p.twoCol {
		rows = (rows + 1) / 2
		d = spinner()
	}
	if avail-above-d < float64(rows)*rowH {
		rowH = max(24*u, (avail-above-d)/float64(rows))
	}
	p.spinR = d / 2
	p.spinX, p.spinY = r.cx(), top+p.spinR
	p.callout = rect{r.cx() - 90*u, p.spinY + p.spinR + calloutGap, 180 * u, 40 * u}
	p.layoutSeats(a, rect{r.x, top + d + above, rw, float64(rows) * rowH}, rows)
}

// layoutBars is the upright layout, built from the rail's parts: a top bar with Menu on the left and the chain
// callout on the right, a bottom bar with the spinner on the left and the seat table beside it, and the board between
// them.
func (p *playScene) layoutBars(a *app, v rect) {
	u := a.u
	pad, gap := 16*u, 16*u
	x, w := v.x+pad, v.w-2*pad
	p.menu = rect{x, v.y + pad, 112 * u, compactButtonHeight * u}
	cw := min(180*u, w-p.menu.w-12*u)
	p.callout = rect{x + w - cw, p.menu.y, cw, 40 * u}

	// More than six seats go into two compact columns without the type names, as on a crowded rail.
	n := len(p.seats)
	p.twoCol = n > 6
	rows, rowH := n, 36*u
	if p.twoCol {
		rows, rowH = (n+1)/2, 32*u
	}
	tableH := float64(rows) * rowH
	d := max(88*u, min(140*u, tableH, w*0.32))
	barH := max(d, tableH)
	barY := v.bottom() - pad - barH
	p.spinR = d / 2
	p.spinX, p.spinY = x+p.spinR, barY+barH/2
	tx := x + d + gap
	p.layoutSeats(a, rect{tx, barY + (barH-tableH)/2, x + w - tx, tableH}, rows)

	top := p.menu.bottom() + gap
	p.b.layout(rect{x, top, w, barY - gap - top}, u)
}

// layoutSeats fills table with the seat rows, rows to a column.
func (p *playScene) layoutSeats(a *app, table rect, rows int) {
	u := a.u
	rowH := table.h / float64(rows)
	w := table.w
	if p.twoCol {
		w = (table.w - 8*u) / 2
	}
	for i := range p.seats {
		p.rowRect[i] = rect{table.x + float64(i/rows)*(w+8*u), table.y + float64(i%rows)*rowH, w, rowH - 4*u}
	}
}

// layoutPause centres the pause menu: a panel of full-width buttons, two by two when the screen is too short for one
// column.
func (p *playScene) layoutPause(a *app, v rect) {
	u := a.u
	bw, bh, gap := min(300*u, v.w-32*u-56*u), quietButtonHeight*u, 12*u
	cols := 1
	rows := func() float64 { return float64((len(pauseItems) + cols - 1) / cols) }
	ph := func() float64 { return 28*u + 40*u + 24*u + rows()*bh + (rows()-1)*gap + 28*u }
	if ph() > v.h-32*u {
		cols = 2
	}
	pw := float64(cols)*bw + float64(cols-1)*gap + 56*u
	p.pausePanel = rect{v.x + (v.w-pw)/2, v.y + (v.h-ph())/2, pw, ph()}
	for i := range p.pauseBtn {
		p.pauseBtn[i] = rect{p.pausePanel.x + 28*u + float64(i%cols)*(bw+gap),
			p.pausePanel.y + 92*u + float64(i/cols)*(bh+gap), bw, bh}
	}
}

// layoutResults places the result: one centred column on the winner's ink, the title as large as the screen allows.
// On a screen too short for the column the dot shrinks and the buttons stand side by side.
func (p *playScene) layoutResults(a *app, v rect) {
	u := a.u
	bw, bh, gap := 220*u, quietButtonHeight*u, 12*u // the pause menu's button height and gap
	size := max(56*u, min(120*u, v.w*0.075, v.h*0.13))
	// "P12 wins!" is the widest title; it still fits across the screen.
	if tw := textWidth(winTitles[len(p.seats)-1], newFace(srcDisplay, size, 800)); tw > v.w-32*u {
		size *= (v.w - 32*u) / tw
	}
	p.winFace = newFace(srcDisplay, size, 800)
	p.winX = v.cx()
	n := 2
	if platform.CanExit {
		n = 3
	}
	p.winDotR = max(44*u, min(80*u, v.h*0.08))
	above := func() float64 { return 2*p.winDotR + 28*u + size*0.6 } // dot top to title centre
	below := size*0.5 + 34*u + 44*u + float64(n)*bh + float64(n-1)*gap
	p.winTitleY = v.y + v.h/2 - 90*u
	row := false
	if p.winTitleY-above() < v.y+16*u || p.winTitleY+below > v.bottom()-16*u {
		row = float64(n)*bw+float64(n-1)*gap <= v.w-32*u
		if row {
			below = size*0.5 + 34*u + 32*u + bh
		}
		p.winDotR = max(24*u, min(p.winDotR, (v.h-32*u-below-28*u-size*0.6)/2))
		p.winTitleY = v.y + (v.h-above()-below)/2 + above()
	}
	p.statsY = p.winTitleY + size*0.5 + 34*u
	p.winDotY = p.winTitleY - size*0.6 - 28*u - p.winDotR
	if row {
		x := p.winX - (float64(n)*bw+float64(n-1)*gap)/2
		p.again = rect{x, p.statsY + 32*u, bw, bh}
		p.toHome = rect{p.again.right() + gap, p.again.y, bw, bh}
		p.exit = rect{p.toHome.right() + gap, p.again.y, bw, bh}
		return
	}
	p.again = rect{p.winX - bw/2, p.statsY + 44*u, bw, bh}
	p.toHome = rect{p.winX - bw/2, p.again.bottom() + gap, bw, bh}
	p.exit = rect{p.winX - bw/2, p.toHome.bottom() + gap, bw, bh}
}

func (p *playScene) draw(dst *ebiten.Image, a *app) {
	p.layout(a)
	sc, u := a.sc, a.u
	turn := p.shown
	if p.over && !p.b.playing {
		turn = -1
	}
	p.b.draw(dst, sc, a.tms(), turn, a.set.Marks)
	if !p.b.playing && !p.over && !p.paused && p.seats[p.game.Turn] == seatHuman {
		if cell := p.b.cellAt(a.in.x, a.in.y); cell >= 0 && p.game.Legal(cell) {
			p.b.drawHover(dst, cell, sc.players[p.game.Turn], u)
		}
	}
	p.drawRail(dst, a)
	switch {
	case p.over && !p.b.playing:
		p.drawResults(dst, a)
	case p.paused:
		p.drawPause(dst, a)
	}
}

// drawMenuButton draws the rail's Menu pill with a three-line glyph, laid out like Back.
func (p *playScene) drawMenuButton(dst *ebiten.Image, a *app) {
	u, r := a.u, p.menu
	ink := drawButtonSurface(dst, a, r, btnTile, true, a.over(compactHit(r, u)) && !p.paused && !p.over)
	hamburger(dst, r.x+22*u, r.cy(), 16*u, 2.5*u, ink)
	drawText(dst, "Menu", a.f.labelBold, r.x+42*u, r.cy(), text.AlignStart, ink)
}

func (p *playScene) drawRail(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	p.drawMenuButton(dst, a)

	// Spinner: a wedge per seat in its ink (out seats printed blank), the pointer on the player to move.
	n := len(p.seats)
	for i := range n {
		a0 := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		c := sc.players[i]
		if !p.alive[i] {
			c = sc.tile
		}
		scratchPath.Reset()
		scratchPath.MoveTo(float32(p.spinX), float32(p.spinY))
		scratchPath.Arc(float32(p.spinX), float32(p.spinY), float32(p.spinR), float32(a0),
			float32(a0+2*math.Pi/float64(n)), 0)
		scratchPath.Close()
		fillPath(dst, &scratchPath, c)
	}
	for i := range n {
		an := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		line(dst, p.spinX, p.spinY, p.spinX+p.spinR*math.Cos(an), p.spinY+p.spinR*math.Sin(an), 3*u, sc.ground)
	}
	if a.set.Marks {
		rm := p.spinR * 0.66
		mr := min(p.spinR*0.17, math.Pi*rm/float64(n)*0.62)
		for i := range n {
			if p.alive[i] {
				an := p.wedgeAngle(i)
				drawMark(dst, i, p.spinX+rm*math.Cos(an), p.spinY+rm*math.Sin(an), mr, sc.players[i])
			}
		}
	}
	fillCircle(dst, p.spinX, p.spinY, p.spinR*0.2, sc.ground)
	tip := p.spinR + 12*u
	sx, sy := math.Cos(p.spin), math.Sin(p.spin)
	w := p.spinR * 0.09
	scratchPath.Reset()
	scratchPath.MoveTo(float32(p.spinX+tip*sx), float32(p.spinY+tip*sy))
	scratchPath.LineTo(float32(p.spinX-w*sy), float32(p.spinY+w*sx))
	scratchPath.LineTo(float32(p.spinX+w*sy), float32(p.spinY-w*sx))
	scratchPath.Close()
	fillPath(dst, &scratchPath, sc.ink)
	fillCircle(dst, p.spinX, p.spinY, p.spinR*0.1, sc.ink)

	// The chain callout, in the mover's ink.
	if p.chainHold > 0 && p.chain >= 2 {
		c := sc.players[p.chainOwner]
		alpha := min(1, float64(p.chainHold)/20)
		fillPill(dst, p.callout, withAlpha(c, alpha))
		drawText(dst, p.chainText, f.callout, p.callout.cx(), p.callout.cy(), text.AlignCenter, withAlpha(inkOn(c), alpha))
	}

	for i, seat := range p.seats {
		r := p.rowRect[i]
		alive, current := p.alive[i], i == p.shown && !p.over
		if current {
			fillPill(dst, r, sc.tile)
		}
		dr := min(12*u, r.h*0.34)
		in, sp := 14*u, 12*u // row inset, and the space after the dot
		if r.w < 130*u {     // two columns beside the spinner on an upright phone
			in, sp = 8*u, 6*u
		}
		seatDot(dst, sc, i, r.x+in+dr, r.cy(), dr, !alive, a.set.Marks)
		ink := sc.ink
		if !alive {
			ink = sc.dim
		}
		// Couch distance: full-size text unless the table is squeezed into two columns.
		name, bold := f.body, f.bodyBold
		if p.twoCol {
			name, bold = f.label, f.labelBold
		}
		x := r.x + in + 2*dr + sp
		drawText(dst, seatNames[i], bold, x, r.cy(), text.AlignStart, ink)
		if !p.twoCol {
			drawText(dst, seatLabels[seat], name, x+44*u, r.cy(), text.AlignStart, ink)
		}
		if alive {
			drawText(dst, p.orbS[i], bold, r.right()-in, r.cy(), text.AlignEnd, ink)
		} else {
			drawText(dst, "out", name, r.right()-in, r.cy(), text.AlignEnd, sc.dim)
		}
	}
}

// drawPause dims the game and shows the pause menu.
func (p *playScene) drawPause(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	fillRound(dst, rect{0, 0, a.w, a.h}, 0, withAlpha(sc.ground, 0.8))
	fillRound(dst, p.pausePanel, 28*u, sc.tile)
	drawText(dst, "Paused", a.f.title, p.pausePanel.cx(), p.pausePanel.y+28*u+20*u, text.AlignCenter, sc.ink)
	for i, r := range p.pauseBtn {
		style := btnGround
		if i == pauseResume {
			style = btnInk
		}
		drawButton(dst, a, r, pauseItems[i], "", style, true)
	}
}

// drawResults floods the screen with the winner's ink from the board's centre, then shows the result.
func (p *playScene) drawResults(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	w := sc.players[p.shown]
	ink := inkOn(w)
	t := min(1, float64(p.overT)/wipeTicks)
	e := 1 - math.Pow(1-t, 3)
	bb := p.b.bounds()
	cx, cy := bb.cx(), bb.cy()
	reach := math.Hypot(max(cx, a.w-cx), max(cy, a.h-cy))
	fillCircle(dst, cx, cy, e*reach, w)
	alpha := p.resultsAlpha()
	if alpha == 0 {
		return
	}
	// The winner's dot at mat scale, then the title at poster size.
	x := p.winX
	fillCircle(dst, x, p.winDotY, p.winDotR, withAlpha(ink, alpha))
	if alpha == 1 {
		fillCircle(dst, x, p.winDotY, p.winDotR-8*u, w)
		if a.set.Marks {
			drawMark(dst, p.shown, x, p.winDotY, p.winDotR-8*u, w)
		}
	}
	drawText(dst, winTitles[p.shown], p.winFace, x, p.winTitleY, text.AlignCenter, withAlpha(ink, alpha))
	face := f.body
	if textWidth(p.results, face) > p.lv.w-32*u { // a phone held upright
		face = f.label
	}
	drawText(dst, p.results, face, x, p.statsY, text.AlignCenter, withAlpha(ink, alpha))

	resultButton(dst, a, p.again, "Play again", w, ink, true, alpha)
	resultButton(dst, a, p.toHome, "Main menu", w, ink, false, alpha)
	if platform.CanExit {
		resultButton(dst, a, p.exit, "Exit", w, ink, false, alpha)
	}
}

// resultsAlpha fades the result in once the flood is halfway out.
func (p *playScene) resultsAlpha() float64 { return min(1, max(0, float64(p.overT-wipeTicks/2)/15)) }

// resultButton draws a button on the winner's ink: filled with the contrasting ink when primary, outlined
// otherwise.
func resultButton(dst *ebiten.Image, a *app, r rect, label string, bg, ink color.RGBA, primary bool, alpha float64) {
	hot := a.over(r)
	if primary {
		fill := ink
		if hot {
			fill = mix(ink, bg, 0.2)
		}
		fillPill(dst, r, withAlpha(fill, alpha))
		drawText(dst, label, a.f.heading, r.cx(), r.cy(), text.AlignCenter, withAlpha(bg, alpha))
		return
	}
	if hot {
		fillPill(dst, r, withAlpha(mix(bg, ink, 0.15), alpha))
	}
	strokeRound(dst, r, r.h/2, 2.5*a.u, withAlpha(ink, alpha))
	pillLabel(dst, a, r, label, "", withAlpha(ink, alpha))
}
