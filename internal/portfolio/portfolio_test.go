package portfolio

import (
	"testing"
	"time"
)

func TestAccountPositions(t *testing.T) {
	acc := NewAccount(100000.0)

	if acc.GetCapital() != 100000.0 {
		t.Errorf("Expected initial cash to be 100000.0, got %f", acc.GetCapital())
	}

	trade1 := Trade{
		ID:       "id1",
		Symbol:   "RELIANCE",
		Lot:      10,
		Price:    2000.0,
		BuyPrice: 2000.0,
		Action:   Fresh,
		Date:     time.Now(),
	}

	acc.AddPosition(trade1)
	acc.SetCapital(acc.GetCapital() - 20000.0)

	if acc.GetCapital() != 80000.0 {
		t.Errorf("Expected cash to be 80000.0, got %f", acc.GetCapital())
	}

	if !acc.HasPosition("RELIANCE") {
		t.Errorf("Expected portfolio to have RELIANCE")
	}

	if acc.UniqueStocksHeld() != 1 {
		t.Errorf("Expected unique stocks held to be 1, got %d", acc.UniqueStocksHeld())
	}

	// Add an average buy lot of same stock
	trade2 := Trade{
		ID:       "id2",
		Symbol:   "RELIANCE",
		Lot:      10,
		Price:    1900.0,
		BuyPrice: 1900.0,
		Action:   Avg,
		Date:     time.Now(),
	}
	acc.AddPosition(trade2)
	acc.SetCapital(acc.GetCapital() - 19000.0)

	// LIFO exit: remove id2 first
	acc.RemovePosition("id2", 2100.0)

	// cash should be 80000 - 19000 + (10 * 2100) = 82000.0
	expectedCash := 82000.0
	if acc.GetCapital() != expectedCash {
		t.Errorf("Expected cash after LIFO exit to be %f, got %f", expectedCash, acc.GetCapital())
	}

	if len(acc.GetCurrentHoldings()) != 1 || acc.GetCurrentHoldings()[0].ID != "id1" {
		t.Errorf("Expected only trade1 to remain in positions")
	}
}
