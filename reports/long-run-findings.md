# Long-run findings: do the NiftyShop strategies beat the index?

Status: results of the complete backtest of 2026-10-03. Supersedes the performance claims in `nifty-shop-v-pivot.md` and `reports/pivot_comparison_report.md`, which came from a data-loader bug (see "What was wrong before").

## Short answer

**No variant clears 15% a year after costs and tax over the long run, and none reliably beats simply holding the index after tax.**

- Nifty 50, Feb 2008 to Aug 2025: the typical 5-year result after costs and tax is **6.3% to 8.2% a year**, against **13.0%** for the Nifty 50 total-return index. The best of six variants beats the index in **24.5%** of 5-year windows. The bar set in advance was 60%.
- Midcap list, Feb 2019 to Jun 2026: the typical 5-year result is **15.2% to 18.9%**, against **26.0%** for the Midcap 50 total-return index. The best variant beats it in **3.5%** of windows.
- The Midcap list in the dataset is mostly Nifty 50 names and never removes a stock, and the period is one bull market, so it is a weak test. Against the Nifty 50 index the best Midcap-list variant wins 76% of windows, which says more about the list than about the strategy.

## Where the 13% for the index comes from

It is the **median 5-year result of buying the Nifty 50 total-return index (TRI) on the first day of every month from Feb 2008** and selling five years later: 151 start months, taxed as a long-term capital gain once, at the sale. It is a typical 5-year holding, not the whole-history figure. Over the whole 17.6 years from Feb 2008 the index returns 10.4% a year before tax and **9.9% after tax** (Rs 10 lakh grows to Rs 56.8 lakh gross; tax Rs 4.19 lakh). The strategies are measured the same way: one start per month, 5-year window, after costs and tax, median of the windows. Index tax uses the same tax code as the strategy lots: long-term gain, exempt before April 2018, 10% above Rs 1 lakh until July 2024, 12.5% above Rs 1.25 lakh after, plus cess, with the 31 Jan 2018 cost step-up.

## What was tested

- **Rules** (confirmed rule set R1 to R8, presets `nsx` and `nsx-lot`):
  1. Universe: stocks of the index (point-in-time monthly lists), one check a day near the close.
  2. New stock: the one furthest below its 20-day average that is not held; no cap on the number of stocks held (cash only).
  3. Size of a buy: capital / 10, where capital is the start money plus profit booked, refreshed after every sale.
  4. Add: when the close is 3% or more below the latest open lot; at most 3 open lots per stock.
  5. Sell: the whole position at +5% over average cost, or (variant) each lot at +5% over its own entry.
  6. A stock that leaves the index is sold that day at any price.
  7. No stop loss and no time limit.
  8. Order of the day: any number of exits first, then at most one purchase a day in total (an add before a new stock).
- **Six variants**: Standard, Camarilla S1 pool 5, Fibonacci S1 pool 15, each with the whole-position exit or an own target per lot.
- **Start of the index only**: Nifty 50 from 2008-02-01 (first tradable month) to 2025-08-31; Midcap list from 2019-02-01 to 2026-06-30.
- **No look-ahead in membership**: trades in a month use the previous month's list (lag 1). The engine's older convention (a month-end list applied to the whole month it is dated in, lag 0) is kept as a sensitivity run.
- **Four accounting stages**: before costs, after costs (statutory charges plus 10 bps slippage a side), after costs and tax at dated rates, after costs and tax at today's rates.
- **Evidence**: one full run per variant (every metric of the scorecard), 3/5/10-year start-date windows (one start per month), harsh costs, capital jitter, walk-forward over the six variants, deflated Sharpe ratio with 240 trials counted.

## Results (lag 1)

After costs and tax at dated rates, 5-year start-month windows; the single-run column is one run after costs only.

| Variant | Nifty 50 one run after costs (CAGR, worst fall) | 5-yr median | 5-yr 10th pct | 5-yr beats Nifty 50 TRI | 10-yr median | Midcap list one run after costs (CAGR, worst fall) | 5-yr median | beats Midcap 50 TRI | beats Nifty 50 TRI |
|---|---|---|---|---|---|---|---|---|---|
| Standard, whole exit | 9.1%, -58% | 8.23% | 0.61% | 18.54% | 8.46% | 17.1%, -40% | 17.28% | 0.00% | 65.52% |
| Standard, own targets | 3.5%, -72% | 7.31% | -2.55% | 17.22% | 6.82% | 15.4%, -38% | 16.68% | 0.00% | 62.07% |
| Camarilla S1/5, whole exit | 12.3%, -54% | 6.28% | -2.56% | 16.56% | 6.83% | 13.9%, -39% | 15.20% | 0.00% | 37.93% |
| Camarilla S1/5, own targets | 9.0%, -63% | 8.13% | -0.44% | 24.50% | 8.91% | 17.6%, -45% | 18.49% | 3.45% | 62.07% |
| Fibonacci S1/15, whole exit | 10.0%, -55% | 8.20% | -1.71% | 23.18% | 6.72% | 10.9%, -44% | 18.87% | 0.00% | 75.86% |
| Fibonacci S1/15, own targets | 8.7%, -50% | 7.17% | -4.46% | 19.87% | 7.41% | 19.5%, -41% | 18.59% | 0.00% | 75.86% |
| Index, buy and hold (after tax) | 9.9% (Nifty 50 TRI) | 13.01% | 6.76% | - | 12.20% | - | 25.96% (Midcap 50) | - | - |

Notes:

- A single full-history run is path luck. Fixing two tiny bugs (below) moved single Nifty 50 runs by up to 5 points a year but moved the 5-year medians by under 1 point. Judge by the windows.
- Over the whole history one Nifty 50 run after costs and dated tax ranges from 1.5% to 9.6% a year across the six variants, against 9.9% for the index.
- Membership timing: lag 0 against lag 1 changes single Nifty 50 runs by several points in both directions, but the best variant beats the index in 24.5% of windows at either lag.
- Walk-forward over the six variants (Nifty 50, train 6 years, test 2 years): 2.4% a year against 13.8% for the index; the pick beat the index in 1 of 6 test windows. Midcap list (train 3, test 1): 14.8% against 16.8%, 3 of 4 test years beaten. Best deflated Sharpe ratios 0.60 and 0.81 against the usual 0.95 bar.
- Harsh costs (0.10% brokerage and 25 bps slippage a side) cut the 5-year median by 0.9 to 4.7 points.
- The full scorecards, the start-date tables and the robustness tables are in `research/out/final/complete/` (`lag1/` is the headline, `lag0/` the sensitivity). A rendered version is `reports/long-run-results.html`.

## Is the code right?

- A second simulator written only from the rules (`research/py/ref_sim.py`) matches the Go simulator lot for lot on 40 of 40 runs (`research/py/verify_sim.sh`), including all six variants at the headline setting.
- A separate line-by-line review against R1 to R8 found all eight hold, and added `internal/sim/confirmed_rules_test.go` (property tests on 160 random panels, hand-built scenarios for R4, R6 and R8, and controls that break a rule to prove the checker notices).
- Two small bugs were found and fixed before the final run, both with regression tests: a buy made with cash just above one slot could take cash below zero by the buy charges (worst Rs 216 on Rs 10 lakh; the share count now fits the cash including charges), and membership lag 1 read the same month on the 31st after a 30-day month (the lookup key is now built from the first of the month).
- One rule is not specified and was not changed: when several held stocks are below their latest lot on the same day, only one add is allowed, and the simulator picks the stock whose lot was bought most recently. It decided the day's add on 36 to 117 days in each 17-year run. "Deepest below its lot first" has not been run.

## What was wrong before

The earlier recommendation in `nifty-shop-v-pivot.md` (for example 16.01% a year for Camarilla S1 pool 5 on the Nifty 50, 2018 to 2025) cannot be reproduced. The old engine's data loader read every cached price file in a folder and kept only the file that sorts last for each stock. With the cache files used for the published table, 31 Nifty 50 stocks had prices from April 2024 only, so the published numbers describe a different, much smaller data set (fixed in this branch: all files of a symbol are merged, with a test). The old engine also applied no costs and no tax, and the written strategy limited holdings to 5 stocks, which the confirmed rules do not.

## Limits

- Nine Nifty 50 members have no price history and can never be bought (about 12% of member-months in 2008, 10% in 2009 to 2013, 2% in 2016 to 2023). This biases every Nifty 50 result upward.
- The Midcap list is not point-in-time (12 additions and 1 removal in 7.5 years), so the index-exit rule never fires there. Smallcap 50 and Midcap 150 have no usable constituent file.
- Nothing before 2008 can be tested. The first 33 months of the Nifty 50 list are a frozen list.
- Fills are at the close; a next-open fill costs about 2 points a year. Tax assumes capital-gains treatment; a high-turnover trader may be taxed as business income.
- Windows overlap, so they are far fewer independent observations than they look.

## Reproduce

```
go build -o /tmp/research_final ./cmd/research
zsh research/run_complete.sh        # lag 1 and lag 0, both lists, all stages and tests
zsh research/py/verify_sim.sh       # Go simulator against the Python reference
go test ./...
```

Price and index data come from `.research-data/` (gitignored; see `research/py/fetch_tri.py`, `research/py/fetch_yahoo.py` and the data audit scripts).

## Suggested next steps

1. Stop treating 15% a year, or beating the index, as the expected outcome of these strategies. Use them, if at all, as a rules-based discipline with an honest cost and tax drag.
2. Decide the "which add first" rule and rerun only if the answer differs from the current one.
3. Repair the data before tuning anything: price history for the nine missing Nifty 50 members, and real point-in-time Midcap 50, Midcap 150 and Smallcap 50 constituent files.
4. Only then look at improvements (exits, filters, position sizing), judged by the same windows and the same 60%-of-windows bar, with every trial counted.
