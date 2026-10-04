# Project Context & Index Metadata

This project is a Go-based backtester evaluating the mechanical NiftyShop mean-reversion strategy, and a momentum rotation, on the Nifty 50.

## 1. Supported Universes & Data Gates

`go run ./cmd/research <subcommand> -universe <id> -start <date> -end <date>` picks the member list with `-universe`. The window must sit inside the list's range, because the CLI does not move `-start` for you.

| Universe | CLI identifier | Members CSV | Range | Benchmark |
| :--- | :--- | :--- | :--- | :--- |
| **Nifty 50**, point-in-time | `nifty50` | `internal/data/nifty50_weights.csv` | Jan 2008 to Aug 2025 (first tradable month Feb 2008) | Nifty 50 TRI |
| **Nifty 50**, Aug 2025 members held fixed (survivorship-bias probe) | `nifty50static` | `research/rules/nifty50_static_2025-08_weights.csv` | Jan 2018 to Aug 2025 | Nifty 50 TRI |

The Nifty Midcap 50, Nifty Smallcap 50 and Nifty500 Momentum 50 lists were removed in October 2026 because they were stale or made up; they are at the `findings-2026-10` tag.

---

## 2. Core Architecture & Key Packages

* **`cmd/research`** ([main.go](cmd/research/main.go)): the command-line entrypoint. Each subcommand (`score`, `lottery`, `jitter`, `wfvariants`, `sweep`, `audit`, ...) builds a data panel for one universe and window, runs named rule presets and prints tables.
* **`internal/panel`** ([panel.go](internal/panel/panel.go)): loads one canonical Yahoo CSV per ticker with dividends, applies symbol aliases and quarantines, and builds month-by-month index membership from the constituents CSVs.
* **`internal/sim`** ([sim.go](internal/sim/sim.go)): the day-by-day NiftyShop simulator. `Rules` holds every rule variant; `LegacyRules()` reproduces the retired original engine, `SpecRules()` the written strategy.
* **`internal/experiment`**: rule presets, cost and tax stages, start-date lotteries, walk-forward and the deflated Sharpe ratio.
* **`internal/costs`**: dated Indian brokerage, STT, stamp duty and capital-gains tax.
* **`internal/bench`**: total-return index (TRI) benchmarks.
* **`internal/analytics`**: strategy metrics (CAGR, drawdown, Sharpe, Sortino, alpha and more).
* **`internal/indicators`**: Simple Moving Average and pivot support levels (Classic, Fibonacci, Camarilla). The pivot levels feed the simulator's dormant `pivot` rule, which no preset uses.
* **`research/`**: study scripts, rule files and an independent Python reference simulator (`research/py/ref_sim.py`).

The original engine (`cmd/backtester`, `internal/engine`, `internal/metrics`, `internal/portfolio`, `internal/config`, the `internal/data` loader, `run_all.sh` and `run_pivot_comparison.py`) was deleted in October 2026. It did not follow the written rules, counted every sale as a win, and its published numbers came from a data-loader bug.

---

## 3. Strategy Configurations

1. **NiftyShop** (`nsx`, `nsx-lot`, the baseline): buy the Nifty 50 member furthest below its 20-day average, add lots on further falls (at most 3 per stock), sell at +5% (whole position for `nsx`, each lot for `nsx-lot`), sell a stock that leaves the index. `legacy`, `spec`, `spec-cap3`, `spec-lot3`, `app-approx`, `app-exact` and `exit:` cover the retired engine, the written spec and exit variants.
2. **Momentum rotation** (`rotation-n50`): the 10 members with the best mean of 6- and 12-month return, equal weight, swapped on the first trading day of January and July. The `mom-<plain|nse>-top<N>-<6m|1m>[-ma<days>]` presets vary the score (plain, or NSE's volatility-adjusted z-score), the names held, the swap months and a market filter on the Nifty 50 price index; `mom-plain-top10-6m` is `rotation-n50`. Study plan: [`research/studies/momentum-nifty50.md`](research/studies/momentum-nifty50.md); findings: [`reports/momentum-nifty50-findings.md`](reports/momentum-nifty50-findings.md) (no variant kept: none beats a Nifty200 Momentum 30 fund); reproduce with `research/run_momentum.sh`.

The NiftyShop V-Pivot variants and the 39-point pivot grid search were removed in October 2026; none beat its own index after tax. They are at the `findings-2026-10` tag.

---

## 4. Long-run research

`cmd/research` and the packages above model the confirmed rule set (`nsx` presets), dated Indian costs and tax (`internal/costs`), total-return benchmarks (`internal/bench`), the standard strategy metrics (`internal/analytics`), start-date lotteries, capital jitter, walk-forward over variants and the deflated Sharpe ratio (`internal/experiment`). Findings: [`reports/long-run-findings.md`](reports/long-run-findings.md). Reproduce the Nifty 50 tables with `research/run_complete.sh` (every published table, including the removed lists and V-Pivot variants, with the script at the `findings-2026-10` tag); check the simulator against the independent Python implementation with `research/py/verify_sim.sh`.

