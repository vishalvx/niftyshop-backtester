package sim

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/engine"
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

func toBuckets(p *panel.Panel) map[string][]engine.EngineStock {
	out := map[string][]engine.EngineStock{}
	for ds, list := range p.Days {
		for _, d := range list {
			out[ds] = append(out[ds], engine.EngineStock{
				EngineBar: engine.EngineBar{Open: d.Bar.Open, Close: d.Bar.Close, High: d.Bar.High, Low: d.Bar.Low, Volume: d.Bar.Volume, Date: d.Bar.Date},
				Symbol:    d.Symbol, SMA20: d.SMA, DiffSMA: d.DiffSMA, IsConstituent: d.IsConstituent,
				ClassicS1: d.Pivots.ClassicS1, ClassicS2: d.Pivots.ClassicS2, ClassicS3: d.Pivots.ClassicS3,
				FibS1: d.Pivots.FibS1, FibS2: d.Pivots.FibS2, FibS3: d.Pivots.FibS3,
				CamS1: d.Pivots.CamS1, CamS2: d.Pivots.CamS2, CamS3: d.Pivots.CamS3, CamS4: d.Pivots.CamS4,
			})
		}
	}
	return out
}

func quiet(t *testing.T, f func()) {
	t.Helper()
	old := os.Stdout
	null, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = null
	defer func() { os.Stdout = old; null.Close() }()
	f()
}

func TestLegacyRulesReproduceProductionEngine(t *testing.T) {
	for _, pv := range []*PivotRule{nil, {System: "camarilla", Level: "S1", Pool: 5}, {System: "fibonacci", Level: "closest", Pool: 10}} {
		for seed := int64(1); seed <= 3; seed++ {
			dir := t.TempDir()
			wf, start, end := writeSynthetic(t, dir, 30, 700, seed)
			p, _, err := panel.Build(panel.Options{DataDir: dir, WeightsFile: wf, Start: start.AddDate(0, 1, 0), End: end, MAWindow: 20, Quarantine: map[string]string{}})
			if err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{Universe: "x", MAWindow: 20, ProfitTargetPct: 0.05, AvgTriggerPct: 0.03, MaxStocks: 5, CapitalDivider: 10,
				StartCapital: 100000, RevisionPeriod: "monthly", MaxFreshEntriesPerDay: 1}
			rules := LegacyRules()
			rules.Pivot = pv
			if pv != nil {
				cfg.PivotFilter = config.PivotFilterConfig{Enabled: true, System: pv.System, Level: pv.Level, PoolSize: pv.Pool}
			}
			var acc interface{ GetCapital() float64 }
			var hist []struct {
				d     string
				sym   string
				act   string
				qty   int
				price float64
			}
			quiet(t, func() {
				a, st := engine.RunNiftyShop(toBuckets(p), cfg)
				acc = a
				for _, tr := range st.History {
					hist = append(hist, struct {
						d     string
						sym   string
						act   string
						qty   int
						price float64
					}{tr.Date.Format("2006-01-02"), tr.Symbol, map[int]string{0: "FRESH", 1: "AVG", 2: "SELL"}[int(tr.Action)], int(tr.Lot), tr.Price})
				}
			})
			res, err := Run(p, rules)
			if err != nil {
				t.Fatal(err)
			}
			if len(hist) == 0 {
				t.Fatalf("engine produced no trades (seed %d)", seed)
			}
			if len(hist) != len(res.Events) {
				t.Fatalf("pivot=%v seed=%d: engine %d trades, sim %d events", pv, seed, len(hist), len(res.Events))
			}
			for i, h := range hist {
				e := res.Events[i]
				if h.d != e.Date.Format("2006-01-02") || h.sym != e.Symbol || h.act != e.Action || h.qty != e.Qty || math.Abs(h.price-e.Price) > 1e-9 {
					t.Fatalf("pivot=%v seed=%d trade %d differs: engine %+v sim %+v", pv, seed, i, h, e)
				}
			}
			if math.Abs(acc.GetCapital()-res.Points[len(res.Points)-1].Cash) > 1e-6 {
				t.Fatalf("final cash differs: engine %.4f sim %.4f", acc.GetCapital(), res.Points[len(res.Points)-1].Cash)
			}
			t.Logf("pivot=%v seed=%d: %d trades identical, final cash %.2f", pv, seed, len(hist), acc.GetCapital())
		}
	}
}
