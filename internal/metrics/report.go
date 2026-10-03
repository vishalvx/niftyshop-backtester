package metrics

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/portfolio"
)

type Report struct {
	StartCapital  float64
	FinalValue    float64
	CashRemaining float64
	HoldingsValue float64
	TotalReturn   float64
	CAGR          float64
	NumTrades     int
	FreshBuys     int
	AvgBuys       int
	Sells         int
	WinRate       float64
	MinHolding    float64
	AvgHolding    float64
	MaxHolding    float64
}

// Generate calculates metrics and saves the trade log
func Generate(account *portfolio.Account, strategy *portfolio.Strategy, cfg *config.Config, currentPrices map[string]float64) *Report {
	startCapital := float64(cfg.StartCapital)

	cashRemaining := account.GetCapital()
	holdingsValue := 0.0

	for _, pos := range account.GetCurrentHoldings() {
		if currentPrice, ok := currentPrices[pos.Symbol]; ok {
			holdingsValue += float64(pos.Lot) * currentPrice
		} else {
			// fallback to last known price
			holdingsValue += float64(pos.Lot) * pos.Price
		}
	}

	finalValue := cashRemaining + holdingsValue
	totalReturn := (finalValue - startCapital) / startCapital

	start, err := time.Parse("2006-01-02", cfg.StartDate)
	if err != nil {
		start = time.Now().AddDate(-5, 0, 0) // fallback
	}
	end, err := time.Parse("2006-01-02", cfg.EndDate)
	if err != nil {
		end = time.Now() // fallback
	}

	years := end.Sub(start).Hours() / (24 * 365.25)
	if years <= 0 {
		years = 1.0 // avoid division by zero
	}

	cagr := math.Pow(finalValue/startCapital, 1/years) - 1

	freshBuys := 0
	avgBuys := 0
	sells := 0
	winTrades := 0

	buyDates := make(map[string]time.Time)
	var holdingPeriods []float64

	for _, trade := range strategy.History {
		if trade.Action == portfolio.Fresh {
			freshBuys++
			buyDates[trade.ID] = trade.Date
		} else if trade.Action == portfolio.Avg {
			avgBuys++
			buyDates[trade.ID] = trade.Date
		} else if trade.Action == portfolio.Sell {
			sells++
			winTrades++ // Assuming all exits in our strategy are target hits (wins)
			if buyDate, ok := buyDates[trade.ID]; ok {
				days := trade.Date.Sub(buyDate).Hours() / 24.0
				holdingPeriods = append(holdingPeriods, days)
			}
		}
	}

	minHolding := 0.0
	maxHolding := 0.0
	avgHolding := 0.0
	if len(holdingPeriods) > 0 {
		minHolding = holdingPeriods[0]
		maxHolding = holdingPeriods[0]
		sum := 0.0
		for _, hp := range holdingPeriods {
			if hp < minHolding {
				minHolding = hp
			}
			if hp > maxHolding {
				maxHolding = hp
			}
			sum += hp
		}
		avgHolding = sum / float64(len(holdingPeriods))
	}

	winRate := 0.0
	if sells > 0 {
		winRate = float64(winTrades) / float64(sells)
	}

	report := &Report{
		StartCapital:  startCapital,
		FinalValue:    finalValue,
		CashRemaining: cashRemaining,
		HoldingsValue: holdingsValue,
		TotalReturn:   totalReturn * 100,
		CAGR:          cagr * 100,
		NumTrades:     len(strategy.History),
		FreshBuys:     freshBuys,
		AvgBuys:       avgBuys,
		Sells:         sells,
		WinRate:       winRate * 100,
		MinHolding:    minHolding,
		AvgHolding:    avgHolding,
		MaxHolding:    maxHolding,
	}

	// Export Trade Log
	exportTradeLog(strategy.History, cfg.Universe)

	return report
}

func exportTradeLog(history []portfolio.Trade, universe string) {
	err := os.MkdirAll("reports", 0755)
	if err != nil {
		fmt.Printf("Error creating reports directory: %v\n", err)
		return
	}
	filename := filepath.Join("reports", fmt.Sprintf("trade_logs_book_%s.csv", strings.ReplaceAll(universe, " ", "_")))
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating trade log file %s: %v\n", filename, err)
		return
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	w.Write([]string{"Date", "Symbol", "Action", "Quantity", "Buy Price", "Sell Price", "PnL", "Return %", "Cash Remaining", "Slot Size"})

	for _, t := range history {
		sellPrice := ""
		buyPrice := fmt.Sprintf("%.2f", t.BuyPrice)
		retPct := ""

		if t.Action == portfolio.Sell {
			sellPrice = fmt.Sprintf("%.2f", t.Price)
			retPct = fmt.Sprintf("%.2f%%", ((t.Price-t.BuyPrice)/t.BuyPrice)*100)
		} else {
			// For Buy or Avg, Price is the BuyPrice.
			sellPrice = "-"
			retPct = "-"
		}

		w.Write([]string{
			t.Date.Format("2006-01-02"),
			t.Symbol,
			t.Action.String(),
			strconv.Itoa(int(t.Lot)),
			buyPrice,
			sellPrice,
			fmt.Sprintf("%.2f", t.PnL),
			retPct,
			fmt.Sprintf("%.2f", t.CashAfter),
			fmt.Sprintf("%.2f", t.SlotSize),
		})
	}
	fmt.Printf("Trade log exported to %s\n", filename)
}

func Print(r *Report) {
	fmt.Println("\n=================================")
	fmt.Println("       BACKTEST RESULTS        ")
	fmt.Println("=================================")
	fmt.Printf("Start Capital:  ₹%.2f\n", r.StartCapital)
	fmt.Printf("Final Value:    ₹%.2f\n", r.FinalValue)
	fmt.Println("---------------------------------")
	fmt.Printf("Cash Remaining: ₹%.2f\n", r.CashRemaining)
	fmt.Printf("Holdings Value: ₹%.2f\n", r.HoldingsValue)
	fmt.Println("---------------------------------")
	fmt.Printf("Total Return:   %.2f%%\n", r.TotalReturn)
	fmt.Printf("CAGR:           %.2f%%\n", r.CAGR)
	fmt.Println("---------------------------------")
	fmt.Printf("Total Trades:   %d\n", r.NumTrades)
	fmt.Printf("  - Fresh Buys: %d\n", r.FreshBuys)
	fmt.Printf("  - Avg Buys:   %d\n", r.AvgBuys)
	fmt.Printf("  - Sells:      %d\n", r.Sells)
	fmt.Printf("Win Rate:       %.2f%%\n", r.WinRate)
	fmt.Println("---------------------------------")
	fmt.Printf("Min Holding:    %.0f days\n", r.MinHolding)
	fmt.Printf("Avg Holding:    %.0f days\n", r.AvgHolding)
	fmt.Printf("Max Holding:    %.0f days\n", r.MaxHolding)
	fmt.Println("=================================")
}
