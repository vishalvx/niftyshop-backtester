package experiment

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
)

// maxIndexStaleDays is how far the index's last value may lag a strategy month-end before the month is shown as n/a
// (a long weekend or holiday is a few days; anything longer means the index file does not cover the month).
const maxIndexStaleDays = 7

// MonthRow is one calendar month of a run beside its index.
type MonthRow struct {
	Month      string    // YYYY-MM
	End        time.Time // last trading day of the month in the run
	Strategy   float64   // strategy return for the month at the run's stage
	Index      float64   // index return for the month, NaN when the index file does not cover it
	Diff       float64   // Strategy - Index, NaN when Index is
	Buys       int       // buy fills (new stock or added lot)
	Sells      int       // sell fills (one per lot sold)
	OpenLots   int       // lots held at the month's last close
	Value      float64   // strategy value in rupees at the month's last close
	IndexValue float64   // value in rupees of the same capital held in the index, NaN when not covered
}

// MonthlyTable is the month-by-month comparison of one (variant, stage) run with buy-and-hold of its index.
type MonthlyTable struct {
	Variant  string
	Stage    Stage
	Window   string
	Scenario string
	Index    string // e.g. "NIFTY 50 TRI"
	Capital  float64
	Start    time.Time
	End      time.Time
	Rows     []MonthRow

	// Whole-run figures. Index ones are NaN when the index does not cover the run.
	StrategyTotal, IndexTotal float64
	StrategyCAGR, IndexCAGR   float64
	IndexTax                  float64 // tax on redeeming the index holding at the end (tax stages only)
	Beat, Compared            int     // months the strategy beat the index, months with an index return
}

// StageLabel says in words what a stage includes.
func StageLabel(st Stage) string {
	switch st {
	case Gross:
		return "before costs and tax"
	case Costed:
		return "after costs, before tax"
	case TaxDated:
		return "after costs and tax (tax rates in force on each sale date)"
	case TaxToday:
		return "after costs and tax (today's tax rates for every year)"
	}
	return string(st)
}

func taxMode(st Stage) (costs.TaxMode, bool) {
	switch st {
	case TaxDated:
		return costs.Dated, true
	case TaxToday:
		return costs.Today, true
	}
	return 0, false
}

// indexAt returns the index value on or before d, or NaN when the series has no value within maxIndexStaleDays of d.
func indexAt(s []analytics.Point, d time.Time) float64 {
	i := sort.Search(len(s), func(i int) bool { return s[i].Date.After(d) })
	if i == 0 || d.Sub(s[i-1].Date) > maxIndexStaleDays*24*time.Hour {
		return math.NaN()
	}
	return s[i-1].Value
}

func cagrOver(total float64, start, end time.Time) float64 {
	years := end.Sub(start).Hours() / (24 * 365.25)
	if math.IsNaN(total) || years <= 0 {
		return math.NaN()
	}
	if total <= -1 {
		return -1
	}
	return math.Pow(1+total, 1/years) - 1
}

// Monthly builds the month-by-month table for an outcome. The strategy side is the outcome's own equity curve, so
// it carries the stage's charges and tax: tax settled at each financial-year end lands in the month it is paid, and in
// the tax stages the last month also carries the tax due if every open lot were sold at the last close. The index side
// is the same capital put in the total-return index on the run's first day and held, the way a growth-option index
// fund is taxed: nothing until redemption, so in the tax stages only the last month carries the redemption tax.
func (e *Env) Monthly(o *Outcome) MonthlyTable {
	res := o.Result
	pts := res.Points
	t := MonthlyTable{Variant: o.Spec.ID, Stage: o.Stage, Window: e.Window, Scenario: e.Scenario.Name, Index: e.TRIName,
		Capital: res.StartCap, StrategyTotal: math.NaN(), IndexTotal: math.NaN(), StrategyCAGR: math.NaN(), IndexCAGR: math.NaN()}
	if len(pts) == 0 {
		return t
	}
	t.Start, t.End = pts[0].Date, pts[len(pts)-1].Date

	// Month-end index of every point, in order.
	var ends []int
	for i, p := range pts {
		if i == len(pts)-1 || p.Date.Month() != pts[i+1].Date.Month() || p.Date.Year() != pts[i+1].Date.Year() {
			ends = append(ends, i)
		}
	}
	buys, sells := map[string]int{}, map[string]int{}
	for _, ev := range res.Events {
		k := ev.Date.Format("2006-01")
		if ev.Action == "SELL" {
			sells[k]++
		} else {
			buys[k]++
		}
	}

	idx0 := indexAt(e.TRI, t.Start)
	// Redemption tax on the index holding, applied to its last value in the tax stages.
	idxScale := 1.0
	if mode, ok := taxMode(o.Stage); ok && !math.IsNaN(idx0) && !math.IsNaN(indexAt(e.TRI, t.End)) {
		if r, err := bench.BuyHold(e.TRI, t.Start, t.End, t.Capital, mode); err == nil && r.GrossFinal > 0 {
			t.IndexTax = r.Tax
			idxScale = r.NetFinal / r.GrossFinal
		}
	}

	prevV, prevI := t.Capital, t.Capital
	for _, i := range ends {
		p := pts[i]
		v := p.Equity
		if i == len(pts)-1 {
			v = o.Final // liquidation-adjusted, as in Evaluate
		}
		iv := t.Capital * indexAt(e.TRI, p.Date) / idx0
		if i == len(pts)-1 {
			iv *= idxScale
		}
		k := p.Date.Format("2006-01")
		row := MonthRow{Month: k, End: p.Date, Strategy: v/prevV - 1, Index: iv/prevI - 1, Buys: buys[k], Sells: sells[k],
			OpenLots: p.Lots, Value: v, IndexValue: iv}
		row.Diff = row.Strategy - row.Index
		if !math.IsNaN(row.Index) {
			t.Compared++
			if row.Diff > 0 {
				t.Beat++
			}
		}
		t.Rows = append(t.Rows, row)
		prevV, prevI = v, iv
	}
	last := t.Rows[len(t.Rows)-1]
	t.StrategyTotal = last.Value/t.Capital - 1
	t.IndexTotal = last.IndexValue/t.Capital - 1
	t.StrategyCAGR = cagrOver(t.StrategyTotal, t.Start, t.End)
	t.IndexCAGR = cagrOver(t.IndexTotal, t.Start, t.End)
	return t
}

func pctCell(x float64) string {
	if math.IsNaN(x) {
		return "n/a"
	}
	return strconv.FormatFloat(x*100, 'f', 2, 64)
}

func rsCell(x float64) string {
	if math.IsNaN(x) {
		return "n/a"
	}
	return strconv.FormatFloat(x, 'f', 0, 64)
}

func (t MonthlyTable) totals() (buys, sells int) {
	for _, r := range t.Rows {
		buys += r.Buys
		sells += r.Sells
	}
	return buys, sells
}

// WriteCSV writes one line per month (returns in percent, difference in percentage points, values in rupees) and a
// final "total" line for the whole run.
func (t MonthlyTable) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"month", "strategy_pct", "index_pct", "difference_pts", "buys", "sells", "open_lots", "strategy_value", "index_value"})
	for _, r := range t.Rows {
		cw.Write([]string{r.Month, pctCell(r.Strategy), pctCell(r.Index), pctCell(r.Diff), strconv.Itoa(r.Buys), strconv.Itoa(r.Sells),
			strconv.Itoa(r.OpenLots), rsCell(r.Value), rsCell(r.IndexValue)})
	}
	if n := len(t.Rows); n > 0 {
		b, s := t.totals()
		last := t.Rows[n-1]
		cw.Write([]string{"total", pctCell(t.StrategyTotal), pctCell(t.IndexTotal), pctCell(t.StrategyTotal - t.IndexTotal), strconv.Itoa(b), strconv.Itoa(s),
			strconv.Itoa(last.OpenLots), rsCell(last.Value), rsCell(last.IndexValue)})
	}
	cw.Flush()
	return cw.Error()
}

// WriteMarkdown writes the same table with a header saying what was compared.
func (t MonthlyTable) WriteMarkdown(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("# %s vs %s, month by month\n\n", t.Variant, t.Index)
	ew.printf("- Run: %s (trading days %s to %s), cost scenario %s, starting capital Rs %s\n", t.Window, t.Start.Format("2006-01-02"), t.End.Format("2006-01-02"), t.Scenario, rsCell(t.Capital))
	ew.printf("- Strategy: stage `%s`, %s\n", t.Stage, StageLabel(t.Stage))
	ew.printf("- Index: the same capital in the %s on the first day, held to the end\n", t.Index)
	_, taxed := taxMode(t.Stage)
	if taxed {
		ew.printf("- Last month (*): both sides carry the tax due on selling everything at the last close; for the index that is Rs %s\n", rsCell(t.IndexTax))
	}
	ew.printf("- Difference is strategy %% minus index %%, in percentage points. Open lots are counted at the month's last close.\n\n")
	if len(t.Rows) == 0 {
		ew.printf("No trading days in the window.\n")
		return ew.err
	}
	b, s := t.totals()
	ew.printf("Whole run: strategy %s%% (CAGR %s%%), index %s%% (CAGR %s%%), CAGR difference %s points. Strategy beat the index in %d of %d months. %d buys, %d sells, %d lots open at the end.\n\n",
		pctCell(t.StrategyTotal), pctCell(t.StrategyCAGR), pctCell(t.IndexTotal), pctCell(t.IndexCAGR), pctCell(t.StrategyCAGR-t.IndexCAGR), t.Beat, t.Compared, b, s, t.Rows[len(t.Rows)-1].OpenLots)
	ew.printf("| Month | Strategy %% | Index %% | Difference (pts) | Buys | Sells | Open lots | Strategy Rs | Index Rs |\n")
	ew.printf("|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for i, r := range t.Rows {
		month := r.Month
		if taxed && i == len(t.Rows)-1 {
			month += "*"
		}
		ew.printf("| %s | %s | %s | %s | %d | %d | %d | %s | %s |\n", month, pctCell(r.Strategy), pctCell(r.Index), pctCell(r.Diff), r.Buys, r.Sells, r.OpenLots, rsCell(r.Value), rsCell(r.IndexValue))
	}
	last := t.Rows[len(t.Rows)-1]
	ew.printf("| **Total** | **%s** | **%s** | **%s** | %d | %d | %d | %s | %s |\n", pctCell(t.StrategyTotal), pctCell(t.IndexTotal), pctCell(t.StrategyTotal-t.IndexTotal), b, s, last.OpenLots, rsCell(last.Value), rsCell(last.IndexValue))
	return ew.err
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, a ...any) {
	if e.err == nil {
		_, e.err = fmt.Fprintf(e.w, format, a...)
	}
}
