package app

import (
	"github.com/bmf-san/gogocoin/v1/internal/analytics"
	"github.com/bmf-san/gogocoin/v1/internal/api"
	"github.com/bmf-san/gogocoin/v1/internal/risk"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
)

// ServiceRegistry manages all application services and their lifecycle.
// This reduces the Application struct's responsibility by centralizing
// service management in a dedicated component.
//
// Benefits:
// - Single source of truth for service references
// - Easier to test (mock the entire registry)
// - Clear service lifecycle management
// - Enables service dependency validation
type ServiceRegistry struct {
	// Core services
	Database             Database
	BitflyerClient       BitflyerClient
	MarketDataService    MarketDataService
	MarketSpecService    MarketSpecService
	TradingService       trading.Trader
	RiskManager          risk.RiskManager
	PerformanceAnalytics analytics.PerformanceAnalyticsService
	CurrentStrategy      strategy.Strategy
	APIServer            *api.Server
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{}
}

// Validate checks that all required services are initialized
func (r *ServiceRegistry) Validate() error {
	if r.Database == nil {
		return ErrServiceNotInitialized("Database")
	}
	if r.TradingService == nil {
		return ErrServiceNotInitialized("TradingService")
	}
	if r.RiskManager == nil {
		return ErrServiceNotInitialized("RiskManager")
	}
	if r.PerformanceAnalytics == nil {
		return ErrServiceNotInitialized("PerformanceAnalytics")
	}
	if r.CurrentStrategy == nil {
		return ErrServiceNotInitialized("CurrentStrategy")
	}
	if r.APIServer == nil {
		return ErrServiceNotInitialized("APIServer")
	}
	return nil
}

// GetTradingService returns the trading service
func (r *ServiceRegistry) GetTradingService() trading.Trader {
	return r.TradingService
}

// GetRiskManager returns the risk manager
func (r *ServiceRegistry) GetRiskManager() risk.RiskManager {
	return r.RiskManager
}

// GetPerformanceAnalytics returns the performance analytics service
func (r *ServiceRegistry) GetPerformanceAnalytics() analytics.PerformanceAnalyticsService {
	return r.PerformanceAnalytics
}

// GetStrategy returns the current strategy
func (r *ServiceRegistry) GetStrategy() strategy.Strategy {
	return r.CurrentStrategy
}

// GetDatabase returns the database
func (r *ServiceRegistry) GetDatabase() Database {
	return r.Database
}

// GetAPIServer returns the API server
func (r *ServiceRegistry) GetAPIServer() *api.Server {
	return r.APIServer
}

// GetMarketDataService returns the market data service
func (r *ServiceRegistry) GetMarketDataService() MarketDataService {
	return r.MarketDataService
}

// GetMarketSpecService returns the market specification service
func (r *ServiceRegistry) GetMarketSpecService() MarketSpecService {
	return r.MarketSpecService
}

// GetBitflyerClient returns the bitFlyer client
func (r *ServiceRegistry) GetBitflyerClient() BitflyerClient {
	return r.BitflyerClient
}
