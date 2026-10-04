package sim

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOverlayRejectsUnknownKeysAndBadValues(t *testing.T) {
	if _, err := Overlay(LegacyRules(), json.RawMessage(`{"profit_targt": 0.1}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("typo in a rule name must be an error, got %v", err)
	}
	if _, err := Overlay(LegacyRules(), json.RawMessage(`{"avg_trigger": 1.5}`)); err == nil {
		t.Fatal("avg_trigger 1.5 must be rejected")
	}
	r, err := Overlay(LegacyRules(), json.RawMessage(`{"profit_target": 0.08, "pivot": {"system":"camarilla","level":"S1","pool":5}}`))
	if err != nil || r.ProfitTarget != 0.08 || r.Pivot == nil || r.Pivot.Pool != 5 || r.AvgTrigger != 0.03 {
		t.Fatalf("overlay should change only the named fields: %+v err=%v", r, err)
	}
}

func TestSweepExpandsCartesianProduct(t *testing.T) {
	s := Sweep{ID: "g", Base: "legacy", Fixed: map[string]any{"dividends": true},
		Grid: map[string][]json.RawMessage{"stop_loss": {json.RawMessage(`0`), json.RawMessage(`0.15`)}, "profit_target": {json.RawMessage(`0.05`), json.RawMessage(`0.08`), json.RawMessage(`0.12`)}}}
	rs, err := s.Expand()
	if err != nil || len(rs) != 6 {
		t.Fatalf("want 6 combinations, got %d err=%v", len(rs), err)
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if !r.Dividends {
			t.Fatal("fixed override lost")
		}
		seen[r.Name] = true
	}
	if len(seen) != 6 {
		t.Fatalf("names must be unique: %v", seen)
	}
}

func TestLoadRulesFile(t *testing.T) {
	r, err := LoadRulesFile("../../research/rules/standard-spec.json")
	if err != nil || r.Name != "standard-spec" || r.ExitBasis != "avgcost" || !r.Dividends {
		t.Fatalf("%+v %v", r, err)
	}
}
