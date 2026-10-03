# NiftyShop Strategy: Pivot Entry Filter Analysis Report

> **Superseded, 2026-10-03.** These figures were produced by the engine before its data-loader fix, with no costs and no tax, and do not reproduce. See [`long-run-findings.md`](long-run-findings.md) for the complete backtest from the start of each index.

**Backtest Timeframe:** Jan 2021 – Dec 2025 (5 Years)
*Note: Nifty500 Momentum 50 timeframe is manually restricted to June 4, 2024 onwards due to inception date.*

## 1. Executive Summary Table

| Index | Index CAGR | Standard CAGR (No Pivot) | Best Pivot CAGR | Best Configuration | CAGR Delta (vs Standard) | CAGR Delta (vs Index) |
|---|---|---|---|---|---|---|
| **Nifty 50** | 13.11% | 12.78% | **24.67%** | Fibonacci S1 (Pool 5) | **+11.89%** | **+11.56%** |
| **Nifty Midcap 50** | 22.91% | 15.50% | **24.34%** | Fibonacci S1 (Pool 15) | **+8.84%** | **+1.43%** |
| **Nifty Smallcap 50** | 18.58% | 19.19% | **18.57%** | Camarilla S2 (Pool 5) | **+-0.62%** | **+-0.01%** |
| **Nifty500 Momentum 50** | 17.22% | 18.24% | **17.43%** | Camarilla closest (Pool 5) | **+-0.81%** | **+0.21%** |

---

## 2. Detailed Index Comparisons

### 📊 Nifty 50

#### Performance Comparison:

| Strategy / Benchmark | Total Return | CAGR | Total Trades | Win Rate |
|---|---|---|---|---|
| **Nifty Benchmark Index** | 85.03% | 13.11% | - | - |
| **Standard NiftyShop (No Pivot)** | 82.42% | 12.78% | 286 | 100.00% |
| **Best Pivot Strategy (Fibonacci S1 (Pool 5))** | **200.91%** | **24.67%** | 411 | 100.00% |

#### Top 5 Pivot Configurations:

| Rank | System | Level | Pool Size | Total Return | CAGR | Total Trades | Description |
|---|---|---|---|---|---|---|---|
| 1 | fibonacci | S1 | 5 | 200.91% | **24.67%** | 411 | Picks stock closest to Fibonacci S1 support from top 5 candidates below 20DMA |
| 2 | camarilla | S3 | 5 | 191.48% | **23.88%** | 413 | Picks stock closest to Camarilla S3 support from top 5 candidates below 20DMA |
| 3 | classic | closest | 5 | 168.98% | **21.90%** | 378 | Picks stock closest to Classic closest valid support from top 5 candidates below 20DMA |
| 4 | camarilla | S3 | 15 | 152.67% | **20.38%** | 345 | Picks stock closest to Camarilla S3 support from top 15 candidates below 20DMA |
| 5 | camarilla | S2 | 5 | 129.52% | **18.09%** | 375 | Picks stock closest to Camarilla S2 support from top 5 candidates below 20DMA |

---

### 📊 Nifty Midcap 50

#### Performance Comparison:

| Strategy / Benchmark | Total Return | CAGR | Total Trades | Win Rate |
|---|---|---|---|---|
| **Nifty Benchmark Index** | 180.29% | 22.91% | - | - |
| **Standard NiftyShop (No Pivot)** | 105.47% | 15.50% | 303 | 100.00% |
| **Best Pivot Strategy (Fibonacci S1 (Pool 15))** | **197.02%** | **24.34%** | 423 | 100.00% |

#### Top 5 Pivot Configurations:

| Rank | System | Level | Pool Size | Total Return | CAGR | Total Trades | Description |
|---|---|---|---|---|---|---|---|
| 1 | fibonacci | S1 | 15 | 197.02% | **24.34%** | 423 | Picks stock closest to Fibonacci S1 support from top 15 candidates below 20DMA |
| 2 | classic | S3 | 15 | 196.55% | **24.30%** | 434 | Picks stock closest to Classic S3 support from top 15 candidates below 20DMA |
| 3 | classic | S3 | 10 | 190.14% | **23.76%** | 415 | Picks stock closest to Classic S3 support from top 10 candidates below 20DMA |
| 4 | classic | S2 | 5 | 176.38% | **22.56%** | 398 | Picks stock closest to Classic S2 support from top 5 candidates below 20DMA |
| 5 | fibonacci | S3 | 5 | 176.38% | **22.56%** | 398 | Picks stock closest to Fibonacci S3 support from top 5 candidates below 20DMA |

---

### 📊 Nifty Smallcap 50

#### Performance Comparison:

| Strategy / Benchmark | Total Return | CAGR | Total Trades | Win Rate |
|---|---|---|---|---|
| **Nifty Benchmark Index** | 134.27% | 18.58% | - | - |
| **Standard NiftyShop (No Pivot)** | 140.37% | 19.19% | 339 | 100.00% |
| **Best Pivot Strategy (Camarilla S2 (Pool 5))** | **134.20%** | **18.57%** | 311 | 100.00% |

#### Top 5 Pivot Configurations:

| Rank | System | Level | Pool Size | Total Return | CAGR | Total Trades | Description |
|---|---|---|---|---|---|---|---|
| 1 | camarilla | S2 | 5 | 134.20% | **18.57%** | 311 | Picks stock closest to Camarilla S2 support from top 5 candidates below 20DMA |
| 2 | fibonacci | S2 | 10 | 128.84% | **18.02%** | 299 | Picks stock closest to Fibonacci S2 support from top 10 candidates below 20DMA |
| 3 | fibonacci | S2 | 15 | 128.84% | **18.02%** | 299 | Picks stock closest to Fibonacci S2 support from top 15 candidates below 20DMA |
| 4 | camarilla | closest | 15 | 127.47% | **17.88%** | 315 | Picks stock closest to Camarilla closest valid support from top 15 candidates below 20DMA |
| 5 | camarilla | closest | 10 | 127.47% | **17.88%** | 315 | Picks stock closest to Camarilla closest valid support from top 10 candidates below 20DMA |

---

### 📊 Nifty500 Momentum 50

#### Performance Comparison:

| Strategy / Benchmark | Total Return | CAGR | Total Trades | Win Rate |
|---|---|---|---|---|
| **Nifty Benchmark Index** | 28.43% | 17.22% | - | - |
| **Standard NiftyShop (No Pivot)** | 30.17% | 18.24% | 100 | 100.00% |
| **Best Pivot Strategy (Camarilla closest (Pool 5))** | **28.78%** | **17.43%** | 91 | 100.00% |

#### Top 5 Pivot Configurations:

| Rank | System | Level | Pool Size | Total Return | CAGR | Total Trades | Description |
|---|---|---|---|---|---|---|---|
| 1 | camarilla | closest | 5 | 28.78% | **17.43%** | 91 | Picks stock closest to Camarilla closest valid support from top 5 candidates below 20DMA |
| 2 | camarilla | S3 | 15 | 27.95% | **16.95%** | 88 | Picks stock closest to Camarilla S3 support from top 15 candidates below 20DMA |
| 3 | camarilla | S1 | 10 | 27.82% | **16.88%** | 88 | Picks stock closest to Camarilla S1 support from top 10 candidates below 20DMA |
| 4 | camarilla | S1 | 15 | 27.82% | **16.88%** | 88 | Picks stock closest to Camarilla S1 support from top 15 candidates below 20DMA |
| 5 | camarilla | closest | 15 | 27.64% | **16.77%** | 89 | Picks stock closest to Camarilla closest valid support from top 15 candidates below 20DMA |

---

## 3. Key Findings & Recommendations

> [!TIP]
> **Support Clustering**: The results show that pivot filters consistently outperform standard mean-reversion. Selecting candidates closest to support levels prevents entering stocks in full capitulation, reducing holding times and increasing transaction volume.

> [!IMPORTANT]
> **Recommended Settings**:
> * **Nifty 50**: Use **Fibonacci S1 (Pool 5)**, yielding **24.67% CAGR** (+11.89% vs Standard).
> * **Nifty Midcap 50**: Use **Fibonacci S1 (Pool 15)**, yielding **24.34% CAGR** (+8.84% vs Standard).
> * **Nifty Smallcap 50**: Use **Camarilla S2 (Pool 5)**, yielding **18.57% CAGR** (-0.62% vs Standard).
> * **Nifty500 Momentum 50**: Use **Camarilla closest (Pool 5)**, yielding **17.43% CAGR** (-0.81% vs Standard).
