// Package experiment runs rule variants through the simulator, computes metrics at four accounting stages and
// appends every run to an experiment log, so that no trial is ever forgotten when results are later judged.
package experiment

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/bench"
	"github.com/vishalvx/back-tester/internal/costs"
	"github.com/vishalvx/back-tester/internal/panel"
	"github.com/vishalvx/back-tester/internal/sim"
)

// Stage is the accounting basis of a run.
type Stage string

const (
	Gross    Stage = "gross"     // no costs, no tax (what the engine reports)
	Costed   Stage = "cost"      // statutory charges + slippage (+ brokerage in harsh scenario)
	TaxDated Stage = "tax-dated" // costed + capital-gains tax at the rates in force on each sale date
	TaxToday Stage = "tax-today" // costed + today's rates (20% STCG, 12.5% LTCG over 1.25 lakh) for all years
)

// Stages in reporting order.
var Stages = []Stage{Gross, Costed, TaxDated, TaxToday}

// Scenario describes friction assumptions.
type Scenario struct {
	Name         string
	BrokerageBps float64
	SlippageBps  float64
	DivTaxRate   float64
}

// Base is a zero-brokerage discount broker plus 10 bps adverse fill per side.
var Base = Scenario{Name: "base", BrokerageBps: 0, SlippageBps: 10, DivTaxRate: 0.312}

// Harsh is 0.10% brokerage and 25 bps slippage per side.
var Harsh = Scenario{Name: "harsh", BrokerageBps: 10, SlippageBps: 25, DivTaxRate: 0.312}

// Env is everything shared by the runs of one study.
type Env struct {
	Panel    *panel.Panel
	TRI, PRI []analytics.Point
	TRIName  string // what TRI is, e.g. "NIFTY 50 TRI"
	Rf       float64
	Capital  float64
	Window   string // label, e.g. "nifty50 2008-01..2025-08"
	LogPath  string
	GitSHA   string
	Scenario Scenario
}

// logMu serialises appends to the experiment log across goroutines.
var logMu sync.Mutex

// WithCapital returns a copy of the environment with a different starting capital.
func (e *Env) WithCapital(c float64) *Env {
	cp := *e
	cp.Capital = c
	return &cp
}

// Spec is a named variant.
type Spec struct {
	ID    string
	Rules sim.Rules
	Notes string
}

// Outcome is one (variant, stage) run.
type Outcome struct {
	Spec    Spec
	Stage   Stage
	Result  *sim.Result
	Metrics analytics.Metrics
	Final   float64 // final equity after realised tax, minus terminal liquidation tax
	CAGR    float64
	Sharpe  float64
	TaxPaid float64
	Charges float64
	Divs    float64
}

// fmv returns an FMV-on-31-Jan-2018 lookup from the panel's series.
func fmv(p *panel.Panel) func(string) (float64, bool) {
	cut := time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC)
	return func(sym string) (float64, bool) {
		s, ok := p.Series[sym]
		if !ok {
			return 0, false
		}
		v := 0.0
		for _, b := range s.Bars {
			if b.Date.After(cut) {
				break
			}
			v = b.Close
		}
		return v, v > 0
	}
}

// Prepare returns rules configured for a stage.
func (e *Env) Prepare(r sim.Rules, st Stage) sim.Rules {
	r.StartCapital = e.Capital
	r.Costs, r.Tax = nil, nil
	r.SlippageBps = 0
	if st == Gross {
		return r
	}
	r.Costs = costs.IndianDelivery{BrokerageBps: e.Scenario.BrokerageBps, BrokerageMin: 0, Statutory: true}
	r.SlippageBps = e.Scenario.SlippageBps
	switch st {
	case TaxDated:
		tb := costs.NewTaxBook(costs.Dated)
		tb.FMV31Jan2018 = fmv(e.Panel)
		tb.DividendTaxRate = e.Scenario.DivTaxRate
		r.Tax = tb
	case TaxToday:
		tb := costs.NewTaxBook(costs.Today)
		tb.DividendTaxRate = e.Scenario.DivTaxRate
		r.Tax = tb
	}
	return r
}

// Run executes a variant at a stage and computes all metrics.
func (e *Env) Run(spec Spec, st Stage) (*Outcome, error) {
	rules := e.Prepare(spec.Rules, st)
	res, err := sim.Run(e.Panel, rules)
	if err != nil {
		return nil, err
	}
	return e.Evaluate(spec, st, res), nil
}

// Evaluate turns a simulation result into metrics.
func (e *Env) Evaluate(spec Spec, st Stage, res *sim.Result) *Outcome {
	eq := make([]analytics.Point, len(res.Points))
	inv := make([]float64, len(res.Points))
	stocks := make([]int, len(res.Points))
	for i, p := range res.Points {
		eq[i] = analytics.Point{Date: p.Date, Value: p.Equity}
		inv[i] = p.Holdings
		stocks[i] = p.Stocks
	}
	final := res.FinalEquity - res.TerminalTax
	eq[len(eq)-1].Value = final // liquidation-adjusted last point (tax stages only; TerminalTax is 0 otherwise)
	var trades []analytics.Trade
	var traded float64
	for _, l := range res.Lots {
		ch := l.BuyCharge.Total + l.SellCharge.Total
		trades = append(trades, analytics.Trade{Symbol: l.Symbol, BuyDate: l.BuyDate, SellDate: l.SellDate, Qty: float64(l.Qty),
			BuyPrice: l.BuyPrice, SellPrice: l.SellPrice, Charges: ch, Open: l.Open, MinPrice: l.MinClose})
		traded += l.BuyPrice * float64(l.Qty)
		if !l.Open {
			traded += l.SellPrice * float64(l.Qty)
		}
	}
	bm := bench.Clip(e.TRI, res.Points[0].Date.AddDate(0, 0, -7), res.Points[len(res.Points)-1].Date)
	m := analytics.Compute(analytics.Input{StartCapital: res.StartCap, Equity: eq, Benchmark: bm, Trades: trades,
		Invested: inv, Stocks: stocks, TradedValue: traded, Rf: e.Rf})
	o := &Outcome{Spec: spec, Stage: st, Result: res, Metrics: m, Final: final, CAGR: m.CAGR, Sharpe: m.Sharpe,
		TaxPaid: res.TaxPaid + res.TerminalTax, Charges: res.CostsPaid, Divs: res.DivReceived}
	e.log(o)
	return o
}

type logLine struct {
	Time    string  `json:"time"`
	Git     string  `json:"git"`
	Window  string  `json:"window"`
	Variant string  `json:"variant"`
	Stage   string  `json:"stage"`
	Rules   any     `json:"rules"`
	CAGR    float64 `json:"cagr"`
	Sharpe  float64 `json:"sharpe"`
	MaxDD   float64 `json:"max_dd"`
	Final   float64 `json:"final"`
	Lots    int     `json:"closed_lots"`
}

func (e *Env) log(o *Outcome) {
	if e.LogPath == "" {
		return
	}
	r := o.Spec.Rules
	rules := map[string]any{"target": r.ProfitTarget, "avg": r.AvgTrigger, "exit": r.ExitBasis, "avg_basis": r.AvgBasis,
		"max_stocks": r.MaxStocks, "max_lots": r.MaxLots, "stop": r.StopLoss, "time_stop": r.TimeStopDays, "trail": r.TrailPct,
		"pivot": r.Pivot, "next_open": r.FillNextOpen, "dividends": r.Dividends, "cash_yield": r.CashYield, "ma": r.MAWindow}
	line := logLine{Time: time.Now().UTC().Format(time.RFC3339), Git: e.GitSHA, Window: e.Window, Variant: o.Spec.ID, Stage: string(o.Stage),
		Rules: rules, CAGR: o.CAGR, Sharpe: o.Sharpe, MaxDD: o.Metrics.MaxDD.Depth, Final: o.Final, Lots: o.Metrics.ClosedLots}
	b, _ := json.Marshal(line)
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(e.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, string(b))
}
