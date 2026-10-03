package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/experiment"
)

type lotteryRun struct {
	Start, End  time.Time
	Variant     string
	Stage       experiment.Stage
	CAGR        float64
	MaxDD       float64
	Sharpe      float64
	BenchGross  float64
	BenchNetDat float64
	BenchNetTod float64
}

func quantile(x []float64, q float64) float64 {
	s := append([]float64{}, x...)
	sort.Float64s(s)
	if len(s) == 0 {
		return 0
	}
	i := int(q * float64(len(s)-1))
	return s[i]
}

// cmdLottery runs fixed-horizon windows that start every month, so the answer to "what CAGR does this give" becomes a
// distribution instead of one lucky or unlucky path.
func cmdLottery(c common, variants string, horizonYears int, outDir string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = ""
	first, last := d(c.Start), d(c.End)
	var starts []time.Time
	for s := first; !s.AddDate(horizonYears, 0, 0).After(last); s = s.AddDate(0, 1, 0) {
		starts = append(starts, s)
	}
	stages := []experiment.Stage{experiment.Gross, experiment.Costed, experiment.TaxDated}
	var specs []experiment.Spec
	for _, v := range strings.Split(variants, ",") {
		sp, err := presetByName(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		sp.Rules.Dividends = true
		specs = append(specs, sp)
	}
	var mu sync.Mutex
	var runs []lotteryRun
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	var firstErr error
	for _, sp := range specs {
		for _, st := range stages {
			for _, s := range starts {
				wg.Add(1)
				sem <- struct{}{}
				go func(sp experiment.Spec, st experiment.Stage, s time.Time) {
					defer wg.Done()
					defer func() { <-sem }()
					e := s.AddDate(horizonYears, 0, 0)
					sp.Rules.From, sp.Rules.To = s, e
					o, err := env.Run(sp, st)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						firstErr = err
						return
					}
					r := lotteryRun{Start: s, End: e, Variant: sp.ID, Stage: st, CAGR: o.CAGR, MaxDD: o.Metrics.MaxDD.Depth, Sharpe: o.Sharpe}
					if g, err := bench.BuyHold(env.TRI, s, e, env.Capital, costs.Dated); err == nil {
						r.BenchGross, r.BenchNetDat = g.GrossCAGR, g.NetCAGR
					}
					if g, err := bench.BuyHold(env.TRI, s, e, env.Capital, costs.Today); err == nil {
						r.BenchNetTod = g.NetCAGR
					}
					runs = append(runs, r)
				}(sp, st, s)
			}
		}
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	fmt.Printf("# Start-date lottery: %s, %d-year horizons starting every month between %s and %s (%d windows per cell)\n\n", c.Universe, horizonYears, c.Start, c.End, len(starts))
	fmt.Println("| variant | stage | min | p10 | median | p90 | max | share >= 15% | share < 0 | median maxDD | beats TRI (same basis) |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|---|")
	type key struct {
		v string
		s experiment.Stage
	}
	groups := map[key][]lotteryRun{}
	for _, r := range runs {
		groups[key{r.Variant, r.Stage}] = append(groups[key{r.Variant, r.Stage}], r)
	}
	var keys []key
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].v != keys[j].v {
			return keys[i].v < keys[j].v
		}
		return keys[i].s < keys[j].s
	})
	for _, k := range keys {
		g := groups[k]
		var cg, dd []float64
		var ge15, lt0, beats int
		for _, r := range g {
			cg = append(cg, r.CAGR)
			dd = append(dd, r.MaxDD)
			if r.CAGR >= 0.15 {
				ge15++
			}
			if r.CAGR < 0 {
				lt0++
			}
			ref := r.BenchGross
			if k.s == experiment.TaxDated {
				ref = r.BenchNetDat
			}
			if r.CAGR > ref {
				beats++
			}
		}
		n := float64(len(g))
		fmt.Printf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", k.v, k.s, pct(quantile(cg, 0)), pct(quantile(cg, .1)), pct(quantile(cg, .5)), pct(quantile(cg, .9)), pct(quantile(cg, 1)),
			pct(float64(ge15)/n), pct(float64(lt0)/n), pct(quantile(dd, .5)), pct(float64(beats)/n))
	}
	// benchmark row
	var bg, bn, bt []float64
	seen := map[time.Time]bool{}
	for _, r := range runs {
		if seen[r.Start] {
			continue
		}
		seen[r.Start] = true
		bg = append(bg, r.BenchGross)
		bn = append(bn, r.BenchNetDat)
		bt = append(bt, r.BenchNetTod)
	}
	for _, row := range []struct {
		n string
		x []float64
	}{{"Nifty TRI buy-and-hold (gross)", bg}, {"Nifty TRI buy-and-hold (after tax, dated)", bn}, {"Nifty TRI buy-and-hold (after tax, today's rates)", bt}} {
		var ge15 int
		var lt0 int
		for _, v := range row.x {
			if v >= 0.15 {
				ge15++
			}
			if v < 0 {
				lt0++
			}
		}
		n := float64(len(row.x))
		fmt.Printf("| %s | - | %s | %s | %s | %s | %s | %s | %s | - | - |\n", row.n, pct(quantile(row.x, 0)), pct(quantile(row.x, .1)), pct(quantile(row.x, .5)), pct(quantile(row.x, .9)), pct(quantile(row.x, 1)), pct(float64(ge15)/n), pct(float64(lt0)/n))
	}
	if outDir != "" {
		os.MkdirAll(outDir, 0o755)
		f, _ := os.Create(fmt.Sprintf("%s/lottery_%s_%dy.csv", outDir, c.Universe, horizonYears))
		w := csv.NewWriter(f)
		w.Write([]string{"variant", "stage", "start", "end", "cagr", "maxdd", "sharpe", "bench_gross", "bench_net_dated", "bench_net_today"})
		sort.Slice(runs, func(i, j int) bool {
			if runs[i].Variant != runs[j].Variant {
				return runs[i].Variant < runs[j].Variant
			}
			if runs[i].Stage != runs[j].Stage {
				return runs[i].Stage < runs[j].Stage
			}
			return runs[i].Start.Before(runs[j].Start)
		})
		for _, r := range runs {
			w.Write([]string{r.Variant, string(r.Stage), r.Start.Format("2006-01-02"), r.End.Format("2006-01-02"), ff(r.CAGR), ff(r.MaxDD), ff(r.Sharpe), ff(r.BenchGross), ff(r.BenchNetDat), ff(r.BenchNetTod)})
		}
		w.Flush()
		f.Close()
	}
	return nil
}
