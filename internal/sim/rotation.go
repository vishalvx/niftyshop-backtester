package sim

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/panel"
)

// RotationRule is a momentum rotation: on each rebalance day rank the current index members by a momentum score, hold
// the TopN equally weighted, and sell whatever fell out. Retained names are not trimmed back to equal weight (that would
// only add tax and charges).
type RotationRule struct {
	LookbackMonths  []int `json:"lookback_months"`  // e.g. [6, 12]; the trailing price returns the score is built from
	TopN            int   `json:"top_n"`            // number of names held
	RebalanceMonths []int `json:"rebalance_months"` // calendar months (1-12) whose first trading day is a rebalance day
	// Score is "plain" (or empty): the mean of the trailing returns; or "nse": the Nifty200 Momentum 30 method, where each
	// trailing return is divided by the annualised volatility of daily log returns over the past 12 months, each ratio
	// becomes a z-score across the ranked members, and the score is the mean of the z-scores.
	Score string `json:"score,omitempty"`
	// MarketMADays, when above 0, is a market filter checked on rebalance days: while the universe's own price index
	// (Panel.Market) closes below its simple average of that many index days, sell everything and hold cash.
	MarketMADays int `json:"market_ma_days,omitempty"`
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
	if rot.Score != "" && rot.Score != "plain" && rot.Score != "nse" {
		return nil, fmt.Errorf("sim: unknown rotation score %q (want plain|nse)", rot.Score)
	}
	if rot.MarketMADays > 0 {
		if _, err := marketBelowMA(p.Market, dates[0], rot.MarketMADays); err != nil {
			return nil, err
		}
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
			riskOff := false
			if rot.MarketMADays > 0 {
				riskOff, _ = marketBelowMA(p.Market, date, rot.MarketMADays) // history checked on the first day
			}
			s.rebalance(today, stocks, date, rot, riskOff)
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

// marketBelowMA reports whether the last market close on or before d is below the simple average of the last n closes
// up to and including it. It is an error when the series does not hold n closes by d.
func marketBelowMA(m []panel.Bar, d time.Time, n int) (bool, error) {
	i := sort.Search(len(m), func(i int) bool { return m[i].Date.After(d) }) - 1
	if i+1 < n {
		return false, fmt.Errorf("sim: the market filter needs %d index closes by %s, the index price series has %d (load Panel.Market)", n, d.Format("2006-01-02"), i+1)
	}
	sum := 0.0
	for _, b := range m[i-n+1 : i+1] {
		sum += b.Close
	}
	return m[i].Close < sum/float64(n), nil
}

// trailingReturns returns the price return to today's close from the last close on or before each lookback, or false
// when the symbol has no price that old.
func trailingReturns(sr *panel.Series, close float64, date time.Time, months []int) ([]float64, bool) {
	out := make([]float64, len(months))
	for k, m := range months {
		past, has := closeOnOrBefore(sr, date.AddDate(0, -m, 0))
		if !has || past <= 0 {
			return nil, false
		}
		out[k] = close/past - 1
	}
	return out, true
}

// annualVol is the annualised standard deviation of daily log returns over the 12 months up to and including date.
func annualVol(sr *panel.Series, date time.Time) float64 {
	bars := sr.Bars
	from := date.AddDate(-1, 0, 0)
	lo := sort.Search(len(bars), func(i int) bool { return bars[i].Date.After(from) })
	hi := sort.Search(len(bars), func(i int) bool { return bars[i].Date.After(date) })
	var r []float64
	for i := max(lo, 1); i < hi; i++ {
		if bars[i-1].Close > 0 && bars[i].Close > 0 {
			r = append(r, math.Log(bars[i].Close/bars[i-1].Close))
		}
	}
	_, sd := meanSD(r)
	return sd * math.Sqrt(252)
}

// meanSD returns the mean and sample standard deviation (0 for fewer than two values).
func meanSD(x []float64) (m, sd float64) {
	if len(x) == 0 {
		return 0, 0
	}
	for _, v := range x {
		m += v
	}
	m /= float64(len(x))
	if len(x) < 2 {
		return m, 0
	}
	for _, v := range x {
		sd += (v - m) * (v - m)
	}
	return m, math.Sqrt(sd / float64(len(x)-1))
}

type scored struct {
	sym   string
	score float64
}

// momentumScores scores every member that has enough history, best first (ties by symbol).
func (s *state) momentumScores(stocks []panel.Day, date time.Time, rot *RotationRule) []scored {
	var syms []string
	var rets [][]float64
	for _, d := range stocks {
		if !d.IsConstituent {
			continue
		}
		sr := s.p.Series[d.Symbol]
		if sr == nil {
			continue
		}
		r, ok := trailingReturns(sr, d.Bar.Close, date, rot.LookbackMonths)
		if !ok {
			continue
		}
		if rot.Score == "nse" {
			vol := annualVol(sr, date)
			if vol <= 0 {
				continue
			}
			for k := range r {
				r[k] /= vol
			}
		}
		syms = append(syms, d.Symbol)
		rets = append(rets, r)
	}
	if rot.Score == "nse" { // each ratio as a z-score across the ranked members
		for k := range rot.LookbackMonths {
			col := make([]float64, len(rets))
			for i := range rets {
				col[i] = rets[i][k]
			}
			m, sd := meanSD(col)
			for i := range rets {
				if sd > 0 {
					rets[i][k] = (rets[i][k] - m) / sd
				} else {
					rets[i][k] = 0
				}
			}
		}
	}
	ranked := make([]scored, len(syms))
	for i, sym := range syms {
		sum := 0.0
		for _, v := range rets[i] {
			sum += v
		}
		ranked[i] = scored{sym, sum / float64(len(rets[i]))}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].sym < ranked[j].sym
	})
	return ranked
}

// rebalance ranks, sells the drop-outs, then spends the cash equally on the new names. With riskOff (the market filter
// is on) it sells everything and buys nothing.
func (s *state) rebalance(today map[string]panel.Day, stocks []panel.Day, date time.Time, rot *RotationRule, riskOff bool) {
	var ranked []scored
	if !riskOff {
		ranked = s.momentumScores(stocks, date, rot)
	}
	if len(ranked) > rot.TopN {
		ranked = ranked[:rot.TopN]
	}
	want := map[string]bool{}
	for _, x := range ranked {
		want[x.sym] = true
	}
	// Sell everything not wanted (only names that trade today can be sold; others stay).
	reason := "rotate"
	if riskOff {
		reason = "market"
	}
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
		s.sell(l, d, reason, !sold[l.Symbol])
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
