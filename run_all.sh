#!/bin/bash
set -e

echo "Cleaning up temporary logs..."
rm -f temp_*.log

echo "Starting backtests..."
for index in nifty50 niftymidcap50 niftysmallcap50 nifty500momentum50; do
    echo "Running backtest for $index..."
    go run cmd/backtester/main.go -universe $index -start-date 2021-01-01 -end-date 2025-12-31 > temp_${index}.log
done

echo "Compiling consolidated report.md..."
python3 -c '
import re
import os
import json
import urllib.request
import urllib.parse
import ssl
from datetime import datetime

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

# Hardcoded index levels for Smallcap 50 and Momentum 50 for standard periods
static_index_levels = {
    "niftysmallcap50": {
        "2021-01-01": 3736.65,
        "2021-01-04": 3736.65,
        "2025-12-31": 8753.85,
        "2025-12-30": 8753.85
    },
    "nifty500momentum50": {
        "2024-06-04": 38623.50,
        "2025-12-31": 49602.80,
        "2025-12-30": 49602.80
    }
}

yahoo_tickers = {
    "nifty50": "^NSEI",
    "niftymidcap50": "^NSEMDCP50"
}

def get_index_performance(key, start_date_str, end_date_str):
    # Try dynamic fetch for Nifty 50 and Midcap 50
    if key in yahoo_tickers:
        ticker = yahoo_tickers[key]
        url = f"https://query2.finance.yahoo.com/v8/finance/chart/{urllib.parse.quote(ticker)}?period1=1609459200&period2=1767225600&interval=1d"
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        try:
            with urllib.request.urlopen(req, context=ctx) as response:
                data = json.loads(response.read().decode())
                timestamps = data["chart"]["result"][0]["timestamp"]
                closes = data["chart"]["result"][0]["indicators"]["quote"][0]["close"]
                
                target_start = datetime.strptime(start_date_str, "%Y-%m-%d").timestamp()
                target_end = datetime.strptime(end_date_str, "%Y-%m-%d").timestamp()
                
                start_price, end_price = None, None
                best_start_diff, best_end_diff = float("inf"), float("inf")
                
                for ts, close in zip(timestamps, closes):
                    if close is None:
                        continue
                    start_diff = ts - target_start
                    if start_diff >= 0 and start_diff < best_start_diff:
                        best_start_diff = start_diff
                        start_price = close
                    end_diff = target_end - ts
                    if end_diff >= 0 and end_diff < best_end_diff:
                        best_end_diff = end_diff
                        end_price = close
                
                if start_price and end_price:
                    total_return = ((end_price - start_price) / start_price) * 100
                    days = (datetime.strptime(end_date_str, "%Y-%m-%d") - datetime.strptime(start_date_str, "%Y-%m-%d")).days
                    years = days / 365.25 if days > 0 else 1.0
                    cagr = ((end_price / start_price) ** (1 / years) - 1) * 100
                    return f"{total_return:.2f}%", f"{cagr:.2f}%"
        except Exception:
            pass

    # Use static lookups for smallcap50/momentum50 if dates match
    if key in static_index_levels:
        levels = static_index_levels[key]
        start_price = levels.get(start_date_str)
        end_price = levels.get(end_date_str)
        
        # Fallbacks for exact start/end dates if not exactly matching key (e.g. 2021-01-01 vs 2021-01-04)
        if not start_price and key == "niftysmallcap50":
            start_price = levels.get("2021-01-01") or levels.get("2021-01-04")
        if not end_price and key == "niftysmallcap50":
            end_price = levels.get("2025-12-31") or levels.get("2025-12-30")
            
        if start_price and end_price:
            total_return = ((end_price - start_price) / start_price) * 100
            days = (datetime.strptime(end_date_str, "%Y-%m-%d") - datetime.strptime(start_date_str, "%Y-%m-%d")).days
            years = days / 365.25 if days > 0 else 1.0
            cagr = ((end_price / start_price) ** (1 / years) - 1) * 100
            return f"{total_return:.2f}%", f"{cagr:.2f}%"
            
    return "N/A", "N/A"

indices = {
    "nifty50": "Nifty 50",
    "niftymidcap50": "Midcap 50",
    "niftysmallcap50": "Smallcap 50",
    "nifty500momentum50": "NIFTY500 Momentum 50"
}

data = []

for key, name in indices.items():
    filename = f"temp_{key}.log"
    if not os.path.exists(filename):
        print(f"Warning: {filename} not found.")
        continue
    with open(filename, "r") as f:
        content = f.read()
    
    # Extract values using regex
    start_capital = re.search(r"Start Capital:\s*₹?([\d\.]+)", content)
    final_value = re.search(r"Final Value:\s*₹?([\d\.]+)", content)
    total_return = re.search(r"Total Return:\s*([\-\d\.]+)%", content)
    cagr = re.search(r"CAGR:\s*([\-\d\.]+)%", content)
    total_trades = re.search(r"Total Trades:\s*(\d+)", content)
    fresh_buys = re.search(r"Fresh Buys:\s*(\d+)", content)
    avg_buys = re.search(r"Avg Buys:\s*(\d+)", content)
    sells = re.search(r"Sells:\s*(\d+)", content)
    win_rate = re.search(r"Win Rate:\s*([\d\.]+)%", content)
    min_hold = re.search(r"Min Holding:\s*(\d+) days", content)
    avg_hold = re.search(r"Avg Holding:\s*(\d+) days", content)
    max_hold = re.search(r"Max Holding:\s*(\d+) days", content)

    # Extract dates from log
    dates_match = re.search(r"historical data \(from ([\d\-]+) to ([\d\-]+)\)", content)
    start_date_str = dates_match.group(1) if dates_match else "2021-01-01"
    end_date_str = dates_match.group(2) if dates_match else "2025-12-31"

    idx_return, idx_cagr = get_index_performance(key, start_date_str, end_date_str)

    data.append({
        "name": name,
        "start_cap": f"₹{float(start_capital.group(1)):,.2f}" if start_capital else "-",
        "final_val": f"₹{float(final_value.group(1)):,.2f}" if final_value else "-",
        "total_ret": f"{total_return.group(1)}%" if total_return else "-",
        "idx_ret": idx_return,
        "cagr": f"{cagr.group(1)}%" if cagr else "-",
        "idx_cagr": idx_cagr,
        "trades": total_trades.group(1) if total_trades else "-",
        "fresh": fresh_buys.group(1) if fresh_buys else "-",
        "avg_buys": avg_buys.group(1) if avg_buys else "-",
        "sells": sells.group(1) if sells else "-",
        "win": f"{win_rate.group(1)}%" if win_rate else "-",
        "min_hold": f"{min_hold.group(1)} days" if min_hold else "-",
        "avg_hold": f"{avg_hold.group(1)} days" if avg_hold else "-",
        "max_hold": f"{max_hold.group(1)} days" if max_hold else "-"
    })

# Write the report.md file
with open(os.path.join("reports", "report.md"), "w") as out:
    out.write("# NiftyShop Strategy — Multi-Index Backtest Report\n\n")
    out.write("**Period:** Jan 2021 – Dec 2025 (5 Years)\n")
    out.write("**Parameters:** 20-Day SMA, 5% Profit Target, 3% Average Down Trigger, Max 5 Stocks, Capital Divider 10\n\n")
    
    out.write("## 1. Executive Summary Comparison\n\n")
    out.write("| Index | Start Capital | Final Portfolio Value | Strategy Return | Index Return | Strategy CAGR | Index CAGR | Total Trades | Win Rate | Avg Hold | Max Hold |\n")
    out.write("|---|---|---|---|---|---|---|---|---|---|---|\n")
    for d in data:
        out.write("| **{}** | {} | {} | **{}** | **{}** | **{}** | **{}** | {} | {} | {} | {} |\n".format(
            d["name"], d["start_cap"], d["final_val"], d["total_ret"], d["idx_ret"], d["cagr"], d["idx_cagr"],
            d["trades"], d["win"], d["avg_hold"], d["max_hold"]
        ))
    
    out.write("\n## 2. Detailed Outputs\n\n")
    for key, name in indices.items():
        out.write(f"### 📊 {name}\n\n")
        out.write("```\n")
        with open(f"temp_{key}.log", "r") as f:
            lines = f.readlines()
        # Find start of BACKTEST RESULTS
        started = False
        for line in lines:
            if "BACKTEST RESULTS" in line:
                started = True
            if started:
                out.write(line)
        out.write("```\n\n")
        out.write("---\n\n")

print("report.md generated successfully!")
'

echo "Cleaning up temporary logs..."
rm -f temp_*.log
echo "Done!"
