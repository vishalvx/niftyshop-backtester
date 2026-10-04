package experiment

import (
	"fmt"

	"github.com/vishalvx/back-tester/internal/sim"
)

// Standard returns the unfiltered strategy under legacy (engine) semantics.
func Standard() Spec { return Spec{ID: "standard-legacy", Rules: sim.LegacyRules()} }

// StandardSpec returns the unfiltered strategy under the semantics written in strategy.md.
func StandardSpec() Spec { return Spec{ID: "standard-spec", Rules: sim.SpecRules()} }

// VPivot returns a V-Pivot variant under legacy semantics.
func VPivot(system, level string, pool int) Spec {
	r := sim.LegacyRules()
	r.Pivot = &sim.PivotRule{System: system, Level: level, Pool: pool}
	r.Name = fmt.Sprintf("vpivot-%s-%s-%d", system, level, pool)
	return Spec{ID: r.Name, Rules: r}
}

// VPivotSpec returns a V-Pivot variant under documented (spec) semantics.
func VPivotSpec(system, level string, pool int) Spec {
	r := sim.SpecRules()
	r.Pivot = &sim.PivotRule{System: system, Level: level, Pool: pool}
	r.Name = fmt.Sprintf("vpivot-spec-%s-%s-%d", system, level, pool)
	return Spec{ID: r.Name, Rules: r}
}

// PivotGrid is the 39-point grid the retired original engine's -find-best-pivot flag searched.
func PivotGrid(base func(system, level string, pool int) Spec) []Spec {
	var out []Spec
	systems := []struct {
		name   string
		levels []string
	}{
		{"classic", []string{"S1", "S2", "S3", "closest"}},
		{"fibonacci", []string{"S1", "S2", "S3", "closest"}},
		{"camarilla", []string{"S1", "S2", "S3", "S4", "closest"}},
	}
	for _, pool := range []int{5, 10, 15} {
		for _, s := range systems {
			for _, l := range s.levels {
				out = append(out, base(s.name, l, pool))
			}
		}
	}
	return out
}

// FromPreset wraps a sim preset as a Spec.
func FromPreset(name string) (Spec, error) {
	r, err := sim.Preset(name)
	if err != nil {
		return Spec{}, err
	}
	return Spec{ID: r.Name, Rules: r}, nil
}

// AppApprox returns the app rule set (whole-position exit approximation) (see sim.Preset).
func AppApprox() Spec { s, _ := FromPreset("app-approx"); return s }
