# Walk-forward over 6 variants: nifty50 2008-02-01..2025-08-31, train 6y / test 2y, selection = best Sharpe after costs on the training window, evaluated after costs and dated tax; benchmark nifty50's own TRI

| train | test | chosen | test CAGR chosen | test CAGR median variant | test CAGR best in hindsight | benchmark (after tax) | chosen beats benchmark |
|---|---|---|---|---|---|---|---|
| 2008-02..2014-01 | 2014-02..2016-01 | nsx-lot:fibonacci:S1:15 | -13.86% |  -2.00% |   2.82% |  12.71% | false |
| 2010-02..2016-01 | 2016-02..2018-01 | nsx-lot |   4.94% |  10.10% |  12.06% |  22.40% | false |
| 2012-02..2018-01 | 2018-02..2020-01 | nsx-lot | -12.00% |   5.41% |  19.15% |   5.55% | false |
| 2014-02..2020-01 | 2020-02..2022-01 | nsx:camarilla:S1:5 |  35.62% |  35.62% |  45.05% |  21.55% | true |
| 2016-02..2022-01 | 2022-02..2024-01 | nsx |  10.72% |  11.88% |  12.99% |  11.76% | false |
| 2018-02..2024-01 | 2024-02..2025-08 | nsx-lot:camarilla:S1:5 |   2.50% |   3.35% |   9.80% |   9.10% | false |

Stitched out-of-sample record over 11.6 years (after costs and dated tax): chosen   3.47% | median variant  10.38% | best in hindsight  16.51% | benchmark  13.85% | chosen beat the benchmark in 1 of 6 test windows

Every fixed variant over the same unseen windows (after costs and dated tax):
  nsx                            2.44%
  nsx-lot                        3.67%
  nsx:camarilla:S1:5             9.08%
  nsx-lot:camarilla:S1:5         9.59%
  nsx:fibonacci:S1:15           12.65%
  nsx-lot:fibonacci:S1:15        9.85%

Sharpe over the whole window, after costs; PSR = P(true Sharpe > 0); DSR = same, charged for N = 240 trials (cross-variant Sharpe sd 0.0083 per day, expected best-of-N Sharpe 0.024 per day = 0.37 a year):
| variant | Sharpe a year | PSR | DSR (N trials) |
|---|---|---|---|
| nsx-lot:fibonacci:S1:15 | 0.43 | 0.965 | 0.600 |
| nsx:fibonacci:S1:15 | 0.34 | 0.923 | 0.451 |
| nsx | 0.30 | 0.896 | 0.384 |
| nsx:camarilla:S1:5 | 0.24 | 0.840 | 0.290 |
| nsx-lot:camarilla:S1:5 | 0.19 | 0.782 | 0.220 |
| nsx-lot | 0.05 | 0.588 | 0.092 |
