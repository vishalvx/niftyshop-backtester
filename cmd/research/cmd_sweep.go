package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/vishalvx/back-tester/internal/experiment"
	"github.com/vishalvx/back-tester/internal/sim"
)

// cmdSweep expands a sweep file into trials, runs each over the window at one stage, logs every trial and prints a table.
func cmdSweep(c common, path, stage string) error {
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
	var ts []experiment.Trial
	for _, r := range rules {
		ts = append(ts, experiment.Trial{Spec: experiment.Spec{ID: r.Name, Rules: r}, Group: sw.ID})
	}
	outs, err := env.RunTrials(ts, experiment.Stage(stage), experiment.Window{From: d(c.Start), To: d(c.End)})
	if err != nil {
		return err
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].Sharpe > outs[j].Sharpe })
	fmt.Printf("# Sweep %s on %s %s..%s, stage %s: %d trials (each one is written to %s)\n\n", sw.ID, c.Universe, c.Start, c.End, stage, len(outs), c.LogPath)
	fmt.Println("| trial | CAGR | maxDD | Sharpe | Calmar | closed lots | open lots |")
	fmt.Println("|---|---|---|---|---|---|---|")
	for _, o := range outs {
		m := o.Metrics
		fmt.Printf("| %s | %s | %s | %.2f | %.2f | %d | %d |\n", o.Spec.ID, pct(o.CAGR), pct(m.MaxDD.Depth), m.Sharpe, m.Calmar, m.ClosedLots, m.OpenLots)
	}
	return nil
}
