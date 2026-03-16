package strategy

import "context"

// Strategy is the interface every trading strategy must implement.
// Implementations are registered with the engine via engine.WithStrategy()
// and are instantiated fresh for each engine.Run() call.
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
}
