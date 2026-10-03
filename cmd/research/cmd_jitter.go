package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/vishalvx/back-tester/internal/experiment"
)

// cmdJitter re-runs the same rules on the same window with the starting capital nudged by up to +-5%, to show how
// much of a single-path result is path luck rather than rule quality.
func cmdJitter(c common, variants string, stage string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	env.LogPath = ""
	base := env.Capital
	fmt.Printf("# Capital-jitter test: %s %s..%s, stage %s, start capital Rs %.0f +-5%% in 21 steps\n\n", c.Universe, c.Start, c.End, stage, base)
	fmt.Println("| variant | min CAGR | p10 | median | p90 | max | spread (max-min) | share >= 15% | share < 0 |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|")
	for _, name := range strings.Split(variants, ",") {
		sp, err := presetByName(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		sp.Rules.Dividends = true
		var mu sync.Mutex
		var vals []float64
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.NumCPU())
		for k := -10; k <= 10; k++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(k int) {
				defer wg.Done()
				defer func() { <-sem }()
				o, err := env.WithCapital(base*(1+float64(k)*0.005)).Run(sp, experiment.Stage(stage))
				if err != nil {
					return
				}
				mu.Lock()
				vals = append(vals, o.CAGR)
				mu.Unlock()
			}(k)
		}
		wg.Wait()
		sort.Float64s(vals)
		var ge, lt int
		for _, v := range vals {
			if v >= 0.15 {
				ge++
			}
			if v < 0 {
				lt++
			}
		}
		n := float64(len(vals))
		fmt.Printf("| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", sp.ID, pct(vals[0]), pct(quantile(vals, .1)), pct(quantile(vals, .5)), pct(quantile(vals, .9)), pct(vals[len(vals)-1]), pct(vals[len(vals)-1]-vals[0]), pct(float64(ge)/n), pct(float64(lt)/n))
	}
	return nil
}
