package sim

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/panel"
)

// RotationRule is a plain momentum rotation: on each rebalance day rank the current index members by the mean of their
// trailing returns over LookbackMonths, hold the TopN equally weighted, and sell whatever fell out.
// Retained names are not trimmed back to equal weight (that would only add tax and charges).
type RotationRule struct {
	LookbackMonths  []int `json:"lookback_months"`  // e.g. [6, 12]; the score is the mean of the trailing price returns
	TopN            int   `json:"top_n"`            // number of names held
	RebalanceMonths []int `json:"rebalance_months"` // calendar months (1-12) whose first trading day is a rebalance day
}

// closeOnOrBefore returns the last close at or before d in the symbol's full bar history, and whether history that
// old exists at all (the first bar must not be later than d).
func closeOnOrBefore(sr *panel.Series, d time.Time) (float64, bool) {
	bars := sr.Bars
	if len(bars) == 0 || bars[0].Date.After(d) {
		return 0, false
	}
	i := sort.Search(len(bars), func(i int) bool { return bars[i].Date.After(d) })
	return bars[i-1].Close, true
}

// runRotation is the day loop for a momentum rotation. It shares state, costs, tax, dividends and the output shape
// with Run, so every downstream tool (stages, scorecard, lottery) works unchanged.
func runRotation(p *panel.Panel, r Rules, dates []time.Time) (*Result, error) {
	rot := r.Rotation
	if rot.TopN <= 0 || len(rot.LookbackMonths) == 0 || len(rot.RebalanceMonths) == 0 {
		return nil, fmt.Errorf("sim: rotation needs top_n, lookback_months and rebalance_months")
	}
	rebalMonth := map[time.Month]bool{}
	for _, m := range rot.RebalanceMonths {
		rebalMonth[time.Month(m)] = true
	}
	res := &Result{Rules: r, StartCap: r.StartCapital}
	s := &state{r: r, p: p, cash: r.StartCapital, slot: r.StartCapital / r.CapitalDivider,
		lastPx: map[string]float64{}, armed: map[string]bool{}, peak: map[string]float64{}, res: res}
	prevDate := dates[0]
	curFY := fyOf(dates[0])
	lastRebalMonth := time.Month(0)
	lastRebalYear := 0
	first := true

	for _, date := range dates {
		ds := date.Format("2006-01-02")
		stocks := p.Days[ds]
		today := make(map[string]panel.Day, len(stocks))
		for _, d := range stocks {
			today[d.Symbol] = d
		}
		if fy := fyOf(date); fy != curFY {
			if r.Tax != nil {
				due := r.Tax.SettleFY(curFY)
				s.taxOwed += due
				res.TaxPaid += due
				res.TaxEvents = append(res.TaxEvents, TaxEvent{FY: curFY, Paid: due})
			}
			curFY = fy
		}
		s.payTax()
		if r.CashYield > 0 && s.cash > 0 {
			days := date.Sub(prevDate).Hours() / 24
			s.cash += s.cash * r.CashYield * days / 365
		}
		prevDate = date
		if r.Dividends {
			for _, l := range s.lots {
				if d, ok := today[l.Symbol]; ok && d.Dividend > 0 {
					amt := d.Dividend * float64(l.Qty)
					s.cash += amt
					l.Divs += amt
					res.DivReceived += amt
					if r.Tax != nil {
						r.Tax.RecordDividend(date, amt)
					}
				}
			}
		}
		for _, l := range s.lots {
			if d, ok := today[l.Symbol]; ok {
				if d.Bar.Close < l.MinClose {
					l.MinClose = d.Bar.Close
				}
				if d.Bar.Close > l.MaxClose {
					l.MaxClose = d.Bar.Close
				}
			}
		}

		// A rebalance day is the first trading day of a listed month, plus the first day of the run.
		newMonth := date.Month() != lastRebalMonth || date.Year() != lastRebalYear
		isRebal := first || (rebalMonth[date.Month()] && newMonth)
		if isRebal {
			lastRebalMonth, lastRebalYear = date.Month(), date.Year()
			first = false
			s.rebalance(today, stocks, date, rot)
		}

		for _, d := range stocks {
			s.lastPx[d.Symbol] = d.Bar.Close
		}
		hold := 0.0
		for _, l := range s.lots {
			px, ok := s.lastPx[l.Symbol]
			if !ok {
				px = l.BuyPrice
			}
			hold += px * float64(l.Qty)
		}
		res.Points = append(res.Points, Point{Date: date, Equity: s.cash + hold - s.taxOwed, Cash: s.cash, Holdings: hold, Lots: len(s.lots), Stocks: s.uniqueStocks()})
	}
	finish(s, res, curFY)
	return res, nil
}

// rebalance ranks, sells the drop-outs, then spends the cash equally on the new names.
func (s *state) rebalance(today map[string]panel.Day, stocks []panel.Day, date time.Time, rot *RotationRule) {
	type scored struct {
		sym   string
		score float64
	}
	var ranked []scored
	for _, d := range stocks {
		if !d.IsConstituent {
			continue
		}
		sr := s.p.Series[d.Symbol]
		if sr == nil {
			continue
		}
		sum, ok := 0.0, true
		for _, m := range rot.LookbackMonths {
			past, has := closeOnOrBefore(sr, date.AddDate(0, -m, 0))
			if !has || past <= 0 {
				ok = false
				break
			}
			sum += d.Bar.Close/past - 1
		}
		if ok {
			ranked = append(ranked, scored{d.Symbol, sum / float64(len(rot.LookbackMonths))})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].sym < ranked[j].sym
	})
	if len(ranked) > rot.TopN {
		ranked = ranked[:rot.TopN]
	}
	want := map[string]bool{}
	for _, x := range ranked {
		want[x.sym] = true
	}
	// Sell everything not wanted (only names that trade today can be sold; others stay).
	sold := map[string]bool{}
	for i := len(s.lots) - 1; i >= 0; i-- {
		l := s.lots[i]
		if want[l.Symbol] {
			continue
		}
		d, ok := today[l.Symbol]
		if !ok {
			continue
		}
		s.sell(l, d, "rotate", !sold[l.Symbol])
		sold[l.Symbol] = true
	}
	var fresh []scored
	for _, x := range ranked {
		if !s.has(x.sym) {
			fresh = append(fresh, x)
		}
	}
	if len(fresh) == 0 {
		return
	}
	alloc := s.cash / float64(len(fresh)) * 0.995 // leave room for charges
	for _, x := range fresh {
		d := today[x.sym]
		units := int(math.Floor(alloc / s.fillBuy(d)))
		if units <= 0 {
			s.res.Skipped.ZeroUnits++
			continue
		}
		s.buy(d, units, "FRESH", true)
	}
}

// finish does the end-of-run settlement shared with Run: last financial year, open lots, terminal liquidation tax.
func finish(s *state, res *Result, curFY int) {
	r := s.r
	last := res.Points[len(res.Points)-1]
	res.FinalEquity = last.Equity
	var open []LotRecord
	for _, l := range s.lots {
		px := s.lastPx[l.Symbol]
		if px == 0 {
			px = l.BuyPrice
		}
		open = append(open, LotRecord{Lot: *l, SellDate: last.Date, SellPrice: px, Open: true, Reason: "open", LastClose: px})
	}
	if r.Tax != nil {
		due := r.Tax.SettleFY(curFY)
		res.TaxPaid += due
		res.TaxEvents = append(res.TaxEvents, TaxEvent{FY: curFY, Paid: due})
		var sales []costs.Sale
		for _, o := range open {
			sales = append(sales, costs.Sale{Symbol: o.Symbol, BuyDate: o.BuyDate, SellDate: o.SellDate, Qty: float64(o.Qty),
				Cost: o.BuyPrice*float64(o.Qty) + o.BuyCharge.Deductible, Proceeds: o.SellPrice * float64(o.Qty)})
		}
		res.TerminalTax = r.Tax.TerminalTax(sales)
	}
	res.Lots = append(res.Lots, open...)
	sort.SliceStable(res.Lots, func(i, j int) bool { return res.Lots[i].BuyDate.Before(res.Lots[j].BuyDate) })
}
