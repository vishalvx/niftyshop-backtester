"""Calendar-year and stress-window returns of the exported equity curves against the Nifty 50 TRI."""
import json, pandas as pd, glob, os, sys
D='research/out/final/nifty50_2008_2025/'
stage=sys.argv[1] if len(sys.argv)>1 else 'cost'
tri=pd.Series({pd.to_datetime(r['Date'],format='%d %b %Y'):float(r['TotalReturnsIndex']) for r in json.load(open('.research-data/indices/NIFTY_50_TRI.json'))}).sort_index()
names={'standard-legacy':'Standard (engine rules)','standard-spec':'Standard (documented rules)','vpivot-camarilla-S1-5':'V-Pivot Cam S1/5 (engine rules)','app-rules':'app rules','app-vpivot-nifty50':'app V-Pivot rules'}
eq={}
for k in names:
    f=f'{D}{k}_{stage}_equity.csv'
    if os.path.exists(f):
        eq[k]=pd.read_csv(f,parse_dates=['date']).set_index('date').equity
first=min(s.index[0] for s in eq.values()); last=min(s.index[-1] for s in eq.values())
tri=tri[(tri.index>=first-pd.Timedelta(days=5))&(tri.index<=last)]
def ret(s,a,b,start_val=None):
    a=pd.Timestamp(a); b=pd.Timestamp(b)
    v0=s.asof(a) if a>s.index[0] else (start_val if start_val else s.iloc[0])
    return s.asof(b)/v0-1
rows=[]
years=range(first.year,last.year+1)
for y in years:
    a=f'{y-1}-12-31' if y>first.year else first
    b=f'{y}-12-31' if y<last.year else last
    row={'year':str(y)+('*' if y==last.year else '')}
    row['Nifty 50 TRI']=ret(tri,a,b) if y>first.year else tri.asof(pd.Timestamp(b))/tri.asof(first)-1
    for k,n in names.items():
        if k in eq:
            row[n]=ret(eq[k],a,b,start_val=1_000_000) if y>first.year else eq[k].asof(pd.Timestamp(b))/1_000_000-1
    rows.append(row)
df=pd.DataFrame(rows).set_index('year')
print(f'### Calendar-year returns, stage {stage} (dividends credited, start Rs 10 lakh each run, * = partial year)\n')
print('| year | '+' | '.join(df.columns)+' |'); print('|---|'+'---|'*len(df.columns))
for y,r in df.iterrows(): print(f'| {y} | '+' | '.join(f'{v*100:6.1f}%' for v in r)+' |')
stress=[('2008 crash (1 Jan 2008 - 9 Mar 2009)','2008-01-01','2009-03-09'),('2011 (calendar year)','2010-12-31','2011-12-31'),('2015-16 (1 Mar 2015 - 29 Feb 2016)','2015-03-01','2016-02-29'),('IL&FS to COVID low (3 Sep 2018 - 23 Mar 2020)','2018-09-03','2020-03-23'),('COVID crash (20 Jan - 23 Mar 2020)','2020-01-20','2020-03-23'),('2022 fall (19 Oct 2021 - 17 Jun 2022)','2021-10-19','2022-06-17'),('Sep 2024 peak to Mar 2025 low','2024-09-27','2025-03-28')]
print(f'\n### Stress windows, stage {stage}\n')
print('| window | Nifty 50 TRI | '+' | '.join(names[k] for k in eq)+' |'); print('|---|---|'+'---|'*len(eq))
for n,a,b in stress:
    print(f'| {n} | {ret(tri,a,b)*100:6.1f}% | '+' | '.join(f'{ret(eq[k],a,b,start_val=1_000_000)*100:6.1f}%' for k in eq)+' |')
