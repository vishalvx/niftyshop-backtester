package main

import (
	"fmt"
	"math"
	"time"

	"github.com/vishalvx/back-tester/internal/panel"
	"github.com/vishalvx/back-tester/internal/sim"
)

const weightsDir = "internal/data/"

type reproJob struct {
	univ, weights string
	start         string
	pivot         *sim.PivotRule
	label         string
	published     string
}

// cmdRepro re-runs the numbers published in summary.md through the research simulator twice: once with the production
// engine's own file-selection rule (to prove the simulator and the diagnosis are right) and once with one clean file per
// symbol (to show how much the data layer alone moves the answer).
func cmdRepro(legacyDir, dataDir, cacheDir string) error {
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	jobs := []reproJob{
		{"nifty50", "nifty50_weights.csv", "2021-01-01", nil, "Standard", "12.78%"},
		{"nifty50", "nifty50_weights.csv", "2021-01-01", &sim.PivotRule{System: "camarilla", Level: "S1", Pool: 5}, "V-Pivot Camarilla S1 pool 5", "16.27%"},
		{"niftymidcap50", "niftymidcap50_weights.csv", "2021-01-01", nil, "Standard", "15.50%"},
		{"niftymidcap50", "niftymidcap50_weights.csv", "2021-01-01", &sim.PivotRule{System: "fibonacci", Level: "S1", Pool: 15}, "V-Pivot Fibonacci S1 pool 15", "24.34%"},
		{"niftysmallcap50", "niftysmallcap50_weights.csv", "2021-01-01", nil, "Standard", "19.19%"},
		{"niftysmallcap50", "niftysmallcap50_weights.csv", "2021-01-01", &sim.PivotRule{System: "camarilla", Level: "S2", Pool: 5}, "V-Pivot Camarilla S2 pool 5", "18.57%"},
		{"nifty500momentum50", "nifty500momentum50_weights.csv", "2024-06-04", nil, "Standard", "18.24%"},
		{"nifty500momentum50", "nifty500momentum50_weights.csv", "2024-06-04", &sim.PivotRule{System: "camarilla", Level: "closest", Pool: 5}, "V-Pivot Camarilla closest pool 5", "17.43%"},
		{"nifty50", "nifty50_weights.csv", "2018-01-01", nil, "Standard, 8-year window", "7.84%"},
		{"nifty50", "nifty50_weights.csv", "2018-01-01", &sim.PivotRule{System: "camarilla", Level: "S1", Pool: 5}, "V-Pivot Camarilla S1 pool 5, 8-year window", "16.01%"},
	}
	fmt.Printf("%-20s %-44s %-9s | %-28s | %-28s\n", "universe", "strategy (window start -> 2025-12-31)", "published", "engine's own file rule", "one clean file per symbol")
	fmt.Printf("%-20s %-44s %-9s | %12s %8s %6s | %12s %8s %6s\n", "", "", "CAGR", "final Rs", "CAGR", "events", "final Rs", "CAGR", "events")
	for _, j := range jobs {
		s, _ := time.Parse("2006-01-02", j.start)
		var cells [2]string
		for k, mode := range []string{"legacy", "clean"} {
			o := panel.Options{WeightsFile: weightsDir + j.weights, Start: s, End: end, MAWindow: 20, Quarantine: map[string]string{}}
			if mode == "legacy" {
				o.LegacyGlobDir = legacyDir
			} else {
				o.DataDir, o.CacheDir, o.AliasFile = dataDir, cacheDir, "research/symbol_aliases.json"
			}
			p, _, err := panel.Build(o)
			if err != nil {
				return err
			}
			r := sim.LegacyRules()
			r.Pivot = j.pivot
			res, err := sim.Run(p, r)
			if err != nil {
				return err
			}
			years := end.Sub(s).Hours() / (24 * 365.25)
			cagr := (math.Pow(res.FinalEquity/res.StartCap, 1/years) - 1) * 100
			first := ""
			if len(res.Events) > 0 {
				first = res.Events[0].Date.Format("2006-01")
			}
			cells[k] = fmt.Sprintf("%12.2f %7.2f%% %6d first trade %s", res.FinalEquity, cagr, len(res.Events), first)
		}
		fmt.Printf("%-20s %-44s %-9s | %s | %s\n", j.univ, j.label+" "+j.start[:4], j.published, cells[0], cells[1])
	}
	return nil
}
