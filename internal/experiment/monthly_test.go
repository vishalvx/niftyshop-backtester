package experiment

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/sim"
)

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// monthlyFixture is a three-month run that ends holding two lots of a stock bought in January and February and
// down 10% since: the open loser must show in the strategy's months and in the open-lot count.
func monthlyFixture(st Stage, tri []analytics.Point) (*Env, *Outcome) {
	res := &sim.Result{StartCap: 100000, FinalEquity: 90000,
		Points: []sim.Point{
			{Date: day("2024-01-02"), Equity: 100000},
			{Date: day("2024-01-15"), Equity: 100500, Lots: 1},
			{Date: day("2024-01-31"), Equity: 101000, Lots: 1},
			{Date: day("2024-02-29"), Equity: 95000, Lots: 2},
			{Date: day("2024-03-28"), Equity: 90000, Lots: 2},
		},
		Events: []sim.Event{
			{Date: day("2024-01-15"), Symbol: "AAA", Action: "FRESH"},
			{Date: day("2024-01-20"), Symbol: "BBB", Action: "FRESH"},
			{Date: day("2024-01-25"), Symbol: "BBB", Action: "SELL"},
			{Date: day("2024-02-12"), Symbol: "AAA", Action: "AVG"},
		},
		Lots: []sim.LotRecord{
			{Lot: sim.Lot{Symbol: "AAA", BuyDate: day("2024-01-15"), BuyPrice: 100, Qty: 500}, SellDate: day("2024-03-28"), SellPrice: 90, Open: true},
			{Lot: sim.Lot{Symbol: "AAA", BuyDate: day("2024-02-12"), BuyPrice: 100, Qty: 450}, SellDate: day("2024-03-28"), SellPrice: 90, Open: true},
		},
	}
	env := &Env{TRI: tri, TRIName: "TEST TRI", Window: "test", Scenario: Base}
	// No terminal tax: both open lots are at a loss.
	return env, &Outcome{Spec: Spec{ID: "fixture"}, Stage: st, Result: res, Final: 90000}
}

var fixtureTRI = []analytics.Point{
	{Date: day("2023-12-29"), Value: 990},
	{Date: day("2024-01-02"), Value: 1000},
	{Date: day("2024-01-31"), Value: 1020},
	{Date: day("2024-02-29"), Value: 1050},
	{Date: day("2024-03-28"), Value: 1100},
}

func TestMonthly_OpenLoserAndIndexAfterTax(t *testing.T) {
	env, o := monthlyFixture(TaxToday, fixtureTRI)
	tb := env.Monthly(o)
	if len(tb.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(tb.Rows))
	}
	// Index: 100000 in at 1000, worth 110000 at 1100; held under a year, so today's 20% short-term rate plus 4% cess
	// on the 10000 gain is 2080 of tax, paid in the last month.
	near(t, "index tax", tb.IndexTax, 2080, 1e-6)
	want := []struct {
		month           string
		strat, idx      float64
		buys, sells, ol int
	}{
		{"2024-01", 101000.0/100000 - 1, 1020.0/1000 - 1, 2, 1, 1},
		{"2024-02", 95000.0/101000 - 1, 1050.0/1020 - 1, 1, 0, 2},
		{"2024-03", 90000.0/95000 - 1, 107920.0/105000 - 1, 0, 0, 2},
	}
	for i, w := range want {
		r := tb.Rows[i]
		if r.Month != w.month || r.Buys != w.buys || r.Sells != w.sells || r.OpenLots != w.ol {
			t.Errorf("row %d = %+v, want month %s buys %d sells %d open lots %d", i, r, w.month, w.buys, w.sells, w.ol)
		}
		near(t, w.month+" strategy", r.Strategy, w.strat, 1e-12)
		near(t, w.month+" index", r.Index, w.idx, 1e-12)
		near(t, w.month+" diff", r.Diff, w.strat-w.idx, 1e-12)
	}
	near(t, "strategy total", tb.StrategyTotal, -0.10, 1e-12)
	near(t, "index total", tb.IndexTotal, 0.0792, 1e-12)
	if tb.Beat != 0 || tb.Compared != 3 {
		t.Errorf("beat %d of %d, want 0 of 3", tb.Beat, tb.Compared)
	}

	var csvOut, mdOut bytes.Buffer
	if err := tb.WriteCSV(&csvOut); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(csvOut.String()), "\n")
	wantCSV := []string{
		"month,strategy_pct,index_pct,difference_pts,buys,sells,open_lots,strategy_value,index_value",
		"2024-01,1.00,2.00,-1.00,2,1,1,101000,102000",
		"2024-02,-5.94,2.94,-8.88,1,0,2,95000,105000",
		"2024-03,-5.26,2.78,-8.04,0,0,2,90000,107920",
		"total,-10.00,7.92,-17.92,3,1,2,90000,107920",
	}
	if strings.Join(lines, "\n") != strings.Join(wantCSV, "\n") {
		t.Errorf("csv =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(wantCSV, "\n"))
	}
	if err := tb.WriteMarkdown(&mdOut); err != nil {
		t.Fatal(err)
	}
	md := mdOut.String()
	for _, s := range []string{
		"# fixture vs TEST TRI, month by month",
		"stage `tax-today`, after costs and tax",
		"for the index that is Rs 2080",
		"Strategy beat the index in 0 of 3 months. 3 buys, 1 sells, 2 lots open at the end.",
		"| 2024-03* | -5.26 | 2.78 | -8.04 | 0 | 0 | 2 | 90000 | 107920 |",
		"| **Total** | **-10.00** | **7.92** | **-17.92** | 3 | 1 | 2 | 90000 | 107920 |",
	} {
		if !strings.Contains(md, s) {
			t.Errorf("markdown lacks %q:\n%s", s, md)
		}
	}
}

func TestMonthly_UntaxedStageHasNoIndexTax(t *testing.T) {
	env, o := monthlyFixture(Costed, fixtureTRI)
	tb := env.Monthly(o)
	near(t, "index tax", tb.IndexTax, 0, 0)
	near(t, "index total", tb.IndexTotal, 0.10, 1e-12)
	var md bytes.Buffer
	if err := tb.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md.String(), "2024-03*") || strings.Contains(md.String(), "for the index that is") {
		t.Errorf("untaxed stage marks the last month as taxed:\n%s", md.String())
	}
}

// An index file that stops early must show n/a for the months it does not cover, never a 0% month.
func TestMonthly_IndexNotCoveringMonthsIsNA(t *testing.T) {
	env, o := monthlyFixture(TaxToday, fixtureTRI[:3])
	tb := env.Monthly(o)
	if math.IsNaN(tb.Rows[0].Index) {
		t.Errorf("January is covered, got n/a")
	}
	for _, r := range tb.Rows[1:] {
		if !math.IsNaN(r.Index) || !math.IsNaN(r.Diff) || !math.IsNaN(r.IndexValue) {
			t.Errorf("%s: index %v diff %v value %v, want n/a", r.Month, r.Index, r.Diff, r.IndexValue)
		}
	}
	if !math.IsNaN(tb.IndexTotal) || !math.IsNaN(tb.IndexCAGR) || tb.Compared != 1 {
		t.Errorf("index total %v CAGR %v compared %d, want n/a, n/a, 1", tb.IndexTotal, tb.IndexCAGR, tb.Compared)
	}
	var buf bytes.Buffer
	if err := tb.WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "2024-02,-5.94,n/a,n/a,1,0,2,95000,n/a") {
		t.Errorf("csv lacks the n/a February row:\n%s", buf.String())
	}
}
