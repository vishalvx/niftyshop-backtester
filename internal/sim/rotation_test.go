package sim

import (
	"math"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/panel"
)

// threeStockPanel: A rises 2%/month, B flat, C falls 2%/month, over 30 months of daily weekday bars from 2019-01-01.
// Bars before the first date give the 12-month lookback.
func threeStockPanel() *panel.Panel {
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	rate := map[string]float64{"A": 0.02, "B": 0, "C": -0.02}
	start := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 6, 30, 0, 0, 0, 0, time.UTC)
	for sym := range rate {
		p.Series[sym] = &panel.Series{Symbol: sym}
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		months := float64(d.Year()-2019)*12 + float64(d.Month()-1) + float64(d.Day())/31
		var day []panel.Day
		for sym, r := range rate {
			px := 100 * math.Pow(1+r, months)
			b := panel.Bar{Date: d, Open: px, High: px, Low: px, Close: px}
			p.Series[sym].Bars = append(p.Series[sym].Bars, b)
			if !d.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
				day = append(day, panel.Day{Symbol: sym, Bar: b, IsConstituent: true})
			}
		}
		if !d.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
			p.Dates = append(p.Dates, d)
			p.Days[d.Format("2006-01-02")] = day
		}
	}
	return p
}

func TestRotationHoldsTopNAndRebalancesTwiceAYear(t *testing.T) {
	p := threeStockPanel()
	r := rulesFor(func(r *Rules) {
		r.Rotation = &RotationRule{LookbackMonths: []int{6, 12}, TopN: 2, RebalanceMonths: []int{1, 7}}
	})
	res, err := Run(p, r)
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	var buyDates []time.Time
	for _, e := range res.Events {
		if e.Action == "FRESH" {
			held[e.Symbol] = true
			buyDates = append(buyDates, e.Date)
		}
	}
	if held["C"] {
		t.Fatalf("the falling stock must never be bought, bought: %v", held)
	}
	if !held["A"] || !held["B"] {
		t.Fatalf("top 2 of 3 should be A and B, got %v", held)
	}
	for _, d := range buyDates {
		if !(d.Month() == 1 || d.Month() == 7) || d.Day() > 4 {
			t.Fatalf("buy on non-rebalance day %s", d.Format("2006-01-02"))
		}
	}
	for _, pt := range res.Points {
		if pt.Cash < -1e-6 {
			t.Fatalf("negative cash %.2f on %s", pt.Cash, pt.Date.Format("2006-01-02"))
		}
	}
	if len(res.Lots) != 2 {
		t.Fatalf("A and B bought once and held (retained names are not trimmed or re-bought): %d lots", len(res.Lots))
	}
}

func TestRotationSellsDropOutsAtNextRebalance(t *testing.T) {
	p := threeStockPanel()
	// flip: from 2020-07 make A fall hard so that it drops out of the top 2 by 2021-01.
	for sym, sr := range p.Series {
		if sym != "A" {
			continue
		}
		for i := range sr.Bars {
			if sr.Bars[i].Date.After(time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)) {
				f := math.Pow(0.90, float64(sr.Bars[i].Date.Sub(time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)).Hours()/24/30))
				sr.Bars[i].Close *= f
				sr.Bars[i].Open, sr.Bars[i].High, sr.Bars[i].Low = sr.Bars[i].Close, sr.Bars[i].Close, sr.Bars[i].Close
			}
		}
		for ds, days := range p.Days {
			for k := range days {
				if days[k].Symbol == "A" {
					for _, b := range sr.Bars {
						if b.Date.Format("2006-01-02") == ds {
							days[k].Bar = b
						}
					}
				}
			}
		}
	}
	r := rulesFor(func(r *Rules) {
		r.Rotation = &RotationRule{LookbackMonths: []int{6, 12}, TopN: 2, RebalanceMonths: []int{1, 7}}
	})
	res, err := Run(p, r)
	if err != nil {
		t.Fatal(err)
	}
	rot := 0
	for _, l := range res.Lots {
		if l.Reason == "rotate" {
			rot++
		}
	}
	if rot == 0 {
		t.Fatalf("expected the falling former winner to be sold at a later rebalance")
	}
}
