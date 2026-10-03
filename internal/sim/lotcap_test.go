package sim

// Adversarial tests for the 3-lots-per-stock cap (Rules.MaxLots) and the own-target-per-lot exit (ExitBasis "lot").
//
// The cap is checked from the outside: open lots are re-derived over time from Result.Events (every fill, in order) and
// cross-checked against Result.Lots (the lot ledger) and Result.Points (the daily lot count), never from sim.go state.
//
// Counting rule under test (what sim.go does, sim.go:574-584): the cap counts OPEN lots of a stock. When a lot is sold
// the stock may add again until it holds 3 open lots. A stock that is fully sold starts a fresh count (it has no lots).

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/indicators"
	"github.com/vishalvx/back-tester/internal/panel"
)

// lotcapRules mirrors cmd/research/cmd_run.go presetByName for spec-cap3 / spec-lot3 (that function lives in package main
// and cannot be imported) and uses the real sim presets for everything else.
func lotcapRules(t testing.TB, name string) Rules {
	t.Helper()
	var r Rules
	if strings.HasPrefix(name, "spec-cap3") || strings.HasPrefix(name, "spec-lot3") {
		parts := strings.Split(name, ":")
		r = SpecRules()
		r.MaxLots = 3
		if strings.HasPrefix(name, "spec-lot3") {
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
	} else {
		var err error
		if r, err = Preset(name); err != nil {
			t.Fatal(err)
		}
	}
	r.StartCapital = 1000000
	return r
}

// ---------------------------------------------------------------------------------------------------------------------
// Random panels

// lotcapRandomPanel builds an in-memory panel of mean-reverting random walks with crash streaks (3 to 8 consecutive
// falls of 3 to 6 percent) so that averaging down, the cap and own-target sells all trigger often. Missing bars, monthly
// constituent flapping, next-open prices and previous-bar pivots are all present.
func lotcapRandomPanel(seed int64, nSyms, nDays int) *panel.Panel {
	rng := rand.New(rand.NewSource(seed))
	var dates []time.Time
	for d := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC); len(dates) < nDays; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			dates = append(dates, d)
		}
	}
	type row struct {
		present bool
		bar     panel.Bar
	}
	rows := make([][]row, nSyms)
	for s := range rows {
		px := 20 + rng.Float64()*380
		anchor := px
		crash, rate := 0, 0.0
		rows[s] = make([]row, nDays)
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
			px = math.Max(math.Round(px*(1+ret)*20)/20, 1) // 5 paise tick, like an NSE price, so exact ties at targets occur
			anchor *= 1.0003
			o := px * (1 + rng.NormFloat64()*0.005)
			rows[s][i] = row{present: rng.Float64() > 0.03, bar: panel.Bar{Date: d, Open: o, Close: px,
				High: math.Max(px, o) * (1 + rng.Float64()*0.01), Low: math.Min(px, o) * (1 - rng.Float64()*0.01)}}
		}
	}
	member := map[[2]int]bool{}
	isMember := func(s int, d time.Time) bool {
		k := [2]int{s, d.Year()*12 + int(d.Month())}
		v, ok := member[k]
		if !ok {
			v = rng.Float64() < 0.85
			member[k] = v
		}
		return v
	}
	p := &panel.Panel{Days: map[string][]panel.Day{}, Dates: dates, Series: map[string]*panel.Series{}}
	for s := 0; s < nSyms; s++ {
		sym := fmt.Sprintf("T%02d", s)
		var present []panel.Bar
		var idx []int
		for i, r := range rows[s] {
			if r.present {
				present = append(present, r.bar)
				idx = append(idx, i)
			}
		}
		for k, i := range idx {
			b := present[k]
			day := panel.Day{Symbol: sym, Bar: b, IsConstituent: isMember(s, b.Date)}
			if k >= 19 {
				sum := 0.0
				for _, x := range present[k-19 : k+1] {
					sum += x.Close
				}
				day.SMA = sum / 20
				day.DiffSMA = (day.SMA - b.Close) / b.Close * 100
			}
			if k > 0 {
				day.Pivots = indicators.CalculatePivotLevels(present[k-1].High, present[k-1].Low, present[k-1].Close)
			}
			if k+1 < len(present) {
				day.NextOpen = present[k+1].Open
			}
			ds := dates[i].Format("2006-01-02")
			p.Days[ds] = append(p.Days[ds], day)
		}
	}
	for ds := range p.Days {
		l := p.Days[ds]
		sort.SliceStable(l, func(i, j int) bool {
			if l[i].DiffSMA != l[j].DiffSMA {
				return l[i].DiffSMA > l[j].DiffSMA
			}
			return l[i].Symbol < l[j].Symbol
		})
	}
	return p
}

// ---------------------------------------------------------------------------------------------------------------------
// Independent replay of a Result

type lotcapObs struct {
	maxOpen    int // most open lots of one symbol at any instant (checked after every single fill)
	maxDayEnd  int // same, measured from the lot ledger at the end of each day
	avgBuys    int
	capReached int // AVG fills that brought a stock to exactly the cap
	buys       int
	sells      int
}

// lotcapCheck replays res and fails the test on any violation. maxLots <= 0 means "do not assert the cap" (control runs).
func lotcapCheck(t *testing.T, label string, p *panel.Panel, res *Result, maxLots int) lotcapObs {
	t.Helper()
	r := res.Rules
	var obs lotcapObs
	fail := func(format string, a ...any) {
		t.Helper()
		t.Errorf("%s: %s", label, fmt.Sprintf(format, a...))
	}

	rec := map[int]LotRecord{}
	for _, lr := range res.Lots {
		if _, dup := rec[lr.ID]; dup {
			fail("lot id %d appears twice in Result.Lots", lr.ID)
		}
		rec[lr.ID] = lr
	}
	closeOn := map[string]map[string]float64{}
	for ds, list := range p.Days {
		m := make(map[string]float64, len(list))
		for _, d := range list {
			m[d.Symbol] = d.Bar.Close
		}
		closeOn[ds] = m
	}

	noExtraExits := r.StopLoss == 0 && r.TimeStopDays == 0 && r.TrailPct == 0
	sameClose := r.SlippageBps == 0 && !r.FillNextOpen

	open := map[string][]int{} // symbol -> open lot ids in buy order
	buyPx := map[int]float64{}
	ei := 0
	for _, pt := range res.Points {
		ds := pt.Date.Format("2006-01-02")
		avgAtSell := map[string]float64{} // avgcost basis: average cost of the position when its first lot sold today
		soldToday := map[string]bool{}
		buysToday := 0
		for ei < len(res.Events) && res.Events[ei].Date.Equal(pt.Date) {
			ev := res.Events[ei]
			ei++
			switch ev.Action {
			case "FRESH", "AVG":
				obs.buys++
				buysToday++
				n := len(open[ev.Symbol])
				if ev.Action == "FRESH" && n != 0 {
					fail("%s FRESH %s while it already holds %d open lots", ds, ev.Symbol, n)
				}
				if ev.Action == "AVG" {
					obs.avgBuys++
					if n == 0 {
						fail("%s AVG %s with no open lot", ds, ev.Symbol)
					} else if r.AvgBasis == "lastlot" {
						// averaging reference is the latest open lot's buy price
						ref := buyPx[open[ev.Symbol][n-1]]
						if c := closeOn[ds][ev.Symbol]; c > ref*(1-r.AvgTrigger) {
							fail("%s AVG %s at close %.4f is not %.1f%% below the latest lot %.4f", ds, ev.Symbol, c, r.AvgTrigger*100, ref)
						}
					}
				}
				if _, ok := rec[ev.LotID]; !ok {
					fail("%s fill of lot %d has no row in Result.Lots", ds, ev.LotID)
				}
				open[ev.Symbol] = append(open[ev.Symbol], ev.LotID)
				buyPx[ev.LotID] = rec[ev.LotID].BuyPrice
				if n+1 > obs.maxOpen {
					obs.maxOpen = n + 1
				}
				if maxLots > 0 && n+1 > maxLots {
					fail("%s %s has %d open lots (cap %d) after %s lot %d", ds, ev.Symbol, n+1, maxLots, ev.Action, ev.LotID)
				}
				if maxLots > 0 && ev.Action == "AVG" && n+1 == maxLots {
					obs.capReached++
				}
			case "SELL":
				obs.sells++
				ids := open[ev.Symbol]
				k := -1
				for i, id := range ids {
					if id == ev.LotID {
						k = i
					}
				}
				if k < 0 {
					fail("%s SELL of lot %d (%s) that is not open", ds, ev.LotID, ev.Symbol)
					continue
				}
				lr := rec[ev.LotID]
				if lr.Open || lr.SellDate != pt.Date || lr.Qty != ev.Qty || lr.SellPrice != ev.Price || lr.Reason != ev.Reason {
					fail("%s SELL of lot %d disagrees with its ledger row %+v", ds, ev.LotID, lr)
				}
				c := closeOn[ds][ev.Symbol]
				if _, seen := avgAtSell[ev.Symbol]; !seen {
					cost, qty := 0.0, 0
					for _, id := range ids {
						cost += buyPx[id] * float64(rec[id].Qty)
						qty += rec[id].Qty
					}
					avgAtSell[ev.Symbol] = cost / float64(qty)
				}
				if ev.Reason == "target" && noExtraExits {
					switch r.ExitBasis {
					case "lot":
						// own target: this lot's own buy price x (1 + target)
						if !(c >= lr.BuyPrice*(1+r.ProfitTarget)) {
							fail("%s lot %d sold on close %.4f below its own target %.4f", ds, ev.LotID, c, lr.BuyPrice*(1+r.ProfitTarget))
						}
						if sameClose && !(ev.Price >= lr.BuyPrice*(1+r.ProfitTarget)) {
							fail("%s lot %d sold at %.4f, below buy %.4f x %.2f", ds, ev.LotID, ev.Price, lr.BuyPrice, 1+r.ProfitTarget)
						}
					case "apptarget":
						if !(c >= lr.Target) {
							fail("%s lot %d sold on close %.4f below its stored target %.4f", ds, ev.LotID, c, lr.Target)
						}
					case "avgcost":
						if !(c >= avgAtSell[ev.Symbol]*(1+r.ProfitTarget)) {
							fail("%s %s sold on close %.4f below avg cost %.4f x %.2f", ds, ev.Symbol, c, avgAtSell[ev.Symbol], 1+r.ProfitTarget)
						}
					}
				}
				soldToday[ev.Symbol] = true
				open[ev.Symbol] = append(append([]int{}, ids[:k]...), ids[k+1:]...)
				if len(open[ev.Symbol]) == 0 {
					delete(open, ev.Symbol)
				}
			default:
				fail("unknown action %q", ev.Action)
			}
		}
		if r.MaxBuysPerDay > 0 && buysToday > r.MaxBuysPerDay {
			fail("%s %d buys in one day, MaxBuysPerDay %d", ds, buysToday, r.MaxBuysPerDay)
		}

		// day-end state from the events
		total, stocks := 0, 0
		for sym, ids := range open {
			total += len(ids)
			stocks++
			if maxLots > 0 && len(ids) > maxLots {
				fail("%s day-end: %s holds %d open lots (cap %d)", ds, sym, len(ids), maxLots)
			}
			c, traded := closeOn[ds][sym]
			if traded && noExtraExits {
				switch r.ExitBasis {
				case "lot", "apptarget":
					if r.ExitBasis == "apptarget" && r.SyncTargets {
						break // targets move when a later lot is bought; only the final value is in the ledger
					}
					for _, id := range ids {
						if rec[id].BuyDate.Equal(pt.Date) {
							continue // exits run before buys, so a lot bought today is first tested tomorrow (its next-open fill may sit far below today's close)
						}
						tgt := buyPx[id] * (1 + r.ProfitTarget)
						if r.ExitBasis == "apptarget" {
							tgt = rec[id].Target
						}
						if c >= tgt {
							fail("%s day-end: lot %d of %s is still open although close %.4f >= its target %.4f", ds, id, sym, c, tgt)
						}
					}
				}
			}
			if soldToday[sym] && r.ExitBasis == "avgcost" {
				for _, id := range ids {
					if !rec[id].BuyDate.Equal(pt.Date) {
						fail("%s day-end: %s sold part of its position but lot %d (bought %s) stayed open", ds, sym, id, rec[id].BuyDate.Format("2006-01-02"))
					}
				}
			}
		}
		if total != pt.Lots || stocks != pt.Stocks {
			fail("%s day-end: events give %d lots / %d stocks, Result.Points says %d / %d", ds, total, stocks, pt.Lots, pt.Stocks)
		}
		// day-end state from the ledger (independent second derivation)
		bySym := map[string]int{}
		for _, lr := range res.Lots {
			if !lr.BuyDate.After(pt.Date) && (lr.Open || lr.SellDate.After(pt.Date)) {
				bySym[lr.Symbol]++
			}
		}
		for sym, n := range bySym {
			if n > obs.maxDayEnd {
				obs.maxDayEnd = n
			}
			if n != len(open[sym]) {
				fail("%s day-end: ledger says %s holds %d lots, events say %d", ds, sym, n, len(open[sym]))
			}
		}
		if len(bySym) != len(open) {
			fail("%s day-end: ledger holds %d symbols, events %d", ds, len(bySym), len(open))
		}
	}
	if ei != len(res.Events) {
		fail("%d events dated outside the simulated days", len(res.Events)-ei)
	}
	// every ledger row is explained by the events
	openIDs := map[int]bool{}
	for _, ids := range open {
		for _, id := range ids {
			openIDs[id] = true
		}
	}
	for _, lr := range res.Lots {
		if lr.Open != openIDs[lr.ID] {
			fail("lot %d: ledger Open=%v but replay says open=%v", lr.ID, lr.Open, openIDs[lr.ID])
		}
	}
	if len(rec) != obs.buys {
		fail("%d buy fills but %d ledger rows", obs.buys, len(rec))
	}
	return obs
}

// ---------------------------------------------------------------------------------------------------------------------
// Property test

type lotcapVariant struct {
	name string
	mod  func(r *Rules)
}

var lotcapVariants = []lotcapVariant{
	{"base", nil},
	{"next-open+slippage", func(r *Rules) { r.FillNextOpen = true; r.SlippageBps = 25 }},
	{"4-fresh-a-day,no-daily-buy-cap", func(r *Rules) { r.MaxFreshPerDay = 4; r.MaxBuysPerDay = 0 }},
	{"hair-trigger-averaging", func(r *Rules) { r.AvgTrigger = 0.005 }},
	{"deep-pockets,unlimited-stocks", func(r *Rules) { r.CapitalDivider = 60; r.MaxStocks = 0 }},
	{"stop+time-exits", func(r *Rules) { r.StopLoss = 0.25; r.TimeStopDays = 60 }},
	{"everything-at-once", func(r *Rules) {
		r.FillNextOpen, r.SlippageBps = true, 10
		r.MaxFreshPerDay, r.MaxBuysPerDay = 3, 0
		r.AvgTrigger, r.CapitalDivider, r.MaxStocks = 0.01, 80, 0
	}},
}

func TestLotCapProperty(t *testing.T) {
	presets := []string{
		"spec-cap3", "spec-lot3",
		"spec-cap3:camarilla:S1:5", "spec-lot3:camarilla:S1:5", "spec-cap3:fibonacci:S1:15", "spec-lot3:fibonacci:S1:15",
		"app-exact", "app-vpivot-nifty50", "app-vpivot-midcap50",
	}
	const seeds = 40
	type cover struct{ maxOpen, capReached, sells, buys int }
	seen := map[string]*cover{}
	for _, name := range presets {
		seen[name] = &cover{}
	}
	for seed := int64(1); seed <= seeds; seed++ {
		rng := rand.New(rand.NewSource(seed * 7919))
		p := lotcapRandomPanel(seed, 8+rng.Intn(7), 260+rng.Intn(160))
		for _, name := range presets {
			for _, v := range lotcapVariants {
				r := lotcapRules(t, name)
				if r.MaxLots != 3 {
					t.Fatalf("%s must be a capped preset, MaxLots=%d", name, r.MaxLots)
				}
				if v.mod != nil {
					v.mod(&r)
				}
				res, err := Run(p, r)
				if err != nil {
					t.Fatal(err)
				}
				label := fmt.Sprintf("seed %d %s [%s]", seed, name, v.name)
				o := lotcapCheck(t, label, p, res, 3)
				c := seen[name]
				if o.maxOpen > c.maxOpen {
					c.maxOpen = o.maxOpen
				}
				c.capReached += o.capReached
				c.sells += o.sells
				c.buys += o.buys
				if o.maxDayEnd > o.maxOpen {
					t.Errorf("%s: day-end max %d exceeds instant max %d", label, o.maxDayEnd, o.maxOpen)
				}
				if t.Failed() {
					t.FailNow() // one precise failure is more useful than thousands
				}
			}
		}
	}
	for _, name := range presets {
		c := seen[name]
		t.Logf("%-28s buys=%-5d sells=%-5d AVG fills that reached the cap=%-4d max open lots of one stock=%d", name, c.buys, c.sells, c.capReached, c.maxOpen)
		// guard against a vacuous test: the cap must actually have been reached, so a missing cap would have been caught
		if c.maxOpen != 3 {
			t.Errorf("%s never reached the cap in %d random panels (max %d): the test does not exercise it", name, seeds, c.maxOpen)
		}
	}
}

// The checker must be able to fail: without the cap the same panels do produce stocks with more than 3 open lots.
func TestLotCapControlUncappedDoesExceed(t *testing.T) {
	for _, name := range []string{"spec", "legacy"} {
		most := 0
		for seed := int64(1); seed <= 10; seed++ {
			p := lotcapRandomPanel(seed, 10, 300)
			r := lotcapRules(t, name)
			r.AvgTrigger, r.CapitalDivider = 0.01, 60
			res, err := Run(p, r)
			if err != nil {
				t.Fatal(err)
			}
			if o := lotcapCheck(t, fmt.Sprintf("control %s seed %d", name, seed), p, res, 0); o.maxOpen > most {
				most = o.maxOpen
			}
		}
		if most <= 3 {
			t.Fatalf("control: uncapped %q never exceeded 3 open lots (max %d), so the property test cannot detect a missing cap", name, most)
		}
		t.Logf("control: uncapped %q reaches %d open lots of one stock (no cap by design)", name, most)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Scenarios

// lotcapPath builds a one-symbol panel from closes; diff[i] is DiffSMA on day i (positive = eligible for a fresh entry).
// Every pivot level is 1, below any price used, so pivot filters always pass.
func lotcapPath(closes, diff []float64) *panel.Panel {
	p := &panel.Panel{Days: map[string][]panel.Day{}, Series: map[string]*panel.Series{}}
	d := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		piv := indicators.PivotLevels{ClassicS1: 1, ClassicS2: 1, ClassicS3: 1, FibS1: 1, FibS2: 1, FibS3: 1, CamS1: 1, CamS2: 1, CamS3: 1, CamS4: 1}
		p.Dates = append(p.Dates, d)
		p.Days[d.Format("2006-01-02")] = []panel.Day{{Symbol: "X", IsConstituent: true, DiffSMA: diff[i], Pivots: piv,
			Bar: panel.Bar{Date: d, Open: c, High: c, Low: c, Close: c}}}
		d = d.AddDate(0, 0, 1)
	}
	return p
}

func lotcapCount(res *Result, action string) (n int) {
	for _, e := range res.Events {
		if e.Action == action {
			n++
		}
	}
	return
}

// Price falls 4% every day for 12 days and the stock stays eligible for a fresh entry throughout: a capped preset must
// buy exactly 3 lots (1 fresh + 2 averaging) and never a 4th, whatever the preset.
func TestLotCapFallingFourPercentTwelveDays(t *testing.T) {
	closes := make([]float64, 12)
	diff := make([]float64, 12)
	px := 100.0
	for i := range closes {
		closes[i], diff[i] = px, 5
		px *= 0.96
	}
	p := lotcapPath(closes, diff)
	for _, name := range []string{
		"spec-cap3", "spec-lot3", "spec-cap3:camarilla:S1:5", "spec-lot3:fibonacci:S1:15",
		"app-exact", "app-approx", "app-vpivot-nifty50", "app-vpivot-midcap50",
	} {
		res, err := Run(p, lotcapRules(t, name))
		if err != nil {
			t.Fatal(err)
		}
		lotcapCheck(t, name, p, res, 3)
		if got := lotcapCount(res, "FRESH") + lotcapCount(res, "AVG"); got != 3 {
			t.Errorf("%s: want exactly 3 lots bought in a 12-day 4%% slide, got %d (%+v)", name, got, res.Events)
		}
		if lotcapCount(res, "FRESH") != 1 || lotcapCount(res, "AVG") != 2 || lotcapCount(res, "SELL") != 0 {
			t.Errorf("%s: want 1 FRESH + 2 AVG + 0 SELL, got %+v", name, res.Events)
		}
		if last := res.Points[len(res.Points)-1]; last.Lots != 3 || len(res.Lots) != 3 {
			t.Errorf("%s: want 3 open lots at the end, got %d (ledger %d)", name, last.Lots, len(res.Lots))
		}
		// buys land on days 1, 2, 3 at 100, 96, 92.16
		for i, want := range []float64{100, 96, 92.16} {
			if got := res.Events[i].Price; math.Abs(got-want) > 1e-9 {
				t.Errorf("%s: lot %d bought at %.4f, want %.4f", name, i+1, got, want)
			}
		}
	}
	// control: the preset without a cap keeps buying down the whole slide (1 fresh + 11 averaging)
	for _, name := range []string{"spec", "legacy"} {
		r := lotcapRules(t, name)
		r.CapitalDivider = 20 // so that cash never limits the control
		res, err := Run(p, r)
		if err != nil {
			t.Fatal(err)
		}
		if got := lotcapCount(res, "FRESH") + lotcapCount(res, "AVG"); got != 12 {
			t.Errorf("control %s: uncapped should buy 12 lots over the slide, got %d", name, got)
		}
	}
}

// Own targets (spec-lot3): every lot sells by itself at its OWN buy price x 1.05, and the averaging reference is the
// latest open lot.
func TestLotCapOwnTargetPerLot(t *testing.T) {
	//        buy A   B     C      C sells B sells A sells  new position
	closes := []float64{100, 96, 92.16, 97, 101, 105.5, 90, 87}
	diff := []float64{5, 5, 5, -1, -1, -1, 5, 5}
	p := lotcapPath(closes, diff)
	res, err := Run(p, lotcapRules(t, "spec-lot3"))
	if err != nil {
		t.Fatal(err)
	}
	if o := lotcapCheck(t, "spec-lot3", p, res, 3); o.maxOpen != 3 {
		t.Errorf("want 3 open lots at the peak, got %d", o.maxOpen)
	}
	type want struct {
		day    int
		action string
		price  float64
		lot    int
	}
	wants := []want{
		{0, "FRESH", 100, 1}, {1, "AVG", 96, 2}, {2, "AVG", 92.16, 3},
		{3, "SELL", 97, 3},    // 97 >= 92.16 x 1.05 = 96.77: only the newest lot; B (100.8) and A (105) stay
		{4, "SELL", 101, 2},   // 101 >= 96 x 1.05 = 100.8
		{5, "SELL", 105.5, 1}, // 105.5 >= 100 x 1.05
		{6, "FRESH", 90, 4},   // fully sold, so a new position starts with a fresh count
		{7, "AVG", 87, 5},     // 87 <= 90 x 0.97 = 87.3
	}
	if len(res.Events) != len(wants) {
		t.Fatalf("want %d events, got %d: %+v", len(wants), len(res.Events), res.Events)
	}
	for i, w := range wants {
		e := res.Events[i]
		if !e.Date.Equal(p.Dates[w.day]) || e.Action != w.action || e.LotID != w.lot || math.Abs(e.Price-w.price) > 1e-9 {
			t.Errorf("event %d: want day %d %s lot %d at %.2f, got %s %s lot %d at %.2f", i, w.day, w.action, w.lot, w.price, e.Date.Format("2006-01-02"), e.Action, e.LotID, e.Price)
		}
		if w.action == "SELL" && e.Reason != "target" {
			t.Errorf("event %d: reason %q, want target", i, e.Reason)
		}
	}
	// every sold lot sold at or above its own buy x 1.05
	for _, lr := range res.Lots {
		if !lr.Open && lr.SellPrice < lr.BuyPrice*1.05 {
			t.Errorf("lot %d sold at %.4f below its own target %.4f", lr.ID, lr.SellPrice, lr.BuyPrice*1.05)
		}
	}

	// The written rule on the same path: the whole position sells together at +5% over average cost (about 100.74).
	cap3, err := Run(p, lotcapRules(t, "spec-cap3"))
	if err != nil {
		t.Fatal(err)
	}
	lotcapCheck(t, "spec-cap3", p, cap3, 3)
	var sellDays []string
	for _, e := range cap3.Events {
		if e.Action == "SELL" {
			sellDays = append(sellDays, e.Date.Format("2006-01-02"))
		}
	}
	if len(sellDays) != 3 || sellDays[0] != sellDays[2] || sellDays[0] != p.Dates[4].Format("2006-01-02") {
		t.Errorf("spec-cap3: want all 3 lots sold together on day 5 (close 101), got sells on %v", sellDays)
	}
}

// Pins down how the cap counts. sim.go counts OPEN lots: after one lot of a stock has sold, the stock may buy again until
// it has 3 open lots again. So over the life of one continuous position a stock can be bought more than 3 times
// (1 fresh + 3 averaging here) while never holding more than 3 at once. If the maintainer means "at most 3 buys per
// position, sold lots included", this test (and sim.go:574-584) must change.
func TestLotCapCountsOpenLotsNotLotsEverBought(t *testing.T) {
	//                     A    B    C      C sells  AVG vs latest open lot B (96 x 0.97 = 93.12)
	closes := []float64{100, 96, 92.16, 97, 93}
	diff := []float64{5, 5, 5, -1, -1}
	p := lotcapPath(closes, diff)
	res, err := Run(p, lotcapRules(t, "spec-lot3"))
	if err != nil {
		t.Fatal(err)
	}
	if o := lotcapCheck(t, "spec-lot3", p, res, 3); o.maxOpen != 3 {
		t.Errorf("max open lots %d, want 3", o.maxOpen)
	}
	if got := lotcapCount(res, "FRESH") + lotcapCount(res, "AVG"); got != 4 {
		t.Fatalf("want 4 buys (A, B, C, then D after C sold) with 3 open at a time, got %d: %+v", got, res.Events)
	}
	last := res.Points[len(res.Points)-1]
	if last.Lots != 3 {
		t.Errorf("want 3 open lots at the end (A, B, D), got %d", last.Lots)
	}
	// The same path under the written exit never sells before the fall ends, so the 4th buy cannot happen.
	cap3, _ := Run(p, lotcapRules(t, "spec-cap3"))
	if got := lotcapCount(cap3, "FRESH") + lotcapCount(cap3, "AVG"); got != 3 {
		t.Errorf("spec-cap3: want 3 buys on this path, got %d", got)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Differential test against the independent Python reference (research/py/ref_sim.py, written from strategy.md and sharing
// no code with this package). Skipped when python3 or the script is not available.

func lotcapWritePanelCSV(t *testing.T, p *panel.Panel, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	g := func(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) } // shortest round-trip, so Python reads the exact float
	w.Write([]string{"date", "symbol", "close", "diff_sma", "is_constituent", "cam_s1", "fib_s1"})
	for _, dt := range p.Dates {
		ds := dt.Format("2006-01-02")
		for _, d := range p.Days[ds] {
			w.Write([]string{ds, d.Symbol, g(d.Bar.Close), g(d.DiffSMA), strconv.FormatBool(d.IsConstituent), g(d.Pivots.CamS1), g(d.Pivots.FibS1)})
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
}

func TestLotCapMatchesIndependentReference(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	ref, _ := filepath.Abs(filepath.Join("..", "..", "research", "py", "ref_sim.py"))
	if _, err := os.Stat(ref); err != nil {
		t.Skip("reference simulator not found: ", ref)
	}
	pairs := []struct{ goName, pyMode string }{
		{"spec-cap3", "cap3"}, {"spec-lot3", "lot3"},
		{"spec-cap3:camarilla:S1:5", "cap3:camarilla:S1:5"}, {"spec-lot3:camarilla:S1:5", "lot3:camarilla:S1:5"},
		{"spec-cap3:fibonacci:S1:15", "cap3:fibonacci:S1:15"}, {"spec-lot3:fibonacci:S1:15", "lot3:fibonacci:S1:15"},
		{"app-exact", "app"},
		{"app-vpivot-nifty50", "app-vpivot:camarilla:S1:5"}, {"app-vpivot-midcap50", "app-vpivot:fibonacci:S1:15"},
	}
	total := 0
	for seed := int64(1); seed <= 6; seed++ {
		p := lotcapRandomPanel(1000+seed, 10, 320)
		dir := t.TempDir()
		csvPath := filepath.Join(dir, "panel.csv")
		lotcapWritePanelCSV(t, p, csvPath)
		for _, pr := range pairs {
			res, err := Run(p, lotcapRules(t, pr.goName))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, l := range res.Lots {
				got = append(got, fmt.Sprintf("%s,%s,%s,%d,%.4f", l.Symbol, l.BuyDate.Format("2006-01-02"), l.SellDate.Format("2006-01-02"), l.Qty, l.BuyPrice))
			}
			out, err := exec.Command(py, ref, csvPath, pr.pyMode, "1000000").Output()
			if err != nil {
				t.Fatalf("seed %d %s: reference simulator failed: %v", seed, pr.pyMode, err)
			}
			// the reference csv.writer ends rows with CR LF; the first row is the header
			lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r", "")), "\n")
			want := lines[1:]
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				n := 0
				for i := 0; i < len(got) && i < len(want); i++ {
					if got[i] != want[i] {
						n = i
						break
					}
				}
				t.Fatalf("seed %d %s vs ref %s: %d lots in Go, %d in reference; first difference at row %d:\n go  %v\n ref %v", seed, pr.goName, pr.pyMode, len(got), len(want), n, got[n:min(n+2, len(got))], want[n:min(n+2, len(want))])
			}
			total += len(got)
		}
	}
	t.Logf("Go simulator and reference simulator agree lot for lot on %d lots across %d presets x 6 random panels", total, len(pairs))
}
