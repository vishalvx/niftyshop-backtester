"""Side-by-side metrics of several strategies at one accounting stage. Usage: scorecard_compare.py <stage> <variant>..."""
import sys, importlib.util
spec=importlib.util.spec_from_file_location('st','research/py/scorecard_tables.py')
src=open('research/py/scorecard_tables.py').read().split("for v in variants:")[0]
ns={}
sys.argv=[sys.argv[0]]+sys.argv[1:]
stage=sys.argv[1]; variants=sys.argv[2:]
sys.argv=[sys.argv[0]]+variants
exec(src,ns)
parse,rows,cell=ns['parse'],ns['rows'],ns['cell']
labels={'legacy':'Standard (engine rules)','spec':'Standard (documented rules)','cam-s1-5':'V-Pivot Cam S1/5 (engine rules)','fib-s1-15':'V-Pivot Fib S1/15 (engine rules)','app-approx':'app rules','app-vpivot-nifty50':'the app Nifty 50 V-Pivot rules'}
data={v:parse(f'research/out/final/scorecard_nifty50_{v}.txt')[stage] for v in variants}
keep=['Final value (Rs, start 10 lakh)','CAGR','Annualised volatility','Sharpe (rf 6%)','Sortino','Calmar','Ulcer index','Max drawdown','Longest underwater (days)','Best calendar year','Worst calendar year','Positive months','Rolling 5y CAGR min / median / max / share >=15%','Closed lots / open lots at end','Win rate, closed lots','Win rate, all lots with open at market','Profit factor, closed / all','Average holding days, closed / all lots','Longest-held losing lot (days)','Worst single-lot drawdown','Open unrealised P&L as % of final equity','Average / peak invested','Average stocks held','Turnover per year (x equity)','Beta / correlation to Nifty 50 TRI','Jensen alpha (annual)','Tracking error / information ratio','Up / down capture','Tax paid + tax if liquidated (Rs)']
print('| metric | '+' | '.join(labels[v] for v in variants)+' |'); print('|---|'+'---|'*len(variants))
for name,k in rows:
    if name in keep: print(f'| {name} | '+' | '.join(cell(data[v],k) for v in variants)+' |')
