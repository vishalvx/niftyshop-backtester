#!/bin/zsh
# Complete backtest of the rule set the maintainer confirmed (round 6) on the Nifty 50, from its first tradable month.
#   lag 1 (headline): trades in month M use the membership snapshot of month M-1, so nothing is known before it could be.
#   lag 0 (engine convention): a month-end snapshot is applied to the whole month it is dated in (up to a month of look-ahead).
# Window: 2008-02-01..2025-08-31 (first snapshot Jan 2008, last Aug 2025).
# It runs on the data the study used (universe nifty50yahoo: month snapshots and Yahoo prices); the nifty50 universe now
# uses the official NSE lists and prices (research/py/nse_build.py), so its numbers differ.
# The published tables in research/out/final/complete also cover the removed Midcap list and V-Pivot variants; the script
# that produced them is at the findings-2026-10 tag.
set -e
R=/tmp/research_final
V="nsx,nsx-lot"
BASE=research/out/final/complete
mkdir -p $BASE
$R refscore -variants NIFTY_50,NIFTY_500 -start 2008-02-01 -end 2025-08-31 -bench NIFTY_50 > $BASE/ref_2008-02_2025-08.txt
for LAG in 1 0; do
  O=$BASE/lag$LAG
  mkdir -p $O/export
  $R score -universe nifty50yahoo -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -export $O/export/u1 > $O/score_u1.txt
  for h in 3 5 10; do $R lottery -universe nifty50yahoo -start 2008-02-01 -end 2025-08-31 -lag $LAG -horizon $h -variants "$V" > $O/lottery_u1_${h}y.md; done
  $R lottery -universe nifty50yahoo -start 2008-02-01 -end 2025-08-31 -lag $LAG -horizon 5 -scenario harsh -variants "$V" > $O/lottery_u1_5y_harsh.md
  $R jitter -universe nifty50yahoo -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -stages cost > $O/jitter_u1_cost.md
  $R wfvariants -universe nifty50yahoo -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -trials 240 > $O/wf_u1.md
  echo done > $O/DONE
done
echo done > $BASE/DONE
