package experiment

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/vishalvx/back-tester/internal/sim"
)

// Trial is one variant in a tuning study together with its coordinates on the parameter axes
// (used to look at parameter neighbourhoods instead of a single best point).
type Trial struct {
	Spec   Spec
	Params map[string]float64
	Group  string
}

func withRules(id, group string, params map[string]float64, f func(r *sim.Rules)) Trial {
	r := sim.LegacyRules()
	f(&r)
	r.Name = id
	r.Dividends = true
	return Trial{Spec: Spec{ID: id, Rules: r}, Params: params, Group: group}
}

// TrialSet returns the bounded set of rule variants used to demonstrate honest tuning. Every member is a rule that
// already exists in the repository's documents or a one-line extension of it. The set is deliberately small and fixed
// before any result is seen; its size N is what the deflated Sharpe ratio is charged for.
func TrialSet() []Trial {
	var ts []Trial
	ts = append(ts, withRules("base-legacy", "base", nil, func(r *sim.Rules) {}))
	ts = append(ts, withRules("base-spec", "base", nil, func(r *sim.Rules) { r.ExitBasis, r.AvgBasis, r.MaxStocks = "avgcost", "lastlot", 5 }))
	// The existing 39-point pivot grid.
	systems := []struct {
		n string
		l []string
	}{{"classic", []string{"S1", "S2", "S3", "closest"}}, {"fibonacci", []string{"S1", "S2", "S3", "closest"}}, {"camarilla", []string{"S1", "S2", "S3", "S4", "closest"}}}
	for _, pool := range []int{5, 10, 15} {
		for _, s := range systems {
			for _, l := range s.l {
				id := fmt.Sprintf("pivot-%s-%s-%d", s.n, l, pool)
				sys, lvl, pl := s.n, l, pool
				ts = append(ts, withRules(id, "pivot", map[string]float64{"pool": float64(pl)}, func(r *sim.Rules) { r.Pivot = &sim.PivotRule{System: sys, Level: lvl, Pool: pl} }))
			}
		}
	}
	// Stop-loss on the whole position, measured from average cost (legacy semantics keep per-lot targets).
	for _, sl := range []float64{0.10, 0.15, 0.20, 0.30} {
		v := sl
		ts = append(ts, withRules(fmt.Sprintf("stop-%.0f", v*100), "stop", map[string]float64{"stop": v}, func(r *sim.Rules) { r.StopLoss = v }))
	}
	// Time stop.
	for _, d := range []int{60, 120, 180, 365} {
		v := d
		ts = append(ts, withRules(fmt.Sprintf("time-%d", v), "time", map[string]float64{"days": float64(v)}, func(r *sim.Rules) { r.TimeStopDays = v }))
	}
	// Different fixed profit targets (letting winners run further).
	for _, t := range []float64{0.03, 0.08, 0.12, 0.20} {
		v := t
		ts = append(ts, withRules(fmt.Sprintf("target-%.0f", v*100), "target", map[string]float64{"target": v}, func(r *sim.Rules) { r.ProfitTarget = v }))
	}
	// Trailing exit after the target (winners run until they fall trail% from the peak).
	for _, a := range []float64{0.05, 0.10} {
		for _, tr := range []float64{0.05, 0.10} {
			av, tv := a, tr
			ts = append(ts, withRules(fmt.Sprintf("trail-arm%.0f-%.0f", av*100, tv*100), "trail", map[string]float64{"arm": av, "trail": tv}, func(r *sim.Rules) { r.ProfitTarget = av; r.TrailPct = tv }))
		}
	}
	// Averaging trigger.
	for _, a := range []float64{0.02, 0.05, 0.07} {
		v := a
		ts = append(ts, withRules(fmt.Sprintf("avgtrig-%.0f", v*100), "avg", map[string]float64{"avg": v}, func(r *sim.Rules) { r.AvgTrigger = v }))
	}
	// Combinations: stop x target on spec-like exits (neighbourhood grid).
	for _, sl := range []float64{0.15, 0.25} {
		for _, t := range []float64{0.05, 0.10} {
			sv, tv := sl, t
			ts = append(ts, withRules(fmt.Sprintf("combo-stop%.0f-target%.0f", sv*100, tv*100), "combo", map[string]float64{"stop": sv, "target": tv}, func(r *sim.Rules) { r.StopLoss = sv; r.ProfitTarget = tv }))
		}
	}
	// Stop-loss with documented (average-cost) exit semantics.
	for _, sl := range []float64{0.15, 0.25} {
		v := sl
		ts = append(ts, withRules(fmt.Sprintf("spec-stop-%.0f", v*100), "spec", map[string]float64{"stop": v}, func(r *sim.Rules) {
			r.ExitBasis, r.AvgBasis, r.MaxStocks, r.StopLoss = "avgcost", "lastlot", 5, v
		}))
	}
	return ts
}

// Window is a dated slice of the panel.
type Window struct {
	Name     string
	From, To time.Time
}

// RunTrials runs every trial over a window at one stage, in parallel.
func (e *Env) RunTrials(ts []Trial, st Stage, w Window) ([]*Outcome, error) {
	out := make([]*Outcome, len(ts))
	errs := make([]error, len(ts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i := range ts {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			spec := ts[i].Spec
			spec.Rules.From, spec.Rules.To = w.From, w.To
			o, err := e.Run(spec, st)
			out[i], errs[i] = o, err
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DailyReturns returns the simple daily returns of an outcome's equity curve (opening value = start capital).
func DailyReturns(o *Outcome) []float64 {
	pts := o.Result.Points
	r := make([]float64, len(pts))
	prev := o.Result.StartCap
	for i, p := range pts {
		v := p.Equity
		if i == len(pts)-1 {
			v = o.Final
		}
		r[i] = v/prev - 1
		prev = v
	}
	return r
}

// ---------- statistics ----------

func meanStd(x []float64) (m, s float64) {
	if len(x) == 0 {
		return
	}
	for _, v := range x {
		m += v
	}
	m /= float64(len(x))
	if len(x) < 2 {
		return
	}
	for _, v := range x {
		s += (v - m) * (v - m)
	}
	return m, math.Sqrt(s / float64(len(x)-1))
}

// SharpePeriod is the non-annualised Sharpe ratio of a return series (excess over rfPer per period).
func SharpePeriod(r []float64, rfPer float64) float64 {
	m, s := meanStd(r)
	if s == 0 {
		return 0
	}
	return (m - rfPer) / s
}

// SkewKurt returns sample skewness and (non-excess) kurtosis.
func SkewKurt(r []float64) (skew, kurt float64) {
	m, s := meanStd(r)
	if s == 0 {
		return 0, 3
	}
	n := float64(len(r))
	s *= math.Sqrt((n - 1) / n) // population standard deviation, so the moments below are the usual third and fourth standardised moments
	var m3, m4 float64
	for _, v := range r {
		d := (v - m) / s
		m3 += d * d * d
		m4 += d * d * d * d
	}
	return m3 / n, m4 / n
}

func normCDF(x float64) float64 { return 0.5 * math.Erfc(-x/math.Sqrt2) }

// normInv is the standard normal quantile (Acklam's approximation, relative error < 1.2e-9).
func normInv(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	a := []float64{-3.969683028665376e+01, 2.209460984245205e+02, -2.759285104469687e+02, 1.383577518672690e+02, -3.066479806614716e+01, 2.506628277459239e+00}
	b := []float64{-5.447609879822406e+01, 1.615858368580409e+02, -1.556989798598866e+02, 6.680131188771972e+01, -1.328068155288572e+01}
	c := []float64{-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e+00, -2.549732539343734e+00, 4.374664141464968e+00, 2.938163982698783e+00}
	d := []float64{7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e+00, 3.754408661907416e+00}
	plow, phigh := 0.02425, 1-0.02425
	switch {
	case p < plow:
		q := math.Sqrt(-2 * math.Log(p))
		return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	case p > phigh:
		q := math.Sqrt(-2 * math.Log(1-p))
		return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	}
	q := p - 0.5
	r := q * q
	return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q / (((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1)
}

// ExpectedMaxSharpe is the Sharpe ratio one expects from the best of n skill-less trials whose Sharpe estimates have
// cross-trial variance srVar (Bailey and Lopez de Prado, 2014, eq. for SR0).
func ExpectedMaxSharpe(n int, srVar float64) float64 {
	if n < 2 {
		return 0
	}
	const euler = 0.5772156649015329
	return math.Sqrt(srVar) * ((1-euler)*normInv(1-1/float64(n)) + euler*normInv(1-1/(float64(n)*math.E)))
}

// DeflatedSharpe is the probability that the selected strategy's true Sharpe ratio exceeds zero after charging for
// n trials: PSR evaluated at the expected maximum Sharpe of n skill-less trials. sr, skew, kurt describe the selected
// strategy's per-period returns; t is their count.
func DeflatedSharpe(sr, srVar float64, n, t int, skew, kurt float64) float64 {
	sr0 := ExpectedMaxSharpe(n, srVar)
	return ProbabilisticSharpe(sr, sr0, t, skew, kurt)
}

// ProbabilisticSharpe is P(true SR > benchmark SR) given the estimate (Bailey and Lopez de Prado, 2012).
func ProbabilisticSharpe(sr, benchSR float64, t int, skew, kurt float64) float64 {
	den := 1 - skew*sr + (kurt-1)/4*sr*sr
	if den <= 0 || t < 2 {
		return math.NaN()
	}
	return normCDF((sr - benchSR) * math.Sqrt(float64(t-1)) / math.Sqrt(den))
}

// Spearman is the rank correlation of two equal-length slices.
func Spearman(x, y []float64) float64 {
	rx, ry := ranks(x), ranks(y)
	mx, sx := meanStd(rx)
	my, sy := meanStd(ry)
	if sx == 0 || sy == 0 {
		return 0
	}
	var c float64
	for i := range rx {
		c += (rx[i] - mx) * (ry[i] - my)
	}
	return c / float64(len(rx)-1) / (sx * sy)
}

func ranks(x []float64) []float64 {
	idx := make([]int, len(x))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return x[idx[a]] < x[idx[b]] })
	r := make([]float64, len(x))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && x[idx[j+1]] == x[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			r[idx[k]] = avg
		}
		i = j + 1
	}
	return r
}

// PBO estimates the probability of backtest overfitting by combinatorially symmetric cross-validation
// (Bailey, Borwein, Lopez de Prado, Zhu 2015). rets[v][t] is variant v's return in period t; the timeline is cut into s
// equal blocks (s even); for every choice of s/2 blocks as in-sample the best in-sample variant (by Sharpe) is found and its
// out-of-sample rank recorded. PBO is the share of splits where that variant falls below the out-of-sample median.
func PBO(rets [][]float64, s int) (pbo float64, splits int, meanOOSRank float64) {
	n := len(rets)
	if n < 3 || s < 4 || s%2 != 0 {
		return math.NaN(), 0, math.NaN()
	}
	t := len(rets[0])
	blk := t / s
	if blk < 2 {
		return math.NaN(), 0, math.NaN()
	}
	// per-block sums, sums of squares and counts so each split is O(n*s)
	sum := make([][]float64, n)
	sq := make([][]float64, n)
	for v := 0; v < n; v++ {
		sum[v], sq[v] = make([]float64, s), make([]float64, s)
		for b := 0; b < s; b++ {
			for _, x := range rets[v][b*blk : (b+1)*blk] {
				sum[v][b] += x
				sq[v][b] += x * x
			}
		}
	}
	sharpe := func(v int, bs []int) float64 {
		var a, q float64
		for _, b := range bs {
			a += sum[v][b]
			q += sq[v][b]
		}
		cnt := float64(len(bs) * blk)
		m := a / cnt
		va := q/cnt - m*m
		if va <= 0 {
			return 0
		}
		return m / math.Sqrt(va)
	}
	var below, total int
	var rankSum float64
	comb := make([]int, s/2)
	var rec func(start, k int)
	rec = func(start, k int) {
		if k == s/2 {
			in := append([]int{}, comb...)
			inSet := map[int]bool{}
			for _, b := range in {
				inSet[b] = true
			}
			var out []int
			for b := 0; b < s; b++ {
				if !inSet[b] {
					out = append(out, b)
				}
			}
			best, bestSR := 0, math.Inf(-1)
			for v := 0; v < n; v++ {
				if x := sharpe(v, in); x > bestSR {
					best, bestSR = v, x
				}
			}
			oos := make([]float64, n)
			for v := 0; v < n; v++ {
				oos[v] = sharpe(v, out)
			}
			rk := ranks(oos)[best] / float64(n+1) // relative rank in (0,1)
			rankSum += rk
			if rk <= 0.5 {
				below++
			}
			total++
			return
		}
		for b := start; b < s; b++ {
			comb[k] = b
			rec(b+1, k+1)
		}
	}
	rec(0, 0)
	return float64(below) / float64(total), total, rankSum / float64(total)
}
