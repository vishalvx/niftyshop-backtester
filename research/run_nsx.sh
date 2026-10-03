#!/bin/zsh
# The rule set the maintainer confirmed on 2026-10-03 (nsx = whole-position exit; nsx-lot = every lot has its own +5% exit):
# no cap on stocks held, 3 open lots per stock, one purchase a day, a stock leaving the index is sold, capital refreshed after every sale.
set -e
R=/tmp/research
O=research/out/final/nsx
mkdir -p $O/export
V="nsx,nsx-lot,nsx:camarilla:S1:5,nsx-lot:camarilla:S1:5,nsx:fibonacci:S1:15,nsx-lot:fibonacci:S1:15"
$R score -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$V" -export $O/export/u1 > $O/score_u1.txt
for h in 3 5 10; do $R lottery -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon $h -variants "$V" > $O/lottery_u1_${h}y.md; done
$R score -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -variants "$V" -export $O/export/u2 > $O/score_u2.txt
for h in 3 5; do
  $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -horizon $h -variants "$V" > $O/lottery_u2_${h}y_vs_midcap50.md
  $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -horizon $h -variants "$V" -bench NIFTY_50 > $O/lottery_u2_${h}y_vs_nifty50.md
done
$R wfvariants -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$V" -trials 240 > $O/wf_u1.md
$R wfvariants -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -variants "$V" -train-years 3 -test-years 1 -trials 240 > $O/wf_u2_vs_midcap50.md
echo done > $O/DONE
