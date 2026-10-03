# Walk-forward over 6 variants: nifty50 2008-02-01..2025-08-31, train 6y / test 2y, selection = best Sharpe after costs on the training window, evaluated after costs and dated tax; benchmark nifty50's own TRI

| train | test | chosen | test CAGR chosen | test CAGR median variant | test CAGR best in hindsight | benchmark (after tax) | chosen beats benchmark |
|---|---|---|---|---|---|---|---|
| 2008-02..2014-01 | 2014-02..2016-01 | nsx:fibonacci:S1:15 | -16.50% |  -8.25% |  -4.97% |  12.71% | false |
| 2010-02..2016-01 | 2016-02..2018-01 | nsx:fibonacci:S1:15 |  14.43% |  14.43% |  20.99% |  22.40% | false |
| 2012-02..2018-01 | 2018-02..2020-01 | nsx | -16.60% |   2.89% |  10.90% |   5.55% | false |
| 2014-02..2020-01 | 2020-02..2022-01 | nsx |  26.49% |  39.71% |  45.05% |  21.55% | true |
| 2016-02..2022-01 | 2022-02..2024-01 | nsx:fibonacci:S1:15 |  11.03% |  11.88% |  12.99% |  11.76% | false |
| 2018-02..2024-01 | 2024-02..2025-08 | nsx-lot:camarilla:S1:5 |   3.05% |   3.35% |  10.80% |   9.10% | false |

Stitched out-of-sample record over 11.6 years (after costs and dated tax): chosen   2.39% | median variant   9.97% | best in hindsight  15.18% | benchmark  13.85% | chosen beat the benchmark in 1 of 6 test windows

Every fixed variant over the same unseen windows (after costs and dated tax):
  nsx                            3.84%
  nsx-lot                        4.80%
  nsx:camarilla:S1:5             5.80%
  nsx-lot:camarilla:S1:5         8.82%
  nsx:fibonacci:S1:15           10.13%
  nsx-lot:fibonacci:S1:15       11.40%

Sharpe over the whole window, after costs; PSR = P(true Sharpe > 0); DSR = same, charged for N = 240 trials (cross-variant Sharpe sd 0.0066 per day, expected best-of-N Sharpe 0.019 per day = 0.30 a year):
| variant | Sharpe a year | PSR | DSR (N trials) |
|---|---|---|---|
| nsx:camarilla:S1:5 | 0.36 | 0.931 | 0.601 |
| nsx:fibonacci:S1:15 | 0.28 | 0.874 | 0.467 |
| nsx | 0.24 | 0.842 | 0.409 |
| nsx-lot:camarilla:S1:5 | 0.24 | 0.837 | 0.403 |
| nsx-lot:fibonacci:S1:15 | 0.23 | 0.826 | 0.384 |
| nsx-lot | 0.04 | 0.565 | 0.143 |
