#!/usr/bin/env python3
"""Keep verdicts and start-year tables of the Nifty 50 momentum study (research/studies/momentum-nifty50.md).

Usage: momentum_tables.py <lottery CSV> [<sensitivity lottery CSV>...]
Reads the per-window CSV written by `research lottery -fund ...` and prints Markdown tables. The keep rule, after costs
and tax at today's rates: the median over the windows of (variant CAGR minus index CAGR) is -2.0 points or better, and
the median of (variant CAGR minus fund CAGR) is above zero. Index and fund are bought and held over the same dates.
"""
import csv
import sys
from collections import defaultdict
from statistics import median

KEEP_GAP = -0.02  # points a year behind the index still allowed
BASIS = {"tax-today": ("bench_net_today", "fund_net_today"), "tax-dated": ("bench_net_dated", "fund_net_dated"),
         "cost": ("bench_gross", "fund_gross")}
LABEL = {"tax-today": "after costs and tax at today's rates", "tax-dated": "after costs and tax at dated rates",
         "cost": "after costs, before tax"}


def load(path):
    runs = defaultdict(list)  # (variant, stage) -> rows
    order = []
    with open(path) as f:
        for r in csv.DictReader(f):
            for k in ("cagr", "maxdd", "bench_gross", "bench_net_dated", "bench_net_today", "fund_gross", "fund_net_dated", "fund_net_today"):
                r[k] = float(r[k])
            if r["variant"] not in order:
                order.append(r["variant"])
            runs[(r["variant"], r["stage"])].append(r)
    return runs, order


def pct(x):
    return f"{x * 100:.2f}%"


def pts(x):
    return f"{x * 100:+.2f}"


def share(xs):
    return f"{100 * sum(1 for x in xs if x > 0) / len(xs):.0f}%"


def summary(rows, stage):
    bk, fk = BASIS[stage]
    gi = [r["cagr"] - r[bk] for r in rows]
    gf = [r["cagr"] - r[fk] for r in rows]
    return {"cagr": median(r["cagr"] for r in rows), "p10": sorted(r["cagr"] for r in rows)[len(rows) // 10],
            "index": median(r[bk] for r in rows), "fund": median(r[fk] for r in rows),
            "gap_index": median(gi), "gap_fund": median(gf), "beats_index": share(gi), "beats_fund": share(gf),
            "worst_index": min(gi), "dd": median(r["maxdd"] for r in rows), "n": len(rows)}


def verdict(today):
    a, b = today["gap_index"] >= KEEP_GAP, today["gap_fund"] > 0
    if a and b:
        return "**kept**"
    why = []
    if not a:
        why.append("more than 2 points behind the index")
    if not b:
        why.append("behind the fund")
    return "dropped: " + " and ".join(why)


def keep_table(runs, order, title):
    n = len(runs[(order[0], "tax-today")])
    print(f"## {title}\n")
    print(f"{n} five-year windows per variant. Gaps are medians over the windows of variant CAGR minus benchmark CAGR "
          "over the same dates, in points a year. Verdict on today's rates; dated rates beside it.\n")
    print("| Variant | Median CAGR | 10th pct | Gap to index | Gap to fund | Beats index | Beats fund | Worst gap to index | Median worst fall | Gap to index (dated) | Gap to fund (dated) | Verdict |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for v in order:
        t, d = summary(runs[(v, "tax-today")], "tax-today"), summary(runs[(v, "tax-dated")], "tax-dated")
        dated = verdict(d)
        note = "" if dated.split(":")[0] == verdict(t).split(":")[0] else f" (dated rates: {dated})"
        print(f"| `{v}` | {pct(t['cagr'])} | {pct(t['p10'])} | {pts(t['gap_index'])} | {pts(t['gap_fund'])} | {t['beats_index']} | {t['beats_fund']} | "
              f"{pts(t['worst_index'])} | {pct(t['dd'])} | {pts(d['gap_index'])} | {pts(d['gap_fund'])} | {verdict(t)}{note} |")
    t0 = summary(runs[(order[0], "tax-today")], "tax-today")
    d0 = summary(runs[(order[0], "tax-dated")], "tax-dated")
    c0 = summary(runs[(order[0], "cost")], "cost")
    print(f"\nBenchmarks over the same windows (median CAGR): Nifty 50 TRI {pct(c0['index'])} before tax, {pct(d0['index'])} after tax at dated rates, "
          f"{pct(t0['index'])} at today's rates; momentum fund {pct(c0['fund'])}, {pct(d0['fund'])}, {pct(t0['fund'])}.\n")
    print("After costs, before tax (median CAGR, gap to index, gap to fund): " + "; ".join(
        f"`{v}` {pct(s['cagr'])}, {pts(s['gap_index'])}, {pts(s['gap_fund'])}" for v in order for s in [summary(runs[(v, 'cost')], 'cost')]) + ".\n")


def start_year_table(runs, order, stage, key, title):
    bk, fk = BASIS[stage]
    years = sorted({r["start"][:4] for r in runs[(order[0], stage)]})
    print(f"## {title}\n")
    print("Median of the windows starting in each year, " + LABEL[stage] + ", points a year.\n")
    print("| Variant | " + " | ".join(y[2:] for y in years) + " |")
    print("|---|" + "---|" * len(years))
    ref = bk if key == "index" else fk
    base = defaultdict(list)
    for r in runs[(order[0], stage)]:
        base[r["start"][:4]].append(r[ref])
    print(f"| {'Index' if key == 'index' else 'Fund'} CAGR, % | " + " | ".join(f"{median(base[y]) * 100:.1f}" for y in years) + " |")
    for v in order:
        by = defaultdict(list)
        for r in runs[(v, stage)]:
            by[r["start"][:4]].append(r["cagr"] - r[ref])
        print(f"| `{v}` | " + " | ".join(f"{median(by[y]) * 100:+.1f}" for y in years) + " |")
    print()


def setting(variant):
    """The four choices of a mom-<score>-top<N>-<swaps>[-ma<d>] preset."""
    p = variant.split("-")
    return {"Names held": p[2], "Swaps": {"6m": "half-yearly", "1m": "monthly"}[p[3]], "Score": p[1],
            "Market filter": "200-day" if "ma200" in p else "none"}


def effects_table(runs, order):
    print("## What each choice did\n")
    print("Mean over the 4 variants with each setting of the median gap to the index and to the fund, after costs and tax at "
          "today's rates, points a year. In the balanced half design each setting is paired equally with every setting "
          "of the other choices.\n")
    print("| Choice | Setting | Gap to index | Gap to fund | Setting | Gap to index | Gap to fund | Difference (gap to index) |")
    print("|---|---|---|---|---|---|---|---|")
    s = {v: summary(runs[(v, "tax-today")], "tax-today") for v in order}
    for choice in ("Names held", "Swaps", "Score", "Market filter"):
        vals = sorted({setting(v)[choice] for v in order}, key=lambda x: (x not in ("top10", "half-yearly", "plain", "none"), x))
        cells, means = [], []
        for val in vals:
            vs = [v for v in order if setting(v)[choice] == val]
            gi, gf = sum(s[v]["gap_index"] for v in vs) / len(vs), sum(s[v]["gap_fund"] for v in vs) / len(vs)
            cells += [val, pts(gi), pts(gf)]
            means.append(gi)
        print(f"| {choice} | " + " | ".join(cells) + f" | {pts(means[1] - means[0])} |")
    print()


def main():
    runs, order = load(sys.argv[1])
    keep_table(runs, order, "Keep verdicts")
    effects_table(runs, order)
    start_year_table(runs, order, "tax-today", "index", "Gap to the Nifty 50 TRI by start year")
    start_year_table(runs, order, "tax-today", "fund", "Gap to the momentum fund by start year")
    for path in sys.argv[2:]:
        r2, o2 = load(path)
        keep_table(r2, o2, "Sensitivity: " + path)


if __name__ == "__main__":
    main()
