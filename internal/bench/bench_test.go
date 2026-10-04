package bench

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/costs"
)

func TestLoadTRIAndBuyHoldAfterTax(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tri.json")
	// out of order, one duplicate date and one malformed value: the loader must sort, dedupe and skip.
	os.WriteFile(p, []byte(`[
	 {"Date":"01 Jan 2025","TotalReturnsIndex":"200.00"},
	 {"Date":"01 Jan 2019","TotalReturnsIndex":"100.00"},
	 {"Date":"01 Jan 2019","TotalReturnsIndex":"999.00"},
	 {"Date":"02 Jan 2019","TotalReturnsIndex":"-"}]`), 0o644)
	s, err := LoadTRI(p)
	if err != nil || len(s) != 2 || s[0].Value != 100 || s[1].Value != 200 {
		t.Fatalf("load: %+v err=%v", s, err)
	}
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	r, err := BuyHold(s, d(2019, 1, 1), d(2025, 1, 1), 1000000, costs.Dated)
	if err != nil {
		t.Fatal(err)
	}
	// gain 1,000,000, long term, sold after 23 Jul 2024: (1,000,000 - 125,000) * 12.5% * 1.04
	want := 875000 * 0.125 * 1.04
	if math.Abs(r.Tax-want) > 1e-6 || math.Abs(r.NetFinal-(2000000-want)) > 1e-6 {
		t.Fatalf("tax %.2f want %.2f", r.Tax, want)
	}
	if math.Abs(r.GrossCAGR-(math.Pow(2, 1/r.Years)-1)) > 1e-9 {
		t.Fatalf("gross CAGR %v", r.GrossCAGR)
	}
	if _, err := BuyHold(s, d(2010, 1, 1), d(2025, 1, 1), 1000000, costs.Dated); err == nil {
		t.Fatal("a series that starts after the requested start must be an error")
	}
}

func TestLessFeeTakesTheFeeOverAnySpan(t *testing.T) {
	d0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var p []analytics.Point
	for i := 0; i <= 4*365; i++ {
		p = append(p, analytics.Point{Date: d0.AddDate(0, 0, i), Value: 100 * math.Pow(1.10, float64(i)/365.25)})
	}
	got := LessFee(p, 0.01)
	if got[0].Value != p[0].Value {
		t.Fatalf("the first point keeps its value: %v", got[0].Value)
	}
	// Any two-year span: the index's 10% a year less a 1% fee is 8.9% a year.
	a, b := 300, 300+730
	years := got[b].Date.Sub(got[a].Date).Hours() / (24 * 365.25)
	cagr := math.Pow(got[b].Value/got[a].Value, 1/years) - 1
	if math.Abs(cagr-(1.10*0.99-1)) > 1e-9 {
		t.Fatalf("CAGR after a 1%% fee = %.6f, want %.6f", cagr, 1.10*0.99-1)
	}
	if &LessFee(p, 0)[0] != &p[0] {
		t.Fatalf("a zero fee returns the series unchanged")
	}
}
