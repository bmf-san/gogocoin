package strategy

import (
	"context"
)

// SignalGenerator generates trading signals from market data
type SignalGenerator interface {
	// GenerateSignal generates a trading signal based on current and historical market data
	GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error)

	// Analyze analyzes market data and returns a trading signal
	Analyze(data []MarketData) (*Signal, error)
}

// StrategyLifecycle manages the lifecycle of a strategy
type StrategyLifecycle interface {
	// Start starts the strategy
	Start(ctx context.Context) error

	// Stop stops the strategy
	Stop(ctx context.Context) error

	// IsRunning returns true if the strategy is running
	IsRunning() bool

	// GetStatus returns the current status of the strategy
	GetStatus() StrategyStatus

	// Reset resets the strategy state
	Reset() error
}

// StrategyMetricsProvider provides performance metrics
type StrategyMetricsProvider interface {
	// GetMetrics returns the performance metrics of the strategy
	GetMetrics() StrategyMetrics

	// RecordTrade records a completed trade
	RecordTrade()

	// InitializeDailyTradeCount initializes the daily trade count
	InitializeDailyTradeCount(count int)
}

// StrategyConfiguration manages strategy configuration
type StrategyConfiguration interface {
	// Name returns the strategy name
	Name() string

	// Description returns the strategy description
	Description() string

	// Version returns the strategy version
	Version() string

	// Initialize initializes the strategy with configuration
	Initialize(config map[string]interface{}) error

	// UpdateConfig updates the strategy configuration
	UpdateConfig(config map[string]interface{}) error

	// GetConfig returns the current configuration
	GetConfig() map[string]interface{}
}

// Strategy is the complete trading strategy interface
// It combines all strategy capabilities
type Strategy interface {
	SignalGenerator
	StrategyLifecycle
	StrategyMetricsProvider
	StrategyConfiguration
}

// MinimalStrategy is a minimal interface for strategies that only need to generate signals
type MinimalStrategy interface {
	SignalGenerator
	StrategyConfiguration
}
