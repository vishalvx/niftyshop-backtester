package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/sim"
)

// cmdSweepLottery judges every grid point of a sweep file by its distribution over monthly start dates, so the table
// shows how a neighbourhood behaves, not where one lucky path ended.
func cmdSweepLottery(c common, path string, horizon int, stage string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var sw sim.Sweep
	if err := json.Unmarshal(b, &sw); err != nil {
		return err
	}
	rules, err := sw.Expand()
	if err != nil {
		return err
	}
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
	type res struct {
		name          string
		med, p10, p90 float64
		ge15, neg     float64
		dd            float64
		beats         float64
	}
	out := make([]res, len(rules))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, r := range rules {
		i, r := i, r
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var vals, dds []float64
			beatN := 0
			for _, s := range starts {
				x := r
				x.From, x.To = s, s.AddDate(horizon, 0, 0)
				o, err := env.Run(experiment.Spec{ID: r.Name, Rules: x}, experiment.Stage(stage))
				if err != nil {
					return
				}
				vals = append(vals, o.CAGR)
				dds = append(dds, o.Metrics.MaxDD.Depth)
				if g, err := bench.BuyHold(env.TRI, x.From, x.To, env.Capital, costs.Dated); err == nil {
					ref := g.GrossCAGR
					switch experiment.Stage(stage) {
					case experiment.TaxDated:
						ref = g.NetCAGR
					case experiment.TaxToday:
						if t, err := bench.BuyHold(env.TRI, x.From, x.To, env.Capital, costs.Today); err == nil {
							ref = t.NetCAGR
						}
					}
					if o.CAGR > ref {
						beatN++
					}
				}
			}
			sort.Float64s(vals)
			var ge, ng int
			for _, v := range vals {
				if v >= 0.15 {
					ge++
				}
				if v < 0 {
					ng++
				}
			}
			n := float64(len(vals))
			out[i] = res{r.Name, quantile(vals, .5), quantile(vals, .1), quantile(vals, .9), float64(ge) / n, float64(ng) / n, quantile(dds, .5), float64(beatN) / n}
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].med > out[j].med })
	fmt.Printf("# Sweep %s judged by %d-year windows starting monthly %s..%s (%d starts), stage %s\n\n", sw.ID, horizon, c.Start, c.End, len(starts), stage)
	fmt.Println("| rank | variant | median CAGR | p10 | p90 | share >= 15% | share < 0 | median maxDD | beats TRI (same basis) |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|")
	for i, r := range out {
		fmt.Printf("| %d | %s | %s | %s | %s | %s | %s | %s | %s |\n", i+1, r.name, pct(r.med), pct(r.p10), pct(r.p90), pct(r.ge15), pct(r.neg), pct(r.dd), pct(r.beats))
	}
	return nil
}
