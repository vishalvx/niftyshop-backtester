# Project Context & Index Metadata

This project is a Go-based backtester evaluating mean-reversion strategies on Indian stock indices.

## Supported Indices

1. **Nifty 50** (`nifty50`):
   - Inception: April 22, 1996
   - Rebalancing: Semi-annual (March/September)
   - Historical Constituents: Loaded from `internal/data/nifty50_weights.csv`
2. **Nifty Midcap 50** (`niftymidcap50`):
   - Inception: January 1, 2004
   - Minimum Backtest Year: 2019
   - Rebalancing: Semi-annual (March/September)
3. **Nifty Smallcap 50** (`niftysmallcap50`):
   - Inception: April 1, 2016
   - Minimum Backtest Year: 2019
   - Rebalancing: Semi-annual (March/September)
4. **Nifty500 Momentum 50** (`nifty500momentum50`):
   - Inception: June 4, 2024
   - Minimum Backtest Year: 2024 (Manual handling: only backtestable from June 4, 2024 onwards)
   - Rebalancing: Semi-annual (June/December)

## Pre-Inception Handling

If the configured backtest start date is prior to the launch/inception date of the selected index, the engine automatically adjusts the backtest start date to the index inception date and prints a warning message.
