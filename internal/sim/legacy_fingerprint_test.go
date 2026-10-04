package sim

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/panel"
)

// writeSynthetic creates n symbols of random-walk bars (mean-reverting enough to trigger every code path)
// plus a weights file where membership changes monthly.
func writeSynthetic(t *testing.T, dir string, n, days int, seed int64) (weights string, start, end time.Time) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	start = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	var dates []time.Time
	for d := start; len(dates) < days; d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			dates = append(dates, d)
		}
	}
	end = dates[len(dates)-1]
	hdr := "DATE"
	for i := 0; i < n; i++ {
		sym := fmt.Sprintf("S%02d", i)
		hdr += "," + sym
		px := 50 + rng.Float64()*950 // some prices exceed the slot size to exercise zero-unit paths
		f, _ := os.Create(filepath.Join(dir, sym+".NS.csv"))
		fmt.Fprintln(f, "Date,Open,High,Low,Close,AdjClose,Volume")
		mean := px
		for _, d := range dates {
			// skip some days for some symbols to exercise missing bars
			if rng.Float64() < 0.02 {
				continue
			}
			mean *= 1 + 0.0002
			px *= 1 + rng.NormFloat64()*0.018 + 0.03*(mean-px)/px
			px = math.Max(px, 1)
			hi := px * (1 + rng.Float64()*0.01)
			lo := px * (1 - rng.Float64()*0.01)
			fmt.Fprintf(f, "%s,%.4f,%.4f,%.4f,%.4f,%.4f,1000\n", d.Format("2006-01-02"), px, hi, lo, px, px)
		}
		f.Close()
	}
	wf := filepath.Join(dir, "weights.csv")
	w, _ := os.Create(wf)
	fmt.Fprintln(w, hdr)
	for m := start; !m.After(end); m = m.AddDate(0, 1, 0) {
		row := m.Format("2006-01-02")
		for i := 0; i < n; i++ {
			v := 0
			if (i+int(m.Month()))%3 != 0 {
				v = 1
			}
			row += fmt.Sprintf(",%d", v)
		}
		fmt.Fprintln(w, row)
	}
	w.Close()
	return wf, start, end
}

// TestLegacyRulesFingerprint pins LegacyRules to the trade lists the retired production engine
// (internal/engine.RunNiftyShop) produced on synthetic data. The engine was deleted after the
// long-run study; the fingerprints below were recorded while an equivalence test still compared
// the two trade by trade, so any change here means LegacyRules no longer reproduces that engine.
func TestLegacyRulesFingerprint(t *testing.T) {
	cases := []struct {
		pivot     *PivotRule
		seed      int64
		trades    int
		finalCash string
		hash      string
	}{
		{nil, 1, 415, "28266.4063", "4bf3b19aff3f547c"},
		{nil, 2, 578, "37440.9089", "36ced72595b8b863"},
		{nil, 3, 549, "4223.5917", "8a5dad1db5dc006d"},
		{&PivotRule{System: "camarilla", Level: "S1", Pool: 5}, 1, 447, "8004.3897", "5ddfcdbacf70d302"},
		{&PivotRule{System: "camarilla", Level: "S1", Pool: 5}, 2, 433, "21293.5117", "9c5216a080cb44fa"},
		{&PivotRule{System: "camarilla", Level: "S1", Pool: 5}, 3, 444, "26418.3434", "ce8f844b7676cc64"},
		{&PivotRule{System: "fibonacci", Level: "closest", Pool: 10}, 1, 471, "3827.1760", "7dc1f68e203c31b5"},
		{&PivotRule{System: "fibonacci", Level: "closest", Pool: 10}, 2, 439, "1674.8625", "769bffdfea692778"},
		{&PivotRule{System: "fibonacci", Level: "closest", Pool: 10}, 3, 422, "25583.2657", "615361294a1b46f2"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		wf, start, end := writeSynthetic(t, dir, 30, 700, c.seed)
		p, _, err := panel.Build(panel.Options{DataDir: dir, WeightsFile: wf, Start: start.AddDate(0, 1, 0), End: end, MAWindow: 20, Quarantine: map[string]string{}})
		if err != nil {
			t.Fatal(err)
		}
		rules := LegacyRules()
		rules.Pivot = c.pivot
		res, err := Run(p, rules)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		for _, e := range res.Events {
			fmt.Fprintf(h, "%s,%s,%s,%d,%.6f\n", e.Date.Format("2006-01-02"), e.Symbol, e.Action, e.Qty, e.Price)
		}
		hash := fmt.Sprintf("%x", h.Sum(nil))[:16]
		cash := fmt.Sprintf("%.4f", res.Points[len(res.Points)-1].Cash)
		if len(res.Events) != c.trades || cash != c.finalCash || hash != c.hash {
			t.Errorf("pivot=%v seed=%d: got %d trades, final cash %s, hash %s; want %d, %s, %s",
				c.pivot, c.seed, len(res.Events), cash, hash, c.trades, c.finalCash, c.hash)
		}
	}
}
