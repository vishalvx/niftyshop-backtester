package costs

import (
	"testing"
	"time"
)

var (
	_ Model = IndianDelivery{}
	_ Model = Zero{}
)

type sideKind int

const (
	buySide sideKind = iota
	sellSide
	sellFirstOfScrip // first sale of the scrip today: the flat DP charge applies
)

func chargeFor(m IndianDelivery, date time.Time, value float64, k sideKind) Charge {
	switch k {
	case buySide:
		return m.Buy(date, value)
	case sellSide:
		return m.Sell(date, value, false)
	default:
		return m.Sell(date, value, true)
	}
}

func TestIndianDelivery_Charges(t *testing.T) {
	// All values below were worked out by hand for a Rs 10 lakh (1e6) order and checked with a short script.
	//
	// 2025-01-15 (STT 0.1%, NSE 0.00307% incl. IPFT, SEBI 0.0001%, stamp 0.015%, GST 18%):
	//   STT 1000, exchange 30.7, SEBI 1.0, GST = 0.18*(brokerage + 30.7 + 1.0), stamp 150 (buy only), DP Rs 15.
	//   zero-brokerage buy:  1000 + 30.7 + 1 + 5.706 + 150 = 1187.406; deductible = total - STT = 187.406
	//   with 30 bps brokerage (Rs 3000): GST = 0.18*3031.7 = 545.706
	//
	// 2010-01-15 (STT 0.125%, NSE 0.00325%, SEBI 0.0002%, stamp 0.01%, service tax 10.3%):
	//   STT 1250, exchange 32.5, SEBI 2.0, tax = 0.103*34.5 = 3.5535, stamp 100.
	zero := IndianDelivery{Statutory: true}
	brk30 := IndianDelivery{BrokerageBps: 30, Statutory: true}
	jan25, jan10 := d(2025, 1, 15), d(2010, 1, 15)
	tests := []struct {
		name         string
		m            IndianDelivery
		date         time.Time
		value        float64
		side         sideKind
		total, deduc float64
	}{
		{"2025 buy, no brokerage", zero, jan25, 1e6, buySide, 1187.406, 187.406},
		{"2025 sell, no brokerage", zero, jan25, 1e6, sellSide, 1037.406, 37.406},
		{"2025 sell, first sale of scrip today adds Rs 15 DP", zero, jan25, 1e6, sellFirstOfScrip, 1052.406, 52.406},
		{"2025 buy, 30 bps brokerage", brk30, jan25, 1e6, buySide, 4727.406, 3727.406},
		{"2025 sell, 30 bps brokerage", brk30, jan25, 1e6, sellSide, 4577.406, 3577.406},
		{"2025 sell, 30 bps brokerage, first of scrip", brk30, jan25, 1e6, sellFirstOfScrip, 4592.406, 3592.406},
		{"2010 buy, no brokerage", zero, jan10, 1e6, buySide, 1388.0535, 138.0535},
		{"2010 sell, no brokerage", zero, jan10, 1e6, sellSide, 1288.0535, 38.0535},
		{"2010 sell, first of scrip", zero, jan10, 1e6, sellFirstOfScrip, 1303.0535, 53.0535},
		// Rs 1000 order at 30 bps = Rs 3, below the Rs 20 floor, so brokerage is Rs 20:
		// buy = 20 + 0.0307 + 0.001 + 0.18*20.0317 + 0.15 + 1 = 24.787406; sell drops the stamp duty.
		{"brokerage floor applies to small buy", IndianDelivery{BrokerageBps: 30, BrokerageMin: 20, Statutory: true}, jan25, 1000, buySide, 24.787406, 23.787406},
		{"brokerage floor applies to small sell", IndianDelivery{BrokerageBps: 30, BrokerageMin: 20, Statutory: true}, jan25, 1000, sellSide, 24.637406, 23.637406},
		{"floor is ignored when brokerage is zero bps", IndianDelivery{BrokerageMin: 20}, jan25, 1000, buySide, 0, 0},
		{"floor applies even without statutory charges", IndianDelivery{BrokerageBps: 30, BrokerageMin: 20}, jan25, 1000, buySide, 20, 20},
		{"no statutory: brokerage only, fully deductible", IndianDelivery{BrokerageBps: 10}, jan25, 1e6, buySide, 1000, 1000},
		{"no statutory: sell of first scrip has no DP charge", IndianDelivery{BrokerageBps: 10}, jan25, 1e6, sellFirstOfScrip, 1000, 1000},
		{"nothing at all", IndianDelivery{}, jan25, 1e6, sellFirstOfScrip, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := chargeFor(tc.m, tc.date, tc.value, tc.side)
			near(t, "Total", c.Total, tc.total, 1e-6)
			near(t, "Deductible", c.Deductible, tc.deduc, 1e-6)
		})
	}
}

func TestIndianDelivery_STTRateChangeOnBothSides(t *testing.T) {
	// Total - Deductible is exactly the STT (STT is not deductible). Rs 10 lakh order:
	// 0.125% = Rs 1250 until 2012-06-30, 0.1% = Rs 1000 from 2012-07-01.
	m := IndianDelivery{Statutory: true}
	tests := []struct {
		date time.Time
		stt  float64
	}{
		{d(2012, 6, 30), 1250},
		{d(2012, 7, 1), 1000},
	}
	for _, tc := range tests {
		for name, c := range map[string]Charge{
			"buy":  m.Buy(tc.date, 1e6),
			"sell": m.Sell(tc.date, 1e6, false),
		} {
			near(t, tc.date.Format("2006-01-02")+" "+name+" STT", c.Total-c.Deductible, tc.stt, 1e-9)
		}
	}
}

func TestIndianDelivery_StampDutyIsBuySideOnly(t *testing.T) {
	m := IndianDelivery{Statutory: true}
	tests := []struct {
		date  time.Time
		stamp float64 // Rs 10 lakh order
	}{
		{d(2010, 1, 15), 100}, // 0.01%
		{d(2020, 6, 30), 100}, // last day of the old rate
		{d(2020, 7, 1), 150},  // 0.015% uniform
		{d(2025, 1, 15), 150},
	}
	for _, tc := range tests {
		buy, sell := m.Buy(tc.date, 1e6), m.Sell(tc.date, 1e6, false)
		near(t, tc.date.Format("2006-01-02")+" buy-sell total gap", buy.Total-sell.Total, tc.stamp, 1e-9)
		near(t, tc.date.Format("2006-01-02")+" buy-sell deductible gap", buy.Deductible-sell.Deductible, tc.stamp, 1e-9)
	}
}

func TestIndianDelivery_DPChargeOnlyOnFirstSellOfScripToday(t *testing.T) {
	m := IndianDelivery{BrokerageBps: 30, Statutory: true}
	date := d(2025, 1, 15)
	later := m.Sell(date, 1e6, false)
	first := m.Sell(date, 1e6, true)
	near(t, "extra total", first.Total-later.Total, 15, 1e-9)
	near(t, "extra deductible (DP charge is deductible)", first.Deductible-later.Deductible, 15, 1e-9)
	// A buy never pays it: Buy has no flag, and its total is unchanged by any sell flag.
	near(t, "buy total", m.Buy(date, 1e6).Total, 4727.406, 1e-6)
}

func TestIndianDelivery_GSTBaseIsBrokerageExchangeAndSEBIOnly(t *testing.T) {
	// Adding Rs 3000 of brokerage must add exactly 3000 * (1 + 18%) = 3540 to the total and deductible:
	// GST is charged on the brokerage, and STT and stamp duty do not move.
	date := d(2025, 1, 15)
	base := IndianDelivery{Statutory: true}
	with := IndianDelivery{BrokerageBps: 30, Statutory: true}
	for name, pair := range map[string][2]Charge{
		"buy":  {base.Buy(date, 1e6), with.Buy(date, 1e6)},
		"sell": {base.Sell(date, 1e6, false), with.Sell(date, 1e6, false)},
	} {
		near(t, name+" total delta", pair[1].Total-pair[0].Total, 3540, 1e-9)
		near(t, name+" deductible delta", pair[1].Deductible-pair[0].Deductible, 3540, 1e-9)
	}

	// GST of 18% on exchange (30.7) + SEBI (1.0) = 5.706 is the only GST in a zero-brokerage buy:
	// total 1187.406 = STT 1000 + stamp 150 + exchange 30.7 + SEBI 1.0 + GST 5.706.
	// If GST were also charged on STT and stamp it would be 0.18*1181.7 = 212.706 instead.
	near(t, "zero-brokerage buy total", base.Buy(date, 1e6).Total, 1187.406, 1e-9)
}

func TestZeroModelIsFree(t *testing.T) {
	var m Model = Zero{}
	if c := m.Buy(d(2025, 1, 15), 1e6); c != (Charge{}) {
		t.Errorf("Zero.Buy = %+v, want zero", c)
	}
	if c := m.Sell(d(2025, 1, 15), 1e6, true); c != (Charge{}) {
		t.Errorf("Zero.Sell = %+v, want zero", c)
	}
}
