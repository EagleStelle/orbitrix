//go:build train

package ai

// Supervised training for Sovereign, the way Stockfish trains NNUE:
//
//	ORBITRIX_TRAIN_MINUTES=480 go test -tags train -run TestTrainSovereign -timeout 0 -v ./core/ai
//
// Each iteration plays games with random first placements on 9×9, 20×20 and 40×40 (a third each, biggest first;
// ORBITRIX_SOV_SIZES changes the mix), 2–12 players (40% duels, a quarter nine to twelve), seated from Hunter,
// Voyager, Tactician and Prodigy, and Sovereign itself once it has trained weights. It keeps at most 100 evenly spaced
// positions a game, each with the move played and every player's result. Once Sovereign has weights, it also searches
// every kept position with twice the default budget (ORBITRIX_SOV_LABEL_BUDGET): the value target of the player to
// move becomes the result blended with that search's winning chance, and its move the policy target
// (self-distillation). Positions are appended to ORBITRIX_SOV_DATA (default: a folder in the temp directory), and
// every iteration trains a fresh network on all of them, AdamW with a tenth of the games held out. The network with
// the lowest held-out loss is quantised, saved there as a candidate, and plays the current weights in duels and at
// free-for-all tables (see sovMatchScore); only one that wins sovGate of those games is written to
// sovereign_weights.go. ORBITRIX_SOV_GAMES sets the games per iteration, ORBITRIX_SOV_ITERATIONS caps the
// iterations, ORBITRIX_SOV_TRAIN_ONLY=1 skips the first iteration's games, ORBITRIX_SOV_DATA_ONLY=1 stops after them,
// ORBITRIX_SOV_NO_LABEL=1 keeps the results alone as targets, ORBITRIX_SOV_NO_GATE=1 keeps every network (scratch
// experiments only) and ORBITRIX_TRAIN_SEED changes the run.

import (
	"encoding/gob"
	"fmt"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"orbitrix/core"
)

const (
	sovLambda    = 0.5  // weight of the game's result in a searched position's value target
	sovGateGames = 150  // duels between the new and the current weights, and as many free-for-all tables
	sovGate      = 0.55 // share of the games either won that the new weights must win
	sovEpochs    = 30
	sovPatience  = 4 // epochs without a better held-out loss before training stops
)

type sovSample struct {
	State  core.Snapshot
	Game   uint64
	Cell   int16                    // the move played: the policy's target unless Best is set
	Best   int16                    // the labelling search's move, -1 without
	Value  [core.MaxPlayers]float32 // 1 for the winner
	Search float32                  // the labelling search's winning chance for the player to move, -1 without
}

func TestTrainSovereign(t *testing.T) {
	deadline := time.Now().Add(time.Duration(envInt("ORBITRIX_TRAIN_MINUTES", 60)) * time.Minute)
	rng := core.NewRNG(uint64(envInt("ORBITRIX_TRAIN_SEED", 1)))
	dir := sovDataDir()
	cur := slices.Clone(sovereignWeights[:])
	trained := slices.ContainsFunc(cur, func(v int16) bool { return v != 0 })
	for iter := 0; iter < envInt("ORBITRIX_SOV_ITERATIONS", 1000) && time.Now().Before(deadline); iter++ {
		if iter > 0 || os.Getenv("ORBITRIX_SOV_TRAIN_ONLY") != "1" {
			start := time.Now()
			pool := []Strategy{Hunter, Voyager, Tactician, Prodigy}
			var label []int16
			if trained {
				pool, label = append(pool, Sovereign), cur
			}
			if os.Getenv("ORBITRIX_SOV_NO_LABEL") == "1" {
				label = nil
			}
			data := sovGames(envInt("ORBITRIX_SOV_GAMES", 960), sovSizes(), pool, cur, label, &rng)
			name := filepath.Join(dir, fmt.Sprintf("data-%d-%d.gob", time.Now().Unix(), iter))
			if err := sovSave(name, data); err != nil {
				t.Fatal(err)
			}
			t.Logf("iteration %d: %d positions to %s, %v", iter, len(data), name, time.Since(start).Round(time.Second))
			if os.Getenv("ORBITRIX_SOV_DATA_ONLY") == "1" {
				return
			}
		}
		train, val, err := sovLoad(dir)
		if err != nil {
			t.Fatal(err)
		}
		w := sovFit(t, train, val, &rng)
		q := sovQuantize(w)
		if err := sovSaveWeights(filepath.Join(dir, fmt.Sprintf("candidate-%d.go.txt", time.Now().Unix())), q); err != nil {
			t.Fatal(err)
		}
		vl, pl := sovIntLoss(q, val)
		t.Logf("iteration %d: quantised held-out value loss %.4f, policy loss %.4f", iter, vl, pl)
		if !trained {
			t.Logf("iteration %d: no trained weights yet; scoring the new ones against the old all the same", iter)
		}
		score := 1.0 // ORBITRIX_SOV_NO_GATE=1: experiments keep every network
		if os.Getenv("ORBITRIX_SOV_NO_GATE") != "1" {
			score = sovMatchScore(q, cur, sovGateGames)
			t.Logf("iteration %d: new weights score %.3f against the current ones", iter, score)
		}
		if score >= sovGate {
			if err := sovSaveWeights("sovereign_weights.go", q); err != nil {
				t.Fatal(err)
			}
			cur, trained = q, true
		}
	}
}

// TestSovereignGradient checks backward against finite differences of the loss.
func TestSovereignGradient(t *testing.T) {
	rng := core.NewRNG(3)
	w := make([]float32, sovParams)
	sovInit(w, &rng)
	for i := range w {
		w[i] += float32(rng.Float64()-0.5) * 0.2
	}
	g := trainPosition(t, 6, 1.0)
	legal := g.LegalMoves(nil)
	s := sovSample{State: g.Snapshot(), Cell: int16(legal[len(legal)/2]), Best: -1, Search: 0.3}
	s.Value[1] = 1
	var wk sovWorker
	grad := make([]float32, sovParams)
	wk.run(w, grad, &s)
	loss := func() float64 {
		var probe sovWorker
		vl, pl := probe.run(w, nil, &s)
		return vl + pl
	}
	gs, _ := core.Restore(s.State)
	occupied := 0
	for i, o := range gs.Owner {
		if o != core.NoOwner {
			occupied = sovPattern(gs, i, o)
			break
		}
	}
	idx := make([]int, sovParams)
	for i := range idx {
		idx[i] = i
	}
	p := sovCut(idx) // each layer's weight indices
	checks := []int{occupied*sovH + 3, occupied*sovH + 17, p.b1[2], p.b1[sovH+5], p.w2[7], p.w2[2*sovH*sovL2+3],
		p.b2[1], p.w3[4], p.b3[0], p.policy[sovPattern(gs, legal[0], int8(gs.Turn))],
		p.policy[sovPattern(gs, int(s.Cell), int8(gs.Turn))]}
	for _, i := range checks {
		old, eps := w[i], float32(1e-2)
		w[i] = old + eps
		up := loss()
		w[i] = old - eps
		down := loss()
		w[i] = old
		want := (up - down) / float64(2*eps)
		got := float64(grad[i])
		if math.Abs(got-want) > 0.02*math.Abs(want)+1e-4 {
			t.Errorf("weight %d: backward %.5f, finite difference %.5f", i, got, want)
		}
	}
}

// sovGames plays n games in parallel, an equal share on each of sizes in order, and returns their positions in game
// order. Sovereign seats play cur; label, when set, labels every kept position with a deeper search.
func sovGames(n int, sizes []int, pool []Strategy, cur, label []int16, rng *core.RNG) []sovSample {
	seeds := make([]uint64, n)
	for i := range seeds {
		seeds[i] = rng.Uint64()
	}
	games := make([][]sovSample, n)
	var done atomic.Int64
	start := time.Now()
	parallel(n, func(i int) {
		games[i] = sovGame(seeds[i], sizes[i*len(sizes)/n], pool, cur, label)
		if k := done.Add(1); k%max(1, int64(n)/20) == 0 { // progress every 5%
			fmt.Printf("    games %d/%d, %v\n", k, n, time.Since(start).Round(time.Second))
		}
	})
	return slices.Concat(games...)
}

// sovSizes returns ORBITRIX_SOV_SIZES (comma-separated board sizes, biggest first; repeat one for a bigger share),
// or the benchmark boards.
func sovSizes() []int {
	v := os.Getenv("ORBITRIX_SOV_SIZES")
	if v == "" {
		return []int{40, 20, 9}
	}
	var sizes []int
	for f := range strings.SplitSeq(v, ",") {
		n, _ := strconv.Atoi(f)
		sizes = append(sizes, n)
	}
	return sizes
}

// sovGame plays one game: each player's first placement at random, then each seat a random bot from pool.
func sovGame(seed uint64, size int, pool []Strategy, cur, label []int16) []sovSample {
	rng := core.NewRNG(seed)
	players := 2 // 40% duels, 20% three or four players, 15% five to eight, 25% nine to twelve
	switch r := rng.Intn(20); {
	case r >= 15:
		players = 9 + rng.Intn(4)
	case r >= 12:
		players = 5 + rng.Intn(4)
	case r >= 8:
		players = 3 + rng.Intn(2)
	}
	g, _ := core.NewGame(size, size, players)
	seats := make([]Strategy, players)
	for p := range seats {
		seats[p] = pool[rng.Intn(len(pool))]
	}
	bot := Bot{sweights: cur}
	var out []sovSample
	var legal []int
	for !g.Over() && g.Moves < 50*g.Geo.Cells() {
		var cell int
		if g.Played&(1<<g.Turn) == 0 {
			legal = g.LegalMoves(legal[:0])
			cell = legal[rng.Intn(len(legal))]
		} else {
			cell = bot.Choose(g, seats[g.Turn], rng.Uint64(), DefaultBudget)
			out = append(out, sovSample{State: g.Snapshot(), Game: seed, Cell: int16(cell), Best: -1, Search: -1})
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
		s := &out[i]
		s.Value[g.Winner] = 1
		if label == nil {
			continue
		}
		h, _ := core.Restore(s.State)
		bot.sweights = label
		s.Best = int16(bot.Choose(h, Sovereign, rng.Uint64(), envInt("ORBITRIX_SOV_LABEL_BUDGET", 2*DefaultBudget)))
		s.Search = float32(sigmoid(float64(bot.sov.score) / (sovOne << sovShift)))
	}
	return out
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

func sovDataDir() string {
	dir := os.Getenv("ORBITRIX_SOV_DATA")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "orbitrix-sovereign")
	}
	return dir
}

func sovSave(name string, data []sovSample) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(data); err != nil {
		f.Close()
		return fmt.Errorf("encode %s: %w", name, err)
	}
	return f.Close()
}

// sovLoad reads every data file in dir and holds out a tenth of the games.
func sovLoad(dir string) (train, val []sovSample, err error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.gob"))
	if err != nil {
		return nil, nil, err
	}
	slices.Sort(names)
	for _, name := range names {
		f, err := os.Open(name)
		if err != nil {
			return nil, nil, err
		}
		var data []sovSample
		err = gob.NewDecoder(f).Decode(&data)
		f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("decode %s: %w", name, err)
		}
		for _, s := range data {
			if sovMix(s.Game)%10 == 0 {
				val = append(val, s)
			} else {
				train = append(train, s)
			}
		}
	}
	if len(train) == 0 || len(val) == 0 {
		return nil, nil, fmt.Errorf("%s: %d training and %d held-out positions", dir, len(train), len(val))
	}
	return train, val, nil
}

// sovFit trains fresh weights on train and returns those with the lowest loss on val, separately for the value
// network and the policy table, which share no weights.
func sovFit(t *testing.T, train, val []sovSample, rng *core.RNG) []float32 {
	w := make([]float32, sovParams)
	sovInit(w, rng)
	tr := &trainer{w: w, m: make([]float64, sovParams), v: make([]float64, sovParams), grad: make([]float32, sovParams)}
	workers := make([]sovWorker, max(1, min(16, runtimeWorkers())))
	grads := make([][]float32, len(workers))
	for i := range grads {
		grads[i] = make([]float32, sovParams)
	}
	best, bestValue, bestPolicy, stale := slices.Clone(w), math.Inf(1), math.Inf(1), 0
	policy := sovParams - sovPatterns // the policy table's first weight
	order := make([]int, len(train))
	for e := range sovEpochs {
		start := time.Now()
		for i := range order {
			order[i] = i
		}
		rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
		var tvl, tpl float64
		for at := 0; at+trainBatch <= len(order); at += trainBatch {
			batch := order[at : at+trainBatch]
			var wg sync.WaitGroup
			var mu sync.Mutex
			for k := range workers {
				clear(grads[k])
				wg.Go(func() {
					var vl, pl float64
					for i := k; i < len(batch); i += len(workers) {
						v, p := workers[k].run(w, grads[k], &train[batch[i]])
						vl, pl = vl+v, pl+p
					}
					mu.Lock()
					tvl, tpl = tvl+vl, tpl+pl
					mu.Unlock()
				})
			}
			wg.Wait()
			clear(tr.grad)
			for k := range grads {
				for i, g := range grads[k] {
					tr.grad[i] += g
				}
			}
			tr.adam(1 / float32(len(batch)))
			sovClamp(w)
		}
		vl, pl := sovLoss(w, val)
		n := float64(len(order) / trainBatch * trainBatch)
		t.Logf("epoch %d: train value %.4f policy %.4f, held-out value %.4f policy %.4f, %v", e, tvl/n, tpl/n, vl, pl,
			time.Since(start).Round(time.Second))
		stale++
		if vl < bestValue {
			copy(best[:policy], w[:policy])
			bestValue, stale = vl, 0
		}
		if pl < bestPolicy {
			copy(best[policy:], w[policy:])
			bestPolicy, stale = pl, 0
		}
		if stale == sovPatience {
			break
		}
	}
	return best
}

// sovLoss returns the float network's average value and policy losses on data.
func sovLoss(w []float32, data []sovSample) (valueLoss, policyLoss float64) {
	vl, pl := make([]float64, len(data)), make([]float64, len(data))
	workers := make(chan *sovWorker, runtimeWorkers())
	for range runtimeWorkers() {
		workers <- new(sovWorker)
	}
	parallel(len(data), func(i int) {
		wk := <-workers
		vl[i], pl[i] = wk.run(w, nil, &data[i])
		workers <- wk
	})
	for i := range data {
		valueLoss, policyLoss = valueLoss+vl[i], policyLoss+pl[i]
	}
	return valueLoss / float64(len(data)), policyLoss / float64(len(data))
}

// sovIntLoss returns the quantised network's average value and policy losses on data, as the search computes them.
func sovIntLoss(q []int16, data []sovSample) (valueLoss, policyLoss float64) {
	net := sovNet{sovCut(q)}
	vl, pl := make([]float64, len(data)), make([]float64, len(data))
	parallel(len(data), func(i int) {
		s := &data[i]
		g, _ := core.Restore(s.State)
		var a sovAcc
		net.refresh(&a, g)
		persp := 0
		for p := range g.Players {
			if g.Alive(p) {
				persp++
			}
		}
		for p := range g.Players {
			if g.Alive(p) {
				prob := sigmoid(float64(net.value(&a, g, int8(p))) / (sovOne << sovShift))
				vl[i] += bce(prob, float64(sovTarget(s, g, p, persp))) / float64(persp)
			}
		}
		top, sum := math.Inf(-1), 0.0
		for c, o := range g.Owner {
			if o == core.NoOwner || int(o) == g.Turn {
				top = max(top, float64(net.policy[sovPattern(g, c, int8(g.Turn))])/sovOne)
			}
		}
		for c, o := range g.Owner {
			if o == core.NoOwner || int(o) == g.Turn {
				sum += math.Exp(float64(net.policy[sovPattern(g, c, int8(g.Turn))])/sovOne - top)
			}
		}
		target := sovPolicyTarget(s)
		pl[i] = math.Log(sum) - (float64(net.policy[sovPattern(g, target, int8(g.Turn))])/sovOne - top)
	})
	for i := range data {
		valueLoss, policyLoss = valueLoss+vl[i], policyLoss+pl[i]
	}
	return valueLoss / float64(len(data)), policyLoss / float64(len(data))
}

func bce(p, y float64) float64 {
	return -y*math.Log(max(p, 1e-12)) - (1-y)*math.Log(max(1-p, 1e-12))
}

// sovTarget is player p's value target in s: their result, blended with the labelling search where there is one. The
// search scores the player to move, and in a duel the other player too.
func sovTarget(s *sovSample, g *core.Game, p, persp int) float32 {
	y := s.Value[p]
	switch {
	case s.Search < 0:
	case p == g.Turn:
		y = sovLambda*y + (1-sovLambda)*s.Search
	case persp == 2:
		y = sovLambda*y + (1-sovLambda)*(1-s.Search)
	}
	return y
}

func sovPolicyTarget(s *sovSample) int {
	if s.Best >= 0 {
		return int(s.Best)
	}
	return int(s.Cell)
}

// sovWorker computes losses and gradients for one position at a time, reusing its buffers.
type sovWorker struct {
	acc, dacc [core.MaxPlayers][sovH]float32
	pats      []int32 // occupied cells' patterns, then the legal cells'
	owners    []int8
	prob      []float32
}

// run returns the float network's value loss (averaged over the alive players) and policy loss on s, and adds their
// gradient to grad unless it is nil.
func (wk *sovWorker) run(w, grad []float32, s *sovSample) (valueLoss, policyLoss float64) {
	g, err := core.Restore(s.State)
	if err != nil {
		panic(err)
	}
	f := sovCut(w)
	var d sovParts[float32]
	if grad != nil {
		d = sovCut(grad)
	}
	for q := range g.Players {
		wk.acc[q], wk.dacc[q] = [sovH]float32{}, [sovH]float32{}
	}
	wk.pats, wk.owners = wk.pats[:0], wk.owners[:0]
	for i, o := range g.Owner {
		if o == core.NoOwner {
			continue
		}
		pat := sovPattern(g, i, o)
		wk.pats, wk.owners = append(wk.pats, int32(pat)), append(wk.owners, o)
		addScaled(wk.acc[o][:], f.w1[pat*sovH:(pat+1)*sovH], 1)
	}
	cells := len(wk.pats)
	inv := float32(0)
	if cells > 0 {
		inv = 1 / float32(cells)
	}

	persp := 0
	for p := range g.Players {
		if g.Alive(p) {
			persp++
		}
	}
	for p := range g.Players {
		if !g.Alive(p) {
			continue
		}
		var opp [sovH]float32
		var arg [sovH]int8
		for k := range arg {
			arg[k] = -1
		}
		for q := range g.Players {
			if q == p || g.Orbs[q] == 0 {
				continue
			}
			for k, v := range wk.acc[q] {
				if arg[k] < 0 || v > opp[k] {
					opp[k], arg[k] = v, int8(q)
				}
			}
		}
		var z1 [2 * sovH]float32
		var x [sovIn]float32
		for k := range sovH {
			z1[k] = wk.acc[p][k]*inv + f.b1[k]
			z1[sovH+k] = opp[k]*inv + f.b1[sovH+k]
		}
		for k, z := range z1 {
			x[k] = min(max(z, 0), 1)
		}
		for k, v := range sovGlobals(g, int8(p), int32(cells)) {
			x[2*sovH+k] = float32(v) / sovOne
		}
		var z2, h2 [sovL2]float32
		copy(z2[:], f.b2)
		for k, v := range x {
			if v != 0 {
				addScaled(z2[:], f.w2[k*sovL2:(k+1)*sovL2], v)
			}
		}
		z := f.b3[0]
		for o, v := range z2 {
			h2[o] = min(max(v, 0), 1)
			z += h2[o] * f.w3[o]
		}
		prob := sigmoid(float64(z))
		y := float64(sovTarget(s, g, p, persp))
		valueLoss += bce(prob, y) / float64(persp)
		if grad == nil {
			continue
		}

		dz := float32(prob-y) / float32(persp)
		d.b3[0] += dz
		var dz2 [sovL2]float32
		for o, v := range z2 {
			d.w3[o] += dz * h2[o]
			if v > 0 && v < 1 {
				dz2[o] = dz * f.w3[o]
			}
		}
		addScaled(d.b2, dz2[:], 1)
		for k, v := range x {
			if v != 0 {
				addScaled(d.w2[k*sovL2:(k+1)*sovL2], dz2[:], v)
			}
		}
		for k, z := range z1 {
			if z <= 0 || z >= 1 {
				continue
			}
			var dx float32
			for o, v := range f.w2[k*sovL2 : (k+1)*sovL2] {
				dx += v * dz2[o]
			}
			d.b1[k] += dx
			switch {
			case k < sovH:
				wk.dacc[p][k] += dx * inv
			case arg[k-sovH] >= 0:
				wk.dacc[arg[k-sovH]][k-sovH] += dx * inv
			}
		}
	}
	if grad != nil {
		for i, pat := range wk.pats {
			addScaled(d.w1[pat*sovH:(pat+1)*sovH], wk.dacc[wk.owners[i]][:], 1)
		}
	}

	// Policy: softmax over the legal cells, cross-entropy with the target move.
	mover, target := int8(g.Turn), sovPolicyTarget(s)
	wk.pats = wk.pats[:0]
	top := float32(math.Inf(-1))
	at := -1
	for i, o := range g.Owner {
		if o == core.NoOwner || o == mover {
			if i == target {
				at = len(wk.pats)
			}
			pat := sovPattern(g, i, mover)
			wk.pats = append(wk.pats, int32(pat))
			top = max(top, f.policy[pat])
		}
	}
	wk.prob = grow(&wk.prob, len(wk.pats))
	sum := 0.0
	for i, pat := range wk.pats {
		wk.prob[i] = float32(math.Exp(float64(f.policy[pat] - top)))
		sum += float64(wk.prob[i])
	}
	policyLoss = -math.Log(max(float64(wk.prob[at])/sum, 1e-12))
	if grad != nil {
		for i, pat := range wk.pats {
			pr := float32(float64(wk.prob[i]) / sum)
			if i == at {
				pr--
			}
			d.policy[pat] += pr
		}
	}
	return valueLoss, policyLoss
}

// sovInit draws starting weights: rows and hidden weights uniform for their fan-in, biases that start the clipped
// ReLUs in their middle, and a zero policy.
func sovInit(w []float32, rng *core.RNG) {
	clear(w)
	p := sovCut(w)
	fill := func(s []float32, a float64) {
		for i := range s {
			s[i] = float32(a * (2*rng.Float64() - 1))
		}
	}
	fill(p.w1, 0.5)
	fill(p.w2, math.Sqrt(6/float64(sovIn+sovL2)))
	fill(p.w3, math.Sqrt(6/float64(sovL2+1)))
	for i := range p.b1 {
		p.b1[i] = 0.5
	}
	for i := range p.b2 {
		p.b2[i] = 0.5
	}
}

// sovScale returns weight i's fixed-point scale and the largest magnitude its int16 holds.
func sovScale(i int) (scale, limit float32) {
	w2 := sovOwned*sovH + 2*sovH
	w3 := w2 + sovIn*sovL2 + sovL2
	if w2 <= i && i < w2+sovIn*sovL2 || w3 <= i && i < w3+sovL2 {
		return 1 << sovShift, 31
	}
	return sovOne, 127
}

// sovClamp keeps every weight within what its int16 holds.
func sovClamp(w []float32) {
	for i, v := range w {
		_, limit := sovScale(i)
		w[i] = min(max(v, -limit), limit)
	}
}

func sovQuantize(w []float32) []int16 {
	q := make([]int16, len(w))
	for i, v := range w {
		scale, _ := sovScale(i)
		q[i] = int16(math.Round(float64(v * scale)))
	}
	return q
}

// sovMatchScore plays Sovereign with weights a against b: n free-for-all tables seating both among random other bots
// (half twelve players, half four, on 20×20 and 9×9), then n duels on the benchmark boards, seats alternating, all
// from fixed random first placements. It returns a's share of the games either won, a duel that hits the move cap
// counting half to each.
func sovMatchScore(a, b []int16, n int) float64 {
	points := make([][2]float64, 2*n) // a's and b's points per game
	parallel(2*n, func(i int) {
		if i < n { // the long twelve-player tables first
			players, size := []int{12, 4}[i%2], []int{20, 9}[i/2%2]
			rng := core.NewRNG(uint64(i) + 1)
			seats, sw := make([]Strategy, players), make([][]int16, players)
			for p := range seats {
				seats[p] = Strategy(rng.Intn(int(Sovereign))) // any bot but Sovereign
			}
			pa := rng.Intn(players)
			pb := (pa + 1 + rng.Intn(players-1)) % players
			seats[pa], seats[pb], sw[pa], sw[pb] = Sovereign, Sovereign, a, b
			switch match(seats, size, uint64(i)+1, false, nil, sw...) {
			case pa:
				points[i][0] = 1
			case pb:
				points[i][1] = 1
			}
			return
		}
		j := i - n
		me := j % 2
		sw := [][]int16{b, b}
		sw[me] = a
		switch winner := match([]Strategy{Sovereign, Sovereign}, bigFirst(j, n), uint64(j/2)+1, false, nil, sw...); {
		case winner == -1:
			points[i] = [2]float64{0.5, 0.5}
		default:
			points[i][winner^me] = 1 // index 0 when a won
		}
	})
	var mine, theirs float64
	for _, p := range points {
		mine, theirs = mine+p[0], theirs+p[1]
	}
	if mine+theirs == 0 {
		return 0.5
	}
	return mine / (mine + theirs)
}

// sovSaveWeights writes q as Go source to name.
func sovSaveWeights(name string, q []int16) error {
	var b strings.Builder
	b.WriteString("// Code generated by TestTrainSovereign; DO NOT EDIT.\n\npackage ai\n\nvar sovereignWeights = [sovParams]int16{\n")
	for i, v := range q {
		b.WriteString(strconv.Itoa(int(v)) + ",")
		if i%16 == 15 {
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
	return os.WriteFile(name, src, 0o644)
}
