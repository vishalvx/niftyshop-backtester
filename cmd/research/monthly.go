package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vishalvx/back-tester/internal/experiment"
)

// monthlyDir is where a window's month-by-month tables go: one folder per universe, window, lag, cost scenario and index.
func monthlyDir(c common, out string, env *experiment.Env) string {
	idx := strings.NewReplacer(" ", "_", "%", "pct").Replace(env.TRIName)
	return filepath.Join(out, "monthly", fmt.Sprintf("%s_%s_%s_lag%d_%s_vs_%s", c.Universe, c.Start, c.End, c.Lag, env.Scenario.Name, idx))
}

// writeMonthly writes the outcome's month-by-month table beside its index as <variant>_<stage>.csv and .md and returns
// the path without extension. Characters a file name should not carry (the ":" in pivot presets) become "-".
func writeMonthly(dir string, env *experiment.Env, o *experiment.Outcome) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.NewReplacer(":", "-", "/", "-", " ", "-").Replace(o.Spec.ID) + "_" + string(o.Stage)
	base := filepath.Join(dir, name)
	t := env.Monthly(o)
	for _, w := range []struct {
		ext   string
		write func(*os.File) error
	}{
		{".csv", func(f *os.File) error { return t.WriteCSV(f) }},
		{".md", func(f *os.File) error { return t.WriteMarkdown(f) }},
	} {
		f, err := os.Create(base + w.ext)
		if err != nil {
			return "", err
		}
		if err := w.write(f); err != nil {
			f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
	}
	return base, nil
}
