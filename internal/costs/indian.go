package costs

import "time"

// Statutory holds the dated statutory charges as fractions of traded value (see rates.go for sources).
type Statutory struct {
	STTBuy, STTSell float64
	Exchange        float64 // NSE transaction charge, both sides
	SEBI            float64 // SEBI turnover fee, both sides
	StampBuy        float64 // stamp duty, buy side only
	GST             float64 // on brokerage + exchange + SEBI charges
	DPCharge        float64 // rupees per scrip per sell day, GST included
}

// IndianDelivery prices equity DELIVERY trades on NSE.
type IndianDelivery struct {
	BrokerageBps float64 // per side, basis points of value; 0 for a zero-brokerage discount broker
	BrokerageMin float64 // rupees per order floor, applied only when BrokerageBps > 0
	Statutory    bool    // include STT, exchange, SEBI, stamp, GST, DP
}

func (m IndianDelivery) side(date time.Time, value float64, buy bool, dp bool) Charge {
	var total, deductible float64
	brk := value * m.BrokerageBps / 1e4
	if m.BrokerageBps > 0 && brk < m.BrokerageMin {
		brk = m.BrokerageMin
	}
	total += brk
	deductible += brk
	if m.Statutory {
		st := statutoryOn(date)
		exch := value * st.Exchange
		sebi := value * st.SEBI
		gst := (brk + exch + sebi) * st.GST
		var stt, stamp float64
		if buy {
			stt, stamp = value*st.STTBuy, value*st.StampBuy
		} else {
			stt = value * st.STTSell
		}
		fees := exch + sebi + gst + stamp
		total += fees + stt
		deductible += fees // STT is not deductible against capital gains
		if !buy && dp {
			total += st.DPCharge
			deductible += st.DPCharge
		}
	}
	return Charge{Total: total, Deductible: deductible}
}

// Buy implements Model.
func (m IndianDelivery) Buy(date time.Time, value float64) Charge {
	return m.side(date, value, true, false)
}

// Sell implements Model.
func (m IndianDelivery) Sell(date time.Time, value float64, firstSellOfScripToday bool) Charge {
	return m.side(date, value, false, firstSellOfScripToday)
}
