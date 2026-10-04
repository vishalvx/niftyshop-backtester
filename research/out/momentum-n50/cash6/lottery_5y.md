# Start-date lottery: nifty50, 5-year horizons starting every month between 2008-02-01 and 2025-08-31 (151 windows per cell)

| variant | stage | min | p10 | median | p90 | max | share >= 15% | share < 0 | median maxDD | beats TRI (same basis) |
|---|---|---|---|---|---|---|---|---|---|---|
| mom-nse-top10-6m-ma200-cash6 | cost |  -1.41% |   3.95% |  10.76% |  14.50% |  17.76% |   7.95% |   0.66% | -16.05% |  27.15% |
| mom-nse-top10-6m-ma200-cash6 | tax-dated |  -1.41% |   3.74% |  10.09% |  13.44% |  16.79% |   3.97% |   0.66% | -16.04% |  24.50% |
| mom-nse-top10-6m-ma200-cash6 | tax-today |  -1.65% |   3.61% |   9.49% |  12.73% |  15.95% |   2.65% |   0.66% | -16.24% |  25.83% |
| mom-nse-top20-1m-ma200-cash6 | cost |   2.73% |   5.87% |   9.79% |  15.26% |  19.31% |  11.26% |   0.00% | -14.04% |  11.92% |
| mom-nse-top20-1m-ma200-cash6 | tax-dated |   2.73% |   5.74% |   9.29% |  14.18% |  17.15% |   7.28% |   0.00% | -14.06% |  11.26% |
| mom-nse-top20-1m-ma200-cash6 | tax-today |   2.39% |   5.37% |   8.51% |  13.85% |  16.93% |   5.96% |   0.00% | -14.10% |   9.93% |
| mom-plain-top10-1m-ma200-cash6 | cost |   3.45% |   6.01% |   9.60% |  15.57% |  21.20% |  12.58% |   0.00% | -18.17% |  15.89% |
| mom-plain-top10-1m-ma200-cash6 | tax-dated |   3.45% |   5.60% |   8.77% |  14.08% |  18.81% |   7.95% |   0.00% | -18.23% |  13.25% |
| mom-plain-top10-1m-ma200-cash6 | tax-today |   3.20% |   5.23% |   8.17% |  13.59% |  18.49% |   5.96% |   0.00% | -18.32% |  11.92% |
| mom-plain-top20-6m-ma200-cash6 | cost |  -1.88% |   5.05% |  10.64% |  15.11% |  19.33% |  11.26% |   0.66% | -16.76% |  25.17% |
| mom-plain-top20-6m-ma200-cash6 | tax-dated |  -1.88% |   4.98% |  10.43% |  14.32% |  18.85% |   6.62% |   0.66% | -16.77% |  24.50% |
| mom-plain-top20-6m-ma200-cash6 | tax-today |  -2.20% |   4.73% |   9.93% |  13.66% |  17.51% |   3.97% |   0.66% | -16.76% |  25.17% |
| NIFTY 50 TRI buy-and-hold (gross) | - |   0.51% |   6.76% |  13.20% |  17.39% |  24.34% |  28.48% |   0.00% | - | - |
| NIFTY 50 TRI buy-and-hold (after tax, dated) | - |   0.51% |   6.76% |  13.01% |  16.80% |  22.26% |  19.87% |   0.00% | - | - |
| NIFTY 50 TRI buy-and-hold (after tax, today's rates) | - |   0.51% |   6.23% |  12.02% |  15.84% |  22.26% |  15.89% |   0.00% | - | - |

## Gap to the benchmarks, same dates (variant CAGR minus benchmark CAGR, points a year, same tax basis)

| variant | stage | median gap to NIFTY 50 TRI | p10 | worst | median gap to NIFTY200 MOMENTUM 30 TRI less 0.20% a year | p10 | worst | beats fund |
|---|---|---|---|---|---|---|---|---|
| mom-nse-top10-6m-ma200-cash6 | cost | -2.97 | -8.93 | -12.79 | -10.49 | -15.01 | -18.79 |   5.30% |
| mom-nse-top10-6m-ma200-cash6 | tax-dated | -3.53 | -8.41 | -12.21 | -10.27 | -15.26 | -19.31 |   4.64% |
| mom-nse-top10-6m-ma200-cash6 | tax-today | -2.98 | -8.41 | -11.65 | -9.71 | -14.00 | -17.57 |   4.64% |
| mom-nse-top20-1m-ma200-cash6 | cost | -2.83 | -5.36 | -8.65 | -9.89 | -16.61 | -18.82 |   4.64% |
| mom-nse-top20-1m-ma200-cash6 | tax-dated | -3.06 | -5.58 | -9.72 | -10.11 | -16.79 | -18.82 |   4.64% |
| mom-nse-top20-1m-ma200-cash6 | tax-today | -2.85 | -5.23 | -8.59 | -9.28 | -15.36 | -17.43 |   4.64% |
| mom-plain-top10-1m-ma200-cash6 | cost | -3.05 | -6.26 | -8.27 | -9.39 | -16.58 | -19.38 |   4.64% |
| mom-plain-top10-1m-ma200-cash6 | tax-dated | -3.33 | -6.58 | -9.62 | -9.69 | -17.00 | -19.55 |   4.64% |
| mom-plain-top10-1m-ma200-cash6 | tax-today | -3.02 | -6.32 | -8.57 | -9.29 | -15.56 | -17.95 |   4.64% |
| mom-plain-top20-6m-ma200-cash6 | cost | -3.61 | -6.94 | -10.65 | -9.69 | -15.46 | -19.65 |   4.64% |
| mom-plain-top20-6m-ma200-cash6 | tax-dated | -3.62 | -6.65 | -10.26 | -9.30 | -15.42 | -19.80 |   4.64% |
| mom-plain-top20-6m-ma200-cash6 | tax-today | -3.37 | -6.31 | -9.70 | -8.85 | -14.23 | -18.13 |   5.30% |
