# NiftyShop Backtester

A Go backtester for the mechanical **NiftyShop** mean-reversion equity strategy on Indian stock indices.

## Features
* **Point-in-time index membership**: trades only stocks that were in the index that month, with an optional one-month lag (`-lag 1`) so no membership is known before it was published.
* **Confirmed rule set**: the `nsx` presets follow the rules the maintainer confirmed in October 2026; other presets cover the written spec, V-Pivot entry filters and exit variants.
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
python3 research/py/fetch_tri.py "NIFTY 50" "NIFTY MIDCAP 50" "NIFTY SMALLCAP 50" "NIFTY500 MOMENTUM 50" "NIFTY 500"
```

### Run a backtest
Score the confirmed rules on the Nifty 50 from its first membership snapshot, against the Nifty 50 TRI:
```bash
go run ./cmd/research score -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag 1 -variants nsx,nsx-lot
```
`-universe` is one of `nifty50`, `niftymidcap50`, `niftysmallcap50` or `nifty500momentum50`. Other subcommands (`lottery`, `jitter`, `wfvariants`, `sweep`, ...) are listed in [cmd/research/main.go](cmd/research/main.go).

### Reproduce the long-run study
```bash
go build -o /tmp/research_final ./cmd/research
zsh research/run_complete.sh        # all tables in reports/long-run-findings.md
zsh research/py/verify_sim.sh       # Go simulator against the independent Python reference
```

---

## Documentation
* **Strategy Rules**: See [strategy.md](strategy.md) for details on entry, exit, and averaging criteria.
* **Long-run findings**: See [reports/long-run-findings.md](reports/long-run-findings.md). The complete backtest from the start of each index finds that no variant clears 15% a year after costs and tax and none reliably beats its own index after tax; earlier figures in `reports/report.md`, `reports/pivot_comparison_report.md` and `nifty-shop-v-pivot.md` do not reproduce and are marked as superseded.
* **Architecture**: See [CONTEXT.md](CONTEXT.md) for the packages and the data each index has. The original engine (`cmd/backtester`) was retired in October 2026: it did not follow the written rules and its published numbers came from a data-loader bug.
* **Contributing**: Check [CONTRIBUTING.md](CONTRIBUTING.md) to set up development and submit code changes.

---

## References & Credits
* **Data Sourcing**: Detailed constituents scraping and data collection were performed using [nifty-indices-datasets](https://github.com/vishalvx/nifty-indices-datasets).
* **Development Superpowers**: This project was developed utilizing the agentic coding skills from [superpowers](https://github.com/obra/superpowers).