package sim

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Preset returns a named base rule set: legacy (what the engine does), spec (what strategy.md says), app-approx / app-exact (rule sets of a live screener app, see below), rotation-n50.
func Preset(name string) (Rules, error) {
	switch name {
	case "", "legacy":
		return LegacyRules(), nil
	case "spec":
		return SpecRules(), nil
	case "app-approx":
		// A live screener app's standard rules, approximated with one whole-position exit:
		// 6% target on weighted average cost, add when 3% below the MOST RECENT lot, 3 lots per stock, 1 buy a day, capital/10.
		r := LegacyRules()
		r.Name = "app-rules"
		r.ProfitTarget = 0.06
		r.ExitBasis, r.AvgBasis = "avgcost", "lastlot"
		r.MaxLots, r.MaxStocks, r.MaxBuysPerDay = 3, 10, 1
		return r, nil
	case "app-exact":
		// The same app rules with exact exits: every lot keeps the target price stored when it was bought,
		// average down against the latest lot, 3 lots, 1 buy a day, capital/10.
		r := LegacyRules()
		r.Name = name
		r.ProfitTarget = 0.06
		r.ExitBasis, r.AvgBasis = "apptarget", "lastlot"
		r.MaxLots, r.MaxStocks, r.MaxBuysPerDay = 3, 10, 1
		return r, nil
	case "rotation-n50":
		// Frozen before running: mean of 6- and 12-month return, top 10,
		// first trading day of January and July. The pullback fields are unused but kept valid.
		r := LegacyRules()
		r.Name = name
		r.Rotation = &RotationRule{LookbackMonths: []int{6, 12}, TopN: 10, RebalanceMonths: []int{1, 7}}
		return r, nil
	}
	if strings.HasPrefix(name, "mom-") {
		return momentumPreset(name)
	}
	return Rules{}, fmt.Errorf("sim: unknown preset %q (want legacy|spec|app-exact|rotation-n50|app-approx|mom-...)", name)
}

// momentumPreset parses the momentum study's names (research/studies/momentum-nifty50.md):
//
//	mom-<plain|nse>-top<N>-<6m|1m>[-ma<days>][-cash<pct>]
//
// plain or nse is the score (RotationRule.Score), top<N> the names held, 6m swaps on the first trading day of January
// and July and 1m on that of every month, ma<days> the market filter (RotationRule.MarketMADays) and cash<pct> a yearly
// yield in percent on idle cash. The lookbacks are always 6 and 12 months; mom-plain-top10-6m is rotation-n50.
func momentumPreset(name string) (Rules, error) {
	bad := func(why string) (Rules, error) {
		return Rules{}, fmt.Errorf("sim: bad momentum preset %q: %s (want mom-<plain|nse>-top<N>-<6m|1m>[-ma<days>][-cash<pct>])", name, why)
	}
	parts := strings.Split(name, "-")
	if len(parts) < 4 || len(parts) > 6 {
		return bad("wrong number of parts")
	}
	rot := &RotationRule{LookbackMonths: []int{6, 12}, Score: parts[1]}
	if rot.Score != "plain" && rot.Score != "nse" {
		return bad("score must be plain or nse")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(parts[2], "top"))
	if !strings.HasPrefix(parts[2], "top") || err != nil || n <= 0 {
		return bad("names held must be top<N>")
	}
	rot.TopN = n
	switch parts[3] {
	case "6m":
		rot.RebalanceMonths = []int{1, 7}
	case "1m":
		rot.RebalanceMonths = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	default:
		return bad("swaps must be 6m or 1m")
	}
	r := LegacyRules()
	r.Name = name
	r.Rotation = rot
	rest := parts[4:]
	if len(rest) > 0 && strings.HasPrefix(rest[0], "ma") {
		d, err := strconv.Atoi(strings.TrimPrefix(rest[0], "ma"))
		if err != nil || d <= 1 {
			return bad("market filter must be ma<days>")
		}
		rot.MarketMADays = d
		rest = rest[1:]
	}
	if len(rest) > 0 && strings.HasPrefix(rest[0], "cash") {
		y, err := strconv.ParseFloat(strings.TrimPrefix(rest[0], "cash"), 64)
		if err != nil || y < 0 {
			return bad("cash yield must be cash<pct>")
		}
		r.CashYield = y / 100
		rest = rest[1:]
	}
	if len(rest) > 0 {
		return bad("unexpected " + strings.Join(rest, "-"))
	}
	return r, nil
}

// RulesFile is the on-disk form of one rule set: a preset plus overrides, e.g.
//
//	{"name": "tight-stop", "base": "spec", "rules": {"stop_loss": 0.15, "profit_target": 0.08}}
type RulesFile struct {
	Name  string          `json:"name"`
	Base  string          `json:"base"`
	Rules json.RawMessage `json:"rules"`
}

// Overlay applies a JSON object of overrides on top of r. Unknown keys are an error, so typos cannot silently do nothing.
func Overlay(r Rules, overrides json.RawMessage) (Rules, error) {
	if len(overrides) == 0 {
		return r, nil
	}
	dec := json.NewDecoder(bytesReader(overrides))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("sim: bad rules overrides: %w", err)
	}
	return r, Validate(r)
}

// LoadRulesFile reads a RulesFile and returns the resulting rules.
func LoadRulesFile(path string) (Rules, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Rules{}, err
	}
	var f RulesFile
	if err := json.Unmarshal(b, &f); err != nil {
		return Rules{}, fmt.Errorf("sim: %s: %w", path, err)
	}
	r, err := Preset(f.Base)
	if err != nil {
		return r, err
	}
	if f.Name != "" {
		r.Name = f.Name
	}
	return Overlay(r, f.Rules)
}

// Validate rejects rule sets that cannot run or would silently misbehave.
func Validate(r Rules) error {
	switch {
	case r.MAWindow <= 0:
		return fmt.Errorf("sim: ma_window must be > 0")
	case r.ProfitTarget <= 0 || r.ProfitTarget > 5:
		return fmt.Errorf("sim: profit_target %v out of range", r.ProfitTarget)
	case r.AvgTrigger <= 0 || r.AvgTrigger >= 1:
		return fmt.Errorf("sim: avg_trigger %v out of range", r.AvgTrigger)
	case r.CapitalDivider <= 0:
		return fmt.Errorf("sim: capital_divider must be > 0")
	case r.MaxFreshPerDay <= 0:
		return fmt.Errorf("sim: max_fresh_per_day must be > 0")
	case r.StopLoss < 0 || r.StopLoss >= 1 || r.TrailPct < 0 || r.TrailPct >= 1:
		return fmt.Errorf("sim: stop_loss / trail_pct must be in [0,1)")
	}
	switch r.ExitBasis {
	case "", "lot", "avgcost", "apptarget":
	default:
		return fmt.Errorf("sim: exit_basis %q", r.ExitBasis)
	}
	switch r.AvgBasis {
	case "", "anylot", "lastlot", "avgcost":
	default:
		return fmt.Errorf("sim: avg_basis %q", r.AvgBasis)
	}
	switch r.Revision {
	case "monthly", "quarterly", "yearly", "sale":
	default:
		return fmt.Errorf("sim: revision %q", r.Revision)
	}
	if r.Pivot != nil && r.Pivot.Pool <= 0 {
		return fmt.Errorf("sim: pivot.pool must be > 0")
	}
	return nil
}

// Sweep is a grid of overrides over a base: every combination becomes one trial.
//
//	{"id": "exit-grid", "base": "legacy", "fixed": {"dividends": true},
//	 "grid": {"stop_loss": [0, 0.15, 0.25], "profit_target": [0.05, 0.10]}}
type Sweep struct {
	ID    string                       `json:"id"`
	Base  string                       `json:"base"`
	Fixed map[string]any               `json:"fixed"`
	Grid  map[string][]json.RawMessage `json:"grid"`
}

// Expand returns one named rule set per grid point (cartesian product, keys in sorted order).
func (s Sweep) Expand() ([]Rules, error) {
	base, err := Preset(s.Base)
	if err != nil {
		return nil, err
	}
	var keys []string
	for k := range s.Grid {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	combos := []map[string]json.RawMessage{{}}
	for _, k := range keys {
		var next []map[string]json.RawMessage
		for _, c := range combos {
			for _, v := range s.Grid[k] {
				n := map[string]json.RawMessage{}
				for ck, cv := range c {
					n[ck] = cv
				}
				n[k] = v
				next = append(next, n)
			}
		}
		combos = next
	}
	var out []Rules
	for _, c := range combos {
		m := map[string]any{}
		for k, v := range s.Fixed {
			m[k] = v
		}
		name := s.ID
		for _, k := range keys {
			var x any
			json.Unmarshal(c[k], &x)
			m[k] = x
			name += fmt.Sprintf("/%s=%v", k, x)
		}
		raw, _ := json.Marshal(m)
		r, err := Overlay(base, raw)
		if err != nil {
			return nil, err
		}
		r.Name = name
		out = append(out, r)
	}
	return out, nil
}
