package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/sim"
)

type tweak struct {
	Name string
	Desc string
	Fn   func(r *sim.Rules)
	Lag  int
}

// cmdAudit measures what each engine behaviour is worth by switching one thing at a time, on one window.
func cmdAudit(c common) error {
	tweaks := []tweak{
		{"A0 engine as built (own data)", "legacy semantics, dividends credited so that it is comparable to the TRI", func(r *sim.Rules) { r.Dividends = true }, 0},
		{"A1 engine accounting (no dividends)", "what the published CAGR measures: Close is not dividend adjusted", func(r *sim.Rules) { r.Dividends = false }, 0},
		{"A2 exit on average cost (spec)", "sell whole position at +5% over weighted average cost", func(r *sim.Rules) { r.Dividends = true; r.ExitBasis = "avgcost" }, 0},
		{"A3 average vs most recent lot (spec)", "add only when 3% below the LATEST lot", func(r *sim.Rules) { r.Dividends = true; r.AvgBasis = "lastlot" }, 0},
		{"A4 enforce max 5 stocks (spec)", "config max_stocks is never read by the engine", func(r *sim.Rules) { r.Dividends = true; r.MaxStocks = 5 }, 0},
		{"A5 cap 3 lots per stock (app)", "the app allows 1 fresh + 2 averages", func(r *sim.Rules) { r.Dividends = true; r.MaxLots = 3 }, 0},
		{"A6 all spec rules together", "A2 + A3 + A4", func(r *sim.Rules) { r.Dividends = true; r.ExitBasis, r.AvgBasis, r.MaxStocks = "avgcost", "lastlot", 5 }, 0},
		{"A7 fill at next open", "signal on close, fill next session open", func(r *sim.Rules) { r.Dividends = true; r.FillNextOpen = true }, 0},
		{"A8 membership known one month late", "use last month's index snapshot (no look-ahead)", func(r *sim.Rules) { r.Dividends = true }, 1},
		{"A9 idle cash earns 6%", "engine leaves idle cash at 0%", func(r *sim.Rules) { r.Dividends = true; r.CashYield = 0.06 }, 0},
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "tweak\tCAGR\tmaxDD\tSharpe\tclosed lots\twin(closed)\twin(all)\topen lots\tavg invested\tfinal Rs\tskipped(noCash/zeroUnit/maxStk)\n")
	for _, tw0 := range tweaks {
		cc := c
		cc.Lag = tw0.Lag
		env, _, err := buildEnv(cc)
		if err != nil {
			return err
		}
		spec := experiment.Standard()
		spec.ID = strings.Fields(tw0.Name)[0]
		tw0.Fn(&spec.Rules)
		o, err := env.Run(spec, experiment.Gross)
		if err != nil {
			return err
		}
		m := o.Metrics
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\t%d\t%s\t%s\t%d\t%s\t%.0f\t%d/%d/%d\n", tw0.Name, pct(m.CAGR), pct(m.MaxDD.Depth), m.Sharpe, m.ClosedLots, pct(m.WinRateClosed), pct(m.WinRateAll), m.OpenLots, pct(m.AvgInvestedPct), o.Final,
			o.Result.Skipped.NoCash, o.Result.Skipped.ZeroUnits, o.Result.Skipped.MaxStocks)
	}
	return tw.Flush()
}
