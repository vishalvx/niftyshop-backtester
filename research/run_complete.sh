#!/bin/zsh
# Complete backtest of the rule set the maintainer confirmed (round 6), from the first tradable month of each index.
#   lag 1 (headline): trades in month M use the membership snapshot of month M-1, so nothing is known before it could be.
#   lag 0 (engine convention): a month-end snapshot is applied to the whole month it is dated in (up to a month of look-ahead).
# Windows: Nifty 50 2008-02-01..2025-08-31 (first snapshot Jan 2008, last Aug 2025); Midcap list 2019-02-01..2026-06-30.
set -e
R=/tmp/research_final
V="nsx,nsx-lot,nsx:camarilla:S1:5,nsx-lot:camarilla:S1:5,nsx:fibonacci:S1:15,nsx-lot:fibonacci:S1:15"
BASE=research/out/final/complete
mkdir -p $BASE
ALL=NIFTY_50,NIFTY_MIDCAP_50,NIFTY_500
$R refscore -variants $ALL -start 2008-02-01 -end 2025-08-31 -bench NIFTY_50 > $BASE/ref_2008-02_2025-08.txt
$R refscore -variants $ALL -start 2019-02-01 -end 2026-06-30 -bench NIFTY_50 > $BASE/ref_2019-02_2026-06.txt
for LAG in 1 0; do
  O=$BASE/lag$LAG
  mkdir -p $O/export
  # Nifty 50 (point-in-time members), benchmark Nifty 50 TRI
  $R score -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -export $O/export/u1 > $O/score_u1.txt
  for h in 3 5 10; do $R lottery -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag $LAG -horizon $h -variants "$V" > $O/lottery_u1_${h}y.md; done
  $R lottery -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag $LAG -horizon 5 -scenario harsh -variants "$V" > $O/lottery_u1_5y_harsh.md
  $R jitter -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -stages cost > $O/jitter_u1_cost.md
  $R wfvariants -universe nifty50 -start 2008-02-01 -end 2025-08-31 -lag $LAG -variants "$V" -trials 240 > $O/wf_u1.md
  # Midcap list, benchmark Midcap 50 TRI (own) and Nifty 50 TRI
  $R score -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -variants "$V" -export $O/export/u2 > $O/score_u2_vs_midcap50.txt
  $R score -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -bench NIFTY_50 -variants "$V" > $O/score_u2_vs_nifty50.txt
  for h in 3 5; do
    $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -horizon $h -variants "$V" > $O/lottery_u2_${h}y_vs_midcap50.md
    $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -horizon $h -bench NIFTY_50 -variants "$V" > $O/lottery_u2_${h}y_vs_nifty50.md
  done
  $R lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -horizon 5 -scenario harsh -variants "$V" > $O/lottery_u2_5y_harsh.md
  $R jitter -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -variants "$V" -stages cost > $O/jitter_u2_cost.md
  $R wfvariants -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -lag $LAG -variants "$V" -train-years 3 -test-years 1 -trials 240 > $O/wf_u2_vs_midcap50.md
  echo done > $O/DONE
done
echo done > $BASE/DONE
