// Package bench loads NSE index series (total-return and price) downloaded from niftyindices.com and values a
// buy-and-hold benchmark after Indian capital-gains tax.
package bench

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/costs"
)

func parse(path, dateKey, valKey string) ([]analytics.Point, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []analytics.Point
	for _, r := range rows {
		ds, _ := r[dateKey].(string)
		vs, _ := r[valKey].(string)
		d, err := time.Parse("02 Jan 2006", strings.TrimSpace(ds))
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(vs), ",", ""), 64)
		if err != nil || v <= 0 {
			continue
		}
		k := d.Format("2006-01-02")
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, analytics.Point{Date: d, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	if len(out) == 0 {
		return nil, fmt.Errorf("bench: no rows parsed from %s", path)
	}
	return out, nil
}

// LoadTRI loads a total-return index (gross dividends reinvested).
func LoadTRI(path string) ([]analytics.Point, error) { return parse(path, "Date", "TotalReturnsIndex") }

// LoadPRI loads the price-return index close.
func LoadPRI(path string) ([]analytics.Point, error) { return parse(path, "HistoricalDate", "CLOSE") }

// Clip returns points with start <= date <= end.
func Clip(p []analytics.Point, start, end time.Time) []analytics.Point {
	var out []analytics.Point
	for _, x := range p {
		if !x.Date.Before(start) && !x.Date.After(end) {
			out = append(out, x)
		}
	}
	return out
}

// At returns the last value on or before d.
func At(p []analytics.Point, d time.Time) (float64, bool) {
	i := sort.Search(len(p), func(i int) bool { return p[i].Date.After(d) })
	if i == 0 {
		return 0, false
	}
	return p[i-1].Value, true
}

// LessFee returns the series with a yearly fee taken out day by day, the way a fund's expense ratio lowers its value
// against the index it tracks: each value is scaled by (1-fee)^(years since the first point). Over any span of y years
// the return is the index's times (1-fee)^y.
func LessFee(p []analytics.Point, fee float64) []analytics.Point {
	if len(p) == 0 || fee == 0 {
		return p
	}
	out := make([]analytics.Point, len(p))
	for i, x := range p {
		years := x.Date.Sub(p[0].Date).Hours() / (24 * 365.25)
		out[i] = analytics.Point{Date: x.Date, Value: x.Value * math.Pow(1-fee, years)}
	}
	return out
}

// Result is the valuation of buying the index on start and holding to end.
type Result struct {
	Start, End time.Time
	Capital    float64
	GrossFinal float64
	GrossCAGR  float64
	Tax        float64
	NetFinal   float64
	NetCAGR    float64
	Years      float64
	TaxMode    costs.TaxMode
}

// BuyHold values a lump-sum purchase of the index at start (price = series value) held to end and then sold,
// the way an index-fund (growth option) investor would be taxed: nothing until redemption.
func BuyHold(series []analytics.Point, start, end time.Time, capital float64, mode costs.TaxMode) (Result, error) {
	v0, ok0 := At(series, start)
	v1, ok1 := At(series, end)
	if !ok0 || !ok1 {
		return Result{}, fmt.Errorf("bench: series does not cover %s..%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	units := capital / v0
	final := units * v1
	book := costs.NewTaxBook(mode)
	book.FMV31Jan2018 = func(string) (float64, bool) {
		if v, ok := At(series, time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC)); ok {
			return v * units, true
		}
		return 0, false
	}
	book.RecordSale(costs.Sale{Symbol: "BENCH", BuyDate: start, SellDate: end, Qty: 1, Cost: capital, Proceeds: final})
	tax := book.SettleFY(fyOf(end))
	years := end.Sub(start).Hours() / (24 * 365.25)
	return Result{Start: start, End: end, Capital: capital, GrossFinal: final, GrossCAGR: cagr(capital, final, years),
		Tax: tax, NetFinal: final - tax, NetCAGR: cagr(capital, final-tax, years), Years: years, TaxMode: mode}, nil
}

func fyOf(d time.Time) int {
	if d.Month() >= time.April {
		return d.Year() + 1
	}
	return d.Year()
}

func cagr(start, end, years float64) float64 {
	if end <= 0 || years <= 0 {
		return -1
	}
	return math.Pow(end/start, 1/years) - 1
}
