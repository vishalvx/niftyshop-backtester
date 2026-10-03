# NiftyShop Strategy Playbook

NiftyShop is a mechanical, rule-based equity mean-reversion strategy designed for quality liquid Indian market indices.

---

## 1. Strategy Philosophy
The strategy exploits temporary market oversold conditions in fundamentally strong, large-to-mid-cap stocks:
* **Mean Reversion**: High-quality stocks that drop significantly below their moving averages have a high statistical probability of reverting back to their mean.
* **No Discretion**: Execution is strictly mechanical and rule-based, taking less than 10 minutes daily.
* **Execution Window**: 3:20 PM – 3:30 PM IST (EOD market close) on trading days.

---

## 2. Core Indicators
* **20-Day Moving Average (20DMA)**: Serves as the baseline "fair value" reference point.
* **% Distance Below 20DMA**:
  $$\% \text{ Below 20DMA} = \frac{20DMA - \text{Current Price}}{20DMA} \times 100$$
  A positive value means the stock is trading below its 20DMA (potential entry candidate).

---

## 3. Entry Rules
1. **Universe Filter**: Only trade stocks belonging to the configured index (e.g., Nifty 50, Nifty Midcap 50, Nifty Smallcap 50, Nifty500 Momentum 50).
2. **Setup Condition**: The stock's current price must be below its 20DMA.
3. **Portfolio Limit**: None on the number of stocks held; only cash limits it (see section 6). Earlier versions of this document said a maximum of 5 stocks; the confirmed rule set of 2026-10-03 (section 7) removed that cap.
4. **Ranking & Selection**:
   * **Standard Strategy**: Rank all eligible stocks below their 20DMA from the largest percentage deviation to the smallest. Enter a fresh buy in the top-ranked stock that is **not** already in the portfolio.
   * **Pivot Support Filter Variant**: Take the top $N$ (e.g. 5, 10, 15) eligible stocks ranked by SMA deviation. For each stock in this pool, calculate the distance from Close to the configured pivot support line (Classic, Fibonacci, or Camarilla). Enter the stock that is closest to its support level (i.e. smallest distance percentage), ensuring we enter near strong support.

---

## 4. Averaging Rules
If a held stock continues to fall, the strategy systematically averages down to reduce the cost basis:
* **Trigger**: A held stock drops $\ge 3\%$ below its **most recent** purchase price.
* **Execution**: Buy an additional unit of the stock at the EOD window. At most 3 lots may be open per stock (1 fresh buy plus 2 adds); a lot that has been sold frees a place.
* **Prioritization**: When capital is constrained, averaging down on an existing position takes priority over taking a fresh entry.

---

## 5. Exit & Profit Booking Rules
* **Target Exit**: Sell the entire position when the stock price hits a **5% profit** relative to the **weighted average buy price** (cost basis). Studied variant: every lot sells on its own at +5% over its own entry price.
* **Index Exit**: A stock that leaves the index is sold, whole position, that day at any price.
* **No Stop Loss, No Time Limit**: nothing else sells a position.
* **Order of Operations**: Always process exits before entries during the 3:20 PM – 3:30 PM window. Redeemed capital can be instantly deployed into fresh entries or averaging on the same day.

---

## 6. Capital Allocation
* **Capital Divider**: The total capital is divided by 10 to determine the base slot size.
* **Refresh**: Capital for the slot size is the start money plus profit booked, refreshed after every sale.
* **No Stock Cap**: Only cash limits the number of stocks held (10 slots of capital / 10 allow up to 10 different stocks).

---

## 7. Confirmed Rule Set (2026-10-03)

The maintainer confirmed these eight rules; the simulator's `nsx` presets implement them and an independent second simulator matches it lot for lot.

* **R1 Universe and timing**: stocks of the index (point-in-time monthly lists), one check a day near the close.
* **R2 New stock**: the stock furthest below its 20-day average that is not held; no cap on stocks held, only cash limits it (capital / 10 per buy).
* **R3 Size of a buy**: capital / 10; capital = start money plus profit booked, refreshed after every sale.
* **R4 Add**: close at least 3% below the latest open lot; at most 3 open lots per stock.
* **R5 Sell: profit**: whole position at +5% over average cost (or, as a studied variant, each lot at +5% over its own entry).
* **R6 Sell: index**: a stock that left the index is sold, whole position, at any price.
* **R7 No stop, no time limit**.
* **R8 Order of the day**: any number of exits first, then at most one purchase a day in total (an add before a new stock).

Not specified: when several held stocks qualify for the day's single add, the simulator takes the stock whose lot was bought most recently.

**Performance.** These rules do not beat the index after costs and tax over the long run; see `reports/long-run-findings.md`.
