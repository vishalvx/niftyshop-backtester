#!/usr/bin/env python3
"""Summarise `research score` outputs: one row per variant with CAGR at the four stages, risk, benchmark CAGR.
Usage: winners_tables.py <score_file> [...]"""
import re, sys

def parse(path):
    rows, cur = {}, None
    for line in open(path):
        m = re.match(r"=== (\S+) \| stage (\S+) \| (\S+) \.\. (\S+) \(([\d.]+) years\) ===", line)
        if m:
            cur = rows.setdefault(m[1], {"years": float(m[5]), "start": m[3], "end": m[4], "stages": {}})
            st = cur["stages"].setdefault(m[2], {})
            continue
        if cur is None:
            continue
        st = cur["stages"][list(cur["stages"])[-1]] if False else None
    return rows

def parse2(path):
    out, v, st = {}, None, None
    for line in open(path):
        m = re.match(r"=== (\S+) \| stage (\S+) \| (\S+) \.\. (\S+) \(([\d.]+) years\) ===", line)
        if m:
            v = out.setdefault(m[1], {"years": float(m[5]), "start": m[3], "end": m[4]})
            st = m[2]
            v[st] = {}
            continue
        if v is None or st is None:
            continue
        d = v[st]
        m = re.search(r"final value Rs (\d+) .*?CAGR\s+(-?[\d.]+)%", line)
        if m: d["final"], d["cagr"] = int(m[1]), float(m[2])
        m = re.search(r"Sharpe (-?[\d.]+)  Sortino (-?[\d.]+)  Calmar (-?[\d.]+)", line)
        if m: d["sharpe"], d["sortino"], d["calmar"] = map(float, m.groups())
        m = re.search(r"max drawdown\s+(-?[\d.]+)%.*?underwater (\d+) days", line)
        if m: d["maxdd"], d["uw"] = float(m[1]), int(m[2])
        m = re.search(r"longest underwater stretch (\d+) days \(([\d.]+) years\)", line)
        if m: d["uwlong"] = float(m[2])
        m = re.search(r"bench CAGR\s+(-?[\d.]+)%", line)
        if m: d["bench"] = float(m[1])
        m = re.search(r"annual vol\s+([\d.]+)%", line)
        if m: d["vol"] = float(m[1])
    return out

for path in sys.argv[1:]:
    r = parse2(path)
    print(f"\n### {path}\n")
    print("| variant | window | gross CAGR | after costs | after tax (dated) | after tax (today) | Sharpe (costs) | max DD (costs) | longest underwater (yrs) | index CAGR (gross, same dates) |")
    print("|---|---|---|---|---|---|---|---|---|---|")
    for k, v in r.items():
        g = lambda s, f: (f"{v[s][f]:.2f}%" if s in v and f in v[s] else "-")
        sh = v.get("cost", {}).get("sharpe", float("nan"))
        print(f"| {k} | {v['start']}..{v['end']} ({v['years']:.1f}y) | {g('gross','cagr')} | {g('cost','cagr')} | {g('tax-dated','cagr')} | {g('tax-today','cagr')} | {sh:.2f} | {g('cost','maxdd')} | {v.get('cost',{}).get('uwlong','-')} | {g('gross','bench')} |")
