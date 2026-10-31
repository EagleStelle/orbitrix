package client

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"
	"strconv"
	"strings"

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
	table                       seatStyle
	orbsMax                     string        // as many 8s as the most orbs this board holds at rest has digits
	compact                     bool          // a phone: label-size seat rows and the small callout
	typeDX                      float64       // seat type name's offset from the player tag
	wheel, pointer              *ebiten.Image // the spinner, drawn by renderSpinner
	wheelX, wheelY              float64
	spinKey                     spinnerKey
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
	// Every orb of a settled board sits in a cell below its capacity, and a player can't own more than all of them.
	most := 0
	for _, c := range g.Geo.Cap {
		most += int(c) - 1
	}
	p.orbsMax = strings.Repeat("8", len(strconv.Itoa(most)))
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
		budget := aiBudget(g.Geo.Cells())
		go func() { out <- bot.Choose(g, s, seed, budget) }()
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

// aiBudget is the AI's work per move on a board of cells. Go in a browser runs on one thread, so a thinking AI holds
// up drawing there; boards bigger than the largest preset get proportionally less work, keeping the pause about as
// short as on that preset.
func aiBudget(cells int) int {
	if runtime.GOOS != "js" {
		return ai.DefaultBudget
	}
	z := boardSizes[len(boardSizes)-1]
	return min(ai.DefaultBudget, ai.DefaultBudget*z.short*z.long/cells)
}

func (p *playScene) layout(a *app) {
	v := a.safe()
	if v == p.lv {
		return
	}
	p.lv = v
	// Phones (upright, or short on their side) set the seat rows at the button label size; the desktop rail keeps body
	// size for couch distance.
	p.compact = a.portrait() || v.h < 640*a.u
	p.typeDX = 44 * a.u
	if p.compact {
		p.typeDX = max(30*a.u, textWidth("P12", a.f.labelBold)) + 8*a.u // as on the Setup seat pill
	}
	if a.portrait() {
		p.layoutBars(a, v)
	} else {
		p.layoutRail(a, v)
	}
	p.layoutPause(a, v)
	p.layoutResults(a, v)
}

// seatStyle is how the seat table sets out its rows: which parts each row shows, and in how many columns.
type seatStyle struct {
	names bool // the seat's type (Human, Gambler, ...)
	tags  bool // the seat's number (P1, P2, ...)
	cols  int
}

// railStyles prefer type names, then fewer columns when the wheel fits equally well. Every style shows seat numbers.
var railStyles = [...]seatStyle{
	{names: true, tags: true, cols: 1}, {names: true, tags: true, cols: 2},
	{tags: true, cols: 1}, {tags: true, cols: 2},
}

// layoutRail puts the board on the left and a rail on the right: Menu, the spinner, the chain callout and the seat
// table. The rail starts at the minimum width its controls need, then takes any width the board cannot use, keeping
// the board at the left margin. Menu and the table fill the rail; the callout matches the wheel. Phones skip type
// names and float the callout over the spinner instead of giving it a slot.
//
// After the board is fitted, the table chooses one or two columns to maximise the wheel in the actual rail width.
// Row sizes stay fixed during this choice; the wheel takes the space they leave, centred in the rail.
func (p *playScene) layoutRail(a *app, v rect) {
	u := a.u
	pad, menuGap, calloutGap, listGap, rowMin, rowMax := 32*u, 24*u, 20*u, 24*u, 40*u, 40*u
	styles := railStyles[:]
	if p.compact {
		// 12 px gaps keep the pointer's tip, which reaches 12 px past the spinner, clear of Menu and the seats.
		pad, menuGap, listGap, rowMin, rowMax = 16*u, 12*u, 12*u, 20*u, 32*u
		styles = styles[2:]
	}
	h := v.h - 2*pad
	cw, ch := p.calloutSize(a)
	fixed := compactButtonHeight*u + menuGap + listGap // the height besides the spinner and the rows
	if !p.compact {
		fixed += calloutGap + ch
	}
	menuW := 42*u + textWidth("Menu", a.f.labelBold) + 18*u // as drawMenuButton sets it out
	n := len(p.seats)
	var w float64
	var rows int
	for _, st := range styles {
		// Two columns of names use spare rail width rather than taking room from the board.
		if st.names && st.cols == 2 {
			continue
		}
		p.table = st
		rows = (n + st.cols - 1) / st.cols
		w = max(menuW, cw, p.seatTableWidth(a))
		if fixed+w+float64(rows)*rowMin <= h {
			break
		}
	}
	rowH := max(rowMin, min(rowMax, (h-fixed-w)/float64(rows)))
	// The board keeps the left margin and the rail takes every px of width the board can't use.
	x, y := v.right()-pad-w, v.y+pad
	area := rect{v.x + pad, y, x - v.x - 2*pad, h}
	if bw, _ := p.b.size(area); bw < area.w {
		w += area.w - bw
		x -= area.w - bw
		area.w = bw
	}
	p.b.layout(area, u)
	chosen, d := p.table, min(w, h-fixed-float64(rows)*rowH)
	if !p.compact {
		d = 0
		for _, st := range styles {
			p.table = st
			if p.seatTableWidth(a) > w {
				continue
			}
			count := (n + st.cols - 1) / st.cols
			if diameter := min(w, h-fixed-float64(count)*rowH); diameter > d {
				chosen, d = st, diameter
			}
		}
	}
	p.table = chosen
	rows = (n + chosen.cols - 1) / chosen.cols
	p.menu = rect{x, y, w, compactButtonHeight * u}
	p.spinR = d / 2
	p.spinX, p.spinY = x+w/2, p.menu.bottom()+menuGap+p.spinR
	below := p.spinY + p.spinR
	if p.compact {
		p.callout = rect{p.spinX - p.spinR, p.spinY - ch/2, d, ch}
	} else {
		p.callout = rect{p.spinX - p.spinR, below + calloutGap, d, ch}
		below = p.callout.bottom()
	}
	p.layoutSeats(a, rect{x, below + listGap, w, float64(rows) * rowH}, rows)
}

// seatFaces keep desktop text at body size in either column layout; phones retain their compact sizes.
func (p *playScene) seatFaces(a *app) (name, bold *text.GoTextFace) {
	switch {
	case p.compact && p.table.cols == 2:
		return a.f.small, a.f.smallBold
	case p.compact:
		return a.f.label, a.f.labelBold
	}
	return a.f.body, a.f.bodyBold
}

// seatInsets are a seat row's inset at each end, the space after its dot, the least space before its orbs, and the
// dot's largest radius. Desktop rows keep the same spacing in either column layout.
func (p *playScene) seatInsets(a *app) (in, sp, gap, dot float64) {
	u := a.u
	dot = 12 * u
	if p.compact {
		dot = 10 * u
	}
	switch {
	case p.compact && p.table.cols == 2:
		return 6 * u, 4 * u, 6 * u, dot - 2*u
	case p.compact:
		return 10 * u, 8 * u, 12 * u, dot
	}
	return 14 * u, 12 * u, 12 * u, dot
}

// seatTableWidth is the narrowest table whose rows fit the parts p.table shows, set out as drawRail draws them:
// inset, dot, space, number, type, gap, orbs (or "out"), inset, and 8 px between columns.
func (p *playScene) seatTableWidth(a *app) float64 {
	name, bold := p.seatFaces(a)
	in, sp, gap, dot := p.seatInsets(a)
	row := 2*in + 2*dot + sp + gap + max(textWidth(p.orbsMax, bold), textWidth("out", name))
	switch {
	case p.table.names:
		label := 0.0
		for _, s := range p.seats {
			label = max(label, textWidth(seatLabels[s], name))
		}
		row += p.typeDX + label
	case p.table.tags:
		tag := 0.0
		for i := range p.seats {
			tag = max(tag, textWidth(seatNames[i], bold))
		}
		row += tag
	}
	return float64(p.table.cols)*row + float64(p.table.cols-1)*8*a.u
}

// calloutSize is the chain callout pill: 180×40 on the desktop rail; on a phone a 32 px pill sized for "Chain ×88",
// which a three-digit chain still fits inside.
func (p *playScene) calloutSize(a *app) (w, h float64) {
	if p.compact {
		return textWidth("Chain ×88", a.f.calloutCompact) + 20*a.u, 32 * a.u
	}
	return 180 * a.u, 40 * a.u
}

// layoutBars is the upright layout, built from the rail's parts: the board at the top, then a bar with the spinner
// and the chain callout under it on the left and the seat table beside them, and Menu across the bottom in thumb
// reach.
func (p *playScene) layoutBars(a *app, v rect) {
	u := a.u
	pad, gap := 16*u, 16*u
	x, w := v.x+pad, v.w-2*pad
	p.menu = rect{x, v.bottom() - pad - compactButtonHeight*u, w, compactButtonHeight * u}

	// More than six seats go into two columns. Each row shows as much as the table's width allows.
	n := len(p.seats)
	styles := []seatStyle{{names: true, tags: true, cols: 1}, {tags: true, cols: 1}}
	rows, rowH := n, 36*u
	if n > 6 {
		styles = []seatStyle{{tags: true, cols: 2}}
		rows, rowH = (n+1)/2, 32*u
	}
	tableH := float64(rows) * rowH
	// The spinner over the callout, as wide as the spinner and 12 px under it so the pointer's tip clears it,
	// centred against the table.
	cw, ch := p.calloutSize(a)
	d := max(cw, 88*u, min(140*u, tableH, w*0.32))
	colH := d + 12*u + ch
	barH := max(colH, tableH)
	barY := p.menu.y - gap - barH
	p.spinR = d / 2
	p.spinX, p.spinY = x+p.spinR, barY+(barH-colH)/2+p.spinR
	p.callout = rect{x, p.spinY + p.spinR + 12*u, d, ch}
	tx := x + d + gap
	for _, p.table = range styles {
		if p.seatTableWidth(a) <= x+w-tx {
			break
		}
	}
	// Twelve seats on a narrow phone take the few px they lack from the gap, down to 8 px.
	tx -= min(gap-8*u, max(0, p.seatTableWidth(a)-(x+w-tx)))
	p.layoutSeats(a, rect{tx, barY + (barH-tableH)/2, x + w - tx, tableH}, rows)

	top := v.y + pad
	p.b.layout(rect{x, top, w, barY - gap - top}, u)
}

// layoutSeats fills table with the seat rows in p.table's columns, rows to a column, 8 px apart.
func (p *playScene) layoutSeats(a *app, table rect, rows int) {
	u := a.u
	rowH := table.h / float64(rows)
	cols := float64(p.table.cols)
	w := (table.w - (cols-1)*8*u) / cols
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
	if p.over && !p.b.playing && p.overT >= wipeTicks {
		p.drawResults(dst, a)
		return
	}
	sc, u := a.sc, a.u
	turn := p.shown
	if p.over && !p.b.playing {
		turn = -1
	}
	// The board's tick clock stops during pause and resumes without jumping ahead.
	p.b.draw(dst, sc, p.b.clock*(1000.0/60), turn, a.set.Marks)
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
	sc, f := a.sc, &a.f
	p.drawMenuButton(dst, a)

	p.drawSpinner(dst, a)

	// The chain callout, in the mover's ink.
	if p.chainHold > 0 && p.chain >= 2 {
		c := sc.players[p.chainOwner]
		alpha := min(1, float64(p.chainHold)/20)
		fillPill(dst, p.callout, withAlpha(c, alpha))
		face := f.callout
		if p.compact {
			face = f.calloutCompact
		}
		drawText(dst, p.chainText, face, p.callout.cx(), p.callout.cy(), text.AlignCenter, withAlpha(inkOn(c), alpha))
	}

	// The seat rows, set out as seatTableWidth measures them.
	name, bold := p.seatFaces(a)
	in, sp, _, dot := p.seatInsets(a)
	for i, seat := range p.seats {
		r := p.rowRect[i]
		alive, current := p.alive[i], i == p.shown && !p.over
		if current {
			fillPill(dst, r, sc.tile)
		}
		dr := min(dot, r.h*0.34)
		seatDot(dst, sc, i, r.x+in+dr, r.cy(), dr, !alive, a.set.Marks)
		ink := sc.ink
		if !alive {
			ink = sc.dim
		}
		x := r.x + in + 2*dr + sp
		if p.table.tags {
			drawText(dst, seatNames[i], bold, x, r.cy(), text.AlignStart, ink)
		}
		if p.table.names {
			drawText(dst, seatLabels[seat], name, x+p.typeDX, r.cy(), text.AlignStart, ink)
		}
		if alive {
			drawText(dst, p.orbS[i], bold, r.right()-in, r.cy(), text.AlignEnd, ink)
		} else {
			drawText(dst, "out", name, r.right()-in, r.cy(), text.AlignEnd, sc.dim)
		}
	}
}

// spinnerKey is what the cached spinner images show.
type spinnerKey struct {
	x, y, r, u float64
	alive      [core.MaxPlayers]bool
	sc         *scheme
	marks      bool
}

// drawSpinner draws the turn spinner: a wedge per seat in its ink (out seats printed blank) and the pointer on the
// player to move. Both are images redrawn only when what they show changes; the pointer turns as it is drawn.
func (p *playScene) drawSpinner(dst *ebiten.Image, a *app) {
	if k := (spinnerKey{p.spinX, p.spinY, p.spinR, a.u, p.alive, a.sc, a.set.Marks}); p.wheel == nil || k != p.spinKey {
		p.spinKey = k
		p.renderSpinner(a)
	}
	imageOp = ebiten.DrawImageOptions{}
	imageOp.GeoM.Translate(p.wheelX, p.wheelY)
	dst.DrawImage(p.wheel, &imageOp)
	h := float64(p.pointer.Bounds().Dy()) / 2
	imageOp = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	imageOp.GeoM.Translate(-h, -h)
	imageOp.GeoM.Rotate(p.spin)
	imageOp.GeoM.Translate(p.spinX, p.spinY)
	dst.DrawImage(p.pointer, &imageOp)
}

// renderSpinner caches the wheel at whole-pixel alignment and the rim pointer pointing right around its hub.
func (p *playScene) renderSpinner(a *app) {
	if p.wheel != nil {
		p.wheel.Deallocate()
		p.pointer.Deallocate()
	}
	sc, u, n, r := a.sc, a.u, len(p.seats), p.spinR
	p.wheelX, p.wheelY = math.Floor(p.spinX-r)-spritePad, math.Floor(p.spinY-r)-spritePad
	x, y := p.spinX-p.wheelX, p.spinY-p.wheelY
	p.wheel = ebiten.NewImage(int(math.Ceil(x+r))+spritePad, int(math.Ceil(y+r))+spritePad)
	for i := range n {
		a0 := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		c := sc.players[i]
		if !p.alive[i] {
			c = sc.tile
		}
		scratchPath.Reset()
		scratchPath.MoveTo(float32(x), float32(y))
		scratchPath.Arc(float32(x), float32(y), float32(r), float32(a0), float32(a0+2*math.Pi/float64(n)), 0)
		scratchPath.Close()
		fillPath(p.wheel, &scratchPath, c)
	}
	for i := range n {
		an := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		line(p.wheel, x, y, x+r*math.Cos(an), y+r*math.Sin(an), 3*u, sc.ground)
	}
	if a.set.Marks {
		rm := r * 0.66
		mr := min(r*0.17, math.Pi*rm/float64(n)*0.62)
		for i := range n {
			if p.alive[i] {
				an := p.wedgeAngle(i)
				drawMark(p.wheel, i, x+rm*math.Cos(an), y+rm*math.Sin(an), mr, sc.players[i])
			}
		}
	}
	fillCircle(p.wheel, x, y, r*0.2, sc.ground)

	tip, w, h := r+12*u, r*0.09, math.Ceil(r*0.1)+spritePad
	p.pointer = ebiten.NewImage(int(math.Ceil(h+tip))+spritePad, int(2*h))
	scratchPath.Reset()
	scratchPath.MoveTo(float32(h+tip), float32(h))
	scratchPath.LineTo(float32(h), float32(h+w))
	scratchPath.LineTo(float32(h), float32(h-w))
	scratchPath.Close()
	fillPath(p.pointer, &scratchPath, sc.ink)
	fillCircle(p.pointer, h, h, r*0.1, sc.ink)
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
	if t < 1 {
		fillCircle(dst, cx, cy, e*reach, w)
	} else {
		dst.Fill(&sc.players[p.shown]) // avoid boxing the colour each frame
	}
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
