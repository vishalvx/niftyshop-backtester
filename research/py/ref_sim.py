#!/usr/bin/env python3
"""Independent reference implementation of the NiftyShop rules, written from strategy.md (and, for the app modes, from
the app rules), sharing no code with the Go simulator. It runs on the daily table exported by `research exportpanel`
and prints every lot as `symbol,buy_date,sell_date,qty,buy_price` (open lots carry the last date).

Modes
  spec            strategy.md: 5% on the weighted average cost, sell the whole position; average when 3% below the MOST RECENT purchase;
                  max 5 stocks; 1 fresh entry a day; averaging before fresh entries; slot = capital / 10.
  spec-pivot:SYS:LVL:POOL   the pivot-support variant of strategy.md (pool of the N most oversold, enter the one closest to support).
  app             the app standard screeners (the app rule constants): +6% per lot with the target stored at purchase
                  (fresh lot: fill x 1.06; averaged lot: new weighted average cost x 1.06), 3% below the latest lot, 3 lots, 1 buy a day, /10, 10 stocks.
  app-vpivot:SYS:LVL:POOL   the app V-Pivot screeners: +5% synced to every lot, slot = capital / 5, 5 stocks, 3 lots, 1 buy a day.
Unspecified in the sources and chosen here the same natural way as the Go code: ties in the ranking go to the alphabetically first
symbol; with several averaging candidates the most recently bought stock goes first; a slot is floor(slot / close) shares.
"""
import csv, sys, math, collections

path, mode, capital = sys.argv[1], sys.argv[2], float(sys.argv[3])
parts = mode.split(':')
base = parts[0]
index_exit = base.endswith('-ix')          # maintainer's rule: sell the whole position when the stock leaves the index
base = base[:-3] if index_exit else base
pivot = tuple(parts[1:]) if len(parts) == 4 else None  # system, level, pool

days = collections.OrderedDict()
for r in csv.DictReader(open(path)):
    days.setdefault(r['date'], []).append(dict(sym=r['symbol'], close=float(r['close']), diff=float(r['diff_sma']), const=r['is_constituent'] == 'true',
                                                cam=float(r['cam_s1']), fib=float(r['fib_s1'])))

own_target = False
slot_on_sale = False
if base in ('spec', 'spec-pivot'):
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.05, 10.0, 5, 0, 0, False
elif base in ('cap3', 'cap3-pivot'):     # written rules plus a cap of 3 lots per stock
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.05, 10.0, 5, 3, 0, False
elif base in ('nsx', 'nsx-lot'):         # the maintainer's confirmed rule set (2026-10-03)
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.05, 10.0, 10 ** 9, 3, 1, base == 'nsx-lot'
    own_target = base == 'nsx-lot'
    index_exit = True
    slot_on_sale = True
elif base in ('lot3', 'lot3-pivot'):     # every lot sells at +5% over its OWN entry price, 3 lots per stock
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.05, 10.0, 5, 3, 0, True
    own_target = True
elif base == 'app':
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.06, 10.0, 10, 3, 1, True
elif base == 'app-vpivot':
    target, divisor, max_stocks, lot_cap, buy_cap, per_lot = 0.05, 5.0, 5, 3, 1, True
else:
    sys.exit('bad mode')
sync = base == 'app-vpivot'
latest_ref = True  # both strategy.md and the app average against the most recent purchase

cash, realized = capital, 0.0
slot = capital / divisor
last_rev = None
held = collections.OrderedDict()   # sym -> list of lots [dict(date, price, qty, target)]
out = []
seq = 0
dates = list(days.keys())


def support(d):
    sysn, lvl, _ = pivot
    if sysn == 'camarilla' and lvl == 'S1':
        return d['cam']
    if sysn == 'fibonacci' and lvl == 'S1':
        return d['fib']
    sys.exit('only S1 supports are exported')


def avg_cost(lots):
    q = sum(l['qty'] for l in lots)
    return sum(l['qty'] * l['price'] for l in lots) / q


for date in dates:
    today = {d['sym']: d for d in days[date]}
    ym = date[:7]
    if last_rev != ym and not slot_on_sale:   # monthly revision of the slot size
        slot = (capital + realized) / divisor
        last_rev = ym
    buys = 0
    # 0. a stock that left the index is sold whatever its price
    if index_exit:
        for sym in list(held.keys()):
            d = today.get(sym)
            if d is not None and not d['const']:
                for l in held[sym]:
                    cash += l['qty'] * d['close']
                    realized += l['qty'] * (d['close'] - l['price'])
                    if slot_on_sale:
                        slot = (capital + realized) / divisor
                    out.append((sym, l['date'], date, l['qty'], l['price']))
                del held[sym]
    # 1. exits first
    for sym in list(held.keys()):
        d = today.get(sym)
        if d is None:
            continue
        lots = held[sym]
        if per_lot:
            for l in list(lots):
                if d['close'] >= l['target']:
                    cash += l['qty'] * d['close']
                    realized += l['qty'] * (d['close'] - l['price'])
                    if slot_on_sale:
                        slot = (capital + realized) / divisor
                    out.append((sym, l['date'], date, l['qty'], l['price']))
                    lots.remove(l)
            if not lots:
                del held[sym]
        else:
            if d['close'] >= avg_cost(lots) * (1 + target):
                for l in lots:
                    cash += l['qty'] * d['close']
                    realized += l['qty'] * (d['close'] - l['price'])
                    if slot_on_sale:
                        slot = (capital + realized) / divisor
                    out.append((sym, l['date'], date, l['qty'], l['price']))
                del held[sym]
    # 2. averaging before fresh entries; most recently bought stock first
    order = sorted(held.keys(), key=lambda s: max(l['seq'] for l in held[s]), reverse=True) if held else []
    for sym in order:
        d = today.get(sym)
        if d is None or not d['const']:
            continue
        lots = held[sym]
        latest = max(lots, key=lambda l: l['seq'])
        if d['close'] > latest['price'] * (1 - 0.03):
            continue
        if lot_cap and len(lots) >= lot_cap:
            continue
        if buy_cap and buys >= buy_cap:
            continue
        if cash < slot:
            continue
        qty = math.floor(slot / d['close'])
        if qty <= 0:
            continue
        seq += 1
        lot = dict(date=date, price=d['close'], qty=qty, seq=seq, target=0.0)
        lots.append(lot)
        cash -= qty * d['close']
        buys += 1
        if per_lot and own_target:
            lot['target'] = d['close'] * (1 + target)
        elif per_lot:
            t = avg_cost(lots) * (1 + target)
            if sync:
                for x in lots:
                    x['target'] = t
            else:
                lot['target'] = t
    # 3. one fresh entry a day
    cands = [d for d in days[date] if d['const'] and d['diff'] > 0 and d['sym'] not in held]
    cands.sort(key=lambda d: (-d['diff'], d['sym']))
    if pivot:
        pool = cands[:int(pivot[2])]
        pool = [d for d in pool if d['close'] >= support(d) > 0]
        pool.sort(key=lambda d: ((d['close'] - support(d)) / d['close'], -d['diff']))
        cands = pool[:1]
    for d in cands:
        if buy_cap and buys >= buy_cap:
            break
        if len(held) >= max_stocks:
            break
        if cash < slot:
            break
        qty = math.floor(slot / d['close'])
        if qty <= 0:
            break
        seq += 1
        lot = dict(date=date, price=d['close'], qty=qty, seq=seq, target=d['close'] * (1 + target))
        held[d['sym']] = [lot]
        cash -= qty * d['close']
        buys += 1
        break  # max one fresh entry a day

last = dates[-1]
for sym, lots in held.items():
    for l in lots:
        out.append((sym, l['date'], last, l['qty'], l['price']))
w = csv.writer(sys.stdout)
w.writerow(['symbol', 'buy_date', 'sell_date', 'qty', 'buy_price'])
for r in sorted(out, key=lambda r: (r[1], r[0], r[3])):
    w.writerow([r[0], r[1], r[2], r[3], f'{r[4]:.4f}'])
