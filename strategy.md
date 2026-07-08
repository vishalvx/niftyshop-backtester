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
3. **Portfolio Limit**: Maximum of 5 unique stocks held simultaneously.
4. **Ranking & Selection**:
   * **Standard Strategy**: Rank all eligible stocks below their 20DMA from the largest percentage deviation to the smallest. Enter a fresh buy in the top-ranked stock that is **not** already in the portfolio.
   * **Pivot Support Filter Variant**: Take the top $N$ (e.g. 5, 10, 15) eligible stocks ranked by SMA deviation. For each stock in this pool, calculate the distance from Close to the configured pivot support line (Classic, Fibonacci, or Camarilla). Enter the stock that is closest to its support level (i.e. smallest distance percentage), ensuring we enter near strong support.

---

## 4. Averaging Rules
If a held stock continues to fall, the strategy systematically averages down to reduce the cost basis:
* **Trigger**: A held stock drops $\ge 3\%$ below its **most recent** purchase price.
* **Execution**: Buy an additional unit of the stock at the EOD window.
* **Prioritization**: When capital is constrained, averaging down on an existing position takes priority over taking a fresh entry.

---

## 5. Exit & Profit Booking Rules
* **Target Exit**: Sell the entire position when the stock price hits a **5% profit** relative to the **weighted average buy price** (cost basis).
* **Order of Operations**: Always process exits before entries during the 3:20 PM – 3:30 PM window. Redeemed capital can be instantly deployed into fresh entries or averaging on the same day.

---

## 6. Capital Allocation
* **Capital Divider**: The total capital is divided by 10 to determine the base slot size.
* **Portfolio Cap**: With 5 slots dedicated to unique stocks, the remaining capital is reserved as a safety buffer for averaging down.
