// Package analytics computes the standard strategy-evaluation metrics from a daily equity curve, an optional
// benchmark curve and a list of lots (trades). It has no dependency on the simulator.
//
// Conventions (so numbers can be reproduced elsewhere):
//   - 252 trading days per year; CAGR uses calendar years = days/365.25 between the first and last observation,
//     with the starting capital as the opening value on the first date.
//   - Returns are simple daily returns of the equity curve (cash included, idle cash earns what the simulator gave it).
//   - Sharpe/Sortino subtract a constant annual risk-free rate (Rf) converted to daily by /252.
//   - Benchmark statistics (beta, alpha, IR, capture) use calendar-month returns, benchmark forward-filled to the
//     strategy's dates.
//   - A lot is "winning" if its net profit after charges is > 0. Open lots are marked at the last close.
package analytics

import (
	"math"
	"sort"
	"time"
)

// Point is one observation of a value series.
type Point struct {
	Date  time.Time
	Value float64
}

// Trade is one lot, closed or open at the end of the test.
type Trade struct {
	Symbol    string
	BuyDate   time.Time
	SellDate  time.Time // last date for open lots
	Qty       float64
	BuyPrice  float64
	SellPrice float64 // last mark for open lots
	Charges   float64 // all transaction charges paid on this lot (buy + sell side)
	Open      bool
	MinPrice  float64 // lowest close while held, 0 if unknown
}

// PnL is the lot's net profit in rupees (after charges).
func (t Trade) PnL() float64 { return (t.SellPrice-t.BuyPrice)*t.Qty - t.Charges }

// Return is the lot's net return on its cost.
func (t Trade) Return() float64 {
	c := t.BuyPrice * t.Qty
	if c == 0 {
		return 0
	}
	return t.PnL() / c
}

// HoldDays is calendar days held.
func (t Trade) HoldDays() float64 { return t.SellDate.Sub(t.BuyDate).Hours() / 24 }

// Input bundles everything Compute needs.
type Input struct {
	StartCapital float64
	Equity       []Point // daily, ascending
	Benchmark    []Point // any frequency >= daily coverage; may be nil
	Trades       []Trade
	Invested     []float64 // optional: holdings value per equity point (same length as Equity)
	Stocks       []int     // optional: number of distinct stocks held per equity point
	TradedValue  float64   // total buy value + sell value in rupees (for turnover)
	Rf           float64   // annual risk-free rate, e.g. 0.06
}

// YearReturn is one calendar year's return.
type YearReturn struct {
	Year    int
	Return  float64
	Partial bool
}

// Rolling summarises rolling-window CAGR.
type Rolling struct {
	Years       int
	Windows     int
	Min, Median float64
	Max         float64
	ShareAbove  float64 // share of windows with CAGR >= 15%
	ShareBelow0 float64
}

// Drawdown describes the deepest drawdown.
type Drawdown struct {
	Depth                                                  float64 // negative number, e.g. -0.32
	PeakDate                                               time.Time
	TroughDate                                             time.Time
	RecoveryDate                                           time.Time // zero if never recovered
	Recovered                                              bool
	DaysPeakToTrough, DaysTroughToRecovery, DaysUnderwater float64
}

// Underwater is the longest continuous stretch below a previous equity peak (textbook "maximum drawdown duration").
type Underwater struct {
	Days      float64
	PeakDate  time.Time
	EndDate   time.Time // recovery date, or the last date if never recovered
	Recovered bool
	Depth     float64 // deepest drawdown inside the stretch
}

// Relative holds benchmark-relative statistics.
type Relative struct {
	Beta, Alpha, Correlation, TrackingError, InfoRatio float64
	UpCapture, DownCapture                             float64
	Months                                             int
	BenchCAGR                                          float64
}

// Metrics is the full report for one equity curve.
type Metrics struct {
	Start, End               time.Time
	Years                    float64
	StartValue, EndValue     float64
	TotalReturn, CAGR        float64
	Vol, Sharpe, Sortino     float64
	MaxDD                    Drawdown
	LongestUnderwater        Underwater
	AvgDrawdown              float64 // mean of the daily drawdown series (negative)
	Skew, ExcessKurtosis     float64 // of daily returns
	VaR95, CVaR95            float64 // historical 1-day 95% VaR and expected shortfall as positive loss fractions
	BestDay, WorstDay        float64
	MaxLosingMonths          int // longest run of consecutive negative calendar months
	Calmar, Ulcer, UlcerPerf float64
	YearReturns              []YearReturn
	BestYear, WorstYear      YearReturn
	PositiveMonths           float64
	Months                   int
	Rolling                  []Rolling
	Rel                      *Relative

	// Trade statistics (lot level).
	ClosedLots, OpenLots                int
	WinRateClosed, WinRateAll           float64
	AvgWinPct, AvgLossPct               float64
	AvgWinRs, AvgLossRs                 float64
	WinLossRatio                        float64
	ProfitFactorClosed, ProfitFactorAll float64
	ExpectancyPct, ExpectancyRs         float64
	AvgHoldDays, MaxHoldDays            float64 // closed lots only
	AvgHoldDaysAll                      float64 // closed lots plus open lots held to the end
	LongestLosingHoldDays               float64 // longest calendar holding of any lot with net loss (closed or open)
	LongestOpenLoserDays                float64
	WorstLotDrawdown                    float64 // most negative min-close/buy-1 across lots
	OpenLoserCount                      int
	OpenUnrealisedPct                   float64 // unrealised P&L of open lots as % of final equity

	// Exposure.
	ExposureDays    float64 // share of days with at least one open lot
	AvgInvestedPct  float64 // mean holdings/equity
	MaxInvestedPct  float64
	AvgStocks       float64
	TurnoverPerYear float64 // (buys+sells)/2 / mean equity / years
	CostDragAnnual  float64 // (sum charges / mean equity / years)
}

// Compute derives all metrics.
func Compute(in Input) Metrics {
	var m Metrics
	eq := in.Equity
	if len(eq) < 2 {
		return m
	}
	m.Start, m.End = eq[0].Date, eq[len(eq)-1].Date
	m.Years = m.End.Sub(m.Start).Hours() / (24 * 365.25)
	if m.Years <= 0 {
		m.Years = 1.0 / 365.25
	}
	m.StartValue = in.StartCapital
	if m.StartValue <= 0 {
		m.StartValue = eq[0].Value
	}
	m.EndValue = eq[len(eq)-1].Value
	m.TotalReturn = m.EndValue/m.StartValue - 1
	if m.EndValue > 0 {
		m.CAGR = math.Pow(m.EndValue/m.StartValue, 1/m.Years) - 1
	} else {
		m.CAGR = -1
	}

	// Daily returns.
	rets := make([]float64, len(eq))
	prev := m.StartValue
	for i, p := range eq {
		rets[i] = p.Value/prev - 1
		prev = p.Value
	}
	mean, sd := meanStd(rets)
	rfd := in.Rf / 252
	m.Vol = sd * math.Sqrt(252)
	if sd > 0 {
		m.Sharpe = (mean - rfd) / sd * math.Sqrt(252)
	}
	var dd2 float64
	for _, r := range rets {
		if x := math.Min(0, r-rfd); x != 0 {
			dd2 += x * x
		}
	}
	if dd := math.Sqrt(dd2 / float64(len(rets))); dd > 0 {
		m.Sortino = (mean - rfd) / dd * math.Sqrt(252)
	} else if mean-rfd > 0 {
		m.Sortino = math.Inf(1) // never below the target
	}

	// Drawdown, ulcer.
	peak := m.StartValue
	curPeakDate := eq[0].Date
	var ulcerSum float64
	worst := Drawdown{}
	var worstPeakVal float64
	troughIdx := -1
	for i, p := range eq {
		if p.Value >= peak {
			peak, curPeakDate = p.Value, p.Date
		}
		dd := p.Value/peak - 1
		ulcerSum += (dd * 100) * (dd * 100)
		if dd < worst.Depth {
			worst.Depth, worst.PeakDate, worstPeakVal, troughIdx = dd, curPeakDate, peak, i
		}
	}
	if troughIdx >= 0 {
		worst.TroughDate = eq[troughIdx].Date
		worst.DaysPeakToTrough = worst.TroughDate.Sub(worst.PeakDate).Hours() / 24
		for j := troughIdx; j < len(eq); j++ {
			if eq[j].Value >= worstPeakVal {
				worst.Recovered, worst.RecoveryDate = true, eq[j].Date
				worst.DaysTroughToRecovery = worst.RecoveryDate.Sub(worst.TroughDate).Hours() / 24
				break
			}
		}
		end := m.End
		if worst.Recovered {
			end = worst.RecoveryDate
		}
		worst.DaysUnderwater = end.Sub(worst.PeakDate).Hours() / 24
	}
	m.MaxDD = worst
	{
		pk, pkDate := m.StartValue, eq[0].Date
		var cur Underwater
		inUW := false
		for _, p := range eq {
			if p.Value >= pk {
				if inUW {
					cur.EndDate, cur.Recovered = p.Date, true
					cur.Days = cur.EndDate.Sub(cur.PeakDate).Hours() / 24
					if cur.Days > m.LongestUnderwater.Days {
						m.LongestUnderwater = cur
					}
					inUW = false
				}
				pk, pkDate = p.Value, p.Date
				continue
			}
			if !inUW {
				inUW = true
				cur = Underwater{PeakDate: pkDate}
			}
			if dd := p.Value/pk - 1; dd < cur.Depth {
				cur.Depth = dd
			}
		}
		if inUW {
			cur.EndDate = eq[len(eq)-1].Date
			cur.Days = cur.EndDate.Sub(cur.PeakDate).Hours() / 24
			if cur.Days > m.LongestUnderwater.Days {
				m.LongestUnderwater = cur
			}
		}
	}
	m.Ulcer = math.Sqrt(ulcerSum/float64(len(eq))) / 100
	{
		pk := m.StartValue
		var sumDD float64
		for _, p := range eq {
			if p.Value > pk {
				pk = p.Value
			}
			sumDD += p.Value/pk - 1
		}
		m.AvgDrawdown = sumDD / float64(len(eq))
		sorted := append([]float64{}, rets...)
		sort.Float64s(sorted)
		m.WorstDay, m.BestDay = sorted[0], sorted[len(sorted)-1]
		k := int(0.05 * float64(len(sorted)))
		if k < 1 {
			k = 1
		}
		m.VaR95 = -sorted[k-1]
		var tail float64
		for _, r := range sorted[:k] {
			tail += r
		}
		m.CVaR95 = -tail / float64(k)
		if sd > 0 {
			var m3, m4 float64
			psd := sd * math.Sqrt(float64(len(rets)-1)/float64(len(rets))) // population sd for standardised moments
			for _, r := range rets {
				z := (r - mean) / psd
				m3 += z * z * z
				m4 += z * z * z * z
			}
			m.Skew = m3 / float64(len(rets))
			m.ExcessKurtosis = m4/float64(len(rets)) - 3
		}
	}
	if worst.Depth < 0 {
		m.Calmar = m.CAGR / -worst.Depth
	}
	if m.Ulcer > 0 {
		m.UlcerPerf = (m.CAGR - in.Rf) / m.Ulcer
	}

	// Calendar years and months.
	yEnd := map[int]float64{}
	var years []int
	for _, p := range eq {
		y := p.Date.Year()
		if _, ok := yEnd[y]; !ok {
			years = append(years, y)
		}
		yEnd[y] = p.Value
	}
	prevV := m.StartValue
	for i, y := range years {
		r := yEnd[y]/prevV - 1
		partial := (i == 0 && eq[0].Date.After(time.Date(y, 1, 10, 0, 0, 0, 0, time.UTC))) ||
			(i == len(years)-1 && eq[len(eq)-1].Date.Before(time.Date(y, 12, 20, 0, 0, 0, 0, time.UTC)))
		m.YearReturns = append(m.YearReturns, YearReturn{Year: y, Return: r, Partial: partial})
		prevV = yEnd[y]
	}
	first := true
	for _, yr := range m.YearReturns {
		if yr.Partial {
			continue
		}
		if first || yr.Return > m.BestYear.Return {
			m.BestYear = yr
		}
		if first || yr.Return < m.WorstYear.Return {
			m.WorstYear = yr
		}
		first = false
	}
	mv := monthEnds(eq, m.StartValue)
	pos := 0
	for _, r := range mv.ret {
		if r > 0 {
			pos++
		}
	}
	{
		run := 0
		for _, r := range mv.ret {
			if r < 0 {
				run++
				if run > m.MaxLosingMonths {
					m.MaxLosingMonths = run
				}
			} else {
				run = 0
			}
		}
	}
	m.Months = len(mv.ret)
	if m.Months > 0 {
		m.PositiveMonths = float64(pos) / float64(m.Months)
	}

	// Rolling CAGR.
	for _, n := range []int{1, 3, 5} {
		m.Rolling = append(m.Rolling, rolling(eq, n))
	}

	// Benchmark-relative.
	if len(in.Benchmark) > 1 {
		m.Rel = relative(eq, in.Benchmark, in.Rf)
	}

	tradeStats(&m, in)

	// Exposure.
	if len(in.Invested) == len(eq) {
		var inv, mx, withPos float64
		for i, p := range eq {
			f := 0.0
			if p.Value > 0 {
				f = in.Invested[i] / p.Value
			}
			inv += f
			if f > mx {
				mx = f
			}
			if in.Invested[i] > 0 {
				withPos++
			}
		}
		m.AvgInvestedPct = inv / float64(len(eq))
		m.MaxInvestedPct = mx
		m.ExposureDays = withPos / float64(len(eq))
	}
	if len(in.Stocks) == len(eq) {
		var s float64
		for _, v := range in.Stocks {
			s += float64(v)
		}
		m.AvgStocks = s / float64(len(in.Stocks))
	}
	var meanEq float64
	for _, p := range eq {
		meanEq += p.Value
	}
	meanEq /= float64(len(eq))
	if meanEq > 0 {
		m.TurnoverPerYear = in.TradedValue / 2 / meanEq / m.Years
		var ch float64
		for _, t := range in.Trades {
			ch += t.Charges
		}
		m.CostDragAnnual = ch / meanEq / m.Years
	}
	return m
}

func meanStd(x []float64) (float64, float64) {
	if len(x) == 0 {
		return 0, 0
	}
	var s float64
	for _, v := range x {
		s += v
	}
	m := s / float64(len(x))
	var ss float64
	for _, v := range x {
		ss += (v - m) * (v - m)
	}
	if len(x) < 2 {
		return m, 0
	}
	return m, math.Sqrt(ss / float64(len(x)-1))
}

type monthly struct {
	ret  []float64
	keys []string
	val  []float64
}

// monthEnds returns calendar-month returns of a series (first month measured from the opening value).
func monthEnds(eq []Point, open float64) monthly {
	var out monthly
	last := map[string]float64{}
	var keys []string
	for _, p := range eq {
		k := p.Date.Format("2006-01")
		if _, ok := last[k]; !ok {
			keys = append(keys, k)
		}
		last[k] = p.Value
	}
	prev := open
	for _, k := range keys {
		out.ret = append(out.ret, last[k]/prev-1)
		out.keys = append(out.keys, k)
		out.val = append(out.val, last[k])
		prev = last[k]
	}
	return out
}

func rolling(eq []Point, years int) Rolling {
	r := Rolling{Years: years}
	var vals []float64
	j := 0
	for i := range eq {
		target := eq[i].Date.AddDate(-years, 0, 0)
		if eq[0].Date.After(target) {
			continue
		}
		for j+1 < len(eq) && !eq[j+1].Date.After(target) {
			j++
		}
		vals = append(vals, math.Pow(eq[i].Value/eq[j].Value, 1/float64(years))-1)
	}
	r.Windows = len(vals)
	if len(vals) == 0 {
		return r
	}
	s := append([]float64{}, vals...)
	sort.Float64s(s)
	r.Min, r.Max = s[0], s[len(s)-1]
	r.Median = s[len(s)/2]
	var above, below int
	for _, v := range vals {
		if v >= 0.15 {
			above++
		}
		if v < 0 {
			below++
		}
	}
	r.ShareAbove = float64(above) / float64(len(vals))
	r.ShareBelow0 = float64(below) / float64(len(vals))
	return r
}

// alignTo forward-fills series b onto the dates of a.
func alignTo(a, b []Point) []float64 {
	out := make([]float64, len(a))
	j := -1
	for i, p := range a {
		for j+1 < len(b) && !b[j+1].Date.After(p.Date) {
			j++
		}
		if j < 0 {
			out[i] = math.NaN()
		} else {
			out[i] = b[j].Value
		}
	}
	return out
}

func relative(eq, bench []Point, rf float64) *Relative {
	bv := alignTo(eq, bench)
	var pts []Point
	for i, p := range eq {
		if !math.IsNaN(bv[i]) {
			pts = append(pts, Point{p.Date, bv[i]})
		}
	}
	if len(pts) < 40 {
		return nil
	}
	// Restrict strategy to the same dates.
	var sub []Point
	for _, p := range eq {
		if !p.Date.Before(pts[0].Date) {
			sub = append(sub, p)
		}
	}
	sm := monthEnds(sub, sub[0].Value)
	bm := monthEnds(pts, pts[0].Value)
	n := len(sm.ret)
	if len(bm.ret) < n {
		n = len(bm.ret)
	}
	if n < 6 {
		return nil
	}
	rs, rb := sm.ret[:n], bm.ret[:n]
	rfm := rf / 12
	var ms, mb float64
	for i := 0; i < n; i++ {
		ms += rs[i]
		mb += rb[i]
	}
	ms /= float64(n)
	mb /= float64(n)
	var cov, vb, vs float64
	for i := 0; i < n; i++ {
		cov += (rs[i] - ms) * (rb[i] - mb)
		vb += (rb[i] - mb) * (rb[i] - mb)
		vs += (rs[i] - ms) * (rs[i] - ms)
	}
	rel := &Relative{Months: n}
	if vb > 0 {
		rel.Beta = cov / vb
	}
	if vb > 0 && vs > 0 {
		rel.Correlation = cov / math.Sqrt(vb*vs)
	}
	rel.Alpha = 12 * ((ms - rfm) - rel.Beta*(mb-rfm))
	diff := make([]float64, n)
	for i := range diff {
		diff[i] = rs[i] - rb[i]
	}
	md, sdd := meanStd(diff)
	rel.TrackingError = sdd * math.Sqrt(12)
	if sdd > 0 {
		rel.InfoRatio = md * 12 / (sdd * math.Sqrt(12))
	}
	geo := func(r []float64, sel func(i int) bool) float64 {
		prod, k := 1.0, 0
		for i := range r {
			if sel(i) {
				prod *= 1 + r[i]
				k++
			}
		}
		if k == 0 {
			return 0
		}
		return math.Pow(prod, 1/float64(k)) - 1
	}
	up := func(i int) bool { return rb[i] > 0 }
	dn := func(i int) bool { return rb[i] < 0 }
	if g := geo(rb, up); g != 0 {
		rel.UpCapture = geo(rs, up) / g
	}
	if g := geo(rb, dn); g != 0 {
		rel.DownCapture = geo(rs, dn) / g
	}
	yrs := pts[len(pts)-1].Date.Sub(pts[0].Date).Hours() / (24 * 365.25)
	if yrs > 0 {
		rel.BenchCAGR = math.Pow(pts[len(pts)-1].Value/pts[0].Value, 1/yrs) - 1
	}
	return rel
}

func tradeStats(m *Metrics, in Input) {
	var closedPnL, openPnL, gains, losses, gainsAll, lossesAll float64
	var winN, lossN, winAllN, nAll int
	var sumWinPct, sumLossPct, sumWinRs, sumLossRs float64
	var holdSum, holdAll float64
	var holdN int
	var retSum float64
	var openPct float64
	for _, t := range in.Trades {
		pnl := t.PnL()
		ret := t.Return()
		nAll++
		if t.MinPrice > 0 && t.BuyPrice > 0 {
			if dd := t.MinPrice/t.BuyPrice - 1; dd < m.WorstLotDrawdown {
				m.WorstLotDrawdown = dd
			}
		}
		if pnl > 0 {
			winAllN++
			gainsAll += pnl
		} else if pnl < 0 {
			lossesAll += -pnl
		}
		if pnl < 0 && t.HoldDays() > m.LongestLosingHoldDays {
			m.LongestLosingHoldDays = t.HoldDays()
		}
		holdAll += t.HoldDays()
		if t.Open {
			m.OpenLots++
			openPnL += pnl
			if pnl < 0 {
				m.OpenLoserCount++
				if t.HoldDays() > m.LongestOpenLoserDays {
					m.LongestOpenLoserDays = t.HoldDays()
				}
			}
			continue
		}
		m.ClosedLots++
		closedPnL += pnl
		retSum += ret
		holdSum += t.HoldDays()
		holdN++
		if t.HoldDays() > m.MaxHoldDays {
			m.MaxHoldDays = t.HoldDays()
		}
		if pnl > 0 {
			winN++
			gains += pnl
			sumWinPct += ret
			sumWinRs += pnl
		} else if pnl < 0 {
			lossN++
			losses += -pnl
			sumLossPct += ret
			sumLossRs += pnl
		}
	}
	if m.ClosedLots > 0 {
		m.WinRateClosed = float64(winN) / float64(m.ClosedLots)
		m.ExpectancyPct = retSum / float64(m.ClosedLots)
		m.ExpectancyRs = closedPnL / float64(m.ClosedLots)
		m.AvgHoldDays = holdSum / float64(holdN)
	}
	if nAll > 0 {
		m.WinRateAll = float64(winAllN) / float64(nAll)
		m.AvgHoldDaysAll = holdAll / float64(nAll)
	}
	if winN > 0 {
		m.AvgWinPct, m.AvgWinRs = sumWinPct/float64(winN), sumWinRs/float64(winN)
	}
	if lossN > 0 {
		m.AvgLossPct, m.AvgLossRs = sumLossPct/float64(lossN), sumLossRs/float64(lossN)
	}
	if m.AvgLossPct < 0 {
		m.WinLossRatio = m.AvgWinPct / -m.AvgLossPct
	} else if m.AvgWinPct > 0 {
		m.WinLossRatio = math.Inf(1) // no closed losers
	}
	if losses > 0 {
		m.ProfitFactorClosed = gains / losses
	} else if gains > 0 {
		m.ProfitFactorClosed = math.Inf(1)
	}
	if lossesAll > 0 {
		m.ProfitFactorAll = gainsAll / lossesAll
	} else if gainsAll > 0 {
		m.ProfitFactorAll = math.Inf(1)
	}
	if m.EndValue > 0 {
		openPct = openPnL / m.EndValue
	}
	m.OpenUnrealisedPct = openPct
}
