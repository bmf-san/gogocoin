package config

// StrategyRuntimeConfig represents runtime configuration for strategies
type StrategyRuntimeConfig struct {
	// Sell size percentage (0.0-1.0)
	SellSizePercentage float64 `yaml:"sell_size_percentage"`

	// Maximum history limit for market data
	HistoryLimit int `yaml:"history_limit"`

	// Signal strength threshold
	SignalStrengthThreshold float64 `yaml:"signal_strength_threshold"`
}

// TradingRuntimeConfig represents runtime trading configuration
type TradingRuntimeConfig struct {
	// Order timeout
	OrderTimeout int `yaml:"order_timeout_seconds"`

	// Price precision (decimal places)
	PricePrecision int `yaml:"price_precision"`

	// Size precision (decimal places)
	SizePrecision int `yaml:"size_precision"`
}

// DefaultStrategyRuntimeConfig returns default strategy runtime configuration
func DefaultStrategyRuntimeConfig() StrategyRuntimeConfig {
	return StrategyRuntimeConfig{
		SellSizePercentage:      0.95,
		HistoryLimit:            1000,
		SignalStrengthThreshold: 0.5,
	}
}

// DefaultTradingRuntimeConfig returns default trading runtime configuration
func DefaultTradingRuntimeConfig() TradingRuntimeConfig {
	return TradingRuntimeConfig{
		OrderTimeout:   30,
		PricePrecision: 0,
		SizePrecision:  8,
	}
}
