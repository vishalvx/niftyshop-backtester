# Study plan: momentum rotation on the Nifty 50

Status: **frozen on 2026-10-04, before any of the runs below.** This file is committed before the code that runs the
variants, so the variant list, the measures and the keep rule cannot be changed after seeing results. The findings go
in `reports/momentum-nifty50-findings.md`; any change to this plan after the first run is listed there as a deviation.

## Question

Does any simple momentum rotation on the Nifty 50 earn a typical 5-year result within 2 points a year of the Nifty 50
total-return index, and ahead of a Nifty200 Momentum 30 index fund, after costs and tax?

The starting point is `rotation-n50` (mean of the 6- and 12-month price return, top 10 of the Nifty 50, equal weight,
swaps on the first trading day of January and July). In the long-run study its typical 5-year result after costs and
tax was 12.61% against 12.99% for the index.

## Variants (8, frozen)

Four choices, two settings each:

| Choice | Setting A | Setting B |
|---|---|---|
| Names held | top 10 | top 20 |
| Swap days | half-yearly: first trading day of January and July | monthly: first trading day of every month |
| Score | plain: mean of the 6- and 12-month price return | NSE-style: volatility-adjusted z-score (below) |
| Market filter | none | 200-day: hold cash while the Nifty 50 is below its 200-day average (below) |

All 16 combinations would double the trial count for little extra knowledge. The 8 below are a balanced half
(a 2^(4-1) design, filter = names x swaps x score): each setting of each choice appears in exactly 4 variants, and every
pair of settings of two choices appears in exactly 2. Variant 1 is `rotation-n50` unchanged, and variant 6 is the
closest to how NSE builds the Nifty200 Momentum 30 (its own score, half-yearly).

| # | Preset | Names | Swaps | Score | Market filter |
|---|---|---|---|---|---|
| 1 | `mom-plain-top10-6m` | 10 | half-yearly | plain | none |
| 2 | `mom-plain-top20-6m-ma200` | 20 | half-yearly | plain | 200-day |
| 3 | `mom-plain-top10-1m-ma200` | 10 | monthly | plain | 200-day |
| 4 | `mom-plain-top20-1m` | 20 | monthly | plain | none |
| 5 | `mom-nse-top10-6m-ma200` | 10 | half-yearly | NSE-style | 200-day |
| 6 | `mom-nse-top20-6m` | 20 | half-yearly | NSE-style | none |
| 7 | `mom-nse-top10-1m` | 10 | monthly | NSE-style | none |
| 8 | `mom-nse-top20-1m-ma200` | 20 | monthly | NSE-style | 200-day |

## Rules common to all variants

- Universe: the stocks that were Nifty 50 members that month (`internal/data/nifty50_weights.csv`), with a one-month
  lag: trades in month M use the list published for month M-1. A stock is ranked only if it has a price at least
  12 months before the swap day. Members without any price history cannot be ranked (9 such stocks, see caveats).
- Plain score: the mean of the 6-month and 12-month price return, each from the last close on or before the date
  6 (12) calendar months before the swap day to the swap-day close.
- NSE-style score, after the published Nifty200 Momentum 30 method: momentum ratio = the 6-month (12-month) price
  return divided by the annualised standard deviation of daily log returns over the past 12 months; each ratio is
  turned into a z-score across the ranked members (minus their mean, divided by their standard deviation); the score is
  50% of the 6-month z-score plus 50% of the 12-month z-score. NSE then turns this into a "normalised" score and tilts
  free-float weights by it; that transform does not change the order, and we hold equal weights, so only the order is
  used. NSE's buffer rule (keeping a member until it falls well below the cut) and its weight caps are not modelled.
- On each swap day: rank, sell every held stock that is not in the top N (a stock that left the index is never in the
  top N, so it is sold at the next swap day), then split the cash equally across the new names. Names kept are not
  trimmed back to equal weight (that would only add charges and tax). The first trading day of a run is also a swap day.
- 200-day market filter, checked only on swap days: if the Nifty 50 price index closed below its 200-trading-day
  simple average on its last value on or before the swap day, sell everything and hold cash until the next swap day on
  which it is above. Index values: NSE's Nifty 50 price index (`.research-data/indices/NIFTY_50_PRI.json`).
- Fills: the swap-day close, plus 10 bps adverse slippage a side, plus Indian statutory charges (STT, exchange,
  SEBI, stamp duty, GST) at the rates in force on the day; zero brokerage (the `base` cost scenario).
- Dividends are credited in cash on the ex-date and taxed at 31.2%.
- Idle cash earns nothing, as in every earlier run.
- Tax: every sale is taxed as a capital gain under the rules in force on its date (short-term below 12 months:
  15% until 22 Jul 2024, 20% after; long-term above Rs 1 lakh or Rs 1.25 lakh: exempt before Apr 2018, then 10%,
  12.5% after 22 Jul 2024; 31 Jan 2018 step-up), and again at today's rates for every year (20% short-term, 12.5%
  long-term above Rs 1.25 lakh). Tax is settled at each financial-year end; at the end of a window, the tax due on
  selling every open lot is taken off.

## Windows and benchmarks

- Data: current Yahoo prices in `.research-data/yahoo` (downloaded with `research/py/fetch_yahoo.py`) and the
  NSE index files from `research/py/fetch_tri.py "NIFTY 50" "NIFTY200 MOMENTUM 30"`.
- Period: 2008-02-01 (first month the lagged member list allows) to 2025-08-31 (last member list).
- Every 5-year window starting on the first day of each month from Feb 2008 to Aug 2020: 151 windows per variant.
- Same dates for both sides of every comparison:
  - Index: Rs 10 lakh in the Nifty 50 total-return index (TRI) on the window's first day, held, sold on its last day,
    taxed as a single capital gain (growth-option index fund).
  - Momentum fund: the same in the Nifty200 Momentum 30 TRI less a fund fee of **0.20% a year**, taken daily (the
    direct-plan expense ratio of the Kotak Nifty 200 Momentum 30 Index Fund, 31 Aug 2026 factsheet; the cheapest
    fund found, so the strictest bar). Taxed the same way as the index. The index launched in Aug 2020; before that
    its values are NSE's back-calculation.
- Stages reported: after costs (no tax), after costs and tax at dated rates, after costs and tax at today's rates.

## Keep rule (from the approved plan)

A variant is **kept** only if both hold, after costs and tax at **today's rates** (what a new investor pays from now):

1. Typical gap to the index: the median over the 151 windows of (variant CAGR minus Nifty 50 TRI CAGR, same dates)
   is -2.0 points a year or better.
2. Ahead of the momentum fund: the median over the 151 windows of (variant CAGR minus fund CAGR, same dates) is
   above zero.

The same two numbers at dated rates are reported beside them; if the verdict differs between the two tax bases, the
findings say so. Median CAGRs, the 10th percentile, the share of windows beating each benchmark and the median worst
fall are reported as context, not as part of the rule.

## Trial count and deflated Sharpe

- Trials in this study: 8 (the variants above; variant 1 is `rotation-n50`, already counted once, not counted again).
- The deflated Sharpe ratio (Bailey and Lopez de Prado) of each variant over the whole period, daily returns after
  costs, risk-free 6% a year, charged for N = 8 trials, and again for N = 248 (the 240 trials counted in the long-run
  NiftyShop study plus these 8, because momentum was picked after those failed). The bar is 0.95.
- A walk-forward over the 8 (pick the best Sharpe after costs on 6 years, run it on the next 2 unseen years, after
  costs and dated tax) shows whether picking a variant from the past would have worked.

## Other outputs

- One full-period run of each variant writes the month-by-month table against the Nifty 50 TRI
  (`cmd/research` `score`, https://github.com/vishalvx/niftyshop-backtester/pull/10), and again against the momentum fund.
- Sensitivity, not part of the keep rule: the four filter variants again with idle cash earning 6% a year untaxed
  (an upper bound for a liquid fund), because the headline gives filter variants no return while in cash. If this
  changes a verdict, the findings say so.
- Start-year table: median gap to the index of the 12 windows starting in each year.

## Caveats known before running

- Prices are Yahoo's: 9 Nifty 50 members of the period have no usable price history (mostly stocks that were
  delisted, merged or renamed), so they can never be picked. A worker is building official NSE prices including dead
  stocks; results may move once those land.
- The momentum fund holds 30 of the Nifty 200 and the variants pick from the Nifty 50, so the fund has a wider choice.
