# Walk-forward over 6 variants: niftymidcap50 2019-02-01..2026-06-30, train 3y / test 1y, selection = best Sharpe after costs on the training window, evaluated after costs and dated tax; benchmark niftymidcap50's own TRI

| train | test | chosen | test CAGR chosen | test CAGR median variant | test CAGR best in hindsight | benchmark (after tax) | chosen beats benchmark |
|---|---|---|---|---|---|---|---|
| 2019-02..2022-01 | 2022-02..2023-01 | nsx-lot:fibonacci:S1:15 |   3.13% |   3.13% |   9.43% |   2.77% | true |
| 2020-02..2023-01 | 2023-02..2024-01 | nsx-lot:fibonacci:S1:15 |  36.76% |  36.51% |  37.09% |  51.23% | false |
| 2021-02..2024-01 | 2024-02..2025-01 | nsx-lot:camarilla:S1:5 |  11.57% |  13.99% |  27.72% |   8.77% | true |
| 2022-02..2025-01 | 2025-02..2026-01 | nsx-lot:fibonacci:S1:15 |  10.52% |  10.52% |  26.09% |   9.91% | true |

Stitched out-of-sample record over 4.0 years (after costs and dated tax): chosen  14.84% | median variant  15.40% | best in hindsight  24.67% | benchmark  16.75% | chosen beat the benchmark in 3 of 4 test windows

Every fixed variant over the same unseen windows (after costs and dated tax):
  nsx                            7.95%
  nsx-lot                        9.45%
  nsx:camarilla:S1:5            11.41%
  nsx-lot:camarilla:S1:5        14.76%
  nsx:fibonacci:S1:15           22.29%
  nsx-lot:fibonacci:S1:15       16.15%

Sharpe over the whole window, after costs; PSR = P(true Sharpe > 0); DSR = same, charged for N = 240 trials (cross-variant Sharpe sd 0.0083 per day, expected best-of-N Sharpe 0.023 per day = 0.37 a year):
| variant | Sharpe a year | PSR | DSR (N trials) |
|---|---|---|---|
| nsx-lot:fibonacci:S1:15 | 0.62 | 0.950 | 0.747 |
| nsx | 0.61 | 0.944 | 0.731 |
| nsx-lot | 0.53 | 0.921 | 0.666 |
| nsx-lot:camarilla:S1:5 | 0.49 | 0.907 | 0.629 |
| nsx:camarilla:S1:5 | 0.47 | 0.896 | 0.604 |
| nsx:fibonacci:S1:15 | 0.26 | 0.755 | 0.382 |
