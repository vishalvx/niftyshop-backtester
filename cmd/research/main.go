// Command research is the scratch experiment harness for the long-run study (see research/REPORT notes).
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: research <score|run|lottery|...> [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	data := fs.String("data", "", "price directory (default: .research-data/nse/prices for nse universes, .research-data/yahoo for yahoo ones)")
	cache := fs.String("cache", ".research-data/yahoo_cache", "fallback cache directory")
	c := common{}
	fs.StringVar(&c.Universe, "universe", "nifty50", "universe")
	fs.StringVar(&c.Start, "start", "2008-01-01", "window start")
	fs.StringVar(&c.End, "end", "2025-08-31", "window end")
	fs.Float64Var(&c.Capital, "capital", 1000000, "starting capital in rupees")
	fs.IntVar(&c.Lag, "lag", 0, "membership lag in months (0 = engine behaviour)")
	fs.StringVar(&c.Scenario, "scenario", "base", "cost scenario: base|harsh")
	fs.StringVar(&c.LogPath, "log", "research/out/experiment-log.jsonl", "experiment log")
	fs.Float64Var(&c.Rf, "rf", 0.06, "annual risk-free rate for Sharpe/Sortino/alpha")
	fs.StringVar(&c.Bench, "bench", "", "benchmark TRI file stem (default: the universe's own index), e.g. NIFTY_50")
	fs.Float64Var(&c.BenchFee, "bench-fee", 0, "yearly fee taken off the benchmark, e.g. 0.002 for a fund with a 0.20% expense ratio")
	fund := fs.String("fund", "", "lottery: a second benchmark TRI file stem, e.g. NIFTY200_MOMENTUM_30, held as a fund")
	fundFee := fs.Float64("fund-fee", 0, "lottery: yearly fee taken off the -fund series, e.g. 0.002")
	variants := fs.String("variants", "nsx,nsx-lot", "comma separated presets")
	stages := fs.String("stages", "gross,cost,tax-dated,tax-today", "stages to run")
	tuneFrom := fs.String("tune-from", "2008-01-01", "tuning window start")
	tuneTo := fs.String("tune-to", "2016-12-31", "tuning window end")
	testFrom := fs.String("test-from", "2017-01-01", "test window start")
	testTo := fs.String("test-to", "2025-08-31", "test window end")
	score := fs.String("score", "sharpe", "selection score: sharpe|calmar|cagr")
	robust := fs.Bool("robust", false, "score = median over 9 start dates")
	outDir := fs.String("out", "research/out", "output directory (run and score write month-by-month tables under <out>/monthly)")
	trainY := fs.Int("train-years", 6, "walk-forward training years")
	testY := fs.Int("test-years", 2, "walk-forward test years")
	config := fs.String("config", "research/rules/sweep-exit-grid.json", "sweep definition file")
	horizon := fs.Int("horizon", 5, "lottery horizon in years")
	export := fs.String("export", "", "directory for equity/lot CSV export")
	trials := fs.Int("trials", 207, "number of trials the deflated Sharpe ratio is charged for")
	divs := fs.Bool("dividends", true, "credit dividends on held shares")
	fs.Parse(os.Args[2:])
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	lotteryStages := "gross,cost,tax-dated" // the lottery's own default, so earlier tables reproduce
	if set["stages"] {
		lotteryStages = *stages
	}
	c.Data, c.Cache = *data, *cache
	var err error
	switch cmd {
	case "sweeplot":
		err = cmdSweepLottery(c, *config, *horizon, *stages)
	case "sweep":
		err = cmdSweep(c, *config, *stages)
	case "walkforward":
		err = cmdWalkForward(c, *trainY, *testY, *score, *stages, *robust)
	case "jitter":
		err = cmdJitter(c, *variants, *stages)
	case "exportpanel":
		err = cmdExportPanel(c, *outDir)
	case "wfvariants":
		err = cmdWFVariants(c, *variants, *trainY, *testY, *trials)
	case "refscore":
		err = cmdRefScore(c, *variants, []int{3, 5, 10})
	case "lottery":
		err = cmdLottery(c, *variants, *horizon, *outDir, lotteryStages, *fund, *fundFee)
	case "tune":
		err = cmdTune(c, *tuneFrom, *tuneTo, *testFrom, *testTo, *score, *robust, *outDir)
	case "debug":
		err = cmdDebug(c, *variants, *divs)
	case "audit":
		err = cmdAudit(c)
	case "score":
		err = cmdScore(c, *variants, *divs, *stages, *export, *outDir)
	case "run":
		err = cmdRun(c, *variants, *divs, *stages, *outDir)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
