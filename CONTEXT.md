# Project Context & Index Metadata

This project is a Go-based backtester evaluating mechanical mean-reversion strategies (Standard NiftyShop & NiftyShop V-Pivot Variant) on Indian stock indices.

## 1. Supported Indices & Data Gates

`go run ./cmd/research <subcommand> -universe <id> -start <date> -end <date>` picks the index with `-universe`. Membership comes from the constituents CSVs in `internal/data/`; the window must sit inside a CSV's range, because the CLI does not move `-start` for you.

| Index Name | CLI Universe Identifier | Actual Index Inception | Backtest Inception Date (Data Gate) | Historical Constituents CSV | Minimum Backtest Year (CSV Boundary) | Rebalancing Schedule |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Nifty 50** | `nifty50` | April 22, 1996 | **2008-01-01** (Data start) | `nifty50_weights.csv` | **2008** | Semi-annual (March/Sept) |
| **Nifty Midcap 50** | `niftymidcap50` | January 1, 2004 | **2004-01-01** (Inception) | `niftymidcap50_weights.csv` | **2019** (CSV limit) | Semi-annual (March/Sept) |
| **Nifty Smallcap 50** | `niftysmallcap50` | April 1, 2016 | **2016-04-01** (Inception) | `niftysmallcap50_weights.csv` | **2019** (CSV limit) | Semi-annual (March/Sept) |
| **Nifty500 Momentum 50** | `nifty500momentum50` | June 4, 2024 | **2024-06-04** (Inception) | `nifty500momentum50_weights.csv` | **2024** | Semi-annual (June/Dec) |

> [!NOTE]
> Even though **Nifty Midcap 50** and **Nifty Smallcap 50** have inception dates in 2004 and 2016, historical constituent weights CSVs are only available from **2019-01-31** onwards. Therefore, backtests before 2019 for these indices find no constituents and make no trades.

---

## 2. Core Architecture & Key Packages

* **`cmd/research`** ([main.go](cmd/research/main.go)): the command-line entrypoint. Each subcommand (`score`, `lottery`, `jitter`, `wfvariants`, `sweep`, `audit`, `repro`, ...) builds a data panel for one universe and window, runs named rule presets and prints tables.
* **`internal/panel`** ([panel.go](internal/panel/panel.go)): loads one canonical Yahoo CSV per ticker with dividends, applies symbol aliases and quarantines, and builds month-by-month index membership from the constituents CSVs.
* **`internal/sim`** ([sim.go](internal/sim/sim.go)): the day-by-day NiftyShop simulator. `Rules` holds every rule variant; `LegacyRules()` reproduces the retired original engine, `SpecRules()` the written strategy.
* **`internal/experiment`**: rule presets, cost and tax stages, start-date lotteries, walk-forward and the deflated Sharpe ratio.
* **`internal/costs`**: dated Indian brokerage, STT, stamp duty and capital-gains tax.
* **`internal/bench`**: total-return index (TRI) benchmarks.
* **`internal/analytics`**: strategy metrics (CAGR, drawdown, Sharpe, Sortino, alpha and more).
* **`internal/indicators`**: Simple Moving Average and pivot support levels (Classic, Fibonacci, Camarilla).
* **`research/`**: study scripts, rule files and an independent Python reference simulator (`research/py/ref_sim.py`).

The original engine (`cmd/backtester`, `internal/engine`, `internal/metrics`, `internal/portfolio`, `internal/config`, the `internal/data` loader, `run_all.sh` and `run_pivot_comparison.py`) was deleted in October 2026. It did not follow the written rules, counted every sale as a win, and its published numbers came from a data-loader bug.

---

## 3. Strategy Configurations

This project supports two core strategy modes:
1. **Standard NiftyShop**:
   - Filter stocks trading below their 20DMA.
   - Rank candidates by `DiffSMA` descending (most oversold first).
   - Enter the top candidate.
2. **NiftyShop V-Pivot (Support Filter Variant)**:
   - Rank candidates below 20DMA by `DiffSMA` descending.
   - Filter down to the **Top $N$ pool** (Pool Size $\in \{5, 10, 15\}$).
   - For these candidates, calculate pivot support levels (Classic, Fibonacci, Camarilla) using the previous day's High, Low, and Close.
   - Disqualify candidates that closed below their calculated support.
   - Calculate percentage distance from Close to support level.
   - Enter the candidate with the **smallest distance** (safely resting closest to support).

---

## 4. Pivot Grid Search

The `grid` subcommand runs the standard rules and all 39 pivot permutations (Systems: Classic, Fibonacci, Camarilla; Levels: S1-S3 (S4 for Camarilla) and closest; Pools: 5, 10, 15) over every start month that fits a `-horizon`-year window (default 5), and reports the median, 10th and 90th percentile CAGR of each:
```bash
go run ./cmd/research grid -universe nifty50 -start 2008-02-01 -end 2025-08-31
```

---

## 5. Long-run research

`cmd/research` and the packages above model the confirmed rule set (`nsx` presets), dated Indian costs and tax (`internal/costs`), total-return benchmarks (`internal/bench`), the standard strategy metrics (`internal/analytics`), start-date lotteries, capital jitter, walk-forward over variants and the deflated Sharpe ratio (`internal/experiment`). Findings: [`reports/long-run-findings.md`](reports/long-run-findings.md). Reproduce with `research/run_complete.sh`; check the simulator against the independent Python implementation with `research/py/verify_sim.sh`.

