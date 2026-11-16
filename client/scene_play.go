package client

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"
	"slices"
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
	aiDelayMin  = 18
	aiDelayMax  = 36
	calloutHold = 90 // ticks a finished chain's callout stays up
	wipeTicks   = 40 // the winner's ink flooding the screen
	flashTicks  = 30 // a tapped cell the player can't use flashes its outline
)

// The pause menu, in order.
var pauseItems = [...]string{"Resume", "Restart", "Save map", "Settings", "Main menu"}

const (
	pauseResume = iota
	pauseRestart
	pauseSaveMap
	pauseSettings
	pauseMainMenu
)

// playScene is a local game with a landscape turn wheel or a portrait turn dot, the mover's name, a team game's
// legend, the Players dialog a tap on any of them opens, a pause menu and the results flood.
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
	spin      float64                  // spinner pointer angle, chasing the shown player's wedge
	share     [core.MaxPlayers]float64 // each seat's part of the wheel, by slice; they sum to 1

	chain, chainHold int // waves shown in the current or last chain, and ticks left to show it
	chainOwner       int
	chainText        string
	chainN           int
	biggest          int // longest chain this game, in waves

	paused   bool
	roster   bool       // the Players dialog is open: every seat's team, name and orbs, over a game that goes on
	ask      *askDialog // Restart throws the game away, so it asks first, in place of the pause menu
	watch    *askDialog // every human is out: watch the bots finish, or leave
	asked    bool       // watch is asked once a game
	flash    int        // cell flashing after a tap the mover can't play there
	flashT   int
	over     bool
	overT    int // ticks since the game ended
	winTitle string

	// The results' two boards, Survival then Knockouts. kills and outAt are kept as the game goes; the rest is set
	// when it ends.
	kills, outAt [core.MaxPlayers]int       // seats a player put out, and the move a seat went out on (0 while in)
	stats        [2]string                  // moves, and the biggest chain from ×2
	order        [2][core.MaxPlayers]int    // seats from first place down
	word         [2][core.MaxPlayers]string // a seat's place ("1st") or knockouts ("3")
	medal        [2][core.MaxPlayers]int    // a seat's index in ui.Medals, or past them for none
	tab          int                        // the board an upright screen shows

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
	name                        ui.Rect                  // the mover's name row, its content centred
	legend                      [core.MaxTeams]ui.Rect   // a team game's legend: each team's chip and dots
	peek                        [3]ui.Rect               // a tap opens Players: wheel or turn dot, name, legend
	rosterPanel, rosterBack     ui.Rect                  // the Players dialog and its Back
	rowRect                     [core.MaxPlayers]ui.Rect // the dialog's seat rows
	tableCols                   int                      // and their columns
	boardView, seatView         ui.Scroll
	legendView                  ui.Scroll // the legend scrolls where a short rail can't hold all its lines
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
	winFace, statFace           *text.GoTextFace
	winX                        float64
	winTitleY, winDotY, winDotR float64
	statsY, statLabelY, statDX  float64    // the stats' numbers, their labels, and each one's distance from winX
	tabs, cards                 [2]ui.Rect // side by side there are no tabs; upright only cards[0], showing p.tab
	rankView                    ui.Scroll  // the boards' rows
	rankPitch, rankChipW        float64    // a row's height and its chip's width
	rankIn                      float64    // a card's padding
}

func newPlay(a *app, cols, rows int, seats []int) (*playScene, error) {
	g, err := core.NewGame(cols, rows, len(seats))
	if a.set.Map != nil {
		g, err = core.NewGameWithMap(*a.set.Map, len(seats))
	}
	if err == nil && a.set.Teams != 0 {
		err = g.SetTeams(a.set.Team)
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
// clock for everyone. What acts on the game comes first and leaving it last; Save map shows only on a custom map.
var (
	pauseLocal  = []int{pauseResume, pauseRestart, pauseSaveMap, pauseSettings, pauseMainMenu}
	pauseOnline = []int{pauseResume, pauseSaveMap, pauseSettings, pauseMainMenu}
)

// newPlayOn shows game g, which may be under way.
func newPlayOn(a *app, g *core.Game, seats []int, labels []string, items []int) *playScene {
	p := &playScene{
		cols: g.Geo.Cols, rows: g.Geo.Rows, seats: append([]int(nil), seats...), labels: labels, items: items, game: g,
		b: boardview.New(g), seed: a.rng.Uint64(), scratch: g.Clone(), aiMove: make(chan int, 1), pace: -1,
		goneP: -1, goneN: -1,
	}
	if g.Map == nil {
		p.items = slices.DeleteFunc(slices.Clone(items), func(i int) bool { return i == pauseSaveMap })
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
	p.slice()
	p.tally(-1) // a seat skipped before it placed an orb, or already out in the host's snapshot
	human, humanIn := false, false
	for i, s := range p.seats {
		human, humanIn = human || s == seatHuman, humanIn || s == seatHuman && p.alive[i]
	}
	if human && !humanIn && !p.asked && !p.game.Over() {
		p.asked, p.watch = true, &askDialog{text: "Continue watching?", yes: "Continue", no: "Leave"}
	}
	if p.game.Over() && !p.over {
		p.over, p.shown, p.paused, p.watch = true, int(p.game.Winner), false, nil
		p.winTitle = p.labels[p.shown] + " wins!"
		if p.game.Mix != nil {
			p.winTitle = "Team " + teamLetters[p.game.Team[p.shown]] + " wins!"
		}
		p.rank()
	}
}

// tally notes the seats that went out since it last ran, and credits mover with each of them but itself. A seat
// nobody put out (mover -1) is nobody's knockout.
// shortcut: kept by this screen, so a player who joins or resyncs mid-game misses earlier knockouts; move the tally
// into core's snapshot if that matters.
func (p *playScene) tally(mover int) {
	for i := range p.game.Players {
		if p.outAt[i] == 0 && !p.game.Alive(i) {
			p.outAt[i] = max(1, p.game.Moves)
			if mover >= 0 && mover != i {
				p.kills[mover]++
			}
		}
	}
}

// rank sets out the results: the stats, and each board's order, words and medals. Seats level on a board share a
// place, and the next place is skipped, as on a podium. No knockouts earns no medal.
func (p *playScene) rank() {
	n := p.game.Players
	p.stats = [2]string{strconv.Itoa(p.game.Moves), ""}
	if p.biggest >= 2 { // a chain only counts from two waves, as the callout does
		p.stats[1] = "×" + strconv.Itoa(p.biggest)
	}
	score := [2]func(int) int{
		func(i int) int { // the winner, or every player of the winning team, outlived everyone; the rest by when they went out
			if p.game.Won(int8(i)) {
				return math.MaxInt
			}
			return p.outAt[i]
		},
		func(i int) int { return p.kills[i] },
	}
	for i := range n {
		p.order[0][i] = i
	}
	for b, s := range score {
		if b == 1 {
			p.order[1] = p.order[0] // level on knockouts, whoever lasted longer comes first
		}
		slices.SortStableFunc(p.order[b][:n], func(x, y int) int { return s(y) - s(x) })
		for i := range n {
			place := 0
			for j := range n {
				if s(j) > s(i) {
					place++
				}
			}
			p.medal[b][i], p.word[b][i] = place, ordinal(place+1)
			if b == 1 {
				p.word[b][i] = strconv.Itoa(p.kills[i])
				if p.kills[i] == 0 {
					p.medal[b][i] = len(ui.Medals)
				}
			}
		}
	}
}

// ordinal is n as a place: 1st, 2nd, 3rd, then 4th to 12th, as far as the seats go.
func ordinal(n int) string {
	if n <= 3 {
		return [...]string{"1st", "2nd", "3rd"}[n-1]
	}
	return strconv.Itoa(n) + "th"
}

func (p *playScene) move(a *app, cell int) {
	mover := p.game.Turn
	waves, err := p.game.Apply(cell)
	if err != nil {
		log.Printf("move: %v", err) // callers check Legal, so this is a bug
		return
	}
	p.b.State.Events = p.game.TerrainEvents
	p.b.Start(cell, int8(mover), waves, p.game.ProducedAt, a.snd)
	p.tally(mover)
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
	if err == nil && p.game.Mix != nil {
		err = g.SetTeams(p.game.Team[:p.game.Players])
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
	if p.watch != nil {
		esc := a.in.esc // Esc keeps watching: leaving is a button, not a slip
		switch yes, leave := p.watch.update(a); {
		case yes || esc:
			p.watch = nil
		case leave:
			p.leave(a)
			a.next = newHome()
		}
		if p.net != nil && a.next == nil { // as behind the pause menu, an online game goes on
			p.advance(a)
		}
		return nil
	}
	if p.roster {
		p.updateRoster(a)
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
		case a.clicked(ui.CompactHit(p.tabs[0], a.u)), a.clicked(ui.CompactHit(p.tabs[1], a.u)):
			p.tab, p.rankView.Offset = 0, 0
			if a.in.x >= p.tabs[1].X {
				p.tab = 1
			}
		case p.rankView.Update(a.ui()):
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
	for _, r := range p.peek {
		if hit := ui.CompactHit(r, a.u); r.W > 0 && a.over(hit) {
			if a.pointer = true; a.clicked(hit) {
				p.roster = true
				p.revealSeat(a)
				return nil
			}
		}
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

// updateRoster runs the Players dialog: Back, Esc or a tap outside closes it. The game goes on behind it, bots
// included; only this screen's own move waits, and a finished game closes it for the result.
func (p *playScene) updateRoster(a *app) {
	if p.seatView.Update(a.ui()) {
		p.layoutScrolled(a)
	}
	if a.in.esc || a.clicked(p.rosterBack) || a.in.clicked && !p.rosterPanel.Contains(a.in.x, a.in.y) {
		p.roster = false
		return
	}
	p.advance(a)
	switch {
	case p.over:
		p.roster = false
	case !p.b.Playing && p.net == nil && p.seats[p.game.Turn] != seatHuman:
		p.thinkAI(a)
	}
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
	if p.ask != nil {
		yes, back := p.ask.update(a)
		if yes || back {
			p.ask = nil
		}
		if yes {
			p.restartGame(a)
		}
		return nil
	}
	if a.in.esc {
		p.paused = false
		return nil
	}
	for k, i := range p.items {
		if !a.clicked(p.pauseBtn[k]) {
			continue
		}
		switch i {
		case pauseResume:
			p.paused = false
		case pauseRestart:
			p.ask = &askDialog{text: "Restart this game?", yes: "Restart"}
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
	if d > 1.5*math.Pi {
		d -= 2 * math.Pi // the slice moved back under the pointer: follow it, not once around
	}
	if math.Abs(d) < 1e-3 {
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
	if mix := p.b.State.Mix; mix != nil { // a team game's cell holds several colours: count each player's own
		stride := len(mix) / len(p.b.State.Count)
		for i, n := range mix {
			orbs[i%stride] += int(n)
		}
	} else {
		for i, o := range p.b.State.Owner {
			if o != core.NoOwner {
				orbs[o] += int(p.b.State.Count[i])
			}
		}
	}
	for i := range p.seats {
		if orbs[i] != p.orbs[i] || p.orbS[i] == "" {
			p.orbs[i], p.orbS[i] = orbs[i], strconv.Itoa(orbs[i])
		}
	}
	p.slice()
}

// slice shares the wheel out as a pie: half equally between the players still in, so each keeps a slice its count
// fits and a player yet to move still shows, and half by orbs. Out players have no slice.
func (p *playScene) slice() {
	in, total := 0, 0
	for i := range p.seats {
		if p.alive[i] {
			in, total = in+1, total+p.orbs[i]
		}
	}
	for i := range p.seats {
		switch {
		case !p.alive[i]:
			p.share[i] = 0
		case total == 0:
			p.share[i] = 1 / float64(in)
		default:
			p.share[i] = 0.5/float64(in) + 0.5*float64(p.orbs[i])/float64(total)
		}
	}
}

// wedgeStart is where seat i's slice starts on the spinner, clockwise from the top.
func (p *playScene) wedgeStart(i int) float64 {
	before := 0.0
	for _, s := range p.share[:i] {
		before += s
	}
	return -math.Pi/2 + 2*math.Pi*before
}

// wedgeAngle is the middle of seat i's slice.
func (p *playScene) wedgeAngle(i int) float64 { return p.wedgeStart(i) + math.Pi*p.share[i] }

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
	p.layoutRoster(a, v)
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

// layoutRail stacks Menu, the wheel, the mover's name, a team game's legend and the chain chip. The rail takes the
// width the board leaves, as far as its height lets the wheel fill it; the wheel is narrower than the rail only
// where a name or the legend needs more width than the height has room for. The legend is what gives way on a short
// rail: it takes the widest rail to wrap onto the fewest lines, and where the wheel would still drop under the
// narrowest rail's width, the wheel keeps that size and the legend scrolls in what is left, a line at least.
func (p *playScene) layoutRail(a *app, v ui.Rect) {
	u := a.u
	pad, menuGap, nameGap, chipGap, listGap := 32*u, 24*u, 20*u, 16*u, 24*u
	if p.compact {
		// 12 px gaps keep the pointer's tip, which reaches 12 px past the spinner, clear of Menu and the name.
		pad, menuGap, nameGap, chipGap, listGap = 16*u, 12*u, 12*u, 8*u, 12*u
	}
	h := v.H - 2*pad
	cw, ch := p.calloutSize(a)
	nameH := ui.ChipHeight * u
	minW := max(menuWidth(a), cw, p.nameWidth(a), p.legendWidth(a))
	maxW := minW
	area := ui.Rect{X: v.X + pad, Y: v.Y + pad, W: v.W - 3*pad - minW, H: h}
	if bw, _ := p.b.Size(area); bw < area.W {
		maxW += area.W - bw
	}
	room := h - (ch + menuGap + nameGap + nameH + chipGap + ch) // what the wheel and the legend share
	lh := p.flowLegend(a, 0, 0, maxW)
	d := min(maxW, room)
	if lh > 0 {
		room -= listGap
		d = min(maxW, max(room-lh, min(minW, room-nameH)))
	}
	d = max(0, d)
	w := max(minW, d)
	if p.flowLegend(a, 0, 0, w) > lh { // the narrowest rail that still wraps the legend onto as few lines
		lo, hi := w, maxW
		for range 20 {
			if mid := (lo + hi) / 2; p.flowLegend(a, 0, 0, mid) > lh {
				lo = mid
			} else {
				hi = mid
			}
		}
		w = hi
	}
	area.W = v.W - 3*pad - w
	x := v.Right() - pad - w
	p.boardView.Set(area, area.H)
	p.menu = ui.Rect{X: x, Y: v.Y + pad, W: w, H: ch}
	p.spinR = d / 2
	p.spinX, p.spinY = x+w/2, p.menu.Bottom()+menuGap+p.spinR
	p.name = ui.Rect{X: x, Y: p.spinY + p.spinR + nameGap, W: w, H: nameH}
	// The legend sits right under the name it belongs with, and the chip under them.
	legend := ui.Rect{X: x, Y: p.name.Bottom() + chipGap, W: w, H: max(0, min(lh, room-d))}
	p.legendView.Set(legend, lh)
	p.callout = ui.Rect{X: x, Y: legend.Y, W: w, H: ch}
	if lh > 0 {
		p.callout.Y = legend.Bottom() + listGap
	}
	p.peek = [3]ui.Rect{{X: p.spinX - p.spinR, Y: p.spinY - p.spinR, W: d, H: d}, p.name, legend}
}

// The mover's name row and the team legend: a dot's radius and the space after it, the space between a team's chip
// and its dots, between teams on a line, and a line's pitch, in px.
const (
	turnDot       = 8
	turnDotGap    = 8
	legendChipGap = 6
	legendGap     = 16
	legendPitch   = 32
)

// nameFace sets the mover's name in the display face: at title size for couch distance, at heading size on a phone.
func (p *playScene) nameFace(a *app) *text.GoTextFace {
	if p.compact {
		return a.f.Heading
	}
	return a.f.Title
}

// nameWidth is the widest the name row gets: the longest label in the display face.
func (p *playScene) nameWidth(a *app) float64 {
	return widestLabel(p.labels, p.nameFace(a))
}

// teamSizes is how many seats each team of a team game has, and one more than the last team with any: 0 in a
// classic game.
func (p *playScene) teamSizes() (size [core.MaxTeams]int, teams int) {
	if p.game.Mix == nil {
		return size, 0
	}
	for i := range p.seats {
		t := int(p.game.Team[i])
		size[t]++
		teams = max(teams, t+1)
	}
	return size, teams
}

// legendGroup is the width of a team of n in the legend: its chip, then a dot for each player.
func legendGroup(a *app, n int) float64 {
	return teamChipW(a) + (legendChipGap+float64(n)*2*turnDot+float64(n-1)*turnDotGap)*a.u
}

// legendWidth is the widest team's group, which a line of the legend must hold.
func (p *playScene) legendWidth(a *app) float64 {
	size, teams := p.teamSizes()
	w := 0.0
	for _, n := range size[:teams] {
		if n > 0 {
			w = max(w, legendGroup(a, n))
		}
	}
	return w
}

// flowLegend sets a team game's legend in the w px from x, down from y: a group for each team with players, in team
// order, wrapping onto centred lines. It returns the legend's height, 0 in a classic game.
func (p *playScene) flowLegend(a *app, x, y, w float64) float64 {
	u := a.u
	size, teams := p.teamSizes()
	p.legend = [core.MaxTeams]ui.Rect{}
	lines, first, lineW := 0, 0, 0.0
	place := func(end int) { // centres the teams from first up to end on the next line
		lx := x + (w-lineW)/2
		for t := first; t < end; t++ {
			if size[t] > 0 {
				p.legend[t] = ui.Rect{X: lx, Y: y + float64(lines)*legendPitch*u, W: legendGroup(a, size[t]), H: ui.ChipHeight * u}
				lx += p.legend[t].W + legendGap*u
			}
		}
		lines, first, lineW = lines+1, end, 0
	}
	for t, n := range size[:teams] {
		if n == 0 {
			continue
		}
		gw := legendGroup(a, n)
		if lineW > 0 && lineW+legendGap*u+gw > w {
			place(t)
		}
		if lineW > 0 {
			lineW += legendGap * u
		}
		lineW += gw
	}
	if lineW == 0 {
		return 0
	}
	place(teams)
	return (float64(lines-1)*legendPitch + ui.ChipHeight) * u
}

// layoutRoster centres the Players dialog: the seat table in one column, or two for more than six seats where they
// fit, scrolling inside the panel on a short screen, above Back.
func (p *playScene) layoutRoster(a *app, v ui.Rect) {
	u := a.u
	in, bh := 28*u, ui.QuietButtonHeight*u
	if p.compact {
		in = 16 * u
	}
	room := v.W - 32*u - 2*in
	p.tableCols = 1
	if len(p.seats) > 6 {
		if p.tableCols = 2; p.seatTableWidth(a) > room {
			p.tableCols = 1
		}
	}
	w := min(room, max(300*u, p.seatTableWidth(a)))
	head, foot := in+40*u+12*u, 16*u+bh+in // the title's band above the table, Back below it
	tableH := min(p.seatTableHeight(a), v.H-32*u-head-foot)
	h := head + tableH + foot
	p.rosterPanel = ui.Rect{X: v.Cx() - w/2 - in, Y: v.Cy() - h/2, W: w + 2*in, H: h}
	p.seatView.Set(ui.Rect{X: p.rosterPanel.X + in, Y: p.rosterPanel.Y + head, W: w, H: tableH}, p.seatTableHeight(a))
	p.rosterBack = ui.Rect{X: p.rosterPanel.X + in, Y: p.rosterPanel.Bottom() - in - bh, W: w, H: bh}
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
	row := 2*in + 2*dot + sp + p.teamRoom(a) + widestLabel(p.labels, name) + gap + max(ui.TextWidth(p.orbsMax, bold), ui.ChipWidth(a.ui(), "Out", p.chipH(a)))
	cols := float64(p.tableCols)
	return cols*row + (cols-1)*8*a.u
}

// teamRoom is the room a team game's chip takes before each seat's type, with the space after it.
func (p *playScene) teamRoom(a *app) float64 {
	if p.game.Mix == nil {
		return 0
	}
	_, sp, _, _ := p.seatInsets(a)
	return teamChipW(a) + sp
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

// layoutDock anchors the portrait board above the mover's name and the fixed bottom controls, a team game's legend
// between board and name. The board fills the width unless screen height constrains it.
func (p *playScene) layoutDock(a *app, v ui.Rect) {
	u := a.u
	pad, gap := 16*u, 16*u
	x, w, top, bottom := v.X+pad, v.W-2*pad, v.Y+pad, v.Bottom()-pad
	_, ch := p.calloutStyle(a)
	side := (w - ch - 16*u) / 2
	p.callout = ui.Rect{X: x, Y: bottom - ch, W: side, H: ch}
	p.tokR, p.tokX, p.tokY = ch/2, v.Cx(), p.callout.Cy()
	menuX := p.tokX + p.tokR + 8*u
	p.menu = ui.Rect{X: menuX, Y: p.callout.Y, W: side, H: ch}
	p.name = ui.Rect{X: x, Y: p.menu.Y - 12*u - ui.ChipHeight*u, W: w, H: ui.ChipHeight * u}
	p.peek = [3]ui.Rect{{X: p.tokX - p.tokR, Y: p.tokY - p.tokR, W: ch, H: ch}, p.name}
	y := p.name.Y
	if lh := p.flowLegend(a, x, 0, w); lh > 0 {
		y -= 12*u + lh
		p.peek[2] = ui.Rect{X: x, Y: y, W: w, H: lh}
	}
	p.legendView.Set(p.peek[2], p.peek[2].H)
	boardW, boardH := p.b.Size(ui.Rect{W: w, H: math.Inf(1)})
	viewH := min(boardH, y-gap-top)
	boardW *= viewH / boardH
	p.boardView.Set(ui.Rect{X: x + (w-boardW)/2, Y: y - gap - viewH, W: boardW, H: viewH}, viewH)
}

func (p *playScene) layoutScrolled(a *app) {
	v := p.boardView.Viewport
	p.b.Layout(ui.Rect{X: v.X, Y: v.Y - p.boardView.Offset, W: v.W, H: p.boardView.Height}, a.u)
	v = p.seatView.Viewport
	p.layoutSeats(a, ui.Rect{X: v.X, Y: v.Y - p.seatView.Offset, W: v.W, H: p.seatView.Height}, p.tableRows())
	v = p.legendView.Viewport
	p.flowLegend(a, v.X, v.Y-p.legendView.Offset, v.W)
}

// revealSeat scrolls the mover's row of the Players dialog and their team in the legend into view.
func (p *playScene) revealSeat(a *app) {
	p.seatView.Reveal(p.rowRect[p.shown])
	p.legendView.Reveal(p.legend[p.game.Team[p.shown]])
	p.listShown = p.shown
	p.layoutScrolled(a)
}

func (p *playScene) updateScroll(a *app) {
	moved := p.boardView.Update(a.ui())
	if p.legendView.Update(a.ui()) || moved {
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

// layoutResults places the result on the winner's ink. On its side the screen splits: the winner's dot, title,
// stats and buttons in a left column, the two boards as cards on the right. Upright, or too narrow for both
// cards, it stacks: the winner, the stats, a tab for each board over one card, and the buttons along the bottom.
// A screen too short tightens the gaps and shrinks the dot; after that the boards' rows scroll.
func (p *playScene) layoutResults(a *app, v ui.Rect) {
	u := a.u
	pad, gap, bh, statSize := 32*u, 12*u, ui.QuietButtonHeight*u, 40*u
	face := a.f.Body
	p.rankIn, p.rankPitch = 20*u, 40*u
	if p.compact {
		pad, statSize, face, p.rankIn, p.rankPitch = 16*u, 28*u, a.f.Label, 16*u, 32*u
		if a.portrait() {
			p.rankPitch = 36 * u
		}
	}
	p.statFace = ui.NewFace(ui.SrcDisplay, statSize, 700)
	p.rankChipW = ui.ChipWidth(a.ui(), "12th", ui.ChipHeight*u)
	// A row, as drawRanks sets it out: chip, 12 px, dot, 8 px, label. The rows stop 12 px short of their card, so the
	// scrollbar stands inside its padding.
	cardW := 2*p.rankIn + p.rankChipW + 12*u + 0.6*p.rankPitch + 8*u + widestLabel(p.labels, face)
	listH := float64(len(p.seats)) * p.rankPitch
	head := p.rankIn + 24*u + 12*u // a card's heading
	btnH := bh
	if p.canAgain() {
		btnH += bh + gap
	}

	col := v.Inset(pad) // where the winner stands
	// Side by side when both cards leave the winner a column, and whenever the screen is too short to stack.
	split := !a.portrait() && (col.W-2*cardW-gap-pad >= 240*u || v.H < 560*u)
	p.tabs, p.cards = [2]ui.Rect{}, [2]ui.Rect{}
	if split {
		cardW = min(cardW, (col.W-gap-pad-200*u)/2)
		cardH := min(head+listH+p.rankIn, col.H)
		x := col.Right() - 2*cardW - gap
		for b := range p.cards {
			p.cards[b] = ui.Rect{X: x + float64(b)*(cardW+gap), Y: v.Cy() - cardH/2, W: cardW, H: cardH}
		}
		p.rankView.Set(ui.Rect{X: x, Y: p.cards[0].Y + head, W: 2*cardW + gap - 12*u, H: cardH - head - p.rankIn}, listH)
		col.W = x - pad - col.X
	} else {
		col.H -= btnH + pad/2 + 8*u
	}

	// Whoever wins, the title fits across its column.
	size := max(56*u, min(120*u, v.W*0.075, v.H*0.13))
	tw, fit := 0.0, ui.NewFace(ui.SrcDisplay, size, 800)
	for _, l := range p.labels {
		tw = max(tw, ui.TextWidth(l+" wins!", fit))
	}
	if tw > col.W {
		size *= col.W / tw
	}
	p.winFace = ui.NewFace(ui.SrcDisplay, size, 800)
	p.winX, p.statDX = col.Cx(), min(80*u, col.W/4)

	// Down the column: dot, title, stats, then the buttons, or the tabs over their card.
	titleH, statH, tabH := size*1.1, statSize+6*u+18*u, ui.CompactButtonTarget*u
	tail := btnH
	if !split {
		tail = tabH + 8*u + 2*p.rankIn + listH
	}
	p.winDotR = max(44*u, min(80*u, v.H*0.08))
	gaps := [3]float64{28 * u, 28 * u, 40 * u}
	rest := func() float64 { return gaps[0] + titleH + gaps[1] + statH + gaps[2] + tail }
	if 2*p.winDotR+rest() > col.H {
		gaps = [3]float64{10 * u, 16 * u, 18 * u}
		p.winDotR = max(24*u, min(p.winDotR, (col.H-rest())/2))
		if over := 2*p.winDotR + rest() - col.H; !split && over > 0 { // the card gives way last, and scrolls
			tail = max(tabH+8*u+2*p.rankIn+p.rankPitch, tail-over)
		}
	}
	y := col.Y + (col.H-2*p.winDotR-rest())/2
	p.winDotY = y + p.winDotR
	y += 2*p.winDotR + gaps[0]
	p.winTitleY = y + size*0.6
	y += titleH + gaps[1]
	p.statsY, p.statLabelY = y+statSize/2, y+statSize+6*u+9*u
	y += statH + gaps[2]

	bw, by := min(280*u, col.W), y
	if !split {
		bw, by = min(col.W, 420*u), v.Bottom()-pad-btnH
		x, half := p.winX-bw/2, (bw-8*u)/2
		for b := range p.tabs {
			p.tabs[b] = ui.Rect{X: x + float64(b)*(half+8*u), Y: y + (tabH-ui.CompactButtonHeight*u)/2, W: half, H: ui.CompactButtonHeight * u}
		}
		p.cards[0] = ui.Rect{X: x, Y: y + tabH + 8*u, W: bw, H: tail - tabH - 8*u}
		p.rankView.Set(ui.Rect{X: x, Y: p.cards[0].Y + p.rankIn, W: bw - 12*u, H: p.cards[0].H - 2*p.rankIn}, listH)
	}
	// Play again across the top where it is offered; Main menu and Exit share the row under it.
	p.again, p.toHome, p.exit = ui.Rect{}, ui.Rect{X: p.winX - bw/2, Y: by, W: bw, H: bh}, ui.Rect{}
	if p.canAgain() {
		p.again = p.toHome
		p.toHome.Y += bh + gap
	}
	if platform.CanExit {
		p.toHome.W = (bw - gap) / 2
		p.exit = p.toHome
		p.exit.X += p.toHome.W + gap
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
	case p.watch != nil:
		p.watch.draw(dst, a)
	case p.roster:
		p.drawRoster(dst, a)
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
	sc, u := a.sc, a.u
	p.drawMenuButton(dst, a)
	if p.spinR > 0 {
		p.drawSpinner(dst, a)
	}

	// The mover's name alone, called out in the display face: the wheel's pointer or the turn dot shows their colour,
	// and the legend's ring their team.
	ui.DrawText(dst, p.labels[p.shown], p.nameFace(a), p.name.Cx(), p.name.Cy(), text.AlignCenter, sc.Ink)

	// The chain callout, in the mover's ink.
	if p.chainHold > 0 && p.chain >= 2 {
		c := sc.Players[p.chainOwner]
		alpha := min(1, float64(p.chainHold)/20)
		ui.FillPill(dst, p.callout, ui.WithAlpha(c, alpha))
		face, _ := p.calloutStyle(a)
		ui.DrawText(dst, p.chainText, face, p.callout.Cx(), p.callout.Cy(), text.AlignCenter, ui.WithAlpha(ui.InkOn(c), alpha))
	}

	// A team game's legend: each team's chip, then its players' dots. Out players are empty rings, a team with
	// nobody left dims, and a ring marks the mover. A legend taller than a short rail has room for scrolls, clipped.
	ldst := dst
	if p.legendView.MaxScroll() > 0 {
		ldst = p.legendView.Clip(dst)
	}
	for t, r := range p.legend {
		if r.W == 0 {
			continue
		}
		ink, dx := sc.Dim, r.X+teamChipW(a)+(legendChipGap+turnDot)*u
		for i := range p.seats {
			if int(p.game.Team[i]) != t {
				continue
			}
			if p.alive[i] {
				ink = sc.Ink
			}
			ui.SeatDot(ldst, sc, i, dx, r.Cy(), turnDot*u, !p.alive[i], a.set.Marks)
			if i == p.shown && !p.over {
				ui.Ring(ldst, dx, r.Cy(), (turnDot+4)*u, 2*u, sc.Ink)
			}
			dx += (2*turnDot + turnDotGap) * u
		}
		drawTeamChip(ldst, a, ui.Rect{X: r.X, Y: r.Y, W: teamChipW(a), H: r.H}, uint8(t), ink, sc.Ground, false)
	}
	p.legendView.DrawScrollbar(dst, a.ui())

	// Upright, the centered turn dot also shows the mover's orb count.
	if p.tokR > 0 {
		p.drawTurnCount(dst, a)
	}
}

// drawRoster dims the game and shows the Players dialog: the seat rows, set out as seatTableWidth measures them.
func (p *playScene) drawRoster(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	ui.FillRound(dst, ui.Rect{X: 0, Y: 0, W: a.w, H: a.h}, 0, ui.WithAlpha(sc.Ground, 0.8))
	ui.FillRound(dst, p.rosterPanel, 28*u, sc.Tile)
	ui.DrawText(dst, "Players", a.f.Title, p.rosterPanel.Cx(), p.seatView.Viewport.Y-32*u, text.AlignCenter, sc.Ink)
	name, bold := p.seatFaces(a)
	in, sp, gap, dot := p.seatInsets(a)
	seatsDst := p.seatView.Clip(dst)
	for i, label := range p.labels {
		r := p.rowRect[i]
		alive, current := p.alive[i], i == p.shown && !p.over
		if current {
			ui.FillPill(seatsDst, r, sc.Ground)
		}
		dr := min(dot, r.H*0.34)
		ui.SeatDot(seatsDst, sc, i, r.X+in+dr, r.Cy(), dr, !alive, a.set.Marks)
		ink := sc.Ink
		if !alive {
			ink = sc.Dim
		}
		lx := r.X + in + 2*dr + sp
		if team := p.teamRoom(a); team > 0 {
			h := p.chipH(a)
			drawTeamChip(seatsDst, a, ui.Rect{X: lx, Y: r.Cy() - h/2, W: team - sp, H: h}, p.game.Team[i], ink, sc.Ground, false)
			lx += team
		}
		ui.DrawText(seatsDst, label, name, lx, r.Cy(), text.AlignStart, ink)
		if alive {
			room := r.Right() - in - lx - ui.TextWidth(label, name) - gap
			scale := min(1, room/max(1, ui.TextWidth(p.orbS[i], bold)))
			ui.DrawScaledText(seatsDst, p.orbS[i], bold, r.Right()-in, r.Cy(), text.AlignEnd, scale, ink)
		} else {
			ui.DrawChip(seatsDst, a.ui(), ui.Chip{Word: "Out", Kind: ui.ChipDanger}, r.Right()-in, r.Cy(), p.chipH(a))
		}
	}
	p.seatView.DrawScrollbar(dst, a.ui())
	ui.DrawButton(dst, a.ui(), p.rosterBack, "Back", "", ui.ButtonGround, true)
}

// spinnerKey is what the cached spinner images show.
type spinnerKey struct {
	x, y, r, u float64
	share      [core.MaxPlayers]float64
	sc         *ui.Palette
}

// drawSpinner draws the turn spinner: a pie with a slice per player still in, in their ink and as wide as slice
// shares it out, and the pointer on the player to move. Both are images redrawn only when what they show changes;
// the pointer turns as it is drawn.
// shortcut: slices step to their new size with each wave instead of easing, so the wheel is redrawn once a wave and
// not every frame; ease p.share and draw the pie from sprites if that reads as a jump.
func (p *playScene) drawSpinner(dst *ebiten.Image, a *app) {
	var op ebiten.DrawImageOptions
	if k := (spinnerKey{p.spinX, p.spinY, p.spinR, a.u, p.share, a.sc}); p.wheel == nil || k != p.spinKey {
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

// canvas returns img cleared when it is w by h px, and a new image of that size in its place otherwise.
func canvas(img *ebiten.Image, w, h int) *ebiten.Image {
	if img != nil && img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		img.Clear()
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return ebiten.NewImage(w, h)
}

// renderSpinner caches the wheel at whole-pixel alignment and the rim pointer pointing right around its hub.
func (p *playScene) renderSpinner(a *app) {
	sc, u, n, r := a.sc, a.u, len(p.seats), p.spinR
	p.wheelX, p.wheelY = math.Floor(p.spinX-r)-ui.SpritePad, math.Floor(p.spinY-r)-ui.SpritePad
	x, y := p.spinX-p.wheelX, p.spinY-p.wheelY
	p.wheel = canvas(p.wheel, int(math.Ceil(x+r))+ui.SpritePad, int(math.Ceil(y+r))+ui.SpritePad)
	for i := range n {
		switch a0 := p.wedgeStart(i); {
		case p.share[i] > 0.999: // the only player left has the whole wheel
			ui.FillCircle(p.wheel, x, y, r, sc.Players[i])
		case p.share[i] > 0:
			spinnerPath.Reset()
			spinnerPath.MoveTo(float32(x), float32(y))
			spinnerPath.Arc(float32(x), float32(y), float32(r), float32(a0), float32(a0+2*math.Pi*p.share[i]), 0)
			spinnerPath.Close()
			ui.FillPath(p.wheel, &spinnerPath, sc.Players[i])
		}
	}
	for i := range n {
		if an := p.wedgeStart(i); p.share[i] > 0 && p.share[i] <= 0.999 {
			ui.Line(p.wheel, x, y, x+r*math.Cos(an), y+r*math.Sin(an), 3*u, sc.Ground)
		}
	}
	ui.FillCircle(p.wheel, x, y, r*0.2, sc.Ground)

	rim := 2 * u
	tip, w, h := r+12*u-rim, r*0.09, math.Ceil(r*0.1+rim)+ui.SpritePad
	p.pointer = canvas(p.pointer, int(math.Ceil(h+tip))+ui.SpritePad, int(2*h))
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
	if p.ask != nil {
		p.ask.draw(dst, a)
		return
	}
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
		style := ui.ButtonGround
		if i == pauseResume {
			style = ui.ButtonInk
		}
		ui.DrawButton(dst, a.ui(), r, pauseItems[i], "", style, true)
	}
}

// The results' stats and boards, by name.
var (
	statNames = [2]string{"Moves", "Biggest chain"}
	rankNames = [2]string{"Survival", "Knockouts"}
)

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
	// The stats: each number over its name, side by side around the column's centre.
	n := 1
	if p.stats[1] != "" {
		n = 2
	}
	for i := range n {
		sx := x + float64(2*i-n+1)*p.statDX
		ui.DrawText(dst, p.stats[i], p.statFace, sx, p.statsY, text.AlignCenter, ui.WithAlpha(ink, alpha))
		ui.DrawText(dst, statNames[i], f.Label, sx, p.statLabelY, text.AlignCenter, ui.WithAlpha(ink, alpha))
	}

	// The boards: mat-coloured cards, so the medals read on any winner's ink. The chosen tab is filled like its card.
	tabbed := p.tabs[0].W > 0
	for b, r := range p.tabs {
		switch {
		case !tabbed:
		case b == p.tab:
			ui.FillPill(dst, r, ui.WithAlpha(sc.Ground, alpha))
			ui.PillLabel(dst, a.ui(), r, rankNames[b], "", ui.WithAlpha(sc.Ink, alpha))
		default:
			resultButton(dst, a, r, rankNames[b], w, ink, false, alpha)
		}
	}
	rows := p.rankView.Clip(dst)
	for b, c := range p.cards {
		if c.W == 0 {
			break
		}
		ui.FillRound(dst, c, 24*u, ui.WithAlpha(sc.Ground, alpha))
		if tabbed {
			b = p.tab
		} else {
			ui.DrawText(dst, rankNames[b], f.Heading, c.X+p.rankIn, c.Y+p.rankIn+12*u, text.AlignStart, ui.WithAlpha(sc.Ink, alpha))
		}
		p.drawRanks(rows, a, c, b, alpha)
	}
	p.rankView.DrawScrollbar(dst, a.ui())

	if p.canAgain() {
		resultButton(dst, a, p.again, "Play again", w, ink, true, alpha)
	}
	resultButton(dst, a, p.toHome, "Main menu", w, ink, false, alpha)
	if platform.CanExit {
		resultButton(dst, a, p.exit, "Exit", w, ink, false, alpha)
	}
}

// drawRanks draws board b's rows on card: each seat's place or knockouts as a chip, a medal for the first three
// and a dim outline for the rest, then its dot and label.
func (p *playScene) drawRanks(dst *ebiten.Image, a *app, card ui.Rect, b int, alpha float64) {
	sc, u, v := a.sc, a.u, p.rankView.Viewport
	face := a.f.Body
	if p.compact {
		face = a.f.Label
	}
	ch, dr := ui.ChipHeight*u, 0.3*p.rankPitch
	for k, i := range p.order[b][:len(p.seats)] {
		cy := v.Y - p.rankView.Offset + (float64(k)+0.5)*p.rankPitch
		if cy+p.rankPitch < v.Y || cy-p.rankPitch > v.Bottom() {
			continue
		}
		chip := ui.Rect{X: card.X + p.rankIn, Y: cy - ch/2, W: p.rankChipW, H: ch}
		wordInk := sc.Dim
		if m := p.medal[b][i]; m < len(ui.Medals) {
			ui.FillPill(dst, chip, ui.WithAlpha(ui.Medals[m], alpha))
			wordInk = ui.InkOn(ui.Medals[m])
		} else {
			ui.StrokeRound(dst, chip, ch/2, 1.5*u, ui.WithAlpha(sc.Dim, alpha))
		}
		ui.DrawText(dst, p.word[b][i], a.f.SmallBold, chip.Cx(), cy, text.AlignCenter, ui.WithAlpha(wordInk, alpha))
		x := chip.Right() + 12*u + dr
		ui.FillCircle(dst, x, cy, dr, ui.WithAlpha(sc.Players[i], alpha))
		if alpha == 1 && a.set.Marks {
			ui.DrawMark(dst, i, x, cy, dr, sc.Players[i])
		}
		x += dr + 8*u
		scale := min(1, (card.Right()-p.rankIn-x)/max(1, ui.TextWidth(p.labels[i], face)))
		ui.DrawScaledText(dst, p.labels[i], face, x, cy, text.AlignStart, scale, ui.WithAlpha(sc.Ink, alpha))
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
