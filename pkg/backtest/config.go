package backtest

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the schema of configs/backtest.yaml.
type Config struct {
	Data      DataConfig                `yaml:"data"`
	Simulator SimulatorYAML             `yaml:"simulator"`
	Scenarios map[string]ScenarioConfig `yaml:"scenarios"`
}

// DataConfig points to the historical data.
type DataConfig struct {
	Source string `yaml:"source"` // "sqlite" | "csv"
	Path   string `yaml:"path"`
	Symbol string `yaml:"symbol"`
	From   string `yaml:"from"` // YYYY-MM-DD
	To     string `yaml:"to"`
	// BarPeriod aggregates the raw source bars into larger time-based bars
	// before feeding them to the strategy. Accepts Go duration strings
	// (e.g. "5m", "1h", "4h"). Empty/unset = pass through raw bars.
	BarPeriod string `yaml:"bar_period"`
}

// SimulatorYAML is the YAML mirror of SimulatorConfig.
type SimulatorYAML struct {
	InitialBalance float64 `yaml:"initial_balance"`
	FeeRate        float64 `yaml:"fee_rate"`
	SlippageBps    float64 `yaml:"slippage_bps"`
	SameBarRule    string  `yaml:"same_bar_rule"`
	MinVolumeRatio float64 `yaml:"min_volume_ratio"`
}

// ScenarioConfig describes a single backtest case.
type ScenarioConfig struct {
	Strategy     string                   `yaml:"strategy"`
	Params       map[string]interface{}   `yaml:"params"`
	Grid         map[string][]interface{} `yaml:"grid"`
	Fixed        map[string]interface{}   `yaml:"fixed"`
	DataOverride *DataOverride            `yaml:"data_override,omitempty"`
	// HistoryLimit overrides the engine's default sliding history size for
	// this scenario. Use this when a strategy needs more than ~1000 bars
	// of look-back (e.g. swing strategies with 200-bar EMAs on 1h data).
	HistoryLimit int `yaml:"history_limit"`
}

// DataOverride lets a scenario override fields on the top-level data block.
// Empty fields fall through to the parent DataConfig.
type DataOverride struct {
	Source    string `yaml:"source"`
	Path      string `yaml:"path"`
	Symbol    string `yaml:"symbol"`
	From      string `yaml:"from"`
	To        string `yaml:"to"`
	BarPeriod string `yaml:"bar_period"`
}

// Apply returns a DataConfig with override fields merged on top of base.
func (o *DataOverride) Apply(base DataConfig) DataConfig {
	if o == nil {
		return base
	}
	out := base
	if o.Source != "" {
		out.Source = o.Source
	}
	if o.Path != "" {
		out.Path = o.Path
	}
	if o.Symbol != "" {
		out.Symbol = o.Symbol
	}
	if o.From != "" {
		out.From = o.From
	}
	if o.To != "" {
		out.To = o.To
	}
	if o.BarPeriod != "" {
		out.BarPeriod = o.BarPeriod
	}
	return out
}

// LoadConfig reads + parses a YAML config file.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return &c, nil
}

// ParseDate parses YYYY-MM-DD; returns zero time when s is empty.
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

// ToSimulatorConfig converts the YAML form to runtime form.
func (s SimulatorYAML) ToSimulatorConfig() SimulatorConfig {
	cfg := DefaultSimulatorConfig()
	if s.InitialBalance > 0 {
		cfg.InitialBalance = s.InitialBalance
	}
	if s.FeeRate > 0 {
		cfg.FeeRate = s.FeeRate
	}
	if s.SlippageBps > 0 {
		cfg.SlippageBps = s.SlippageBps
	}
	if s.SameBarRule != "" {
		cfg.SameBarRule = SameBarRule(s.SameBarRule)
	}
	if s.MinVolumeRatio > 0 {
		cfg.MinVolumeRatio = s.MinVolumeRatio
	}
	return cfg
}
