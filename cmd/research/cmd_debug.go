package main

import (
	"fmt"
	"sort"

	"github.com/vishalvx/back-tester/internal/experiment"
)

// cmdDebug prints the worst lots and deepest equity points for one preset (diagnostics).
func cmdDebug(c common, variant string, dividends bool) error {
	env, _, err := buildEnv(c)
	if err != nil {
		return err
	}
	spec, err := presetByName(variant)
	if err != nil {
		return err
	}
	spec.Rules.Dividends = dividends
	o, err := env.Run(spec, experiment.Gross)
	if err != nil {
		return err
	}
	res := o.Result
	lots := append([]interface{}{}, nil)
	_ = lots
	type row struct {
		sym        string
		buy, sell  string
		qty        int
		bp, sp, pl float64
		open       bool
	}
	var rows []row
	for _, l := range res.Lots {
		rows = append(rows, row{l.Symbol, l.BuyDate.Format("2006-01-02"), l.SellDate.Format("2006-01-02"), l.Qty, l.BuyPrice, l.SellPrice, (l.SellPrice - l.BuyPrice) * float64(l.Qty), l.Open})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pl < rows[j].pl })
	fmt.Println("worst 12 lots by P&L:")
	for _, r := range rows[:12] {
		fmt.Printf("  %-12s buy %s @%.2f x%d -> %s @%.2f  pnl %.0f open=%v\n", r.sym, r.buy, r.bp, r.qty, r.sell, r.sp, r.pl, r.open)
	}
	// equity trajectory every ~year
	for i := 0; i < len(res.Points); i += 250 {
		p := res.Points[i]
		fmt.Printf("  %s equity %.0f cash %.0f holdings %.0f lots %d stocks %d\n", p.Date.Format("2006-01-02"), p.Equity, p.Cash, p.Holdings, p.Lots, p.Stocks)
	}
	return nil
}
