#!/usr/bin/env python3
"""Trade-ledger metrics in the style a retail backtest report prints: XIRR of the trade cash flows, average holding days of
closed lots, net P&L as a share of start capital, and the drawdown of REALISED P&L only. Reads `research score -export` output.
Usage: ledger_metrics.py <export_dir> [...]"""
import csv, datetime as dt, glob, json, sys


def d(s):
    return dt.date.fromisoformat(s)


def xirr(cfs):
    t0 = min(t for t, _ in cfs)

    def f(r):
        return sum(c / ((1 + r) ** ((t - t0).days / 365.0)) for t, c in cfs)
    lo, hi = -0.9, 20.0
    flo = f(lo)
    for _ in range(120):
        mid = (lo + hi) / 2
        fm = f(mid)
        if flo * fm <= 0:
            hi = mid
        else:
            lo, flo = mid, fm
    return (lo + hi) / 2


def metrics(run, cap=1_000_000):
    lots = list(csv.DictReader(open(glob.glob(run + '/*_lots.csv')[0])))
    eq = list(csv.DictReader(open(glob.glob(run + '/*_equity.csv')[0])))
    cfs, closed, realized = [], [], []
    for l in lots:
        q, bp, sp = float(l['qty']), float(l['buy_price']), float(l['sell_price'])
        cfs.append((d(l['buy_date']), -q * bp))
        cfs.append((d(l['sell_date']), q * sp))
        if l['open'] == 'false':
            closed.append((d(l['sell_date']) - d(l['buy_date'])).days)
            realized.append((d(l['sell_date']), q * (sp - bp)))
    realized.sort()
    cum = peak = dd = 0.0
    for _, p in realized:
        cum += p
        peak = max(peak, cum)
        dd = min(dd, (cum - peak) / (cap + peak))
    last = float(eq[-1]['equity'])
    unreal = sum(float(l['qty']) * (float(l['sell_price']) - float(l['buy_price'])) for l in lots if l['open'] == 'true')
    return dict(xirr=xirr(cfs), avg_hold_days=sum(closed) / len(closed), net_pnl_pct_of_start=(last - cap) / cap, realised_pnl_max_drawdown=dd,
                closed_lots=len(closed), open_lots=sum(1 for l in lots if l['open'] == 'true'), open_unrealised_pct_of_start=unreal / cap, final_equity=last)


out = {}
for run in sys.argv[1:]:
    out[run.rstrip('/').split('/')[-1]] = metrics(run)
print(json.dumps(out, indent=1))
