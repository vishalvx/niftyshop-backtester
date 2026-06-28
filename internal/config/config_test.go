package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}

func TestLoad_RevisionPeriodMonthly(t *testing.T) {
	path := writeConfigFile(t, `{
		"universe": "nifty50",
		"ma_window": 20,
		"profit_target_pct": 0.05,
		"avg_trigger_pct": 0.03,
		"max_stocks": 5,
		"capital_divider": 10,
		"start_capital": 100000,
		"start_date": "2021-01-01",
		"end_date": "2025-12-31",
		"revision_period": "monthly",
		"max_fresh_entries_per_day": 1
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.RevisionPeriod != "monthly" {
		t.Errorf("expected RevisionPeriod 'monthly', got '%s'", cfg.RevisionPeriod)
	}
	if cfg.MaxFreshEntriesPerDay != 1 {
		t.Errorf("expected MaxFreshEntriesPerDay 1, got %d", cfg.MaxFreshEntriesPerDay)
	}
}

func TestLoad_RevisionPeriodQuarterly(t *testing.T) {
	path := writeConfigFile(t, `{
		"universe": "nifty50",
		"ma_window": 20,
		"profit_target_pct": 0.05,
		"avg_trigger_pct": 0.03,
		"max_stocks": 5,
		"capital_divider": 10,
		"start_capital": 100000,
		"start_date": "2021-01-01",
		"end_date": "2025-12-31",
		"revision_period": "quarterly",
		"max_fresh_entries_per_day": 2
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.RevisionPeriod != "quarterly" {
		t.Errorf("expected RevisionPeriod 'quarterly', got '%s'", cfg.RevisionPeriod)
	}
}

func TestLoad_InvalidRevisionPeriod(t *testing.T) {
	path := writeConfigFile(t, `{
		"universe": "nifty50",
		"ma_window": 20,
		"profit_target_pct": 0.05,
		"avg_trigger_pct": 0.03,
		"max_stocks": 5,
		"capital_divider": 10,
		"start_capital": 100000,
		"start_date": "2021-01-01",
		"end_date": "2025-12-31",
		"revision_period": "weekly",
		"max_fresh_entries_per_day": 1
	}`)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid revision_period 'weekly', got nil")
	}
}

func TestLoad_MissingRevisionPeriodUsesDefault(t *testing.T) {
	// When revision_period is absent, Load should apply a default of "monthly"
	path := writeConfigFile(t, `{
		"universe": "nifty50",
		"ma_window": 20,
		"profit_target_pct": 0.05,
		"avg_trigger_pct": 0.03,
		"max_stocks": 5,
		"capital_divider": 10,
		"start_capital": 100000,
		"start_date": "2021-01-01",
		"end_date": "2025-12-31",
		"max_fresh_entries_per_day": 1
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for missing revision_period, got: %v", err)
	}
	if cfg.RevisionPeriod != "monthly" {
		t.Errorf("expected default RevisionPeriod 'monthly', got '%s'", cfg.RevisionPeriod)
	}
}

func TestLoad_MissingMaxFreshEntriesUsesDefault(t *testing.T) {
	// When max_fresh_entries_per_day is absent, Load should apply default of 1
	path := writeConfigFile(t, `{
		"universe": "nifty50",
		"ma_window": 20,
		"profit_target_pct": 0.05,
		"avg_trigger_pct": 0.03,
		"max_stocks": 5,
		"capital_divider": 10,
		"start_capital": 100000,
		"start_date": "2021-01-01",
		"end_date": "2025-12-31",
		"revision_period": "monthly"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for missing max_fresh_entries_per_day, got: %v", err)
	}
	if cfg.MaxFreshEntriesPerDay != 1 {
		t.Errorf("expected default MaxFreshEntriesPerDay 1, got %d", cfg.MaxFreshEntriesPerDay)
	}
}

func TestConfig_JSONRoundTrip(t *testing.T) {
	original := Config{
		Universe:              "nifty50",
		MAWindow:              20,
		ProfitTargetPct:       0.05,
		AvgTriggerPct:         0.03,
		MaxStocks:             5,
		CapitalDivider:        10,
		StartCapital:          100000,
		StartDate:             "2021-01-01",
		EndDate:               "2025-12-31",
		RevisionPeriod:        "yearly",
		MaxFreshEntriesPerDay: 3,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.RevisionPeriod != original.RevisionPeriod {
		t.Errorf("RevisionPeriod round-trip: want %q, got %q", original.RevisionPeriod, got.RevisionPeriod)
	}
	if got.MaxFreshEntriesPerDay != original.MaxFreshEntriesPerDay {
		t.Errorf("MaxFreshEntriesPerDay round-trip: want %d, got %d", original.MaxFreshEntriesPerDay, got.MaxFreshEntriesPerDay)
	}
}
