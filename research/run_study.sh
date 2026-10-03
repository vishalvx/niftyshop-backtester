#!/bin/bash
# Regenerates every table in the long-run study from scratch. Needs network for the first step only.
# Usage: caffeinate -i ./research/run_study.sh   (about 10 minutes on an M-series Mac)
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=research/out/final
mkdir -p "$OUT" .research-data/yahoo .research-data/indices
filter() { grep -v 'zoxide\|^$\|github.com/ajeet\|Please ensure\|If the issue\|Disable this\|config init' || true; }

echo "1/9 data (Yahoo prices + niftyindices TRI/price series)"
python3 research/py/fetch_yahoo.py $(sed 's/^/ /' research/symbols_literal.txt | tr '\n' ' ') TMPV.NS LTF.NS UNITDSPR.NS INDUSTOWER.NS SAMMAANCAP.NS BAJAJ-AUTO.NS '^NSEI' '^NSEMDCP50' || true
python3 research/py/fetch_tri.py "NIFTY 50" "NIFTY MIDCAP 50" "NIFTY SMALLCAP 50" "NIFTY500 MOMENTUM 50" "NIFTY 500" || true
go build -o /tmp/research ./cmd/research

echo "2/9 data audit"
python3 research/py/data_audit.py > $OUT/data_audit.txt 2>&1 || true
python3 research/py/index_gap.py > $OUT/index_price_vs_tri.txt 2>&1 || true

echo "3/9 reproduce the published numbers"
/tmp/research repro 2>&1 | filter > $OUT/repro.txt

echo "4/9 engine-behaviour audit (what each deviation is worth)"
for w in "2008-01-01:2025-08-31" "2021-01-01:2025-12-31"; do
  s=${w%%:*}; e=${w##*:}
  /tmp/research audit -universe nifty50 -start "$s" -end "$e" -log /dev/null 2>&1 | filter > "$OUT/audit_nifty50_${s}_${e}.txt"
done

echo "5/9 headline runs and scorecards"
for u in "nifty50 2008-01-01 2025-08-31" "niftymidcap50 2019-02-01 2026-06-30" "niftysmallcap50 2019-04-01 2026-06-30" "nifty500momentum50 2024-06-04 2026-06-30"; do
  set -- $u
  /tmp/research run -universe "$1" -start "$2" -end "$3" -variants legacy,spec,spec-cam-s1-5,cam-s1-5,fib-s1-15,app-approx,app-vpivot-nifty50 -log /dev/null 2>&1 | filter > "$OUT/run_$1.txt"
done
for v in legacy spec cam-s1-5 fib-s1-15 app-approx app-vpivot-nifty50; do
  /tmp/research score -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants $v -export $OUT/nifty50_2008_2025 -log /dev/null 2>&1 | filter > "$OUT/scorecard_nifty50_${v}.txt"
done

echo "6/9 start-date lotteries"
for h in 3 5 10; do
  /tmp/research lottery -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon $h -variants legacy,spec,cam-s1-5,fib-s1-15,app-approx,app-vpivot-nifty50 -out $OUT 2>&1 | filter > "$OUT/lottery_nifty50_${h}y.md"
done
/tmp/research lottery -universe niftymidcap50 -start 2019-02-01 -end 2026-06-30 -horizon 3 -variants legacy,spec,cam-s1-5,fib-s1-15 -out $OUT 2>&1 | filter > "$OUT/lottery_niftymidcap50_3y.md"
mkdir -p $OUT/harsh
/tmp/research lottery -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon 5 -scenario harsh -variants legacy,spec,cam-s1-5 -out $OUT/harsh 2>&1 | filter > "$OUT/lottery_nifty50_5y_harsh.md"
/tmp/research jitter -universe nifty50 -start 2008-01-01 -end 2025-08-31 -variants legacy,spec,cam-s1-5,fib-s1-15 -stages cost 2>&1 | filter > "$OUT/jitter_nifty50_cost.md"

echo "7/9 pivot-grid neighbourhood"
/tmp/research grid -universe nifty50 -start 2008-01-01 -end 2025-08-31 -horizon 5 -stages cost 2>&1 | filter > "$OUT/grid_nifty50_5y.md"

echo "8/9 honest tuning: tune/test split and walk-forward"
for r in "" "-robust"; do
  /tmp/research tune -universe nifty50 -start 2007-06-01 -end 2025-08-31 $r -out $OUT -log $OUT/experiment-log.jsonl 2>&1 | filter > "$OUT/tune_nifty50${r}.md"
  /tmp/research walkforward -universe nifty50 -start 2008-01-01 -end 2025-08-31 -stages cost $r -log /dev/null 2>&1 | filter > "$OUT/walkforward_nifty50${r}.md"
done

echo "9/9 stale-capital and calendar-year tables"
python3 research/py/stale_capital.py > $OUT/stale_capital.txt 2>&1 || true
echo done
