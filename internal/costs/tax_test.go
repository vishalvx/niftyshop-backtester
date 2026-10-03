package costs

import (
	"testing"
	"time"
)

// All expected rupee amounts are worked out by hand from the rules in research/notes/tax-and-charges-verified.md.
// Effective rates (rate x (1 + cess)):
//   STCG  to 2008-03-31: 10% x 1.03 = 10.3%      from 2008-04-01 to 2018-03-31: 15% x 1.03 = 15.45%
//   STCG  2018-04-01 to 2024-07-22: 15% x 1.04 = 15.6%      from 2024-07-23: 20% x 1.04 = 20.8%
//   LTCG  exempt to 2018-03-31; 10% x 1.04 = 10.4% above Rs 1 lakh to 2024-07-22; 12.5% x 1.04 = 13% above Rs 1.25 lakh after.

const rupeeTol = 1e-6

// sale builds a single-share lot (so Cost and Proceeds are the totals).
func sale(buy, sell time.Time, cost, proceeds float64) Sale {
	return Sale{Symbol: "XYZ", BuyDate: buy, SellDate: sell, Qty: 1, Cost: cost, Proceeds: proceeds}
}

// settleOne records the sales and settles the financial year of the first sale.
func settleOne(mode TaxMode, sales ...Sale) float64 {
	tb := NewTaxBook(mode)
	for _, s := range sales {
		tb.RecordSale(s)
	}
	return tb.SettleFY(fyOf(sales[0].SellDate))
}

func TestTax_SingleShortTermSale(t *testing.T) {
	// A 3-month holding with a gain of Rs 50,000 (cost 100,000, proceeds 150,000), unless noted.
	tests := []struct {
		name         string
		mode         TaxMode
		sell         time.Time
		cost, procds float64
		want         float64
	}{
		{"before 2008-04-01: 10.3%", Dated, d(2007, 9, 1), 100000, 150000, 5150},
		{"FY2017-18: 15.45% (3% cess)", Dated, d(2017, 9, 1), 100000, 150000, 7725},
		{"last day with 3% cess, 2018-03-31", Dated, d(2018, 3, 31), 100000, 150000, 7725},
		{"2018-04-01: 15.6% (4% cess)", Dated, d(2018, 4, 1), 100000, 150000, 7800},
		{"2023: 15.6%", Dated, d(2023, 9, 1), 100000, 150000, 7800},
		{"2024-07-22 is the last day of 15.6%", Dated, d(2024, 7, 22), 100000, 150000, 7800},
		{"2024-07-23: 20.8%", Dated, d(2024, 7, 23), 100000, 150000, 10400},
		{"2025: 20.8%", Dated, d(2025, 1, 15), 100000, 150000, 10400},
		{"Today mode applies 20.8% to an old sale", Today, d(2017, 9, 1), 100000, 150000, 10400},
		{"loss pays nothing", Dated, d(2023, 9, 1), 100000, 60000, 0},
		{"break-even pays nothing", Dated, d(2023, 9, 1), 100000, 100000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buy := tc.sell.AddDate(0, -3, 0)
			near(t, "tax", settleOne(tc.mode, sale(buy, tc.sell, tc.cost, tc.procds)), tc.want, rupeeTol)
		})
	}
}

func TestTax_SingleLongTermSale(t *testing.T) {
	// A 2-year holding. No FMV lookup is set, so there is no grandfathering here.
	tests := []struct {
		name string
		mode TaxMode
		sell time.Time
		gain float64
		want float64
	}{
		{"before 2018-04-01 exempt", Dated, d(2017, 6, 1), 500000, 0},
		{"2018-03-31 is the last exempt day", Dated, d(2018, 3, 31), 500000, 0},
		{"2018-04-01: 10% above 1 lakh: (300000-100000) x 10.4%", Dated, d(2018, 4, 1), 300000, 20800},
		{"2021: same rule", Dated, d(2021, 3, 1), 300000, 20800},
		{"2024-07-22 still 10%", Dated, d(2024, 7, 22), 300000, 20800},
		{"2024-07-23: 12.5% above 1.25 lakh: (300000-125000) x 13%", Dated, d(2024, 7, 23), 300000, 22750},
		{"2025: same rule", Dated, d(2025, 2, 1), 300000, 22750},
		{"gain equal to the old exemption pays nothing", Dated, d(2021, 3, 1), 100000, 0},
		{"Rs 1 above the old exemption", Dated, d(2021, 3, 1), 100001, 0.104},
		{"gain equal to the new exemption pays nothing", Dated, d(2025, 2, 1), 125000, 0},
		{"Rs 1 above the new exemption", Dated, d(2025, 2, 1), 125001, 0.13},
		{"gain of 1.1 lakh is above the old exemption but below the new one", Dated, d(2025, 2, 1), 110000, 0},
		{"Today mode taxes an exempt-era sale at the new rule", Today, d(2017, 6, 1), 300000, 22750},
		{"long-term loss pays nothing", Dated, d(2021, 3, 1), -50000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buy := tc.sell.AddDate(-2, 0, 0)
			near(t, "tax", settleOne(tc.mode, sale(buy, tc.sell, 100000, 100000+tc.gain)), tc.want, rupeeTol)
		})
	}
}

func TestTax_HoldingPeriodBoundaryDecidesShortVersusLong(t *testing.T) {
	// Sold on the one-year anniversary: still short term (20.8% on 50,000 = 10,400, no exemption).
	// One day later: long term, gain 50,000 is under the 1.25 lakh exemption so nothing is due.
	buy := d(2023, 8, 1)
	near(t, "exactly 12 months", settleOne(Dated, sale(buy, d(2024, 8, 1), 100000, 150000)), 10400, rupeeTol)
	near(t, "12 months and a day", settleOne(Dated, sale(buy, d(2024, 8, 2), 100000, 150000)), 0, rupeeTol)
}

func TestTax_ShortAndLongGainsAreTaxedSeparately(t *testing.T) {
	// FY2023-24: short-term gain 50,000 -> 7,800; long-term gain 300,000 -> (300000-100000) x 10.4% = 20,800.
	got := settleOne(Dated,
		sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 150000),
		sale(d(2020, 1, 1), d(2023, 10, 1), 100000, 400000),
	)
	near(t, "tax", got, 28600, rupeeTol)
}

func TestTax_LongTermExemptionIsPerFinancialYear(t *testing.T) {
	t.Run("two long-term sales in one year share one exemption", func(t *testing.T) {
		// 80,000 + 80,000 = 160,000; (160000-100000) x 10.4% = 6,240. Each alone would have been exempt.
		got := settleOne(Dated,
			sale(d(2019, 1, 1), d(2022, 6, 1), 100000, 180000),
			sale(d(2019, 1, 1), d(2022, 9, 1), 100000, 180000),
		)
		near(t, "tax", got, 6240, rupeeTol)
	})
	t.Run("each financial year gets its own exemption", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(d(2019, 1, 1), d(2021, 2, 1), 100000, 200000)) // FY2021
		tb.RecordSale(sale(d(2019, 1, 1), d(2021, 6, 1), 100000, 200000)) // FY2022
		near(t, "FY2021", tb.SettleFY(2021), 0, rupeeTol)
		near(t, "FY2022", tb.SettleFY(2022), 0, rupeeTol)
		near(t, "TotalPaid", tb.TotalPaid, 0, rupeeTol)
	})
}

func TestTax_SingleExemptionAcrossThe23July2024Boundary(t *testing.T) {
	// FY2024-25: one long-term sale on 2024-06-10 (old rules, 10.4% effective) and one on 2024-09-10 (new, 13%),
	// each with a gain of 150,000. The Rs 1.25 lakh limit applies once "on aggregate", not 1 lakh + 1.25 lakh.
	// The code applies the exemption to the higher-rate (13%) gains first:
	//   13% bucket: 150,000 - 125,000 = 25,000 -> 3,250;  10.4% bucket: 150,000 -> 15,600;  total 18,850.
	// Using the exemption on the 10.4% bucket first would give 22,100; granting 1 lakh and 1.25 lakh separately would
	// give 15,600 + 19,500 - 10,400 - 16,250 = 8,450. Only a single Rs 1.25 lakh limit gives 18,850.
	before := sale(d(2021, 1, 1), d(2024, 6, 10), 100000, 250000)
	after := sale(d(2021, 1, 1), d(2024, 9, 10), 100000, 250000)
	t.Run("recorded before then after", func(t *testing.T) {
		near(t, "tax", settleOne(Dated, before, after), 18850, rupeeTol)
	})
	t.Run("recorded in the opposite order gives the same tax", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(after)
		tb.RecordSale(before)
		near(t, "tax", tb.SettleFY(2025), 18850, rupeeTol)
	})
	t.Run("only pre-boundary sales in the year keep the 1 lakh exemption", func(t *testing.T) {
		// (300000 - 100000) x 10.4%.
		near(t, "tax", settleOne(Dated, sale(d(2021, 1, 1), d(2024, 6, 10), 100000, 400000)), 20800, rupeeTol)
	})
	t.Run("only post-boundary sales get the 1.25 lakh exemption", func(t *testing.T) {
		// (300000 - 125000) x 13%.
		near(t, "tax", settleOne(Dated, sale(d(2021, 1, 1), d(2024, 9, 10), 100000, 400000)), 22750, rupeeTol)
	})
}

func TestTax_FinancialYearBoundaryIs31MarchAnd1April(t *testing.T) {
	tb := NewTaxBook(Dated)
	tb.RecordSale(sale(d(2023, 6, 1), d(2024, 3, 31), 100000, 150000)) // FY2024
	tb.RecordSale(sale(d(2023, 6, 1), d(2024, 4, 1), 100000, 150000))  // FY2025, 15.6% era still applies on 1 Apr 2024
	near(t, "FY2024", tb.SettleFY(2024), 7800, rupeeTol)
	near(t, "FY2025", tb.SettleFY(2025), 7800, rupeeTol)
	near(t, "TotalPaid", tb.TotalPaid, 15600, rupeeTol)
	near(t, "settling again charges nothing", tb.SettleFY(2024), 0, rupeeTol)
}

func TestTax_LossSetOff(t *testing.T) {
	t.Run("short-term loss in the same year reduces short-term gain", func(t *testing.T) {
		// (100000 - 30000) x 15.6% = 10,920.
		got := settleOne(Dated,
			sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 200000),
			sale(d(2023, 6, 1), d(2023, 10, 1), 100000, 70000),
		)
		near(t, "tax", got, 10920, rupeeTol)
	})

	t.Run("short-term loss sets off against long-term gain", func(t *testing.T) {
		// ST loss 40,000; LT gain 300,000 -> 260,000; less the Rs 1 lakh exemption = 160,000 x 10.4% = 16,640.
		got := settleOne(Dated,
			sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 60000),
			sale(d(2020, 1, 1), d(2023, 10, 1), 100000, 400000),
		)
		near(t, "tax", got, 16640, rupeeTol)
	})

	t.Run("long-term loss does not reduce a short-term gain", func(t *testing.T) {
		// ST gain 50,000 -> 7,800 regardless of the 100,000 long-term loss.
		got := settleOne(Dated,
			sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 150000),
			sale(d(2020, 1, 1), d(2023, 10, 1), 200000, 100000),
		)
		near(t, "tax", got, 7800, rupeeTol)
	})

	t.Run("a year with only losses pays nothing", func(t *testing.T) {
		near(t, "tax", settleOne(Dated,
			sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 60000),
			sale(d(2020, 1, 1), d(2023, 10, 1), 200000, 100000),
		), 0, rupeeTol)
	})
}

func TestTax_CarryForward(t *testing.T) {
	t.Run("short-term loss beyond this year's long-term gain is carried and used next year", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		// FY2023: ST loss 400,000 absorbs the whole 300,000 LT gain; 100,000 is left over.
		tb.RecordSale(sale(d(2022, 6, 1), d(2022, 9, 1), 500000, 100000))
		tb.RecordSale(sale(d(2019, 1, 1), d(2022, 10, 1), 100000, 400000))
		near(t, "FY2023", tb.SettleFY(2023), 0, rupeeTol)
		// FY2024: ST gain 150,000 less carried 100,000 = 50,000 x 15.6% = 7,800.
		tb.RecordSale(sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 250000))
		near(t, "FY2024", tb.SettleFY(2024), 7800, rupeeTol)
		near(t, "TotalPaid", tb.TotalPaid, 7800, rupeeTol)
	})

	t.Run("short-term loss carried into a year with only a long-term gain", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(d(2022, 6, 1), d(2022, 9, 1), 100000, 60000)) // ST loss 40,000 in FY2023
		near(t, "FY2023", tb.SettleFY(2023), 0, rupeeTol)
		// FY2024 LT gain 300,000 - 40,000 = 260,000; less 100,000 exempt = 160,000 x 10.4% = 16,640.
		tb.RecordSale(sale(d(2020, 1, 1), d(2023, 10, 1), 100000, 400000))
		near(t, "FY2024", tb.SettleFY(2024), 16640, rupeeTol)
	})

	t.Run("long-term loss is carried across the 2024 regime change", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(d(2020, 1, 1), d(2023, 10, 1), 200000, 100000)) // LT loss 100,000 in FY2024
		tb.RecordSale(sale(d(2023, 6, 1), d(2023, 9, 1), 100000, 150000))  // ST gain 50,000: 7,800, loss not usable here
		near(t, "FY2024", tb.SettleFY(2024), 7800, rupeeTol)
		// FY2025: LT gain 400,000 - 100,000 = 300,000; less 125,000 = 175,000 x 13% = 22,750.
		tb.RecordSale(sale(d(2021, 1, 1), d(2025, 2, 1), 100000, 500000))
		near(t, "FY2025", tb.SettleFY(2025), 22750, rupeeTol)
	})

	t.Run("a partly used long-term carry keeps its remainder for later years", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(d(2019, 1, 1), d(2022, 10, 1), 200000, 100000)) // LT loss 100,000 in FY2023
		near(t, "FY2023", tb.SettleFY(2023), 0, rupeeTol)
		// FY2024: LT gain 30,000 is wiped out by the carry; 70,000 remains.
		tb.RecordSale(sale(d(2019, 1, 1), d(2023, 10, 1), 100000, 130000))
		near(t, "FY2024", tb.SettleFY(2024), 0, rupeeTol)
		// FY2025: LT gain 270,000 - 70,000 = 200,000; less 125,000 = 75,000 x 13% = 9,750.
		tb.RecordSale(sale(d(2021, 1, 1), d(2025, 2, 1), 100000, 370000))
		near(t, "FY2025", tb.SettleFY(2025), 9750, rupeeTol)
	})

	t.Run("a loss expires after eight assessment years", func(t *testing.T) {
		// ST loss 50,000 in FY2013. Used in FY2021 (8 years later), gone by FY2022 (9 years later).
		tests := []struct {
			name string
			sell time.Time
			fy   int
			want float64
		}{
			{"eighth year after still absorbs", d(2020, 9, 1), 2021, 7800}, // (100000-50000) x 15.6%
			{"ninth year after has expired", d(2021, 9, 1), 2022, 15600},   // 100000 x 15.6%
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				tb := NewTaxBook(Dated)
				tb.RecordSale(sale(d(2012, 6, 1), d(2012, 9, 1), 100000, 50000))
				tb.SettleFY(2013)
				tb.RecordSale(sale(tc.sell.AddDate(0, -3, 0), tc.sell, 100000, 200000))
				near(t, "tax", tb.SettleFY(tc.fy), tc.want, rupeeTol)
			})
		}
	})
}

// Known bug: a loss realised before the 23 July 2024 rate change and a gain realised after it land in different rate
// buckets inside taxFor. The bucket with the negative amount is neither netted against the other bucket nor carried
// forward: it is silently dropped, so the whole gain is taxed. The net gain of the year is what the law taxes.
func TestTax_FY2024_25_LossAndGainOnOppositeSidesOfTheRateChange(t *testing.T) {
	tests := []struct {
		name  string
		sales []Sale
		want  float64
	}{
		{
			// ST loss 50,000 (Jun 2024, 15.6% bucket), ST gain 100,000 (Aug 2024, 20.8% bucket).
			// Net ST gain 50,000, all of it at 20.8% = 10,400. The code returns 20,800.
			name: "short-term loss before, short-term gain after",
			sales: []Sale{
				sale(d(2024, 4, 15), d(2024, 6, 10), 100000, 50000),
				sale(d(2024, 4, 15), d(2024, 8, 1), 100000, 200000),
			},
			want: 10400,
		},
		{
			// ST gain 100,000 (Jun 2024, 15.6%), ST loss 50,000 (Aug 2024, 20.8% bucket).
			// Net ST gain 50,000, all of it at 15.6% = 7,800. The code returns 15,600.
			name: "short-term gain before, short-term loss after",
			sales: []Sale{
				sale(d(2024, 4, 15), d(2024, 6, 10), 100000, 200000),
				sale(d(2024, 4, 15), d(2024, 8, 1), 100000, 50000),
			},
			want: 7800,
		},
		{
			// LT loss 50,000 (Jun 2024, 10.4% bucket), LT gain 200,000 (Sep 2024, 13% bucket).
			// Net LT gain 150,000; less the single Rs 1.25 lakh exemption = 25,000 x 13% = 3,250.
			// The code returns (200,000 - 125,000) x 13% = 9,750.
			name: "long-term loss before, long-term gain after",
			sales: []Sale{
				sale(d(2021, 1, 1), d(2024, 6, 10), 100000, 50000),
				sale(d(2021, 1, 1), d(2024, 9, 10), 100000, 300000),
			},
			want: 3250,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			near(t, "tax", settleOne(Dated, tc.sales...), tc.want, rupeeTol)
		})
	}
}

func fmvOf(prices map[string]float64) func(string) (float64, bool) {
	return func(sym string) (float64, bool) {
		v, ok := prices[sym]
		return v, ok
	}
}

func TestTax_GrandfatheringGain(t *testing.T) {
	// 1000 shares bought at 100 (cost 100,000); FMV on 31 Jan 2018 is 150 for AAA.
	// Cost becomes max(actual cost, min(FMV, sale price)) x qty, only for lots bought before 2018-02-01
	// and long-term sales on or after 2018-04-01, in Dated mode.
	lot := func(buy, sell time.Time, proceeds float64) Sale {
		return Sale{Symbol: "AAA", BuyDate: buy, SellDate: sell, Qty: 1000, Cost: 100000, Proceeds: proceeds}
	}
	old := d(2016, 1, 1)
	tests := []struct {
		name     string
		mode     TaxMode
		fmv      func(string) (float64, bool)
		sale     Sale
		wantGain float64
		wantLong bool
	}{
		{"stepped up to FMV: 300000 - 150000", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2019, 6, 1), 300000), 150000, true},
		{"sale price below FMV: cost = sale price, gain 0", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2019, 6, 1), 120000), 0, true},
		{"sale below original cost: real loss is kept", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2019, 6, 1), 90000), -10000, true},
		{"FMV below original cost: no step-up", Dated, fmvOf(map[string]float64{"AAA": 80}), lot(old, d(2019, 6, 1), 300000), 200000, true},
		{"sold 2018-03-31 (still exempt era): raw gain", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2018, 3, 31), 300000), 200000, true},
		{"sold 2018-04-01: step-up applies", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2018, 4, 1), 300000), 150000, true},
		{"bought 2018-01-31: step-up applies", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(d(2018, 1, 31), d(2019, 6, 1), 300000), 150000, true},
		{"bought 2018-02-01: no step-up", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(d(2018, 2, 1), d(2019, 6, 1), 300000), 200000, true},
		{"short-term sale: no step-up", Dated, fmvOf(map[string]float64{"AAA": 150}), lot(d(2018, 1, 15), d(2018, 6, 1), 300000), 200000, false},
		{"Today mode never grandfathers", Today, fmvOf(map[string]float64{"AAA": 150}), lot(old, d(2019, 6, 1), 300000), 200000, true},
		{"unknown symbol: no step-up", Dated, fmvOf(map[string]float64{"BBB": 150}), lot(old, d(2019, 6, 1), 300000), 200000, true},
		{"no FMV source: no step-up", Dated, nil, lot(old, d(2019, 6, 1), 300000), 200000, true},
		{"zero quantity cannot divide by zero", Dated, fmvOf(map[string]float64{"AAA": 150}), Sale{Symbol: "AAA", BuyDate: old, SellDate: d(2019, 6, 1), Qty: 0, Cost: 100000, Proceeds: 300000}, 200000, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTaxBook(tc.mode)
			tb.FMV31Jan2018 = tc.fmv
			g, long := tb.gain(tc.sale)
			near(t, "gain", g, tc.wantGain, rupeeTol)
			if long != tc.wantLong {
				t.Errorf("long = %v, want %v", long, tc.wantLong)
			}
		})
	}

	t.Run("end to end: step-up lowers the tax", func(t *testing.T) {
		s := lot(old, d(2019, 6, 1), 300000)
		// Without grandfathering: (200000 - 100000) x 10.4% = 10,400. With it: (150000 - 100000) x 10.4% = 5,200.
		tb := NewTaxBook(Dated)
		tb.FMV31Jan2018 = fmvOf(map[string]float64{"AAA": 150})
		tb.RecordSale(s)
		near(t, "with FMV", tb.SettleFY(2020), 5200, rupeeTol)
		plain := NewTaxBook(Dated)
		plain.RecordSale(s)
		near(t, "without FMV", plain.SettleFY(2020), 10400, rupeeTol)
	})
}

func TestTax_Dividends(t *testing.T) {
	const rate = 0.30
	tests := []struct {
		name string
		mode TaxMode
		rate float64
		date time.Time
		fy   int
		want float64
	}{
		{"Dated: 2020-03-31 (FY2019-20) is tax-free", Dated, rate, d(2020, 3, 31), 2020, 0},
		{"Dated: 2020-04-01 (FY2020-21) is taxed", Dated, rate, d(2020, 4, 1), 2021, 3000},
		{"Dated: later years are taxed", Dated, rate, d(2024, 1, 15), 2024, 3000},
		{"Dated: pre-2020 never taxed", Dated, rate, d(2012, 6, 1), 2013, 0},
		{"Today: always taxed, even for an old dividend", Today, rate, d(2015, 6, 1), 2016, 3000},
		{"zero rate disables the tax", Dated, 0, d(2024, 1, 15), 2024, 0},
		{"zero rate disables the tax in Today mode", Today, 0, d(2024, 1, 15), 2024, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTaxBook(tc.mode)
			tb.DividendTaxRate = tc.rate
			tb.RecordDividend(tc.date, 10000)
			near(t, "tax", tb.SettleFY(tc.fy), tc.want, rupeeTol)
		})
	}

	t.Run("dividends in one year add up, and sit on top of capital-gains tax", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.DividendTaxRate = rate
		tb.RecordDividend(d(2022, 5, 1), 4000)
		tb.RecordDividend(d(2022, 11, 1), 6000)
		tb.RecordSale(sale(d(2022, 1, 1), d(2022, 6, 1), 100000, 150000)) // ST gain 50,000 x 15.6% = 7,800
		// 10,000 x 30% = 3,000 on top.
		near(t, "FY2023", tb.SettleFY(2023), 10800, rupeeTol)
		near(t, "TotalPaid", tb.TotalPaid, 10800, rupeeTol)
	})

	t.Run("the 31 March / 1 April boundary of the dividend start", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.DividendTaxRate = rate
		tb.RecordDividend(d(2020, 3, 31), 10000)
		tb.RecordDividend(d(2020, 4, 1), 10000)
		near(t, "FY2020", tb.SettleFY(2020), 0, rupeeTol)
		near(t, "FY2021", tb.SettleFY(2021), 3000, rupeeTol)
	})
}

func TestTerminalTax(t *testing.T) {
	buy := d(2021, 1, 1)
	t.Run("is the increment on top of the sales already recorded this year", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		recorded := sale(buy, d(2025, 1, 10), 100000, 200000) // LT gain 100,000: inside the 1.25 lakh exemption
		tb.RecordSale(recorded)
		open := []Sale{sale(buy, d(2025, 3, 31), 100000, 200000)} // another 100,000 if sold at year end
		// With both: 200,000 - 125,000 = 75,000 x 13% = 9,750; recorded alone: 0.
		near(t, "TerminalTax", tb.TerminalTax(open), 9750, rupeeTol)
	})

	t.Run("does not change the book", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.DividendTaxRate = 0.3
		tb.RecordDividend(d(2025, 1, 5), 1000)
		tb.RecordSale(sale(buy, d(2025, 1, 10), 100000, 200000))
		open := []Sale{sale(buy, d(2025, 3, 31), 100000, 200000)}

		first := tb.TerminalTax(open)
		second := tb.TerminalTax(open)
		near(t, "idempotent", second, first, rupeeTol)
		if len(tb.sales[2025]) != 1 {
			t.Errorf("recorded sales for FY2025 = %d, want 1 (open lots must not be added)", len(tb.sales[2025]))
		}
		if tb.TotalPaid != 0 {
			t.Errorf("TotalPaid = %v, want 0", tb.TotalPaid)
		}
		if len(tb.carryST) != 0 || len(tb.carryLT) != 0 {
			t.Errorf("carry state changed: ST %v LT %v", tb.carryST, tb.carryLT)
		}
		// Settling afterwards still sees only the recorded sale and the dividend: 0 + 1,000 x 30% = 300.
		near(t, "SettleFY after TerminalTax", tb.SettleFY(2025), 300, rupeeTol)
	})

	t.Run("brought-forward losses reduce it but are not consumed", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(d(2020, 1, 1), d(2023, 10, 1), 200000, 100000)) // LT loss 100,000 in FY2024
		near(t, "FY2024", tb.SettleFY(2024), 0, rupeeTol)
		open := []Sale{sale(buy, d(2025, 3, 31), 100000, 400000)} // LT gain 300,000
		// 300,000 - 100,000 carried = 200,000; less 125,000 = 75,000 x 13% = 9,750.
		near(t, "TerminalTax", tb.TerminalTax(open), 9750, rupeeTol)
		if len(tb.carryLT) != 1 || tb.carryLT[0].amt != 100000 || tb.carryLT[0].fy != 2024 {
			t.Fatalf("carryLT = %+v, want one entry {2024 100000}", tb.carryLT)
		}
		// Selling for real afterwards pays the same and then uses the carry up.
		tb.RecordSale(open[0])
		near(t, "real settlement", tb.SettleFY(2025), 9750, rupeeTol)
		if len(tb.carryLT) != 0 {
			t.Errorf("carryLT after use = %+v, want empty", tb.carryLT)
		}
	})

	t.Run("with nothing recorded it is the plain tax on the open lots", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		open := []Sale{sale(d(2025, 1, 1), d(2025, 3, 31), 100000, 200000)} // ST gain 100,000 x 20.8%
		near(t, "TerminalTax", tb.TerminalTax(open), 20800, rupeeTol)
	})

	t.Run("no open lots means no tax", func(t *testing.T) {
		tb := NewTaxBook(Dated)
		tb.RecordSale(sale(buy, d(2025, 1, 10), 100000, 900000))
		near(t, "TerminalTax", tb.TerminalTax(nil), 0, rupeeTol)
	})
}
