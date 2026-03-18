package domain

// TradeRepository is the interface for trade data persistence
type TradeRepository interface {
	SaveTrade(trade *Trade) error
	GetRecentTrades(limit int) ([]Trade, error)
	GetAllTrades() ([]Trade, error)
}

// PositionRepository is the interface for position data persistence
type PositionRepository interface {
	SavePosition(position *Position) error
	GetOpenPositions(symbol string, side string) ([]Position, error)
	UpdatePosition(position *Position) error
}

// BalanceRepository is the interface for balance data persistence
type BalanceRepository interface {
	SaveBalance(balance Balance) error
}

// LogRepository is the interface for log data persistence
type LogRepository interface {
	SaveLog(entry *LogEntry) error
}

// MarketDataRepository is the interface for market data persistence
type MarketDataRepository interface {
	SaveMarketData(data *MarketData) error // Save market data to database
	SaveTicker(ticker *MarketData) error   // Alias for SaveMarketData (backward compatibility)
}

// PerformanceRepository is the interface for performance metrics persistence
type PerformanceRepository interface {
	SavePerformanceMetric(metric *PerformanceMetric) error
	GetLatestPerformanceMetric() (*PerformanceMetric, error)
}

// TransactionManager provides transaction support for atomic operations
type TransactionManager interface {
	BeginTx() (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	Commit() error
	Rollback() error
	SaveTrade(trade *Trade) error
	SavePosition(position *Position) error
	UpdatePosition(position *Position) error
}

// TradingRepository is the unified interface for trading-related persistence.
//
// Deprecated: Use individual repositories (TradeRepository, PositionRepository,
// BalanceRepository, TransactionManager) with separate injection.
// Will be removed in Phase 5 once all callers have been migrated.
type TradingRepository interface {
	TradeRepository
	PositionRepository
	BalanceRepository
	TransactionManager
}

// AnalyticsRepository provides performance analytics persistence operations
type AnalyticsRepository interface {
	SavePerformanceMetric(metric *PerformanceMetric) error
	GetPerformanceMetrics(days int) ([]PerformanceMetric, error)
}

// AppStateRepository provides application state persistence operations
type AppStateRepository interface {
	SaveAppState(key, value string) error
	GetAppState(key string) (string, error)
}

// DatabaseLifecycle provides database lifecycle management operations
type DatabaseLifecycle interface {
	Close() error
	Ping() error
}

// MaintenanceRepository provides database maintenance operations
type MaintenanceRepository interface {
	GetDatabaseSize() (int64, error)
	CleanupOldData(retentionDays int) error
	GetTableStats() (map[string]int, error)
}
