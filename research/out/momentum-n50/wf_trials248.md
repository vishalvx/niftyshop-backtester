# Walk-forward over 8 variants: nifty50 2008-02-01..2025-08-31, train 6y / test 2y, selection = best Sharpe after costs on the training window, evaluated after costs and dated tax; benchmark nifty50's own TRI

| train | test | chosen | test CAGR chosen | test CAGR median variant | test CAGR best in hindsight | benchmark (after tax) | chosen beats benchmark |
|---|---|---|---|---|---|---|---|
| 2008-02..2014-01 | 2014-02..2016-01 | mom-plain-top20-1m |  12.37% |  12.37% |  14.66% |  12.71% | false |
| 2010-02..2016-01 | 2016-02..2018-01 | mom-nse-top10-6m-ma200 |   4.68% |  18.59% |  26.40% |  22.40% | false |
| 2012-02..2018-01 | 2018-02..2020-01 | mom-plain-top10-6m |   8.98% |   8.37% |   9.50% |   5.55% | true |
| 2014-02..2020-01 | 2020-02..2022-01 | mom-nse-top20-6m |  28.02% |  22.49% |  28.02% |  21.55% | true |
| 2016-02..2022-01 | 2022-02..2024-01 | mom-nse-top20-6m |  14.38% |   8.71% |  14.57% |  11.76% | true |
| 2018-02..2024-01 | 2024-02..2025-08 | mom-nse-top20-6m |   2.65% |   4.29% |   6.81% |   9.10% | false |

Stitched out-of-sample record over 11.6 years (after costs and dated tax): chosen  11.89% | median variant  12.60% | best in hindsight  16.76% | benchmark  13.85% | chosen beat the benchmark in 3 of 6 test windows

Every fixed variant over the same unseen windows (after costs and dated tax):
  mom-plain-top10-6m            12.98%
  mom-plain-top20-6m-ma200       8.18%
  mom-plain-top10-1m-ma200       9.22%
  mom-plain-top20-1m            13.84%
  mom-nse-top10-6m-ma200         7.71%
  mom-nse-top20-6m              14.65%
  mom-nse-top10-1m              10.33%
  mom-nse-top20-1m-ma200         9.65%

Sharpe over the whole window, after costs; PSR = P(true Sharpe > 0); DSR = same, charged for N = 248 trials (cross-variant Sharpe sd 0.0062 per day, expected best-of-N Sharpe 0.018 per day = 0.28 a year):
| variant | Sharpe a year | PSR | DSR (N trials) |
|---|---|---|---|
| mom-plain-top20-1m | 0.35 | 0.925 | 0.609 |
| mom-nse-top20-6m | 0.32 | 0.907 | 0.563 |
| mom-plain-top10-6m | 0.25 | 0.853 | 0.456 |
| mom-plain-top10-1m-ma200 | 0.24 | 0.837 | 0.427 |
| mom-nse-top10-1m | 0.23 | 0.834 | 0.423 |
| mom-nse-top20-1m-ma200 | 0.23 | 0.828 | 0.412 |
| mom-plain-top20-6m-ma200 | 0.08 | 0.631 | 0.203 |
| mom-nse-top10-6m-ma200 | 0.07 | 0.620 | 0.195 |
