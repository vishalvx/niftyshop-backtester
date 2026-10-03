// Package costs models Indian equity delivery transaction costs and capital-gains tax.
//
// Cost and tax rate tables are dated; every row cites its authority in rates.go.
package costs

import "time"

// Charge is the cost of one fill. Deductible is the part an Indian taxpayer may deduct from the
// capital gain (brokerage, exchange/SEBI fees, stamp duty, GST, DP charges; STT is NOT deductible).
type Charge struct {
	Total      float64
	Deductible float64
}

// Model prices a fill. value is price*quantity in rupees. sellDay is true for the first sale of a scrip on a day
// (a flat DP charge applies once per scrip per sell day).
type Model interface {
	Buy(date time.Time, value float64) Charge
	Sell(date time.Time, value float64, firstSellOfScripToday bool) Charge
}

// Zero is the legacy engine's cost model: free trading.
type Zero struct{}

func (Zero) Buy(time.Time, float64) Charge        { return Charge{} }
func (Zero) Sell(time.Time, float64, bool) Charge { return Charge{} }
