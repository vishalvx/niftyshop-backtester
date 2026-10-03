package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/experiment"
)

// cmdWFVariants is a walk-forward over a short, pre-declared list of variants: pick the best Sharpe on the training
// window (after costs), run every variant on the next unseen window (after costs and tax at dated rates), and stitch
// the unseen windows. It also prints the probabilistic and deflated Sharpe ratio of every variant over the whole
// window, charged for `trials` trials.
func cmdWFVariants(c common, variants string, trainYears, testYears, trials int) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = ""
	var ts []experiment.Trial
	for _, v := range strings.Split(variants, ",") {
		sp, err := presetByName(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		sp.Rules.Dividends = true
		ts = append(ts, experiment.Trial{Spec: sp})
	}
	first, last := d(c.Start), d(c.End)
	fmt.Printf("# Walk-forward over %d variants: %s %s..%s, train %dy / test %dy, selection = best Sharpe after costs on the training window, evaluated after costs and dated tax; benchmark %s\n\n", len(ts), c.Universe, c.Start, c.End, trainYears, testYears, benchName(c))
	fmt.Println("| train | test | chosen | test CAGR chosen | test CAGR median variant | test CAGR best in hindsight | benchmark (after tax) | chosen beats benchmark |")
	fmt.Println("|---|---|---|---|---|---|---|---|")
	logP := map[string]float64{"chosen": 1, "median": 1, "hindsight": 1, "benchmark": 1}
	fixed := make([]float64, len(ts))
	for i := range fixed {
		fixed[i] = 1
	}
	var yrs float64
	wins, nwin := 0, 0
	for tr := first; ; tr = tr.AddDate(testYears, 0, 0) {
		trainFrom, trainTo := tr, tr.AddDate(trainYears, 0, -1)
		testFrom, testTo := trainTo.AddDate(0, 0, 1), trainTo.AddDate(testYears, 0, 0)
		if testTo.After(last) {
			testTo = last
		}
		if !testFrom.Before(last) || testTo.Sub(testFrom).Hours()/24 < 300 {
			break
		}
		trainRes, err := env.RunTrials(ts, experiment.Costed, experiment.Window{From: trainFrom, To: trainTo})
		if err != nil {
			return err
		}
		best := 0
		for i, o := range trainRes {
			if o.Sharpe > trainRes[best].Sharpe {
				best = i
			}
		}
		testRes, err := env.RunTrials(ts, experiment.TaxDated, experiment.Window{From: testFrom, To: testTo})
		if err != nil {
			return err
		}
		var cg []float64
		hind := math.Inf(-1)
		for i, o := range testRes {
			cg = append(cg, o.CAGR)
			hind = math.Max(hind, o.CAGR)
			_ = i
		}
		bm, err := bench.BuyHold(env.TRI, testFrom, testTo, env.Capital, costs.Dated)
		if err != nil {
			return err
		}
		years := testTo.Sub(testFrom).Hours() / (24 * 365.25)
		yrs += years
		acc := func(k string, x float64) { logP[k] *= math.Pow(1+x, years) }
		acc("chosen", testRes[best].CAGR)
		acc("median", median(cg))
		acc("hindsight", hind)
		acc("benchmark", bm.NetCAGR)
		for i := range ts {
			fixed[i] *= math.Pow(1+testRes[i].CAGR, years)
		}
		nwin++
		beat := testRes[best].CAGR > bm.NetCAGR
		if beat {
			wins++
		}
		fmt.Printf("| %s..%s | %s..%s | %s | %s | %s | %s | %s | %v |\n", trainFrom.Format("2006-01"), trainTo.Format("2006-01"), testFrom.Format("2006-01"), testTo.Format("2006-01"),
			ts[best].Spec.ID, pct(testRes[best].CAGR), pct(median(cg)), pct(hind), pct(bm.NetCAGR), beat)
		if testTo.Equal(last) {
			break
		}
	}
	fmt.Printf("\nStitched out-of-sample record over %.1f years (after costs and dated tax): chosen %s | median variant %s | best in hindsight %s | benchmark %s | chosen beat the benchmark in %d of %d test windows\n", yrs,
		pct(math.Pow(logP["chosen"], 1/yrs)-1), pct(math.Pow(logP["median"], 1/yrs)-1), pct(math.Pow(logP["hindsight"], 1/yrs)-1), pct(math.Pow(logP["benchmark"], 1/yrs)-1), wins, nwin)
	fmt.Println("\nEvery fixed variant over the same unseen windows (after costs and dated tax):")
	for i := range ts {
		fmt.Printf("  %-28s %s\n", ts[i].Spec.ID, pct(math.Pow(fixed[i], 1/yrs)-1))
	}

	// Probabilistic and deflated Sharpe over the whole window, after costs.
	whole, err := env.RunTrials(ts, experiment.Costed, experiment.Window{From: first, To: last})
	if err != nil {
		return err
	}
	var srs []float64
	rets := make([][]float64, len(whole))
	for i, o := range whole {
		rets[i] = experiment.DailyReturns(o)
		srs = append(srs, experiment.SharpePeriod(rets[i], 0.06/252))
	}
	_, sd := meanStdF(srs)
	fmt.Printf("\nSharpe over the whole window, after costs; PSR = P(true Sharpe > 0); DSR = same, charged for N = %d trials (cross-variant Sharpe sd %.4f per day, expected best-of-N Sharpe %.3f per day = %.2f a year):\n", trials, sd, experiment.ExpectedMaxSharpe(trials, sd*sd), experiment.ExpectedMaxSharpe(trials, sd*sd)*math.Sqrt(252))
	idx := make([]int, len(ts))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return srs[idx[a]] > srs[idx[b]] })
	fmt.Println("| variant | Sharpe a year | PSR | DSR (N trials) |")
	fmt.Println("|---|---|---|---|")
	for _, i := range idx {
		sk, ku := experiment.SkewKurt(rets[i])
		psr := experiment.ProbabilisticSharpe(srs[i], 0, len(rets[i]), sk, ku)
		dsr := experiment.DeflatedSharpe(srs[i], sd*sd, trials, len(rets[i]), sk, ku)
		fmt.Printf("| %s | %.2f | %.3f | %.3f |\n", ts[i].Spec.ID, srs[i]*math.Sqrt(252), psr, dsr)
	}
	return nil
}

func benchName(c common) string {
	if c.Bench != "" {
		return c.Bench + " TRI"
	}
	return c.Universe + "'s own TRI"
}
