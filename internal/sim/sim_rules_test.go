package sim

import (
	"math"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/panel"
)

// hand-built one-stock panels: each step is (close, diffSMA) for a single symbol X on consecutive weekdays.
type step struct {
	close, diff float64
	nextOpen    float64
}

func onePanel(sym string, steps []step) *panel.Panel {
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	d := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, s := range steps {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		ds := d.Format("2006-01-02")
		p.Dates = append(p.Dates, d)
		p.Days[ds] = []panel.Day{{Symbol: sym, IsConstituent: true, DiffSMA: s.diff, NextOpen: s.nextOpen,
			Bar: panel.Bar{Date: d, Open: s.close, High: s.close, Low: s.close, Close: s.close}}}
		d = d.AddDate(0, 0, 1)
	}
	return p
}

func rulesFor(mod func(r *Rules)) Rules {
	r := LegacyRules()
	r.StartCapital = 1000000 // slot 100,000 so integer rounding is negligible
	if mod != nil {
		mod(&r)
	}
	return r
}

func sells(res *Result) (n int) {
	for _, e := range res.Events {
		if e.Action == "SELL" {
			n++
		}
	}
	return
}

func TestExitBasisLotVersusAverageCost(t *testing.T) {
	// buy 100, average at 96.9 (>=3% down), then rally to 102 and 104.
	steps := []step{{100, 5, 0}, {96.9, 5, 0}, {102, -1, 0}, {104, -1, 0}}
	lot, _ := Run(onePanel("X", steps), rulesFor(nil))
	// lot basis: second lot sells at 102 (>= 96.9*1.05 = 101.745); first lot still open (needs 105).
	if got := sells(lot); got != 1 {
		t.Fatalf("lot basis: want 1 sale after the rally, got %d", got)
	}
	avg, _ := Run(onePanel("X", steps), rulesFor(func(r *Rules) { r.ExitBasis = "avgcost" }))
	// average cost basis: avg = 98.45, target 103.37 -> nothing at 102, whole position at 104.
	if got := sells(avg); got != 2 {
		t.Fatalf("avgcost basis: want both lots sold together, got %d sales", got)
	}
	if avg.Events[len(avg.Events)-1].Date != avg.Points[len(avg.Points)-1].Date {
		t.Fatalf("avgcost exit should happen on the last day (104)")
	}
}

func TestAverageReferenceAnyLotVersusLastLot(t *testing.T) {
	// lots at 100 and 96.9; close 96.5 is 3.5% below the first lot but only 0.4% below the latest.
	steps := []step{{100, 5, 0}, {96.9, 5, 0}, {96.5, 5, 0}}
	anyLot, _ := Run(onePanel("X", steps), rulesFor(nil))
	lastLot, _ := Run(onePanel("X", steps), rulesFor(func(r *Rules) { r.AvgBasis = "lastlot" }))
	count := func(r *Result, a string) (n int) {
		for _, e := range r.Events {
			if e.Action == a {
				n++
			}
		}
		return
	}
	if count(anyLot, "AVG") != 2 {
		t.Fatalf("engine behaviour: a buy every day while below the first lot, want 2 AVG got %d", count(anyLot, "AVG"))
	}
	if count(lastLot, "AVG") != 1 {
		t.Fatalf("documented behaviour: only 3%% below the latest lot, want 1 AVG got %d", count(lastLot, "AVG"))
	}
}

func TestMaxStocksIsEnforcedOnlyWhenSet(t *testing.T) {
	p := &panel.Panel{Days: map[string][]panel.Day{}}
	d0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		d := d0.AddDate(0, 0, i)
		p.Dates = append(p.Dates, d)
		var list []panel.Day
		for k, s := range []string{"A", "B", "C", "D"} {
			list = append(list, panel.Day{Symbol: s, IsConstituent: true, DiffSMA: float64(10 - k), Bar: panel.Bar{Date: d, Close: 100, Open: 100, High: 100, Low: 100}})
		}
		p.Days[d.Format("2006-01-02")] = list
	}
	unl, _ := Run(p, rulesFor(nil))
	cap2, _ := Run(p, rulesFor(func(r *Rules) { r.MaxStocks = 2 }))
	if unl.Points[len(unl.Points)-1].Stocks != 4 {
		t.Fatalf("engine ignores max_stocks: want 4 stocks, got %d", unl.Points[len(unl.Points)-1].Stocks)
	}
	if cap2.Points[len(cap2.Points)-1].Stocks != 2 || cap2.Skipped.MaxStocks == 0 {
		t.Fatalf("MaxStocks=2 must cap at 2 stocks, got %d", cap2.Points[len(cap2.Points)-1].Stocks)
	}
}

func TestStopLossAndTimeStop(t *testing.T) {
	stop, _ := Run(onePanel("X", []step{{100, 5, 0}, {85, -1, 0}}), rulesFor(func(r *Rules) { r.StopLoss = 0.10; r.AvgTrigger = 0.50 }))
	if sells(stop) != 1 || stop.Lots[len(stop.Lots)-1].Reason != "stop" {
		t.Fatalf("a 15%% fall with a 10%% stop must exit with reason stop: %+v", stop.Lots)
	}
	flat := make([]step, 12)
	for i := range flat {
		flat[i] = step{100, 5, 0}
	}
	flat[0].diff = 5
	for i := 1; i < len(flat); i++ {
		flat[i].diff = -1 // no further entries
	}
	ts, _ := Run(onePanel("X", flat), rulesFor(func(r *Rules) { r.TimeStopDays = 7 }))
	if sells(ts) != 1 || ts.Lots[0].Reason != "time" {
		t.Fatalf("time stop must close the position after 7 days: %+v", ts.Lots)
	}
}

func TestTrailingExitLetsWinnersRun(t *testing.T) {
	steps := []step{{100, 5, 0}, {106, -1, 0}, {120, -1, 0}, {115, -1, 0}, {107, -1, 0}}
	fixed, _ := Run(onePanel("X", steps), rulesFor(nil))
	trail, _ := Run(onePanel("X", steps), rulesFor(func(r *Rules) { r.TrailPct = 0.10 }))
	if fixed.Lots[0].SellPrice >= 110 {
		t.Fatalf("fixed 5%% target should sell near 106, got %.2f", fixed.Lots[0].SellPrice)
	}
	// armed at 106, peak 120, exit when close <= 108 (10% off peak): 107 on the last day.
	if got := trail.Lots[0].SellPrice; math.Abs(got-107) > 1e-9 || trail.Lots[0].Reason != "trail" {
		t.Fatalf("trailing exit should fire at 107, got %.2f (%s)", got, trail.Lots[0].Reason)
	}
}

func TestNextOpenFillAndSlippage(t *testing.T) {
	steps := []step{{100, 5, 101}, {110, -1, 0}}
	r, _ := Run(onePanel("X", steps), rulesFor(func(r *Rules) { r.FillNextOpen = true; r.SlippageBps = 100 }))
	// buy fills at next open 101 plus 1% adverse = 102.01
	if got := r.Lots[0].BuyPrice; math.Abs(got-102.01) > 1e-9 {
		t.Fatalf("buy price want 102.01 got %v", got)
	}
}

func TestCashNeverNegativeWithCostsAndTax(t *testing.T) {
	for seed := int64(1); seed <= 3; seed++ {
		dir := t.TempDir()
		wf, start, end := writeSynthetic(t, dir, 30, 900, seed)
		p, _, err := panel.Build(panel.Options{DataDir: dir, WeightsFile: wf, Start: start.AddDate(0, 1, 0), End: end, MAWindow: 20, Quarantine: map[string]string{}})
		if err != nil {
			t.Fatal(err)
		}
		r := LegacyRules()
		r.StartCapital = 1000000
		r.Costs = costs.IndianDelivery{Statutory: true}
		r.SlippageBps = 10
		r.Tax = costs.NewTaxBook(costs.Dated)
		res, err := Run(p, r)
		if err != nil {
			t.Fatal(err)
		}
		for _, pt := range res.Points {
			if pt.Cash < -1e-6 {
				t.Fatalf("seed %d: cash went negative (%.2f) on %s", seed, pt.Cash, pt.Date.Format("2006-01-02"))
			}
		}
		if res.TaxPaid == 0 && len(res.Lots) > 20 {
			t.Fatalf("seed %d: expected some tax on a profitable synthetic book", seed)
		}
	}
}

func TestAppTargetPerLotPrices(t *testing.T) {
	// buy 100 (target 106), average at 96.9 (new avg 98.45, lot 2 target 104.36), rally to 105: lot 2 sells, lot 1 stays.
	steps := []step{{100, 5, 0}, {96.9, 5, 0}, {105, -1, 0}, {106.5, -1, 0}}
	res, err := Run(onePanel("X", steps), rulesFor(func(r *Rules) {
		r.ExitBasis, r.AvgBasis, r.ProfitTarget = "apptarget", "lastlot", 0.06
	}))
	if err != nil {
		t.Fatal(err)
	}
	var sellDays []int
	for _, e := range res.Events {
		if e.Action == "SELL" {
			for i, p := range res.Points {
				if p.Date.Equal(e.Date) {
					sellDays = append(sellDays, i)
				}
			}
		}
	}
	if len(sellDays) != 2 || sellDays[0] != 2 || sellDays[1] != 3 {
		t.Fatalf("lot 2 should sell on day 2 and lot 1 on day 3, got sells on days %v", sellDays)
	}
	// With synced targets (V-Pivot) both lots share the averaged target and sell together on day 2 (105 >= 104.36).
	res, _ = Run(onePanel("X", steps), rulesFor(func(r *Rules) {
		r.ExitBasis, r.AvgBasis, r.ProfitTarget, r.SyncTargets = "apptarget", "lastlot", 0.06, true
	}))
	if sells(res) != 2 || res.Events[len(res.Events)-1].Date != res.Points[2].Date {
		t.Fatalf("synced targets: both lots should sell together on day 2")
	}
}

func TestExitOnIndexRemoval(t *testing.T) {
	steps := []step{{100, 5, 0}, {96, 1, 0}, {92, 1, 0}, {90, 1, 0}, {91, 1, 0}}
	build := func() *panel.Panel {
		p := onePanel("X", steps)
		for i, d := range p.Dates {
			ds := d.Format("2006-01-02")
			days := p.Days[ds]
			if i >= 3 { // the stock drops out of the index on day 3
				days[0].IsConstituent = false
			}
			p.Days[ds] = days
		}
		return p
	}
	on, err := Run(build(), rulesFor(func(r *Rules) { r.ExitOnIndexRemoval = true }))
	if err != nil {
		t.Fatal(err)
	}
	var closed *LotRecord
	for i := range on.Lots {
		if !on.Lots[i].Open && on.Lots[i].Reason == "index" {
			closed = &on.Lots[i]
		}
	}
	if closed == nil {
		t.Fatalf("a stock that leaves the index must be sold; lots: %+v", on.Lots)
	}
	if !closed.SellDate.Equal(on.Points[3].Date) || math.Abs(closed.SellPrice-90) > 1e-9 {
		t.Fatalf("sold on %s at %.2f, want day 3 at 90", closed.SellDate.Format("2006-01-02"), closed.SellPrice)
	}
	off, _ := Run(build(), rulesFor(nil))
	for _, l := range off.Lots {
		if !l.Open {
			t.Fatalf("without the rule the lot stays open, got a sale: %+v", l)
		}
	}
}
