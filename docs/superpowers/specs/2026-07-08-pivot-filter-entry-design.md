# Design Specification: Pivot Point Support Filter for NiftyShop Entry

This document outlines the design for incorporating a pivot-point-based support filter into the NiftyShop mean-reversion strategy. The goal is to prioritize entering stocks that are trading closest to a key support level to maximize bounce-back probability and minimize drawdown.

---

## 1. Background & Rationale
Currently, the NiftyShop strategy ranks all eligible stocks below their 20-day moving average (20DMA) by distance below the SMA descending. It then enters the top-ranked stock that is not already in the portfolio.

While this ensures we buy the most oversold stocks, it doesn't consider support and resistance levels. A stock that is slightly less oversold but sitting exactly on a strong pivot support level might be a higher-probability bounce candidate than a stock falling through empty space.

By applying a pivot support filter, we select stocks from a candidate pool that are nearest to a support line.

---

## 2. Pivot Level Formulas
For any given trading day, we calculate pivot support levels using the previous day's High ($H$), Low ($L$), and Close ($C$).

### A. Classic Pivot Point Levels
1. **Pivot Point ($P$)**:
   $$P = \frac{H + L + C}{3}$$
2. **Support 1 ($S_1$)**:
   $$S_1 = 2P - H$$
3. **Support 2 ($S_2$)**:
   $$S_2 = P - (H - L)$$
4. **Support 3 ($S_3$)**:
   $$S_3 = L - 2(H - P)$$

### B. Fibonacci Pivot Point Levels
1. **Pivot Point ($P$)**:
   $$P = \frac{H + L + C}{3}$$
2. **Support 1 ($S_1$)**:
   $$S_1 = P - 0.382 \times (H - L)$$
3. **Support 2 ($S_2$)**:
   $$S_2 = P - 0.618 \times (H - L)$$
4. **Support 3 ($S_3$)**:
   $$S_3 = P - 1.000 \times (H - L)$$

### C. Camarilla Pivot Point Levels
1. **Support 1 ($S_1$)**:
   $$S_1 = C - (H - L) \times \frac{1.1}{12}$$
2. **Support 2 ($S_2$)**:
   $$S_2 = C - (H - L) \times \frac{1.1}{6}$$
3. **Support 3 ($S_3$)**:
   $$S_3 = C - (H - L) \times \frac{1.1}{4}$$
4. **Support 4 ($S_4$)**:
   $$S_4 = C - (H - L) \times \frac{1.1}{2}$$

---

## 3. Pivot Distance Metric
We filter for support levels that are strictly below or equal to the current Close price ($\text{Close} \ge \text{Support}$) to ensure the level acts as active downside support.

The distance metric is calculated as the percentage difference relative to the Close price:
$$\text{Distance (\%)} = \frac{\text{Close} - \text{Support}}{\text{Close}} \times 100$$

If the Close price is below the target support level, the distance is invalid and the stock is disqualified from entry under that configuration.

---

## 4. Entry Decision Algorithm
On any trading day:
1. Identify all stocks that:
   * Are active constituents of the index.
   * Are below their 20DMA (`DiffSMA > 0`).
   * Are not currently held in the portfolio.
2. Select the top $N$ stocks (candidate pool size where $N \in \{5, 10, 15\}$) ranked by `DiffSMA` descending.
3. For each stock in this pool, evaluate the target support level based on the configuration:
   * **Specific Level (e.g., S2)**: If $\text{Close} \ge \text{Support}$, calculate the distance percentage. Else, mark as invalid.
   * **Closest Level**: Find the support level ($S_i \le \text{Close}$) for the active system that minimizes the distance to $\text{Close}$.
4. If a stock has a valid distance, keep it as an active candidate. If none do, skip entries for the day.
5. Sort the active candidates by distance ascending (smallest distance first).
6. Enter the top-ranked stock (subject to slot size and capital limits).

---

## 5. Proposed Code Changes

### A. Configuration updates (`internal/config`)
We will add `PivotFilterConfig` fields to `Config` in `internal/config/config.go` and validate them.

### B. Precomputing Indicator Values (`cmd/backtester/main.go`)
During the daily data loading and preparation phase:
* Fetch `bars[i-1]` (previous day) for any bar `i > 0`.
* Calculate all Classic, Fibonacci, and Camarilla support levels.
* Assign them to the new fields on the `EngineStock` struct.

### C. Engine Strategy Execution (`internal/engine/engine.go`)
* Implement the candidate pool extraction and distance sorting logic inside `RunNiftyShop`.

### D. Grid-Search Runner
Add a `-find-best-pivot` flag in `cmd/backtester/main.go`. When executed:
* The runner runs a simulation loop over all 39 permutations.
* It collects the results and writes a sorted Markdown table to stdout.

---

## 6. Permutations Grid
Total permutations to evaluate (39):
* **Classic**: $S_1$, $S_2$, $S_3$, "closest" (4 levels) $\times$ $\{5, 10, 15\}$ pool sizes = 12 permutations.
* **Fibonacci**: $S_1$, $S_2$, $S_3$, "closest" (4 levels) $\times$ $\{5, 10, 15\}$ pool sizes = 12 permutations.
* **Camarilla**: $S_1$, $S_2$, $S_3$, $S_4$, "closest" (5 levels) $\times$ $\{5, 10, 15\}$ pool sizes = 15 permutations.

---

## 7. Verification Plan
* **Unit Tests**: Add tests in `internal/engine/engine_test.go` verifying the pivot calculations and the candidate pool selection logic.
* **Grid Search Run**: Run the `-find-best-pivot` search on the `nifty50` universe to ensure the grid search compiles, executes without panic, and lists the sorted outcomes.
