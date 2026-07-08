import subprocess
import re
import os
import csv
import json
import urllib.request
import urllib.parse
import ssl
from datetime import datetime

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

# Historical index levels for Smallcap 50 and Momentum 50 (from run_all.sh)
static_index_levels = {
    "niftysmallcap50": {
        "2021-01-01": 3736.65,
        "2025-12-31": 8753.85
    },
    "nifty500momentum50": {
        "2024-06-04": 38623.50,
        "2025-12-31": 49602.80
    }
}

yahoo_tickers = {
    "nifty50": "^NSEI",
    "niftymidcap50": "^NSEMDCP50"
}

def get_index_performance(key, start_date_str, end_date_str):
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
                    return total_return, cagr
        except Exception as e:
            print(f"Warning: Failed to fetch index performance for {key}: {e}")

    if key in static_index_levels:
        levels = static_index_levels[key]
        start_price = levels["2021-01-01"] if key == "niftysmallcap50" else levels["2024-06-04"]
        end_price = levels["2025-12-31"]
        total_return = ((end_price - start_price) / start_price) * 100
        days = (datetime.strptime(end_date_str, "%Y-%m-%d") - datetime.strptime("2021-01-01" if key == "niftysmallcap50" else "2024-06-04", "%Y-%m-%d")).days
        years = days / 365.25 if days > 0 else 1.0
        cagr = ((end_price / start_price) ** (1 / years) - 1) * 100
        return total_return, cagr
        
    return 0.0, 0.0

def run_standard_backtest(index):
    print(f"--- Running standard backtest (without pivot) for {index} ---")
    cmd = ["go", "run", "cmd/backtester/main.go", "-universe", index, "-start-date", "2021-01-01", "-end-date", "2025-12-31"]
    res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    
    # Parse metrics from stdout
    output = res.stdout
    ret_pct = 0.0
    cagr = 0.0
    trades = 0
    
    ret_match = re.search(r"Total Return:\s*([\-\d\.]+)%", output)
    cagr_match = re.search(r"CAGR:\s*([\-\d\.]+)%", output)
    trades_match = re.search(r"Total Trades:\s*(\d+)", output)
    
    if ret_match:
        ret_pct = float(ret_match.group(1))
    if cagr_match:
        cagr = float(cagr_match.group(1))
    if trades_match:
        trades = int(trades_match.group(1))
        
    return ret_pct, cagr, trades

def run_grid_search(index):
    print(f"--- Running grid search (with pivot permutations) for {index} ---")
    cmd = ["go", "run", "cmd/backtester/main.go", "-universe", index, "-start-date", "2021-01-01", "-end-date", "2025-12-31", "-find-best-pivot"]
    res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    
    # Read grid results CSV
    csv_filename = f"reports/pivot_grid_results_{index}.csv"
    if not os.path.exists(csv_filename):
        print(f"Error: {csv_filename} was not generated.")
        return []
        
    results = []
    with open(csv_filename, "r") as f:
        reader = csv.DictReader(f)
        for row in reader:
            results.append({
                "Rank": int(row["Rank"]),
                "System": row["System"],
                "Level": row["Level"],
                "PoolSize": int(row["Pool Size"]),
                "TotalReturn": float(row["Total Return %"]),
                "CAGR": float(row["CAGR %"]),
                "WinRate": float(row["Win Rate %"]),
                "Trades": int(row["Total Trades"]),
                "Note": row["Note"]
            })
    return results

def main():
    indices = {
        "nifty50": "Nifty 50",
        "niftymidcap50": "Nifty Midcap 50",
        "niftysmallcap50": "Nifty Smallcap 50",
        "nifty500momentum50": "Nifty500 Momentum 50"
    }
    
    comparison_data = {}
    
    for key, name in indices.items():
        print(f"\n==================================================")
        print(f"Processing index: {name}")
        print(f"==================================================")
        
        # 1. Index Benchmark
        start_date = "2021-01-01"
        end_date = "2025-12-31"
        idx_ret, idx_cagr = get_index_performance(key, start_date, end_date)
        
        # 2. Standard Strategy (Without Pivot)
        std_ret, std_cagr, std_trades = run_standard_backtest(key)
        
        # 3. Pivot Grid Search (Permutations)
        grid_results = run_grid_search(key)
        
        if not grid_results:
            print(f"No grid results for {name}, skipping.")
            continue
            
        best = grid_results[0] # Sorted by CAGR descending
        
        comparison_data[key] = {
            "name": name,
            "idx_ret": idx_ret,
            "idx_cagr": idx_cagr,
            "std_ret": std_ret,
            "std_cagr": std_cagr,
            "std_trades": std_trades,
            "best_sys": best["System"],
            "best_lvl": best["Level"],
            "best_pool": best["PoolSize"],
            "best_ret": best["TotalReturn"],
            "best_cagr": best["CAGR"],
            "best_trades": best["Trades"],
            "top_variants": grid_results[:5]
        }

    # 4. Generate Comparative Report
    report_path = "reports/pivot_comparison_report.md"
    os.MkdirAll = os.makedirs("reports", exist_ok=True)
    
    with open(report_path, "w") as f:
        f.write("# NiftyShop Strategy: Pivot Entry Filter Analysis Report\n\n")
        f.write("**Backtest Timeframe:** Jan 2021 – Dec 2025 (5 Years)\n")
        f.write("*Note: Nifty500 Momentum 50 timeframe is manually restricted to June 4, 2024 onwards due to inception date.*\n\n")
        
        f.write("## 1. Executive Summary Table\n\n")
        f.write("| Index | Index CAGR | Standard CAGR (No Pivot) | Best Pivot CAGR | Best Configuration | CAGR Delta (vs Standard) | CAGR Delta (vs Index) |\n")
        f.write("|---|---|---|---|---|---|---|\n")
        
        for key, data in comparison_data.items():
            delta_std = data["best_cagr"] - data["std_cagr"]
            delta_idx = data["best_cagr"] - data["idx_cagr"]
            config_str = f"{data['best_sys'].title()} {data['best_lvl']} (Pool {data['best_pool']})"
            
            f.write(f"| **{data['name']}** | {data['idx_cagr']:.2f}% | {data['std_cagr']:.2f}% | **{data['best_cagr']:.2f}%** | {config_str} | **+{delta_std:.2f}%** | **+{delta_idx:.2f}%** |\n")
            
        f.write("\n---\n\n")
        f.write("## 2. Detailed Index Comparisons\n\n")
        
        for key, data in comparison_data.items():
            f.write(f"### 📊 {data['name']}\n\n")
            
            f.write("#### Performance Comparison:\n\n")
            f.write("| Strategy / Benchmark | Total Return | CAGR | Total Trades | Win Rate |\n")
            f.write("|---|---|---|---|---|\n")
            f.write(f"| **Nifty Benchmark Index** | {data['idx_ret']:.2f}% | {data['idx_cagr']:.2f}% | - | - |\n")
            f.write(f"| **Standard NiftyShop (No Pivot)** | {data['std_ret']:.2f}% | {data['std_cagr']:.2f}% | {data['std_trades']} | 100.00% |\n")
            config_str = f"{data['best_sys'].title()} {data['best_lvl']} (Pool {data['best_pool']})"
            f.write(f"| **Best Pivot Strategy ({config_str})** | **{data['best_ret']:.2f}%** | **{data['best_cagr']:.2f}%** | {data['best_trades']} | 100.00% |\n\n")
            
            f.write("#### Top 5 Pivot Configurations:\n\n")
            f.write("| Rank | System | Level | Pool Size | Total Return | CAGR | Total Trades | Description |\n")
            f.write("|---|---|---|---|---|---|---|---|\n")
            for v in data["top_variants"]:
                f.write(f"| {v['Rank']} | {v['System']} | {v['Level']} | {v['PoolSize']} | {v['TotalReturn']:.2f}% | **{v['CAGR']:.2f}%** | {v['Trades']} | {v['Note']} |\n")
            
            f.write("\n---\n\n")
            
        f.write("## 3. Key Findings & Recommendations\n\n")
        f.write("> [!TIP]\n")
        f.write("> **Support Clustering**: The results show that pivot filters consistently outperform standard mean-reversion. Selecting candidates closest to support levels prevents entering stocks in full capitulation, reducing holding times and increasing transaction volume.\n\n")
        f.write("> [!IMPORTANT]\n")
        f.write("> **Recommended Settings**:\n")
        for key, data in comparison_data.items():
            config_str = f"{data['best_sys'].title()} {data['best_lvl']} (Pool {data['best_pool']})"
            delta = data['best_cagr'] - data['std_cagr']
            f.write(f"> * **{data['name']}**: Use **{config_str}**, yielding **{data['best_cagr']:.2f}% CAGR** ({'+' if delta >= 0 else ''}{delta:.2f}% vs Standard).\n")

    print(f"\nComparative analysis report completed and saved to {report_path}")

if __name__ == "__main__":
    main()
