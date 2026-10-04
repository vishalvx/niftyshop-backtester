# NiftyShop Backtester

A Go backtester for the mechanical **NiftyShop** mean-reversion equity strategy on Indian stock indices.

## Features
* **Point-in-time index membership**: trades only stocks that were in the index that month, with an optional one-month lag (`-lag 1`) so no membership is known before it was published.
* **Confirmed rule set**: the `nsx` and `nsx-lot` presets follow the rules the maintainer confirmed in October 2026 and are the baseline; other presets cover the written spec, exit variants and momentum rotations (`rotation-n50`, `mom-...`).
* **Costs and tax**: dated Indian brokerage, STT, stamp duty and capital-gains tax, reported in stages (`gross`, `cost`, `tax-dated`, `tax-today`).
* **Total-return benchmarks**: every result is compared with the index's total-return series (TRI), not its price series.
* **Robustness checks**: start-date lotteries, capital jitter, walk-forward tests and the deflated Sharpe ratio.

---

## Getting Started

### Prerequisites
* Go 1.24.0 or higher.
* Python 3 and `curl` (for downloading price and index data).

### Download data
Prices and index series are not in the repository. Download them into `.research-data/` (gitignored):
```bash
python3 research/py/fetch_yahoo.py $(tr '\n' ' ' < research/symbols_literal.txt) TMPV.NS LTF.NS UNITDSPR.NS INDUSTOWER.NS SAMMAANCAP.NS BAJAJ-AUTO.NS
python3 research/py/fetch_tri.py "NIFTY 50" "NIFTY 500"
```

### Run a backtest
Score the confirmed rules on the Nifty 50 from its first membership snapshot, against the Nifty 50 TRI:
```bash
go run ./cmd/research score -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag 1 -variants nsx,nsx-lot
```
`-universe` is `nifty50` (point-in-time members) or `nifty50static` (the August 2025 members held fixed, a survivorship-bias probe). Other subcommands (`lottery`, `jitter`, `wfvariants`, `sweep`, ...) are listed in [cmd/research/main.go](cmd/research/main.go).

### Reproduce the long-run study
```bash
go build -o /tmp/research_final ./cmd/research
zsh research/run_complete.sh        # the Nifty 50 tables for nsx and nsx-lot
zsh research/py/verify_sim.sh       # Go simulator against the independent Python reference
```

### Removed in October 2026
The study tested NiftyShop on four member lists and with V-Pivot entry filters. Only the Nifty 50 list is trustworthy, and no V-Pivot variant beat its own index after tax, so the rest was removed:
* the Nifty Midcap 50, Nifty Smallcap 50 and Nifty500 Momentum 50 member lists (`internal/data/*_weights.csv`) and their `-universe` values. They were stale or made up (see [issue #4](https://github.com/vishalvx/niftyshop-backtester/issues/4)).
* the V-Pivot presets (`cam-s1-5`, `fib-s1-15`, `spec-cam-s1-5`, `app-vpivot-*`, `pivot:`, `appv:` and the `:<system>:<level>:<pool>` suffix), the 39-point pivot grid (`grid` subcommand and the pivot family of the tuning trial set), and `research/rules/vpivot-camarilla-s1-5.json`.
* the `repro` subcommand, which re-ran the retired engine's published numbers, and the study scripts built on the removed parts (`run_study.sh`, `rerun_study.sh`, `rerun_all.sh`, `run_lot3.sh`, `run_lot3ix.sh`, `run_nsx.sh`).

The tag [`findings-2026-10`](https://github.com/vishalvx/niftyshop-backtester/tree/findings-2026-10) is the last commit that reproduces every table in [reports/long-run-findings.md](reports/long-run-findings.md); check it out to re-run the removed parts. The committed outputs in `research/out/final/complete` came from that tag. The simulator still accepts a `pivot` rule in a rules file; nothing in the repository uses it.

---

## Documentation
* **Strategy Rules**: See [strategy.md](strategy.md) for details on entry, exit, and averaging criteria.
* **Long-run findings**: See [reports/long-run-findings.md](reports/long-run-findings.md). The complete backtest from the start of each index finds that no variant clears 15% a year after costs and tax and none reliably beats its own index after tax; earlier figures in `reports/report.md`, `reports/pivot_comparison_report.md` and `nifty-shop-v-pivot.md` do not reproduce and are marked as superseded.
* **Momentum findings**: See [reports/momentum-nifty50-findings.md](reports/momentum-nifty50-findings.md). Eight momentum rotations on the Nifty 50, frozen in advance: the best match or beat the Nifty 50 index after costs and tax, but every one trails a Nifty200 Momentum 30 index fund by 4.5 points a year or more, so none is kept.
* **Architecture**: See [CONTEXT.md](CONTEXT.md) for the packages and the data each index has. The original engine (`cmd/backtester`) was retired in October 2026: it did not follow the written rules and its published numbers came from a data-loader bug.
* **Contributing**: Check [CONTRIBUTING.md](CONTRIBUTING.md) to set up development and submit code changes.

---

## References & Credits
* **Data Sourcing**: Detailed constituents scraping and data collection were performed using [nifty-indices-datasets](https://github.com/vishalvx/nifty-indices-datasets).
* **Development Superpowers**: This project was developed utilizing the agentic coding skills from [superpowers](https://github.com/obra/superpowers).