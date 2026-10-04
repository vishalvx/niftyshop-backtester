package experiment

import "github.com/vishalvx/back-tester/internal/sim"

// Standard returns the unfiltered strategy under legacy (engine) semantics.
func Standard() Spec { return Spec{ID: "standard-legacy", Rules: sim.LegacyRules()} }

// StandardSpec returns the unfiltered strategy under the semantics written in strategy.md.
func StandardSpec() Spec { return Spec{ID: "standard-spec", Rules: sim.SpecRules()} }

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
