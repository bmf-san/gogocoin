package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application-wide configuration
type Config struct {
	App            AppConfig      `yaml:"app"`
	API            APIConfig      `yaml:"api"`
	Trading        TradingConfig  `yaml:"trading"`
	StrategyParams StrategyParams `yaml:"strategy_params"`
	Data           DataConfig     `yaml:"data"`
	UI             UIConfig       `yaml:"ui"`
	Logging        LoggingConfig  `yaml:"logging"`

	// Runtime configuration (set from command line arguments)
	Mode string `yaml:"-"` // Not included in YAML
}

// AppConfig represents basic application settings
type AppConfig struct {
	Name string `yaml:"name"`
}

// APIConfig represents bitFlyer API settings
type APIConfig struct {
	Endpoint          string            `yaml:"endpoint"`
	WebSocketEndpoint string            `yaml:"websocket_endpoint"`
	Credentials       CredentialsConfig `yaml:"credentials"`
	Timeout           time.Duration     `yaml:"timeout"`
	RetryCount        int               `yaml:"retry_count"`
	RateLimit         RateLimitConfig   `yaml:"rate_limit"`
	Port              int               `yaml:"port"`
}

// CredentialsConfig represents API credentials
type CredentialsConfig struct {
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
}

// RateLimitConfig represents rate limit settings
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

// TradingConfig represents trading settings
type TradingConfig struct {
	FeeRate        float64              `yaml:"fee_rate"`
	InitialBalance float64              `yaml:"initial_balance"`
	Symbols        []string             `yaml:"symbols"`
	Strategy       StrategyConfig       `yaml:"strategy"`
	RiskManagement RiskManagementConfig `yaml:"risk_management"`
}

// StrategyConfig represents strategy settings
type StrategyConfig struct {
	Name string `yaml:"name"`
}

// RiskManagementConfig represents risk management settings
type RiskManagementConfig struct {
	MaxTotalLossPercent   float64 `yaml:"max_total_loss_percent"`
	MaxTradeLossPercent   float64 `yaml:"max_trade_loss_percent"`
	MaxDailyLossPercent   float64 `yaml:"max_daily_loss_percent"`
	MaxTradeAmountPercent float64 `yaml:"max_trade_amount_percent"`
	MaxDailyTrades        int     `yaml:"max_daily_trades"`
	MinTradeInterval      string  `yaml:"min_trade_interval"`
	StopLossPercent       float64 `yaml:"stop_loss_percent"`
	TakeProfitPercent     float64 `yaml:"take_profit_percent"`
}

// StrategyParams represents strategy-specific parameters
type StrategyParams struct {
	MovingAverageCross MovingAverageCrossParams `yaml:"moving_average_cross"`
	SimpleTest         SimpleTestParams         `yaml:"simple_test"`
}

// MovingAverageCrossParams represents moving average cross strategy parameters
type MovingAverageCrossParams struct {
	ShortPeriod      int     `yaml:"short_period"`
	LongPeriod       int     `yaml:"long_period"`
	MAType           string  `yaml:"ma_type"`
	MinCrossStrength float64 `yaml:"min_cross_strength"`
}

// SimpleTestParams represents simple test strategy parameters
type SimpleTestParams struct {
	TradeInterval        int     `yaml:"trade_interval"`
	FixedAmount          float64 `yaml:"fixed_amount"`
	MaxTrades            int     `yaml:"max_trades"`
	PriceChangeThreshold float64 `yaml:"price_change_threshold"`
}

// DataConfig represents data management settings
type DataConfig struct {
	MarketData MarketDataConfig `yaml:"market_data"`
}

// MarketDataConfig represents market data settings
type MarketDataConfig struct {
	HistoryDays int `yaml:"history_days"`
}

// UIConfig represents Web UI settings
type UIConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LoggingConfig represents logging settings
type LoggingConfig struct {
	Level      string            `yaml:"level"`
	Format     string            `yaml:"format"`
	Output     string            `yaml:"output"`
	FilePath   string            `yaml:"file_path"`
	MaxSizeMB  int               `yaml:"max_size_mb"`
	MaxBackups int               `yaml:"max_backups"`
	MaxAgeDays int               `yaml:"max_age_days"`
	Categories map[string]string `yaml:"categories"`
}

// Load reads the configuration file
func Load(configPath string) (*Config, error) {
	// Read configuration file
	// #nosec G304 - Configuration file path is specified by command line argument
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData := expandEnvVars(string(data))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate checks the validity of the configuration
func (c *Config) Validate() error {
	// Validate application settings
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	// Validate trading settings
	// Mode is set at runtime, so not validated here

	if c.Trading.InitialBalance <= 0 {
		return fmt.Errorf("trading.initial_balance must be positive")
	}

	if len(c.Trading.Symbols) == 0 {
		return fmt.Errorf("trading.symbols must not be empty")
	}

	// risk management configurationof検証
	rm := c.Trading.RiskManagement
	if rm.MaxTotalLossPercent <= 0 || rm.MaxTotalLossPercent > 100 {
		return fmt.Errorf("risk_management.max_total_loss_percent must be between 0 and 100")
	}

	if rm.MaxTradeAmountPercent <= 0 || rm.MaxTradeAmountPercent > 100 {
		return fmt.Errorf("risk_management.max_trade_amount_percent must be between 0 and 100")
	}

	// API configurationof検証
	if c.API.Endpoint == "" {
		return fmt.Errorf("api.endpoint is required")
	}

	if c.Mode == "live" {
		if c.API.Credentials.APIKey == "" || c.API.Credentials.APISecret == "" {
			return fmt.Errorf("api credentials are required for live trading")
		}
	}

	// UIconfigurationof検証
	if c.UI.Port <= 0 || c.UI.Port > 65535 {
		return fmt.Errorf("ui.port must be between 1 and 65535")
	}

	return nil
}

// IsPaperTrading returns whether paper trading mode is enabled
func (c *Config) IsPaperTrading() bool {
	return c.Mode == "paper" || c.Mode == "dev"
}

// IsLiveTrading returns whether live trading mode is enabled
func (c *Config) IsLiveTrading() bool {
	return c.Mode == "live"
}

// expandEnvVars expands environment variables in a string
func expandEnvVars(s string) string {
	return os.Expand(s, os.Getenv)
}

// GetStrategyParams returns parameters for the specified strategy
func (c *Config) GetStrategyParams(strategyName string) (interface{}, error) {
	switch strategyName {
	case "moving_average_cross":
		return c.StrategyParams.MovingAverageCross, nil
	case "simple_test":
		return c.StrategyParams.SimpleTest, nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", strategyName)
	}
}

// String returns a string representation of the configuration (masks sensitive information)
func (c *Config) String() string {
	// Create a copy with masked sensitive information
	masked := *c
	if masked.API.Credentials.APIKey != "" {
		masked.API.Credentials.APIKey = strings.Repeat("*", len(masked.API.Credentials.APIKey))
	}
	if masked.API.Credentials.APISecret != "" {
		masked.API.Credentials.APISecret = strings.Repeat("*", len(masked.API.Credentials.APISecret))
	}

	data, _ := yaml.Marshal(&masked)
	return string(data)
}
