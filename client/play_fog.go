package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/client/internal/ui"
)

// look picks whose sight a fog game's board shows: the human here whose turn it is, and the last one through other
// players' turns. Everything shows once the game is over, or once no human here has their side still in it. A turn
// passing to a human here on another side covers the board until they tap.
func (p *playScene) look() {
	g, b := p.game, p.b
	if !g.Fog {
		return
	}
	first := -1 // a human here whose side is still in
	for i := range p.seats {
		if p.here(i) && p.sideIn(i) {
			first = i
			break
		}
	}
	if first < 0 || g.Over() {
		b.Viewer, p.cover = -1, false
		return
	}
	switch t := g.Turn; {
	case p.here(t):
		if b.Viewer >= 0 && g.Side(int8(t)) != g.Side(b.Viewer) {
			p.cover, p.coverText = true, "Pass to "+p.labels[t]
		}
		b.Viewer = int8(t)
	case b.Viewer < 0 || !p.here(int(b.Viewer)):
		b.Viewer = int8(first)
	}
}

// here reports whether seat i is played at this screen: a human seat of a local game, this player's own online.
func (p *playScene) here(i int) bool {
	return p.seats[i] == seatHuman && (p.net == nil || i == p.net.seat)
}

// sideIn reports whether seat i's side, their team in a team game, still has a player in the game.
func (p *playScene) sideIn(i int) bool {
	g := p.game
	for j := range g.Players {
		if g.Side(int8(j)) == g.Side(int8(i)) && g.Alive(j) {
			return true
		}
	}
	return false
}

// drawCover hides the board from the human who just played, naming the one whose turn it is.
func (p *playScene) drawCover(dst *ebiten.Image, a *app) {
	r := p.boardView.Viewport.Inset(-4 * a.u) // over the outer tiles' edges too
	ui.FillRound(dst, r, 0, a.sc.Ground)
	face := a.f.Heading
	scale := min(1, r.W/max(1, ui.TextWidth(p.coverText, face)))
	ui.DrawScaledText(dst, p.coverText, face, r.Cx(), r.Cy(), text.AlignCenter, scale, a.sc.Ink)
}

// orbText is seat i's orb count as this screen shows it: in the fog a rival's visible orbs, with "~" before it while some are hidden
// (tickRail), and "?" for everyone while the board is covered for the next human.
func (p *playScene) orbText(i int) string {
	switch {
	case p.cover:
		return "?"
	case p.orbS[i] == "":
		return "0"
	}
	return p.orbS[i]
}
