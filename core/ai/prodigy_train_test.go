//go:build train

package ai

// Self-play training for Prodigy, AlphaZero style:
//
//	ORBITRIX_TRAIN_MINUTES=60 go test -tags train -run TestTrainProdigy -timeout 0 -v ./core/ai
//
// It starts from the current prodigyWeights, or from random ones that first imitate Tactician, Hunter and Voyager for
// a head start. Then it loops: play games with noise on the root priors, half self-play and half against those three
// bots, fit the network to each search's visit shares (the other bots' chosen moves) and to the game's winner
// averaged with the search's own value, and every ten generations score it (duels against Hunter and Voyager, and
// free-for-all tables; see score). Only weights that beat the best score so far, the starting weights' included, are
// written to prodigy_weights.go. ORBITRIX_TRAIN_SEED changes the run.
//
// Like KataGo's playout cap randomisation, only a quarter of Prodigy's moves get the full budget and become samples;
// the rest search with an eighth of it, so the slow 40×40 games finish sooner without thinning the targets.

import (
	"go/format"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"orbitrix/core"
)

const (
	trainBatch       = 256
	trainWindow      = 250_000 // samples kept for fitting, newest first
	trainGames       = 192     // self-play games per generation
	trainBootstrap   = 1500    // imitation games when starting from random weights
	trainEvalEvery   = 10      // generations between scoring matches, see score
	trainEvalGames   = 96      // duels; 48 was too noisy to tell a lucky score from a better network
	trainEvalTables  = 16      // free-for-all games
	trainNoise       = 0.25    // share of the root priors replaced by noise in self-play
	trainExplore     = 1.5     // PUCT exploration in self-play, as the weights were trained with; play uses less
	trainFull        = 0.25    // share of Prodigy's moves searched with the full budget and kept as samples
	trainLearnRate   = 1e-3
	trainWeightDecay = 1e-4
	trainGameSamples = 100 // positions kept per game, evenly spaced, so long 40×40 games don't flood the window
)

// trainSizes are the benchmark boards: self-play and scoring both use them, a third of the games each.
var trainSizes = [3]int{9, 20, 40}

// bigFirst returns game i of n's board size: an equal share per size, the biggest first, so that no long game is left
// running alone at the end of a parallel batch.
func bigFirst(i, n int) int { return trainSizes[len(trainSizes)-1-i*len(trainSizes)/n] }

type sample struct {
	state core.Snapshot
	cells []int16   // target cells
	probs []float32 // their target probabilities
	won   float32   // value target: 1 if the player to move won the game, averaged with q when searched
	q     float32   // the search's winning chance for the player to move, -1 without a search
}

func TestTrainProdigy(t *testing.T) {
	deadline := time.Now().Add(time.Duration(envInt("ORBITRIX_TRAIN_MINUTES", 30)) * time.Minute)
	rng := core.NewRNG(uint64(envInt("ORBITRIX_TRAIN_SEED", 1)))
	tr := newTrainer(slices.Clone(prodigyWeights[:]))
	best := -1.0
	var buffer []sample
	if !slices.ContainsFunc(tr.w, func(v float32) bool { return v != 0 }) {
		initWeights(tr.w, &rng)
		buffer = playGames(tr.w, trainBootstrap, true, &rng)
		pl, vl := tr.fit(buffer, 6*len(buffer)/trainBatch, &rng)
		t.Logf("bootstrap: %d samples, policy loss %.3f, value loss %.3f", len(buffer), pl, vl)
	} else {
		best = score(tr.w)
		t.Logf("starting weights score %.3f", best)
	}
	for gen := 1; time.Now().Before(deadline); gen++ {
		start := time.Now()
		fresh := playGames(tr.w, trainGames, false, &rng)
		buffer = append(buffer, fresh...)
		if len(buffer) > trainWindow {
			buffer = slices.Delete(buffer, 0, len(buffer)-trainWindow)
		}
		pl, vl := tr.fit(buffer, 4*len(fresh)/trainBatch, &rng)
		t.Logf("generation %d: %d new samples, policy loss %.3f, value loss %.3f, %v", gen, len(fresh), pl, vl,
			time.Since(start).Round(time.Second))
		if gen%trainEvalEvery != 0 {
			continue
		}
		s := score(tr.w)
		t.Logf("generation %d scores %.3f (best %.3f)", gen, s, best)
		if s > best {
			best = s
			if err := saveWeights(tr.w); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestProdigyGradient checks backward against finite differences of the loss.
func TestProdigyGradient(t *testing.T) {
	rng := core.NewRNG(3)
	w := make([]float32, netParams)
	initWeights(w, &rng)
	for i := range w {
		w[i] += float32(rng.Float64()-0.5) * 0.2 // non-zero heads and biases, so every path carries gradient
	}
	g := trainPosition(t, 6, 0.8)
	legal := g.LegalMoves(nil)
	s := sample{state: g.Snapshot(), cells: []int16{int16(legal[0]), int16(legal[len(legal)-1])},
		probs: []float32{0.7, 0.3}, won: 1}
	var wk worker
	wk.grad = make([]float32, netParams)
	wk.backward(w, &s)
	loss := func() float64 {
		var probe worker
		probe.grad = make([]float32, netParams)
		pl, vl := probe.backward(w, &s)
		return pl + vl
	}
	for _, i := range []int{0, 37, 300, 700, 1500, 1860, netParams - 30, netParams - 5, netParams - 1} {
		old, eps := w[i], float32(1e-2)
		w[i] = old + eps
		up := loss()
		w[i] = old - eps
		down := loss()
		w[i] = old
		want := (up - down) / float64(2*eps)
		if got := float64(wk.grad[i]); math.Abs(got-want) > 0.05*math.Abs(want)+2e-3 {
			t.Errorf("weight %d: backward %.5f, finite difference %.5f", i, got, want)
		}
	}
}

// trainPosition plays random moves on an n×n board for 3 players until it holds about fill orbs per cell.
func trainPosition(t testing.TB, n int, fill float64) *core.Game {
	for seed := uint64(1); ; seed++ {
		g, _ := core.NewGame(n, n, 3)
		rng := core.NewRNG(seed)
		for !g.Over() && float64(g.Moves) < fill*float64(n*n) {
			legal := g.LegalMoves(nil)
			if err := g.Simulate(legal[rng.Intn(len(legal))]); err != nil {
				t.Fatal(err)
			}
		}
		if !g.Over() {
			return g
		}
	}
}

// playGames plays n games in parallel and returns their samples in game order. With imitate every seat is
// Tactician, Hunter or Voyager; otherwise every other game is self-play and the rest league games, where Prodigy
// meets them.
func playGames(w []float32, n int, imitate bool, rng *core.RNG) []sample {
	seeds := make([]uint64, n)
	for i := range seeds {
		seeds[i] = rng.Uint64()
	}
	games := make([][]sample, n)
	parallel(n, func(i int) {
		share := []float64{1, 0.5}[i%2]
		if imitate {
			share = 0
		}
		games[i] = playGame(w, seeds[i], bigFirst(i, n), share)
	})
	return slices.Concat(games...)
}

// playGame plays one game seating Prodigy with chance share at each seat and Tactician, Hunter or Voyager at the
// rest, at least one of each unless share is 0 or 1. The other bots' moves are samples too, teaching their choices.
func playGame(w []float32, seed uint64, size int, share float64) []sample {
	rng := core.NewRNG(seed)
	// Mostly duels, but every count up to twelve.
	players := 2
	switch r := rng.Intn(10); {
	case r >= 9:
		players = 9 + rng.Intn(4)
	case r >= 7:
		players = 5 + rng.Intn(4)
	case r >= 4:
		players = 3 + rng.Intn(2)
	}
	g, _ := core.NewGame(size, size, players)
	var seats [core.MaxPlayers]Strategy
	for done := false; !done; {
		mine := 0
		for p := range players {
			seats[p] = Prodigy
			if rng.Float64() >= share {
				seats[p] = []Strategy{Tactician, Hunter, Voyager}[rng.Intn(3)]
			} else {
				mine++
			}
		}
		done = share == 0 || share == 1 || mine > 0 && mine < players
	}
	bot := Bot{weights: w, explore: trainExplore}
	explore := g.Geo.Cells() / 8 // opening moves drawn by visit share, for variety
	var out []sample
	for !g.Over() && g.Moves < 50*g.Geo.Cells() {
		full := seats[g.Turn] != Prodigy || rng.Float64() < trainFull
		budget := DefaultBudget / 8
		bot.noise = 0
		if full {
			budget, bot.noise = DefaultBudget, trainNoise
		}
		cell := bot.Choose(g, seats[g.Turn], rng.Uint64(), budget)
		s := sample{state: g.Snapshot(), cells: []int16{int16(cell)}, probs: []float32{1}, q: -1}
		if seats[g.Turn] == Prodigy && len(bot.ptree) > 0 {
			root := bot.ptree[0]
			kids := bot.ptree[root.first : root.first+int32(root.kids)]
			total, reward := 0, 0.0
			for _, k := range kids {
				total, reward = total+int(k.visits), reward+k.reward
			}
			s.cells, s.probs, s.q = s.cells[:0], s.probs[:0], float32(reward/float64(max(total, 1)))
			pick := rng.Intn(max(total, 1))
			for _, k := range kids {
				s.cells, s.probs = append(s.cells, k.cell), append(s.probs, float32(k.visits)/float32(max(total, 1)))
				if pick -= int(k.visits); pick < 0 && g.Moves < explore {
					cell, pick = int(k.cell), total
				}
			}
		}
		if full {
			out = append(out, s)
		}
		if err := g.Simulate(cell); err != nil {
			panic(err)
		}
	}
	if !g.Over() {
		return nil
	}
	if n := len(out); n > trainGameSamples {
		kept := out[:0] // in place: each kept index is at or after its new slot
		for k := range trainGameSamples {
			kept = append(kept, out[k*n/trainGameSamples])
		}
		out = kept
	}
	for i := range out {
		if out[i].state.Turn == int(g.Winner) {
			out[i].won = 1
		}
		if out[i].q >= 0 { // the outcome is noisy so long before the end; the search's value is biased but steady
			out[i].won = (out[i].won + out[i].q) / 2
		}
	}
	return out
}

// score plays Prodigy with weights w against Hunter and Voyager on each benchmark board with seats alternating, and
// in free-for-all tables on 9×9 and 20×20: four players with Hunter, Voyager and Tactician, and twelve with every
// other bot plus a second Hunter. Every run uses the same random first placements. It returns Prodigy's
// share of the points: a win is 1, a duel draw half.
func score(w []float32) float64 {
	points := make([]float64, trainEvalTables+trainEvalGames)
	parallel(len(points), func(i int) {
		var seats []Strategy
		size, me := 0, 0
		if i < trainEvalTables { // the long twelve-player games first
			seats = []Strategy{Prodigy, Hunter, Voyager, Tactician}
			if i%2 == 0 {
				seats = []Strategy{Prodigy, Jester, Mason, Warden, Gambler, Nomad, Raider, Spy, Tactician, Voyager,
					Hunter, Hunter}
			}
			size, me = []int{20, 9}[i/2%2], i/2%len(seats)
			seats[0], seats[me] = seats[me], seats[0]
		} else {
			j := i - trainEvalTables
			foe := []Strategy{Hunter, Voyager}[j%2]
			size, me, seats = bigFirst(j, trainEvalGames), j/2%2, []Strategy{foe, foe}
			seats[me] = Prodigy
		}
		switch winner := match(seats, size, uint64(i)+1, false, w); {
		case winner == me:
			points[i] = 1
		case winner == -1 && len(seats) == 2:
			points[i] = 0.5
		}
	})
	total := 0.0
	for _, p := range points {
		total += p
	}
	return total / float64(len(points))
}

// TestProdigyMatches plays Prodigy against the other bots on the benchmark boards, with random first placements so
// bots without randomness still play varied games:
//
//	ORBITRIX_MATCH_GAMES=100 go test -tags train -run TestProdigyMatches -timeout 0 -v ./core/ai
//
// Each duel opponent gets that many games per board, seats alternating. Free-for-all tables seat Prodigy with three
// random other bots, and with all nine plus two random extras for twelve players, a quarter as many games each.
// ORBITRIX_MATCH_SIZES (comma-separated), ORBITRIX_MATCH_FOES (comma-separated names: those duels only, no
// free-for-all) and ORBITRIX_MATCH_WEB=1 (the browser's budget) narrow the run.
func TestProdigyMatches(t *testing.T) {
	games := envInt("ORBITRIX_MATCH_GAMES", 40)
	web := os.Getenv("ORBITRIX_MATCH_WEB") == "1"
	sizes := trainSizes[:]
	if v := os.Getenv("ORBITRIX_MATCH_SIZES"); v != "" {
		sizes = nil
		for f := range strings.SplitSeq(v, ",") {
			n, _ := strconv.Atoi(f)
			sizes = append(sizes, n)
		}
	}
	foes := os.Getenv("ORBITRIX_MATCH_FOES")
	for _, size := range sizes {
		for foe := range NumStrategies {
			if foe == Prodigy || foes != "" && !slices.Contains(strings.Split(foes, ","), foe.String()) {
				continue
			}
			start := time.Now()
			wins, _ := tables(games, size, web, func(i int, rng *core.RNG) []Strategy {
				if i%2 == 0 {
					return []Strategy{Prodigy, foe}
				}
				return []Strategy{foe, Prodigy}
			})
			t.Logf("%dx%d duel vs %-9v Prodigy %3d/%d (%.0f%%) %v", size, size, foe, wins[Prodigy], games,
				100*float64(wins[Prodigy])/float64(games), time.Since(start).Round(time.Second))
		}
		for _, players := range []int{4, 12} {
			if foes != "" {
				break
			}
			wins, seen := tables(max(1, games/4), size, web, func(i int, rng *core.RNG) []Strategy {
				seats := []Strategy{Prodigy}
				for _, s := range rngPerm(rng) {
					if s != Prodigy && len(seats) < min(players, int(NumStrategies)) {
						seats = append(seats, s)
					}
				}
				for len(seats) < players {
					if s := Strategy(rng.Intn(int(NumStrategies))); s != Prodigy {
						seats = append(seats, s)
					}
				}
				rng.Shuffle(len(seats), func(a, b int) { seats[a], seats[b] = seats[b], seats[a] })
				return seats
			})
			var line strings.Builder
			for s := range NumStrategies {
				if seen[s] > 0 {
					line.WriteString(" " + s.String() + " " + strconv.Itoa(wins[s]) + "/" + strconv.Itoa(seen[s]))
				}
			}
			t.Logf("%dx%d %dp wins/seats:%s", size, size, players, line.String())
		}
	}
}

// rngPerm returns the strategies in random order.
func rngPerm(rng *core.RNG) []Strategy {
	p := make([]Strategy, NumStrategies)
	for i := range p {
		p[i] = Strategy(i)
	}
	rng.Shuffle(len(p), func(a, b int) { p[a], p[b] = p[b], p[a] })
	return p
}

// tables plays n games on a size×size board in parallel, seating each with seats(i), and returns the wins and seats
// taken per strategy.
func tables(n, size int, web bool, seats func(i int, rng *core.RNG) []Strategy) (wins, seen [NumStrategies]int) {
	winners := make([]int, n)
	lineups := make([][]Strategy, n)
	for i := range n {
		rng := core.NewRNG(uint64(size)<<32 | uint64(i)) // the same tables every run
		lineups[i] = seats(i, &rng)
	}
	parallel(n, func(i int) { winners[i] = match(lineups[i], size, uint64(i)+1, web, nil) })
	for i, w := range winners {
		if w >= 0 {
			wins[lineups[i][w]]++
		}
		for _, s := range lineups[i] {
			seen[s]++
		}
	}
	return wins, seen
}

// match plays one game, each player's first placement at random, and returns the winning seat, or -1 if it hits the
// move cap. Prodigy plays with weights w, or prodigyWeights when nil. web gives every bot the browser's budget.
func match(seats []Strategy, size int, seed uint64, web bool, w []float32) int {
	g, _ := core.NewGame(size, size, len(seats))
	rng := core.NewRNG(seed)
	budget := DefaultBudget
	if web {
		budget = min(DefaultBudget, DefaultBudget*234/g.Geo.Cells()) // client/play.go aiBudget
	}
	bot := Bot{weights: w}
	var legal []int
	for !g.Over() && g.Moves < 50*g.Geo.Cells() {
		cell := 0
		if g.Played&(1<<g.Turn) == 0 {
			legal = g.LegalMoves(legal[:0])
			cell = legal[rng.Intn(len(legal))]
		} else {
			cell = bot.Choose(g, seats[g.Turn], rng.Uint64(), budget)
		}
		if err := g.Simulate(cell); err != nil {
			panic(err)
		}
	}
	if !g.Over() {
		return -1
	}
	return int(g.Winner)
}

type trainer struct {
	w       []float32
	m, v    []float64 // Adam moments
	grad    []float32
	step    int
	workers []worker
}

type worker struct {
	net      netEval
	grad     []float32
	dh, din  []float32
	sum      []float32
	prob     []float32
	pl, vl   float64
	examples int
}

func newTrainer(w []float32) *trainer {
	tr := &trainer{w: w, m: make([]float64, netParams), v: make([]float64, netParams), grad: make([]float32, netParams)}
	tr.workers = make([]worker, max(1, min(16, runtimeWorkers())))
	for i := range tr.workers {
		tr.workers[i].grad = make([]float32, netParams)
	}
	return tr
}

// fit runs steps Adam steps on random batches of buffer and returns the average policy and value losses.
func (tr *trainer) fit(buffer []sample, steps int, rng *core.RNG) (policyLoss, valueLoss float64) {
	batch := make([]int, trainBatch)
	examples := 0
	for range steps {
		for i := range batch {
			batch[i] = rng.Intn(len(buffer))
		}
		var wg sync.WaitGroup
		for k := range tr.workers {
			wk := &tr.workers[k]
			clear(wk.grad)
			wg.Go(func() {
				for i := k; i < len(batch); i += len(tr.workers) {
					pl, vl := wk.backward(tr.w, &buffer[batch[i]])
					wk.pl, wk.vl, wk.examples = wk.pl+pl, wk.vl+vl, wk.examples+1
				}
			})
		}
		wg.Wait()
		clear(tr.grad)
		for k := range tr.workers {
			for i, g := range tr.workers[k].grad {
				tr.grad[i] += g
			}
		}
		tr.adam(1 / float32(len(batch)))
	}
	for k := range tr.workers {
		wk := &tr.workers[k]
		policyLoss, valueLoss, examples = policyLoss+wk.pl, valueLoss+wk.vl, examples+wk.examples
		wk.pl, wk.vl, wk.examples = 0, 0, 0
	}
	if examples == 0 {
		return 0, 0
	}
	return policyLoss / float64(examples), valueLoss / float64(examples)
}

// adam applies one AdamW step with the summed gradient scaled by scale.
func (tr *trainer) adam(scale float32) {
	tr.step++
	c1, c2 := 1-math.Pow(0.9, float64(tr.step)), 1-math.Pow(0.999, float64(tr.step))
	for i, w := range tr.w {
		g := float64(tr.grad[i] * scale)
		tr.m[i] = 0.9*tr.m[i] + 0.1*g
		tr.v[i] = 0.999*tr.v[i] + 0.001*g*g
		w -= float32(trainLearnRate * (tr.m[i]/c1/(math.Sqrt(tr.v[i]/c2)+1e-8) + trainWeightDecay*float64(w)))
		tr.w[i] = w
	}
}

// backward adds the gradient of one sample's loss (policy cross-entropy plus value log loss) to wk.grad and returns
// both losses.
func (wk *worker) backward(w []float32, s *sample) (policyLoss, valueLoss float64) {
	g, err := core.Restore(s.state)
	if err != nil {
		panic(err)
	}
	b, geo, n := &wk.net, g.Geo, g.Geo.Cells()
	win := forward(g, w, b)

	// Policy: softmax over the legal cells.
	p, top := int8(g.Turn), float32(math.Inf(-1))
	for i, o := range g.Owner {
		if o == core.NoOwner || o == p {
			top = max(top, b.logit[i])
		}
	}
	prob := grow(&wk.prob, n)
	sum := 0.0
	for i, o := range g.Owner {
		prob[i] = 0
		if o == core.NoOwner || o == p {
			prob[i] = float32(math.Exp(float64(b.logit[i] - top)))
			sum += float64(prob[i])
		}
	}
	for i := range prob {
		prob[i] /= float32(sum)
	}
	for k, c := range s.cells {
		policyLoss -= float64(s.probs[k]) * math.Log(max(float64(prob[c]), 1e-12))
		prob[c] -= s.probs[k] // prob now holds the logit gradient
	}

	// Value: logistic output, log loss.
	z := float64(s.won)
	valueLoss = -z*math.Log(max(win, 1e-12)) - (1-z)*math.Log(max(1-win, 1e-12))
	dv := float32(win - z)

	policy, value := netHeads(w)
	gpolicy, gvalue := netHeads(wk.grad)
	for k, v := range valueInputs(g, &b.pool) {
		gvalue[k] += dv * v
	}
	gvalue[netWidth+netGlobal] += dv
	h := b.act[netLayers]
	dh := grow(&wk.dh, n*netWidth)
	for i := range n {
		hi, di := h[i*netWidth:(i+1)*netWidth], dh[i*netWidth:(i+1)*netWidth]
		gpolicy[netWidth] += prob[i]
		for c, v := range hi {
			gpolicy[c] += prob[i] * v
			di[c] = prob[i]*policy[c] + dv*value[c]/float32(n)
		}
	}

	for l := netLayers - 1; l >= 0; l-- {
		wl, _, in := netLayer(w, l)
		gl, gbias, _ := netLayer(wk.grad, l)
		self, nbr, gself, gnbr := wl[:in*netWidth], wl[in*netWidth:], gl[:in*netWidth], gl[in*netWidth:]
		out, x := b.act[l+1], b.act[l]
		sm := grow(&wk.sum, n*in) // each cell's neighbour sum of x
		clear(sm)
		for i := range n {
			for _, j := range geo.Neighbours(i) {
				addScaled(sm[i*in:(i+1)*in], x[int(j)*in:(int(j)+1)*in], 1)
			}
		}
		din := grow(&wk.din, n*in)
		clear(din)
		for i := range n {
			dz := dh[i*netWidth : (i+1)*netWidth]
			for c := range dz {
				if out[i*netWidth+c] <= 0 {
					dz[c] = 0 // ReLU
				}
				gbias[c] += dz[c]
			}
			for k := range in {
				if v := x[i*in+k]; v != 0 {
					addScaled(gself[k*netWidth:(k+1)*netWidth], dz, v)
				}
				if v := sm[i*in+k]; v != 0 {
					addScaled(gnbr[k*netWidth:(k+1)*netWidth], dz, v)
				}
			}
			if l == 0 {
				continue
			}
			for k := range in {
				var own, spread float32
				rs, rn := self[k*netWidth:(k+1)*netWidth], nbr[k*netWidth:(k+1)*netWidth]
				for c, d := range dz {
					own += rs[c] * d
					spread += rn[c] * d
				}
				din[i*in+k] += own
				for _, j := range geo.Neighbours(i) {
					din[int(j)*in+k] += spread
				}
			}
		}
		dh, wk.din = din, dh
	}
	wk.dh = dh
	return policyLoss, valueLoss
}

// initWeights draws uniform weights scaled for each layer's fan-in (a cell and its up to four neighbours).
func initWeights(w []float32, rng *core.RNG) {
	clear(w)
	for l := range netLayers {
		wl, _, in := netLayer(w, l)
		a := math.Sqrt(6 / float64(5*in))
		for i := range wl {
			wl[i] = float32(a * (2*rng.Float64() - 1))
		}
	}
	policy, value := netHeads(w)
	for i := range policy[:netWidth] {
		policy[i] = float32(0.02 * (2*rng.Float64() - 1))
	}
	for i := range value[:netWidth+netGlobal] {
		value[i] = float32(0.02 * (2*rng.Float64() - 1))
	}
}

// saveWeights writes w as prodigy_weights.go in the package directory.
func saveWeights(w []float32) error {
	var b strings.Builder
	b.WriteString("// Code generated by TestTrainProdigy; DO NOT EDIT.\n\npackage ai\n\nvar prodigyWeights = [netParams]float32{\n")
	for i, v := range w {
		b.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32) + ",")
		if i%8 == 7 {
			b.WriteString("\n")
		} else {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n}\n")
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return err
	}
	return os.WriteFile("prodigy_weights.go", src, 0o644)
}

func addScaled(dst, row []float32, v float32) {
	row = row[:len(dst)]
	for c := range dst {
		dst[c] += v * row[c]
	}
}

// parallel calls f(0..n-1) on every core.
func parallel(n int, f func(i int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtimeWorkers() {
		wg.Go(func() {
			for i := range next {
				f(i)
			}
		})
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func runtimeWorkers() int { return runtime.GOMAXPROCS(0) }
