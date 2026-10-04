package sim

import (
	"math"
	"reflect"
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

// fallFromJuly makes A fall 10% every 30 days from 2020-07-01, so that it drops out of the ranking within a few months.
func fallFromJuly(p *panel.Panel) {
	julyFirst := time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)
	sr := p.Series["A"]
	for i := range sr.Bars {
		if sr.Bars[i].Date.After(julyFirst) {
			f := math.Pow(0.90, sr.Bars[i].Date.Sub(julyFirst).Hours()/24/30)
			sr.Bars[i].Close *= f
			sr.Bars[i].Open, sr.Bars[i].High, sr.Bars[i].Low = sr.Bars[i].Close, sr.Bars[i].Close, sr.Bars[i].Close
		}
	}
	byDate := map[string]panel.Bar{}
	for _, b := range sr.Bars {
		byDate[b.Date.Format("2006-01-02")] = b
	}
	for ds, days := range p.Days {
		for k := range days {
			if days[k].Symbol == "A" {
				days[k].Bar = byDate[ds]
			}
		}
	}
}

func TestRotationSellsDropOutsAtNextRebalance(t *testing.T) {
	p := threeStockPanel()
	fallFromJuly(p)
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

// firstRotateSale returns the date of the first sale made because a stock left the top N.
func firstRotateSale(t *testing.T, res *Result) time.Time {
	t.Helper()
	for _, e := range res.Events {
		if e.Action == "SELL" && e.Reason == "rotate" {
			return e.Date
		}
	}
	t.Fatal("no rotation sale")
	return time.Time{}
}

func TestRotationMonthlySwapsSellAFallingLeaderSooner(t *testing.T) {
	// Held alone from Jan 2020, A starts falling in July. By 1 September its mean 6/12-month return is below flat B's,
	// so monthly swaps sell it then; half-yearly swaps hold it until January.
	run := func(name string) *Result {
		p := threeStockPanel()
		fallFromJuly(p)
		r, err := Preset(name)
		if err != nil {
			t.Fatal(err)
		}
		r.StartCapital = 1000000
		res, err := Run(p, r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if d := firstRotateSale(t, run("mom-plain-top1-1m")); d.Format("2006-01") != "2020-09" {
		t.Fatalf("monthly swaps: first sale %s, want September 2020", d.Format("2006-01-02"))
	}
	if d := firstRotateSale(t, run("mom-plain-top1-6m")); d.Format("2006-01") != "2021-01" {
		t.Fatalf("half-yearly swaps: first sale %s, want January 2021", d.Format("2006-01-02"))
	}
}

// noisyPanel: A rises 3% a month but jumps 10% up and back on alternate days after the first week of each month (high
// volatility), B rises a steady 2% a month (low volatility) and C falls a steady 1% a month. The first week of each
// month is noise-free, so the swap-day and lookback closes sit on the trend and A has the highest plain return.
func noisyPanel() *panel.Panel {
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	rate := map[string]float64{"A": 0.03, "B": 0.02, "C": -0.01}
	start := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	first := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for sym := range rate {
		p.Series[sym] = &panel.Series{Symbol: sym}
	}
	n := 0
	for d := start; !d.After(time.Date(2020, 3, 31, 0, 0, 0, 0, time.UTC)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		n++
		months := float64(d.Year()-2019)*12 + float64(d.Month()-1) + float64(d.Day())/31
		var day []panel.Day
		for _, sym := range []string{"A", "B", "C"} {
			px := 100 * math.Pow(1+rate[sym], months)
			if sym == "A" && d.Day() > 7 && n%2 == 1 {
				px *= 1.10
			}
			b := panel.Bar{Date: d, Open: px, High: px, Low: px, Close: px}
			p.Series[sym].Bars = append(p.Series[sym].Bars, b)
			if !d.Before(first) {
				day = append(day, panel.Day{Symbol: sym, Bar: b, IsConstituent: true})
			}
		}
		if !d.Before(first) {
			p.Dates = append(p.Dates, d)
			p.Days[d.Format("2006-01-02")] = day
		}
	}
	return p
}

func TestRotationNSEScoreDiscountsVolatility(t *testing.T) {
	bought := func(score string) string {
		r := rulesFor(func(r *Rules) {
			r.Rotation = &RotationRule{LookbackMonths: []int{6, 12}, TopN: 1, RebalanceMonths: []int{1, 7}, Score: score}
		})
		res, err := Run(noisyPanel(), r)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Events) == 0 || res.Events[0].Action != "FRESH" {
			t.Fatalf("score %q: expected a first-day purchase, events %v", score, res.Events)
		}
		return res.Events[0].Symbol
	}
	if got := bought("plain"); got != "A" {
		t.Fatalf("plain score picks the highest return A, got %s", got)
	}
	if got := bought("nse"); got != "B" {
		t.Fatalf("NSE score picks the steady riser B over the volatile A, got %s", got)
	}
}

// stepMarket is a market index flat at 100 through May 2020, 80 from June through December 2020, and 120 from 2021.
func stepMarket() []panel.Bar {
	var m []panel.Bar
	for d := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC); !d.After(time.Date(2021, 6, 30, 0, 0, 0, 0, time.UTC)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		v := 100.0
		if !d.Before(time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)) {
			v = 80
		}
		if !d.Before(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)) {
			v = 120
		}
		m = append(m, panel.Bar{Date: d, Close: v})
	}
	return m
}

func TestRotationMarketFilterHoldsCashBelowTheAverage(t *testing.T) {
	p := threeStockPanel()
	p.Market = stepMarket()
	r := rulesFor(func(r *Rules) {
		r.Rotation = &RotationRule{LookbackMonths: []int{6, 12}, TopN: 2, RebalanceMonths: []int{1, 7}, MarketMADays: 200}
	})
	res, err := Run(p, r)
	if err != nil {
		t.Fatal(err)
	}
	// The filter acts only on swap days: the drop in June leaves the holdings alone until July.
	july, jan := time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, pt := range res.Points {
		switch {
		case pt.Date.Before(july) && pt.Lots != 2:
			t.Fatalf("%s: above the average the top 2 are held, got %d lots", pt.Date.Format("2006-01-02"), pt.Lots)
		case !pt.Date.Before(july) && pt.Date.Before(jan) && pt.Lots != 0:
			t.Fatalf("%s: the July swap day is below the average, so cash until January, got %d lots", pt.Date.Format("2006-01-02"), pt.Lots)
		case !pt.Date.Before(jan) && pt.Lots != 2:
			t.Fatalf("%s: back above the average in January, got %d lots", pt.Date.Format("2006-01-02"), pt.Lots)
		}
	}
	market := 0
	for _, l := range res.Lots {
		if l.Reason == "market" {
			market++
		}
	}
	if market != 2 {
		t.Fatalf("both holdings sold by the filter in July, got %d", market)
	}
	p.Market = nil
	if _, err := Run(p, r); err == nil {
		t.Fatalf("a market filter without the index series must fail")
	}
}

func TestMomentumPresetNames(t *testing.T) {
	base, err := Preset("rotation-n50")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Preset("mom-plain-top10-6m")
	if err != nil {
		t.Fatal(err)
	}
	r.Name, r.Rotation.Score = base.Name, ""
	if !reflect.DeepEqual(r, base) {
		t.Fatalf("mom-plain-top10-6m must be rotation-n50:\n%+v\n%+v", *r.Rotation, *base.Rotation)
	}
	r, err = Preset("mom-nse-top20-1m-ma200-cash6")
	if err != nil {
		t.Fatal(err)
	}
	want := RotationRule{LookbackMonths: []int{6, 12}, TopN: 20, RebalanceMonths: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, Score: "nse", MarketMADays: 200}
	if !reflect.DeepEqual(*r.Rotation, want) || r.CashYield != 0.06 || r.Name != "mom-nse-top20-1m-ma200-cash6" {
		t.Fatalf("parsed %+v cash %v name %s", *r.Rotation, r.CashYield, r.Name)
	}
	for _, bad := range []string{"mom-fast-top10-6m", "mom-plain-10-6m", "mom-plain-top0-6m", "mom-plain-top10-3m", "mom-plain-top10-6m-ma", "mom-plain-top10-6m-cash6-ma200", "mom-plain-top10-6m-x"} {
		if _, err := Preset(bad); err == nil {
			t.Errorf("%s: expected an error", bad)
		}
	}
}
