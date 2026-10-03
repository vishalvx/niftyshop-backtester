import glob, os, json, pandas as pd, numpy as np
Y='.research-data/yahoo/'
al=json.load(open('research/symbol_aliases.json'))
al={k:v for k,v in al.items() if not k.startswith('_')}
def path(sym):
    t=al.get(sym, sym+'.NS')
    return Y+t+'.csv'
rows=[]
for f in sorted(glob.glob(Y+'*.NS.csv')):
    t=os.path.basename(f)[:-4]
    df=pd.read_csv(f,parse_dates=['Date'])
    r=df.Close.pct_change()
    big=df[(r.abs()>0.25)]
    gaps=df.Date.diff().dt.days
    sp=pd.read_csv(f.replace('.csv','_split.csv'))
    dv=pd.read_csv(f.replace('.csv','_div.csv'))
    rows.append(dict(ticker=t,first=df.Date.min().date(),last=df.Date.max().date(),rows=len(df),
        zero_close=(df.Close<=0).sum(),zero_vol_days=(df.Volume==0).sum(),max_gap_days=int(gaps.max()) if len(gaps.dropna()) else 0,
        jumps_gt25=len(big),splits=len(sp),divs=len(dv),adj_ratio_first=float(df.AdjClose.iloc[0]/df.Close.iloc[0]) if df.AdjClose.iloc[0]==df.AdjClose.iloc[0] else None))
a=pd.DataFrame(rows)
a.to_csv('research/out/data_audit_symbols.csv',index=False)
print(len(a),'tickers with data')
print(a.describe(include='all').T[['count','min','max']].to_string())
print('\nsymbols with >25% daily jumps:'); print(a[a.jumps_gt25>0][['ticker','first','last','jumps_gt25','splits']].to_string(index=False))
print('\nsymbols starting after 2008-01-31:'); print(a[pd.to_datetime(a['first'])>pd.Timestamp('2008-01-31')][['ticker','first']].to_string(index=False))
print('\nmax gap>10d:'); print(a[a.max_gap_days>10][['ticker','max_gap_days']].to_string(index=False))
