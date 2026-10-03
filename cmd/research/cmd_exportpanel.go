package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// cmdExportPanel writes the daily table the simulator sees (close, distance below the 20-day average, membership, pivot
// supports), so an independent implementation of the written strategy can be run on identical inputs.
func cmdExportPanel(c common, outDir string) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	suffix := ""
	if c.Lag != 0 {
		suffix = fmt.Sprintf("_lag%d", c.Lag)
	}
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("panel_%s_%s_%s%s.csv", c.Universe, c.Start, c.End, suffix)))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"date", "symbol", "close", "diff_sma", "is_constituent", "cam_s1", "fib_s1"})
	g := func(x float64) string { return strconv.FormatFloat(x, 'f', 8, 64) }
	for _, dt := range env.Panel.Dates {
		ds := dt.Format("2006-01-02")
		for _, d := range env.Panel.Days[ds] {
			w.Write([]string{ds, d.Symbol, g(d.Bar.Close), g(d.DiffSMA), strconv.FormatBool(d.IsConstituent), g(d.Pivots.CamS1), g(d.Pivots.FibS1)})
		}
	}
	w.Flush()
	return w.Error()
}
