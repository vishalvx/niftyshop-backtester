package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
)

type stressWindow struct {
	Name     string
	From, To string
}

// Calendar windows are fixed in advance, so every candidate sees the same ones.
var stressWindows = []stressWindow{
	{"2008 crash", "2008-01-01", "2008-12-31"},
	{"2011", "2011-01-01", "2011-12-31"},
	{"2015-16", "2015-03-01", "2016-02-29"},
	{"2018-19 slump", "2018-01-01", "2019-03-31"},
	{"COVID", "2020-01-14", "2020-03-23"},
	{"2022", "2022-01-01", "2022-12-31"},
}

// stressLine reports, for each stress window the series fully covers, the return and the worst peak-to-trough fall inside it.
func stressLine(pts []analytics.Point) string {
	var out []string
	for _, w := range stressWindows {
		from, to := d(w.From), d(w.To)
		seg := bench.Clip(pts, from, to)
		if len(seg) < 5 || seg[0].Date.After(from.AddDate(0, 0, 10)) || seg[len(seg)-1].Date.Before(to.AddDate(0, 0, -10)) {
			out = append(out, fmt.Sprintf("%s: n/a", w.Name))
			continue
		}
		peak, dd := seg[0].Value, 0.0
		for _, p := range seg {
			if p.Value > peak {
				peak = p.Value
			}
			if x := p.Value/peak - 1; x < dd {
				dd = x
			}
		}
		out = append(out, fmt.Sprintf("%s: %+.1f%% (worst fall %.1f%%)", w.Name, (seg[len(seg)-1].Value/seg[0].Value-1)*100, dd*100))
	}
	return strings.Join(out, " | ")
}

func ptsOf(res []analytics.Point) []analytics.Point { return res }

// cmdRefScore scores published total-return index series as if they were candidates: buy on day one, hold, tax once at
// the end. The window starts at the later of -start and the series' first date. -bench names the comparison series.
func cmdRefScore(c common, series string, horizons []int) error {
	cmpStem := c.Bench
	if cmpStem == "" {
		cmpStem = "NIFTY_50"
	}
	cmp, err := bench.LoadTRI(".research-data/indices/" + cmpStem + "_TRI.json")
	if err != nil {
		return err
	}
	start, end := mustDate(c.Start), mustDate(c.End)
	for _, stem := range strings.Split(series, ",") {
		stem = strings.TrimSpace(stem)
		tri, err := bench.LoadTRI(".research-data/indices/" + stem + "_TRI.json")
		if err != nil {
			return err
		}
		s0 := start
		if tri[0].Date.After(s0) {
			s0 = tri[0].Date
		}
		fmt.Printf("\n=== %s | %s .. %s | compared with %s ===\n", stem, s0.Format("2006-01-02"), end.Format("2006-01-02"), cmpStem)
		for _, mode := range []costs.TaxMode{costs.Dated, costs.Today} {
			r, err := bench.BuyHold(tri, s0, end, c.Capital, mode)
			if err != nil {
				return err
			}
			mn := "dated"
			if mode == costs.Today {
				mn = "today"
			}
			fmt.Printf("buy-and-hold: gross CAGR %s | after tax (%s rates) %s | final Rs %.0f -> Rs %.0f (tax Rs %.0f)\n", pct(r.GrossCAGR), mn, pct(r.NetCAGR), r.GrossFinal, r.NetFinal, r.Tax)
		}
		seg := bench.Clip(tri, s0, end)
		eq := make([]analytics.Point, len(seg))
		for i, p := range seg {
			eq[i] = analytics.Point{Date: p.Date, Value: p.Value / seg[0].Value * c.Capital}
		}
		m := analytics.Compute(analytics.Input{StartCapital: c.Capital, Equity: eq, Benchmark: bench.Clip(cmp, s0.AddDate(0, 0, -7), end), Rf: c.Rf})
		fmt.Printf("annual vol %s  Sharpe %.2f  Sortino %.2f  Calmar %.2f  Ulcer %.1f%%  max drawdown %s (peak %s trough %s, %.0f days underwater)  longest underwater %.1f years\n",
			pct(m.Vol), m.Sharpe, m.Sortino, m.Calmar, m.Ulcer*100, pct(m.MaxDD.Depth), m.MaxDD.PeakDate.Format("2006-01-02"), m.MaxDD.TroughDate.Format("2006-01-02"), m.MaxDD.DaysUnderwater, m.LongestUnderwater.Days/365.25)
		fmt.Printf("best year %d %s  worst year %d %s  positive months %s of %d | skew %.2f kurt %.1f VaR95 %s\n", m.BestYear.Year, pct(m.BestYear.Return), m.WorstYear.Year, pct(m.WorstYear.Return), pct(m.PositiveMonths), m.Months, m.Skew, m.ExcessKurtosis, pct(m.VaR95))
		if r := m.Rel; r != nil {
			fmt.Printf("vs %s: beta %.2f alpha %s corr %.2f TE %s IR %.2f up-capture %.0f%% down-capture %.0f%%\n", cmpStem, r.Beta, pct(r.Alpha), r.Correlation, pct(r.TrackingError), r.InfoRatio, r.UpCapture*100, r.DownCapture*100)
		}
		var ys []string
		for _, y := range m.YearReturns {
			p := ""
			if y.Partial {
				p = "*"
			}
			ys = append(ys, fmt.Sprintf("%d:%.1f%%%s", y.Year, y.Return*100, p))
		}
		fmt.Println("calendar years:", strings.Join(ys, " "))
		fmt.Println("stress:", stressLine(eq))
		fmt.Println("\n| horizon | windows | median (after tax, dated) | p10 | p90 | share >= 15% | share beating comparison | comparison median |")
		fmt.Println("|---|---|---|---|---|---|---|---|")
		for _, h := range horizons {
			var cg, cc []float64
			var ge15, beats int
			for s := s0; !s.AddDate(h, 0, 0).After(end); s = s.AddDate(0, 1, 0) {
				e := s.AddDate(h, 0, 0)
				a, err1 := bench.BuyHold(tri, s, e, c.Capital, costs.Dated)
				b, err2 := bench.BuyHold(cmp, s, e, c.Capital, costs.Dated)
				if err1 != nil || err2 != nil {
					continue
				}
				cg = append(cg, a.NetCAGR)
				cc = append(cc, b.NetCAGR)
				if a.NetCAGR >= 0.15 {
					ge15++
				}
				if a.NetCAGR > b.NetCAGR {
					beats++
				}
			}
			if len(cg) == 0 {
				fmt.Printf("| %dy | 0 | n/a | n/a | n/a | n/a | n/a | n/a |\n", h)
				continue
			}
			n := float64(len(cg))
			fmt.Printf("| %dy | %d | %s | %s | %s | %.0f%% | %.0f%% | %s |\n", h, len(cg), pct(quantile(cg, 0.5)), pct(quantile(cg, 0.1)), pct(quantile(cg, 0.9)), float64(ge15)/n*100, float64(beats)/n*100, pct(quantile(cc, 0.5)))
		}
	}
	return nil
}

var _ = time.Time{}
