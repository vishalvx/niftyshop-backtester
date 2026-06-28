package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/vishalvx/back-tester/internal/config"
	"github.com/vishalvx/back-tester/internal/data"
	"github.com/vishalvx/back-tester/internal/engine"
	"github.com/vishalvx/back-tester/internal/indicators"
	"github.com/vishalvx/back-tester/internal/metrics"
)

func main() {
	universeFlag := flag.String("universe", "", "Override universe in config")
	startDateFlag := flag.String("start-date", "", "Override start date in config (YYYY-MM-DD)")
	endDateFlag := flag.String("end-date", "", "Override end date in config (YYYY-MM-DD)")
	flag.Parse()

	fmt.Println("Starting NiftyShop Backtester...")

	// 1. Load Config
	cfg, err := config.Load("internal/config/config.json")
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	fmt.Println("Config loaded successfully")

	// Apply CLI overrides if specified
	if *universeFlag != "" {
		cfg.Universe = *universeFlag
	}
	if *startDateFlag != "" {
		cfg.StartDate = *startDateFlag
	}
	if *endDateFlag != "" {
		cfg.EndDate = *endDateFlag
	}

	// Ensure data directories exist
	os.MkdirAll("internal/data/stocks", 0755)

	// 2. Fetch Universe Symbols
	universeName := cfg.Universe
	if universeName == "nifty50" {
		universeName = "NIFTY 50" // Exact name expected by NSE API
	}
	fmt.Printf("Fetching symbols for index: %s\n", universeName)
	
	var stocksList []string
	var monthConstituents map[string]map[string]bool
	var useRebalancing bool

	var weightsPath string
	var inceptionDate string

	switch strings.ToLower(universeName) {
	case "nifty 50", "nifty50":
		weightsPath = "internal/data/nifty50_weights.csv"
		inceptionDate = "2008-01-01" // Available data start
	case "nifty midcap 50", "niftymidcap50":
		weightsPath = "internal/data/niftymidcap50_weights.csv"
		inceptionDate = "2004-01-01"
	case "nifty smallcap 50", "niftysmallcap50":
		weightsPath = "internal/data/niftysmallcap50_weights.csv"
		inceptionDate = "2016-04-01"
	case "nifty500 momentum 50", "nifty500momentum50":
		weightsPath = "internal/data/nifty500momentum50_weights.csv"
		inceptionDate = "2024-06-04"
	}

	if weightsPath != "" {
		// Inception date gate
		if cfg.StartDate < inceptionDate {
			fmt.Printf("WARNING: Configured start_date (%s) is before the launch/inception of index %s (%s).\n", cfg.StartDate, universeName, inceptionDate)
			fmt.Printf("Automatically adjusting backtest start_date to %s.\n", inceptionDate)
			cfg.StartDate = inceptionDate
		}

		var err error
		monthConstituents, stocksList, err = data.LoadHistoricalConstituents(weightsPath)
		if err != nil {
			fmt.Printf("Error loading historical constituents from %s: %v. Falling back to default list.\n", weightsPath, err)
		} else {
			useRebalancing = true
			fmt.Printf("Loaded historical constituents from %s. Found %d unique symbols across history.\n", weightsPath, len(stocksList))
		}
	}

	if !useRebalancing {
		if universeName == "NIFTY 50" {
			stocksList = []string{"RELIANCE", "TCS", "HDFCBANK", "INFY", "ICICIBANK", "SBIN", "BHARTIARTL", "ITC"}
		}
		
		fetched, err := data.FetchIndexSymbols(universeName)
		if err == nil && len(fetched) > 0 {
			stocksList = fetched
			fmt.Printf("Successfully fetched %d symbols from NSE.\n", len(stocksList))
		} else {
			fmt.Println("Could not fetch from NSE (often blocked), using fallback list.")
		}
	}

	limit := len(stocksList)

	// 3. Load Data
	fmt.Printf("Downloading/Loading historical data (from %s to %s)...\n", cfg.StartDate, cfg.EndDate)
	for i := 0; i < limit; i++ {
		stockName := stocksList[i]
		if !strings.HasSuffix(stockName, ".NS") {
			stockName = stockName + ".NS"
		}
		if err := data.LoadData(stockName, cfg.StartDate, cfg.EndDate); err != nil {
			fmt.Printf("Failed to load %s: %v\n", stockName, err)
		} else {
			fmt.Printf("%s data loaded.\n", stockName)
		}
	}

	// 4. Precompute Indicators & Group by Date
	fmt.Println("Precomputing SMA indicators and aggregating daily buckets...")
	rawStockData, err := data.GetAllStock("internal/data/stocks")
	if err != nil {
		fmt.Printf("Failed to load local stock data: %v\n", err)
		return
	}

	dateWiseBucket := make(map[string][]engine.EngineStock)
	window := int(cfg.MAWindow)

	// Parse configured date range for bucket filtering
	cfgStart, err := time.Parse("2006-01-02", cfg.StartDate)
	if err != nil {
		fmt.Printf("Invalid start_date in config: %v\n", err)
		return
	}
	cfgEnd, err := time.Parse("2006-01-02", cfg.EndDate)
	if err != nil {
		fmt.Printf("Invalid end_date in config: %v\n", err)
		return
	}

	for symbol, bars := range rawStockData {
		closes := make([]float64, len(bars))
		for i, b := range bars {
			closes[i] = b.Close
		}

		smas := indicators.CalculateSMA(closes, window)

		for i, b := range bars {
			// Skip bars before we have a complete SMA window to avoid noise/zeros
			if i < window-1 {
				continue
			}

			smaVal := smas[i]
			diffSMA := ((smaVal - b.Close) / b.Close) * 100
			dateStr := b.Date.Format("2006-01-02")

			isConstituent := true
			if useRebalancing {
				isConstituent = false
				monthKey := b.Date.Format("2006-01")
				if constituents, exists := monthConstituents[monthKey]; exists {
					if constituents[symbol] {
						isConstituent = true
					}
				} else {
					// Fallback: find the closest previous month in monthConstituents
					latestKey := ""
					for k := range monthConstituents {
						if k <= monthKey {
							if latestKey == "" || k > latestKey {
								latestKey = k
							}
						}
					}
					if latestKey != "" && monthConstituents[latestKey][symbol] {
						isConstituent = true
					}
				}
			}

			engineStock := engine.EngineStock{
				EngineBar: engine.EngineBar{
					Open:   b.Open,
					Close:  b.Close,
					High:   b.High,
					Low:    b.Low,
					Volume: b.Volume,
					Date:   b.Date,
				},
				Symbol:        symbol,
				SMA20:         smaVal,
				DiffSMA:       diffSMA,
				IsConstituent: isConstituent,
			}

			// Skip bars outside the configured [StartDate, EndDate] window.
			// This prevents cached CSVs from earlier/later runs bleeding into results.
			if b.Date.Before(cfgStart) || b.Date.After(cfgEnd) {
				continue
			}

			dateWiseBucket[dateStr] = append(dateWiseBucket[dateStr], engineStock)
		}
	}

	// Sort daily stocks list by difference from SMA descending (most below SMA first)
	for date, stockList := range dateWiseBucket {
		sort.Slice(stockList, func(i, j int) bool {
			return stockList[i].DiffSMA > stockList[j].DiffSMA
		})
		dateWiseBucket[date] = stockList
	}

	// 5. Run Trading Engine
	fmt.Println("Running Engine...")
	account, strategy := engine.RunNiftyShop(dateWiseBucket, cfg)

	if account == nil || strategy == nil {
		fmt.Println("Engine did not process any trades.")
		return
	}

	// Extract the last known prices to calculate final portfolio values
	currentPrices := make(map[string]float64)
	var dates []string
	for d := range dateWiseBucket {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for _, date := range dates {
		stocks := dateWiseBucket[date]
		for _, s := range stocks {
			currentPrices[s.Symbol] = s.Close
		}
	}

	// 6. Report Metrics
	report := metrics.Generate(account, strategy, cfg, currentPrices)
	metrics.Print(report)
}
