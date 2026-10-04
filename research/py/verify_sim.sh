#!/bin/zsh
# Independent check: run the Go simulator and research/py/ref_sim.py on identical inputs and compare every lot.
# Usage (from the worktree root, after `go build -o /tmp/research ./cmd/research`): research/py/verify_sim.sh
R=/tmp/research
V=research/out/final/verify
mkdir -p $V/tmp
check() { # universe start end goPreset pyMode [membership lag, default 0]
  u=$1; s=$2; e=$3; gp=$4; pm=$5; lag=${6:-0}
  sfx=""; [ "$lag" != 0 ] && sfx="_lag$lag"
  panel=$V/panel_${u}_${s}_${e}${sfx}.csv
  [ -f $panel ] || $R exportpanel -universe $u -start $s -end $e -lag $lag -out $V
  rm -rf $V/tmp/go; $R score -universe $u -start $s -end $e -lag $lag -variants "$gp" -dividends=false -stages gross -export $V/tmp/go > /dev/null || return 1
  gofile=$(ls $V/tmp/go/*_lots.csv | head -1)
  python3 - "$gofile" > $V/tmp/go_lots.csv <<'PY'
import csv, sys
w = csv.writer(sys.stdout); w.writerow(['symbol','buy_date','sell_date','qty','buy_price'])
rows = sorted(csv.DictReader(open(sys.argv[1])), key=lambda r: (r['buy_date'], r['symbol'], int(r['qty'])))
for r in rows: w.writerow([r['symbol'], r['buy_date'], r['sell_date'], r['qty'], f"{float(r['buy_price']):.4f}"])
PY
  python3 research/py/ref_sim.py $panel "$pm" 1000000 > $V/tmp/py_lots.csv
  n_go=$(($(wc -l < $V/tmp/go_lots.csv) - 1)); n_py=$(($(wc -l < $V/tmp/py_lots.csv) - 1))
  if diff -q $V/tmp/go_lots.csv $V/tmp/py_lots.csv > /dev/null; then echo "MATCH   $u $s..$e lag$lag  go=$gp  ref=$pm  lots: go=$n_go ref=$n_py"; else echo "DIFFER  $u $s..$e lag$lag  go=$gp  ref=$pm  lots: go=$n_go ref=$n_py"; diff $V/tmp/go_lots.csv $V/tmp/py_lots.csv | head -6; fi
}
check nifty50 2018-01-01 2025-08-31 spec spec
check nifty50 2018-01-01 2025-08-31 app-exact app
check nifty50 2018-01-01 2025-08-31 spec-cap3 cap3
check nifty50 2018-01-01 2025-08-31 spec-lot3 lot3
# maintainer's rule (2026-10-03): a stock that leaves the index is sold.
check nifty50 2008-01-01 2025-08-31 spec-cap3-ix cap3-ix
check nifty50 2008-01-01 2025-08-31 spec-lot3-ix lot3-ix
# the maintainer's confirmed rule set (2026-10-03): no stock cap, 3 open lots, one purchase a day, index exit, capital refreshed after every sale
check nifty50 2008-01-01 2025-08-31 nsx nsx
check nifty50 2008-01-01 2025-08-31 nsx-lot nsx-lot
# headline setting of the complete study: first tradable month, membership lag 1 (previous month's list)
for pm in nsx nsx-lot; do
  check nifty50 2008-02-01 2025-08-31 $pm $pm 1
done
