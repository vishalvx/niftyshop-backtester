package engine

import (
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/portfolio"
)

func TestRunNiftyShop_PivotFilter(t *testing.T) {
	cfg := &config.Config{
		Universe:              "nifty50",
		MAWindow:              20,
		ProfitTargetPct:       0.05,
		AvgTriggerPct:         0.03,
		MaxStocks:             5,
		CapitalDivider:        10,
		StartCapital:          100000,
		StartDate:             "2021-01-01",
		EndDate:               "2021-01-01",
		RevisionPeriod:        "monthly",
		MaxFreshEntriesPerDay: 1,
		PivotFilter: config.PivotFilterConfig{
			Enabled:  true,
			System:   "classic",
			Level:    "S1",
			PoolSize: 5,
		},
	}

	dateWiseBucket := make(map[string][]EngineStock)
	day1, _ := time.Parse("2006-01-02", "2021-01-01")

	// Stock A is further below SMA (DiffSMA 11.11%), but further from S1 (distance 11.11%)
	// Stock B is closer to SMA (DiffSMA 5.26%), but closer to S1 (distance 1.05%)
	dateWiseBucket["2021-01-01"] = []EngineStock{
		{
			Symbol:  "A.NS",
			SMA20:   100.0,
			DiffSMA: 11.11,
			EngineBar: EngineBar{
				Close: 90.0,
				Date:  day1,
			},
			IsConstituent: true,
			ClassicS1:     80.0, // distance = (90-80)/90 = 11.11%
		},
		{
			Symbol:  "B.NS",
			SMA20:   100.0,
			DiffSMA: 5.26,
			EngineBar: EngineBar{
				Close: 95.0,
				Date:  day1,
			},
			IsConstituent: true,
			ClassicS1:     94.0, // distance = (95-94)/95 = 1.05%
		},
	}

	account, strategy := RunNiftyShop(dateWiseBucket, cfg)
	if account == nil || strategy == nil {
		t.Fatalf("RunNiftyShop returned nil")
	}

	// Verify only B.NS was entered, not A.NS
	var enteredA, enteredB bool
	for _, trade := range strategy.History {
		if trade.Action == portfolio.Fresh {
			if trade.Symbol == "A.NS" {
				enteredA = true
			}
			if trade.Symbol == "B.NS" {
				enteredB = true
			}
		}
	}

	if enteredA {
		t.Errorf("Expected A.NS to NOT be entered because B.NS was closer to its S1 support level")
	}
	if !enteredB {
		t.Errorf("Expected B.NS to be entered because it was closer to its S1 support level")
	}
}
