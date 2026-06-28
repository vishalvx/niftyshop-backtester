# Contributing to NiftyShop Backtester

We welcome contributions to improve the backtester's performance, indicators, and reporting features.

## Getting Started

### Prerequisites
* Go 1.24 or higher installed.
* Python 3 (for running consolidated reports).

### Run the Backtester
1. Download historical stock datasets and save them in CSV format under `internal/data`.
2. Run a specific backtest using the command line:
   ```bash
   go run cmd/backtester/main.go -universe nifty50 -start-date 2021-01-01 -end-date 2025-12-31
   ```
3. Run all index backtests and generate a markdown report:
   ```bash
   ./run_all.sh
   ```

### Running Tests
Make sure all unit tests pass before submitting changes:
```bash
go test ./...
```

## Pull Request Guidelines
1. Fork the repository and create your branch from `main`.
2. Write unit tests for any new features or bug fixes.
3. Format the code with `go fmt ./...`.
4. Ensure `go vet ./...` reports no warnings.
5. Submit a pull request detailing the changes, context, and verification results.
