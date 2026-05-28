package config

import "time"

// WorkerConfig represents configuration for background workers
type WorkerConfig struct {
	// Channel buffer sizes
	MarketDataChannelBuffer int `yaml:"market_data_channel_buffer"`
	SignalChannelBuffer     int `yaml:"signal_channel_buffer"`

	// Reconnection settings
	ReconnectIntervalSeconds       int `yaml:"reconnect_interval_seconds"`
	MaxReconnectIntervalSeconds    int `yaml:"max_reconnect_interval_seconds"`
	ConnectionCheckIntervalSeconds int `yaml:"connection_check_interval_seconds"`
	StaleDataTimeoutSeconds        int `yaml:"stale_data_timeout_seconds"`

	// Database save settings
	MaxConcurrentSaves int `yaml:"max_concurrent_saves"`

	// Market data worker settings
	MarketData MarketDataWorkerConfig `yaml:"market_data"`

	// Signal worker settings
	Signal SignalWorkerConfig `yaml:"signal"`

	// Strategy worker settings
	Strategy StrategyWorkerConfig `yaml:"strategy"`

	// Maintenance worker settings
	Maintenance MaintenanceWorkerConfig `yaml:"maintenance"`
}

// MarketDataWorkerConfig represents market data worker configuration
type MarketDataWorkerConfig struct {
	// Reconnection settings
	ReconnectInterval       time.Duration `yaml:"reconnect_interval"`
	MaxReconnectInterval    time.Duration `yaml:"max_reconnect_interval"`
	ConnectionCheckInterval time.Duration `yaml:"connection_check_interval"`
}

// SignalWorkerConfig represents signal worker configuration
type SignalWorkerConfig struct {
	// Performance update interval
	PerformanceUpdateInterval time.Duration `yaml:"performance_update_interval"`

	// Strategy evaluation interval
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`
}

// StrategyWorkerConfig represents strategy worker configuration
type StrategyWorkerConfig struct {
	// Cleanup interval
	CleanupInterval time.Duration `yaml:"cleanup_interval"`

	// History retention
	HistoryRetentionHours int `yaml:"history_retention_hours"`

	// BarPeriod, when > 0, causes the strategy worker to aggregate incoming
	// market data ticks into time-aligned OHLCV bars (UTC-aligned) of this
	// length and invoke the strategy once per completed bar instead of on
	// every tick. The bar's Close price is exposed to the strategy as the
	// MarketData.Price for both the latest data point and each history
	// entry. Stop-loss and take-profit checks continue to run on every tick.
	// When 0 (the default) the worker uses the legacy per-tick behavior and
	// passes raw ticks through to the strategy unchanged.
	BarPeriod time.Duration `yaml:"bar_period"`
}

// MaintenanceWorkerConfig represents maintenance worker configuration
type MaintenanceWorkerConfig struct {
	// Cleanup interval
	CleanupInterval time.Duration `yaml:"cleanup_interval"`

	// Database maintenance interval
	MaintenanceInterval time.Duration `yaml:"maintenance_interval"`
}

// DefaultWorkerConfig returns default worker configuration
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		MarketDataChannelBuffer:        1000,
		SignalChannelBuffer:            100,
		ReconnectIntervalSeconds:       10,
		MaxReconnectIntervalSeconds:    300,
		ConnectionCheckIntervalSeconds: 30,
		StaleDataTimeoutSeconds:        180,
		MaxConcurrentSaves:             10,
		MarketData: MarketDataWorkerConfig{
			ReconnectInterval:       10 * time.Second,
			MaxReconnectInterval:    5 * time.Minute,
			ConnectionCheckInterval: 30 * time.Second,
		},
		Signal: SignalWorkerConfig{
			PerformanceUpdateInterval: 5 * time.Minute,
			EvaluationInterval:        1 * time.Second,
		},
		Strategy: StrategyWorkerConfig{
			CleanupInterval:       1 * time.Hour,
			HistoryRetentionHours: 24,
		},
		Maintenance: MaintenanceWorkerConfig{
			CleanupInterval:     24 * time.Hour,
			MaintenanceInterval: 7 * 24 * time.Hour,
		},
	}
}
