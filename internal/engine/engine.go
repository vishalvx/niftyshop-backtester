package engine

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/portfolio"
)

type EngineBar struct {
	Open   float64
	Close  float64
	High   float64
	Low    float64
	Volume int64
	Date   time.Time
}

type EngineStock struct {
	EngineBar
	Symbol        string
	SMA20         float64
	DiffSMA       float64
	IsConstituent bool
}

// FindStock locates a stock by symbol in a slice of EngineStocks.
func FindStock(stocks []EngineStock, symbol string) (int, EngineStock) {
	for i, stock := range stocks {
		if stock.Symbol == symbol {
			return i, stock
		}
	}
	return -1, EngineStock{}
}

// isNewRevisionPeriod returns true when current falls in a new revision period
// relative to last, based on the given period string ("monthly", "quarterly", "yearly").
func isNewRevisionPeriod(current, last time.Time, period string) bool {
	switch period {
	case "monthly":
		return current.Year() != last.Year() || current.Month() != last.Month()
	case "quarterly":
		return current.Year() != last.Year() || quarter(current) != quarter(last)
	case "yearly":
		return current.Year() != last.Year()
	}
	return false
}

// quarter returns 1-4 for the calendar quarter of t.
func quarter(t time.Time) int {
	return (int(t.Month()) - 1) / 3
}

// RunNiftyShop executes the trading strategy on the daily data bucket.
func RunNiftyShop(dateWiseBucket map[string][]EngineStock, cfg *config.Config) (*portfolio.Account, *portfolio.Strategy) {
	if len(dateWiseBucket) == 0 {
		fmt.Println("*** There is nothing to process")
		return nil, nil
	}

	account := portfolio.NewAccount(float64(cfg.StartCapital))
	strategy := portfolio.NewStrategy("Nifty Shop", dateWiseBucket)

	// Sort dates to ensure chronological processing
	var dates []string
	for d := range dateWiseBucket {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// Capital revision state
	startCapital := float64(cfg.StartCapital)
	var realizedPnL float64
	slotSize := startCapital / cfg.CapitalDivider

	// Parse start date as the initial revision anchor
	lastRevisionDate, _ := time.Parse("2006-01-02", dates[0])

	for _, date := range dates {
		stocks := dateWiseBucket[date]
		currentDate, _ := time.Parse("2006-01-02", date)

		// Revise slot size at the start of each new revision period
		if isNewRevisionPeriod(currentDate, lastRevisionDate, cfg.RevisionPeriod) {
			slotSize = (startCapital + realizedPnL) / cfg.CapitalDivider
			lastRevisionDate = currentDate
			fmt.Printf(">>> Slot revised: start=%v realized=%v slot=%.2f\n", startCapital, realizedPnL, slotSize)
		}

		freshEntriesToday := 0

		// 1. Exits (Priority 1) — unlimited per day, updates realizedPnL
		for i := len(account.Positions) - 1; i >= 0; i-- {
			position := account.Positions[i]
			if _, stock := FindStock(stocks, position.Symbol); stock.Symbol != "" {
				if stock.Close >= position.Price*(1+cfg.ProfitTargetPct) {
					account.RemovePosition(position.ID, stock.Close)

					pnl := (stock.Close - position.Price) * float64(position.Lot)
					realizedPnL += pnl

					trade := portfolio.Trade{
						Symbol:    position.Symbol,
						Date:      stock.Date,
						Lot:       position.Lot,
						Price:     stock.Close,
						Action:    portfolio.Sell,
						ID:        position.ID,
						BuyPrice:  position.Price,
						PnL:       pnl,
						CashAfter: account.GetCapital(),
						SlotSize:  slotSize,
					}
					fmt.Printf(">>> SELL Trade: %v @ %v (PnL=%.2f)\n", trade.Symbol, trade.Price, pnl)
					strategy.AddTrade(trade)
				}
			}
		}

		// 2. Averaging (Priority 2) — 1 per stock per day, only if cash >= slotSize
		averagedToday := make(map[string]bool)
		for i := len(account.Positions) - 1; i >= 0; i-- {
			position := account.Positions[i]
			if averagedToday[position.Symbol] {
				continue
			}

			if _, stock := FindStock(stocks, position.Symbol); stock.Symbol != "" {
				if stock.IsConstituent && stock.Close > 0 && stock.Close <= position.Price*(1-cfg.AvgTriggerPct) {
					if account.GetCapital() >= slotSize {
						units := math.Floor(slotSize / stock.Close)
						if units > 0 {
							actualCost := units * stock.Close
							account.SetCapital(account.GetCapital() - actualCost)

							newLot := portfolio.Trade{
								ID:        uuid.New().String(),
								Symbol:    stock.Symbol,
								Date:      stock.Date,
								Lot:       int32(units),
								Price:     stock.Close,
								Action:    portfolio.Avg,
								BuyPrice:  stock.Close,
								PnL:       0,
								CashAfter: account.GetCapital(),
								SlotSize:  slotSize,
							}
							account.AddPosition(newLot)
							averagedToday[stock.Symbol] = true

							fmt.Printf(">>> AVG Trade: %v @ %v (slot=%.2f)\n", newLot.Symbol, newLot.Price, slotSize)
							strategy.AddTrade(newLot)
						}
					} else {
						fmt.Printf(">>> AVG skipped %v: cash=%.2f < slot=%.2f\n", position.Symbol, account.GetCapital(), slotSize)
					}
				}
			}
		}

		// 3. Fresh Entries (Priority 3) — strictly MaxFreshEntriesPerDay per day
		var radarStocks []EngineStock
		for _, stock := range stocks {
			if stock.IsConstituent && stock.DiffSMA > 0 {
				radarStocks = append(radarStocks, stock)
			}
		}

		for _, stock := range radarStocks {
			if freshEntriesToday >= cfg.MaxFreshEntriesPerDay {
				break
			}
			if account.HasPosition(stock.Symbol) {
				continue
			}
			if account.GetCapital() < slotSize {
				fmt.Printf(">>> FRESH skipped %v: cash=%.2f < slot=%.2f\n", stock.Symbol, account.GetCapital(), slotSize)
				break // skip all further entries — not enough capital
			}

			units := math.Floor(slotSize / stock.Close)
			if units > 0 {
				actualCost := units * stock.Close
				account.SetCapital(account.GetCapital() - actualCost)

				newLot := portfolio.Trade{
					ID:        uuid.New().String(),
					Symbol:    stock.Symbol,
					Date:      stock.Date,
					Lot:       int32(units),
					Price:     stock.Close,
					Action:    portfolio.Fresh,
					BuyPrice:  stock.Close,
					PnL:       0,
					CashAfter: account.GetCapital(),
					SlotSize:  slotSize,
				}
				account.AddPosition(newLot)
				freshEntriesToday++

				fmt.Printf(">>> FRESH Trade: %v @ %v (slot=%.2f, day-entry#%d)\n", newLot.Symbol, newLot.Price, slotSize, freshEntriesToday)
				strategy.AddTrade(newLot)
			}
		}
	}

	return account, strategy
}
