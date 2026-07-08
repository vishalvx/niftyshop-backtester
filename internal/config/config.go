package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	fmt.Println(">>> config init() called!!")
	validate = validator.New(validator.WithRequiredStructEnabled())
}

type PivotFilterConfig struct {
	Enabled  bool   `json:"enabled"`
	System   string `json:"system"    validate:"omitempty,oneof=classic fibonacci camarilla"`
	Level    string `json:"level"     validate:"omitempty,oneof=S1 S2 S3 S4 closest"`
	PoolSize int    `json:"pool_size" validate:"omitempty,gt=0"`
}

type Config struct {
	Universe               string            `json:"universe"                  validate:"required"`
	MAWindow               int32             `json:"ma_window"                 validate:"required,gt=0"`
	ProfitTargetPct        float64           `json:"profit_target_pct"         validate:"required,gt=0,lte=100"`
	AvgTriggerPct          float64           `json:"avg_trigger_pct"           validate:"required,gt=0,lte=100"`
	MaxStocks              int32             `json:"max_stocks"                validate:"required,gt=0"`
	CapitalDivider         float64           `json:"capital_divider"           validate:"required,gt=0"`
	StartCapital           int32             `json:"start_capital"             validate:"required,gt=0"`
	StartDate              string            `json:"start_date"                validate:"required,datetime=2006-01-02"`
	EndDate                string            `json:"end_date"                  validate:"required,datetime=2006-01-02"`
	RevisionPeriod         string            `json:"revision_period"           validate:"required,oneof=monthly quarterly yearly"`
	MaxFreshEntriesPerDay  int               `json:"max_fresh_entries_per_day" validate:"required,gt=0"`
	PivotFilter            PivotFilterConfig `json:"pivot_filter"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to open file %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("Unable to parse the file %w", err)
	}

	// Apply defaults for optional fields
	if config.RevisionPeriod == "" {
		config.RevisionPeriod = "monthly"
	}
	if config.MaxFreshEntriesPerDay == 0 {
		config.MaxFreshEntriesPerDay = 1
	}

	if err := validate.Struct(&config); err != nil {
		// Nice error formatting (very useful)
		var errs []string
		for _, e := range err.(validator.ValidationErrors) {
			errs = append(errs, e.Error()) // or use e.Translate() with translator
		}
		return nil, fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return &config, nil
}
