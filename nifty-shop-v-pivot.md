# NiftyShop V-Pivot Playbook

NiftyShop V-Pivot is an optimized, rule-based equity mean-reversion strategy. It enhances the standard NiftyShop strategy by introducing a **Pivot Support Entry Filter** to enter positions close to key price supports.

---

## 1. Strategy Philosophy
The strategy combines traditional mean reversion (buying oversold stocks) with structural price levels (Pivot Points):
* **Support Clustering**: A stock is most likely to experience a swift bounce-back if it is trading immediately above a major historical support line (Pivot Point).
* **Capital Velocity**: By entering trades close to supports, we reduce holding times and accelerate capital recycling.

---

## 2. Core Index & Configuration Recommendations

Based on 5-year (2021-2025) and 8-year (2018-2025) historical backtesting, the following indices and configurations are recommended:

### **A. Primary Choice: Nifty 50 (Large Cap)**
* **Recommended Pivot Config**: **Fibonacci S1 (Pool Size 5)**
* **Performance (2021-2025)**: **24.67% CAGR** (vs. 12.78% Standard NiftyShop, and 13.11% Index Buy-and-Hold)
* **Alpha vs. Index**: **+11.56% CAGR**
* **Why it works**: Large-cap stocks are highly liquid and heavily traded. Setting a tight pool size of **5** ensures we only target the most oversold heavyweights, while the **Fibonacci S1** support acts as an excellent floor for quick institutional buying.

### **B. Secondary Choice: Nifty Midcap 50 (Mid Cap)**
* **Recommended Pivot Config**: **Fibonacci S1 (Pool Size 15)**
* **Performance (2021-2025)**: **24.34% CAGR** (vs. 15.50% Standard NiftyShop, and 22.91% Index Buy-and-Hold)
* **Why it works**: Mid-cap stocks are more volatile and prone to deep corrections. A larger pool size of **15** gives the engine the flexibility to ignore "falling knives" and find a stock that is safely resting on a support level.

---

## 3. Entry Rules

1. **Universe Filter**: Only trade stocks belonging to the active Nifty 50 or Nifty Midcap 50 index.
2. **Setup Condition**: The stock's EOD Close price must be below its 20-day Simple Moving Average (20DMA).
3. **Portfolio Limit**: Maximum of 5 unique stocks held simultaneously.
4. **Candidate Selection**:
   * Rank all eligible stocks below their 20DMA by distance descending (`DiffSMA` descending).
   * Filter out stocks that are already in the portfolio.
   * Extract the **top $N$ candidates** (where $N = 5$ for Nifty 50, and $N = 15$ for Midcap 50).
5. **Support Proximity Calculation**:
   * For each of the top $N$ stocks, calculate the **Fibonacci S1** support level using the previous day's High ($H$), Low ($L$), and Close ($C$):
     $$P = \frac{H + L + C}{3}$$
     $$S_1 = P - 0.382 \times (H - L)$$
   * Ensure $\text{Close} \ge S_1$. If a stock has fallen below its $S_1$ level, it is disqualified.
   * Calculate the percentage distance to the support line:
     $$\text{Distance (\%)} = \frac{\text{Close} - S_1}{\text{Close}} \times 100$$
6. **Execution**: Buy the single stock with the **smallest distance** (closest support) at the EOD window (3:20 PM - 3:30 PM).

---

## 4. Averaging Rules (Priority Over New Entries)
If a held stock continues to fall, average down to reduce the cost basis:
* **Trigger**: A held stock drops $\ge 3\%$ below its **most recent** purchase price.
* **Execution**: Buy an additional slot of the stock at the EOD window.
* **Prioritization**: When capital is constrained, averaging down on an existing position takes priority over taking a fresh entry.

---

## 5. Exit & Profit Booking Rules
* **Target Exit**: Sell the entire position when the stock price hits a **5% profit** relative to the **weighted average buy price** (cost basis).
* **Order of Operations**: Process exits before entries. Capital from same-day exits can be instantly re-deployed.
