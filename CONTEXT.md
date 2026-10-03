# Project Context & Index Metadata

This project is a Go-based backtester evaluating mechanical mean-reversion strategies (Standard NiftyShop & NiftyShop V-Pivot Variant) on Indian stock indices.

## 1. Supported Indices & Data Gates

The backtest engine handles historical data restrictions dynamically. If a configured backtest `start_date` is prior to the launch or available data start of the selected index, the engine automatically adjusts `start_date` to the index data inception date and prints a warning.

| Index Name | CLI Universe Identifier | Actual Index Inception | Backtest Inception Date (Data Gate) | Historical Constituents CSV | Minimum Backtest Year (CSV Boundary) | Rebalancing Schedule |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Nifty 50** | `nifty50` | April 22, 1996 | **2008-01-01** (Data start) | `nifty50_weights.csv` | **2008** | Semi-annual (March/Sept) |
| **Nifty Midcap 50** | `niftymidcap50` | January 1, 2004 | **2004-01-01** (Inception) | `niftymidcap50_weights.csv` | **2019** (CSV limit) | Semi-annual (March/Sept) |
| **Nifty Smallcap 50** | `niftysmallcap50` | April 1, 2016 | **2016-04-01** (Inception) | `niftysmallcap50_weights.csv` | **2019** (CSV limit) | Semi-annual (March/Sept) |
| **Nifty500 Momentum 50** | `nifty500momentum50` | June 4, 2024 | **2024-06-04** (Inception) | `nifty500momentum50_weights.csv` | **2024** | Semi-annual (June/Dec) |

> [!NOTE]
> Even though **Nifty Midcap 50** and **Nifty Smallcap 50** have inception dates in 2004 and 2016, historical constituent weights CSVs are only available from **2019-01-31** onwards. Therefore, running backtests prior to 2019 for these indices will not find any active constituents and will skip trades.

---

## 2. Core Architecture & Key Packages

The codebase is modularized into several internal packages:

* **`cmd/backtester`** ([main.go](cmd/backtester/main.go)):
  - Main entrypoint of the simulation engine.
  - Handles command-line arguments (`-universe`, `-start-date`, `-end-date`, `-find-best-pivot`).
  - Precomputes technical indicators (SMA, Pivot Levels) and coordinates simulation or parameter grid-search runs.
* **`internal/config`** ([config.go](internal/config/config.go)):
  - Defines and validates configuration parameters (`Config` and `PivotFilterConfig`).
  - Configuration files: `config.json` (runtime defaults) and `universe.json` (supported universes).
* **`internal/data`** ([loader.go](internal/data/loader.go)):
  - Fetches index symbols dynamically from NSE or falls back to standard constituents.
  - Downloads daily historical OHLCV data from Yahoo Finance API.
  - Loads historical constituent weights from CSV to handle index rebalancing.
* **`internal/engine`** ([engine.go](internal/engine/engine.go)):
  - Executes EOD strategy logic: exit processing, averaging down, candidate pool selection, and fresh entries.
* **`internal/portfolio`** ([portfolio.go](internal/portfolio/portfolio.go)):
  - Manages portfolio ledger, cash remaining, open positions, averaging units, and purchase lot details.
* **`internal/indicators`**:
  - Implements indicator calculation logic, such as Simple Moving Average (SMA) and Pivot Support Lines (Classic, Fibonacci, Camarilla).
* **`internal/metrics`** ([report.go](internal/metrics/report.go)):
  - Aggregates trade results and computes metrics: CAGR, Win Rate, total trades, average holding period, and exports CSV trade logs.

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

## 4. Run Scripts & Grid-Search Flag

* **Consolidated Runner** ([run_all.sh](run_all.sh)): Runs simulations for all indices.
* **Comparison Script** ([run_pivot_comparison.py](run_pivot_comparison.py)): Performs comparative backtesting (Standard vs. Best Pivot Permutations) and generates `reports/pivot_comparison_report.md`.
* **Grid-Search Command**:
  ```bash
  go run cmd/backtester/main.go -universe nifty50 -find-best-pivot
  ```
  Runs a simulation loop across all 39 pivot permutations (Systems: Classic, Fibonacci, Camarilla; Levels: S1-S4, closest; Pools: 5, 10, 15) to identify the optimal configuration.

---

## 5. Long-run research (`cmd/research`, `internal/sim`, `internal/panel`, `research/`)

A second, parameterised simulator and a clean data panel sit beside the original engine. They model the confirmed rule set (`nsx` presets), dated Indian costs and tax (`internal/costs`), total-return benchmarks (`internal/bench`), the standard strategy metrics (`internal/analytics`), start-date lotteries, capital jitter, walk-forward over variants and the deflated Sharpe ratio (`internal/experiment`). Findings: [`reports/long-run-findings.md`](reports/long-run-findings.md). Reproduce with `research/run_complete.sh`; check the simulator against the independent Python implementation with `research/py/verify_sim.sh`.

