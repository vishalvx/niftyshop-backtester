// Package sim is a parameterised re-implementation of the NiftyShop day loop (internal/engine.RunNiftyShop).
//
// With LegacyRules() it reproduces the production engine trade for trade (see sim_equiv_test.go). Every other
// field is a rule variant: documented-spec semantics, stops, time exits, trailing exits, costs, dividends, tax.
package sim

import (
	"fmt"
	"math"
	"time"

	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/panel"
)

// PivotRule is the V-Pivot entry filter.
type PivotRule struct {
	System string `json:"system"` // classic | fibonacci | camarilla
	Level  string `json:"level"`  // S1 S2 S3 S4 closest
	Pool   int    `json:"pool"`
}

// Rules is one fully specified strategy variant. The zero value is not valid; start from LegacyRules().
// Every field has a JSON name, so a rule set (or a sweep over any field) can be written as configuration.
type Rules struct {
	Name string `json:"name"`

	// Parameters present in config.json.
	MAWindow       int        `json:"ma_window"`
	ProfitTarget   float64    `json:"profit_target"`   // 0.05 = sell at +5%
	AvgTrigger     float64    `json:"avg_trigger"`     // 0.03 = add when 3% below reference
	CapitalDivider float64    `json:"capital_divider"` // slot = (capital + realized) / divider
	StartCapital   float64    `json:"start_capital"`
	MaxFreshPerDay int        `json:"max_fresh_per_day"`
	Revision       string     `json:"revision"` // monthly | quarterly | yearly | sale (after every sale)
	Pivot          *PivotRule `json:"pivot,omitempty"`
	// Rotation, when set, replaces the whole pullback strategy with a momentum rotation (see rotation.go).
	Rotation *RotationRule `json:"rotation,omitempty"`

	// Semantics the production engine hard-codes (legacy value first).
	// "apptarget": each lot carries its own sell-at price fixed at purchase, as the app stores it: a fresh lot at fill*(1+target),
	// an averaged lot at the new weighted average cost*(1+target); with SyncTargets every lot of the stock takes the new price.
	ExitBasis   string `json:"exit_basis"`   // "lot": every lot sells at its own +target (engine). "avgcost": whole position at avg cost +target (documented spec, the app uses lot).
	SyncTargets bool   `json:"sync_targets"` // apptarget only: a new lot's target replaces the targets of the stock's older lots (the app V-Pivot)
	AvgBasis    string `json:"avg_basis"`    // "anylot": trigger vs ANY open lot's price (engine). "lastlot": vs most recent lot (documented). "avgcost": vs average cost.
	MaxStocks   int    `json:"max_stocks"`   // 0 = not enforced (engine ignores config max_stocks). 5 = documented spec.
	MaxLots     int    `json:"max_lots"`     // lots per stock, 0 = unlimited (engine). the app uses 3.
	// ExitOnIndexRemoval sells the whole position of a stock on the first day it is no longer a member of the index
	// (maintainer's rule, 2026-10-03). It only has an effect on universes with real removals (the point-in-time Nifty 50).
	ExitOnIndexRemoval bool `json:"exit_on_index_removal"`
	// MaxBuysPerDay caps fresh+average buys per day across the whole portfolio: 0 = engine (one fresh entry plus one
	// average per stock), 1 = the app ("maximum 1 buy action per day").
	MaxBuysPerDay int `json:"max_buys_per_day"`

	// Window restricts the run to [From, To] when set (the panel may be wider); capital starts fresh at From.
	From time.Time `json:"from,omitempty"`
	To   time.Time `json:"to,omitempty"`

	// Extra exit rules, all evaluated on the closing price and applied to the whole position of a symbol.
	StopLoss     float64 `json:"stop_loss"`      // 0 = off. Exit when Close <= avgcost*(1-StopLoss).
	TimeStopDays int     `json:"time_stop_days"` // 0 = off. Exit when calendar days since first lot >= N.
	TrailPct     float64 `json:"trail_pct"`      // 0 = off. After ProfitTarget is reached, exit on a TrailPct fall from the peak close (replaces the fixed exit).

	// Execution.
	FillNextOpen bool    `json:"fill_next_open"` // false = signal and fill on the same close (engine). true = fill at next bar open.
	SlippageBps  float64 `json:"slippage_bps"`   // adverse price move per side, in basis points of the fill price.

	// Accounting. Costs and Tax are attached by the experiment layer from named presets, not from the rules file.
	Dividends bool           `json:"dividends"`  // credit dividends on held shares (engine ignores them; Close is not dividend adjusted)
	CashYield float64        `json:"cash_yield"` // annual yield credited on idle cash (engine: 0)
	Costs     costs.Model    `json:"-"`          // nil = free
	Tax       *costs.TaxBook `json:"-"`          // nil = no tax
}

// LegacyRules returns the exact behaviour of the production engine with config.json defaults.
func LegacyRules() Rules {
	return Rules{
		Name: "standard-legacy", MAWindow: 20, ProfitTarget: 0.05, AvgTrigger: 0.03, CapitalDivider: 10,
		StartCapital: 100000, MaxFreshPerDay: 1, Revision: "monthly",
		ExitBasis: "lot", AvgBasis: "anylot",
	}
}

// SpecRules returns the rules as written in strategy.md / the V-Pivot playbook (what the docs say the engine does).
func SpecRules() Rules {
	r := LegacyRules()
	r.Name = "standard-spec"
	r.ExitBasis, r.AvgBasis, r.MaxStocks = "avgcost", "lastlot", 5
	return r
}

// Lot is one purchase. It is the unit that is sold in "lot" exit mode and the unit tax is computed on.
type Lot struct {
	ID        int
	Symbol    string
	BuyDate   time.Time
	BuyPrice  float64 // fill price including slippage
	RefPrice  float64 // signal close, used for trigger logic (equals BuyPrice when slippage is 0 and fills are at close)
	Qty       int
	BuyCharge costs.Charge
	Fresh     bool
	Target    float64 // sell-at price for ExitBasis "apptarget" (set when the lot, or a later lot of the same stock, is bought)
	MinClose  float64 // lowest close while held (for max adverse excursion)
	MaxClose  float64
	Divs      float64
}

// LotRecord is a closed or still-open lot as reported.
type LotRecord struct {
	Lot
	SellDate   time.Time
	SellPrice  float64 // fill price after slippage, or last close for an open lot
	SellCharge costs.Charge
	Open       bool
	Reason     string // target | stop | time | trail | open
	LastClose  float64
}

// Point is one daily equity observation (after all of that day's trading).
type Point struct {
	Date     time.Time
	Equity   float64
	Cash     float64
	Holdings float64
	Lots     int
	Stocks   int
}

// Event is every fill, in order.
type Event struct {
	Date   time.Time
	Symbol string
	Action string // FRESH AVG SELL
	Qty    int
	Price  float64
	LotID  int
	Reason string
}

// TaxEvent is one financial-year tax settlement.
type TaxEvent struct {
	FY   int // financial year ending 31 March of this year
	Paid float64
}

// Result is the output of one simulation.
type Result struct {
	Rules       Rules
	Points      []Point
	Lots        []LotRecord
	Events      []Event
	StartCap    float64
	FinalEquity float64
	CostsPaid   float64
	DivReceived float64
	TaxPaid     float64
	TaxEvents   []TaxEvent
	// TerminalTax is the extra tax that would be due if every open lot were sold at the last close.
	TerminalTax float64
	Skipped     struct {
		NoCash, ZeroUnits, MaxStocks int
	}
}

func isNewPeriod(cur, last time.Time, period string) bool {
	switch period {
	case "monthly":
		return cur.Year() != last.Year() || cur.Month() != last.Month()
	case "quarterly":
		return cur.Year() != last.Year() || (int(cur.Month())-1)/3 != (int(last.Month())-1)/3
	case "yearly":
		return cur.Year() != last.Year()
	}
	return false
}

func fyOf(d time.Time) int { // financial year label = calendar year in which it ends
	if d.Month() >= time.April {
		return d.Year() + 1
	}
	return d.Year()
}

type state struct {
	r         Rules
	p         *panel.Panel
	cash      float64
	realized  float64 // cash-basis realized P&L used for slot sizing
	slot      float64
	lots      []*Lot
	nextID    int
	lastPx    map[string]float64
	armed     map[string]bool
	peak      map[string]float64
	res       *Result
	buysToday int
	taxOwed   float64
}

func (s *state) cost() costs.Model {
	if s.r.Costs == nil {
		return costs.Zero{}
	}
	return s.r.Costs
}

// payTax settles tax owed out of available cash; it never overdraws. Unpaid tax stays a liability in equity.
func (s *state) payTax() {
	if s.taxOwed <= 0 || s.cash <= 0 {
		return
	}
	pay := math.Min(s.cash, s.taxOwed)
	s.cash -= pay
	s.taxOwed -= pay
	s.realized -= pay
}

// lotTarget is the sell-at price of one lot under the lot-level exit bases.
func (s *state) lotTarget(l *Lot) float64 {
	if s.r.ExitBasis == "apptarget" && l.Target > 0 {
		return l.Target
	}
	return l.BuyPrice * (1 + s.r.ProfitTarget)
}

func (s *state) has(sym string) bool {
	for _, l := range s.lots {
		if l.Symbol == sym {
			return true
		}
	}
	return false
}

func (s *state) uniqueStocks() int {
	seen := map[string]bool{}
	for _, l := range s.lots {
		seen[l.Symbol] = true
	}
	return len(seen)
}

func (s *state) fillBuy(d panel.Day) float64 {
	px := d.Bar.Close
	if s.r.FillNextOpen && d.NextOpen > 0 {
		px = d.NextOpen
	}
	return px * (1 + s.r.SlippageBps/1e4)
}

// slotUnits is the number of shares one slot buys at today's fill price, reduced if needed so that the shares plus the buy charges
// fit in the cash on hand (a buy made with cash just above one slot must not take cash below zero).
func (s *state) slotUnits(d panel.Day) int {
	px := s.fillBuy(d)
	units := int(math.Floor(s.slot / px))
	for units > 0 && float64(units)*px+s.cost().Buy(d.Bar.Date, float64(units)*px).Total > s.cash {
		units--
	}
	return units
}

func (s *state) fillSell(d panel.Day) float64 {
	px := d.Bar.Close
	if s.r.FillNextOpen && d.NextOpen > 0 {
		px = d.NextOpen
	}
	return px * (1 - s.r.SlippageBps/1e4)
}

func (s *state) avgCost(sym string) (cost float64, qty int, first time.Time) {
	for _, l := range s.lots {
		if l.Symbol == sym {
			cost += l.BuyPrice * float64(l.Qty)
			qty += l.Qty
			if first.IsZero() || l.BuyDate.Before(first) {
				first = l.BuyDate
			}
		}
	}
	if qty > 0 {
		cost /= float64(qty)
	}
	return
}

// sell closes one lot at today's fill price.
func (s *state) sell(l *Lot, d panel.Day, reason string, firstOfScripToday bool) {
	px := s.fillSell(d)
	value := px * float64(l.Qty)
	ch := s.cost().Sell(d.Bar.Date, value, firstOfScripToday)
	s.cash += value - ch.Total
	pnl := value - ch.Total - (l.BuyPrice*float64(l.Qty) + l.BuyCharge.Total)
	s.realized += pnl
	if s.r.Revision == "sale" { // maintainer's rule (2026-10-03): capital for the slot size is refreshed after every sale
		s.slot = (s.r.StartCapital + s.realized) / s.r.CapitalDivider
	}
	s.res.CostsPaid += ch.Total
	rec := LotRecord{Lot: *l, SellDate: d.Bar.Date, SellPrice: px, SellCharge: ch, Reason: reason, LastClose: d.Bar.Close}
	s.res.Lots = append(s.res.Lots, rec)
	s.res.Events = append(s.res.Events, Event{Date: d.Bar.Date, Symbol: l.Symbol, Action: "SELL", Qty: l.Qty, Price: px, LotID: l.ID, Reason: reason})
	if s.r.Tax != nil {
		s.r.Tax.RecordSale(costs.Sale{
			Symbol: l.Symbol, BuyDate: l.BuyDate, SellDate: d.Bar.Date, Qty: float64(l.Qty),
			Cost: l.BuyPrice*float64(l.Qty) + l.BuyCharge.Deductible, Proceeds: value - ch.Deductible,
		})
	}
	for i, x := range s.lots {
		if x == l {
			s.lots = append(s.lots[:i], s.lots[i+1:]...)
			break
		}
	}
}

func (s *state) buy(d panel.Day, units int, action string, fresh bool) {
	s.buysToday++
	px := s.fillBuy(d)
	value := px * float64(units)
	ch := s.cost().Buy(d.Bar.Date, value)
	s.cash -= value + ch.Total
	s.res.CostsPaid += ch.Total
	s.nextID++
	l := &Lot{ID: s.nextID, Symbol: d.Symbol, BuyDate: d.Bar.Date, BuyPrice: px, RefPrice: d.Bar.Close, Qty: units,
		BuyCharge: ch, Fresh: fresh, MinClose: d.Bar.Close, MaxClose: d.Bar.Close}
	s.lots = append(s.lots, l)
	if s.r.ExitBasis == "apptarget" {
		t := l.BuyPrice * (1 + s.r.ProfitTarget)
		if !fresh {
			avg, _, _ := s.avgCost(d.Symbol)
			t = avg * (1 + s.r.ProfitTarget)
		}
		l.Target = t
		if s.r.SyncTargets {
			for _, x := range s.lots {
				if x.Symbol == d.Symbol {
					x.Target = t
				}
			}
		}
	}
	s.res.Events = append(s.res.Events, Event{Date: d.Bar.Date, Symbol: d.Symbol, Action: action, Qty: units, Price: px, LotID: l.ID})
}

func pivotDistance(d panel.Day, rule *PivotRule) (float64, bool) {
	pv := d.Pivots
	var sup []float64
	switch rule.System {
	case "classic":
		switch rule.Level {
		case "S1":
			sup = []float64{pv.ClassicS1}
		case "S2":
			sup = []float64{pv.ClassicS2}
		case "S3":
			sup = []float64{pv.ClassicS3}
		case "closest":
			sup = []float64{pv.ClassicS1, pv.ClassicS2, pv.ClassicS3}
		}
	case "fibonacci":
		switch rule.Level {
		case "S1":
			sup = []float64{pv.FibS1}
		case "S2":
			sup = []float64{pv.FibS2}
		case "S3":
			sup = []float64{pv.FibS3}
		case "closest":
			sup = []float64{pv.FibS1, pv.FibS2, pv.FibS3}
		}
	case "camarilla":
		switch rule.Level {
		case "S1":
			sup = []float64{pv.CamS1}
		case "S2":
			sup = []float64{pv.CamS2}
		case "S3":
			sup = []float64{pv.CamS3}
		case "S4":
			sup = []float64{pv.CamS4}
		case "closest":
			sup = []float64{pv.CamS1, pv.CamS2, pv.CamS3, pv.CamS4}
		}
	}
	best, ok := math.MaxFloat64, false
	for _, v := range sup {
		if v > 0 && d.Bar.Close >= v {
			if dist := (d.Bar.Close - v) / d.Bar.Close * 100; dist < best {
				best, ok = dist, true
			}
		}
	}
	return best, ok
}

// Run simulates one rule set over a panel.
func Run(p *panel.Panel, r Rules) (*Result, error) {
	dates := p.Dates
	if !r.From.IsZero() || !r.To.IsZero() {
		dates = nil
		for _, d := range p.Dates {
			if (!r.From.IsZero() && d.Before(r.From)) || (!r.To.IsZero() && d.After(r.To)) {
				continue
			}
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return nil, fmt.Errorf("sim: empty panel")
	}
	if r.ExitBasis == "" {
		r.ExitBasis = "lot"
	}
	if r.AvgBasis == "" {
		r.AvgBasis = "anylot"
	}
	if r.TrailPct > 0 {
		r.ExitBasis = "avgcost"
	}
	if r.Rotation != nil {
		return runRotation(p, r, dates)
	}
	res := &Result{Rules: r, StartCap: r.StartCapital}
	s := &state{r: r, p: p, cash: r.StartCapital, slot: r.StartCapital / r.CapitalDivider,
		lastPx: map[string]float64{}, armed: map[string]bool{}, peak: map[string]float64{}, res: res}
	lastRev := dates[0]
	prevDate := dates[0]
	curFY := fyOf(dates[0])

	for _, date := range dates {
		s.buysToday = 0
		ds := date.Format("2006-01-02")
		stocks := p.Days[ds]
		today := make(map[string]panel.Day, len(stocks))
		for _, d := range stocks {
			today[d.Symbol] = d
		}

		// Financial-year rollover: settle tax for the year that just ended.
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

		// Idle-cash yield over the calendar days since the previous trading day.
		if r.CashYield > 0 && s.cash > 0 {
			days := date.Sub(prevDate).Hours() / 24
			s.cash += s.cash * r.CashYield * days / 365
		}
		prevDate = date

		if isNewPeriod(date, lastRev, r.Revision) {
			s.slot = (r.StartCapital + s.realized) / r.CapitalDivider
			lastRev = date
		}

		// Dividends going ex today on shares held.
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

		// Track excursions and peaks.
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

		sellDone := map[string]bool{}
		// ---- 0. A stock that has left the index is sold, whatever its price ----
		if r.ExitOnIndexRemoval {
			var gone []string
			seenG := map[string]bool{}
			for _, l := range s.lots {
				if seenG[l.Symbol] {
					continue
				}
				seenG[l.Symbol] = true
				if d, ok := today[l.Symbol]; ok && !d.IsConstituent {
					gone = append(gone, l.Symbol)
				}
			}
			for _, sym := range gone {
				d := today[sym]
				for i := len(s.lots) - 1; i >= 0; i-- {
					if s.lots[i].Symbol == sym {
						s.sell(s.lots[i], d, "index", !sellDone[sym])
						sellDone[sym] = true
					}
				}
				delete(s.armed, sym)
				delete(s.peak, sym)
			}
		}
		// ---- 1. Exits ----
		if (r.ExitBasis == "lot" || r.ExitBasis == "apptarget") && r.StopLoss == 0 && r.TimeStopDays == 0 {
			for i := len(s.lots) - 1; i >= 0; i-- {
				l := s.lots[i]
				d, ok := today[l.Symbol]
				if !ok {
					continue
				}
				if d.Bar.Close >= s.lotTarget(l) {
					s.sell(l, d, "target", !sellDone[l.Symbol])
					sellDone[l.Symbol] = true
				}
			}
		} else {
			// Position-level evaluation, deterministic symbol order by first lot.
			var order []string
			seen := map[string]bool{}
			for _, l := range s.lots {
				if !seen[l.Symbol] {
					seen[l.Symbol] = true
					order = append(order, l.Symbol)
				}
			}
			for _, sym := range order {
				d, ok := today[sym]
				if !ok {
					continue
				}
				avg, _, first := s.avgCost(sym)
				cl := d.Bar.Close
				reason := ""
				if r.StopLoss > 0 && cl <= avg*(1-r.StopLoss) {
					reason = "stop"
				} else if r.TimeStopDays > 0 && date.Sub(first).Hours()/24 >= float64(r.TimeStopDays) {
					reason = "time"
				} else if r.TrailPct > 0 {
					if !s.armed[sym] && cl >= avg*(1+r.ProfitTarget) {
						s.armed[sym] = true
						s.peak[sym] = cl
					}
					if s.armed[sym] {
						if cl > s.peak[sym] {
							s.peak[sym] = cl
						}
						if cl <= s.peak[sym]*(1-r.TrailPct) {
							reason = "trail"
						}
					}
				} else if r.ExitBasis == "avgcost" && cl >= avg*(1+r.ProfitTarget) {
					reason = "target"
				}
				if reason != "" {
					for i := len(s.lots) - 1; i >= 0; i-- {
						if s.lots[i].Symbol == sym {
							s.sell(s.lots[i], d, reason, !sellDone[sym])
							sellDone[sym] = true
						}
					}
					delete(s.armed, sym)
					delete(s.peak, sym)
				} else if (r.ExitBasis == "lot" || r.ExitBasis == "apptarget") && r.TrailPct == 0 {
					// lot-level target alongside stop/time rules
					for i := len(s.lots) - 1; i >= 0; i-- {
						l := s.lots[i]
						if l.Symbol == sym && d.Bar.Close >= s.lotTarget(l) {
							s.sell(l, d, "target", !sellDone[sym])
							sellDone[sym] = true
						}
					}
				}
			}
		}

		s.payTax()

		// ---- 2. Averaging ----
		avgDone := map[string]bool{}
		seenSym := map[string]bool{}
		for i := len(s.lots) - 1; i >= 0; i-- {
			l := s.lots[i]
			if avgDone[l.Symbol] {
				continue
			}
			if r.AvgBasis != "anylot" {
				if seenSym[l.Symbol] {
					continue
				}
				seenSym[l.Symbol] = true
			}
			d, ok := today[l.Symbol]
			if !ok || !d.IsConstituent || d.Bar.Close <= 0 {
				continue
			}
			ref := l.BuyPrice
			if r.AvgBasis == "avgcost" {
				ref, _, _ = s.avgCost(l.Symbol)
			}
			if d.Bar.Close > ref*(1-r.AvgTrigger) {
				continue
			}
			if r.MaxLots > 0 {
				n := 0
				for _, x := range s.lots {
					if x.Symbol == l.Symbol {
						n++
					}
				}
				if n >= r.MaxLots {
					continue
				}
			}
			if r.MaxBuysPerDay > 0 && s.buysToday >= r.MaxBuysPerDay {
				continue
			}
			if s.cash < s.slot {
				res.Skipped.NoCash++
				continue
			}
			units := s.slotUnits(d)
			if units <= 0 {
				res.Skipped.ZeroUnits++
				continue
			}
			s.buy(d, units, "AVG", false)
			avgDone[l.Symbol] = true
		}

		// ---- 3. Fresh entries ----
		fresh := 0
		maxStocksReached := func() bool { return r.MaxStocks > 0 && s.uniqueStocks() >= r.MaxStocks }
		buysCapped := func() bool { return r.MaxBuysPerDay > 0 && s.buysToday >= r.MaxBuysPerDay }
		if r.Pivot != nil {
			for fresh < r.MaxFreshPerDay {
				if s.cash < s.slot || buysCapped() {
					break
				}
				if maxStocksReached() {
					res.Skipped.MaxStocks++
					break
				}
				var cands []panel.Day
				for _, d := range stocks {
					if d.IsConstituent && d.DiffSMA > 0 && !s.has(d.Symbol) {
						cands = append(cands, d)
					}
				}
				if len(cands) == 0 {
					break
				}
				pool := r.Pivot.Pool
				if pool <= 0 {
					pool = 5
				}
				if len(cands) > pool {
					cands = cands[:pool]
				}
				var best panel.Day
				bestDist, found := math.MaxFloat64, false
				for _, d := range cands {
					if dist, ok := pivotDistance(d, r.Pivot); ok && dist < bestDist {
						bestDist, best, found = dist, d, true
					}
				}
				if !found {
					break
				}
				units := s.slotUnits(best)
				if units <= 0 {
					res.Skipped.ZeroUnits++
					break
				}
				s.buy(best, units, "FRESH", true)
				fresh++
			}
		} else {
			for _, d := range stocks {
				if !(d.IsConstituent && d.DiffSMA > 0) {
					continue
				}
				if fresh >= r.MaxFreshPerDay || buysCapped() {
					break
				}
				if s.has(d.Symbol) {
					continue
				}
				if maxStocksReached() {
					res.Skipped.MaxStocks++
					break
				}
				if s.cash < s.slot {
					res.Skipped.NoCash++
					break
				}
				units := s.slotUnits(d)
				if units > 0 {
					s.buy(d, units, "FRESH", true)
					fresh++
				} else {
					res.Skipped.ZeroUnits++
				}
			}
		}

		// ---- mark to market ----
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
