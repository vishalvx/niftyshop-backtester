#!/bin/zsh
# Re-runs the benchmarks of the first study (old study steps 2-9) into research/out/rerun. The confirmed-rules study is research/run_complete.sh.
cd "$(dirname "$0")/.."
export RERUN_OUT=research/out/rerun
rm -rf $RERUN_OUT; mkdir -p $RERUN_OUT
go build -o /tmp/research ./cmd/research || exit 1
bash research/rerun_study.sh > $RERUN_OUT/rerun_study.log 2>&1
echo finished > $RERUN_OUT/FINISHED
