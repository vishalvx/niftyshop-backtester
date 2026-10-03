package costs

import "time"

// Every number in this file was checked against a primary source in the research report (section 5, table A).
// Sources: Finance Acts / Budget memoranda (indiabudget.gov.in), egazette.gov.in Finance (No.2) Act 2024,
// NSE circulars (nsearchives.nseindia.com), SEBI notifications; DP charge and brokerage are broker-set.

// Windows in this study start in 2008; dates before 2007-04-01 (2% cess) and 2006-06-01 (no STT) are not modelled.

// CessRate is the current health and education cess; ratesOn returns the cess in force on a date.
const CessRate = 0.04

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

var (
	grandfatherCutoff = d(2018, 2, 1) // buys before this date get cost stepped up to the 31-Jan-2018 price
	ltcgStart         = d(2018, 4, 1) // LTCG on listed equity taxable from here (s.112A)
	regime2024        = d(2024, 7, 23)
)

// dividendTaxableFromFY: dividends are taxed in the shareholder's hands from FY2020-21 (label 2021).
const dividendTaxableFromFY = 2020

// Rates in force on a date. LTCG < 0 means long-term gains are exempt.
type Rates struct {
	STCG, LTCG, LTExempt, Cess float64
}

func ratesOn(t time.Time, mode TaxMode) Rates {
	if mode == Today || !t.Before(regime2024) {
		return Rates{STCG: 0.20, LTCG: 0.125, LTExempt: 125000, Cess: 0.04}
	}
	cess := 0.04
	if t.Before(ltcgStart) {
		cess = 0.03 // 2% + 1% SHEC until 31 Mar 2018
	}
	switch {
	case t.Before(d(2008, 4, 1)):
		return Rates{STCG: 0.10, LTCG: -1, Cess: cess}
	case t.Before(ltcgStart):
		return Rates{STCG: 0.15, LTCG: -1, Cess: cess}
	default:
		return Rates{STCG: 0.15, LTCG: 0.10, LTExempt: 100000, Cess: cess}
	}
}

func between(t, from, to time.Time) bool { return !t.Before(from) && t.Before(to) }

// statutoryOn returns the statutory charge table in force on a date (NSE cash segment, delivery).
func statutoryOn(t time.Time) Statutory {
	s := Statutory{DPCharge: 15} // DP charge is broker/depository set; Rs 13.5-15.9 across the period
	// Securities Transaction Tax, delivery, each side.
	if t.Before(d(2012, 7, 1)) {
		s.STTBuy, s.STTSell = 0.00125, 0.00125
	} else {
		s.STTBuy, s.STTSell = 0.001, 0.001
	}
	// NSE transaction charge per side (+ IPFT Rs 10/crore from April 2023).
	switch {
	case t.Before(d(2021, 1, 1)):
		s.Exchange = 0.0000325
	case t.Before(d(2023, 4, 1)):
		s.Exchange = 0.0000345
	case t.Before(d(2024, 4, 1)):
		s.Exchange = 0.0000325 + 0.000001
	case t.Before(d(2024, 10, 1)):
		s.Exchange = 0.0000322 + 0.000001
	case t.Before(d(2026, 3, 1)):
		s.Exchange = 0.0000297 + 0.000001
	default:
		s.Exchange = 0.000030699 + 0.000000001 // IPFT Rs 0.01 per crore = 1e-9
	}
	// SEBI turnover fee.
	switch {
	case t.Before(d(2017, 1, 1)):
		s.SEBI = 0.000002
	case t.Before(d(2019, 4, 1)):
		s.SEBI = 0.0000015
	default:
		s.SEBI = 0.000001
	}
	// Stamp duty, buy side. Before July 2020 it varied by state (about 0.01% typical).
	if t.Before(d(2020, 7, 1)) {
		s.StampBuy = 0.0001
	} else {
		s.StampBuy = 0.00015
	}
	// Service tax / GST on brokerage and exchange charges.
	switch {
	case t.Before(d(2009, 2, 24)):
		s.GST = 0.1236
	case t.Before(d(2012, 4, 1)):
		s.GST = 0.103
	case t.Before(d(2015, 6, 1)):
		s.GST = 0.1236
	case t.Before(d(2015, 11, 15)):
		s.GST = 0.14
	case t.Before(d(2016, 6, 1)):
		s.GST = 0.145
	case t.Before(d(2017, 7, 1)):
		s.GST = 0.15
	default:
		s.GST = 0.18
	}
	return s
}
