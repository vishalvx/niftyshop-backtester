# NiftyShop Backtester

A Go-based historical backtesting engine for the mechanical **NiftyShop** mean-reversion equity strategy on Indian stock indices.

## Features
* **Rebalancing Support**: Tracks historical index constituents dynamically over time.
* **Averaging Engine**: Automatically handles multiple levels of average-down entries when positions drop.
* **Pre-inception Protection**: Gracefully restricts backtest windows according to historical index launch dates.
* **Consolidated Reporting**: Evaluates multi-index metrics, calculating CAGR, Win Rate, holding periods, and outputting trade logs.

---

## Getting Started

### Prerequisites
* Go 1.24.0 or higher.
* Python 3 (for executing consolidated reports).

### Usage

#### Run a Single Backtest
Run a backtest for a specific index universe with CLI overrides:
```bash
go run cmd/backtester/main.go -universe nifty50 -start-date 2021-01-01 -end-date 2025-12-31
```

#### Run All Index Universes
Run backtests for all supported indices (Nifty 50, Midcap 50, Smallcap 50, Nifty500 Momentum 50) and output a consolidated summary report:
```bash
./run_all.sh
```
All outputs (consolidated `report.md` and CSV trade logs) will be saved in the `reports/` directory.

---

## Documentation
* **Strategy Rules**: See [strategy.md](strategy.md) for details on entry, exit, and averaging criteria.
* **Contributing**: Check [CONTRIBUTING.md](CONTRIBUTING.md) to set up development and submit code changes.

---

## References & Credits
* **Data Sourcing**: Detailed constituents scraping and data collection were performed using [nifty-indices-datasets](https://github.com/vishalvx/nifty-indices-datasets).
* **Development Superpowers**: This project was developed utilizing the agentic coding skills from [superpowers](https://github.com/obra/superpowers).