import json, pandas as pd
def load(p,dk,vk):
    rows=json.load(open(p))
    return pd.Series({pd.to_datetime(r[dk],format='%d %b %Y'):float(r[vk].replace(',','')) for r in rows if r[vk] not in ('-','')}).sort_index()
res=[]
for name,fn,s,e in [('Nifty 50','NIFTY_50',"2021-01-01","2025-12-31"),('Nifty Midcap 50','NIFTY_MIDCAP_50',"2021-01-01","2025-12-31"),('Nifty Smallcap 50','NIFTY_SMALLCAP_50',"2021-01-01","2025-12-31"),('Nifty500 Momentum 50','NIFTY500_MOMENTUM_50',"2024-06-04","2025-12-31"),('Nifty 50 (8y)','NIFTY_50',"2018-01-01","2025-12-31"),('Nifty 50 (long)','NIFTY_50',"2008-01-01","2025-08-31"),('Nifty Midcap 50 (long)','NIFTY_MIDCAP_50',"2008-01-01","2025-08-31")]:
    tri=load(f'.research-data/indices/{fn}_TRI.json','Date','TotalReturnsIndex'); pri=load(f'.research-data/indices/{fn}_PRI.json','HistoricalDate','CLOSE')
    def cagr(x):
        a=x.asof(pd.Timestamp(s)); b=x.asof(pd.Timestamp(e)); yrs=(pd.Timestamp(e)-pd.Timestamp(s)).days/365.25
        return (b/a)**(1/yrs)-1, a, b
    pc,pa,pb=cagr(pri); tc,ta,tb=cagr(tri)
    res.append((name,s,e,round(pc*100,2),round(tc*100,2),round((tc-pc)*100,2),pa,pb))
print(pd.DataFrame(res,columns=['index','from','to','price CAGR %','TRI CAGR %','TRI minus price pp','price start','price end']).to_string(index=False))
pri=load('.research-data/indices/NIFTY_SMALLCAP_50_PRI.json','HistoricalDate','CLOSE')
print('smallcap price 2021-01-01 asof',pri.asof(pd.Timestamp('2021-01-01')),'2025-12-31',pri.asof(pd.Timestamp('2025-12-31')),'(run_all.sh hardcodes 3736.65 and 8753.85)')
pri=load('.research-data/indices/NIFTY500_MOMENTUM_50_PRI.json','HistoricalDate','CLOSE')
print('momentum price 2024-06-04',pri.asof(pd.Timestamp('2024-06-04')),'2025-12-31',pri.asof(pd.Timestamp('2025-12-31')),'(run_all.sh hardcodes 38623.50 and 49602.80)')
