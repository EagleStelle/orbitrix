// Command aitourney plays every pair of AI strategies against each other on one board size and prints the win rates
// and move times. Seats alternate between games; each game's seed comes from -seed and its index, so results are
// reproducible whatever the worker count.
//
//	go run ./cmd/aitourney -games 20 -size 9x9
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"orbitrix/core"
	"orbitrix/core/ai"
)

type game struct {
	seats [2]ai.Strategy
	seed  uint64
}

type stats struct {
	mu    sync.Mutex
	wins  [ai.NumStrategies][ai.NumStrategies]int // wins[a][b]: games a won against b
	games [ai.NumStrategies][ai.NumStrategies]int
	draws int
	times [ai.NumStrategies][]time.Duration
}

func main() {
	games := flag.Int("games", 10, "games per pair of strategies")
	size := flag.String("size", "9x9", "board size, COLSxROWS")
	seed := flag.Uint64("seed", 1, "base seed")
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "games played in parallel; move times are per core")
	flag.Parse()

	var cols, rows int
	if _, err := fmt.Sscanf(*size, "%dx%d", &cols, &rows); err != nil {
		log.Fatalf("bad -size %q: want COLSxROWS", *size)
	}
	if _, err := core.NewGame(cols, rows, 2); err != nil {
		log.Fatal(err)
	}

	var jobs []game
	for a := range ai.NumStrategies {
		for b := a + 1; b < ai.NumStrategies; b++ {
			for k := range *games {
				g := game{seats: [2]ai.Strategy{a, b}}
				if k%2 == 1 {
					g.seats = [2]ai.Strategy{b, a}
				}
				rng := core.NewRNG(*seed + uint64(len(jobs)))
				g.seed = rng.Uint64()
				jobs = append(jobs, g)
			}
		}
	}

	var st stats
	queue := make(chan game)
	var wg sync.WaitGroup
	start := time.Now()
	for range max(*workers, 1) {
		wg.Go(func() {
			for g := range queue {
				play(cols, rows, g, &st)
			}
		})
	}
	for _, g := range jobs {
		queue <- g
	}
	close(queue)
	wg.Wait()

	fmt.Printf("Orbitrix AI tournament: %dx%d, %d games per pair (seats alternate), seed %d, %d games in %v\n\n",
		cols, rows, *games, *seed, len(jobs), time.Since(start).Round(time.Millisecond))
	report(&st)
}

// play runs one game to the end, or calls it a draw after maxMoves.
func play(cols, rows int, job game, st *stats) {
	g, _ := core.NewGame(cols, rows, 2)
	rng := core.NewRNG(job.seed)
	var bots [2]ai.Bot
	var times [2][]time.Duration
	maxMoves := 100 * cols * rows
	for !g.Over() && g.Moves < maxMoves {
		s := job.seats[g.Turn]
		t0 := time.Now()
		cell := bots[g.Turn].Choose(g, s, rng.Uint64(), ai.DefaultBudget)
		times[g.Turn] = append(times[g.Turn], time.Since(t0))
		if _, err := g.Apply(cell); err != nil {
			log.Fatalf("%v played an illegal move: %v", s, err)
		}
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	a, b := job.seats[0], job.seats[1]
	st.games[a][b]++
	st.games[b][a]++
	switch {
	case !g.Over():
		st.draws++
	case g.Winner == 0:
		st.wins[a][b]++
	default:
		st.wins[b][a]++
	}
	for seat, s := range job.seats {
		st.times[s] = append(st.times[s], times[seat]...)
	}
}

func report(st *stats) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Println("Win % of the row strategy against the column strategy:")
	header := []string{""}
	for s := range ai.NumStrategies {
		header = append(header, s.String())
	}
	fmt.Fprintln(w, strings.Join(header, "\t")+"\t")
	for a := range ai.NumStrategies {
		row := []string{a.String()}
		for b := range ai.NumStrategies {
			if a == b {
				row = append(row, "-")
			} else {
				row = append(row, fmt.Sprintf("%.0f", pct(st.wins[a][b], st.games[a][b])))
			}
		}
		fmt.Fprintln(w, strings.Join(row, "\t")+"\t")
	}
	w.Flush()

	fmt.Printf("\nDraws (move cap reached): %d\n\n", st.draws)
	fmt.Fprintln(w, "strategy\twin %\tmoves\tavg ms\tp99 ms\tmax ms\t")
	for s := range ai.NumStrategies {
		won, played := 0, 0
		for b := range ai.NumStrategies {
			won, played = won+st.wins[s][b], played+st.games[s][b]
		}
		t := st.times[s]
		slices.Sort(t)
		var sum time.Duration
		for _, d := range t {
			sum += d
		}
		var avg, p99, worst time.Duration
		if len(t) > 0 {
			avg, p99, worst = sum/time.Duration(len(t)), t[(len(t)-1)*99/100], t[len(t)-1]
		}
		fmt.Fprintf(w, "%v\t%.0f\t%d\t%.3f\t%.3f\t%.3f\t\n", s, pct(won, played), len(t), ms(avg), ms(p99), ms(worst))
	}
	w.Flush()
}

func pct(n, of int) float64 {
	if of == 0 {
		return 0
	}
	return 100 * float64(n) / float64(of)
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
