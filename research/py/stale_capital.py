import pandas as pd, sys, glob
rows=[]
for tag in ['standard-legacy','standard-spec','vpivot-camarilla-S1-5','app-rules','app-vpivot-nifty50']:
    lots=pd.read_csv(f'research/out/final/nifty50_2008_2025/{tag}_cost_lots.csv',parse_dates=['buy_date','sell_date'])
    eq=pd.read_csv(f'research/out/final/nifty50_2008_2025/{tag}_cost_equity.csv',parse_dates=['date']).set_index('date')
    lots['cost']=lots.buy_price*lots.qty
    dates=eq.index
    stale=pd.Series(0.0,index=dates)
    for _,l in lots.iterrows():
        end=l.sell_date if not l.open else dates[-1]
        a=l.buy_date+pd.Timedelta(days=365)
        if a>end: continue
        m=(dates>=a)&(dates<=end)
        stale[m]+=l.cost
    frac=stale/eq.equity
    rows.append((tag,round(frac.mean()*100,1),round(frac.max()*100,1),round((frac>0.5).mean()*100,1),round((frac>0.25).mean()*100,1),round(frac.iloc[-1]*100,1)))
print(pd.DataFrame(rows,columns=['variant','avg % of equity in lots older than 1y','max %','% of days above 50%','% of days above 25%','at end %']).to_string(index=False))
