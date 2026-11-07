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
	"github.com/hajimehoshi/ebiten/v2/vector"

	"orbitrix/client/internal/boardview"
	"orbitrix/client/internal/ui"
	"orbitrix/client/platform"
	"orbitrix/core"
	"orbitrix/core/ai"
	"orbitrix/proto"
)

// AI moves wait aiDelayMin..aiDelayMax ticks (300–600 ms) so people can follow them. Presentation only: the
// move itself depends on the position, strategy, seed and budget alone.
const (
	aiDelayMin   = 18
	aiDelayMax   = 36
	calloutHold  = 90  // ticks a finished chain's callout stays up
	wipeTicks    = 40  // the winner's ink flooding the screen
	confirmTicks = 180 // a pause button that throws the game away waits this long for its second tap
	flashTicks   = 30  // a tapped cell the player can't use flashes its outline
)

// The pause menu, in order.
var pauseItems = [...]string{"Resume", "Restart", "Settings", "Main menu", "Save map"}

// pauseConfirm is what a button that waits for its second tap reads.
var pauseConfirm = [len(pauseItems)]string{pauseRestart: "Restart?"}

const (
	pauseResume = iota
	pauseRestart
	pauseSettings
	pauseMainMenu
	pauseSaveMap
)

// playScene is a local game with a landscape turn wheel or a portrait turn dot, a scrolling seat table,
// a pause menu and the results flood.
type playScene struct {
	cols, rows int
	seats      []int
	labels     []string // what each seat is called: its type, or an online player's name
	items      []int    // the pause menu's buttons, from pauseItems
	game       *core.Game
	returnTo   scene // editor test play returns to its unchanged draft
	b          *boardview.Board
	seed       uint64 // per game; AI seeds derive from it

	bot     ai.Bot
	scratch *core.Game // the AI thinks on this copy, off the update goroutine
	aiMove  chan int
	aiBusy  bool
	aiDelay int

	shown     int // player whose turn is shown: the mover until their chain has played out
	alive     [core.MaxPlayers]bool
	orbs      [core.MaxPlayers]int
	orbS      [core.MaxPlayers]string
	orbLabels [core.MaxPlayers]boardview.Counter
	spin      float64 // spinner pointer angle, chasing the shown player's wedge

	chain, chainHold int // waves shown in the current or last chain, and ticks left to show it
	chainOwner       int
	chainText        string
	chainN           int
	biggest          int // longest chain this game, in waves

	paused   bool
	confirm  int // pause button waiting for its second tap, or -1
	confirmT int
	flash    int // cell flashing after a tap the mover can't play there
	flashT   int
	over     bool
	overT    int // ticks since the game ended
	results  string
	winTitle string

	// Online: the room this game is played in. Moves arrive from the host instead of being made here.
	net      *session
	seq      uint32 // moves played on this board
	sent     bool   // this player's move is on its way to the host
	resync   bool   // the board went wrong; waiting for the host's snapshot
	pace     int    // ticks before an AI's move plays, or -1 before the wait starts
	lobbyN   int    // the room update the seats were last read from
	goneP    int    // host: the seat whose player left, asking Bot or Skip; -1 when none
	goneB    [2]ui.Rect
	goneD    ui.Rect
	goneN    int // the seat goneText names
	goneText string

	// Layout in device px, recomputed when the screen size changes.
	lv                          ui.Rect
	lu                          float64
	menu                        ui.Rect
	spinX, spinY, spinR         float64
	callout                     ui.Rect
	rowRect                     [core.MaxPlayers]ui.Rect
	tableCols                   int // the seat table's columns
	boardView, seatView         ui.Scroll
	listShown                   int
	tokX, tokY, tokR            float64       // upright: the mover's dot beside Menu; tokR 0 elsewhere
	orbsMax                     string        // as many 8s as the most orbs this board holds at rest has digits
	compact                     bool          // a phone: label-size seat rows and the small callout
	wheel, pointer              *ebiten.Image // the spinner, drawn by renderSpinner
	wheelX, wheelY              float64       // the wheel image's top left
	spinKey                     spinnerKey
	pausePanel                  ui.Rect
	pauseBtn                    [len(pauseItems)]ui.Rect
	again, toHome               ui.Rect
	exit                        ui.Rect
	winFace                     *text.GoTextFace
	winX                        float64
	winTitleY, winDotY, winDotR float64
	statsY                      float64
}

func newPlay(a *app, cols, rows int, seats []int) (*playScene, error) {
	g, err := core.NewGame(cols, rows, len(seats))
	if a.set.Map != nil {
		g, err = core.NewGameWithMap(*a.set.Map, len(seats))
	}
	if err != nil {
		return nil, fmt.Errorf("new game: %w", err)
	}
	labels := make([]string, len(seats))
	for i := range seats {
		labels[i] = a.set.label(seats, i)
	}
	return newPlayOn(a, g, seats, labels, pauseLocal), nil
}

// The pause menu of a local game, and of an online one, where only the host could restart and nobody can stop the
// clock for everyone.
var (
	pauseLocal  = []int{pauseResume, pauseRestart, pauseSettings, pauseMainMenu}
	pauseOnline = []int{pauseResume, pauseSettings, pauseMainMenu}
)

// newPlayOn shows game g, which may be under way.
func newPlayOn(a *app, g *core.Game, seats []int, labels []string, items []int) *playScene {
	p := &playScene{
		cols: g.Geo.Cols, rows: g.Geo.Rows, seats: append([]int(nil), seats...), labels: labels, items: items, game: g,
		b: boardview.New(g), seed: a.rng.Uint64(), scratch: g.Clone(), aiMove: make(chan int, 1), confirm: -1, pace: -1,
		goneP: -1, goneN: -1,
	}
	if g.Map != nil {
		p.items = append(append([]int(nil), items...), pauseSaveMap)
	}
	// Every orb of a settled board sits in a cell below its capacity, and a player can't own more than all of them.
	most := 0
	for _, c := range g.Geo.Cap {
		most += max(0, int(c)-1)
	}
	p.orbsMax = strings.Repeat("8", len(strconv.Itoa(most)))
	p.settle()
	p.spin = p.wedgeAngle(p.shown)
	return p
}

// settle runs when a move has finished playing: the turn passes and the rail catches up.
func (p *playScene) settle() {
	p.shown = p.game.Turn
	for i := range p.game.Players {
		p.alive[i] = p.game.Alive(i)
	}
	if p.game.Over() && !p.over {
		p.over, p.shown, p.paused = true, int(p.game.Winner), false
		p.winTitle = p.labels[p.shown] + " wins!"
		p.results = strconv.Itoa(p.game.Moves) + " moves"
		if p.biggest >= 2 { // a chain only counts from two waves, as the callout does
			p.results += ", biggest chain ×" + strconv.Itoa(p.biggest)
		}
	}
}

func (p *playScene) move(a *app, cell int) {
	mover := p.game.Turn
	waves, err := p.game.Apply(cell)
	if err != nil {
		log.Printf("move: %v", err) // callers check Legal, so this is a bug
		return
	}
	a.snd.Place(mover)
	p.b.State.Events = p.game.TerrainEvents
	p.b.Start(cell, int8(mover), waves, a.snd)
	p.biggest = max(p.biggest, len(waves))
	if len(waves) >= 2 {
		p.chainOwner, p.chainHold = mover, 0
	}
	if !p.b.Playing {
		p.settle()
	}
}

func (p *playScene) restartGame(a *app) {
	if p.net != nil { // the host's room goes back to its lobby, and the room update takes everyone there
		if err := p.net.room.Rematch(); err != nil {
			log.Printf("rematch: %v", err)
		}
		return
	}
	g, err := core.NewGame(p.cols, p.rows, len(p.seats))
	if p.game.Map != nil {
		g, err = core.NewGameWithMap(*p.game.Map, len(p.seats))
	}
	if err == nil {
		np := newPlayOn(a, g, p.seats, p.labels, pauseLocal)
		np.returnTo = p.returnTo
		a.next = np
	}
}

func (p *playScene) update(a *app) error {
	defer func() {
		if p.returnTo != nil {
			if _, ok := a.next.(*homeScene); ok {
				a.next = p.returnTo
			}
		}
	}()
	if p.returnTo != nil && a.in.esc {
		a.next = p.returnTo
		return nil
	}
	if p.net != nil && p.pumpNet(a) {
		return nil
	}
	p.layout(a)
	if p.paused {
		err := p.updatePause(a)
		if p.net != nil && a.next == nil { // nobody can stop an online game's clock for everyone
			p.advance(a)
		}
		return err
	}
	if p.goneP >= 0 {
		p.updateGone(a)
		return nil
	}
	if a.in.esc {
		switch {
		case p.over && !p.b.Playing && p.net != nil: // leaving a finished room is a button, not a slip
		case p.over && !p.b.Playing:
			a.next = newHome()
		default:
			p.paused = true
		}
		return nil
	}
	p.updateScroll(a)
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		p.b.FastForward()
	}

	p.advance(a)

	if p.over && !p.b.Playing {
		p.overT++
		switch {
		case p.resultsAlpha() == 0: // the buttons have not faded in yet
		case p.canAgain() && a.clicked(p.again):
			p.restartGame(a)
		case a.clicked(p.toHome):
			p.leave(a)
			a.next = newHome()
		case platform.CanExit && a.clicked(p.exit):
			p.leave(a)
			return ebiten.Termination
		}
		return nil
	}

	if a.clicked(ui.CompactHit(p.menu, a.u)) {
		p.paused = true
		return nil
	}
	if p.b.Playing {
		if a.in.clicked && p.boardView.Viewport.Contains(a.in.x, a.in.y) {
			p.b.FastForward()
		}
		return nil
	}
	if p.flashT > 0 {
		p.flashT--
	}
	cell := p.cellAt(a.in.x, a.in.y)
	mine := p.mine()
	switch {
	case cell < 0:
	case mine && p.game.Legal(cell):
		a.pointer = true
		if a.in.clicked {
			p.play(a, cell)
			return nil
		}
	case a.in.clicked: // someone else's cell, or someone else's turn: say so instead of ignoring the tap
		p.flash, p.flashT = cell, flashTicks
	}
	if p.net == nil && p.seats[p.game.Turn] != seatHuman {
		p.thinkAI(a)
	}
	return nil
}

// mine reports whether the player at this screen may move now: any human seat in a local game, their own online.
func (p *playScene) mine() bool {
	if p.net != nil {
		return p.myTurn()
	}
	return p.seats[p.game.Turn] == seatHuman
}

// play makes this player's move: at once in a local game, through the host online.
func (p *playScene) play(a *app, cell int) {
	if p.net == nil {
		p.move(a, cell)
		return
	}
	c, r := p.game.Geo.Coords(cell)
	p.net.link.Send(proto.Move{C: uint8(c), R: uint8(r)})
	p.sent = true
}

// canAgain reports whether Play again is offered: always locally, and online only to the host, whose room goes back
// to its lobby for everyone.
func (p *playScene) canAgain() bool { return p.net == nil || p.net.room != nil }

// leave lets go of an online room on the way out: a host closes it, a joiner gives up the seat.
func (p *playScene) leave(a *app) {
	if p.net != nil {
		p.net.leave(a)
		a.net = nil
	}
}

// advance plays the board, the rail and, online, the host's next move on by one tick.
func (p *playScene) advance(a *app) {
	wasPlaying := p.b.Playing
	p.b.Tick(a.snd)
	p.tickRail()
	if wasPlaying && !p.b.Playing {
		p.settle()
	}
	if p.net != nil {
		p.playNet(a)
	}
}

// updatePause runs the pause menu. A local game stands still behind it: no animation ticks, and an AI move that is
// ready waits. An online game goes on.
func (p *playScene) updatePause(a *app) error {
	if p.confirmT--; p.confirmT <= 0 {
		p.confirm = -1
	}
	if a.in.esc {
		p.paused, p.confirm = false, -1
		return nil
	}
	for k, i := range p.items {
		if !a.clicked(p.pauseBtn[k]) {
			continue
		}
		// Restart throws the game away, so it asks for a second tap.
		if i == pauseRestart && p.confirm != i {
			p.confirm, p.confirmT = i, confirmTicks
			return nil
		}
		p.confirm = -1
		switch i {
		case pauseResume:
			p.paused = false
		case pauseRestart:
			p.restartGame(a)
		case pauseSettings:
			a.next = newSettingsMenu(p)
		case pauseMainMenu:
			p.leave(a)
			a.next = newHome()
		case pauseSaveMap:
			if p.game.Map != nil {
				s := newMaps(a, p, nil)
				s.importText, _ = core.MapCode(*p.game.Map)
				a.next = s
			}
		}
		return nil
	}
	if a.in.clicked && !p.pausePanel.Contains(a.in.x, a.in.y) {
		p.paused, p.confirm = false, -1
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

	if p.b.Playing && len(p.b.Waves) >= 2 {
		p.chain, p.chainHold = p.b.Wave+1, calloutHold
	} else if p.chainHold > 0 {
		p.chainHold--
	}
	if p.chain != p.chainN {
		p.chainN, p.chainText = p.chain, "Chain ×"+strconv.Itoa(p.chain)
	}

	var orbs [core.MaxPlayers]int
	for i, o := range p.b.State.Owner {
		if o != core.NoOwner {
			orbs[o] += int(p.b.State.Count[i])
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
		budget := aiBudget(s, g.Geo.Cells())
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

// aiBudget is strategy s's work per move on a board of cells. Go in a browser runs on one thread, so a thinking AI
// holds up drawing there: every strategy gets the default budget, and boards bigger than the largest preset
// proportionally less, keeping the pause about as short as on that preset.
func aiBudget(s ai.Strategy, cells int) int {
	if runtime.GOOS != "js" {
		return s.Budget()
	}
	z := boardSizes[len(boardSizes)-1]
	return min(ai.DefaultBudget, ai.DefaultBudget*z.short*z.long/cells)
}

func (p *playScene) layout(a *app) {
	v := a.safe()
	if v == p.lv && a.u == p.lu {
		if p.listShown != p.shown {
			p.revealSeat(a)
		}
		return
	}
	p.lv, p.lu = v, a.u
	// Phones (upright, or short on their side) set the seat rows at the button label size; the desktop rail keeps body
	// size for couch distance.
	p.compact = a.portrait() || v.H < 640*a.u
	p.tokR, p.spinR = 0, 0
	if a.portrait() {
		p.layoutDock(a, v)
	} else {
		p.layoutRail(a, v)
	}
	p.layoutScrolled(a)
	p.revealSeat(a)
	p.layoutPause(a, v)
	p.layoutResults(a, v)
	p.layoutGone(a, v)
}

// tableRows is how many rows the seats take in p.tableCols columns.
func (p *playScene) tableRows() int { return (len(p.seats) + p.tableCols - 1) / p.tableCols }

// menuWidth is the narrowest Menu pill, as drawMenuButton sets it out.
func menuWidth(a *app) float64 { return 42*a.u + ui.TextWidth("Menu", a.f.LabelBold) + 18*a.u }

// layoutRail keeps Menu, wheel, chip and scrolling seats in that order. The rail's width is bounded by its height
// so the wheel always fills its width while leaving a visible strip of the player list below.
func (p *playScene) layoutRail(a *app, v ui.Rect) {
	u := a.u
	pad, menuGap, calloutGap, listGap := 32*u, 24*u, 20*u, 24*u
	if p.compact {
		// 12 px gaps keep the pointer's tip, which reaches 12 px past the spinner, clear of Menu and the seats.
		pad, menuGap, calloutGap, listGap = 16*u, 12*u, 12*u, 12*u
	}
	h := v.H - 2*pad
	cw, ch := p.calloutSize(a)
	fixed := 2*ch + menuGap + calloutGap + listGap
	p.tableCols = 1
	minW := max(menuWidth(a), cw, p.seatTableWidth(a))
	visible := p.seatRowPitch(a) - 4*u
	maxW := max(minW, h-fixed-visible)
	w := minW
	area := ui.Rect{X: v.X + pad, Y: v.Y + pad, W: v.W - 3*pad - w, H: h}
	if bw, _ := p.b.Size(area); bw < area.W {
		w = min(maxW, w+area.W-bw)
		area.W = v.W - 3*pad - w
	}
	x, y := v.Right()-pad-w, v.Y+pad
	if len(p.seats) > 6 {
		p.tableCols = 2
		if p.seatTableWidth(a) > w {
			p.tableCols = 1
		}
	}
	p.boardView.Set(area, area.H)
	p.menu = ui.Rect{X: x, Y: y, W: w, H: ch}
	p.spinR = w / 2
	p.spinX, p.spinY = x+w/2, p.menu.Bottom()+menuGap+p.spinR
	below := p.spinY + p.spinR
	p.callout = ui.Rect{X: x, Y: below + calloutGap, W: w, H: ch}
	below = p.callout.Bottom()
	p.seatView.Set(ui.Rect{X: x, Y: below + listGap, W: w, H: v.Bottom() - pad - below - listGap}, p.seatTableHeight(a))
}

// seatFaces keep desktop text at body size in either column layout; phones retain their compact sizes.
func (p *playScene) seatFaces(a *app) (name, bold *text.GoTextFace) {
	switch {
	case p.compact && p.tableCols > 1:
		return a.f.Small, a.f.SmallBold
	case p.compact:
		return a.f.Label, a.f.LabelBold
	}
	return a.f.Body, a.f.BodyBold
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
	case p.compact && p.tableCols > 1:
		return 6 * u, 4 * u, 6 * u, dot - 2*u
	case p.compact:
		return 10 * u, 8 * u, 12 * u, dot
	}
	return 14 * u, 12 * u, 12 * u, dot
}

// seatTableWidth is the narrowest table whose rows fit in p.tableCols columns, set out as drawRail draws them:
// inset, dot, space, type, gap, orbs (or "out"), inset, and 8 px between columns.
func (p *playScene) seatTableWidth(a *app) float64 {
	name, bold := p.seatFaces(a)
	in, sp, gap, dot := p.seatInsets(a)
	row := 2*in + 2*dot + sp + widestLabel(p.labels, name) + gap + max(ui.TextWidth(p.orbsMax, bold), ui.ChipWidth(a.ui(), "Out", p.chipH(a)))
	cols := float64(p.tableCols)
	return cols*row + (cols-1)*8*a.u
}

// widestLabel is the width of the widest of labels, set in face.
func widestLabel(labels []string, face *text.GoTextFace) float64 {
	w := 0.0
	for _, l := range labels {
		w = max(w, ui.TextWidth(l, face))
	}
	return w
}

// calloutStyle keeps text and shared Menu/chip height together; only short landscape rails tighten them.
func (p *playScene) calloutStyle(a *app) (*text.GoTextFace, float64) {
	if !p.compact {
		return a.f.Callout, ui.CompactButtonHeight * a.u
	}
	if a.portrait() {
		return a.f.CalloutCompact, ui.CompactButtonHeight * a.u
	}
	return a.f.CalloutTight, ui.CompactButtonTightHeight * a.u
}

// calloutSize keeps the chain pill at Menu's height, with compact text sized for "Chain ×88" (also fitting three digits).
func (p *playScene) calloutSize(a *app) (w, h float64) {
	face, h := p.calloutStyle(a)
	if p.compact {
		return ui.TextWidth("Chain ×88", face) + 20*a.u, h
	}
	return 180 * a.u, h
}

// seatRowPitch preserves readable rows; overflow belongs to the viewport instead of the type or wheel.
func (p *playScene) seatRowPitch(a *app) float64 {
	pitch := 40 * a.u
	if p.compact {
		pitch = 32 * a.u
		if a.portrait() && p.tableCols == 1 {
			pitch = 36 * a.u
		}
	}
	return pitch
}

// chipH is the Out chip's height: the standard one, or less on rows too short to fit it inside their 4 px inset.
func (p *playScene) chipH(a *app) float64 {
	return min(ui.ChipHeight*a.u, p.seatRowPitch(a)-12*a.u)
}

func (p *playScene) seatTableHeight(a *app) float64 {
	return float64(p.tableRows()) * p.seatRowPitch(a)
}

// layoutDock anchors the portrait board and players above the fixed bottom controls. The board fills the width
// unless screen height constrains it; only the players scroll.
func (p *playScene) layoutDock(a *app, v ui.Rect) {
	u := a.u
	pad, gap := 16*u, 16*u
	x, w, top, bottom := v.X+pad, v.W-2*pad, v.Y+pad, v.Bottom()-pad
	p.tableCols = 1
	if len(p.seats) > 6 {
		p.tableCols = 2
		if p.seatTableWidth(a) > w {
			p.tableCols = 1
		}
	}
	_, ch := p.calloutStyle(a)
	side := (w - ch - 16*u) / 2
	p.callout = ui.Rect{X: x, Y: bottom - ch, W: side, H: ch}
	p.tokR, p.tokX, p.tokY = ch/2, v.Cx(), p.callout.Cy()
	menuX := p.tokX + p.tokR + 8*u
	p.menu = ui.Rect{X: menuX, Y: p.callout.Y, W: side, H: ch}
	boardW, boardH := p.b.Size(ui.Rect{W: w, H: math.Inf(1)})
	viewH := min(boardH, p.menu.Y-gap-top-gap-p.seatRowPitch(a))
	boardW *= viewH / boardH
	tableH := min(p.seatTableHeight(a), p.menu.Y-gap-top-gap-viewH)
	tableY := p.menu.Y - gap - tableH
	p.boardView.Set(ui.Rect{X: x + (w-boardW)/2, Y: tableY - gap - viewH, W: boardW, H: viewH}, viewH)
	p.seatView.Set(ui.Rect{X: x, Y: tableY, W: w, H: tableH}, p.seatTableHeight(a))
}

func (p *playScene) layoutScrolled(a *app) {
	v := p.boardView.Viewport
	p.b.Layout(ui.Rect{X: v.X, Y: v.Y - p.boardView.Offset, W: v.W, H: p.boardView.Height}, a.u)
	v = p.seatView.Viewport
	p.layoutSeats(a, ui.Rect{X: v.X, Y: v.Y - p.seatView.Offset, W: v.W, H: p.seatView.Height}, p.tableRows())
}

func (p *playScene) revealSeat(a *app) {
	p.seatView.Reveal(p.rowRect[p.shown])
	p.listShown = p.shown
	p.layoutScrolled(a)
}

func (p *playScene) updateScroll(a *app) {
	seats, board := p.seatView.Update(a.ui()), p.boardView.Update(a.ui())
	if seats || board {
		p.layoutScrolled(a)
	}
}

func (p *playScene) cellAt(x, y float64) int {
	if !p.boardView.Viewport.Contains(x, y) {
		return -1
	}
	return p.b.CellAt(x, y)
}

// layoutSeats fills table with the seat rows in p.tableCols columns, rows to a column, 8 px apart.
func (p *playScene) layoutSeats(a *app, table ui.Rect, rows int) {
	u := a.u
	rowH := table.H / float64(rows)
	cols := float64(p.tableCols)
	w := (table.W - (cols-1)*8*u) / cols
	for i := range p.seats {
		p.rowRect[i] = ui.Rect{X: table.X + float64(i/rows)*(w+8*u), Y: table.Y + float64(i%rows)*rowH, W: w, H: rowH - 4*u}
	}
}

// layoutPause centres the pause menu: a panel of full-width buttons, two by two when the screen is too short for one
// column.
func (p *playScene) layoutPause(a *app, v ui.Rect) {
	u := a.u
	bw, bh, gap := min(300*u, v.W-32*u-56*u), ui.QuietButtonHeight*u, 12*u
	cols := 1
	rows := func() float64 { return float64((len(p.items) + cols - 1) / cols) }
	ph := func() float64 { return 28*u + 40*u + 24*u + rows()*bh + (rows()-1)*gap + 28*u }
	if ph() > v.H-32*u {
		cols = 2
		bw = min(bw, (v.W-32*u-56*u-gap)/2)
	}
	pw := float64(cols)*bw + float64(cols-1)*gap + 56*u
	p.pausePanel = ui.Rect{X: v.X + (v.W-pw)/2, Y: v.Y + (v.H-ph())/2, W: pw, H: ph()}
	for i := range p.items {
		p.pauseBtn[i] = ui.Rect{X: p.pausePanel.X + 28*u + float64(i%cols)*(bw+gap), Y: p.pausePanel.Y + 92*u + float64(i/cols)*(bh+gap), W: bw, H: bh}
	}
}

// layoutResults places the result: one centred column on the winner's ink, the title as large as the screen allows.
// On a screen too short for the column the dot shrinks and the buttons stand side by side.
func (p *playScene) layoutResults(a *app, v ui.Rect) {
	u := a.u
	bw, bh, gap := 220*u, ui.QuietButtonHeight*u, 12*u // the pause menu's button height and gap
	size := max(56*u, min(120*u, v.W*0.075, v.H*0.13))
	// Whoever wins, the title fits across the screen.
	face, tw := ui.NewFace(ui.SrcDisplay, size, 800), 0.0
	for _, l := range p.labels {
		tw = max(tw, ui.TextWidth(l+" wins!", face))
	}
	if tw > v.W-32*u {
		size *= (v.W - 32*u) / tw
	}
	p.winFace = ui.NewFace(ui.SrcDisplay, size, 800)
	p.winX = v.Cx()
	n := 2
	if platform.CanExit {
		n = 3
	}
	if !p.canAgain() {
		n--
	}
	if v.W >= v.H {
		bw = min(bw, (v.W-32*u-float64(n-1)*gap)/float64(n))
	}
	p.winDotR = max(44*u, min(80*u, v.H*0.08))
	above := func() float64 { return 2*p.winDotR + 28*u + size*0.6 } // dot top to title centre
	below := size*0.5 + 34*u + 44*u + float64(n)*bh + float64(n-1)*gap
	p.winTitleY = v.Y + v.H/2 - 90*u
	row := false
	if p.winTitleY-above() < v.Y+16*u || p.winTitleY+below > v.Bottom()-16*u {
		row = float64(n)*bw+float64(n-1)*gap <= v.W-32*u
		if row {
			below = size*0.5 + 34*u + 32*u + bh
		}
		p.winDotR = max(24*u, min(p.winDotR, (v.H-32*u-below-28*u-size*0.6)/2))
		p.winTitleY = v.Y + (v.H-above()-below)/2 + above()
	}
	p.statsY = p.winTitleY + size*0.5 + 34*u
	p.winDotY = p.winTitleY - size*0.6 - 28*u - p.winDotR
	if row {
		x := p.winX - (float64(n)*bw+float64(n-1)*gap)/2
		p.again = ui.Rect{X: x, Y: p.statsY + 32*u, W: bw, H: bh}
		p.toHome = ui.Rect{X: p.again.Right() + gap, Y: p.again.Y, W: bw, H: bh}
		p.exit = ui.Rect{X: p.toHome.Right() + gap, Y: p.again.Y, W: bw, H: bh}
		p.dropAgain()
		return
	}
	p.again = ui.Rect{X: p.winX - bw/2, Y: p.statsY + 44*u, W: bw, H: bh}
	p.toHome = ui.Rect{X: p.winX - bw/2, Y: p.again.Bottom() + gap, W: bw, H: bh}
	p.exit = ui.Rect{X: p.winX - bw/2, Y: p.toHome.Bottom() + gap, W: bw, H: bh}
	p.dropAgain()
}

// dropAgain moves the other result buttons up into Play again's place when it isn't offered.
func (p *playScene) dropAgain() {
	if !p.canAgain() {
		p.again, p.toHome, p.exit = ui.Rect{}, p.again, p.toHome
	}
}

func (p *playScene) draw(dst *ebiten.Image, a *app) {
	p.layout(a)
	if p.over && !p.b.Playing && p.overT >= wipeTicks {
		p.drawResults(dst, a)
		return
	}
	sc, u := a.sc, a.u
	turn := p.shown
	if p.over && !p.b.Playing {
		turn = -1
	}
	// The board always fits its view, so it draws unclipped and burst rings at the edges spill onto the mat as on Home.
	// The board's tick clock stops during a local pause and resumes without jumping ahead.
	p.b.Draw(dst, sc, p.b.Clock*(1000.0/60), turn, a.set.Marks)
	if !p.b.Playing && !p.over && !p.paused && p.mine() {
		if cell := p.cellAt(a.in.x, a.in.y); cell >= 0 && p.game.Legal(cell) {
			p.b.DrawHover(dst, cell, sc.Players[p.game.Turn], u)
		}
	}
	if p.flashT > 0 && !p.over && !p.b.Playing {
		p.b.DrawHover(dst, p.flash, ui.WithAlpha(sc.Players[p.game.Turn], float64(p.flashT)/flashTicks), u)
	}
	p.boardView.DrawScrollbar(dst, a.ui())
	p.drawRail(dst, a)
	switch {
	case p.over && !p.b.Playing:
		p.drawResults(dst, a)
	case p.paused:
		p.drawPause(dst, a)
	case p.goneP >= 0:
		p.drawGone(dst, a)
	}
}

// drawMenuButton draws the rail's Menu pill with a three-line glyph, laid out like Back.
func (p *playScene) drawMenuButton(dst *ebiten.Image, a *app) {
	u, r := a.u, p.menu
	ink := ui.DrawButtonSurface(dst, a.ui(), r, ui.ButtonTile, true, a.over(ui.CompactHit(r, u)) && !p.paused && !p.over)
	ui.Hamburger(dst, r.X+22*u, r.Cy(), 16*u, 2.5*u, ink)
	ui.DrawText(dst, "Menu", a.f.LabelBold, r.X+42*u, r.Cy(), text.AlignStart, ink)
}

func (p *playScene) drawRail(dst *ebiten.Image, a *app) {
	sc := a.sc
	p.drawMenuButton(dst, a)
	if p.spinR > 0 {
		p.drawSpinner(dst, a)
	}

	// The chain callout, in the mover's ink.
	if p.chainHold > 0 && p.chain >= 2 {
		c := sc.Players[p.chainOwner]
		alpha := min(1, float64(p.chainHold)/20)
		ui.FillPill(dst, p.callout, ui.WithAlpha(c, alpha))
		face, _ := p.calloutStyle(a)
		ui.DrawText(dst, p.chainText, face, p.callout.Cx(), p.callout.Cy(), text.AlignCenter, ui.WithAlpha(ui.InkOn(c), alpha))
	}

	// The seat rows, set out as seatTableWidth measures them.
	name, bold := p.seatFaces(a)
	in, sp, gap, dot := p.seatInsets(a)
	seatsDst := p.seatView.Clip(dst)
	for i, label := range p.labels {
		r := p.rowRect[i]
		alive, current := p.alive[i], i == p.shown && !p.over
		if current {
			ui.FillPill(seatsDst, r, sc.Tile)
		}
		dr := min(dot, r.H*0.34)
		ui.SeatDot(seatsDst, sc, i, r.X+in+dr, r.Cy(), dr, !alive, a.set.Marks)
		ink := sc.Ink
		if !alive {
			ink = sc.Dim
		}
		ui.DrawText(seatsDst, label, name, r.X+in+2*dr+sp, r.Cy(), text.AlignStart, ink)
		if alive {
			room := r.W - 2*in - 2*dr - sp - ui.TextWidth(label, name) - gap
			scale := min(1, room/max(1, ui.TextWidth(p.orbS[i], bold)))
			ui.DrawScaledText(seatsDst, p.orbS[i], bold, r.Right()-in, r.Cy(), text.AlignEnd, scale, ink)
		} else {
			ui.DrawChip(seatsDst, a.ui(), ui.Chip{Word: "Out", Kind: ui.ChipDanger}, r.Right()-in, r.Cy(), p.chipH(a))
		}
	}

	p.seatView.DrawScrollbar(dst, a.ui(
	// Upright, the centered turn dot also shows the mover's orb count.
	))

	if p.tokR > 0 {
		p.drawTurnCount(dst, a)
	}
}

// spinnerKey is what the cached spinner images show.
type spinnerKey struct {
	x, y, r, u float64
	alive      [core.MaxPlayers]bool
	sc         *ui.Palette
}

// drawSpinner draws the turn spinner: a wedge per seat in its ink (out seats printed blank) and the pointer on the
// player to move. Both are images redrawn only when what they show changes; the pointer turns as it is drawn.
func (p *playScene) drawSpinner(dst *ebiten.Image, a *app) {
	var op ebiten.DrawImageOptions
	if k := (spinnerKey{p.spinX, p.spinY, p.spinR, a.u, p.alive, a.sc}); p.wheel == nil || k != p.spinKey {
		p.spinKey = k
		p.renderSpinner(a)
	}
	op = ebiten.DrawImageOptions{}
	op.GeoM.Translate(p.wheelX, p.wheelY)
	dst.DrawImage(p.wheel, &op)
	h := float64(p.pointer.Bounds().Dy()) / 2
	op = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Translate(-h, -h)
	op.GeoM.Rotate(p.spin)
	op.GeoM.Translate(p.spinX, p.spinY)
	dst.DrawImage(p.pointer, &op)
	p.drawWheelCounts(dst, a)
}

var spinnerPath vector.Path

// renderSpinner caches the wheel at whole-pixel alignment and the rim pointer pointing right around its hub.
func (p *playScene) renderSpinner(a *app) {
	if p.wheel != nil {
		p.wheel.Deallocate()
		p.pointer.Deallocate()
	}
	sc, u, n, r := a.sc, a.u, len(p.seats), p.spinR
	p.wheelX, p.wheelY = math.Floor(p.spinX-r)-ui.SpritePad, math.Floor(p.spinY-r)-ui.SpritePad
	x, y := p.spinX-p.wheelX, p.spinY-p.wheelY
	p.wheel = ebiten.NewImage(int(math.Ceil(x+r))+ui.SpritePad, int(math.Ceil(y+r))+ui.SpritePad)
	for i := range n {
		a0 := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		c := sc.Players[i]
		if !p.alive[i] {
			c = sc.Tile
		}
		spinnerPath.Reset()
		spinnerPath.MoveTo(float32(x), float32(y))
		spinnerPath.Arc(float32(x), float32(y), float32(r), float32(a0), float32(a0+2*math.Pi/float64(n)), 0)
		spinnerPath.Close()
		ui.FillPath(p.wheel, &spinnerPath, c)
	}
	for i := range n {
		an := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		ui.Line(p.wheel, x, y, x+r*math.Cos(an), y+r*math.Sin(an), 3*u, sc.Ground)
	}
	ui.FillCircle(p.wheel, x, y, r*0.2, sc.Ground)

	rim := 2 * u
	tip, w, h := r+12*u-rim, r*0.09, math.Ceil(r*0.1+rim)+ui.SpritePad
	p.pointer = ebiten.NewImage(int(math.Ceil(h+tip))+ui.SpritePad, int(2*h))
	spinnerPath.Reset()
	spinnerPath.MoveTo(float32(h+tip), float32(h))
	spinnerPath.LineTo(float32(h+r-4*u), float32(h+w))
	spinnerPath.LineTo(float32(h+r-4*u), float32(h-w))
	spinnerPath.Close()
	ui.StrokePath(p.pointer, &spinnerPath, 2*rim, sc.Ground)
	ui.FillPath(p.pointer, &spinnerPath, sc.Ink)
	ui.FillCircle(p.pointer, h, h, r*0.1, sc.Ink)
}

// drawPause dims the game and shows the pause menu.
func (p *playScene) drawPause(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	ui.FillRound(dst, ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, 0, ui.WithAlpha(sc.Ground, 0.8))
	ui.FillRound(dst, p.pausePanel, 28*u, sc.Tile)
	title := "Paused"
	if p.net != nil {
		title = "Menu" // the game goes on for everyone else
	}
	ui.DrawText(dst, title, a.f.Title, p.pausePanel.Cx(), p.pausePanel.Y+28*u+20*u, text.AlignCenter, sc.Ink)
	for k, i := range p.items {
		r := p.pauseBtn[k]
		style, label := ui.ButtonGround, pauseItems[i]
		switch {
		case p.confirm == i:
			style, label = ui.ButtonInk, pauseConfirm[i]
		case i == pauseResume:
			style = ui.ButtonInk
		}
		ui.DrawButton(dst, a.ui(), r, label, "", style, true)
	}
}

// drawResults floods the screen with the winner's ink from the board's centre, then shows the result.
func (p *playScene) drawResults(dst *ebiten.Image, a *app) {
	sc, f, u := a.sc, &a.f, a.u
	w := sc.Players[p.shown]
	ink := ui.InkOn(w)
	t := min(1, float64(p.overT)/wipeTicks)
	e := 1 - math.Pow(1-t, 3)
	cx, cy := p.boardView.Viewport.Cx(), p.boardView.Viewport.Cy()
	if t < 1 {
		ui.FillCircle(dst, cx, cy, e*math.Hypot(max(cx, a.w-cx), max(cy, a.h-cy)), w)
	} else {
		dst.Fill(&sc.Players[p.shown]) // a pointer, so the colour isn't boxed into a new interface value every frame
	}
	alpha := p.resultsAlpha()
	if alpha == 0 {
		return
	}
	// The winner's dot, solid in the contrasting ink so it can't read as an out seat's ring, then the title at poster
	// size.
	x := p.winX
	ui.FillCircle(dst, x, p.winDotY, p.winDotR, ui.WithAlpha(ink, alpha))
	if alpha == 1 && a.set.Marks {
		ui.DrawMarkIn(dst, p.shown, x, p.winDotY, p.winDotR, w)
	}
	ui.DrawText(dst, p.winTitle, p.winFace, x, p.winTitleY, text.AlignCenter, ui.WithAlpha(ink, alpha))
	face := f.Body
	if ui.TextWidth(p.results, face) > p.lv.W-32*u { // a phone held upright
		face = f.Label
	}
	ui.DrawText(dst, p.results, face, x, p.statsY, text.AlignCenter, ui.WithAlpha(ink, alpha))

	if p.canAgain() {
		resultButton(dst, a, p.again, "Play again", w, ink, true, alpha)
	}
	resultButton(dst, a, p.toHome, "Main menu", w, ink, false, alpha)
	if platform.CanExit {
		resultButton(dst, a, p.exit, "Exit", w, ink, false, alpha)
	}
}

// resultsAlpha fades the result in once the flood is halfway out.
func (p *playScene) resultsAlpha() float64 { return min(1, max(0, float64(p.overT-wipeTicks/2)/15)) }

// resultButton draws a button on the winner's ink: filled with the contrasting ink when primary, outlined
// otherwise.
func resultButton(dst *ebiten.Image, a *app, r ui.Rect, label string, bg, ink color.RGBA, primary bool, alpha float64) {
	hot := a.over(r)
	if primary {
		fill := ink
		if hot {
			fill = ui.Mix(ink, bg, 0.2)
		}
		ui.FillPill(dst, r, ui.WithAlpha(fill, alpha))
		ui.DrawText(dst, label, a.f.Heading, r.Cx(), r.Cy(), text.AlignCenter, ui.WithAlpha(bg, alpha))
		return
	}
	if hot {
		ui.FillPill(dst, r, ui.WithAlpha(ui.Mix(bg, ink, 0.15), alpha))
	}
	ui.StrokeRound(dst, r, r.H/2, 2.5*a.u, ui.WithAlpha(ink, alpha))
	ui.PillLabel(dst, a.ui(), r, label, "", ui.WithAlpha(ink, alpha))
}
