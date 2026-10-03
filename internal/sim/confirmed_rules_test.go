package sim

// Verification of the confirmed rule set (captain, 2026-10-03) for the presets nsx and nsx-lot, with and without a pivot suffix.
//
// The rules under test (R1-R8):
//   R1 universe: members of the index, checked once a day at the close.
//   R2 new stock: the member furthest below its 20-day average that is not held; no cap on how many stocks (cash only: a buy needs cash >= one slot).
//   R3 slot = (start capital + realised profit) / 10, refreshed after EVERY sale; before the first sale it is start capital / 10.
//   R4 add (average down): close <= 97% of the price of the stock's latest OPEN lot and fewer than 3 OPEN lots.
//   R5 sell: nsx sells the whole position at close >= weighted average cost x 1.05; nsx-lot sells each lot at close >= its own entry x 1.05.
//   R6 a stock that is no longer a member is sold in full that day at the close, at any price.
//   R7 no stop loss, no time limit.
//   R8 order of the day: exits first (any number), then at most ONE purchase (adds before a new stock); sale cash is reusable the same day.
//
// The property tests replay Result.Events, Result.Lots and Result.Points from the outside (never from sim.go state) and fail on the
// first deviation. How the numbers are derived from the event history:
//   - realised profit (R3, test f) = the sum over every SELL event so far of (sell price x qty) minus (buy price x qty) of THAT lot,
//     with the lot's buy price taken from its FRESH/AVG event. Gross stage, so there are no charges and no tax; dividends are off.
//   - slot before a buy = (StartCapital + realised profit from all events before this buy, including same-day sales) / 10.
//   - expected shares = floor(slot / price), compared with the Qty of the buy event.
//   - cash is replayed from the events (start capital, plus sales, minus buys) and compared with Result.Points after every day.

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/indicators"
	"github.com/vishalvx/back-tester/internal/panel"
)

// The confirmed numbers. They are variables on purpose: the checker then does the same float64 arithmetic at run time as sim.go.
var (
	crProfit  = 0.05
	crTrigger = 0.03
)

const (
	crMaxLots  = 3
	crDivider  = 10.0
	crStartCap = 100000.0
)

var crVariants = []string{"nsx", "nsx-lot", "nsx:camarilla:S1:5", "nsx-lot:fibonacci:S1:15"}

// crRules mirrors presetByName in cmd/research/cmd_run.go (package main cannot be imported); TestConfirmedRulesPresetShape pins the fields.
func crRules(t testing.TB, name string) Rules {
	t.Helper()
	parts := strings.Split(name, ":")
	if parts[0] != "nsx" && parts[0] != "nsx-lot" {
		t.Fatalf("bad preset %q", name)
	}
	r := SpecRules()
	r.MaxStocks = 0
	r.MaxLots = 3
	r.MaxBuysPerDay = 1
	r.ExitOnIndexRemoval = true
	r.Revision = "sale"
	if parts[0] == "nsx-lot" {
		r.ExitBasis = "lot"
	}
	if len(parts) == 4 {
		pool, err := strconv.Atoi(parts[3])
		if err != nil {
			t.Fatalf("bad preset %q", name)
		}
		r.Pivot = &PivotRule{System: parts[1], Level: parts[2], Pool: pool}
	} else if len(parts) != 1 {
		t.Fatalf("bad preset %q", name)
	}
	r.Name = name
	r.StartCapital = crStartCap
	return r
}

func TestConfirmedRulesPresetShape(t *testing.T) {
	for _, name := range crVariants {
		r := crRules(t, name)
		lot := strings.HasPrefix(name, "nsx-lot")
		ok := r.MAWindow == 20 && r.ProfitTarget == 0.05 && r.AvgTrigger == 0.03 && r.CapitalDivider == 10 && r.MaxFreshPerDay == 1 &&
			r.Revision == "sale" && r.AvgBasis == "lastlot" && r.MaxStocks == 0 && r.MaxLots == 3 && r.MaxBuysPerDay == 1 &&
			r.ExitOnIndexRemoval && r.StopLoss == 0 && r.TimeStopDays == 0 && r.TrailPct == 0 && !r.FillNextOpen && r.SlippageBps == 0 &&
			r.Costs == nil && r.Tax == nil && r.Rotation == nil && !r.Dividends && r.CashYield == 0 &&
			((lot && r.ExitBasis == "lot") || (!lot && r.ExitBasis == "avgcost")) &&
			(strings.Count(name, ":") == 0) == (r.Pivot == nil)
		if !ok {
			t.Errorf("%s: preset fields drifted from the confirmed rules: %+v", name, r)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Panels

// crBar is one stock on one day of a hand-built panel.
type crBar struct {
	sym   string
	close float64
	diff  float64 // (SMA - close) / close * 100; > 0 means below the 20-day average
	out   bool    // true: not a member of the index that day
	cam   float64 // support level for the S1 of every pivot system (classic, fibonacci, camarilla), only read by the pivot variants; 0 = none
}

func crSortDays(list []panel.Day) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].DiffSMA != list[j].DiffSMA {
			return list[i].DiffSMA > list[j].DiffSMA
		}
		return list[i].Symbol < list[j].Symbol
	})
}

// crHandPanel builds a multi-stock panel on consecutive weekdays from 2020-01-01; the day list is sorted like panel.Build does.
func crHandPanel(days [][]crBar) *panel.Panel {
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	d := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, bars := range days {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		var list []panel.Day
		for _, b := range bars {
			list = append(list, panel.Day{Symbol: b.sym, IsConstituent: !b.out, DiffSMA: b.diff,
				Pivots: indicators.PivotLevels{CamS1: b.cam, FibS1: b.cam, ClassicS1: b.cam},
				Bar:    panel.Bar{Date: d, Open: b.close, High: b.close, Low: b.close, Close: b.close}})
		}
		crSortDays(list)
		ds := d.Format("2006-01-02")
		p.Dates = append(p.Dates, d)
		p.Days[ds] = list
		d = d.AddDate(0, 0, 1)
	}
	return p
}

// crRandomPanel builds nSyms mean-reverting random walks with crash streaks (so averaging down, the lot limit, own-lot targets and
// the whole-position target all trigger often), a few missing bars, previous-bar pivots, and a membership flag that changes part way:
// about half of the symbols drop out of the index for good or for a while, and some join late.
func crRandomPanel(seed int64, nSyms, nDays int) *panel.Panel {
	rng := rand.New(rand.NewSource(seed))
	var dates []time.Time
	for d := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC); len(dates) < nDays; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			dates = append(dates, d)
		}
	}
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	p.Dates = dates[19:]
	for s := 0; s < nSyms; s++ {
		sym := fmt.Sprintf("T%02d", s)
		join, drop, rejoin := 0, nDays+1, nDays+1
		if rng.Float64() < 0.2 {
			join = 25 + rng.Intn(150)
		}
		if rng.Float64() < 0.55 {
			drop = nDays/4 + rng.Intn(nDays/2)
			if rng.Float64() < 0.5 {
				rejoin = drop + 15 + rng.Intn(60)
			}
		}
		px := 20 + rng.Float64()*380
		anchor := px
		crash, rate := 0, 0.0
		var bars []panel.Bar
		var idx []int
		for i, d := range dates {
			var ret float64
			if crash > 0 {
				ret = -rate * (0.8 + 0.4*rng.Float64())
				crash--
			} else {
				ret = rng.NormFloat64()*0.02 + 0.05*(anchor-px)/px
				if rng.Float64() < 0.03 {
					crash, rate = 3+rng.Intn(6), 0.03+rng.Float64()*0.03
				}
			}
			px = math.Max(math.Round(px*(1+ret)*20)/20, 1) // 5 paise tick, so exact ties at the targets occur
			anchor *= 1.0003
			o := px * (1 + rng.NormFloat64()*0.005)
			if rng.Float64() < 0.03 {
				continue // no bar for this symbol today
			}
			bars = append(bars, panel.Bar{Date: d, Open: o, Close: px,
				High: math.Max(px, o) * (1 + rng.Float64()*0.01), Low: math.Min(px, o) * (1 - rng.Float64()*0.01)})
			idx = append(idx, i)
		}
		for k, b := range bars {
			if k < 19 {
				continue // the average needs 20 bars
			}
			i := idx[k]
			sum := 0.0
			for _, x := range bars[k-19 : k+1] {
				sum += x.Close
			}
			sma := sum / 20
			day := panel.Day{Symbol: sym, Bar: b, SMA: sma, DiffSMA: (sma - b.Close) / b.Close * 100,
				IsConstituent: i >= join && (i < drop || i >= rejoin),
				Pivots:        indicators.CalculatePivotLevels(bars[k-1].High, bars[k-1].Low, bars[k-1].Close)}
			if k+1 < len(bars) {
				day.NextOpen = bars[k+1].Open
			}
			ds := b.Date.Format("2006-01-02")
			p.Days[ds] = append(p.Days[ds], day)
		}
	}
	for _, l := range p.Days {
		crSortDays(l)
	}
	return p
}

// ---------------------------------------------------------------------------------------------------------------------
// Independent replay of a Result

type crObs struct {
	fresh, avg, sells, indexSells, targetSells int
	maxStocks, maxOpenLots                     int
	addBeforeFresh                             int // a day with an add although a new stock was also available
	multiSellDays                              int
	partialSells                               int // a lot sold while other lots of the same stock stayed open
	reuseBuys                                  int // a buy that was only possible because of cash from a sale the same day
	threeLotStocks                             int
	// not rules, only to describe what the panels exercised (the rule set does not say how these cases go)
	multiAddDays   int // several held stocks qualified for the single add of the day
	rebuyAfterSale int // a stock sold today was bought again as a new stock today
	addAfterSale   int // a stock that sold a lot today added a lot today
}

func crSupport(d panel.Day, rule *PivotRule) float64 {
	switch rule.System + ":" + rule.Level {
	case "camarilla:S1":
		return d.Pivots.CamS1
	case "fibonacci:S1":
		return d.Pivots.FibS1
	case "classic:S1":
		return d.Pivots.ClassicS1
	}
	panic("pivot variant not supported by the test oracle: " + rule.System + ":" + rule.Level)
}

type crOpen struct {
	id  int
	px  float64
	qty int
}

func crAvg(ls []crOpen) float64 {
	cost, qty := 0.0, 0
	for _, l := range ls {
		cost += l.px * float64(l.qty)
		qty += l.qty
	}
	return cost / float64(qty)
}

// crReplay checks res against R1-R8 and returns every problem it finds (empty means the run follows the rules).
func crReplay(p *panel.Panel, res *Result) (crObs, []string) {
	r := res.Rules
	lotMode := r.ExitBasis == "lot"
	var obs crObs
	var problems []string
	fail := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	open := map[string][]crOpen{}
	cash := r.StartCapital
	realized := 0.0
	buyPx := map[int]float64{}
	ledger := map[int]LotRecord{}
	for _, lr := range res.Lots {
		ledger[lr.ID] = lr
	}
	ei := 0
	for _, pt := range res.Points {
		ds := pt.Date.Format("2006-01-02")
		list := p.Days[ds]
		today := map[string]panel.Day{}
		for _, d := range list {
			today[d.Symbol] = d
		}
		cashAtOpen := cash
		avgAtSell := map[string]float64{}
		soldSyms := map[string]bool{}
		sellsToday, buysToday := 0, 0
		snapDone := false

		slotNow := func() float64 { return (r.StartCapital + realized) / crDivider }
		// units a slot buys at a price
		units := func(px float64) int { return int(math.Floor(slotNow() / px)) }

		// snap runs once per day, after the exits and before the first purchase (nb = that purchase, nil if none).
		snap := func(nb *Event) {
			snapDone = true
			slot := slotNow()
			syms := make([]string, 0, len(open))
			for s := range open {
				syms = append(syms, s)
			}
			sort.Strings(syms)
			// R6 and R5 completeness: nothing that must be sold is still held.
			for _, sym := range syms {
				d, traded := today[sym]
				if !traded {
					continue
				}
				if !d.IsConstituent {
					fail("%s R6: %s is not a member but %d lots are still held after the exits", ds, sym, len(open[sym]))
					continue
				}
				if lotMode {
					for _, l := range open[sym] {
						if d.Bar.Close >= l.px*(1+crProfit) {
							fail("%s R5: lot %d of %s is held although close %.4f >= its own entry %.4f x 1.05", ds, l.id, sym, d.Bar.Close, l.px)
						}
					}
				} else if avg := crAvg(open[sym]); d.Bar.Close >= avg*(1+crProfit) {
					fail("%s R5: %s is held although close %.4f >= average cost %.4f x 1.05", ds, sym, d.Bar.Close, avg)
				}
			}
			for sym := range soldSyms {
				if !lotMode && len(open[sym]) != 0 {
					fail("%s R5: %s was sold only in part under whole-position selling", ds, sym)
				}
			}
			// R4: which held stocks may add today
			var addCand []string
			for _, sym := range syms {
				d, traded := today[sym]
				ls := open[sym]
				if !traded || !d.IsConstituent || d.Bar.Close <= 0 || len(ls) >= crMaxLots {
					continue
				}
				if d.Bar.Close > ls[len(ls)-1].px*(1-crTrigger) { // latest OPEN lot
					continue
				}
				if cash < slot || units(d.Bar.Close) <= 0 {
					continue
				}
				addCand = append(addCand, sym)
			}
			// R2: the new stock
			var best panel.Day
			haveBest := false
			if r.Pivot == nil {
				for _, d := range list {
					if d.IsConstituent && d.DiffSMA > 0 && len(open[d.Symbol]) == 0 && cash >= slot && units(d.Bar.Close) > 0 {
						best, haveBest = d, true
						break
					}
				}
			} else {
				pool := r.Pivot.Pool
				if pool <= 0 {
					pool = 5
				}
				var cands []panel.Day
				for _, d := range list {
					if d.IsConstituent && d.DiffSMA > 0 && len(open[d.Symbol]) == 0 {
						cands = append(cands, d)
					}
				}
				if len(cands) > pool {
					cands = cands[:pool]
				}
				bestDist := math.MaxFloat64
				for _, d := range cands {
					if v := crSupport(d, r.Pivot); v > 0 && d.Bar.Close >= v {
						if dist := (d.Bar.Close - v) / d.Bar.Close * 100; dist < bestDist {
							bestDist, best = dist, d
							haveBest = true
						}
					}
				}
				if haveBest && !(cash >= slot && units(best.Bar.Close) > 0) {
					haveBest = false
				}
			}
			if len(addCand) > 1 {
				obs.multiAddDays++
			}
			switch {
			case nb == nil:
				if len(addCand) > 0 {
					fail("%s R8: an add was possible (%v) but no purchase was made", ds, addCand)
				}
				if haveBest {
					fail("%s R2: %s could have been bought (cash %.2f, slot %.2f) but no purchase was made", ds, best.Symbol, cash, slot)
				}
			case len(addCand) > 0:
				if nb.Action != "AVG" {
					fail("%s R8: %s bought as new although an add was possible (%v)", ds, nb.Symbol, addCand)
				} else {
					found := false
					for _, s := range addCand {
						found = found || s == nb.Symbol
					}
					if !found {
						fail("%s R4: AVG %s is not an eligible add (eligible %v)", ds, nb.Symbol, addCand)
					}
					if haveBest {
						obs.addBeforeFresh++
					}
				}
			case nb.Action == "AVG":
				fail("%s R4: AVG %s although no held stock qualifies for an add", ds, nb.Symbol)
			default: // a new stock
				if !haveBest {
					fail("%s R2: FRESH %s although no new stock qualifies", ds, nb.Symbol)
				} else if best.Symbol != nb.Symbol {
					fail("%s R2: FRESH %s but the stock to buy is %s", ds, nb.Symbol, best.Symbol)
				}
			}
			if nb != nil && cashAtOpen < slot && sellsToday > 0 {
				obs.reuseBuys++
			}
		}

		for ei < len(res.Events) && res.Events[ei].Date.Equal(pt.Date) {
			ev := res.Events[ei]
			ei++
			switch ev.Action {
			case "SELL":
				obs.sells++
				sellsToday++
				if buysToday > 0 {
					fail("%s R8: SELL %s after a purchase the same day (exits come first)", ds, ev.Symbol)
				}
				ls := open[ev.Symbol]
				k := -1
				for i, l := range ls {
					if l.id == ev.LotID {
						k = i
					}
				}
				d, traded := today[ev.Symbol]
				if k < 0 {
					fail("%s SELL of lot %d (%s) that is not open", ds, ev.LotID, ev.Symbol)
					continue
				}
				if !traded {
					fail("%s SELL of %s on a day it has no bar", ds, ev.Symbol)
					continue
				}
				if ev.Price != d.Bar.Close {
					fail("%s SELL %s at %.4f, not at the close %.4f", ds, ev.Symbol, ev.Price, d.Bar.Close)
				}
				l := ls[k]
				if _, seen := avgAtSell[ev.Symbol]; !seen {
					avgAtSell[ev.Symbol] = crAvg(ls)
				}
				if !d.IsConstituent {
					obs.indexSells++
					if ev.Reason != "index" {
						fail("%s R6: %s is not a member but lot %d was sold with reason %q", ds, ev.Symbol, ev.LotID, ev.Reason)
					}
				} else {
					obs.targetSells++
					if ev.Reason != "target" {
						fail("%s R7: %s lot %d sold with reason %q (only target and index exist)", ds, ev.Symbol, ev.LotID, ev.Reason)
					}
					if lotMode && !(ev.Price >= l.px*(1+crProfit)) {
						fail("%s R5: lot %d of %s sold at %.4f, below its own entry %.4f x 1.05", ds, ev.LotID, ev.Symbol, ev.Price, l.px)
					}
					if !lotMode && !(ev.Price >= avgAtSell[ev.Symbol]*(1+crProfit)) {
						fail("%s R5: %s sold at %.4f, below average cost %.4f x 1.05", ds, ev.Symbol, ev.Price, avgAtSell[ev.Symbol])
					}
					if lotMode && len(ls) > 1 {
						obs.partialSells++
					}
				}
				q := float64(ev.Qty)
				realized += ev.Price*q - 0 - (l.px*q + 0)
				cash += ev.Price * q
				open[ev.Symbol] = append(append([]crOpen{}, ls[:k]...), ls[k+1:]...)
				if len(open[ev.Symbol]) == 0 {
					delete(open, ev.Symbol)
				}
				soldSyms[ev.Symbol] = true
			case "FRESH", "AVG":
				if !snapDone {
					snap(&ev)
				}
				buysToday++
				if ev.Action == "FRESH" {
					obs.fresh++
					if soldSyms[ev.Symbol] {
						obs.rebuyAfterSale++
					}
				} else {
					obs.avg++
					if soldSyms[ev.Symbol] {
						obs.addAfterSale++
					}
				}
				if buysToday > 1 {
					fail("%s R8: %d purchases in one day", ds, buysToday)
				}
				d, traded := today[ev.Symbol]
				if !traded || !d.IsConstituent {
					fail("%s R1: %s bought on a day it is not a member (or has no bar)", ds, ev.Symbol)
				}
				if traded && ev.Price != d.Bar.Close {
					fail("%s buy %s at %.4f, not at the close %.4f", ds, ev.Symbol, ev.Price, d.Bar.Close)
				}
				slot := slotNow()
				if cash < slot {
					fail("%s R2: buy of %s with cash %.2f below one slot %.2f", ds, ev.Symbol, cash, slot)
				}
				if want := int(math.Floor(slot / ev.Price)); ev.Qty != want {
					fail("%s R3: buy of %s has %d shares, slot %.4f (capital %.2f realised %.2f) at %.4f gives %d", ds, ev.Symbol, ev.Qty, slot, r.StartCapital+realized, realized, ev.Price, want)
				}
				cash -= ev.Price * float64(ev.Qty)
				if cash < -1e-6 {
					fail("%s cash negative (%.4f) after buying %s", ds, cash, ev.Symbol)
				}
				n := len(open[ev.Symbol])
				if ev.Action == "FRESH" && n != 0 {
					fail("%s FRESH %s while %d lots are open", ds, ev.Symbol, n)
				}
				if ev.Action == "AVG" && n == 0 {
					fail("%s AVG %s with no open lot", ds, ev.Symbol)
				}
				if n+1 > crMaxLots {
					fail("%s R4: %s would hold %d open lots (max %d)", ds, ev.Symbol, n+1, crMaxLots)
				}
				if n+1 == crMaxLots {
					obs.threeLotStocks++
				}
				if n+1 > obs.maxOpenLots {
					obs.maxOpenLots = n + 1
				}
				open[ev.Symbol] = append(open[ev.Symbol], crOpen{id: ev.LotID, px: ev.Price, qty: ev.Qty})
				buyPx[ev.LotID] = ev.Price
			default:
				fail("%s unknown action %q", ds, ev.Action)
			}
		}
		if !snapDone {
			snap(nil)
		}
		if sellsToday >= 2 {
			obs.multiSellDays++
		}
		// day-end state against Result.Points
		lots := 0
		for sym, ls := range open {
			lots += len(ls)
			if len(ls) > crMaxLots {
				fail("%s R4: %s holds %d open lots (max %d)", ds, sym, len(ls), crMaxLots)
			}
		}
		if len(open) > obs.maxStocks {
			obs.maxStocks = len(open)
		}
		if cash < -1e-6 {
			fail("%s cash negative: %.4f", ds, cash)
		}
		if math.Abs(cash-pt.Cash) > 1e-6*math.Max(1, math.Abs(cash)) {
			fail("%s replayed cash %.6f differs from Result.Points cash %.6f", ds, cash, pt.Cash)
		}
		if pt.Cash < -1e-6 {
			fail("%s Result.Points cash negative: %.4f", ds, pt.Cash)
		}
		if pt.Lots != lots || pt.Stocks != len(open) {
			fail("%s Result.Points says %d lots / %d stocks, events say %d / %d", ds, pt.Lots, pt.Stocks, lots, len(open))
		}
	}
	if ei != len(res.Events) {
		fail("%d events are dated outside the Result.Points days", len(res.Events)-ei)
	}
	// ledger: every open lot is in Result.Lots as open, every sold lot as closed
	openLedger := 0
	for _, lr := range res.Lots {
		if lr.Open {
			openLedger++
		}
	}
	held := 0
	for _, ls := range open {
		held += len(ls)
	}
	if openLedger != held {
		fail("Result.Lots has %d open lots, events leave %d", openLedger, held)
	}
	for id, px := range buyPx {
		if lr, ok := ledger[id]; !ok || lr.BuyPrice != px {
			fail("lot %d: ledger row missing or buy price differs", id)
		}
	}
	return obs, problems
}

func crCheck(t *testing.T, label string, p *panel.Panel, res *Result) crObs {
	t.Helper()
	obs, problems := crReplay(p, res)
	for i, s := range problems {
		if i == 8 {
			t.Errorf("%s: ... and %d more problems", label, len(problems)-8)
			break
		}
		t.Errorf("%s: %s", label, s)
	}
	return obs
}

// ---------------------------------------------------------------------------------------------------------------------
// Property tests (a) to (f)

func TestConfirmedRulesProperties(t *testing.T) {
	for _, name := range crVariants {
		var sum crObs
		runs := 0
		for seed := int64(1); seed <= 40; seed++ {
			p := crRandomPanel(seed, 8+int(seed%7), 420)
			res, err := Run(p, crRules(t, name))
			if err != nil {
				t.Fatal(err)
			}
			o := crCheck(t, fmt.Sprintf("%s seed %d", name, seed), p, res)
			runs++
			sum.fresh += o.fresh
			sum.avg += o.avg
			sum.sells += o.sells
			sum.indexSells += o.indexSells
			sum.targetSells += o.targetSells
			sum.addBeforeFresh += o.addBeforeFresh
			sum.multiSellDays += o.multiSellDays
			sum.partialSells += o.partialSells
			sum.reuseBuys += o.reuseBuys
			sum.threeLotStocks += o.threeLotStocks
			sum.maxStocks = max(sum.maxStocks, o.maxStocks)
			sum.maxOpenLots = max(sum.maxOpenLots, o.maxOpenLots)
		}
		t.Logf("%-26s %d runs: %d new, %d adds, %d target sells, %d index sells, max %d stocks, max %d lots, %d stocks at 3 lots, %d add-before-new days, %d multi-sale days, %d buys on sale cash, %d partial lot sells",
			name, runs, sum.fresh, sum.avg, sum.targetSells, sum.indexSells, sum.maxStocks, sum.maxOpenLots, sum.threeLotStocks, sum.addBeforeFresh, sum.multiSellDays, sum.reuseBuys, sum.partialSells)
		// the panels must actually exercise every rule, otherwise the properties above prove nothing
		if sum.fresh < 100 || sum.avg < 20 || sum.targetSells < 50 || sum.indexSells < 10 || sum.maxOpenLots != 3 || sum.threeLotStocks < 5 ||
			sum.addBeforeFresh < 3 || sum.multiSellDays < 3 || sum.reuseBuys < 3 {
			t.Errorf("%s: random panels do not exercise the rules enough: %+v", name, sum)
		}
		if strings.HasPrefix(name, "nsx-lot") && sum.partialSells < 10 {
			t.Errorf("%s: no partial lot sells were exercised: %+v", name, sum)
		}
		// (g) no cap on stocks: more than 5 different stocks held at once on at least one panel
		if sum.maxStocks <= 5 {
			t.Errorf("%s: never more than %d stocks held, the 5-stock cap seems to be back", name, sum.maxStocks)
		}
	}
}

// The checker must be able to see violations: break one confirmed rule at a time and require the matching complaint.
func TestConfirmedRulesCheckerDetectsBrokenRules(t *testing.T) {
	cases := []struct {
		name string
		mod  func(r *Rules)
		want string
	}{
		{"monthly slot refresh instead of per sale", func(r *Rules) { r.Revision = "monthly" }, "R3"},
		{"no lot cap", func(r *Rules) { r.MaxLots = 0 }, "R4"},
		{"engine buying rule (several buys a day)", func(r *Rules) { r.MaxBuysPerDay = 0 }, "R8"},
		{"no sale on index removal", func(r *Rules) { r.ExitOnIndexRemoval = false }, "R6"},
		{"5-stock cap", func(r *Rules) { r.MaxStocks = 5 }, "R2"},
		{"stop loss", func(r *Rules) { r.StopLoss = 0.10 }, "R7"},
		{"add against any lot instead of the latest", func(r *Rules) { r.AvgBasis = "anylot" }, "R4"},
		{"6% target instead of 5%", func(r *Rules) { r.ProfitTarget = 0.06 }, "R5"},
	}
	for _, c := range cases {
		hits := 0
		for seed := int64(1); seed <= 12 && hits == 0; seed++ {
			p := crRandomPanel(seed, 10, 420)
			r := crRules(t, "nsx")
			c.mod(&r)
			res, err := Run(p, r)
			if err != nil {
				t.Fatal(err)
			}
			_, problems := crReplay(p, res)
			for _, s := range problems {
				if strings.Contains(s, c.want) {
					hits++
					break
				}
			}
		}
		if hits == 0 {
			t.Errorf("checker did not notice: %s", c.name)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// (g) no limit on distinct stocks

func TestConfirmedRulesNoLimitOnDistinctStocks(t *testing.T) {
	// 12 stocks, all members and all below their average every day, with distinct distances. Cash buys one slot a day.
	var days [][]crBar
	for d := 0; d < 14; d++ {
		var bars []crBar
		for k := 0; k < 12; k++ {
			bars = append(bars, crBar{sym: fmt.Sprintf("S%02d", k), close: 100, diff: float64(100 - k), cam: 99})
		}
		days = append(days, bars)
	}
	p := crHandPanel(days)
	for _, name := range crVariants {
		res, err := Run(p, crRules(t, name))
		if err != nil {
			t.Fatal(err)
		}
		crCheck(t, name, p, res)
		maxStocks := 0
		for _, pt := range res.Points {
			maxStocks = max(maxStocks, pt.Stocks)
		}
		// slot = 100000/10 = 10000 = 100 shares at 100, so cash for exactly 10 stocks: more than 5, and cash is the only limit
		if maxStocks != 10 {
			t.Errorf("%s: held at most %d stocks, want 10 (cash for 10 slots, no cap)", name, maxStocks)
		}
		if res.Skipped.MaxStocks != 0 {
			t.Errorf("%s: %d buys skipped for a stock cap, there must be none", name, res.Skipped.MaxStocks)
		}
		if got := res.Points[len(res.Points)-1].Cash; got != 0 {
			t.Errorf("%s: final cash %.2f, want 0 after ten full slots", name, got)
		}
		if res.Points[9].Stocks != 10 || res.Points[5].Stocks != 6 {
			t.Errorf("%s: one new stock per day expected, got %d after 6 days and %d after 10", name, res.Points[5].Stocks, res.Points[9].Stocks)
		}
		if !strings.Contains(name, ":") && res.Skipped.NoCash == 0 {
			t.Errorf("%s: the 11th stock should be refused for lack of cash", name)
		}
	}
	// control: the old documented cap of 5 would stop at 5
	r := crRules(t, "nsx")
	r.MaxStocks = 5
	res, _ := Run(p, r)
	if got := res.Points[len(res.Points)-1].Stocks; got != 5 {
		t.Errorf("control with MaxStocks 5: held %d stocks, want 5", got)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Hand-built scenarios: R4, R6, R8

// crLines renders the events as "day SIDE symbol qty price [reason]". Within a day the sales come first (by lot id), then the purchase.
func crLines(res *Result) []string {
	dayOf := map[string]int{}
	for i, pt := range res.Points {
		dayOf[pt.Date.Format("2006-01-02")] = i
	}
	type row struct {
		day, rank, lot int
		text           string
	}
	var rows []row
	for _, e := range res.Events {
		d := dayOf[e.Date.Format("2006-01-02")]
		if e.Action == "SELL" {
			rows = append(rows, row{d, 0, e.LotID, fmt.Sprintf("%d SELL %s %d %.2f %s", d, e.Symbol, e.Qty, e.Price, e.Reason)})
		} else {
			rows = append(rows, row{d, 1, e.LotID, fmt.Sprintf("%d %s %s %d %.2f", d, e.Action, e.Symbol, e.Qty, e.Price)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.day != b.day {
			return a.day < b.day
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		return a.lot < b.lot
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.text
	}
	return out
}

func crExpect(t *testing.T, label string, res *Result, want ...string) {
	t.Helper()
	got := crLines(res)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: events differ\n got:\n  %s\nwant:\n  %s", label, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// R4: the add reference is the latest OPEN lot (not the first lot, not the average cost, not a sold lot) and the cap counts OPEN lots.
func TestConfirmedRulesR4AddAgainstLatestOpenLot(t *testing.T) {
	common := []step{
		{100, 5, 0},   // d0 new stock: 100 shares at 100
		{97.5, -1, 0}, // d1 2.5% below the lot: no add
		{96.9, -1, 0}, // d2 3.1% below: add 103 shares (lot 2)
		{94.0, -1, 0}, // d3 6% below the FIRST lot but only 2.99% below the latest (96.9 x 0.97 = 93.993): no add
		{93.9, -1, 0}, // d4 add 106 shares (lot 3)
		{80, -1, 0},   // d5 far below, but 3 open lots: no add
	}
	lot := append(append([]step{}, common...), step{99, -1, 0}, step{93.9, -1, 0}, step{80, -1, 0})
	whole := append(append([]step{}, common...), step{99, -1, 0}, step{102, -1, 0}, step{100, -1, 0})

	// nsx-lot: d6 lot 3 alone reaches 93.9 x 1.05 = 98.595 and is sold, which frees a place. The slot is refreshed by that sale:
	// realised 106 x (99 - 93.9) = 540.6, slot (100000 + 540.6) / 10 = 10054.06. d7 close 93.9 is 3% below the latest OPEN lot
	// (lot 2 at 96.9 -> 93.993) so a fourth purchase of this stock is allowed (3 open lots again), 10054.06 / 93.9 = 107 shares.
	// Measured against the sold lot 3 (93.9 x 0.97 = 91.08) it would not be allowed. d8 at 80: 3 open lots again, no add.
	res, err := Run(onePanel("X", lot), crRules(t, "nsx-lot"))
	if err != nil {
		t.Fatal(err)
	}
	crExpect(t, "nsx-lot", res,
		"0 FRESH X 100 100.00",
		"2 AVG X 103 96.90",
		"4 AVG X 106 93.90",
		"6 SELL X 106 99.00 target",
		"7 AVG X 107 93.90")
	crCheck(t, "nsx-lot R4", onePanel("X", lot), res)

	// nsx: same buys. d6 99 is below the whole-position target (average cost 96.874 x 1.05 = 101.72) so nothing is sold although lot 3
	// alone would be; d7 102 sells all three lots together.
	res, err = Run(onePanel("X", whole), crRules(t, "nsx"))
	if err != nil {
		t.Fatal(err)
	}
	crExpect(t, "nsx", res,
		"0 FRESH X 100 100.00",
		"2 AVG X 103 96.90",
		"4 AVG X 106 93.90",
		"7 SELL X 100 102.00 target",
		"7 SELL X 103 102.00 target",
		"7 SELL X 106 102.00 target")
	crCheck(t, "nsx R4", onePanel("X", whole), res)
}

// R6: a stock that leaves the index is sold in full that day at the close at any price (here -20%), the cash is used the same day with the
// capital refreshed by that loss, and the stock is not bought while it is outside the index, however far below its average it is.
func TestConfirmedRulesR6IndexExit(t *testing.T) {
	days := [][]crBar{
		{{sym: "A", close: 100, diff: 5}, {sym: "B", close: 50, diff: -1}},            // d0 new stock A: 100 shares
		{{sym: "A", close: 96.9, diff: -1}, {sym: "B", close: 50, diff: -1}},          // d1 add A: 103 shares
		{{sym: "A", close: 95, diff: -1}, {sym: "B", close: 50, diff: -1}},            // d2 nothing
		{{sym: "A", close: 80, diff: -1, out: true}, {sym: "B", close: 50, diff: 6}},  // d3 A leaves the index
		{{sym: "A", close: 70, diff: 20, out: true}, {sym: "B", close: 50, diff: -1}}, // d4 A is the most oversold stock but not a member
		{{sym: "A", close: 70, diff: 20}, {sym: "B", close: 50, diff: -1}},            // d5 A is a member again: bought as a new stock
		{{sym: "A", close: 70, diff: -1, out: true}, {sym: "B", close: 50, diff: -1}}, // d6 A leaves the index again: its single lot (137 shares) is sold at 70, no loss on it
	}
	// d3: A (2 lots, 203 shares, cost 19980.70) is sold at 80 (proceeds 16240.00): realised -3740.70, slot (100000 - 3740.70) / 10 = 9625.93,
	// so B is bought the same day with floor(9625.93 / 50) = 192 shares. d5: floor(9625.93 / 70) = 137 shares.
	for _, name := range []string{"nsx", "nsx-lot"} {
		p := crHandPanel(days)
		res, err := Run(p, crRules(t, name))
		if err != nil {
			t.Fatal(err)
		}
		crExpect(t, name, res,
			"0 FRESH A 100 100.00",
			"1 AVG A 103 96.90",
			"3 SELL A 100 80.00 index",
			"3 SELL A 103 80.00 index",
			"3 FRESH B 192 50.00",
			"5 FRESH A 137 70.00",
			"6 SELL A 137 70.00 index")
		crCheck(t, name+" R6", p, res)
		for _, l := range res.Lots {
			if l.Symbol == "A" && l.Open {
				t.Errorf("%s: a lot of A is still open at the end: %+v", name, l)
			}
		}
	}
}

// R8: exits first and any number of them, then at most one purchase per day, an add before a new stock, sale cash reusable the same day.
func TestConfirmedRulesR8OrderOfTheDay(t *testing.T) {
	t.Run("add comes before a new stock", func(t *testing.T) {
		days := [][]crBar{
			{{sym: "A", close: 100, diff: 5}, {sym: "B", close: 100, diff: -1}},  // d0 new stock A
			{{sym: "A", close: 96, diff: -1}, {sym: "B", close: 100, diff: 8}},   // d1 A qualifies for an add and B for a new stock: only the add
			{{sym: "A", close: 96.5, diff: -1}, {sym: "B", close: 100, diff: 8}}, // d2 no add any more: now the new stock
		}
		for _, name := range []string{"nsx", "nsx-lot"} {
			p := crHandPanel(days)
			res, err := Run(p, crRules(t, name))
			if err != nil {
				t.Fatal(err)
			}
			crExpect(t, name, res, "0 FRESH A 100 100.00", "1 AVG A 104 96.00", "2 FRESH B 100 100.00")
			crCheck(t, name+" R8 add first", p, res)
		}
	})
	t.Run("two exits, one purchase, sale cash reused with the refreshed capital", func(t *testing.T) {
		// 12 stocks at 100, S00 the most oversold. d0..d9 buy S00..S09 (100 shares = one slot each), cash is then 0.
		// d10: S00 closes 105.5 and S01 106, both over +5%: two exits (+550 and +600). Realised 1150, slot (100000 + 1150) / 10 = 10115.
		// Cash 21150 would pay for two slots, but only ONE purchase is allowed: S10, floor(10115 / 100) = 101 shares (a monthly refresh
		// would give 100 shares). d11 buys S11 the same way. After that cash 950 < slot.
		var days [][]crBar
		for d := 0; d < 14; d++ {
			var bars []crBar
			for k := 0; k < 12; k++ {
				b := crBar{sym: fmt.Sprintf("S%02d", k), close: 100, diff: float64(100 - k)}
				if d >= 10 && k < 2 {
					b.diff = -1 // they have risen above their average
				}
				if d == 10 && k == 0 {
					b.close = 105.5
				}
				if d == 10 && k == 1 {
					b.close = 106
				}
				bars = append(bars, b)
			}
			days = append(days, bars)
		}
		var want []string
		for k := 0; k < 10; k++ {
			want = append(want, fmt.Sprintf("%d FRESH S%02d 100 100.00", k, k))
		}
		want = append(want, "10 SELL S00 100 105.50 target", "10 SELL S01 100 106.00 target", "10 FRESH S10 101 100.00", "11 FRESH S11 101 100.00")
		for _, name := range []string{"nsx", "nsx-lot"} {
			p := crHandPanel(days)
			res, err := Run(p, crRules(t, name))
			if err != nil {
				t.Fatal(err)
			}
			crExpect(t, name, res, want...)
			crCheck(t, name+" R8 sale cash", p, res)
			if got := res.Points[10].Cash; math.Abs(got-11050) > 1e-6 {
				t.Errorf("%s: cash after d10 is %.2f, want 11050 (21150 from sales, one purchase of 10100)", name, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------
// Known violation, reported and not fixed (only test files may change in this verification)

// At the cost and tax stages a buy can overdraw cash. sim.go checks "cash >= one slot" (sim.go:619, 638, 694) and sizes the shares as
// floor(slot / fill price), but then debits shares x price PLUS the charges (sim.go:309), so a buy made with cash just above one slot
// can take cash below zero by up to the charges (about 0.12% of the slot). On the real Nifty 50 panel (2008-02..2025-08, capital 1,000,000,
// statutory charges and 10 bps slippage) cash dips below zero in 3 of the 8 lag/preset runs, worst -215.86. The gross stage is not
// affected (no charges), which is what the property tests above cover. The deterministic reproduction below uses a divider of 1 only to
// put cash exactly on the slot. Fixed (slotUnits in sim.go); kept as a regression test.
func TestConfirmedRulesCostStageCashNeverNegative(t *testing.T) {
	p := crHandPanel([][]crBar{{{sym: "A", close: 100, diff: 9}}, {{sym: "A", close: 100, diff: -1}}})
	r := crRules(t, "nsx")
	r.CapitalDivider = 1
	r.Costs = costs.IndianDelivery{Statutory: true}
	res, err := Run(p, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, pt := range res.Points {
		if pt.Cash < 0 {
			t.Errorf("%s: cash %.2f after a buy that was allowed because cash >= one slot", pt.Date.Format("2006-01-02"), pt.Cash)
		}
	}
}

// Membership lag 1 is meant to use the PREVIOUS calendar month's snapshot. panel.IsMember (panel.go:227) shifts the date with
// t.AddDate(0, -1, 0), and Go normalises a day that does not exist in the previous month: 2013-10-31 minus one month is "September 31"
// = 2013-10-01, so the lookup key is the SAME month (2013-10) and lag 1 silently behaves like lag 0 on that day. It hits the 31st after a
// 30-day month (May 31, Jul 31, Oct 31, Dec 31) and Mar 29-31: 75 of 4332 Nifty 50 trading days and 30 of 1853 Midcap 50 trading days.
// The effect is tiny (the snapshot used is dated that very day or earlier, and only on a handful of those days at most 2 days
// ahead), but it is not what the parameter says. Fixed in IsMember (the key is built from the first of the month); kept as a regression test.
func TestConfirmedRulesMembershipLag1UsesPreviousMonthOnThe31st(t *testing.T) {
	w := filepath.Join(t.TempDir(), "w.csv")
	if err := os.WriteFile(w, []byte("DATE,X\n2013-09-30,1\n2013-10-31,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := panel.LoadMembership(w)
	if err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	// lag 1 on any day of October 2013 must read the 2013-09-30 row, where X is a member
	for _, ds := range []string{"2013-10-01", "2013-10-30", "2013-10-31"} {
		if !m.IsMember("X", day(ds), 1) {
			t.Errorf("lag 1 on %s must use the September snapshot (X is a member) but X is reported as not a member", ds)
		}
	}
}
