package client

import (
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"orbitrix/core"
	"orbitrix/proto"
)

// newNetPlay shows the room's game from the snapshot the host sent, or returns nil when it doesn't restore (the host
// is asked for another).
func newNetPlay(a *app, n *session) *playScene {
	g, err := core.Restore(n.snap.Game)
	seq := n.snap.Seq
	n.snap = nil
	if err != nil || len(n.lobby.Seats) != g.Players {
		log.Printf("online: bad snapshot: %v", err)
		n.link.Send(proto.Resync{})
		return nil
	}
	p := newPlayOn(a, g, n.seats(), n.labels(), pauseOnline)
	p.net, p.seq, p.lobbyN = n, seq, n.lobbyN
	return p
}

// readSeats takes the seats' types and names from the latest room update: a player who left may have become a bot.
func (p *playScene) readSeats() {
	n := p.net
	p.lobbyN = n.lobbyN
	p.seats, p.labels = n.seats(), n.labels()
	p.lv = rect{} // the seat table measures its labels
}

// pumpNet takes in what the host sent and reports whether the scene changes.
func (p *playScene) pumpNet(a *app) bool {
	n := p.net
	n.pump(a)
	if n.route(a, p) {
		return true
	}
	if n.lobbyN != p.lobbyN {
		p.readSeats()
	}
	if n.snap != nil { // the answer to a resync, or a rejoin's catch-up
		g, err := core.Restore(n.snap.Game)
		if err == nil && g.Players == len(p.seats) {
			p.game, p.b, p.seq = g, newBoard(g), n.snap.Seq
			p.resync, p.sent, p.pace, p.lv = false, false, -1, rect{}
			p.settle()
		}
		n.snap = nil
	}
	p.askGone()
	return false
}

// playNet plays the next move from the host once the board has finished the last one. An AI's move waits a moment
// first, as in a local game. Any move that doesn't fit this board, or leaves it with a different hash from the
// host's, asks for the whole game again.
func (p *playScene) playNet(a *app) {
	n := p.net
	if p.b.playing || p.resync {
		return
	}
	for len(n.moves) > 0 && n.moves[0].Seq <= p.seq {
		n.moves = n.moves[1:]
	}
	if len(n.moves) == 0 || p.game.Over() {
		return
	}
	m := n.moves[0]
	if int(m.Player) < len(p.seats) && p.seats[m.Player] != seatHuman && m.C != proto.Pass {
		if p.pace < 0 {
			p.pace = aiDelayMin + a.rng.Intn(aiDelayMax-aiDelayMin+1)
		}
		if p.pace > 0 {
			p.pace--
			return
		}
	}
	p.pace = -1
	n.moves = n.moves[1:]
	g := p.game
	ok := m.Seq == p.seq+1 && int(m.Player) == g.Turn
	switch {
	case !ok:
	case m.C == proto.Pass:
		ok = g.Pass() == nil
		p.settle()
	case int(m.C) < g.Geo.Cols && int(m.R) < g.Geo.Rows && g.Legal(g.Geo.Index(int(m.C), int(m.R))):
		p.move(a, g.Geo.Index(int(m.C), int(m.R)))
	default:
		ok = false
	}
	p.seq = m.Seq
	if int(m.Player) == n.seat {
		p.sent = false
	}
	if !ok || g.Hash() != m.Hash {
		log.Printf("online: move %d doesn't match the host's game; asking for it again", m.Seq)
		p.resync = true
		n.moves = n.moves[:0]
		n.link.Send(proto.Resync{})
	}
}

// myTurn reports whether this player may move now: their turn, shown on the board, with nothing still to come in.
func (p *playScene) myTurn() bool {
	n := p.net
	return p.game.Turn == n.seat && !p.sent && !p.resync && len(n.moves) == 0
}

// askGone finds a seat whose player left the game, so the host can hand it to a bot or skip it.
func (p *playScene) askGone() {
	n := p.net
	p.goneP = -1
	if n.room == nil || p.over {
		return
	}
	for i, st := range n.lobby.Seats {
		if st.State == proto.SeatGone {
			p.goneP = i
			return
		}
	}
}

// updateGone runs the host's Bot or Skip question for a seat whose player left.
func (p *playScene) updateGone(a *app) {
	r, i := p.net.room, p.goneP
	var err error
	switch {
	case a.clicked(p.goneB[0]):
		err = r.Replace(i, defaultAI(i))
	case a.clicked(p.goneB[1]):
		err = r.Skip(i)
	default:
		return
	}
	if err != nil {
		log.Printf("seat %d: %v", i, err)
	}
	p.goneP = -1 // until the room says otherwise
}

// layoutGone centres the host's question about a seat whose player left, in the pause menu's dialog style.
func (p *playScene) layoutGone(a *app, v rect) {
	u := a.u
	bw, bh, gap := min(300*u, v.w-88*u), quietButtonHeight*u, 12*u
	h := 28*u + 40*u + 24*u + 2*bh + gap + 28*u
	p.goneD = rect{v.cx() - bw/2 - 28*u, v.cy() - h/2, bw + 56*u, h}
	for i := range p.goneB {
		p.goneB[i] = rect{p.goneD.x + 28*u, p.goneD.y + 92*u + float64(i)*(bh+gap), bw, bh}
	}
}

// drawGone asks the host what becomes of a seat whose player left: a bot plays it, or its turns are skipped.
func (p *playScene) drawGone(dst *ebiten.Image, a *app) {
	sc, u := a.sc, a.u
	fillRound(dst, rect{0, 0, a.w, a.h}, 0, withAlpha(sc.ground, 0.8))
	fillRound(dst, p.goneD, 28*u, sc.tile)
	if p.goneN != p.goneP {
		p.goneN, p.goneText = p.goneP, strings.TrimSuffix(p.labels[p.goneP], " (you)")+" left"
	}
	drawText(dst, p.goneText, a.f.title, p.goneD.cx(), p.goneD.y+48*u, text.AlignCenter, sc.ink)
	drawButton(dst, a, p.goneB[0], "Hand to a bot", "", btnInk, true)
	drawButton(dst, a, p.goneB[1], "Skip their turns", "", btnGround, true)
}
