package engine

import (
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/portfolio"
)

func TestRunNiftyShop_Rebalancing(t *testing.T) {
	cfg := &config.Config{
		Universe:              "nifty50",
		MAWindow:              20,
		ProfitTargetPct:       0.05,
		AvgTriggerPct:         0.03,
		MaxStocks:             5,
		CapitalDivider:        10,
		StartCapital:          100000,
		StartDate:             "2021-01-01",
		EndDate:               "2021-01-03",
		RevisionPeriod:        "monthly",
		MaxFreshEntriesPerDay: 1,
	}

	dateWiseBucket := make(map[string][]EngineStock)

	day1, _ := time.Parse("2006-01-02", "2021-01-01")
	day2, _ := time.Parse("2006-01-02", "2021-01-02")
	day3, _ := time.Parse("2006-01-02", "2021-01-03")

	// Day 1: X.NS is below SMA and IS constituent. Y.NS is below SMA but is NOT constituent.
	dateWiseBucket["2021-01-01"] = []EngineStock{
		{
			Symbol:  "X.NS",
			SMA20:   100.0,
			DiffSMA: 10.0, // (100 - 90)/90 * 100 > 0
			EngineBar: EngineBar{
				Close: 90.0,
				Date:  day1,
			},
			IsConstituent: true,
		},
		{
			Symbol:  "Y.NS",
			SMA20:   100.0,
			DiffSMA: 20.0,
			EngineBar: EngineBar{
				Close: 80.0,
				Date:  day1,
			},
			IsConstituent: false,
		},
	}

	// Day 2: X.NS is no longer constituent, but falls below averaging trigger (Close = 85.0 <= 90.0 * 0.97)
	dateWiseBucket["2021-01-02"] = []EngineStock{
		{
			Symbol:  "X.NS",
			SMA20:   100.0,
			DiffSMA: 15.0,
			EngineBar: EngineBar{
				Close: 85.0,
				Date:  day2,
			},
			IsConstituent: false,
		},
	}

	// Day 3: X.NS price rises to 95.0, hitting exit target (>= 90.0 * 1.05 = 94.5)
	dateWiseBucket["2021-01-03"] = []EngineStock{
		{
			Symbol:  "X.NS",
			SMA20:   100.0,
			DiffSMA: 5.0,
			EngineBar: EngineBar{
				Close: 95.0,
				Date:  day3,
			},
			IsConstituent: false,
		},
	}

	account, strategy := RunNiftyShop(dateWiseBucket, cfg)

	if account == nil || strategy == nil {
		t.Fatalf("RunNiftyShop returned nil")
	}

	// Verify Y.NS was never bought, and X.NS was bought on Day 1
	var hasX, hasY bool
	var totalTrades int
	var avgTrades int
	var sellTrades int

	for _, trade := range strategy.History {
		totalTrades++
		if trade.Symbol == "Y.NS" {
			hasY = true
		}
		if trade.Symbol == "X.NS" {
			hasX = true
			if trade.Action == portfolio.Avg {
				avgTrades++
			}
			if trade.Action == portfolio.Sell {
				sellTrades++
			}
		}
	}

	if !hasX {
		t.Errorf("Expected X.NS to be traded")
	}
	if hasY {
		t.Errorf("Expected Y.NS to NOT be traded because it is not a constituent")
	}
	if avgTrades > 0 {
		t.Errorf("Expected 0 AVG trades for X.NS because it was removed from constituents on Day 2, got %d", avgTrades)
	}
	if sellTrades != 1 {
		t.Errorf("Expected 1 Sell trade for X.NS, got %d", sellTrades)
	}
	if len(account.GetCurrentHoldings()) != 0 {
		t.Errorf("Expected all positions to be closed, still holding: %v", account.GetCurrentHoldings())
	}
}
