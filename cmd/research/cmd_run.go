package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/sim"
)

func presetByName(n string) (experiment.Spec, error) {
	if strings.HasPrefix(n, "pivot:") { // pivot:<system>:<level>:<pool>
		var sys, lvl string
		var pool int
		parts := strings.Split(n, ":")
		if len(parts) != 4 {
			return experiment.Spec{}, fmt.Errorf("bad pivot preset %q", n)
		}
		sys, lvl = parts[1], parts[2]
		fmt.Sscanf(parts[3], "%d", &pool)
		return experiment.VPivot(sys, lvl, pool), nil
	}
	// spec-cap3 / spec-lot3 [-ix] [:<system>:<level>:<pool>] - the written rules with the 3-lots-per-stock cap the maintainer named.
	//   cap3: sell the whole position at +5% over average cost (written exit)
	//   lot3: every lot has its own +5% target from its own entry price (maintainer's variant, 2026-10-02)
	//   -ix:  also sell the whole position when the stock leaves the index (maintainer's rule, 2026-10-03)
	if strings.HasPrefix(n, "spec-cap3") || strings.HasPrefix(n, "spec-lot3") {
		parts := strings.Split(n, ":")
		base := parts[0]
		ix := strings.HasSuffix(base, "-ix")
		base = strings.TrimSuffix(base, "-ix")
		if base != "spec-cap3" && base != "spec-lot3" {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
		}
		r := sim.SpecRules()
		r.MaxLots = 3
		r.ExitOnIndexRemoval = ix
		if base == "spec-lot3" {
			r.ExitBasis = "lot"
		}
		if len(parts) == 4 {
			var pool int
			fmt.Sscanf(parts[3], "%d", &pool)
			r.Pivot = &sim.PivotRule{System: parts[1], Level: parts[2], Pool: pool}
		} else if len(parts) != 1 {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
		}
		r.Name = n
		return experiment.Spec{ID: n, Rules: r}, nil
	}
	// nsx / nsx-lot [:<system>:<level>:<pool>] - the rule set the maintainer confirmed on 2026-10-03 (strategy-flow review, round 6):
	//   no cap on how many stocks are held (cash only), at most 3 open lots per stock, one purchase a day in all (an add comes
	//   before a new stock), a stock that leaves the index is sold, capital for the slot size refreshed after every sale, no stop.
	//   nsx: the whole position sells at +5% over average cost. nsx-lot: every lot sells at +5% over its own entry price.
	if strings.HasPrefix(n, "nsx") {
		parts := strings.Split(n, ":")
		if parts[0] != "nsx" && parts[0] != "nsx-lot" && parts[0] != "nsx-noix" {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
		}
		r := sim.SpecRules()
		r.MaxStocks = 0
		r.MaxLots = 3
		r.MaxBuysPerDay = 1
		r.ExitOnIndexRemoval = true
		r.Revision = "sale"
		if parts[0] == "nsx-lot" {
			r.ExitBasis = "lot"
		}
		if parts[0] == "nsx-noix" { // illustration only: the confirmed rules without the index-exit rule
			r.ExitOnIndexRemoval = false
		}
		if len(parts) == 4 {
			var pool int
			fmt.Sscanf(parts[3], "%d", &pool)
			r.Pivot = &sim.PivotRule{System: parts[1], Level: parts[2], Pool: pool}
		} else if len(parts) != 1 {
			return experiment.Spec{}, fmt.Errorf("bad preset %q", n)
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
	if strings.HasPrefix(n, "appv:") { // appv:<system>:<level>:<pool> - the app V-Pivot rules (5%, capital/5) with another pivot setting
		parts := strings.Split(n, ":")
		if len(parts) != 4 {
			return experiment.Spec{}, fmt.Errorf("bad sxv preset %q", n)
		}
		sp, err := experiment.FromPreset("app-vpivot-midcap50")
		if err != nil {
			return sp, err
		}
		var pool int
		fmt.Sscanf(parts[3], "%d", &pool)
		sp.Rules.Pivot = &sim.PivotRule{System: parts[1], Level: parts[2], Pool: pool}
		sp.Rules.Name = n
		sp.ID = n
		return sp, nil
	}
	switch n {
	case "app-approx", "app-exact", "app-vpivot-nifty50", "app-vpivot-midcap50", "rotation-n50":
		return experiment.FromPreset(n)
	case "legacy":
		return experiment.Standard(), nil
	case "spec":
		return experiment.StandardSpec(), nil
	case "cam-s1-5":
		return experiment.VPivot("camarilla", "S1", 5), nil
	case "fib-s1-15":
		return experiment.VPivot("fibonacci", "S1", 15), nil
	case "cam-s2-5":
		return experiment.VPivot("camarilla", "S2", 5), nil
	case "cam-closest-5":
		return experiment.VPivot("camarilla", "closest", 5), nil
	case "spec-cam-s1-5":
		return experiment.VPivotSpec("camarilla", "S1", 5), nil
	case "spec-fib-s1-15":
		return experiment.VPivotSpec("fibonacci", "S1", 15), nil
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
