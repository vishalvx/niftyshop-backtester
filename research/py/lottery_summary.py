import pandas as pd, sys
names={'standard-legacy':'Standard, engine rules','standard-spec':'Standard, documented rules','vpivot-camarilla-S1-5':'V-Pivot Camarilla S1 pool 5 (engine rules)','vpivot-fibonacci-S1-15':'V-Pivot Fibonacci S1 pool 15 (engine rules)','app-rules':'the app smallcap-style rules','app-vpivot-nifty50':'the app Nifty 50 V-Pivot rules'}
for h in (3,5,10):
    df=pd.read_csv(f'research/out/final/lottery_nifty50_{h}y.csv')
    n=df[df.variant=='standard-legacy'].groupby('stage').size().iloc[0]
    print(f'\n#### {h}-year windows, {n} start months, Jan 2008 to Aug {2025-h}\n')
    print('| strategy | median CAGR before costs | after costs | after costs and tax | 10th percentile (after tax) | 90th percentile (after tax) | windows with >= 15% after tax | windows with a loss after tax | windows beating Nifty 50 TRI after tax |')
    print('|---|---|---|---|---|---|---|---|---|')
    for v,nm in names.items():
        g=df[df.variant==v]
        if g.empty: continue
        med={s:g[g.stage==s].cagr.median() for s in ('gross','cost','tax-dated')}
        t=g[g.stage=='tax-dated']
        beat=(t.cagr>t.bench_net_dated).mean()
        print(f"| {nm} | {med['gross']*100:.1f}% | {med['cost']*100:.1f}% | {med['tax-dated']*100:.1f}% | {t.cagr.quantile(.1)*100:.1f}% | {t.cagr.quantile(.9)*100:.1f}% | {(t.cagr>=.15).mean()*100:.0f}% | {(t.cagr<0).mean()*100:.0f}% | {beat*100:.0f}% |")
    t=df[(df.variant=='standard-legacy')&(df.stage=='tax-dated')]
    b=t.bench_gross; bn=t.bench_net_dated; bt=t.bench_net_today
    print(f"| **Nifty 50 total-return index, buy and hold** | {b.median()*100:.1f}% | n/a | {bn.median()*100:.1f}% (today's rates {bt.median()*100:.1f}%) | {bn.quantile(.1)*100:.1f}% | {bn.quantile(.9)*100:.1f}% | {(bn>=.15).mean()*100:.0f}% | {(bn<0).mean()*100:.0f}% | - |")
