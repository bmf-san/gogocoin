package strategy

import "context"

// AutoScaleConfig holds the order-size auto-scaling parameters that the engine
// needs to compute effective buy notional. Returned by Strategy.GetAutoScaleConfig.
type AutoScaleConfig struct {
	Enabled     bool
	BalancePct  float64 // percentage of available JPY balance to use (0-100)
	MaxNotional float64 // hard cap in JPY; 0 = unlimited
	FeeRate     float64
}

// Strategy is the interface every trading strategy must implement.
// Register implementations via strategy.Register() in an init() function so
// that main.go only needs a blank import to make the strategy available.
type Strategy interface {
	// GenerateSignal generates a signal from the latest market data point and
	// the historical series for the same symbol.
	GenerateSignal(ctx context.Context, data *MarketData, history []MarketData) (*Signal, error)

	// Analyze generates a signal from a batch of historical data.
	Analyze(data []MarketData) (*Signal, error)

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	GetStatus() StrategyStatus
	Reset() error

	// Metrics & trade accounting
	GetMetrics() StrategyMetrics
	RecordTrade()
	InitializeDailyTradeCount(count int)

	// Configuration
	Name() string
	Description() string
	Version() string
	// Initialize is called once with the strategy_params block from config.yaml.
	// Keys and value types mirror the YAML structure (e.g. "ema_fast_period" → int).
	Initialize(config map[string]interface{}) error
	UpdateConfig(config map[string]interface{}) error
	GetConfig() map[string]interface{}

	// Order sizing — implemented by each strategy so the engine never needs to
	// read strategy-specific config keys directly.

	// GetStopLossPrice returns the stop-loss exit price for a long position
	// opened at entry. Returns 0 when stop-loss is not configured.
	GetStopLossPrice(entry float64) float64

	// GetTakeProfitPrice returns the take-profit exit price for a long position
	// opened at entry. Returns 0 when take-profit is not configured.
	GetTakeProfitPrice(entry float64) float64

	// GetBaseNotional returns the base JPY notional for a single order on symbol.
	// The engine uses this as the floor when computing auto-scaled order sizes.
	GetBaseNotional(symbol string) float64

	// GetAutoScaleConfig returns the auto-scaling configuration so the engine
	// can apply balance-aware order sizing without knowing strategy internals.
	GetAutoScaleConfig() AutoScaleConfig
}
