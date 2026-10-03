package analytics

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

// Every expected number below is hard-coded. Where it is not trivial arithmetic it was produced by an independent
// numpy snippet (never by calling or re-deriving the production formulas), and the arithmetic is sketched in a
// comment next to the fixture.

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// series returns points on consecutive calendar days starting at start.
func series(start time.Time, vals ...float64) []Point {
	out := make([]Point, len(vals))
	for i, v := range vals {
		out[i] = Point{Date: start.AddDate(0, 0, i), Value: v}
	}
	return out
}

// compound turns a list of simple returns into an equity curve (fixture builder, not under test).
func compound(start float64, rets []float64) []float64 {
	out := make([]float64, len(rets))
	v := start
	for i, r := range rets {
		v *= 1 + r
		out[i] = v
	}
	return out
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.IsInf(want, 0) {
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
		return
	}
	tol := 1e-9 * math.Max(1, math.Abs(want))
	if math.IsNaN(got) || math.Abs(got-want) > tol {
		t.Errorf("%s = %.12g, want %.12g", name, got, want)
	}
}

func sameDay(t *testing.T, name string, got time.Time, base time.Time, idx int) {
	t.Helper()
	if idx < 0 {
		if !got.IsZero() {
			t.Errorf("%s = %v, want zero time", name, got)
		}
		return
	}
	if want := base.AddDate(0, 0, idx); !got.Equal(want) {
		t.Errorf("%s = %v, want %v (day %d)", name, got.Format("2006-01-02"), want.Format("2006-01-02"), idx)
	}
}

func TestCompute_TooShortReturnsZeroMetrics(t *testing.T) {
	for name, eq := range map[string][]Point{
		"nil":        nil,
		"one point":  series(day(2020, 1, 1), 100),
		"empty list": {},
	} {
		t.Run(name, func(t *testing.T) {
			m := Compute(Input{StartCapital: 100, Equity: eq, Rf: 0.06})
			if !reflect.DeepEqual(m, Metrics{}) {
				t.Errorf("Compute on %d points = %+v, want zero Metrics", len(eq), m)
			}
		})
	}
}

func TestCompute_ReturnAndCAGR(t *testing.T) {
	// 2020-01-01 to 2024-01-01 is 1461 days = exactly 4 x 365.25, so Years is exactly 4 and CAGR = ratio^(1/4) - 1.
	first, last := day(2020, 1, 1), day(2024, 1, 1)
	tests := []struct {
		name                       string
		startCap, eq0, end         float64
		wantStart, wantTotal, cagr float64
	}{
		{"gain 1.6x: 1.6^0.25-1", 100, 100, 160, 100, 0.6, 0.12468265038069815},
		{"double: 2^0.25-1", 100, 100, 200, 100, 1.0, 0.18920711500272103},
		{"flat", 100, 100, 100, 100, 0, 0},
		{"halved: 0.5^0.25-1", 100, 100, 50, 100, -0.5, -0.1591035847462855},
		{"wipe-out is -100 percent", 100, 100, 0, 100, -1, -1},
		{"zero start capital falls back to first point", 0, 100, 160, 100, 0.6, 0.12468265038069815},
		{"negative start capital falls back to first point", -5, 100, 160, 100, 0.6, 0.12468265038069815},
		{"start capital is the base even if first point differs", 80, 100, 160, 80, 1.0, 0.18920711500272103},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(Input{StartCapital: tc.startCap, Equity: []Point{{first, tc.eq0}, {last, tc.end}}})
			approx(t, "Years", m.Years, 4)
			approx(t, "StartValue", m.StartValue, tc.wantStart)
			approx(t, "EndValue", m.EndValue, tc.end)
			approx(t, "TotalReturn", m.TotalReturn, tc.wantTotal)
			approx(t, "CAGR", m.CAGR, tc.cagr)
			if !m.Start.Equal(first) || !m.End.Equal(last) {
				t.Errorf("Start/End = %v/%v, want %v/%v", m.Start, m.End, first, last)
			}
		})
	}
}

func TestCompute_CAGRUsesCalendarYearsOf365Point25Days(t *testing.T) {
	// 2020-01-01 to 2021-01-01 is 366 days, so Years = 366/365.25 = 1.002053388090349 and
	// CAGR = 1.1^(365.25/366) - 1 = 0.09978518245839707, slightly below the 10% simple return.
	m := Compute(Input{StartCapital: 100, Equity: []Point{{day(2020, 1, 1), 100}, {day(2021, 1, 1), 110}}})
	approx(t, "Years", m.Years, 1.002053388090349)
	approx(t, "CAGR", m.CAGR, 0.09978518245839707)
}

func TestCompute_SharpeSortinoVol(t *testing.T) {
	// Daily returns r = +2%, -1%, +3%, -2%, +1%, 0, +2%, -1% on 100 of capital.
	// mean = 0.04/8 = 0.005; squared deviations sum to 0.0022 so sample sd = sqrt(0.0022/7) = 0.0177281052.
	// Vol = sd*sqrt(252) = 0.2814249456.
	// rf=0: downside returns are -1%, -2%, -1% -> sqrt((1e-4+4e-4+1e-4)/8) = 0.00866025.
	// rf=5.04%: daily rf = 0.0002; shortfalls are -0.0102, -0.0202, -0.0002 (the flat day), -0.0102
	//           -> sqrt(0.00061616/8) = 0.0087761.
	rets := []float64{0.02, -0.01, 0.03, -0.02, 0.01, 0.0, 0.02, -0.01}
	eq := series(day(2021, 3, 1), compound(100, rets)...)
	tests := []struct {
		name            string
		rf              float64
		sharpe, sortino float64
	}{
		{"rf zero", 0, 4.477215043467818, 9.165151389911678},
		{"rf 5.04 percent", 0.0504, 4.298126441729106, 8.6823992127365},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(Input{StartCapital: 100, Equity: eq, Rf: tc.rf})
			approx(t, "Vol", m.Vol, 0.28142494558940584)
			approx(t, "Sharpe", m.Sharpe, tc.sharpe)
			approx(t, "Sortino", m.Sortino, tc.sortino)
		})
	}
}

func TestCompute_FlatEquityHasNoRiskStatsAndNoNaN(t *testing.T) {
	m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 1, 1), 100, 100, 100, 100)})
	for name, v := range map[string]float64{
		"Vol": m.Vol, "Sharpe": m.Sharpe, "Sortino": m.Sortino, "Calmar": m.Calmar, "Ulcer": m.Ulcer,
		"UlcerPerf": m.UlcerPerf, "TotalReturn": m.TotalReturn, "CAGR": m.CAGR, "MaxDD.Depth": m.MaxDD.Depth,
	} {
		if v != 0 {
			t.Errorf("%s = %v on a flat curve, want 0", name, v)
		}
	}
}

func TestCompute_Drawdowns(t *testing.T) {
	base := day(2021, 3, 1)
	tests := []struct {
		name     string
		startCap float64
		vals     []float64

		depth                                 float64
		peak, trough, rec                     int // day index from base, -1 = zero time
		recovered                             bool
		peakToTrough, troughToRec, underwater float64
		uwDays                                float64
		uwPeak, uwEnd                         int
		uwRecovered                           bool
		uwDepth, ulcer                        float64
	}{
		{
			// Peak 120 (day 1), trough 80 (day 3), back to 120 on day 5 (equal to the peak counts as recovered).
			// A second, shallower dip (130 -> 117, 3 days) must not displace the deepest one.
			name: "classic recovered drawdown", startCap: 100,
			vals:  []float64{110, 120, 90, 80, 100, 120, 130, 117, 117, 130, 140},
			depth: -1.0 / 3, peak: 1, trough: 3, rec: 5, recovered: true,
			peakToTrough: 2, troughToRec: 2, underwater: 4,
			uwDays: 4, uwPeak: 1, uwEnd: 5, uwRecovered: true, uwDepth: -1.0 / 3,
			ulcer: 0.14186705969414687,
		},
		{
			// Deepest drawdown (-50%, 2 days to recover) is NOT the longest underwater stretch:
			// that is 120 (day 4) -> 121 (day 10) = 6 days at a depth of only 115/120-1.
			name: "deepest drawdown differs from longest underwater", startCap: 100,
			vals:  []float64{105, 110, 55, 110, 120, 119, 118, 117, 116, 115, 121},
			depth: -0.5, peak: 1, trough: 2, rec: 3, recovered: true,
			peakToTrough: 1, troughToRec: 1, underwater: 2,
			uwDays: 6, uwPeak: 4, uwEnd: 10, uwRecovered: true, uwDepth: 115.0/120 - 1,
			ulcer: 0.1519029129065501,
		},
		{
			// Never recovers: peak 120 (day 1), trough 90 (day 3), ends at 95. Underwater runs to the last date.
			name: "never recovered", startCap: 100,
			vals:  []float64{100, 120, 100, 90, 95},
			depth: -0.25, peak: 1, trough: 3, rec: -1, recovered: false,
			peakToTrough: 2, troughToRec: 0, underwater: 3,
			uwDays: 3, uwPeak: 1, uwEnd: 4, uwRecovered: false, uwDepth: -0.25,
			ulcer: 0.16351180725290487,
		},
		{
			// Starting capital 100 is the first high-water mark, so 95 and 90 are already drawdowns.
			name: "below start capital from the first point", startCap: 100,
			vals:  []float64{95, 90, 100},
			depth: -0.1, peak: 0, trough: 1, rec: 2, recovered: true,
			peakToTrough: 1, troughToRec: 1, underwater: 2,
			uwDays: 2, uwPeak: 0, uwEnd: 2, uwRecovered: true, uwDepth: -0.1,
			ulcer: 0.06454972243679029,
		},
		{
			// Returning to exactly the old peak (120 on day 3) resets the high-water mark date, so the later -25% dip
			// is measured from day 3, not day 1. The earlier 2-day stretch (day 1 -> 3) is the longest underwater one.
			name: "recovery to exactly the old peak moves the peak date", startCap: 100,
			vals:  []float64{100, 120, 110, 120, 90},
			depth: -0.25, peak: 3, trough: 4, rec: -1, recovered: false,
			peakToTrough: 1, troughToRec: 0, underwater: 1,
			uwDays: 2, uwPeak: 1, uwEnd: 3, uwRecovered: true, uwDepth: 110.0/120 - 1,
			ulcer: 0.11785113019775792,
		},
		{
			name: "monotone rise has no drawdown", startCap: 100,
			vals:  []float64{100, 101, 102, 103},
			depth: 0, peak: -1, trough: -1, rec: -1,
			uwPeak: -1, uwEnd: -1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(Input{StartCapital: tc.startCap, Equity: series(base, tc.vals...)})
			dd := m.MaxDD
			approx(t, "MaxDD.Depth", dd.Depth, tc.depth)
			sameDay(t, "PeakDate", dd.PeakDate, base, tc.peak)
			sameDay(t, "TroughDate", dd.TroughDate, base, tc.trough)
			sameDay(t, "RecoveryDate", dd.RecoveryDate, base, tc.rec)
			if dd.Recovered != tc.recovered {
				t.Errorf("Recovered = %v, want %v", dd.Recovered, tc.recovered)
			}
			approx(t, "DaysPeakToTrough", dd.DaysPeakToTrough, tc.peakToTrough)
			approx(t, "DaysTroughToRecovery", dd.DaysTroughToRecovery, tc.troughToRec)
			approx(t, "DaysUnderwater", dd.DaysUnderwater, tc.underwater)

			uw := m.LongestUnderwater
			approx(t, "Underwater.Days", uw.Days, tc.uwDays)
			sameDay(t, "Underwater.PeakDate", uw.PeakDate, base, tc.uwPeak)
			sameDay(t, "Underwater.EndDate", uw.EndDate, base, tc.uwEnd)
			if uw.Recovered != tc.uwRecovered {
				t.Errorf("Underwater.Recovered = %v, want %v", uw.Recovered, tc.uwRecovered)
			}
			approx(t, "Underwater.Depth", uw.Depth, tc.uwDepth)
			approx(t, "Ulcer", m.Ulcer, tc.ulcer)
		})
	}
}

func TestCompute_CalmarUlcerPerformance(t *testing.T) {
	// Yearly points over exactly 4 years: 100, 120, 90, 130, 160.
	// CAGR = 1.6^0.25 - 1 = 0.12468265. Only drawdown: 120 -> 90 = -25%, recovered at 130 (365 + 365 days later).
	// Calmar = CAGR/0.25 = 0.4987306. Ulcer = sqrt((25^2)/5)/100 = sqrt(125)/100 = 0.1118034.
	// UlcerPerf = (CAGR - 0.04)/Ulcer = 0.7574247.
	eq := []Point{
		{day(2020, 1, 1), 100}, {day(2021, 1, 1), 120}, {day(2022, 1, 1), 90}, {day(2023, 1, 1), 130}, {day(2024, 1, 1), 160},
	}
	m := Compute(Input{StartCapital: 100, Equity: eq, Rf: 0.04})
	approx(t, "CAGR", m.CAGR, 0.12468265038069815)
	approx(t, "MaxDD.Depth", m.MaxDD.Depth, -0.25)
	approx(t, "DaysPeakToTrough", m.MaxDD.DaysPeakToTrough, 365)
	approx(t, "DaysTroughToRecovery", m.MaxDD.DaysTroughToRecovery, 365)
	approx(t, "DaysUnderwater", m.MaxDD.DaysUnderwater, 730)
	approx(t, "LongestUnderwater.Days", m.LongestUnderwater.Days, 730)
	approx(t, "Calmar", m.Calmar, 0.4987306015227926)
	approx(t, "Ulcer", m.Ulcer, 0.1118033988749895)
	approx(t, "UlcerPerf", m.UlcerPerf, 0.7574246510643579)
}

func TestCompute_CalendarYearsAndMonths(t *testing.T) {
	// Year ends: 2019 = 120, 2020 = 150, 2021 = 120, 2022 = 132 (last date 15 Feb, so the year is partial).
	// Year returns: 120/100-1 = 0.2, 150/120-1 = 0.25, 120/150-1 = -0.2, 132/120-1 = 0.1.
	// Calendar months with an observation (8): returns 0 (first month, no change from 100), +, +, -, +, -, -, +
	// -> 4 strictly positive of 8 = 0.5. A flat month is not "positive".
	eq := []Point{
		{day(2019, 1, 2), 100}, {day(2019, 6, 28), 110}, {day(2019, 12, 31), 120},
		{day(2020, 3, 31), 90}, {day(2020, 12, 31), 150},
		{day(2021, 6, 30), 135}, {day(2021, 12, 31), 120},
		{day(2022, 2, 15), 132},
	}
	m := Compute(Input{StartCapital: 100, Equity: eq})
	want := []YearReturn{
		{2019, 0.2, false}, {2020, 0.25, false}, {2021, -0.2, false}, {2022, 0.1, true},
	}
	if len(m.YearReturns) != len(want) {
		t.Fatalf("got %d year returns, want %d: %+v", len(m.YearReturns), len(want), m.YearReturns)
	}
	for i, w := range want {
		g := m.YearReturns[i]
		if g.Year != w.Year || g.Partial != w.Partial {
			t.Errorf("year[%d] = %+v, want year %d partial %v", i, g, w.Year, w.Partial)
		}
		approx(t, fmt.Sprintf("return %d", w.Year), g.Return, w.Return)
	}
	if m.BestYear.Year != 2020 {
		t.Errorf("BestYear = %d, want 2020", m.BestYear.Year)
	}
	if m.WorstYear.Year != 2021 {
		t.Errorf("WorstYear = %d, want 2021 (partial 2022 must be ignored)", m.WorstYear.Year)
	}
	if m.Months != 8 {
		t.Errorf("Months = %d, want 8", m.Months)
	}
	approx(t, "PositiveMonths", m.PositiveMonths, 0.5)
}

func TestCompute_PartialYearFlags(t *testing.T) {
	// First year is partial if the series starts after 10 Jan; last year is partial if it ends before 20 Dec.
	// BestYear/WorstYear ignore partial years (wantBest 0 means no full year exists; -1 means not checked).
	tests := []struct {
		name                        string
		first, last                 time.Time
		wantFirstPart, wantLastPart bool
		wantBest                    int
	}{
		{"starts 10 Jan and ends 20 Dec: both full", day(2019, 1, 10), day(2020, 12, 20), false, false, -1},
		{"starts 11 Jan and ends 19 Dec: both partial", day(2019, 1, 11), day(2020, 12, 19), true, true, 0},
		{"starts mid-March, ends 31 Dec", day(2019, 3, 15), day(2020, 12, 31), true, false, 2020},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eq := []Point{{tc.first, 100}, {day(2019, 12, 31), 110}, {tc.last, 121}}
			m := Compute(Input{StartCapital: 100, Equity: eq})
			if len(m.YearReturns) != 2 {
				t.Fatalf("got %d year returns, want 2", len(m.YearReturns))
			}
			if got := m.YearReturns[0].Partial; got != tc.wantFirstPart {
				t.Errorf("first year Partial = %v, want %v", got, tc.wantFirstPart)
			}
			if got := m.YearReturns[1].Partial; got != tc.wantLastPart {
				t.Errorf("last year Partial = %v, want %v", got, tc.wantLastPart)
			}
			approx(t, "2019 return", m.YearReturns[0].Return, 0.1)
			approx(t, "2020 return", m.YearReturns[1].Return, 0.1)
			if tc.wantBest >= 0 && m.BestYear.Year != tc.wantBest {
				t.Errorf("BestYear = %d, want %d", m.BestYear.Year, tc.wantBest)
			}
			if tc.wantBest >= 0 && m.WorstYear.Year != tc.wantBest {
				t.Errorf("WorstYear = %d, want %d", m.WorstYear.Year, tc.wantBest)
			}
		})
	}
}

func TestMonthEnds(t *testing.T) {
	// Month return = last value in the month / last value of the previous month; the first month uses the open.
	// Jan 105/100-1 = 0.05, Feb 110.25/105-1 = 0.05, Mar 99/110.25-1 = -0.10204081632653061.
	eq := []Point{
		{day(2021, 1, 4), 102}, {day(2021, 1, 29), 105},
		{day(2021, 2, 1), 100}, {day(2021, 2, 26), 110.25},
		{day(2021, 3, 10), 99},
	}
	got := monthEnds(eq, 100)
	wantKeys := []string{"2021-01", "2021-02", "2021-03"}
	wantRet := []float64{0.05, 0.05, -0.10204081632653061}
	wantVal := []float64{105, 110.25, 99}
	if !reflect.DeepEqual(got.keys, wantKeys) {
		t.Fatalf("keys = %v, want %v", got.keys, wantKeys)
	}
	for i := range wantKeys {
		approx(t, "ret "+wantKeys[i], got.ret[i], wantRet[i])
		approx(t, "val "+wantKeys[i], got.val[i], wantVal[i])
	}
}

func TestRolling(t *testing.T) {
	// Yearly points 100, 110, 99, 128.7, 141.57 on 1 Jan 2020..2024 (exact anniversaries).
	// 1y windows end on 2021..2024: +10%, -10%, +30%, +10% -> 4 windows, min -0.1, max 0.3, median (index 2 of the
	// sorted four) 0.1, one of four >= 15%, one of four < 0.
	// 3y windows end on 2023 and 2024; both are 1.287^(1/3)-1 = 0.08774271237426245.
	// 5y: the first possible window would need data from 2019, so there are none.
	eq := []Point{
		{day(2020, 1, 1), 100}, {day(2021, 1, 1), 110}, {day(2022, 1, 1), 99},
		{day(2023, 1, 1), 128.7}, {day(2024, 1, 1), 141.57},
	}
	t.Run("1y", func(t *testing.T) {
		r := rolling(eq, 1)
		if r.Years != 1 || r.Windows != 4 {
			t.Fatalf("Years/Windows = %d/%d, want 1/4", r.Years, r.Windows)
		}
		approx(t, "Min", r.Min, -0.1)
		approx(t, "Max", r.Max, 0.3)
		approx(t, "Median", r.Median, 0.1)
		approx(t, "ShareAbove", r.ShareAbove, 0.25)
		approx(t, "ShareBelow0", r.ShareBelow0, 0.25)
	})
	t.Run("3y", func(t *testing.T) {
		r := rolling(eq, 3)
		if r.Windows != 2 {
			t.Fatalf("Windows = %d, want 2", r.Windows)
		}
		for name, v := range map[string]float64{"Min": r.Min, "Max": r.Max, "Median": r.Median} {
			approx(t, name, v, 0.08774271237426245)
		}
		approx(t, "ShareAbove", r.ShareAbove, 0)
		approx(t, "ShareBelow0", r.ShareBelow0, 0)
	})
	t.Run("5y has no windows", func(t *testing.T) {
		r := rolling(eq, 5)
		if r.Years != 5 || r.Windows != 0 || r.Min != 0 || r.Max != 0 || r.Median != 0 {
			t.Errorf("rolling 5y = %+v, want empty with Years 5", r)
		}
	})
	t.Run("Compute reports 1, 3 and 5 year windows", func(t *testing.T) {
		m := Compute(Input{StartCapital: 100, Equity: eq})
		if len(m.Rolling) != 3 {
			t.Fatalf("len(Rolling) = %d, want 3", len(m.Rolling))
		}
		for i, y := range []int{1, 3, 5} {
			if m.Rolling[i].Years != y {
				t.Errorf("Rolling[%d].Years = %d, want %d", i, m.Rolling[i].Years, y)
			}
		}
		if m.Rolling[0].Windows != 4 || m.Rolling[1].Windows != 2 || m.Rolling[2].Windows != 0 {
			t.Errorf("windows = %d/%d/%d, want 4/2/0", m.Rolling[0].Windows, m.Rolling[1].Windows, m.Rolling[2].Windows)
		}
	})
}

func TestRolling_ShareAboveThreshold(t *testing.T) {
	// 1y windows of +16% and +14%: only the first reaches the 15% bar.
	eq := []Point{{day(2020, 1, 1), 100}, {day(2021, 1, 1), 116}, {day(2022, 1, 1), 116 * 1.14}}
	r := rolling(eq, 1)
	if r.Windows != 2 {
		t.Fatalf("Windows = %d, want 2", r.Windows)
	}
	approx(t, "ShareAbove", r.ShareAbove, 0.5)
	approx(t, "ShareBelow0", r.ShareBelow0, 0)
}

func TestMeanStd(t *testing.T) {
	tests := []struct {
		name         string
		in           []float64
		mean, sample float64
	}{
		{"empty", nil, 0, 0},
		{"single value has zero sd", []float64{7}, 7, 0},
		{"classic eight", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 5, 2.138089935299395}, // sqrt(32/7)
		{"constant", []float64{3, 3, 3}, 3, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, s := meanStd(tc.in)
			approx(t, "mean", m, tc.mean)
			approx(t, "sd", s, tc.sample)
		})
	}
}

func TestAlignTo(t *testing.T) {
	a := series(day(2021, 1, 1), 0, 0, 0, 0, 0) // 1..5 Jan
	t.Run("forward fills and uses same-day values", func(t *testing.T) {
		b := []Point{{day(2021, 1, 2), 10}, {day(2021, 1, 4), 20}}
		got := alignTo(a, b)
		if !math.IsNaN(got[0]) {
			t.Errorf("before the first benchmark point want NaN, got %v", got[0])
		}
		for i, want := range []float64{10, 10, 20, 20} {
			approx(t, "got", got[i+1], want)
		}
	})
	t.Run("older benchmark fills every date", func(t *testing.T) {
		got := alignTo(a, []Point{{day(2020, 12, 25), 5}})
		for _, g := range got {
			approx(t, "got", g, 5)
		}
	})
}

var benchMonthly = []float64{
	0.02, -0.01, 0.03, 0.015, -0.025, 0.01, 0.04, -0.02, 0.005, 0.0, -0.015, 0.025,
	0.012, -0.03, 0.018, 0.007, -0.008, 0.022, -0.012, 0.03, -0.005, 0.014, 0.009,
}

// monthlyCurve builds a curve with one point on 2 Jan 2020 (value 100), then two points (15th flat, 28th moved) in
// each following month. Each month-end value is the previous month-end times (1 + mult*r).
// That gives 47 points over 24 calendar months, and every month return of the curve equals mult * r exactly
// (January contributes 0 for every multiplier).
func monthlyCurve(rets []float64, mult float64) []Point {
	pts := []Point{{day(2020, 1, 2), 100}}
	v := 100.0
	for k, r := range rets {
		m := time.Month(2 + k) // Date normalises months above 12 into the next year
		pts = append(pts, Point{day(2020, m, 15), v})
		v *= 1 + mult*r
		pts = append(pts, Point{day(2020, m, 28), v})
	}
	return pts
}

func TestCompute_BenchmarkRelativeStats(t *testing.T) {
	// The strategy's monthly returns are mult x the benchmark's, so beta = mult and correlation = sign(mult).
	// Jensen alpha with the monthly rf: 12*((mult*mb - rf/12) - mult*(mb - rf/12)) = (mult-1)*rf, i.e. 0 for rf=0.
	// TE, IR and capture ratios, and BenchCAGR, come from numpy over the 24 monthly pairs.
	bench := monthlyCurve(benchMonthly, 1)
	tests := []struct {
		name                   string
		mult, rf               float64
		beta, corr, alpha      float64
		te, ir, upCap, downCap float64
	}{
		{"identical to benchmark", 1, 0, 1, 1, 0, 0, 0, 1, 1},
		{"2x benchmark, rf 0", 2, 0, 2, 1, 0, 0.06383402391119998, 1.0339313732095774, 1.9951207969219935, 2.0044717548124766},
		{"2x benchmark, rf 6 percent: alpha = rf", 2, 0.06, 2, 1, 0.06, 0.06383402391119998, 1.0339313732095774, 1.9951207969219935, 2.0044717548124766},
		{"half benchmark, rf 0", 0.5, 0, 0.5, 1, 0, 0.03191701195559999, -1.0339313732095774, 0.5006298846609046, 0.4994565175464146},
		{"half benchmark, rf 6 percent", 0.5, 0.06, 0.5, 1, -0.03, 0.03191701195559999, -1.0339313732095774, 0.5006298846609046, 0.4994565175464146},
		{"inverse of benchmark, rf 0", -1, 0, -1, -1, 0, 0.12766804782239996, -1.0339313732095774, -1.0052116474825283, -0.99576856192469},
		{"inverse of benchmark, rf 6 percent", -1, 0.06, -1, -1, -0.12, 0.12766804782239996, -1.0339313732095774, -1.0052116474825283, -0.99576856192469},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(Input{StartCapital: 100, Equity: monthlyCurve(benchMonthly, tc.mult), Benchmark: bench, Rf: tc.rf})
			if m.Rel == nil {
				t.Fatal("Rel is nil, want benchmark statistics")
			}
			r := m.Rel
			if r.Months != 24 {
				t.Errorf("Months = %d, want 24", r.Months)
			}
			approx(t, "Beta", r.Beta, tc.beta)
			approx(t, "Correlation", r.Correlation, tc.corr)
			approx(t, "Alpha", r.Alpha, tc.alpha)
			approx(t, "TrackingError", r.TrackingError, tc.te)
			approx(t, "InfoRatio", r.InfoRatio, tc.ir)
			approx(t, "UpCapture", r.UpCapture, tc.upCap)
			approx(t, "DownCapture", r.DownCapture, tc.downCap)
			// Benchmark: 100 -> 113.62885487511845 between 2020-01-02 and 2021-12-28 (726 days).
			approx(t, "BenchCAGR", r.BenchCAGR, 0.06639054026841018)
		})
	}
}

func TestCompute_BenchmarkRelativeNeedsEnoughData(t *testing.T) {
	strategy := monthlyCurve(benchMonthly, 2)
	tests := []struct {
		name   string
		eq     []Point
		bench  []Point
		wantRe bool
	}{
		{"no benchmark", strategy, nil, false},
		{"single-point benchmark", strategy, []Point{{day(2020, 1, 2), 100}}, false},
		{"benchmark starts after the strategy ends", strategy, series(day(2022, 1, 1), 100, 101, 102), false},
		{"benchmark covers only the last 20 strategy points", strategy, monthlyCurve(benchMonthly, 1)[27:], false},
		// 45 daily points, but only 2 calendar months, below the 6-month minimum.
		{"enough points but under six months", series(day(2021, 1, 1), ramp(45)...), series(day(2021, 1, 1), ramp(45)...), false},
		{"full overlap", strategy, monthlyCurve(benchMonthly, 1), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(Input{StartCapital: 100, Equity: tc.eq, Benchmark: tc.bench})
			if (m.Rel != nil) != tc.wantRe {
				t.Errorf("Rel != nil is %v, want %v", m.Rel != nil, tc.wantRe)
			}
		})
	}
}

func ramp(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 100 + float64(i)
	}
	return out
}

func TestTradeMethods(t *testing.T) {
	b := day(2021, 1, 1)
	tests := []struct {
		name           string
		tr             Trade
		pnl, ret, hold float64
	}{
		{"winner with charges", Trade{BuyDate: b, SellDate: b.AddDate(0, 0, 10), Qty: 20, BuyPrice: 50, SellPrice: 60, Charges: 10}, 190, 0.19, 10},
		{"loser", Trade{BuyDate: b, SellDate: b.AddDate(0, 0, 30), Qty: 10, BuyPrice: 200, SellPrice: 190}, -100, -0.05, 30},
		{"flat price but charges make it a loss", Trade{BuyDate: b, SellDate: b, Qty: 10, BuyPrice: 100, SellPrice: 100, Charges: 5}, -5, -0.005, 0},
		{"zero cost has zero return, not NaN", Trade{BuyDate: b, SellDate: b.AddDate(0, 0, 1), Qty: 0, BuyPrice: 100, SellPrice: 110}, 0, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			approx(t, "PnL", tc.tr.PnL(), tc.pnl)
			approx(t, "Return", tc.tr.Return(), tc.ret)
			approx(t, "HoldDays", tc.tr.HoldDays(), tc.hold)
		})
	}
}

// lot builds a trade bought on 1 Jan 2021 and sold (or marked) hold days later.
func lot(buy, sell, qty, charges float64, hold int, open bool, minPx float64) Trade {
	b := day(2021, 1, 1)
	return Trade{Symbol: "X", BuyDate: b, SellDate: b.AddDate(0, 0, hold), Qty: qty, BuyPrice: buy, SellPrice: sell,
		Charges: charges, Open: open, MinPrice: minPx}
}

// tradeInput wraps trades in the smallest valid Input; the curve ends at endValue.
func tradeInput(endValue float64, trades ...Trade) Input {
	return Input{StartCapital: 1000, Equity: series(day(2021, 1, 1), 1000, endValue), Trades: trades}
}

func TestComputeTradeStats_MixedBook(t *testing.T) {
	// Closed lots (net pnl / net return / hold days):
	//   A +100 / +10%   / 10     B -100 / -5% / 30     C +190 / +19% / 20 (50->60 x20, Rs 10 charges)
	//   D  -5 / -0.5%   / 5 (flat price, Rs 5 charges)  E  0 / 0 / 2 (neither a win nor a loss)
	// Open lots: F +200 / hold 100;  G -300 / hold 200 (lowest close 60 vs buy 100);  H -50 / hold 150.
	// Final equity 1000.
	trades := []Trade{
		lot(100, 110, 10, 0, 10, false, 0),
		lot(200, 190, 10, 0, 30, false, 180),
		lot(50, 60, 20, 10, 20, false, 0),
		lot(100, 100, 10, 5, 5, false, 0),
		lot(100, 100, 10, 0, 2, false, 0),
		lot(100, 120, 10, 0, 100, true, 0),
		lot(100, 70, 10, 0, 200, true, 60),
		lot(50, 45, 10, 0, 150, true, 0),
	}
	m := Compute(tradeInput(1000, trades...))
	if m.ClosedLots != 5 || m.OpenLots != 3 {
		t.Fatalf("Closed/Open = %d/%d, want 5/3", m.ClosedLots, m.OpenLots)
	}
	checks := []struct {
		name      string
		got, want float64
	}{
		{"WinRateClosed 2 of 5", m.WinRateClosed, 0.4},
		{"WinRateAll 3 of 8 (open winner counts)", m.WinRateAll, 0.375},
		{"AvgWinPct (0.10+0.19)/2", m.AvgWinPct, 0.145},
		{"AvgWinRs (100+190)/2", m.AvgWinRs, 145},
		{"AvgLossPct (-0.05-0.005)/2", m.AvgLossPct, -0.0275},
		{"AvgLossRs (-100-5)/2", m.AvgLossRs, -52.5},
		{"WinLossRatio 0.145/0.0275", m.WinLossRatio, 5.2727272727272725},
		{"ProfitFactorClosed 290/105", m.ProfitFactorClosed, 2.761904761904762},
		{"ProfitFactorAll 490/455", m.ProfitFactorAll, 1.0769230769230769},
		{"ExpectancyPct 0.235/5", m.ExpectancyPct, 0.047},
		{"ExpectancyRs 185/5", m.ExpectancyRs, 37},
		{"AvgHoldDays 67/5", m.AvgHoldDays, 13.4},
		{"MaxHoldDays closed only", m.MaxHoldDays, 30},
		{"AvgHoldDaysAll 517/8", m.AvgHoldDaysAll, 64.625},
		{"LongestLosingHoldDays", m.LongestLosingHoldDays, 200},
		{"LongestOpenLoserDays", m.LongestOpenLoserDays, 200},
		{"WorstLotDrawdown 60/100-1", m.WorstLotDrawdown, -0.4},
		{"OpenUnrealisedPct (200-300-50)/1000", m.OpenUnrealisedPct, -0.15},
	}
	for _, c := range checks {
		approx(t, c.name, c.got, c.want)
	}
	if m.OpenLoserCount != 2 {
		t.Errorf("OpenLoserCount = %d, want 2", m.OpenLoserCount)
	}
}

func TestComputeTradeStats_LongestLosingHoldCanBeAClosedLot(t *testing.T) {
	m := Compute(tradeInput(1000,
		lot(100, 80, 10, 0, 300, false, 0), // closed loser held 300 days
		lot(100, 90, 10, 0, 100, true, 0),  // open loser held 100 days
	))
	approx(t, "LongestLosingHoldDays", m.LongestLosingHoldDays, 300)
	approx(t, "LongestOpenLoserDays", m.LongestOpenLoserDays, 100)
	if m.OpenLoserCount != 1 {
		t.Errorf("OpenLoserCount = %d, want 1", m.OpenLoserCount)
	}
}

func TestComputeTradeStats_EdgeBooks(t *testing.T) {
	inf := math.Inf(1)
	tests := []struct {
		name                                           string
		trades                                         []Trade
		closed, open                                   int
		winClosed, winAll, pfClosed, pfAll, wlr, expRs float64
	}{
		{"no trades", nil, 0, 0, 0, 0, 0, 0, 0, 0},
		{
			"only winners: profit factor and win/loss ratio are infinite",
			[]Trade{lot(100, 110, 10, 0, 5, false, 0), lot(100, 120, 10, 0, 5, false, 0)},
			2, 0, 1, 1, inf, inf, inf, 150,
		},
		{
			"only losers: profit factor and win/loss ratio are zero",
			[]Trade{lot(100, 90, 10, 0, 5, false, 0), lot(100, 80, 10, 0, 5, false, 0)},
			2, 0, 0, 0, 0, 0, 0, -150,
		},
		{
			"closed winner plus open loser: closed PF infinite, all-lots PF finite (100/50)",
			[]Trade{lot(100, 110, 10, 0, 5, false, 0), lot(100, 95, 10, 0, 50, true, 0)},
			1, 1, 1, 0.5, inf, 2, inf, 100,
		},
		{
			"open lots only: closed-lot stats stay zero",
			[]Trade{lot(100, 110, 10, 0, 5, true, 0), lot(100, 95, 10, 0, 50, true, 0)},
			0, 2, 0, 0.5, 0, 2, 0, 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Compute(tradeInput(1000, tc.trades...))
			if m.ClosedLots != tc.closed || m.OpenLots != tc.open {
				t.Errorf("Closed/Open = %d/%d, want %d/%d", m.ClosedLots, m.OpenLots, tc.closed, tc.open)
			}
			approx(t, "WinRateClosed", m.WinRateClosed, tc.winClosed)
			approx(t, "WinRateAll", m.WinRateAll, tc.winAll)
			approx(t, "ProfitFactorClosed", m.ProfitFactorClosed, tc.pfClosed)
			approx(t, "ProfitFactorAll", m.ProfitFactorAll, tc.pfAll)
			approx(t, "WinLossRatio", m.WinLossRatio, tc.wlr)
			approx(t, "ExpectancyRs", m.ExpectancyRs, tc.expRs)
		})
	}
}

func TestCompute_ExposureAndTurnover(t *testing.T) {
	// Four observations over exactly 4 years, equity 100, 100, 200, 200 (mean 150).
	// Invested rupees 0, 50, 100, 200 -> fractions 0, 0.5, 0.5, 1 -> average 0.5, max 1, 3 of 4 days exposed.
	// Stocks 0, 1, 2, 5 -> average 2.
	// Turnover: traded 1200 / 2 / mean equity 150 / 4 years = 1.0 per year.
	// Cost drag: charges (30 + 60) / 150 / 4 = 0.15 per year.
	eq := []Point{{day(2020, 1, 1), 100}, {day(2021, 1, 1), 100}, {day(2022, 1, 1), 200}, {day(2024, 1, 1), 200}}
	in := Input{
		StartCapital: 100,
		Equity:       eq,
		Invested:     []float64{0, 50, 100, 200},
		Stocks:       []int{0, 1, 2, 5},
		TradedValue:  1200,
		Trades: []Trade{
			lot(100, 100, 1, 30, 10, false, 0),
			lot(100, 100, 1, 60, 10, false, 0),
		},
	}
	t.Run("full inputs", func(t *testing.T) {
		m := Compute(in)
		approx(t, "AvgInvestedPct", m.AvgInvestedPct, 0.5)
		approx(t, "MaxInvestedPct", m.MaxInvestedPct, 1.0)
		approx(t, "ExposureDays", m.ExposureDays, 0.75)
		approx(t, "AvgStocks", m.AvgStocks, 2)
		approx(t, "TurnoverPerYear", m.TurnoverPerYear, 1.0)
		approx(t, "CostDragAnnual", m.CostDragAnnual, 0.15)
	})
	t.Run("length mismatch ignores invested and stocks but keeps turnover", func(t *testing.T) {
		bad := in
		bad.Invested = []float64{0, 50, 100}
		bad.Stocks = []int{1}
		m := Compute(bad)
		if m.AvgInvestedPct != 0 || m.MaxInvestedPct != 0 || m.ExposureDays != 0 || m.AvgStocks != 0 {
			t.Errorf("exposure fields = %v/%v/%v/%v, want all 0", m.AvgInvestedPct, m.MaxInvestedPct, m.ExposureDays, m.AvgStocks)
		}
		approx(t, "TurnoverPerYear", m.TurnoverPerYear, 1.0)
	})
	t.Run("no trading means no turnover or cost drag", func(t *testing.T) {
		quiet := in
		quiet.TradedValue = 0
		quiet.Trades = nil
		m := Compute(quiet)
		approx(t, "TurnoverPerYear", m.TurnoverPerYear, 0)
		approx(t, "CostDragAnnual", m.CostDragAnnual, 0)
	})
}

func TestCompute_AverageDrawdown(t *testing.T) {
	// Start 100, curve 100, 120, 100, 90, 95: drawdowns 0, 0, -1/6, -1/4, -5/24 = -15/24 in total; mean over 5 = -0.125.
	m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 3, 1), 100, 120, 100, 90, 95)})
	approx(t, "AvgDrawdown", m.AvgDrawdown, -0.125)
	if m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 3, 1), 100, 101, 102)}); m.AvgDrawdown != 0 {
		t.Errorf("AvgDrawdown on a rising curve = %v, want 0", m.AvgDrawdown)
	}
}

func TestCompute_DayExtremesVaRAndExpectedShortfall(t *testing.T) {
	// 40 daily returns 0, 0.1%, 0.2%, 0.3%, 0.4% repeating, except day 7 = -2%, day 19 = -5%, day 30 = -1%, day 35 = +4%.
	// Sorted worst first: -5%, -2%, -1%, 0, ... With 5% of 40 days = 2 observations in the tail:
	//   VaR95 = the 2nd worst loss = 2%;  CVaR95 = mean of the 2 worst = (5% + 2%)/2 = 3.5%.
	rets := make([]float64, 40)
	for i := range rets {
		rets[i] = 0.001 * float64(i%5)
	}
	rets[7], rets[19], rets[30], rets[35] = -0.02, -0.05, -0.01, 0.04
	m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 3, 1), compound(100, rets)...)})
	approx(t, "WorstDay", m.WorstDay, -0.05)
	approx(t, "BestDay", m.BestDay, 0.04)
	approx(t, "VaR95", m.VaR95, 0.02)
	approx(t, "CVaR95", m.CVaR95, 0.035)

	t.Run("short history still uses at least one observation", func(t *testing.T) {
		// 8 returns: 5% of 8 rounds down to 0, so the tail is the single worst day (-2%).
		short := compound(100, []float64{0.02, -0.01, 0.03, -0.02, 0.01, 0.0, 0.02, -0.01})
		m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 3, 1), short...)})
		approx(t, "VaR95", m.VaR95, 0.02)
		approx(t, "CVaR95", m.CVaR95, 0.02)
		approx(t, "WorstDay", m.WorstDay, -0.02)
		approx(t, "BestDay", m.BestDay, 0.03)
	})
}

func TestCompute_SkewAndKurtosisOfDailyReturns(t *testing.T) {
	t.Run("alternating +1/-1 percent is symmetric and flat-topped", func(t *testing.T) {
		rets := make([]float64, 100)
		for i := range rets {
			rets[i] = 0.01 - 0.02*float64(i%2)
		}
		m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 1, 1), compound(100, rets)...)})
		approx(t, "Skew", m.Skew, 0)
		// A two-point distribution has population kurtosis 1, i.e. excess -2 (the code's n-1 scaling moves it by 0.02).
		if m.ExcessKurtosis < -2.05 || m.ExcessKurtosis > -1.95 {
			t.Errorf("ExcessKurtosis = %v, want about -2", m.ExcessKurtosis)
		}
	})
	t.Run("mirroring the returns flips the skew and keeps the kurtosis", func(t *testing.T) {
		rets := make([]float64, 40)
		neg := make([]float64, 40)
		for i := range rets {
			rets[i] = 0.001 * float64(i%5)
		}
		rets[7], rets[19], rets[30], rets[35] = -0.02, -0.05, -0.01, 0.04
		for i, r := range rets {
			neg[i] = -r
		}
		a := Compute(Input{StartCapital: 100, Equity: series(day(2021, 1, 1), compound(100, rets)...)})
		b := Compute(Input{StartCapital: 100, Equity: series(day(2021, 1, 1), compound(100, neg)...)})
		if a.Skew == 0 {
			t.Fatal("fixture should be skewed")
		}
		approx(t, "mirrored skew", b.Skew, -a.Skew)
		approx(t, "mirrored kurtosis", b.ExcessKurtosis, a.ExcessKurtosis)
	})
	t.Run("flat curve leaves them at zero", func(t *testing.T) {
		m := Compute(Input{StartCapital: 100, Equity: series(day(2021, 1, 1), 100, 100, 100, 100)})
		if m.Skew != 0 || m.ExcessKurtosis != 0 || m.VaR95 != 0 || m.CVaR95 != 0 || m.WorstDay != 0 || m.BestDay != 0 {
			t.Errorf("flat curve tail stats = %v %v %v %v %v %v, want all 0", m.Skew, m.ExcessKurtosis, m.VaR95, m.CVaR95, m.WorstDay, m.BestDay)
		}
	})
}

func TestCompute_MaxLosingMonths(t *testing.T) {
	tests := []struct {
		name string
		eq   []Point
		want int
	}{
		{
			// Month returns for the observed months: 0 (flat), +, +, -, +, -, -, + -> longest losing run is 2.
			name: "run of two",
			eq: []Point{
				{day(2019, 1, 2), 100}, {day(2019, 6, 28), 110}, {day(2019, 12, 31), 120},
				{day(2020, 3, 31), 90}, {day(2020, 12, 31), 150},
				{day(2021, 6, 30), 135}, {day(2021, 12, 31), 120},
				{day(2022, 2, 15), 132},
			},
			want: 2,
		},
		{"four down months in a row", []Point{
			{day(2021, 1, 29), 100}, {day(2021, 2, 26), 95}, {day(2021, 3, 31), 90}, {day(2021, 4, 30), 85},
			{day(2021, 5, 31), 80}, {day(2021, 6, 30), 90},
		}, 4},
		{"a flat month breaks the streak", []Point{
			{day(2021, 1, 29), 100}, {day(2021, 2, 26), 95}, {day(2021, 3, 31), 95}, {day(2021, 4, 30), 90},
		}, 1},
		{"never down", []Point{{day(2021, 1, 29), 100}, {day(2021, 2, 26), 101}, {day(2021, 3, 31), 102}}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Opening capital 100 so that January's return is measured from 100.
			m := Compute(Input{StartCapital: 100, Equity: tc.eq})
			if m.MaxLosingMonths != tc.want {
				t.Errorf("MaxLosingMonths = %d, want %d", m.MaxLosingMonths, tc.want)
			}
		})
	}
}
