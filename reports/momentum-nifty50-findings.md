# Momentum rotation on the Nifty 50: findings

Status: results of 2026-10-04 for the study frozen in [`research/studies/momentum-nifty50.md`](../research/studies/momentum-nifty50.md)
(committed before any run). Data: Yahoo prices and NSE index files as described there, Nifty 50 members Feb 2008 to Aug 2025.

## Short answer

**No variant is kept.** The keep rule asks for a typical 5-year result within 2 points a year of the Nifty 50
total-return index *and* ahead of a Nifty200 Momentum 30 index fund, after costs and tax at today's rates.

- Four of the eight variants pass the first test: two hold 20 names without a market filter and beat the index in
  79% of 5-year windows, by a typical +1.1 and +0.8 points a year.
- None passes the second. Every variant trails the momentum fund by 4.5 to 11.1 points a year in the typical window
  and beats it in at most 6% of the 151 windows. The verdicts are the same at the tax rates in force at the time.
- The 200-day market filter is what hurt most: the four filter variants hold cash about a quarter of the time and
  trail the index by 4.3 to 4.8 points a year. Even with idle cash earning 6% a year they trail by 2.9 to 3.4.
- None of the eight is likely to have real skill: the best deflated Sharpe ratio is 0.80 charged for 8 trials and
  0.61 charged for 248, against the usual 0.95 bar.

Under the approved plan this means: on the Nifty 50, hold the index or the momentum fund rather than run our own
rotation. The Midcap 150 study (same frozen variants) is still to run.

## Results

151 five-year windows per variant, one starting on the first of every month from Feb 2008 to Aug 2020. Each window
compares the variant with Rs 10 lakh bought and held in the Nifty 50 TRI, and in the Nifty200 Momentum 30 TRI less a
0.20% a year fund fee, over the same dates. Gaps are the median over the windows of variant CAGR minus benchmark CAGR,
in points a year. After costs and tax at today's rates (20% short-term, 12.5% long-term); the dated-rate gaps are beside.

| Variant | Names | Swaps | Score | Filter | Median CAGR | Gap to index | Gap to fund | Beats index | Beats fund | Gap to index, dated | Gap to fund, dated | Verdict |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `mom-nse-top20-6m` | 20 | half-yearly | NSE-style | none | 12.47% | **+1.10** | -4.54 | 79% | 5% | +1.13 | -4.86 | dropped: behind the fund |
| `mom-plain-top20-1m` | 20 | monthly | plain | none | 12.49% | **+0.76** | -5.50 | 79% | 6% | +0.88 | -5.95 | dropped: behind the fund |
| `mom-plain-top10-6m` (= `rotation-n50`) | 10 | half-yearly | plain | none | 11.74% | **+0.13** | -5.84 | 56% | 0% | +0.13 | -6.07 | dropped: behind the fund |
| `mom-nse-top10-1m` | 10 | monthly | NSE-style | none | 10.00% | **-1.30** | -8.45 | 19% | 2% | -1.37 | -8.45 | dropped: behind the fund |
| `mom-nse-top20-1m-ma200` | 20 | monthly | NSE-style | 200-day | 6.98% | -4.34 | -10.74 | 5% | 3% | -4.53 | -11.48 | dropped: both tests |
| `mom-nse-top10-6m-ma200` | 10 | half-yearly | NSE-style | 200-day | 8.10% | -4.42 | -11.09 | 20% | 3% | -5.08 | -11.73 | dropped: both tests |
| `mom-plain-top10-1m-ma200` | 10 | monthly | plain | 200-day | 6.62% | -4.74 | -10.52 | 5% | 3% | -5.06 | -11.04 | dropped: both tests |
| `mom-plain-top20-6m-ma200` | 20 | half-yearly | plain | 200-day | 8.51% | -4.80 | -10.24 | 23% | 3% | -5.00 | -10.90 | dropped: both tests |

Benchmarks over the same 151 windows, median CAGR after tax at today's rates: Nifty 50 TRI **12.02%**, momentum
fund **19.12%** (dated rates: 13.01% and 20.48%).

The baseline reproduces the earlier study: `mom-plain-top10-6m` is `rotation-n50`, and its median after costs and
dated tax is 12.61%, the figure published before.

### What each choice did

The eight variants are a balanced half of the 16 combinations, so each setting can be compared with the other over
four variants each. Mean of the median gap to the index, after costs and tax at today's rates:

| Choice | Setting A | Setting B | B minus A |
|---|---|---|---|
| Names held | top 10: -2.58 | top 20: -1.82 | +0.76 |
| Swaps | half-yearly: -2.00 | monthly: -2.40 | -0.41 |
| Score | plain: -2.16 | NSE-style: -2.24 | -0.08 |
| Market filter | none: +0.17 | 200-day: -4.57 | **-4.75** |

- **Market filter:** the four filter variants held no stock in 54 to 57 of 211 months (26-27%). Checked only on swap
  days, the filter sold near lows and bought back late: the half-yearly 20-name variant sat in cash from Jan to Jun
  2009 and from Jul to Dec 2020, both strong rebounds. It cut the median worst fall from about 21% to 16-20% but
  cost 4.7 points a year.
- **Names held:** 20 names did a little better than 10 (less luck from a single stock).
- **Monthly swaps** cost 0.4 points: more trading (516 to 878 purchases over 17.6 years against 195 to 335 for
  half-yearly) means more charges and more short-term tax, for no better picks.
- **Score:** the NSE-style volatility adjustment made no difference on the Nifty 50.

### By start year

Median gap to the Nifty 50 TRI of the windows starting in each year (after costs and tax at today's rates, points):

| Variant | 08 | 09 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Index CAGR, % | 5.5 | 12.1 | 10.1 | 8.5 | 12.3 | 12.8 | 10.3 | 6.4 | 14.0 | 12.6 | 11.8 | 15.0 | 18.1 |
| `mom-nse-top20-6m` | +1.8 | +2.6 | +1.4 | +0.6 | +0.0 | -0.9 | -0.3 | +1.5 | +2.2 | +1.2 | +0.9 | +3.2 | -0.2 |
| `mom-plain-top20-1m` | +2.8 | +2.5 | +1.1 | +1.2 | +0.3 | -0.0 | +0.8 | +0.5 | +0.3 | +0.5 | -0.4 | +2.1 | +1.0 |
| `mom-plain-top10-6m` | -0.5 | +0.8 | +0.0 | +0.2 | +0.1 | -0.4 | +2.0 | +2.6 | +0.4 | -2.4 | -2.0 | +1.4 | -2.4 |
| `mom-nse-top10-1m` | +0.0 | -0.3 | -1.2 | -1.1 | -1.7 | -1.4 | +0.0 | +1.2 | -0.8 | -4.5 | -5.8 | -3.4 | -5.6 |

The two 20-name variants without a filter were never more than 0.9 points behind the index in any start year, so
their lead over the index is steady. Against the momentum fund they were behind in every start year except 2008
(`mom-plain-top20-1m`, +1.1); the full start-year tables for all eight are in `research/out/momentum-n50/tables.md`.

## Why the fund is so far ahead, and is that fair?

- **Wider choice.** The fund picks 30 names from the Nifty 200; our rotation picks from the Nifty 50. Momentum is
  stronger among mid-sized companies, which the Nifty 50 does not hold.
- **Most of its history is back-calculated.** NSE launched the Nifty200 Momentum 30 in Aug 2020; earlier values are
  NSE's reconstruction. Only one of the 151 windows (Aug 2020 to Aug 2025) uses live values throughout. In that
  window, after tax at today's rates, the fund made 19.86%, the index 17.04% and the best variant
  (`mom-plain-top20-1m`) 17.79%, so the fund still led by 2.1 points.
- **Fee:** 0.20% a year is the cheapest direct plan found (Kotak Nifty 200 Momentum 30 Index Fund, Aug 2026
  factsheet). A higher fee would not change any verdict: the closest variant trails by 4.5 points.
- **Tax:** the fund is taxed only when sold (as a growth-option fund held for the whole window); the rotation pays
  tax on every swap. That is a real cost of running the rotation ourselves, not a flaw in the comparison.

## Is any of it more than luck?

Deflated Sharpe ratio (the chance the true Sharpe ratio is above zero after allowing for the number of variants
tried), whole period Feb 2008 to Aug 2025, daily returns after costs, risk-free rate 6% a year:

| Variant | Sharpe a year | DSR, 8 trials | DSR, 248 trials |
|---|---|---|---|
| `mom-plain-top20-1m` | 0.35 | 0.80 | 0.61 |
| `mom-nse-top20-6m` | 0.32 | 0.77 | 0.56 |
| `mom-plain-top10-6m` | 0.25 | 0.67 | 0.46 |
| `mom-plain-top10-1m-ma200` | 0.24 | 0.65 | 0.43 |
| `mom-nse-top10-1m` | 0.23 | 0.64 | 0.42 |
| `mom-nse-top20-1m-ma200` | 0.23 | 0.64 | 0.41 |
| `mom-plain-top20-6m-ma200` | 0.08 | 0.40 | 0.20 |
| `mom-nse-top10-6m-ma200` | 0.07 | 0.38 | 0.20 |

- **Trial count:** 8 in this study; 248 counts the 240 trials of the NiftyShop study too, because momentum was
  picked after those failed. No variant reaches 0.95 on either count.
- For scale, on the same days and the same basis the Nifty 50 TRI has a Sharpe ratio of 0.31 a year and the
  momentum fund (back-calculated before 2020) 0.48.
- **Walk-forward:** picking the variant with the best Sharpe over the previous 6 years and holding it for the next 2
  gave 11.89% a year over 11.6 unseen years after costs and dated tax, against 13.85% for the index; the pick beat
  the index in 3 of 6 test windows.

## Deviations from the frozen plan

- The plan said 9 Nifty 50 members have no price history. The run reports 11 of the period's 100 members without
  usable prices: CAIRN, HDFC, IDFC, JPASSOCIAT, LTIM, RANBAXY, RELCAPITAL, RPL, SATYAMCOMP, SESAGOA and STER. They can
  never be picked. Nothing else changed.

## Caveats

- **Prices:** Yahoo prices miss the 11 members above (mostly merged, renamed or delisted companies) and carry some
  unadjusted corporate actions. Official NSE prices including dead stocks are being built; these results may move
  either way once they land (a missing stock could have been a pick that later failed, or one that kept rising).
- **One market, one period:** 17.6 years of the Nifty 50, a large-company index.
- **Execution:** fills at the swap-day close with 10 bps slippage and statutory charges; no brokerage.

## Reproduce

```bash
python3 research/py/fetch_tri.py "NIFTY 50" "NIFTY200 MOMENTUM 30"   # plus the Yahoo prices, see README
go build -o /tmp/research_final ./cmd/research
zsh research/run_momentum.sh
```

Outputs in `research/out/momentum-n50/`: `lottery_5y.md` and `lottery_nifty50_5y.csv` (every window),
`tables.md` (verdicts, choice effects, start-year tables, cash sensitivity), `wf_trials8.md` and `wf_trials248.md`
(walk-forward and deflated Sharpe), `score.txt` (full-period scorecards), and `monthly/` (month by month against the
Nifty 50 TRI and against the momentum fund).
