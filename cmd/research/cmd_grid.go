package main

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/vishalvx/back-tester/internal/experiment"
)

// cmdGrid runs the whole 39-point pivot grid through the start-date lottery so that every configuration is judged by
// its distribution of outcomes, then prints the neighbourhood as a matrix.
func cmdGrid(c common, horizon int, stage string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = ""
	first, last := d(c.Start), d(c.End)
	var starts []time.Time
	for s := first; !s.AddDate(horizon, 0, 0).After(last); s = s.AddDate(0, 1, 0) {
		starts = append(starts, s)
	}
	grid := experiment.PivotGrid(experiment.VPivot)
	grid = append([]experiment.Spec{experiment.Standard()}, grid...)
	type cell struct{ med, p10, p90, ge15 float64 }
	res := map[string]cell{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for _, sp := range grid {
		sp := sp
		sp.Rules.Dividends = true
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var vals []float64
			for _, s := range starts {
				x := sp
				x.Rules.From, x.Rules.To = s, s.AddDate(horizon, 0, 0)
				o, err := env.Run(x, experiment.Stage(stage))
				if err != nil {
					return
				}
				vals = append(vals, o.CAGR)
			}
			sort.Float64s(vals)
			ge := 0
			for _, v := range vals {
				if v >= 0.15 {
					ge++
				}
			}
			mu.Lock()
			res[sp.ID] = cell{quantile(vals, .5), quantile(vals, .1), quantile(vals, .9), float64(ge) / float64(len(vals))}
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("# Pivot-grid neighbourhood: %s, %d-year windows starting monthly %s..%s (%d starts), stage %s. Cell = median CAGR [p10 .. p90]\n\n", c.Universe, horizon, c.Start, c.End, len(starts), stage)
	b := res["standard-legacy"]
	fmt.Printf("Reference, no pivot filter (standard-legacy): median %s [%s .. %s], share >= 15%%: %s\n\n", pct(b.med), pct(b.p10), pct(b.p90), pct(b.ge15))
	fmt.Println("| system / level | pool 5 | pool 10 | pool 15 |")
	fmt.Println("|---|---|---|---|")
	rows := []struct{ sys, lvl string }{{"classic", "S1"}, {"classic", "S2"}, {"classic", "S3"}, {"classic", "closest"}, {"fibonacci", "S1"}, {"fibonacci", "S2"}, {"fibonacci", "S3"}, {"fibonacci", "closest"},
		{"camarilla", "S1"}, {"camarilla", "S2"}, {"camarilla", "S3"}, {"camarilla", "S4"}, {"camarilla", "closest"}}
	for _, r := range rows {
		fmt.Printf("| %s %s |", r.sys, r.lvl)
		for _, pool := range []int{5, 10, 15} {
			x, ok := res[fmt.Sprintf("vpivot-%s-%s-%d", r.sys, r.lvl, pool)]
			if !ok {
				fmt.Printf(" - |")
				continue
			}
			fmt.Printf(" %s [%s .. %s] |", pct(x.med), pct(x.p10), pct(x.p90))
		}
		fmt.Println()
	}
	var meds []float64
	for k, v := range res {
		if k != "standard-legacy" {
			meds = append(meds, v.med)
		}
	}
	sort.Float64s(meds)
	fmt.Printf("\nAcross the 39 pivot configurations: best median %s, worst %s, spread %s, median of medians %s\n", pct(meds[len(meds)-1]), pct(meds[0]), pct(meds[len(meds)-1]-meds[0]), pct(meds[len(meds)/2]))
	return nil
}
