package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// grid builds a game from one string per row. Each cell is "." when empty, or a player letter (a = player 0)
// followed by its orb count. Every player is marked as having played, so the win check is live.
func grid(t testing.TB, players, turn int, rows ...string) *Game {
	t.Helper()
	var cells [][]string
	for _, row := range rows {
		cells = append(cells, strings.Fields(row))
	}
	s := Snapshot{Cols: len(cells[0]), Rows: len(cells), Players: players, Turn: turn,
		Played: 1<<players - 1, Winner: NoOwner}
	for c := 0; c < s.Cols; c++ {
		for r := 0; r < s.Rows; r++ {
			cell := cells[r][c]
			if cell == "." {
				s.Count, s.Owner = append(s.Count, 0), append(s.Owner, NoOwner)
				continue
			}
			var n uint8
			if _, err := fmt.Sscanf(cell[1:], "%d", &n); err != nil {
				t.Fatalf("bad cell %q: %v", cell, err)
			}
			s.Count, s.Owner = append(s.Count, n), append(s.Owner, int8(cell[0]-'a'))
		}
	}
	g, err := Restore(s)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// render is the inverse of grid.
func render(g *Game) []string {
	rows := make([]string, g.Geo.Rows)
	for r := range rows {
		cells := make([]string, g.Geo.Cols)
		for c := range cells {
			i := g.Geo.Index(c, r)
			cells[c] = "."
			if g.Owner[i] != NoOwner {
				cells[c] = fmt.Sprintf("%c%d", 'a'+g.Owner[i], g.Count[i])
			}
		}
		rows[r] = strings.Join(cells, " ")
	}
	return rows
}

// waveString formats waves as "[c,r ...] [...]", sorting each wave because order within a wave doesn't matter.
func waveString(g *Game, waves []Wave) string {
	var out []string
	for _, w := range waves {
		cells := slices.Sorted(slices.Values(w))
		var coords []string
		for _, i := range cells {
			c, r := g.Geo.Coords(int(i))
			coords = append(coords, fmt.Sprintf("%d,%d", c, r))
		}
		out = append(out, "["+strings.Join(coords, " ")+"]")
	}
	return strings.Join(out, " ")
}

func TestGeometry(t *testing.T) {
	for _, size := range [][2]int{{3, 3}, {5, 9}, {40, 40}, {3, 40}} {
		g, err := NewGeometry(size[0], size[1])
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < g.Cells(); i++ {
			c, r := g.Coords(i)
			edges := 0
			if c == 0 || c == g.Cols-1 {
				edges++
			}
			if r == 0 || r == g.Rows-1 {
				edges++
			}
			if want := uint8(4 - edges); g.Cap[i] != want {
				t.Fatalf("%dx%d cell %d,%d: capacity %d, want %d", g.Cols, g.Rows, c, r, g.Cap[i], want)
			}
			for _, n := range g.Neighbours(i) {
				nc, nr := g.Coords(int(n))
				if abs(nc-c)+abs(nr-r) != 1 {
					t.Fatalf("cell %d,%d: %d,%d is not adjacent", c, r, nc, nr)
				}
				if !slices.Contains(g.Neighbours(int(n)), int16(i)) {
					t.Fatalf("cell %d,%d: neighbour %d,%d doesn't point back", c, r, nc, nr)
				}
			}
		}
	}
	for _, size := range [][2]int{{2, 9}, {9, 2}, {41, 9}, {9, 41}} {
		if _, err := NewGeometry(size[0], size[1]); err == nil {
			t.Errorf("NewGeometry(%d, %d) accepted an out-of-range size", size[0], size[1])
		}
	}
}

func abs(x int) int { return max(x, -x) }

func TestApply(t *testing.T) {
	tests := []struct {
		name       string
		players    int
		turn       int
		board      []string
		c, r       int
		want       []string
		waves      string
		winner     int8
		nextPlayer int
	}{
		{
			name: "no explosion",
			board: []string{
				"a1 . .",
				". . .",
				". . b1"},
			c: 1, r: 1,
			want: []string{
				"a1 . .",
				". a1 .",
				". . b1"},
			winner: NoOwner, nextPlayer: 1,
		},
		{
			name: "corner explodes at 2",
			board: []string{
				"a1 . .",
				". . .",
				". . b1"},
			want: []string{
				". a1 .",
				"a1 . .",
				". . b1"},
			waves:  "[0,0]",
			winner: NoOwner, nextPlayer: 1,
		},
		{
			name: "edge explodes at 3",
			board: []string{
				". a2 .",
				". . .",
				". . b1"},
			c: 1, r: 0,
			want: []string{
				"a1 . a1",
				". a1 .",
				". . b1"},
			waves:  "[1,0]",
			winner: NoOwner, nextPlayer: 1,
		},
		{
			name: "interior explodes at 4",
			board: []string{
				". . .",
				". a3 .",
				". . b1"},
			c: 1, r: 1,
			want: []string{
				". a1 .",
				"a1 . a1",
				". a1 b1"},
			waves:  "[1,1]",
			winner: NoOwner, nextPlayer: 1,
		},
		{
			name: "explosion captures a neighbour with its orbs",
			board: []string{
				"a1 b1 .",
				". . .",
				". . b1"},
			want: []string{
				". a2 .",
				"a1 . .",
				". . b1"},
			waves:  "[0,0]",
			winner: NoOwner, nextPlayer: 1,
		},
		{
			// Wave 2 sends two orbs into the centre (3 → 5), so it explodes in wave 3 and keeps 1. The corner
			// refills to 2 in wave 2 and explodes a second time.
			name: "chain with leftover orbs",
			board: []string{
				"a1 a2 .",
				"a2 a3 .",
				". . b1"},
			want: []string{
				". a2 a1",
				"a2 a1 a1",
				"a1 a1 b1"},
			waves:  "[0,0] [0,1 1,0] [0,0 1,1]",
			winner: NoOwner, nextPlayer: 1,
		},
		{
			// Wave 2 captures b's last orb. a wins on the spot and the corner it captured stays at capacity.
			name: "win partway through a chain",
			board: []string{
				"a1 a2 b1",
				". . .",
				". . ."},
			want: []string{
				"a1 . a2",
				"a1 a1 .",
				". . ."},
			waves:  "[0,0] [1,0]",
			winner: 0, nextPlayer: 0,
		},
		{
			name:    "eliminated player is skipped",
			players: 3,
			board: []string{
				"a1 b1 .",
				". . .",
				". . c1"},
			want: []string{
				". a2 .",
				"a1 . .",
				". . c1"},
			waves:  "[0,0]",
			winner: NoOwner, nextPlayer: 2,
		},
		{
			name:    "turn wraps past the last player",
			players: 3, turn: 2,
			board: []string{
				"a1 b1 .",
				". . .",
				". . c1"},
			c: 1, r: 1,
			want: []string{
				"a1 b1 .",
				". c1 .",
				". . c1"},
			winner: NoOwner, nextPlayer: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := grid(t, max(tt.players, 2), tt.turn, tt.board...)
			waves, err := g.Apply(g.Geo.Index(tt.c, tt.r))
			if err != nil {
				t.Fatal(err)
			}
			if got := render(g); !slices.Equal(got, tt.want) {
				t.Errorf("board:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
			if got := waveString(g, waves); got != tt.waves {
				t.Errorf("waves %q, want %q", got, tt.waves)
			}
			if g.Winner != tt.winner {
				t.Errorf("winner %d, want %d", g.Winner, tt.winner)
			}
			if g.Turn != tt.nextPlayer {
				t.Errorf("turn %d, want %d", g.Turn, tt.nextPlayer)
			}
			checkTallies(t, g)
		})
	}
}

func TestIllegalMoves(t *testing.T) {
	g := grid(t, 2, 0,
		"a1 b1 .",
		". . .",
		". . .")
	for _, cell := range []int{-1, g.Geo.Cells(), g.Geo.Index(1, 0)} {
		if _, err := g.Apply(cell); !errors.Is(err, ErrIllegal) {
			t.Errorf("Apply(%d): err %v, want ErrIllegal", cell, err)
		}
	}
	if g.Moves != 0 || g.Turn != 0 {
		t.Fatalf("illegal move changed the game: moves %d, turn %d", g.Moves, g.Turn)
	}

	won := grid(t, 2, 0,
		"a1 a2 b1",
		". . .",
		". . .")
	if _, err := won.Apply(0); err != nil || !won.Over() {
		t.Fatalf("setup: err %v, over %v", err, won.Over())
	}
	if _, err := won.Apply(won.Geo.Index(2, 2)); !errors.Is(err, ErrGameOver) {
		t.Errorf("move after the win: err %v, want ErrGameOver", err)
	}
	if moves := won.LegalMoves(nil); len(moves) != 0 {
		t.Errorf("LegalMoves after the win: %v", moves)
	}
}

func TestLegalMoves(t *testing.T) {
	g := grid(t, 2, 1,
		"a1 b1 .",
		". . .",
		". . a3")
	got := g.LegalMoves(nil)
	for i := 0; i < g.Geo.Cells(); i++ {
		if g.Legal(i) != slices.Contains(got, i) {
			t.Fatalf("cell %d: Legal %v but LegalMoves %v", i, g.Legal(i), got)
		}
	}
	if len(got) != 7 {
		t.Errorf("player b has %d legal moves, want 7", len(got))
	}
}

// TestFirstRound checks that players who haven't moved yet are alive and that nobody can win in the first round,
// with the maximum of 12 players.
func TestFirstRound(t *testing.T) {
	g, err := NewGame(5, 5, MaxPlayers)
	if err != nil {
		t.Fatal(err)
	}
	for p := 0; p < MaxPlayers; p++ {
		if g.Turn != p {
			t.Fatalf("move %d: turn %d", p, g.Turn)
		}
		if _, err := g.Apply(2 * p); err != nil {
			t.Fatal(err)
		}
		if g.Over() {
			t.Fatalf("player %d won in the first round", g.Winner)
		}
	}
	if g.Turn != 0 {
		t.Fatalf("second round starts with player %d", g.Turn)
	}
}

func TestSinglePlayerOnBoardIsNotAWinUntilTwoHavePlayed(t *testing.T) {
	g, _ := NewGame(3, 3, 2)
	if _, err := g.Apply(4); err != nil {
		t.Fatal(err)
	}
	if g.Over() || g.Turn != 1 || !g.Alive(1) {
		t.Fatalf("after the first move: over %v, turn %d, player 1 alive %v", g.Over(), g.Turn, g.Alive(1))
	}
}

func TestFloodEndsInAWin(t *testing.T) {
	g := flood(t, 40, 40)
	waves, err := g.Apply(0)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Over() || g.Winner != 0 || len(waves) >= MaxWaves {
		t.Fatalf("winner %d after %d waves", g.Winner, len(waves))
	}
	checkTallies(t, g)
}

// TestWaveLimit cuts a chain short and checks that the next move, by the other player, resolves the leftover
// unstable cells together with its own explosion. Two owners exploding at once can recapture each other forever,
// so the second move may hit the limit too; it must still conserve orbs.
func TestWaveLimit(t *testing.T) {
	defer func(old int) { waveLimit = old }(waveLimit)
	waveLimit = 3
	g := flood(t, 9, 9)
	waves, err := g.Apply(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 3 || g.Over() || !g.unstable || g.Turn != 1 {
		t.Fatalf("after the cut: %d waves, over %v, unstable %v, turn %d", len(waves), g.Over(), g.unstable, g.Turn)
	}
	checkTallies(t, g)

	before := total(g)
	waves, err = g.Apply(g.Geo.Cells() - 1) // b's corner: it explodes alongside a's leftovers
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(waves[0], int16(g.Geo.Cells()-1)) || len(waves[0]) < 2 {
		t.Fatalf("first wave %v should hold b's corner and a's leftovers", waves[0])
	}
	if total(g) != before+1 {
		t.Fatalf("orbs %d → %d", before, total(g))
	}
	checkTallies(t, g)
}

func total(g *Game) int {
	n := 0
	for _, c := range g.Count {
		n += int(c)
	}
	return n
}

// flood returns a cols×rows board where every cell is one orb short of exploding. a owns everything except the
// far corner, which is b's only orb. a to move: placing at cell 0 floods the board until b is captured.
func flood(t testing.TB, cols, rows int) *Game {
	t.Helper()
	geo, err := NewGeometry(cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{Cols: cols, Rows: rows, Players: 2, Played: 3, Winner: NoOwner,
		Count: make([]uint8, geo.Cells()), Owner: make([]int8, geo.Cells())}
	for i := range s.Count {
		s.Count[i] = geo.Cap[i] - 1
	}
	s.Owner[geo.Cells()-1] = 1
	g, err := Restore(s)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// checkTallies verifies the per-player orb totals and the count/owner pairing against the board.
func checkTallies(t testing.TB, g *Game) {
	t.Helper()
	var orbs [MaxPlayers]int32
	for i, c := range g.Count {
		o := g.Owner[i]
		if (c == 0) != (o == NoOwner) {
			t.Fatalf("cell %d: %d orbs, owner %d", i, c, o)
		}
		if o != NoOwner {
			orbs[o] += int32(c)
		}
	}
	if orbs != g.Orbs {
		t.Fatalf("tallies %v, board says %v", g.Orbs, orbs)
	}
}
