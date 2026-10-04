#!/bin/zsh
# Momentum study on the Nifty 50, as frozen in research/studies/momentum-nifty50.md.
# Needs: go build -o /tmp/research_final ./cmd/research
#        python3 research/py/fetch_tri.py "NIFTY 50" "NIFTY200 MOMENTUM 30"   (plus the Yahoo prices, see README)
set -e
R=${R:-/tmp/research_final}
V="mom-plain-top10-6m,mom-plain-top20-6m-ma200,mom-plain-top10-1m-ma200,mom-plain-top20-1m,mom-nse-top10-6m-ma200,mom-nse-top20-6m,mom-nse-top10-1m,mom-nse-top20-1m-ma200"
CASH6="mom-plain-top20-6m-ma200-cash6,mom-plain-top10-1m-ma200-cash6,mom-nse-top10-6m-ma200-cash6,mom-nse-top20-1m-ma200-cash6"
W=(-universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag 1)
FUND=(-fund NIFTY200_MOMENTUM_30 -fund-fee 0.002)
O=research/out/momentum-n50
mkdir -p $O/cash6
# Every 5-year window from each month, beside the Nifty 50 TRI and the momentum fund (headline and keep rule).
$R lottery $W -horizon 5 -stages cost,tax-dated,tax-today $FUND -variants "$V" -out $O > $O/lottery_5y.md
# Sensitivity: the four filter variants with idle cash earning 6% a year.
$R lottery $W -horizon 5 -stages cost,tax-dated,tax-today $FUND -variants "$CASH6" -out $O/cash6 > $O/cash6/lottery_5y.md
# Deflated Sharpe charged for this study's 8 trials, and for the 240 of the NiftyShop study plus these 8; walk-forward.
$R wfvariants $W -variants "$V" -trials 8 > $O/wf_trials8.md
$R wfvariants $W -variants "$V" -trials 248 > $O/wf_trials248.md
# One full-period run each: scorecards and month-by-month tables against the index and against the fund.
$R score $W -variants "$V" -stages cost,tax-dated,tax-today -out $O -log "" > $O/score.txt
$R score $W -variants "$V" -stages tax-today -bench NIFTY200_MOMENTUM_30 -bench-fee 0.002 -out $O -log "" > $O/score_vs_fund.txt
# Keep verdicts and the start-year table from the window CSV.
python3 research/py/momentum_tables.py $O/lottery_nifty50_5y.csv $O/cash6/lottery_nifty50_5y.csv > $O/tables.md
