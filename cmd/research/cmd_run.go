package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/sim"
)

func presetByName(n string) (experiment.Spec, error) {
	// spec-cap3 / spec-lot3 [-ix] - the written rules with the 3-lots-per-stock cap the maintainer named.
	//   cap3: sell the whole position at +5% over average cost (written exit)
	//   lot3: every lot has its own +5% target from its own entry price (maintainer's variant, 2026-10-02)
	//   -ix:  also sell the whole position when the stock leaves the index (maintainer's rule, 2026-10-03)
	if strings.HasPrefix(n, "spec-cap3") || strings.HasPrefix(n, "spec-lot3") {
		ix := strings.HasSuffix(n, "-ix")
		base := strings.TrimSuffix(n, "-ix")
		if base != "spec-cap3" && base != "spec-lot3" {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
		}
		r := sim.SpecRules()
		r.MaxLots = 3
		r.ExitOnIndexRemoval = ix
		if base == "spec-lot3" {
			r.ExitBasis = "lot"
		}
		r.Name = n
		return experiment.Spec{ID: n, Rules: r}, nil
	}
	// nsx / nsx-lot - the rule set the maintainer confirmed on 2026-10-03 (strategy-flow review, round 6):
	//   no cap on how many stocks are held (cash only), at most 3 open lots per stock, one purchase a day in all (an add comes
	//   before a new stock), a stock that leaves the index is sold, capital for the slot size refreshed after every sale, no stop.
	//   nsx: the whole position sells at +5% over average cost. nsx-lot: every lot sells at +5% over its own entry price.
	if strings.HasPrefix(n, "nsx") {
		if n != "nsx" && n != "nsx-lot" && n != "nsx-noix" {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
		}
		r := sim.SpecRules()
		r.MaxStocks = 0
		r.MaxLots = 3
		r.MaxBuysPerDay = 1
		r.ExitOnIndexRemoval = true
		r.Revision = "sale"
		if n == "nsx-lot" {
			r.ExitBasis = "lot"
		}
		if n == "nsx-noix" { // illustration only: the confirmed rules without the index-exit rule
			r.ExitOnIndexRemoval = false
		}
		r.Name = n
		return experiment.Spec{ID: n, Rules: r}, nil
	}
	if strings.HasPrefix(n, "exit:") { // exit:<stop>:<timeDays>:<target> on the documented (spec) rules, as in research/rules/sweep-exit-grid.json
		parts := strings.Split(n, ":")
		if len(parts) != 4 {
			return experiment.Spec{}, fmt.Errorf("bad exit preset %q", n)
		}
		r := sim.SpecRules()
		fmt.Sscanf(parts[1], "%g", &r.StopLoss)
		fmt.Sscanf(parts[2], "%d", &r.TimeStopDays)
		fmt.Sscanf(parts[3], "%g", &r.ProfitTarget)
		r.Name = n
		return experiment.Spec{ID: n, Rules: r}, nil
	}
	switch n {
	case "app-approx", "app-exact", "rotation-n50":
		return experiment.FromPreset(n)
	case "legacy":
		return experiment.Standard(), nil
	case "spec":
		return experiment.StandardSpec(), nil
	}
	return experiment.Spec{}, fmt.Errorf("unknown preset %q", n)
}

// cmdRun runs named presets over a window at all four stages and prints a metrics table.
func cmdRun(c common, variants string, dividends bool, stages, out string) error {
	env, mem, err := buildEnv(c)
	if err != nil {
		return err
	}
	fmt.Printf("window %s: %d trading days, %d symbols with data, %d without (%v)\n", env.Window, len(env.Panel.Dates), len(env.Panel.Series), len(env.Panel.Missing), env.Panel.Missing)
	_ = mem
	dir := monthlyDir(c, out, env)
	printHeader()
	var first, last = env.Panel.Dates[0], env.Panel.Dates[len(env.Panel.Dates)-1]
	for _, name := range strings.Split(variants, ",") {
		spec, err := presetByName(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		spec.Rules.Dividends = dividends
		for _, st := range experiment.Stages {
			if !strings.Contains(stages, string(st)) {
				continue
			}
			o, err := env.Run(spec, st)
			if err != nil {
				return err
			}
			printRow(o)
			if _, err := writeMonthly(dir, env, o); err != nil {
				return err
			}
		}
	}
	benchRows(env, first, last)
	fmt.Fprintln(os.Stderr, "monthly tables:", dir)
	return nil
}

// cmdScore prints full scorecards (and exports CSVs for verification) for one preset at the given stages.
func cmdScore(c common, variants string, dividends bool, stages, export, out string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	dir := monthlyDir(c, out, env)
	defer fmt.Fprintln(os.Stderr, "monthly tables:", dir)
	for _, variant := range strings.Split(variants, ",") {
		spec, err := presetByName(strings.TrimSpace(variant))
		if err != nil {
			return err
		}
		spec.Rules.Dividends = dividends
		for _, st := range experiment.Stages {
			if !strings.Contains(stages, string(st)) {
				continue
			}
			o, err := env.Run(spec, st)
			if err != nil {
				return err
			}
			printScorecard(o)
			if _, err := writeMonthly(dir, env, o); err != nil {
				return err
			}
			if export != "" {
				if err := exportOutcome(export, o); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
