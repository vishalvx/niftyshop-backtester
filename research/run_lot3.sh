#!/bin/zsh
# Captain's variant (2026-10-02): max 3 lots per stock, and every lot has its own +5% target from its own entry price.
# Compared with the written rules (whole position at +5% over average cost) and with the written rules plus the 3-lot cap.
set -e
R=/tmp/research
O=research/out/final/lot3
mkdir -p $O/export
V="spec,spec-cap3,spec-lot3"
C="spec-cam-s1-5,spec-cap3:camarilla:S1:5,spec-lot3:camarilla:S1:5"
F="spec-fib-s1-15,spec-cap3:fibonacci:S1:15,spec-lot3:fibonacci:S1:15"
U1="$V,$C,$F"
$R score -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$U1" -export $O/export/u1 > $O/score_u1.txt
for h in 3 5 10; do $R lottery -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon $h -variants "$U1" > $O/lottery_u1_${h}y.md; done
$R score -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -variants "$U1" -export $O/export/u2 > $O/score_u2.txt
for h in 3 5; do
  $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -horizon $h -variants "$U1" > $O/lottery_u2_${h}y_vs_midcap50.md
  $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -horizon $h -variants "$U1" -bench NIFTY_50 > $O/lottery_u2_${h}y_vs_nifty50.md
done
$R wfvariants -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$U1" -trials 222 > $O/wf_u1.md
$R wfvariants -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -variants "$U1" -train-years 3 -test-years 1 -trials 222 > $O/wf_u2_vs_midcap50.md
echo done > $O/DONE
