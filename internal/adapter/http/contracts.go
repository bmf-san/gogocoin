package api

import (
	"context"

	"github.com/bmf-san/gogocoin/internal/domain"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// DatabaseService defines the database operations needed by the API server.
// This interface follows the Consumer-Driven Contracts pattern, where each layer
// defines only the methods it needs. This decouples the API layer from the
// concrete database implementation and allows for independent evolution of layers.
type DatabaseService interface {
	// Balance operations
	GetLatestBalances() ([]domain.Balance, error)

	// Trade operations
	GetRecentTrades(limit int) ([]domain.Trade, error)
	GetTradesCount() (int, error)

	// Position operations
	GetActivePositions() ([]domain.Position, error)
	GetActivePositionsCount() (int, error)

	// Market data operations
	GetLatestMarketData(symbol string, limit int) ([]domain.MarketData, error)
	GetLatestMarketDataForSymbols(symbols []string) (map[string]domain.MarketData, error)

	// Performance metrics
	GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error)

	// Log operations
	GetRecentLogsWithFilters(limit int, level, category string) ([]domain.LogEntry, error)
}

// ApplicationService is the application service interface.
// It is defined here (consumer-driven) so adapter/http does not import cmd/ or usecase/.
type ApplicationService interface {
	GetBalances(ctx context.Context) ([]domain.Balance, error)
	GetCurrentStrategy() strategy.Strategy
	IsTradingEnabled() bool
	SetTradingEnabled(enabled bool) error
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	ConfigPath string
	WebRoot    string
	Version    string
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ConfigPath: "configs/config.yaml",
		WebRoot:    "web",
		Version:    "v1.0.0",
	}
}
