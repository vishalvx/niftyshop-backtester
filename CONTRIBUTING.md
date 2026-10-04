# Contributing to NiftyShop Backtester

We welcome contributions to improve the backtester's performance, indicators, and reporting features.

## Getting Started

### Prerequisites
* Go 1.24 or higher installed.
* Python 3 and `curl` (for downloading data and the Python reference simulator), and poppler's `pdftotext` for the NSE press releases.

### Run the Backtester
1. Download price and index data into `.research-data/` as described in the [README](README.md#download-data).
2. Run a backtest with the research CLI:
   ```bash
   go run ./cmd/research score -universe nifty50 -start 2008-02-01 -end 2026-09-30 -variants nsx
   ```
3. Regenerate the long-run study tables:
   ```bash
   go build -o /tmp/research_final ./cmd/research
   zsh research/run_complete.sh
   ```

### Running Tests
Make sure all unit tests pass before submitting changes:
```bash
go test ./...
python3 -m unittest research/py/test_nse_build.py
```

## Pull Request Guidelines
1. Fork the repository and create your branch from `main`.
2. Write unit tests for any new features or bug fixes.
3. Format the code with `go fmt ./...`.
4. Ensure `go vet ./...` reports no warnings.
5. Submit a pull request detailing the changes, context, and verification results.
