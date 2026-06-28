package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAllStock(t *testing.T) {
	// Create a temp directory for test CSV files
	tmpDir, err := os.MkdirTemp("", "test_stocks")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a mock CSV
	mockCSVContent := `Date,Open,High,Low,Close,AdjClose,Volume
2026-01-01,100.0,105.0,99.0,102.0,102.0,1000
2026-01-02,102.0,108.0,101.0,107.0,107.0,1500
`
	tmpFile := filepath.Join(tmpDir, "TEST_2026-01-01_to_2026-01-02.csv")
	err = os.WriteFile(tmpFile, []byte(mockCSVContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock CSV: %v", err)
	}

	stockList, err := GetAllStock(tmpDir)
	if err != nil {
		t.Fatalf("GetAllStock failed: %v", err)
	}

	bars, exists := stockList["TEST"]
	if !exists {
		t.Errorf("Expected 'TEST' symbol to be parsed")
	}
	if len(bars) != 2 {
		t.Errorf("Expected 2 bars, got %d", len(bars))
	}
	if bars[0].Close != 102.0 || bars[1].Close != 107.0 {
		t.Errorf("Bars close values not matched")
	}
}

func TestLoadHistoricalConstituents(t *testing.T) {
	// Create a mock weights CSV
	mockWeightsContent := `DATE,RELIANCE,TCS,YESBANK
2021-01-31,10.5,5.2,0
2021-02-28,10.0,5.0,2.1
`
	tmpDir, err := os.MkdirTemp("", "test_weights")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "weights.csv")
	if err := os.WriteFile(tmpFile, []byte(mockWeightsContent), 0644); err != nil {
		t.Fatalf("Failed to write mock weights CSV: %v", err)
	}

	monthMap, uniqueTickers, err := LoadHistoricalConstituents(tmpFile)
	if err != nil {
		t.Fatalf("LoadHistoricalConstituents failed: %v", err)
	}

	// Verify unique list
	expectedUnique := map[string]bool{
		"RELIANCE.NS": true,
		"TCS.NS":      true,
		"YESBANK.NS":  true,
	}
	if len(uniqueTickers) != len(expectedUnique) {
		t.Errorf("Expected %d unique tickers, got %d", len(expectedUnique), len(uniqueTickers))
	}
	for _, ticker := range uniqueTickers {
		if !expectedUnique[ticker] {
			t.Errorf("Unexpected unique ticker: %s", ticker)
		}
	}

	// Verify month map
	janMap, exists := monthMap["2021-01"]
	if !exists {
		t.Fatalf("Expected '2021-01' key in monthMap")
	}
	if !janMap["RELIANCE.NS"] || !janMap["TCS.NS"] {
		t.Errorf("Expected RELIANCE.NS and TCS.NS to be active in Jan")
	}
	if janMap["YESBANK.NS"] {
		t.Errorf("Expected YESBANK.NS to be inactive in Jan")
	}

	febMap, exists := monthMap["2021-02"]
	if !exists {
		t.Fatalf("Expected '2021-02' key in monthMap")
	}
	if !febMap["RELIANCE.NS"] || !febMap["TCS.NS"] || !febMap["YESBANK.NS"] {
		t.Errorf("Expected all tickers to be active in Feb")
	}
}

func TestLoadHistoricalConstituentsNewIndices(t *testing.T) {
	// Test Midcap 50
	monthMap, symbols, err := LoadHistoricalConstituents("niftymidcap50_weights.csv")
	// Wait, we need the correct path: since the test runs from the package internal/data directory, the path should be compared relative to that.
	// But let's check: LoadHistoricalConstituents takes a path relative to the test runner's CWD (which is internal/data).
	// But wait! When Go tests run, CWD is the package directory (internal/data). But in cmd/backtester, the path is internal/data/niftymidcap50_weights.csv.
	// So relative to internal/data, the path is "niftymidcap50_weights.csv" or "niftysmallcap50_weights.csv".
	// Let's use the local file names or check if they exist.
	monthMap, symbols, err = LoadHistoricalConstituents("niftymidcap50_weights.csv")
	if err != nil {
		t.Fatalf("Failed to load niftymidcap50_weights.csv: %v", err)
	}
	if len(symbols) == 0 {
		t.Error("Loaded symbols list is empty")
	}
	if len(monthMap) == 0 {
		t.Error("Loaded monthMap is empty")
	}

	// Test Smallcap 50
	_, _, err = LoadHistoricalConstituents("niftysmallcap50_weights.csv")
	if err != nil {
		t.Fatalf("Failed to load niftysmallcap50_weights.csv: %v", err)
	}

	// Test Momentum 50
	_, _, err = LoadHistoricalConstituents("nifty500momentum50_weights.csv")
	if err != nil {
		t.Fatalf("Failed to load nifty500momentum50_weights.csv: %v", err)
	}
}


