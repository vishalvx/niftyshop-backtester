package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vishalvx/back-tester/internal/analytics"
	"github.com/vishalvx/back-tester/internal/experiment"
)

func fnum(x float64, f string) string {
	if math.IsInf(x, 1) {
		return "inf"
	}
	if math.IsNaN(x) {
		return "n/a"
	}
	return fmt.Sprintf(f, x)
}

// printScorecard prints every metric for one outcome.
func printScorecard(o *experiment.Outcome) {
	m := o.Metrics
	fmt.Printf("\n=== %s | stage %s | %s .. %s (%.2f years) ===\n", o.Spec.ID, o.Stage, m.Start.Format("2006-01-02"), m.End.Format("2006-01-02"), m.Years)
	fmt.Printf("final value Rs %.0f (start %.0f)  total return %s  CAGR %s  tax paid+terminal Rs %.0f  charges Rs %.0f  dividends Rs %.0f\n", o.Final, m.StartValue, pct(m.TotalReturn), pct(m.CAGR), o.TaxPaid, o.Charges, o.Divs)
	fmt.Printf("annual vol %s  Sharpe %.2f  Sortino %.2f  Calmar %.2f  Ulcer %.1f%%  UPI %.2f\n", pct(m.Vol), m.Sharpe, m.Sortino, m.Calmar, m.Ulcer*100, m.UlcerPerf)
	d := m.MaxDD
	rec := "NOT recovered by end"
	if d.Recovered {
		rec = fmt.Sprintf("recovered %s (%.0f days after trough)", d.RecoveryDate.Format("2006-01-02"), d.DaysTroughToRecovery)
	}
	fmt.Printf("daily return distribution: skew %.2f, excess kurtosis %.1f, 1-day VaR95 %s, CVaR95 %s, best day %s, worst day %s | average drawdown %s | longest run of losing months %d\n", m.Skew, m.ExcessKurtosis, pct(m.VaR95), pct(m.CVaR95), pct(m.BestDay), pct(m.WorstDay), pct(m.AvgDrawdown), m.MaxLosingMonths)
	fmt.Printf("max drawdown %s: peak %s -> trough %s (%.0f days), %s; underwater %.0f days\n", pct(d.Depth), d.PeakDate.Format("2006-01-02"), d.TroughDate.Format("2006-01-02"), d.DaysPeakToTrough, rec, d.DaysUnderwater)
	if u := m.LongestUnderwater; u.Days > 0 {
		st := "recovered " + u.EndDate.Format("2006-01-02")
		if !u.Recovered {
			st = "NOT recovered by " + u.EndDate.Format("2006-01-02")
		}
		fmt.Printf("longest underwater stretch %.0f days (%.1f years): peak %s, %s, deepest point inside %s\n", u.Days, u.Days/365.25, u.PeakDate.Format("2006-01-02"), st, pct(u.Depth))
	}
	fmt.Printf("best year %d %s  worst year %d %s  positive months %s of %d\n", m.BestYear.Year, pct(m.BestYear.Return), m.WorstYear.Year, pct(m.WorstYear.Return), pct(m.PositiveMonths), m.Months)
	for _, r := range m.Rolling {
		fmt.Printf("rolling %dy CAGR: windows %d  min %s  median %s  max %s  share>=15%% %s  share<0 %s\n", r.Years, r.Windows, pct(r.Min), pct(r.Median), pct(r.Max), pct(r.ShareAbove), pct(r.ShareBelow0))
	}
	fmt.Printf("lots: closed %d open %d | win rate closed %s, all-with-open-MTM %s | avg win %s (Rs %.0f) avg loss %s (Rs %.0f) | win/loss ratio %s\n",
		m.ClosedLots, m.OpenLots, pct(m.WinRateClosed), pct(m.WinRateAll), pct(m.AvgWinPct), m.AvgWinRs, pct(m.AvgLossPct), m.AvgLossRs, fnum(m.WinLossRatio, "%.2f"))
	fmt.Printf("profit factor closed %s all %s | expectancy per closed lot %s (Rs %.0f)\n", fnum(m.ProfitFactorClosed, "%.2f"), fnum(m.ProfitFactorAll, "%.2f"), pct(m.ExpectancyPct), m.ExpectancyRs)
	fmt.Printf("holding days: avg(closed lots) %.0f, avg(all lots incl. open) %.0f, max closed %.0f | longest-held losing lot %.0f days | open underwater lots %d (longest %.0f days) | worst single-lot drawdown %s | open unrealised %s of equity\n",
		m.AvgHoldDays, m.AvgHoldDaysAll, m.MaxHoldDays, m.LongestLosingHoldDays, m.OpenLoserCount, m.LongestOpenLoserDays, pct(m.WorstLotDrawdown), pct(m.OpenUnrealisedPct))
	fmt.Printf("exposure: days with a position %s, avg invested %s, max invested %s, avg stocks held %.1f | turnover %.2fx/yr | charges drag %s/yr\n",
		pct(m.ExposureDays), pct(m.AvgInvestedPct), pct(m.MaxInvestedPct), m.AvgStocks, m.TurnoverPerYear, pct(m.CostDragAnnual))
	if r := m.Rel; r != nil {
		fmt.Printf("vs benchmark TRI (%d months): beta %.2f alpha %s corr %.2f TE %s IR %.2f up-capture %.0f%% down-capture %.0f%% bench CAGR %s\n",
			r.Months, r.Beta, pct(r.Alpha), r.Correlation, pct(r.TrackingError), r.InfoRatio, r.UpCapture*100, r.DownCapture*100, pct(r.BenchCAGR))
	}
	var ys []string
	for _, y := range m.YearReturns {
		p := ""
		if y.Partial {
			p = "*"
		}
		ys = append(ys, fmt.Sprintf("%d:%.1f%%%s", y.Year, y.Return*100, p))
	}
	fmt.Println("calendar years:", strings.Join(ys, " "))
	var eqp []analytics.Point
	for i, p := range o.Result.Points {
		v := p.Equity
		if i == len(o.Result.Points)-1 {
			v = o.Final
		}
		eqp = append(eqp, analytics.Point{Date: p.Date, Value: v})
	}
	fmt.Println("stress windows:", stressLine(eqp))
}

// exportOutcome writes the equity curve and lot ledger for external verification.
func exportOutcome(dir string, o *experiment.Outcome) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, fmt.Sprintf("%s_%s", o.Spec.ID, o.Stage))
	f, err := os.Create(base + "_equity.csv")
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write([]string{"date", "equity", "cash", "holdings", "lots", "stocks"})
	for i, p := range o.Result.Points {
		eq := p.Equity
		if i == len(o.Result.Points)-1 {
			eq = o.Final
		}
		w.Write([]string{p.Date.Format("2006-01-02"), strconv.FormatFloat(eq, 'f', 4, 64), strconv.FormatFloat(p.Cash, 'f', 4, 64), strconv.FormatFloat(p.Holdings, 'f', 4, 64), strconv.Itoa(p.Lots), strconv.Itoa(p.Stocks)})
	}
	w.Flush()
	f.Close()
	g, err := os.Create(base + "_lots.csv")
	if err != nil {
		return err
	}
	w = csv.NewWriter(g)
	w.Write([]string{"symbol", "buy_date", "sell_date", "qty", "buy_price", "sell_price", "charges", "open", "reason", "min_close"})
	for _, l := range o.Result.Lots {
		w.Write([]string{l.Symbol, l.BuyDate.Format("2006-01-02"), l.SellDate.Format("2006-01-02"), strconv.Itoa(l.Qty),
			strconv.FormatFloat(l.BuyPrice, 'f', 4, 64), strconv.FormatFloat(l.SellPrice, 'f', 4, 64),
			strconv.FormatFloat(l.BuyCharge.Total+l.SellCharge.Total, 'f', 4, 64), strconv.FormatBool(l.Open), l.Reason, strconv.FormatFloat(l.MinClose, 'f', 4, 64)})
	}
	w.Flush()
	return g.Close()
}

var _ = analytics.Point{}
