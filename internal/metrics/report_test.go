package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishalvx/back-tester/internal/portfolio"
)

func makeTrade(symbol string, action portfolio.ActionOption, slot float64) portfolio.Trade {
	return portfolio.Trade{
		Symbol:    symbol,
		Date:      time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		Lot:       10,
		Price:     100.0,
		Action:    action,
		ID:        "test-id",
		BuyPrice:  100.0,
		PnL:       50.0,
		CashAfter: 9000.0,
		SlotSize:  slot,
	}
}

func TestExportTradeLog_ContainsSlotSizeColumn(t *testing.T) {
	// Set up a temp dir and redirect trade log output there
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	history := []portfolio.Trade{
		makeTrade("A.NS", portfolio.Fresh, 10000.0),
		makeTrade("B.NS", portfolio.Avg, 10500.0),
		makeTrade("C.NS", portfolio.Sell, 10500.0),
	}

	exportTradeLog(history, "test_universe")

	// Read the generated file
	data, err := os.ReadFile(filepath.Join(dir, "reports", "trade_logs_book_test_universe.csv"))
	if err != nil {
		t.Fatalf("trade log not created: %v", err)
	}

	content := string(data)

	// Header must include Slot Size
	if !contains(content, "Slot Size") {
		t.Errorf("expected CSV header to contain 'Slot Size', got:\n%s", content[:min(len(content), 200)])
	}

	// Each row should have slot size value
	if !contains(content, "10000.00") {
		t.Errorf("expected slot size 10000.00 in CSV rows")
	}
	if !contains(content, "10500.00") {
		t.Errorf("expected slot size 10500.00 in CSV rows")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
