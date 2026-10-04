# Project Context & Index Metadata

This project is a Go-based backtester evaluating the mechanical NiftyShop mean-reversion strategy, and a momentum rotation, on the Nifty 50.

## 1. Supported Universes & Data Gates

`go run ./cmd/research <subcommand> -universe <id> -start <date> -end <date>` picks the member list with `-universe`. The window must sit inside the list's range, because the CLI does not move `-start` for you.

| Universe | CLI identifier | Members | Prices | Range | Benchmark |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Nifty 50**, point-in-time | `nifty50` | `internal/data/nifty50_members.csv` | NSE bhavcopy | 1 Jan 2008 to the day the list was rebuilt (first tradable month Feb 2008) | Nifty 50 TRI |
| **Nifty Midcap 150**, point-in-time | `niftymidcap150` | `internal/data/niftymidcap150_members.csv` | NSE bhavcopy | 30 Sep 2016 to the day the list was rebuilt | Nifty Midcap 150 TRI |
| **Nifty 50** as the long-run study ran it | `nifty50yahoo` | `internal/data/nifty50_weights.csv` (month snapshots) | Yahoo | Jan 2008 to Aug 2025 | Nifty 50 TRI |
| **Nifty 50**, Aug 2025 members held fixed (survivorship-bias probe) | `nifty50static` | `research/rules/nifty50_static_2025-08_weights.csv` | Yahoo | Jan 2018 to Aug 2025 | Nifty 50 TRI |

**Official lists and prices (`nifty50`, `niftymidcap150`).** The member files give exact join and leave dates (`symbol,from,to,added_by,removed_by`, `to` exclusive, empty while still a member), rebuilt from NSE Indices press releases and checked against every archived copy of NSE's own list. Prices are NSE's daily bhavcopy, stitched across symbol changes and back-adjusted for splits, bonuses, rights issues and demergers; symbols are the company's latest NSE symbol, and companies that merged or delisted keep their last one (HDFC, SATYAMCOMP). A run fails if any trading day in the window has the wrong member count or a member without a price bar, and `-lag` must be 0 (NSE announces each change weeks before it takes effect). How the data is built, and where each hand-checked decision is recorded: [`research/nse/README.md`](research/nse/README.md).

**Study data (`nifty50yahoo`, `nifty50static`).** Month-end snapshots and Yahoo prices, kept so the October 2026 findings reproduce (`research/run_complete.sh`). Nine Nifty 50 members have no Yahoo prices there and are skipped, which tilts results upward.

The Nifty Midcap 50, Nifty Smallcap 50 and Nifty500 Momentum 50 lists were removed in October 2026 because they were stale or made up; they are at the `findings-2026-10` tag.

---

## 2. Core Architecture & Key Packages

* **`cmd/research`** ([main.go](cmd/research/main.go)): the command-line entrypoint. Each subcommand (`score`, `lottery`, `jitter`, `wfvariants`, `sweep`, `audit`, ...) builds a data panel for one universe and window, runs named rule presets and prints tables.
* **`internal/panel`** ([panel.go](internal/panel/panel.go)): loads one price CSV per symbol with dividends (NSE files as built, or Yahoo files with symbol aliases, quarantines and hand adjustments), reads either membership format, and runs the member-count and price checks.
* **`internal/sim`** ([sim.go](internal/sim/sim.go)): the day-by-day NiftyShop simulator. `Rules` holds every rule variant; `LegacyRules()` reproduces the retired original engine, `SpecRules()` the written strategy.
* **`internal/experiment`**: rule presets, cost and tax stages, start-date lotteries, walk-forward and the deflated Sharpe ratio.
* **`internal/costs`**: dated Indian brokerage, STT, stamp duty and capital-gains tax.
* **`internal/bench`**: total-return index (TRI) benchmarks.
* **`internal/analytics`**: strategy metrics (CAGR, drawdown, Sharpe, Sortino, alpha and more).
* **`internal/indicators`**: Simple Moving Average and pivot support levels (Classic, Fibonacci, Camarilla). The pivot levels feed the simulator's dormant `pivot` rule, which no preset uses.
* **`research/`**: study scripts, rule files, an independent Python reference simulator (`research/py/ref_sim.py`), and the NSE data pipeline (`research/py/nse_fetch.py`, `research/py/nse_build.py`, hand-checked inputs and audit tables in `research/nse/`).

The original engine (`cmd/backtester`, `internal/engine`, `internal/metrics`, `internal/portfolio`, `internal/config`, the `internal/data` loader, `run_all.sh` and `run_pivot_comparison.py`) was deleted in October 2026. It did not follow the written rules, counted every sale as a win, and its published numbers came from a data-loader bug.

---

## 3. Strategy Configurations

1. **NiftyShop** (`nsx`, `nsx-lot`, the baseline): buy the Nifty 50 member furthest below its 20-day average, add lots on further falls (at most 3 per stock), sell at +5% (whole position for `nsx`, each lot for `nsx-lot`), sell a stock that leaves the index. `legacy`, `spec`, `spec-cap3`, `spec-lot3`, `app-approx`, `app-exact` and `exit:` cover the retired engine, the written spec and exit variants.
2. **Momentum rotation** (`rotation-n50`): the 10 members with the best mean of 6- and 12-month return, equal weight, swapped on the first trading day of January and July. The `mom-<plain|nse>-top<N>-<6m|1m>[-ma<days>]` presets vary the score (plain, or NSE's volatility-adjusted z-score), the names held, the swap months and a market filter on the Nifty 50 price index; `mom-plain-top10-6m` is `rotation-n50`. Study plan: [`research/studies/momentum-nifty50.md`](research/studies/momentum-nifty50.md); findings: [`reports/momentum-nifty50-findings.md`](reports/momentum-nifty50-findings.md) (no variant kept: none beats a Nifty200 Momentum 30 fund); reproduce with `research/run_momentum.sh`.

The NiftyShop V-Pivot variants and the 39-point pivot grid search were removed in October 2026; none beat its own index after tax. They are at the `findings-2026-10` tag.

---

## 4. Long-run research

`cmd/research` and the packages above model the confirmed rule set (`nsx` presets), dated Indian costs and tax (`internal/costs`), total-return benchmarks (`internal/bench`), the standard strategy metrics (`internal/analytics`), start-date lotteries, capital jitter, walk-forward over variants and the deflated Sharpe ratio (`internal/experiment`). Findings: [`reports/long-run-findings.md`](reports/long-run-findings.md). Reproduce the Nifty 50 tables with `research/run_complete.sh` (every published table, including the removed lists and V-Pivot variants, with the script at the `findings-2026-10` tag); check the simulator against the independent Python implementation with `research/py/verify_sim.sh`.

