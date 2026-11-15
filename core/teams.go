package core

import "fmt"

// MaxTeams is the most teams a team game holds.
const MaxTeams = 6

// Cells is the orbs on a board, shared by Game and the client's replay of it. Change them only through its methods,
// or the per-player tallies stop matching the board.
//
// In a team game teammates' orbs share a cell without capturing each other: Mix counts each colour there, Count is
// their sum and Owner the colour holding the most (the earlier holder on a tie), which is the one map devices work
// for. A cell never holds orbs of two teams.
type Cells struct {
	Count []uint8           // orbs in each cell
	Owner []int8            // owner of each cell, NoOwner when empty
	Orbs  [MaxPlayers]int32 // orbs on the board per player
	Team  [MaxPlayers]uint8 // each player's team, in a team game
	Mix   []uint8           // orbs of each player in each cell, stride per cell; nil outside team games

	stride  int
	in      []uint8 // orbs sent to each cell, as Mix, until Land
	queued  []uint8 // the cell is in touched
	touched []int16
}

// SetTeams makes g a team game before its first move: team holds each player's team, below MaxTeams, and at least
// two teams must have a player.
func (g *Game) SetTeams(team []uint8) error {
	if g.Moves != 0 || g.Played != 0 {
		return fmt.Errorf("teams: the game has started")
	}
	if len(team) != g.Players {
		return fmt.Errorf("teams: %d teams for %d players", len(team), g.Players)
	}
	rivals := false
	for _, t := range team {
		if t >= MaxTeams {
			return fmt.Errorf("teams: team %d, must be under %d", t, MaxTeams)
		}
		rivals = rivals || t != team[0]
	}
	if !rivals {
		return fmt.Errorf("teams: every player is on one team")
	}
	copy(g.Team[:], team)
	g.stride = g.Players
	g.Mix = make([]uint8, len(g.Count)*g.stride)
	g.in, g.queued = make([]uint8, len(g.Mix)), make([]uint8, len(g.Count))
	return nil
}

// CopyFrom makes c a copy of src, reusing c's buffers when they fit.
func (c *Cells) CopyFrom(src *Cells) {
	c.Count = append(c.Count[:0], src.Count...)
	c.Owner = append(c.Owner[:0], src.Owner...)
	c.Orbs, c.Team, c.stride = src.Orbs, src.Team, src.stride
	if src.Mix == nil {
		c.Mix = nil
		return
	}
	c.Mix = append(c.Mix[:0], src.Mix...)
	if len(c.in) != len(c.Mix) {
		c.in, c.queued = make([]uint8, len(c.Mix)), make([]uint8, len(c.Count))
	}
}

// Allied reports whether p and q are the same player, or teammates in a team game.
func (c *Cells) Allied(p, q int8) bool {
	return p == q || c.Mix != nil && p >= 0 && q >= 0 && c.Team[p] == c.Team[q]
}

// Has reports whether player p has orbs in cell i.
func (c *Cells) Has(i int, p int8) bool {
	if c.Mix != nil {
		return c.Mix[i*c.stride+int(p)] != 0
	}
	return c.Owner[i] == p
}

// Add gives cell i n more of p's orbs, which must be free to hold them: empty, p's or a teammate's.
func (c *Cells) Add(i int, p int8, n uint8) {
	c.Count[i] += n
	c.Orbs[p] += int32(n)
	if c.Mix == nil {
		c.Owner[i] = p
		return
	}
	c.Mix[i*c.stride+int(p)] += n
	c.lead(i)
}

// Remove takes n of p's orbs out of cell i.
func (c *Cells) Remove(i int, p int8, n uint8) {
	c.Count[i] -= n
	c.Orbs[p] -= int32(n)
	if c.Mix != nil {
		c.Mix[i*c.stride+int(p)] -= n
		c.lead(i)
	} else if c.Count[i] == 0 {
		c.Owner[i] = NoOwner
	}
}

// Lift empties cell i and returns how many orbs it held, adding each player's to stack unless it is nil.
func (c *Cells) Lift(i int, stack *[MaxPlayers]uint8) uint8 {
	n := c.Count[i]
	if n == 0 {
		return 0
	}
	if c.Mix == nil {
		c.Orbs[c.Owner[i]] -= int32(n)
		if stack != nil {
			stack[c.Owner[i]] += n
		}
	} else {
		for q, m := range c.Mix[i*c.stride : (i+1)*c.stride] {
			c.Orbs[q] -= int32(m)
			if stack != nil {
				stack[q] += m
			}
		}
		clear(c.Mix[i*c.stride : (i+1)*c.stride])
	}
	c.Count[i], c.Owner[i] = 0, NoOwner
	return n
}

// lead makes the colour with the most orbs in cell i its owner: the one it has while that still holds the most, else
// the lowest seat.
func (c *Cells) lead(i int) {
	if c.Count[i] == 0 {
		c.Owner[i] = NoOwner
		return
	}
	mix, best := c.Mix[i*c.stride:(i+1)*c.stride], c.Owner[i]
	for q, n := range mix {
		if best == NoOwner || n > mix[best] {
			best = int8(q)
		}
	}
	c.Owner[i] = best
}

// Pop takes the orbs a team game's cell e gives up as it explodes on geo, and returns each one's colour by the target
// slot it flies to; slot MaxConnections is a Reservoir's extra orb. The colour with the fewest orbs there goes first
// (the lower seat on a tie) and the one with the most last, to the targets clockwise from up, then a Link, a
// Spillway's pit and the extra orb. What is left stays with the colours not yet reached.
func (c *Cells) Pop(geo *Geometry, e int) (deal [MaxConnections + 1]int8) {
	var buf [MaxConnections + 1]uint8
	order := geo.Clockwise(e, buf[:0])
	if int(geo.Released(e)) > len(order) {
		order = append(order, MaxConnections)
	}
	mix := c.Mix[e*c.stride : (e+1)*c.stride]
	var dealt uint16
	for k := 0; k < len(order); {
		q := -1
		for p, m := range mix {
			if m > 0 && dealt&(1<<p) == 0 && (q < 0 || m < mix[q]) {
				q = p
			}
		}
		if q < 0 {
			break
		}
		dealt |= 1 << q
		for ; k < len(order) && mix[q] > 0; k++ {
			deal[order[k]] = int8(q)
			mix[q]--
			c.Orbs[q]--
			c.Count[e]--
		}
	}
	c.lead(e)
	return deal
}

// Send queues n of p's orbs for a team game's cell i until Land, so that every orb of a wave arrives at once.
func (c *Cells) Send(i int, p int8, n uint8) {
	if c.queued[i] == 0 {
		c.queued[i] = 1
		c.touched = append(c.touched, int16(i))
	}
	c.in[i*c.stride+int(p)] += n
}

// Land gives every cell the orbs sent to it. Teammates' orbs join the ones there; a rival team's orbs there are
// captured first, shared evenly between the arriving colours, the odd ones going to the lower seats. With limit, a
// Geometry's Cap, it appends to next the cells that reached their capacity, and returns next.
func (c *Cells) Land(limit []uint8, next []int16) []int16 {
	s := c.stride
	for _, t := range c.touched {
		i := int(t)
		in, mix, before := c.in[i*s:(i+1)*s], c.Mix[i*s:(i+1)*s], c.Count[i]
		// One team's orbs at a time: several arrive together only when a wave explodes cells of more than one team.
		for {
			team, colours := -1, 0
			for q, n := range in {
				if n > 0 && team < 0 {
					team = int(c.Team[q])
				}
				if n > 0 && int(c.Team[q]) == team {
					colours++
				}
			}
			if team < 0 {
				break
			}
			if o := c.Owner[i]; o != NoOwner && int(c.Team[o]) != team {
				taken := c.Count[i]
				for q, n := range mix {
					c.Orbs[q] -= int32(n)
				}
				clear(mix)
				c.Owner[i] = NoOwner
				share, odd := taken/uint8(colours), taken%uint8(colours)
				for q, n := range in {
					if n > 0 && int(c.Team[q]) == team {
						got := share
						if odd > 0 {
							got++
							odd--
						}
						mix[q] += got
						c.Orbs[q] += int32(got)
					}
				}
			}
			for q, n := range in {
				if n > 0 && int(c.Team[q]) == team {
					mix[q] += n
					c.Orbs[q] += int32(n)
					c.Count[i] += n
					in[q] = 0
				}
			}
			c.lead(i)
		}
		c.queued[i] = 0
		if limit != nil && before < limit[i] && c.Count[i] >= limit[i] {
			next = append(next, t)
		}
	}
	c.touched = c.touched[:0]
	return next
}

// dealWidth is the colours explodeTeams keeps for each cell of a wave: see Cells.Pop.
const dealWidth = MaxConnections + 1

// explodeTeams is explode for a team game: each orb keeps its own colour, and the wave's orbs land together.
func (g *Game) explodeTeams() {
	geo := g.Geo
	if g.Map != nil {
		clear(g.arrived)
	}
	g.next, g.from, g.deal = g.next[:0], g.from[:0], g.deal[:0]
	for k, e := range g.cur {
		p := g.Owner[e]
		g.from = append(g.from, p)
		if k < g.shots {
			var none [dealWidth]int8 // a Turret's shot is its holder's
			g.deal = append(g.deal, none[:]...)
			continue
		}
		deal := g.Pop(geo, int(e))
		g.deal = append(g.deal, deal[:]...)
		if geo.Outlets != nil && geo.Outlets[e] != 0 {
			var makes bool
			if makes, g.Pressed[e] = ReservoirMakes(g.Pressed[e]); makes {
				g.Add(int(e), p, 1)
			}
		}
		if g.Count[e] >= geo.Cap[e] {
			g.next = append(g.next, e)
		}
	}
	for k, e := range g.cur {
		to, deal := geo.Neighbours(int(e)), g.deal[k*dealWidth:]
		if k < g.shots {
			var slot int
			slot, g.Durability[e] = geo.Aim(int(e), g.Durability[e])
			g.send(int(to[slot]), g.from[k])
			continue
		}
		for slot, n := range to {
			g.send(int(n), deal[slot])
		}
		if geo.Outlets != nil && geo.Outlets[e] != 0 {
			var slot int
			slot, g.Pressed[e] = geo.Outlet(int(e), g.Pressed[e])
			g.send(int(to[slot]), deal[MaxConnections])
		}
	}
	g.next = g.Land(geo.Cap, g.next)
}

// send flies one of p's orbs to cell n in a team game, unless a Vault's shield there takes it.
func (g *Game) send(n int, p int8) {
	if g.Map != nil {
		var hit bool
		if hit, g.Pressed[n] = g.shielded(n, p); hit {
			return
		}
		g.arrived[n] = 1
	}
	g.Send(n, p, 1)
}

// shielded is Shielded for cell i, whose shield lets its holder's teammates in.
func (g *Game) shielded(i int, p int8) (bool, uint8) {
	if state := g.Pressed[i]; g.Mix != nil && g.Allied(Holder(state), p) {
		return false, state
	}
	return Shielded(g.Pressed[i], p)
}

// sender is the colour of the orb the k-th cell of the wave sent out of slot, MaxConnections for a Reservoir's extra
// orb.
func (g *Game) sender(k, slot int) int8 {
	if g.Mix == nil || k < g.shots {
		return g.from[k]
	}
	return g.deal[k*dealWidth+slot]
}

// checkTeamWin is checkWin for a team game: one team holds every orb. Winner is the mover when theirs is that team,
// else its lowest seat with orbs.
func (g *Game) checkTeamWin() bool {
	winner := NoOwner
	for p := 0; p < g.Players; p++ {
		if g.Orbs[p] > 0 {
			if winner != NoOwner && g.Team[p] != g.Team[winner] {
				return false
			}
			if winner == NoOwner {
				winner = int8(p)
			}
		}
	}
	if winner != NoOwner && g.Team[g.Turn] == g.Team[winner] {
		winner = int8(g.Turn)
	}
	g.Winner = winner
	return winner != NoOwner
}

// Won reports whether the game is over and p won it, alone or with their team.
func (g *Game) Won(p int8) bool { return g.Over() && g.Allied(g.Winner, p) }

// CheckTeams reports why team, each player's team, can't be the line-up of a game of teams teams with at most
// capacity players each. A line-up that fits may still put every player on one team, which SetTeams refuses.
func CheckTeams(team []uint8, teams, capacity int) error {
	if teams < 2 || teams > MaxTeams || capacity < 1 {
		return fmt.Errorf("teams: %d teams of %d", teams, capacity)
	}
	var size [MaxTeams]int
	for p, t := range team {
		if int(t) >= teams {
			return fmt.Errorf("teams: player %d on team %d of %d", p, t, teams)
		}
		if size[t]++; size[t] > capacity {
			return fmt.Errorf("teams: team %d holds more than %d players", t, capacity)
		}
	}
	return nil
}
