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
	App            AppConfig             `yaml:"app"`
	API            APIConfig             `yaml:"api"`
	Trading        TradingConfig         `yaml:"trading"`
	StrategyParams StrategyParams        `yaml:"strategy_params"`
	Data           DataConfig            `yaml:"data"`
	UI             UIConfig              `yaml:"ui"`
	Logging        LoggingConfig         `yaml:"logging"`
	DataRetention  DataRetentionConfig   `yaml:"data_retention"`
	Worker         WorkerConfig          `yaml:"worker"`
	Runtime        StrategyRuntimeConfig `yaml:"runtime"`
	TradingRuntime TradingRuntimeConfig  `yaml:"trading_runtime"`
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
	MaxTradeAmountPercent        float64 `yaml:"max_trade_amount_percent"`
	MaxDailyTrades               int     `yaml:"max_daily_trades"`
	MinTradeInterval             string  `yaml:"min_trade_interval"`
	MaxOpenPositionsPerSymbol    int     `yaml:"max_open_positions_per_symbol"` // 0 = unlimited
}

// StrategyParams represents strategy-specific parameters
type StrategyParams struct {
	Scalping ScalpingParams `yaml:"scalping"`
}

// ScalpingParams represents scalping strategy parameters
type ScalpingParams struct {
	EMAFastPeriod  int     `yaml:"ema_fast_period"`
	EMASlowPeriod  int     `yaml:"ema_slow_period"`
	// TrendEMAPeriod is the long-term EMA used as a trend direction filter.
	// BUY signals are suppressed when price < EMA(TrendEMAPeriod). 0 = disabled.
	TrendEMAPeriod int     `yaml:"trend_ema_period"`
	TakeProfitPct  float64 `yaml:"take_profit_pct"`
	StopLossPct    float64 `yaml:"stop_loss_pct"`
	CooldownSec    int     `yaml:"cooldown_sec"`
	MaxDailyTrades int     `yaml:"max_daily_trades"`
	OrderNotional  float64 `yaml:"order_notional"`
	// Auto-scale buy order notional by current JPY balance.
	AutoScaleEnabled     bool    `yaml:"auto_scale_enabled"`
	AutoScaleBalancePct  float64 `yaml:"auto_scale_balance_pct"`
	AutoScaleMaxNotional float64 `yaml:"auto_scale_max_notional"`
	FeeRate              float64 `yaml:"fee_rate"`
	// RSI filter (0 = disabled)
	RSIPeriod     int     `yaml:"rsi_period"`
	RSIOverbought float64 `yaml:"rsi_overbought"`
	RSIOversold   float64 `yaml:"rsi_oversold"`
	// Per-symbol parameter overrides
	SymbolParams map[string]ScalpingSymbolOverride `yaml:"symbol_params"`
}

// ScalpingSymbolOverride holds per-symbol overrides for the scalping strategy.
// Zero values mean "use the global default".
type ScalpingSymbolOverride struct {
	EMAFastPeriod int     `yaml:"ema_fast_period"`
	EMASlowPeriod int     `yaml:"ema_slow_period"`
	CooldownSec   int     `yaml:"cooldown_sec"`
	OrderNotional float64 `yaml:"order_notional"`
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

// DataRetentionConfig represents data retention and cleanup settings
type DataRetentionConfig struct {
	// RetentionDays specifies how many days of data to keep in the database
	// - 1 (default): Keep only today's data (delete yesterday and older)
	// - 2: Keep today + yesterday (delete 2 days ago and older)
	// - 7: Keep last 7 days (delete 8 days ago and older)
	// Daily cleanup runs at midnight (00:00)
	RetentionDays int `yaml:"retention_days"`
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

	if c.Trading.InitialBalance <= 0 {
		return fmt.Errorf("trading.initial_balance must be positive")
	}

	if len(c.Trading.Symbols) == 0 {
		return fmt.Errorf("trading.symbols must not be empty")
	}

	// Validate risk management configuration
	rm := c.Trading.RiskManagement
	if rm.MaxTotalLossPercent <= 0 || rm.MaxTotalLossPercent > 100 {
		return fmt.Errorf("risk_management.max_total_loss_percent must be between 0 and 100")
	}

	if rm.MaxTradeAmountPercent <= 0 || rm.MaxTradeAmountPercent > 100 {
		return fmt.Errorf("risk_management.max_trade_amount_percent must be between 0 and 100")
	}

        // Default API timeout to 30 s if not set in config.
        if c.API.Timeout == 0 {
                c.API.Timeout = 30 * time.Second
        }

        // Validate API configuration
	if c.API.Endpoint == "" {
		return fmt.Errorf("api.endpoint is required")
	}

	if c.API.Credentials.APIKey == "" || c.API.Credentials.APISecret == "" {
		return fmt.Errorf("api credentials are required")
	}

	// Validate UI configuration
	if c.UI.Port <= 0 || c.UI.Port > 65535 {
		return fmt.Errorf("ui.port must be between 1 and 65535")
	}

	// Data retention configuration
	if c.DataRetention.RetentionDays < 0 {
		return fmt.Errorf("data_retention.retention_days must be non-negative")
	}
	// Default to 1 day if not specified or set to 0
	if c.DataRetention.RetentionDays == 0 {
		c.DataRetention.RetentionDays = 1
	}

	// Apply default worker configuration per field if not specified.
	// Replacing the whole struct would discard fields the user intentionally configured.
	{
		defaults := DefaultWorkerConfig()
		if c.Worker.MarketDataChannelBuffer == 0 {
			c.Worker.MarketDataChannelBuffer = defaults.MarketDataChannelBuffer
		}
		if c.Worker.SignalChannelBuffer == 0 {
			c.Worker.SignalChannelBuffer = defaults.SignalChannelBuffer
		}
		if c.Worker.ReconnectIntervalSeconds == 0 {
			c.Worker.ReconnectIntervalSeconds = defaults.ReconnectIntervalSeconds
		}
		if c.Worker.MaxReconnectIntervalSeconds == 0 {
			c.Worker.MaxReconnectIntervalSeconds = defaults.MaxReconnectIntervalSeconds
		}
		if c.Worker.ConnectionCheckIntervalSeconds == 0 {
			c.Worker.ConnectionCheckIntervalSeconds = defaults.ConnectionCheckIntervalSeconds
		}
		if c.Worker.StaleDataTimeoutSeconds == 0 {
			c.Worker.StaleDataTimeoutSeconds = defaults.StaleDataTimeoutSeconds
		}
		if c.Worker.MaxConcurrentSaves == 0 {
			c.Worker.MaxConcurrentSaves = defaults.MaxConcurrentSaves
		}
		if c.Worker.MarketData == (MarketDataWorkerConfig{}) {
			c.Worker.MarketData = defaults.MarketData
		}
		if c.Worker.Signal == (SignalWorkerConfig{}) {
			c.Worker.Signal = defaults.Signal
		}
		if c.Worker.Strategy == (StrategyWorkerConfig{}) {
			c.Worker.Strategy = defaults.Strategy
		}
		if c.Worker.Maintenance == (MaintenanceWorkerConfig{}) {
			c.Worker.Maintenance = defaults.Maintenance
		}
	}

	// Validate worker configuration
	if c.Worker.MarketDataChannelBuffer <= 0 {
		return fmt.Errorf("worker.market_data_channel_buffer must be positive")
	}
	if c.Worker.SignalChannelBuffer <= 0 {
		return fmt.Errorf("worker.signal_channel_buffer must be positive")
	}
	if c.Worker.ReconnectIntervalSeconds <= 0 {
		return fmt.Errorf("worker.reconnect_interval_seconds must be positive")
	}
	if c.Worker.MaxConcurrentSaves <= 0 {
		return fmt.Errorf("worker.max_concurrent_saves must be positive")
	}

	// Apply default runtime configuration when fields are zero / not present in YAML.
	// Without this, omitting the [runtime] section causes sell_size_percentage to be
	// 0.0 (Go zero value), which makes getAvailableSellSize always return 0 and every
	// SELL signal to be skipped with "no crypto holdings available".
	{
		runtimeDefaults := DefaultStrategyRuntimeConfig()
		if c.Runtime.SellSizePercentage == 0 {
			c.Runtime.SellSizePercentage = runtimeDefaults.SellSizePercentage
		}
		if c.Runtime.HistoryLimit == 0 {
			c.Runtime.HistoryLimit = runtimeDefaults.HistoryLimit
		}
		if c.Runtime.SignalStrengthThreshold == 0 {
			c.Runtime.SignalStrengthThreshold = runtimeDefaults.SignalStrengthThreshold
		}
	}

	// Validate runtime configuration
	if c.Runtime.SellSizePercentage <= 0 || c.Runtime.SellSizePercentage > 1 {
		return fmt.Errorf("runtime.sell_size_percentage must be between 0 (exclusive) and 1 (inclusive), got %v", c.Runtime.SellSizePercentage)
	}

	return nil
}

// expandEnvVars expands environment variables in a string
func expandEnvVars(s string) string {
	return os.Expand(s, os.Getenv)
}

// GetStrategyParams returns parameters for the specified strategy
func (c *Config) GetStrategyParams(strategyName string) (interface{}, error) {
	switch strategyName {
	case "scalping":
		return c.StrategyParams.Scalping, nil
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
