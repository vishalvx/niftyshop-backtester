package engine

import (
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/portfolio"
)

// ---- isNewRevisionPeriod tests ----

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustParse %q: %v", s, err)
	}
	return ts
}

func TestIsNewRevisionPeriod_Monthly_SameMonth(t *testing.T) {
	last := mustParse(t, "2021-01-05")
	current := mustParse(t, "2021-01-20")
	if isNewRevisionPeriod(current, last, "monthly") {
		t.Error("same month should NOT trigger revision")
	}
}

func TestIsNewRevisionPeriod_Monthly_NewMonth(t *testing.T) {
	last := mustParse(t, "2021-01-31")
	current := mustParse(t, "2021-02-01")
	if !isNewRevisionPeriod(current, last, "monthly") {
		t.Error("new month should trigger revision")
	}
}

func TestIsNewRevisionPeriod_Monthly_NewYear(t *testing.T) {
	last := mustParse(t, "2021-12-31")
	current := mustParse(t, "2022-01-01")
	if !isNewRevisionPeriod(current, last, "monthly") {
		t.Error("new year is also a new month — should trigger revision")
	}
}

func TestIsNewRevisionPeriod_Quarterly_SameQuarter(t *testing.T) {
	last := mustParse(t, "2021-01-01")
	current := mustParse(t, "2021-03-31")
	if isNewRevisionPeriod(current, last, "quarterly") {
		t.Error("Q1→Q1 should NOT trigger revision")
	}
}

func TestIsNewRevisionPeriod_Quarterly_NewQuarter(t *testing.T) {
	last := mustParse(t, "2021-03-31")
	current := mustParse(t, "2021-04-01")
	if !isNewRevisionPeriod(current, last, "quarterly") {
		t.Error("Q1→Q2 should trigger revision")
	}
}

func TestIsNewRevisionPeriod_Yearly_SameYear(t *testing.T) {
	last := mustParse(t, "2021-01-01")
	current := mustParse(t, "2021-12-31")
	if isNewRevisionPeriod(current, last, "yearly") {
		t.Error("same year should NOT trigger revision")
	}
}

func TestIsNewRevisionPeriod_Yearly_NewYear(t *testing.T) {
	last := mustParse(t, "2021-12-31")
	current := mustParse(t, "2022-01-01")
	if !isNewRevisionPeriod(current, last, "yearly") {
		t.Error("new year should trigger revision")
	}
}

// ---- Slot size / capital revision engine tests ----

func baseCfg() *config.Config {
	return &config.Config{
		Universe:              "nifty50",
		MAWindow:              20,
		ProfitTargetPct:       0.05,
		AvgTriggerPct:         0.03,
		MaxStocks:             5,
		CapitalDivider:        10,
		StartCapital:          100000,
		StartDate:             "2021-01-01",
		EndDate:               "2021-12-31",
		RevisionPeriod:        "monthly",
		MaxFreshEntriesPerDay: 1,
	}
}

func makeStock(symbol string, close float64, diffSMA float64, date time.Time, constituent bool) EngineStock {
	return EngineStock{
		Symbol:  symbol,
		DiffSMA: diffSMA,
		EngineBar: EngineBar{
			Close: close,
			Date:  date,
		},
		IsConstituent: constituent,
	}
}

// TestEngine_OneFreshEntryPerDay verifies that only 1 fresh position is opened
// per day even when multiple qualifying stocks are available.
func TestEngine_OneFreshEntryPerDay(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxFreshEntriesPerDay = 1

	day1, _ := time.Parse("2006-01-02", "2021-01-04")

	bucket := map[string][]EngineStock{
		"2021-01-04": {
			makeStock("A.NS", 90.0, 15.0, day1, true),
			makeStock("B.NS", 80.0, 10.0, day1, true),
			makeStock("C.NS", 70.0, 8.0, day1, true),
		},
	}

	_, strategy := RunNiftyShop(bucket, cfg)

	freshBuys := 0
	for _, t := range strategy.History {
		if t.Action == portfolio.Fresh {
			freshBuys++
		}
	}
	if freshBuys != 1 {
		t.Errorf("expected exactly 1 fresh buy, got %d", freshBuys)
	}
}

// TestEngine_SkipFreshEntryWhenCapitalFullyDeployed ensures that when available
// cash < slotSize, no fresh entry is made.
func TestEngine_SkipFreshEntryWhenCapitalFullyDeployed(t *testing.T) {
	cfg := baseCfg()
	cfg.StartCapital = 100000
	cfg.CapitalDivider = 10 // slot = 10000

	day1, _ := time.Parse("2006-01-02", "2021-01-04")
	day2, _ := time.Parse("2006-01-02", "2021-01-05")

	// Day 1: buy 10 slots worth, exhaust cash (10 different stocks to fill capital)
	stocks := []EngineStock{}
	symbols := []string{"A.NS", "B.NS", "C.NS", "D.NS", "E.NS", "F.NS", "G.NS", "H.NS", "I.NS", "J.NS"}
	for _, sym := range symbols {
		stocks = append(stocks, makeStock(sym, 100.0, 10.0, day1, true))
	}

	// Day 2: a new stock qualifies, but we have no cash left
	stocks2 := []EngineStock{
		makeStock("NEW.NS", 100.0, 12.0, day2, true),
	}

	// We run with MaxFreshEntriesPerDay=10 so day1 fills up all 10 slots
	cfg.MaxFreshEntriesPerDay = 10
	bucket := map[string][]EngineStock{
		"2021-01-04": stocks,
		"2021-01-05": stocks2,
	}

	_, strategy := RunNiftyShop(bucket, cfg)

	// NEW.NS should never be bought — no cash available
	for _, trade := range strategy.History {
		if trade.Symbol == "NEW.NS" {
			t.Errorf("expected NEW.NS to NOT be bought (no cash), but it was: action=%s", trade.Action)
		}
	}
}

// TestEngine_SlotSizeGrowsAfterRevision verifies that after a revision period,
// the slot size reflects realized gains.
func TestEngine_SlotSizeGrowsAfterRevision(t *testing.T) {
	cfg := baseCfg()
	cfg.StartCapital = 100000
	cfg.CapitalDivider = 10    // initial slot = 10000
	cfg.ProfitTargetPct = 0.06 // 6% profit target
	cfg.MaxFreshEntriesPerDay = 1
	cfg.RevisionPeriod = "monthly"

	// Jan 4: buy A.NS at 100 (slot = 10000, gets 100 units)
	// Jan 5: A.NS hits target at 106 (>= 100 * 1.06) → sell, realized PnL = 600
	// Feb 1: new month → slot revised = (100000 + 600) / 10 = 10060
	// Feb 1: buy B.NS at 100 using new slot size (10060 units ~ 100 units)

	jan4, _ := time.Parse("2006-01-02", "2021-01-04")
	jan5, _ := time.Parse("2006-01-02", "2021-01-05")
	feb1, _ := time.Parse("2006-01-02", "2021-02-01")

	bucket := map[string][]EngineStock{
		"2021-01-04": {makeStock("A.NS", 100.0, 10.0, jan4, true)},
		"2021-01-05": {makeStock("A.NS", 106.0, 5.0, jan5, true)},  // hits 6% target
		"2021-02-01": {makeStock("B.NS", 100.0, 12.0, feb1, true)}, // new month, new slot
	}

	_, strategy := RunNiftyShop(bucket, cfg)

	// Find the B.NS fresh buy trade
	var bFreshLot int32
	for _, trade := range strategy.History {
		if trade.Symbol == "B.NS" && trade.Action == portfolio.Fresh {
			bFreshLot = trade.Lot
		}
	}

	// With slot = 10060 and price = 100, units = floor(10060/100) = 100
	// With original slot = 10000, also floor(10000/100) = 100
	// So Lot count is same; we verify SlotSize field on the B.NS trade > 10000
	var bSlotSize float64
	for _, trade := range strategy.History {
		if trade.Symbol == "B.NS" && trade.Action == portfolio.Fresh {
			bSlotSize = trade.SlotSize
		}
	}

	if bFreshLot == 0 {
		t.Fatal("expected B.NS to be bought in February")
	}
	if bSlotSize <= 10000 {
		t.Errorf("expected B.NS SlotSize > 10000 (grew from realized P&L), got %.2f", bSlotSize)
	}
}

// TestEngine_AveragingSkippedWhenCashBelowSlot ensures averaging is not done
// when available cash is below the current slot size.
//
// Setup: StartCapital=11000, divider=10, slot=1100.
// Day 1: buy 10 stocks at 1000 each (floor(1100/1000)=1 unit, cost=1000 each).
//
//	After 10 buys: cash = 11000 - (10 * 1000) = 1000.
//
// Day 2: A.NS drops to 960 (<=1000*0.97=970), triggers avg check.
//
//	cash(1000) < slot(1100) → NO averaging.
func TestEngine_AveragingSkippedWhenCashBelowSlot(t *testing.T) {
	cfg := baseCfg()
	cfg.StartCapital = 11000
	cfg.CapitalDivider = 10 // slot = 1100
	cfg.MaxFreshEntriesPerDay = 10

	day1, _ := time.Parse("2006-01-02", "2021-01-04")
	day2, _ := time.Parse("2006-01-02", "2021-01-05")

	symbols := []string{"A.NS", "B.NS", "C.NS", "D.NS", "E.NS", "F.NS", "G.NS", "H.NS", "I.NS", "J.NS"}
	day1Stocks := make([]EngineStock, len(symbols))
	for i, sym := range symbols {
		day1Stocks[i] = makeStock(sym, 1000.0, 10.0, day1, true)
	}

	bucket := map[string][]EngineStock{
		"2021-01-04": day1Stocks,
		// A.NS drops to 960 — triggers avg (960 <= 1000 * 0.97 = 970)
		// cash=1000 < slot=1100 → should NOT avg
		"2021-01-05": {makeStock("A.NS", 960.0, 5.0, day2, true)},
	}

	_, strategy := RunNiftyShop(bucket, cfg)

	avgCount := 0
	for _, trade := range strategy.History {
		if trade.Symbol == "A.NS" && trade.Action == portfolio.Avg {
			avgCount++
		}
	}

	if avgCount != 0 {
		t.Errorf("expected 0 avg trades (cash=1000 < slot=1100), got %d", avgCount)
	}
}

// TestRunNiftyShop_Rebalancing was in engine_test.go — keep it here after merging.
// (Kept in engine_test.go as-is; this file adds the NEW tests only.)

func TestEngine_TradeHasSlotSizeField(t *testing.T) {
	cfg := baseCfg()
	cfg.StartCapital = 100000
	cfg.CapitalDivider = 10 // slot = 10000

	day1, _ := time.Parse("2006-01-02", "2021-01-04")
	bucket := map[string][]EngineStock{
		"2021-01-04": {makeStock("A.NS", 100.0, 10.0, day1, true)},
	}

	_, strategy := RunNiftyShop(bucket, cfg)

	for _, trade := range strategy.History {
		if trade.SlotSize <= 0 {
			t.Errorf("expected SlotSize > 0 on trade %s, got %.2f", trade.Symbol, trade.SlotSize)
		}
	}
}
