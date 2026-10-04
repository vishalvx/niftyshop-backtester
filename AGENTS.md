# Project agent memory

This file is the project's committed home for project-intrinsic agent knowledge: build, test, release, architecture, and sharp-edge notes that should travel with the code.

- No price or index data is in git. Download it into `.research-data/` as [README.md](README.md#download-data) describes before any `cmd/research` run.
- `-universe nifty50` and `niftymidcap150` use the official NSE member lists and prices (`research/py/nse_fetch.py`, then `research/py/nse_build.py prices`). A run stops on a wrong member count or an unpriced member, and `-lag` must be 0. `nifty50yahoo` is the old Yahoo setup, kept so `research/run_complete.sh` reproduces the published findings.
- Never edit `internal/data/*_members.csv` or `research/nse/*.csv` by hand. Record a hand-checked decision, with its reason, in `research/nse/*_overrides.json` and rebuild with `research/py/nse_build.py`; [research/nse/README.md](research/nse/README.md) explains each check.
- Checks: `go test ./...`, `python3 -m unittest research/py/test_nse_build.py`, and `research/py/verify_sim.sh` (the Go simulator against the independent Python reference, trade for trade).

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
