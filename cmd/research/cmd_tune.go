package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/experiment"
)

func d(s string) time.Time { return mustDate(s) }

type trialRow struct {
	T            experiment.Trial
	TuneCAGR     float64
	TuneSharpe   float64
	TuneDD       float64
	TuneCalmar   float64
	TestCAGR     float64
	TestSharpe   float64
	TestDD       float64
	TestCalmar   float64
	TestMedSh    float64
	TuneScore    float64
	TestScore    float64
	TuneRet      []float64
	TestRet      []float64
	TuneLots     int
	TestLots     int
	TestOpenLoss float64
}

func scoreOf(name string, cagr, sharpe, calmar float64) float64 {
	switch name {
	case "cagr":
		return cagr
	case "calmar":
		return calmar
	}
	return sharpe
}

// medianOverStarts runs a trial from several start dates (same end) and returns the median of the chosen score.
func medianOverStarts(env *experiment.Env, t experiment.Trial, st experiment.Stage, from, to time.Time, offsetsMonths []int, score string) (float64, error) {
	var vals []float64
	for _, off := range offsetsMonths {
		sp := t.Spec
		sp.Rules.From, sp.Rules.To = from.AddDate(0, off, 0), to
		o, err := env.Run(sp, st)
		if err != nil {
			return 0, err
		}
		vals = append(vals, scoreOf(score, o.CAGR, o.Sharpe, o.Metrics.Calmar))
	}
	sort.Float64s(vals)
	return vals[len(vals)/2], nil
}

// cmdTune demonstrates tuning on one period and judging on an untouched later period.
func cmdTune(c common, tuneFrom, tuneTo, testFrom, testTo, score string, robust bool, outDir string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = outDir + "/experiment-log.jsonl"
	ts := experiment.TrialSet()
	st := experiment.Costed
	tuneW := experiment.Window{Name: "tune", From: d(tuneFrom), To: d(tuneTo)}
	testW := experiment.Window{Name: "test", From: d(testFrom), To: d(testTo)}
	env.Window = fmt.Sprintf("%s TUNE %s..%s", c.Universe, tuneFrom, tuneTo)
	tune, err := env.RunTrials(ts, st, tuneW)
	if err != nil {
		return err
	}
	env.Window = fmt.Sprintf("%s TEST %s..%s", c.Universe, testFrom, testTo)
	test, err := env.RunTrials(ts, st, testW)
	if err != nil {
		return err
	}
	rows := make([]trialRow, len(ts))
	for i := range ts {
		a, b := tune[i], test[i]
		rows[i] = trialRow{T: ts[i], TuneCAGR: a.CAGR, TuneSharpe: a.Sharpe, TuneDD: a.Metrics.MaxDD.Depth, TuneCalmar: a.Metrics.Calmar,
			TestCAGR: b.CAGR, TestSharpe: b.Sharpe, TestDD: b.Metrics.MaxDD.Depth, TestCalmar: b.Metrics.Calmar,
			TuneRet: experiment.DailyReturns(a), TestRet: experiment.DailyReturns(b), TuneLots: a.Metrics.ClosedLots, TestLots: b.Metrics.ClosedLots,
			TestOpenLoss: b.Metrics.OpenUnrealisedPct}
		rows[i].TuneScore = scoreOf(score, a.CAGR, a.Sharpe, a.Metrics.Calmar)
		rows[i].TestScore = scoreOf(score, b.CAGR, b.Sharpe, b.Metrics.Calmar)
	}
	if robust {
		off := []int{0, 3, 6, 9, 12, 15, 18, 21, 24}
		for i := range rows {
			v, err := medianOverStarts(env, ts[i], st, tuneW.From, tuneW.To, off, score)
			if err != nil {
				return err
			}
			rows[i].TuneScore = v
			w, err := medianOverStarts(env, ts[i], st, testW.From, testW.To, off, score)
			if err != nil {
				return err
			}
			rows[i].TestScore = w
		}
	}
	// selection
	best := 0
	for i := range rows {
		if rows[i].TuneScore > rows[best].TuneScore {
			best = i
		}
	}
	tuneScores := make([]float64, len(rows))
	testScores := make([]float64, len(rows))
	tuneSh := make([]float64, len(rows))
	for i := range rows {
		tuneScores[i], testScores[i], tuneSh[i] = rows[i].TuneScore, rows[i].TestScore, experiment.SharpePeriod(rows[i].TuneRet, 0.06/252)
	}
	rho := experiment.Spearman(tuneScores, testScores)
	// rank of the selected variant on the test score (1 = best)
	rank := 1
	for i := range rows {
		if rows[i].TestScore > rows[best].TestScore {
			rank++
		}
	}
	// deflated Sharpe on the tuning period for the selected variant
	_, sd := meanStdF(tuneSh)
	srPer := experiment.SharpePeriod(rows[best].TuneRet, 0.06/252)
	sk, ku := experiment.SkewKurt(rows[best].TuneRet)
	dsr := experiment.DeflatedSharpe(srPer, sd*sd, len(rows), len(rows[best].TuneRet), sk, ku)
	sr0 := experiment.ExpectedMaxSharpe(len(rows), sd*sd)
	// PBO over the tuning+test timeline: concatenate returns so every variant has the same length
	all := make([][]float64, len(rows))
	for i := range rows {
		all[i] = append(append([]float64{}, rows[i].TuneRet...), rows[i].TestRet...)
	}
	pbo, nsplit, mrk := experiment.PBO(all, 12)

	// base comparators
	var baseLegacy, baseSpec *trialRow
	for i := range rows {
		switch rows[i].T.Spec.ID {
		case "base-legacy":
			baseLegacy = &rows[i]
		case "base-spec":
			baseSpec = &rows[i]
		}
	}
	medTest := median(testScores)

	fmt.Printf("# Tuning study: %s, stage %s, selection score = %s%s\n", c.Universe, st, score, map[bool]string{true: " (median over 9 start dates)", false: " (single path)"}[robust])
	fmt.Printf("tuning %s..%s | untouched test %s..%s | trials N = %d | capital Rs %.0f\n\n", tuneFrom, tuneTo, testFrom, testTo, len(rows), c.Capital)
	fmt.Printf("selected on tuning: %s  tuning score %.3f (CAGR %s, Sharpe %.2f, maxDD %s)\n", rows[best].T.Spec.ID, rows[best].TuneScore, pct(rows[best].TuneCAGR), rows[best].TuneSharpe, pct(rows[best].TuneDD))
	fmt.Printf("  on the untouched test: score %.3f (CAGR %s, Sharpe %.2f, maxDD %s); rank %d of %d; median trial test score %.3f\n", rows[best].TestScore, pct(rows[best].TestCAGR), rows[best].TestSharpe, pct(rows[best].TestDD), rank, len(rows), medTest)
	if baseLegacy != nil {
		fmt.Printf("  baseline legacy: tuning %.3f -> test %.3f (CAGR %s)\n", baseLegacy.TuneScore, baseLegacy.TestScore, pct(baseLegacy.TestCAGR))
	}
	if baseSpec != nil {
		fmt.Printf("  baseline spec:   tuning %.3f -> test %.3f (CAGR %s)\n", baseSpec.TuneScore, baseSpec.TestScore, pct(baseSpec.TestCAGR))
	}
	fmt.Printf("rank correlation between tuning score and test score across all %d trials (Spearman): %.2f\n", len(rows), rho)
	fmt.Printf("deflated Sharpe ratio of the selected variant on the tuning period: per-day SR %.4f (annualised %.2f); expected best-of-%d skill-less SR0 %.4f; DSR = %.3f (probability its true Sharpe > 0 after charging for the trials)\n",
		srPer, srPer*math.Sqrt(252), len(rows), sr0, dsr)
	fmt.Printf("probability of backtest overfitting (CSCV, 12 blocks, %d splits): %.2f; mean relative OOS rank of the in-sample winner: %.2f (0.5 = no skill)\n\n", nsplit, pbo, mrk)

	// full table
	sort.Slice(rows, func(i, j int) bool { return rows[i].TuneScore > rows[j].TuneScore })
	fmt.Println("| tune rank | variant | group | tune CAGR | tune Sharpe | tune maxDD | test CAGR | test Sharpe | test maxDD | test open-lot loss/equity |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|")
	for i, r := range rows {
		fmt.Printf("| %d | %s | %s | %s | %.2f | %s | %s | %.2f | %s | %s |\n", i+1, r.T.Spec.ID, r.T.Group, pct(r.TuneCAGR), r.TuneSharpe, pct(r.TuneDD), pct(r.TestCAGR), r.TestSharpe, pct(r.TestDD), pct(r.TestOpenLoss))
	}
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		f, err := os.Create(fmt.Sprintf("%s/tune_%s_%s%s.csv", outDir, c.Universe, score, map[bool]string{true: "_robust", false: ""}[robust]))
		if err != nil {
			return err
		}
		w := csv.NewWriter(f)
		w.Write([]string{"variant", "group", "params", "tune_cagr", "tune_sharpe", "tune_maxdd", "tune_score", "test_cagr", "test_sharpe", "test_maxdd", "test_score", "tune_lots", "test_lots"})
		for _, r := range rows {
			var ps []string
			for k, v := range r.T.Params {
				ps = append(ps, fmt.Sprintf("%s=%g", k, v))
			}
			sort.Strings(ps)
			w.Write([]string{r.T.Spec.ID, r.T.Group, strings.Join(ps, ";"), ff(r.TuneCAGR), ff(r.TuneSharpe), ff(r.TuneDD), ff(r.TuneScore), ff(r.TestCAGR), ff(r.TestSharpe), ff(r.TestDD), ff(r.TestScore), strconv.Itoa(r.TuneLots), strconv.Itoa(r.TestLots)})
		}
		w.Flush()
		f.Close()
	}
	return nil
}

func ff(x float64) string { return strconv.FormatFloat(x, 'f', 5, 64) }

func meanStdF(x []float64) (float64, float64) {
	var m float64
	for _, v := range x {
		m += v
	}
	m /= float64(len(x))
	var s float64
	for _, v := range x {
		s += (v - m) * (v - m)
	}
	return m, math.Sqrt(s / float64(len(x)-1))
}

func median(x []float64) float64 {
	s := append([]float64{}, x...)
	sort.Float64s(s)
	return s[len(s)/2]
}
