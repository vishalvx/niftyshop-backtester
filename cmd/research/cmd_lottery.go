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

	"github.com/vishalvx/back-tester/internal/analytics"
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
	Fund        *buyHoldCAGRs // the -fund benchmark over the same dates, nil without one
}

// buyHoldCAGRs is a buy-and-hold benchmark's CAGR before tax and after tax at dated and today's rates.
type buyHoldCAGRs struct{ Gross, NetDated, NetToday float64 }

func buyHold(series []analytics.Point, s, e time.Time, capital float64) (buyHoldCAGRs, error) {
	d, err := bench.BuyHold(series, s, e, capital, costs.Dated)
	if err != nil {
		return buyHoldCAGRs{}, err
	}
	t, err := bench.BuyHold(series, s, e, capital, costs.Today)
	if err != nil {
		return buyHoldCAGRs{}, err
	}
	return buyHoldCAGRs{Gross: d.GrossCAGR, NetDated: d.NetCAGR, NetToday: t.NetCAGR}, nil
}

// on returns the benchmark CAGR on the same accounting basis as a strategy stage: before tax for the gross and cost
// stages, after tax at dated or today's rates for the tax stages.
func (b buyHoldCAGRs) on(st experiment.Stage) float64 {
	switch st {
	case experiment.TaxDated:
		return b.NetDated
	case experiment.TaxToday:
		return b.NetToday
	}
	return b.Gross
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
// distribution instead of one lucky or unlucky path. Every window is compared with the index (and, with fund set, a
// second index held as a fund less fundFee a year) bought and held over the same dates, on the same tax basis.
func cmdLottery(c common, variants string, horizonYears int, outDir, stageList, fund string, fundFee float64) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = ""
	var fundTRI []analytics.Point
	fundName := ""
	if fund != "" {
		path := ".research-data/indices/" + fund + "_TRI.json"
		if fundTRI, err = bench.LoadTRI(path); err != nil {
			return err
		}
		fundTRI = bench.LessFee(fundTRI, fundFee)
		fundName = feeName(triName(path), fundFee)
	}
	first, last := d(c.Start), d(c.End)
	var starts []time.Time
	for s := first; !s.AddDate(horizonYears, 0, 0).After(last); s = s.AddDate(0, 1, 0) {
		starts = append(starts, s)
	}
	var stages []experiment.Stage
	for _, st := range experiment.Stages {
		if strings.Contains(stageList, string(st)) {
			stages = append(stages, st)
		}
	}
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
					if g, err := buyHold(env.TRI, s, e, env.Capital); err == nil {
						r.BenchGross, r.BenchNetDat, r.BenchNetTod = g.Gross, g.NetDated, g.NetToday
					}
					if fundTRI != nil {
						g, err := buyHold(fundTRI, s, e, env.Capital)
						if err != nil {
							firstErr = fmt.Errorf("fund %s: %w", fund, err)
							return
						}
						r.Fund = &g
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
			if r.CAGR > r.index().on(k.s) {
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
	}{{env.TRIName + " buy-and-hold (gross)", bg}, {env.TRIName + " buy-and-hold (after tax, dated)", bn}, {env.TRIName + " buy-and-hold (after tax, today's rates)", bt}} {
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
	printGaps(keys, groups, env.TRIName, fundName)
	if outDir != "" {
		os.MkdirAll(outDir, 0o755)
		f, _ := os.Create(fmt.Sprintf("%s/lottery_%s_%dy.csv", outDir, c.Universe, horizonYears))
		w := csv.NewWriter(f)
		w.Write([]string{"variant", "stage", "start", "end", "cagr", "maxdd", "sharpe", "bench_gross", "bench_net_dated", "bench_net_today", "fund_gross", "fund_net_dated", "fund_net_today"})
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
			fg, fd, ft := "", "", ""
			if r.Fund != nil {
				fg, fd, ft = ff(r.Fund.Gross), ff(r.Fund.NetDated), ff(r.Fund.NetToday)
			}
			w.Write([]string{r.Variant, string(r.Stage), r.Start.Format("2006-01-02"), r.End.Format("2006-01-02"), ff(r.CAGR), ff(r.MaxDD), ff(r.Sharpe), ff(r.BenchGross), ff(r.BenchNetDat), ff(r.BenchNetTod), fg, fd, ft})
		}
		w.Flush()
		f.Close()
	}
	return nil
}

func (r lotteryRun) index() buyHoldCAGRs {
	return buyHoldCAGRs{Gross: r.BenchGross, NetDated: r.BenchNetDat, NetToday: r.BenchNetTod}
}

// printGaps prints, per variant and stage, the window-by-window gap to each benchmark over the same dates, in points a
// year on the same tax basis: the median, the 10th percentile and the worst window, and the share of windows ahead.
func printGaps[K comparable](keys []K, groups map[K][]lotteryRun, indexName, fundName string) {
	fmt.Printf("\n## Gap to the benchmarks, same dates (variant CAGR minus benchmark CAGR, points a year, same tax basis)\n\n")
	head := fmt.Sprintf("| variant | stage | median gap to %s | p10 | worst |", indexName)
	sep := "|---|---|---|---|---|"
	if fundName != "" {
		head += fmt.Sprintf(" median gap to %s | p10 | worst | beats fund |", fundName)
		sep += "---|---|---|---|"
	}
	fmt.Println(head)
	fmt.Println(sep)
	pts := func(x float64) string { return fmt.Sprintf("%+.2f", x*100) }
	for _, k := range keys {
		g := groups[k]
		if len(g) == 0 {
			continue
		}
		st := g[0].Stage
		var gi, gf []float64
		beats := 0
		for _, r := range g {
			gi = append(gi, r.CAGR-r.index().on(st))
			if r.Fund != nil {
				d := r.CAGR - r.Fund.on(st)
				gf = append(gf, d)
				if d > 0 {
					beats++
				}
			}
		}
		line := fmt.Sprintf("| %s | %s | %s | %s | %s |", g[0].Variant, st, pts(quantile(gi, .5)), pts(quantile(gi, .1)), pts(quantile(gi, 0)))
		if fundName != "" {
			line += fmt.Sprintf(" %s | %s | %s | %s |", pts(quantile(gf, .5)), pts(quantile(gf, .1)), pts(quantile(gf, 0)), pct(float64(beats)/float64(len(gf))))
		}
		fmt.Println(line)
	}
}
