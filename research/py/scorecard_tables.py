"""Turn research/out/final/scorecard_nifty50_<variant>.txt into markdown tables."""
import re, sys, glob
variants=sys.argv[1:] or ['legacy']
def parse(path):
    txt=open(path).read()
    blocks=re.split(r'\n(?==== )',txt.strip())
    out={}
    for b in blocks:
        m=re.match(r'=== (\S+) \| stage (\S+) \| (\S+) \.\. (\S+) \(([\d.]+) years\) ===',b)
        if not m: continue
        v,stage=m.group(1),m.group(2)
        d={}
        g=lambda pat,default='': (re.search(pat,b).group(1) if re.search(pat,b) else default)
        d['final']=g(r'final value Rs (\d+)')
        d['cagr']=g(r'CAGR\s+(-?[\d.]+%)')
        d['vol']=g(r'annual vol\s+(-?[\d.]+%)')
        d['sharpe']=g(r'Sharpe (-?[\d.]+)'); d['sortino']=g(r'Sortino (-?[\d.]+|inf)'); d['calmar']=g(r'Calmar (-?[\d.]+)')
        d['ulcer']=g(r'Ulcer ([\d.]+%)'); d['upi']=g(r'UPI (-?[\d.]+)')
        d['maxdd']=g(r'max drawdown (-?[\d.]+%)')
        d['dd_peak']=g(r'peak (\d{4}-\d\d-\d\d) -> trough'); d['dd_trough']=g(r'trough (\d{4}-\d\d-\d\d)')
        d['dd_rec']=g(r'recovered (\d{4}-\d\d-\d\d) \(',  'not recovered')
        d['uw']=g(r'longest underwater stretch (\d+) days')
        d['best']=g(r'best year (\d+\s+-?[\d.]+%)'); d['worst']=g(r'worst year (\d+\s+-?[\d.]+%)'); d['posm']=g(r'positive months\s+([\d.]+%)')
        d['r1']=g(r'rolling 1y CAGR: windows \d+  min\s+(-?[\d.]+%)  median\s+(-?[\d.]+%)'); 
        for n in (1,3,5):
            mm=re.search(rf'rolling {n}y CAGR: windows \d+  min\s+(-?[\d.]+%)  median\s+(-?[\d.]+%)  max\s+(-?[\d.]+%)  share>=15%\s+([\d.]+%)',b)
            d[f'r{n}']=f'{mm.group(1)} / {mm.group(2)} / {mm.group(3)} / {mm.group(4)}' if mm else ''
        d['win_closed']=g(r'win rate closed\s+([\d.]+%)'); d['win_all']=g(r'all-with-open-MTM\s+([\d.]+%)')
        d['avgwin']=g(r'avg win\s+(-?[\d.]+%)'); d['avgloss']=g(r'avg loss\s+(-?[\d.]+%)')
        d['pf']=g(r'profit factor closed (\S+) all (\S+)'); d['pf_all']=re.search(r'profit factor closed (\S+) all (\S+)',b).group(2) if re.search(r'profit factor closed (\S+) all (\S+)',b) else ''
        d['exp']=g(r'expectancy per closed lot\s+(-?[\d.]+%)')
        d['hold']=g(r'avg\(closed lots\) (\d+)'); d['hold_all']=g(r'avg\(all lots incl. open\) (\d+)'); d['maxhold']=g(r'max closed (\d+)')
        d['longlose']=g(r'longest-held losing lot (\d+) days'); d['openlosers']=g(r'open underwater lots (\d+)'); d['worstlot']=g(r'worst single-lot drawdown (-?[\d.]+%)')
        d['openun']=g(r'open unrealised\s+(-?[\d.]+%)')
        d['closed']=g(r'lots: closed (\d+)'); d['open']=g(r'lots: closed \d+ open (\d+)')
        d['exposure']=g(r'days with a position\s+([\d.]+%)'); d['avginv']=g(r'avg invested\s+([\d.]+%)'); d['maxinv']=g(r'max invested\s+([\d.]+%)'); d['stocks']=g(r'avg stocks held ([\d.]+)')
        d['turn']=g(r'turnover ([\d.]+)x/yr'); d['drag']=g(r'charges drag\s+([\d.]+%)/yr')
        d['beta']=g(r'beta (-?[\d.]+)'); d['alpha']=g(r'alpha\s+(-?[\d.]+%)'); d['ir']=g(r'IR (-?[\d.]+)'); d['up']=g(r'up-capture (\d+)%'); d['down']=g(r'down-capture (\d+)%'); d['te']=g(r'TE\s+([\d.]+%)'); d['corr']=g(r'corr (-?[\d.]+)')
        d['skew']=g(r'skew (-?[\d.]+)'); d['kurt']=g(r'excess kurtosis (-?[\d.]+)'); d['var95']=g(r'VaR95\s+([\d.]+%)'); d['cvar95']=g(r'CVaR95\s+([\d.]+%)'); d['bestday']=g(r'best day\s+(-?[\d.]+%)'); d['worstday']=g(r'worst day\s+(-?[\d.]+%)'); d['avgdd']=g(r'average drawdown\s+(-?[\d.]+%)'); d['losemonths']=g(r'losing months (\d+)')
        d['tax']=g(r'tax paid\+terminal Rs (\d+)'); d['charges']=g(r'charges Rs (\d+)'); d['divs']=g(r'dividends Rs (\d+)')
        out[stage]=d
    return out
rows=[('Final value (Rs, start 10 lakh)','final'),('CAGR','cagr'),('Annualised volatility','vol'),('Sharpe (rf 6%)','sharpe'),('Sortino','sortino'),('Calmar','calmar'),('Ulcer index','ulcer'),('Ulcer performance index','upi'),
('Max drawdown','maxdd'),('  peak date','dd_peak'),('  trough date','dd_trough'),('  recovered','dd_rec'),('Longest underwater (days)','uw'),('Best calendar year','best'),('Worst calendar year','worst'),('Positive months','posm'),
('Average drawdown','avgdd'),('Daily return skew / excess kurtosis','sk2'),('1-day VaR 95% / expected shortfall','var2'),('Best day / worst day','bw'),('Longest run of losing months','losemonths'),('Rolling 1y CAGR min / median / max / share >=15%','r1'),('Rolling 3y CAGR min / median / max / share >=15%','r3'),('Rolling 5y CAGR min / median / max / share >=15%','r5'),
('Closed lots / open lots at end','closed_open'),('Win rate, closed lots','win_closed'),('Win rate, all lots with open at market','win_all'),('Average win / average loss (closed)','winloss'),('Profit factor, closed / all','pf2'),('Expectancy per closed lot','exp'),
('Average holding days, closed / all lots','hold2'),('Longest closed hold (days)','maxhold'),('Longest-held losing lot (days)','longlose'),('Open lots under water','openlosers'),('Worst single-lot drawdown','worstlot'),('Open unrealised P&L as % of final equity','openun'),
('Days with a position','exposure'),('Average / peak invested','inv2'),('Average stocks held','stocks'),('Turnover per year (x equity)','turn'),('Statutory charges drag per year','drag'),
('Beta / correlation to Nifty 50 TRI','beta2'),('Jensen alpha (annual)','alpha'),('Tracking error / information ratio','te2'),('Up / down capture','ud'),('Tax paid + tax if liquidated (Rs)','tax'),('Charges paid (Rs)','charges'),('Dividends received (Rs)','divs')]
def cell(d,k):
    if k=='sk2': return f"{d['skew']} / {d['kurt']}"
    if k=='var2': return f"{d['var95']} / {d['cvar95']}"
    if k=='bw': return f"{d['bestday']} / {d['worstday']}"
    if k=='closed_open': return f"{d['closed']} / {d['open']}"
    if k=='winloss': return f"{d['avgwin']} / {d['avgloss']}"
    if k=='pf2': return f"{d['pf']} / {d['pf_all']}"
    if k=='hold2': return f"{d['hold']} / {d['hold_all']}"
    if k=='inv2': return f"{d['avginv']} / {d['maxinv']}"
    if k=='beta2': return f"{d['beta']} / {d['corr']}"
    if k=='te2': return f"{d['te']} / {d['ir']}"
    if k=='ud': return f"{d['up']}% / {d['down']}%"
    return d.get(k,'')
for v in variants:
    p=parse(f'research/out/final/scorecard_nifty50_{v}.txt')
    stages=[s for s in ['gross','cost','tax-dated','tax-today'] if s in p]
    print(f'#### {v}\n')
    print('| metric | '+' | '.join({'gross':'before costs','cost':'after costs','tax-dated':'after costs + tax (rates by sale date)','tax-today':'after costs + tax (today\'s rates throughout)'}[s] for s in stages)+' |')
    print('|---|'+'---|'*len(stages))
    for name,k in rows:
        print(f'| {name} | '+' | '.join(cell(p[s],k) for s in stages)+' |')
    print()

if __name__ == '__main__' and len(sys.argv) > 1 and False:
    pass
