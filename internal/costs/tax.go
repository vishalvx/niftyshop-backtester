package costs

import (
	"sort"
	"time"
)

// Sale is one realised lot. Cost and Proceeds are already net of deductible charges.
type Sale struct {
	Symbol   string
	BuyDate  time.Time
	SellDate time.Time
	Qty      float64
	Cost     float64
	Proceeds float64
}

// TaxMode selects which rate table applies.
type TaxMode int

const (
	// Dated applies the law in force on each sale date (STCG 15%/20%, LTCG exempt/10%/12.5% ...).
	Dated TaxMode = iota
	// Today applies the regime in force since 23 July 2024 to every sale (forward-looking view).
	Today
)

// TaxBook accumulates realised gains and computes tax per financial year (April-March).
type TaxBook struct {
	Mode TaxMode
	// FMV31Jan2018 returns the market value per share on 31 Jan 2018 for LTCG grandfathering (Dated mode only).
	FMV31Jan2018 func(symbol string) (float64, bool)
	// DividendTaxRate applies to dividends received on/after 1 Apr 2020 (Dated) or always (Today); 0 disables.
	DividendTaxRate float64

	sales     map[int][]Sale
	divs      map[int]float64
	carryST   []carry // unabsorbed short-term capital loss by FY of origin
	carryLT   []carry
	TotalPaid float64
}

type carry struct {
	fy  int
	amt float64
}

// NewTaxBook returns an empty book.
func NewTaxBook(mode TaxMode) *TaxBook {
	return &TaxBook{Mode: mode, sales: map[int][]Sale{}, divs: map[int]float64{}}
}

func fyOf(d time.Time) int {
	if d.Month() >= time.April {
		return d.Year() + 1
	}
	return d.Year()
}

// RecordSale adds a realised sale.
func (t *TaxBook) RecordSale(s Sale) {
	fy := fyOf(s.SellDate)
	t.sales[fy] = append(t.sales[fy], s)
}

// RecordDividend adds dividend income received on date d.
func (t *TaxBook) RecordDividend(d time.Time, amt float64) {
	t.divs[fyOf(d)] += amt
}

func isLongTerm(buy, sell time.Time) bool { return sell.After(buy.AddDate(0, 12, 0)) }

// gain returns the taxable gain of a sale after LTCG grandfathering (Dated mode).
func (t *TaxBook) gain(s Sale) (g float64, long bool) {
	long = isLongTerm(s.BuyDate, s.SellDate)
	cost := s.Cost
	if long && t.Mode == Dated && t.FMV31Jan2018 != nil && s.BuyDate.Before(grandfatherCutoff) && !s.SellDate.Before(ltcgStart) && s.Qty > 0 {
		if fmv, ok := t.FMV31Jan2018(s.Symbol); ok {
			unitSell := s.Proceeds / s.Qty
			floor := fmv
			if unitSell < floor {
				floor = unitSell
			}
			if gf := floor * s.Qty; gf > cost {
				cost = gf
			}
		}
	}
	return s.Proceeds - cost, long
}

type bucket struct{ rate, amt float64 }

// taxFor computes tax for a set of sales given carried-forward losses, returning tax and updated carries.
// If commit is false the book's carry state is left unchanged.
func (t *TaxBook) taxFor(fy int, sales []Sale, divs float64, commit bool) float64 {
	var stB, ltB []bucket
	exempt := 0.0
	stIdx := map[float64]int{}
	ltIdx := map[float64]int{}
	for _, s := range sales {
		g, long := t.gain(s)
		r := ratesOn(s.SellDate, t.Mode)
		if long {
			if r.LTCG < 0 { // exempt
				continue
			}
			if e := r.LTExempt; e > exempt {
				exempt = e
			}
			eff := r.LTCG * (1 + r.Cess)
			if i, ok := ltIdx[eff]; ok {
				ltB[i].amt += g
			} else {
				ltIdx[eff] = len(ltB)
				ltB = append(ltB, bucket{eff, g})
			}
		} else {
			eff := r.STCG * (1 + r.Cess)
			if i, ok := stIdx[eff]; ok {
				stB[i].amt += g
			} else {
				stIdx[eff] = len(stB)
				stB = append(stB, bucket{eff, g})
			}
		}
	}
	// absorb reduces positive gains in buckets (highest rate first) by loss; returns remaining loss.
	absorb := func(b []bucket, loss float64) float64 {
		sort.Slice(b, func(i, j int) bool { return b[i].rate > b[j].rate })
		for i := range b {
			if loss <= 0 {
				break
			}
			if b[i].amt > 0 {
				use := b[i].amt
				if use > loss {
					use = loss
				}
				b[i].amt -= use
				loss -= use
			}
		}
		return loss
	}
	// Current-year set-off. Losses first net against gains in the same category (this also nets across the
	// 23-Jul-2024 rate change), a leftover short-term loss then offsets long-term gains, a leftover long-term loss
	// can only be carried forward.
	takeLoss := func(b []bucket) float64 {
		loss := 0.0
		for i := range b {
			if b[i].amt < 0 {
				loss += -b[i].amt
				b[i].amt = 0
			}
		}
		return loss
	}
	stLoss, ltLoss := takeLoss(stB), takeLoss(ltB)
	stLoss = absorb(stB, stLoss)
	ltLoss = absorb(ltB, ltLoss)
	if stLoss > 0 {
		stLoss = absorb(ltB, stLoss)
	}
	newCarryST, newCarryLT := stLoss, ltLoss
	// Brought-forward losses (8 assessment years).
	var keepST, keepLT []carry
	for _, c := range t.carryST {
		if fy-c.fy > 8 {
			continue
		}
		rem := absorb(stB, c.amt)
		if rem > 0 {
			rem = absorb(ltB, rem)
		}
		if rem > 0 {
			keepST = append(keepST, carry{c.fy, rem})
		}
	}
	for _, c := range t.carryLT {
		if fy-c.fy > 8 {
			continue
		}
		rem := absorb(ltB, c.amt)
		if rem > 0 {
			keepLT = append(keepLT, carry{c.fy, rem})
		}
	}
	if newCarryST > 0 {
		keepST = append(keepST, carry{fy, newCarryST})
	}
	if newCarryLT > 0 {
		keepLT = append(keepLT, carry{fy, newCarryLT})
	}
	if commit {
		t.carryST, t.carryLT = keepST, keepLT
	}
	tax := 0.0
	for _, b := range stB {
		if b.amt > 0 {
			tax += b.amt * b.rate
		}
	}
	// LTCG exemption (applied against the highest-rate gains first).
	remEx := exempt
	sort.Slice(ltB, func(i, j int) bool { return ltB[i].rate > ltB[j].rate })
	for _, b := range ltB {
		if b.amt <= 0 {
			continue
		}
		use := b.amt
		if remEx > 0 {
			cut := remEx
			if cut > use {
				cut = use
			}
			use -= cut
			remEx -= cut
		}
		tax += use * b.rate
	}
	if t.DividendTaxRate > 0 && divs > 0 && (t.Mode == Today || fy > dividendTaxableFromFY) {
		tax += divs * t.DividendTaxRate
	}
	return tax
}

// SettleFY computes and commits the tax for a financial year (label = calendar year in which it ends).
func (t *TaxBook) SettleFY(fy int) float64 {
	due := t.taxFor(fy, t.sales[fy], t.divs[fy], true)
	t.TotalPaid += due
	delete(t.sales, fy)
	delete(t.divs, fy)
	return due
}

// TerminalTax is the tax that would arise if the given open lots were all sold on their SellDate, on top of
// sales already recorded in that financial year. It does not change the book.
func (t *TaxBook) TerminalTax(open []Sale) float64 {
	if len(open) == 0 {
		return 0
	}
	fy := fyOf(open[0].SellDate)
	with := append(append([]Sale{}, t.sales[fy]...), open...)
	// current-year recorded sales have already been settled by the caller; compute the increment on a clean carry.
	return t.taxFor(fy, with, 0, false) - t.taxFor(fy, t.sales[fy], 0, false)
}
