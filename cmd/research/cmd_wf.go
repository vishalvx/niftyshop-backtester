package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/experiment"
)

// cmdWalkForward tunes on a rolling training window, applies the winner to the next unseen window, and stitches the
// unseen windows into one out-of-sample record.
func cmdWalkForward(c common, trainYears, testYears int, score string, stage string, robust bool) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = "research/out/experiment-log.jsonl"
	ts := experiment.TrialSet()
	first, last := d(c.Start), d(c.End)
	fmt.Printf("# Walk-forward: %s, train %dy / test %dy, step %dy, selection score %s%s, evaluated at stage %s, N = %d trials per window\n\n", c.Universe, trainYears, testYears, testYears, score,
		map[bool]string{true: " (median over 5 start dates)", false: " (single path)"}[robust], stage, len(ts))
	fmt.Println("| train | test | chosen on train | train score | test CAGR chosen | test CAGR legacy | test CAGR spec | test CAGR median trial | test CAGR Nifty TRI (gross) | chosen rank in test (of N) |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|")
	logWF := map[string]float64{"chosen": 1, "legacy": 1, "spec": 1, "median": 1, "tri": 1, "best-in-hindsight": 1}
	var yrs float64
	for tr := first; ; tr = tr.AddDate(testYears, 0, 0) {
		trainFrom, trainTo := tr, tr.AddDate(trainYears, 0, -1)
		testFrom, testTo := trainTo.AddDate(0, 0, 1), trainTo.AddDate(testYears, 0, 0)
		if testTo.After(last) {
			testTo = last
		}
		if !testFrom.Before(last) || testTo.Sub(testFrom).Hours()/24 < 300 {
			break
		}
		env.Window = fmt.Sprintf("%s WF train %s..%s", c.Universe, trainFrom.Format("2006-01-02"), trainTo.Format("2006-01-02"))
		trainRes, err := env.RunTrials(ts, experiment.Costed, experiment.Window{From: trainFrom, To: trainTo})
		if err != nil {
			return err
		}
		scores := make([]float64, len(ts))
		for i, o := range trainRes {
			scores[i] = scoreOf(score, o.CAGR, o.Sharpe, o.Metrics.Calmar)
			if robust {
				v, err := medianOverStarts(env, ts[i], experiment.Costed, trainFrom, trainTo, []int{0, 6, 12, 18, 24}, score)
				if err != nil {
					return err
				}
				scores[i] = v
			}
		}
		best := 0
		for i := range scores {
			if scores[i] > scores[best] {
				best = i
			}
		}
		env.Window = fmt.Sprintf("%s WF test %s..%s", c.Universe, testFrom.Format("2006-01-02"), testTo.Format("2006-01-02"))
		testRes, err := env.RunTrials(ts, experiment.Stage(stage), experiment.Window{From: testFrom, To: testTo})
		if err != nil {
			return err
		}
		var cagrs []float64
		var legacy, spec float64
		for i, o := range testRes {
			cagrs = append(cagrs, o.CAGR)
			switch ts[i].Spec.ID {
			case "base-legacy":
				legacy = o.CAGR
			case "base-spec":
				spec = o.CAGR
			}
		}
		rank := 1
		bestHind := math.Inf(-1)
		for i := range testRes {
			if testRes[i].CAGR > testRes[best].CAGR {
				rank++
			}
			bestHind = math.Max(bestHind, testRes[i].CAGR)
		}
		med := median(cagrs)
		mode := costs.Dated
		tri, _ := bench.BuyHold(env.TRI, testFrom, testTo, env.Capital, mode)
		years := testTo.Sub(testFrom).Hours() / (24 * 365.25)
		yrs += years
		g := func(k string, cg float64) { logWF[k] *= math.Pow(1+cg, years) }
		g("chosen", testRes[best].CAGR)
		g("legacy", legacy)
		g("spec", spec)
		g("median", med)
		g("tri", tri.GrossCAGR)
		g("best-in-hindsight", bestHind)
		fmt.Printf("| %s..%s | %s..%s | %s | %.2f | %s | %s | %s | %s | %s | %d |\n", trainFrom.Format("2006-01"), trainTo.Format("2006-01"), testFrom.Format("2006-01"), testTo.Format("2006-01"),
			ts[best].Spec.ID, scores[best], pct(testRes[best].CAGR), pct(legacy), pct(spec), pct(med), pct(tri.GrossCAGR), rank)
		if testTo.Equal(last) {
			break
		}
	}
	fmt.Printf("\nStitched out-of-sample record over %.1f years (compounded test-window returns, stage %s):\n", yrs, stage)
	var keys []string
	for k := range logWF {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-18s CAGR %s\n", k, pct(math.Pow(logWF[k], 1/yrs)-1))
	}
	_ = time.Now
	return nil
}
