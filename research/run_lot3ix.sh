#!/bin/zsh
# Maintainer's rules, 2026-10-03: a stock that leaves the index is sold (suffix -ix). Point-in-time Nifty 50 only: the Midcap list
# in the dataset never removes a member, so the rule changes nothing there.
set -e
R=/tmp/research
O=research/out/final/lot3ix
mkdir -p $O/export
S="spec,spec-cap3,spec-cap3-ix,spec-lot3,spec-lot3-ix"
C="spec-cam-s1-5,spec-cap3:camarilla:S1:5,spec-cap3-ix:camarilla:S1:5,spec-lot3:camarilla:S1:5,spec-lot3-ix:camarilla:S1:5"
F="spec-fib-s1-15,spec-cap3:fibonacci:S1:15,spec-cap3-ix:fibonacci:S1:15,spec-lot3:fibonacci:S1:15,spec-lot3-ix:fibonacci:S1:15"
V="$S,$C,$F"
$R score -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$V" -export $O/export/u1 > $O/score_u1.txt
for h in 3 5 10; do $R lottery -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon $h -variants "$V" > $O/lottery_u1_${h}y.md; done
$R wfvariants -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants "$V" -trials 228 > $O/wf_u1.md
echo done > $O/DONE
