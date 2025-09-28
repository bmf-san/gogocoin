package database

import (
	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// DatabaseInterface is the interface for database operations.
// Following Interface Segregation Principle, this is composed of smaller,
// focused interfaces rather than a monolithic Fat Interface.
// This interface composes multiple domain repository interfaces to provide
// the necessary capabilities to components that depend on it.
type DatabaseInterface interface {
	domain.TradingRepository      // Trade and position operations
	domain.AppStateRepository     // Application state persistence
	domain.DatabaseLifecycle      // Database lifecycle management
	domain.AnalyticsRepository    // Performance analytics operations
	domain.MaintenanceRepository  // Database maintenance operations
	domain.MarketDataRepository   // Market data persistence
}
