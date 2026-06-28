package portfolio

import (
	"fmt"
	"time"
)

type ActionOption int

const (
	Fresh ActionOption = iota
	Avg
	Sell
)

func (a ActionOption) String() string {
	return [...]string{"Fresh", "Avg", "Sell"}[a]
}

type Trade struct {
	Symbol    string
	Date      time.Time
	Lot       int32
	Price     float64
	Action    ActionOption
	ID        string
	BuyPrice  float64
	PnL       float64
	CashAfter float64
	SlotSize  float64
}

type Account struct {
	Cash      float64
	Positions []Trade
}

func NewAccount(initialCash float64) *Account {
	return &Account{
		Cash:      initialCash,
		Positions: make([]Trade, 0),
	}
}

func (a *Account) GetCapital() float64 {
	return a.Cash
}

func (a *Account) GetCurrentHoldings() []Trade {
	return a.Positions
}

func (a *Account) SetCapital(cash float64) {
	a.Cash = cash
}

func (a *Account) AddPosition(p Trade) {
	a.Positions = append(a.Positions, p)
}

func (a *Account) UniqueStocksHeld() int {
	unique := make(map[string]bool)
	for _, p := range a.Positions {
		unique[p.Symbol] = true
	}
	return len(unique)
}

func (a *Account) HasPosition(symbol string) bool {
	for _, p := range a.Positions {
		if p.Symbol == symbol {
			return true
		}
	}
	return false
}

// RemovePosition exits a specific lot ID and updates cash
func (a *Account) RemovePosition(id string, price float64) {
	for i, position := range a.Positions {
		if position.ID == id {
			a.Positions = append(a.Positions[:i], a.Positions[i+1:]...)
			a.Cash += float64(position.Lot) * price // update capital
			fmt.Printf(">>> REMOVED Position: %v\n", position.Symbol)
			fmt.Printf(">>> Updated Cash: %v\n", a.Cash)
			break
		}
	}
}

type Strategy struct {
	Name           string
	FilteredStocks any
	History        []Trade
}

func NewStrategy(name string, filteredStocks any) *Strategy {
	return &Strategy{
		Name:           name,
		FilteredStocks: filteredStocks,
		History:        make([]Trade, 0),
	}
}

func (s *Strategy) AddTrade(trade Trade) {
	s.History = append(s.History, trade)
}
