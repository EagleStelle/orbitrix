package ai

import (
	"math"
	"slices"
	"testing"

	"orbitrix/core"
)

// TestSovereignPatterns: occupied cells' patterns fill [0, sovOwned) and empty cells' [sovOwned, sovPatterns), each
// range exactly once over every fill and neighbourhood.
func TestSovereignPatterns(t *testing.T) {
	var size [5]int
	for code := range sovCodes {
		n := 0
		for c := code; c > 0; c /= 5 {
			n += c % 5
		}
		if n >= 2 && n <= 4 {
			size[n]++
		}
	}
	seen := make([]bool, sovPatterns)
	for k := 2; k <= 4; k++ {
		for fill := range 4 {
			base := int(sovFills[k][fill])
			if base < 0 {
				continue
			}
			if (fill == 3) != (base >= sovOwned) {
				t.Errorf("capacity %d fill %d starts at %d, on the wrong side of %d", k, fill, base, sovOwned)
			}
			for i := range size[k] {
				if seen[base+i] {
					t.Fatalf("pattern %d used twice", base+i)
				}
				seen[base+i] = true
			}
		}
	}
	for i, ok := range seen {
		if !ok {
			t.Errorf("pattern %d unused", i)
		}
	}
}

// TestSovereignUpdateMatchesRefresh: accumulators, hashes and cached logits and patterns updated move by move, in and
// out of turn, through chains and eliminations, equal ones computed from scratch, and undoing every move restores the
// root's cache.
func TestSovereignUpdateMatchesRefresh(t *testing.T) {
	rng := core.NewRNG(7)
	w := make([]int16, sovParams)
	for i := range w {
		w[i] = int16(rng.Intn(2001) - 1000)
	}
	for _, size := range []int{3, 6, 9, 20} {
		for _, players := range []int{2, 5} {
			g, _ := core.NewGame(size, size, players)
			var legal []int
			for !g.Over() && g.Moves < size*size*3/2 {
				legal = g.LegalMoves(legal[:0])
				_ = g.Simulate(legal[rng.Intn(len(legal))])
			}
			if g.Over() {
				continue
			}
			b := Bot{g: g, me: int8(g.Turn), left: 1 << 30}
			s := sovSearch{b: &b, s: &b.sov, net: sovNet{sovCut(w)}}
			s.begin()
			root := slices.Clone(b.sov.cache)
			parent, ply := g, 1
			for ; ply < maxDepth && !parent.Over() && parent.Alive(int(b.me)); ply++ {
				p := parent.Turn
				if rng.Intn(3) == 0 { // an opponent out of turn, as in Best-Reply Search
					p = (int(b.me) + 1 + rng.Intn(players-1)) % players
				}
				var moves []int
				for i, o := range parent.Owner {
					if o == core.NoOwner || int(o) == p {
						moves = append(moves, i)
					}
				}
				child := &b.nodes[ply-1]
				s.play(child, parent, moves[rng.Intn(len(moves))], p, ply)
				var want sovAcc
				s.net.refresh(&want, child)
				got := b.sov.acc[ply]
				if got.cells != want.cells || !slices.Equal(got.sum[:players], want.sum[:players]) {
					t.Fatalf("%dx%d, %d players, ply %d: accumulators differ from a refresh", size, size, players, ply)
				}
				h := sovTurnKey(child)
				for i := range child.Owner {
					h ^= sovKey(child, i)
				}
				if b.sov.hash[ply] != h {
					t.Fatalf("%dx%d, %d players, ply %d: hash %x, from scratch %x", size, size, players, ply,
						b.sov.hash[ply], h)
				}
				n, cache := len(child.Owner), b.sov.cache
				for i, o := range child.Owner {
					for p := range players {
						if (o == core.NoOwner || int(o) == p) &&
							cache[p*n+i] != s.net.policy[sovPattern(child, i, int8(p))] {
							t.Fatalf("ply %d: cell %d's cached logit for player %d is stale", ply, i, p)
						}
					}
					if o != core.NoOwner && int(cache[players*n+i]) != sovPattern(child, i, o) {
						t.Fatalf("ply %d: cell %d's cached pattern is stale", ply, i)
					}
				}
				parent = child
			}
			for ply--; ply > 0; ply-- {
				s.undo(ply)
			}
			if !slices.Equal(b.sov.cache, root) || len(b.sov.log) != 0 {
				t.Fatalf("%dx%d, %d players: undoing every move left the cache changed", size, size, players)
			}
		}
	}
}

// TestSovereignSearchMatchesMinimax: alpha-beta with the transposition table and killer moves scores a position as
// plain minimax over the same moves and captures does.
func TestSovereignSearchMatchesMinimax(t *testing.T) {
	rng := core.NewRNG(11)
	w := make([]int16, sovParams)
	for i := range w {
		w[i] = int16(rng.Intn(601) - 300)
	}
	for _, players := range []int{2, 3} {
		for k := range 6 {
			g, _ := core.NewGame(5, 5, players)
			var legal []int
			for !g.Over() && g.Moves < 12+k {
				legal = g.LegalMoves(legal[:0])
				_ = g.Simulate(legal[rng.Intn(len(legal))])
			}
			if g.Over() {
				continue
			}
			b := Bot{g: g, me: int8(g.Turn), left: 1 << 30}
			s := sovSearch{b: &b, s: &b.sov, net: sovNet{sovCut(w)}}
			for depth := 1; depth <= 3; depth++ {
				s.begin()
				want := minimax(&s, g, depth, 0)
				s.begin()
				got, _ := s.search(g, depth, -math.MaxInt32, math.MaxInt32, 0)
				if got != want {
					t.Errorf("%d players, case %d, depth %d: alpha-beta %d, minimax %d", players, k, depth, got, want)
				}
			}
		}
	}
}

// minimax is search without pruning, table or killers.
func minimax(t *sovSearch, n *core.Game, depth, ply int) int32 {
	me := t.b.me
	if n.Over() || !n.Alive(int(me)) {
		return t.terminal(n, ply)
	}
	if depth == 0 {
		return quiet(t, n, ply, sovQuiet)
	}
	maximizing := int8(n.Turn) == me
	value := int32(math.MaxInt32)
	if maximizing {
		value = -math.MaxInt32
	}
	for _, m := range slices.Clone(t.candidates(n, maximizing, ply)) {
		child := &t.b.nodes[ply]
		t.play(child, n, int(m.cell), int(m.player), ply+1)
		v := minimax(t, child, depth-1, ply+1)
		t.undo(ply + 1)
		if maximizing {
			value = max(value, v)
		} else {
			value = min(value, v)
		}
	}
	return value
}

// quiet is quiesce without pruning: the side to move takes the better of standing pat and each capture.
func quiet(t *sovSearch, n *core.Game, ply, left int) int32 {
	me := t.b.me
	if n.Over() || !n.Alive(int(me)) {
		return t.terminal(n, ply)
	}
	value := t.net.value(&t.s.acc[ply], n, me)
	if left == 0 || ply == maxDepth {
		return value
	}
	maximizing := int8(n.Turn) == me
	for _, m := range slices.Clone(t.captures(n, maximizing, ply)) {
		child := &t.b.nodes[ply]
		t.play(child, n, int(m.cell), int(m.player), ply+1)
		v := quiet(t, child, ply+1, left-1)
		t.undo(ply + 1)
		if maximizing {
			value = max(value, v)
		} else {
			value = min(value, v)
		}
	}
	return value
}

// TestSovereignDoesNotAllocate: after one move has sized the buffers, moves on the same board allocate nothing.
func TestSovereignDoesNotAllocate(t *testing.T) {
	g := grid(t, 2, 0, "a1 . . . .", ". b2 . . .", ". . a3 . .", ". . . b1 .", ". . . . a1")
	var bot Bot
	bot.Choose(g, Sovereign, 1, DefaultBudget)
	seed := uint64(1)
	if n := testing.AllocsPerRun(5, func() { seed++; bot.Choose(g, Sovereign, seed, DefaultBudget) }); n != 0 {
		t.Errorf("%v allocations per move", n)
	}
}
