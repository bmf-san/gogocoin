package app

import (
	"testing"
)

func TestNewServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry()

	if registry == nil {
		t.Fatal("Expected non-nil service registry")
	}

	if registry.Database != nil {
		t.Error("Expected Database to be nil initially")
	}

	if registry.TradingService != nil {
		t.Error("Expected TradingService to be nil initially")
	}
}

func TestServiceRegistry_Validate(t *testing.T) {
	// Skip full validation tests as they require complete mock implementations
	// To properly test Validate(), we would need to implement all interfaces:
	// - Database, BitflyerClient, MarketDataService, MarketSpecService
	// - trading.Trader, risk.RiskManager, analytics.PerformanceAnalyticsService
	// - strategy.Strategy, api.Server
	// This is beyond the scope of a unit test for ServiceRegistry itself.
	t.Skip("Validation tests require full interface implementations")
}

func TestServiceRegistry_Getters(t *testing.T) {
	registry := NewServiceRegistry()

	// Test that getters return nil when not set
	if registry.GetDatabase() != nil {
		t.Error("GetDatabase() should return nil when not set")
	}

	if registry.GetTradingService() != nil {
		t.Error("GetTradingService() should return nil when not set")
	}

	if registry.GetRiskManager() != nil {
		t.Error("GetRiskManager() should return nil when not set")
	}

	if registry.GetPerformanceAnalytics() != nil {
		t.Error("GetPerformanceAnalytics() should return nil when not set")
	}

	if registry.GetStrategy() != nil {
		t.Error("GetStrategy() should return nil when not set")
	}

	if registry.GetAPIServer() != nil {
		t.Error("GetAPIServer() should return nil when not set")
	}
}
