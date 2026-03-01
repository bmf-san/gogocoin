package risk

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/usecase/strategy"
)

// Mock implementations
type mockTradingRepo struct {
	trades []domain.Trade
}

func (m *mockTradingRepo) GetRecentTrades(limit int) ([]domain.Trade, error) {
	if limit > len(m.trades) {
		return m.trades, nil
	}
	return m.trades[:limit], nil
}

type mockAnalyticsRepo struct {
	metrics []domain.PerformanceMetric
}

func (m *mockAnalyticsRepo) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {
	return m.metrics, nil
}

type mockTrader struct {
	balances []domain.Balance
}

func (m *mockTrader) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	return m.balances, nil
}

func (m *mockTrader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	return nil, nil
}

func (m *mockTrader) CancelOrder(ctx context.Context, orderID string) error {
	return nil
}

func (m *mockTrader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return nil, nil
}

func (m *mockTrader) SetDatabase(db domain.TradingRepository)            {}
func (m *mockTrader) SetMarketSpecService(svc domain.MarketSpecService) {}
func (m *mockTrader) SetStrategyName(name string)                        {}
func (m *mockTrader) SetOnOrderCompleted(fn func(*domain.OrderResult))   {}
func (m *mockTrader) InvalidateBalanceCache()                            {}
func (m *mockTrader) UpdateBalanceToDB(ctx context.Context)              {}
func (m *mockTrader) Shutdown() error                                    { return nil }

func TestCheckTradeAmount(t *testing.T) {
	cfg := ManagerConfig{
		MaxTradeAmountPercent: 10.0,  // 10% of balance
		FeeRate:               0.001, // 0.1%
	}

	rm := NewRiskManager(cfg, nil, nil, nil, nil)

	tests := []struct {
		name        string
		signal      *strategy.Signal
		balance     float64
		expectError bool
	}{
		{
			name: "Valid trade amount",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.0009, // 900 JPY, 10% of 10000 = 1000
			},
			balance:     10000,
			expectError: false,
		},
		{
			name: "Trade amount too large",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.002, // 2000 JPY + fee > 1000 JPY limit
			},
			balance:     10000,
			expectError: true,
		},
		{
			name: "Valid SELL order",
			signal: &strategy.Signal{
				Action:   strategy.SignalSell,
				Price:    1000000,
				Quantity: 0.0009, // 900 JPY, no fee check for SELL
			},
			balance:     10000,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.checkTradeAmount(tt.signal, tt.balance)
			if (err != nil) != tt.expectError {
				t.Errorf("checkTradeAmount() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckDailyTradeLimit(t *testing.T) {
	cfg := ManagerConfig{
		MaxDailyTrades: 5,
	}

	now := time.Now()

	tests := []struct {
		name        string
		trades      []domain.Trade
		expectError bool
	}{
		{
			name: "Under daily limit",
			trades: []domain.Trade{
				{CreatedAt: now.Add(-1 * time.Hour)},
				{CreatedAt: now.Add(-2 * time.Hour)},
			},
			expectError: false,
		},
		{
			name: "At daily limit",
			trades: []domain.Trade{
				{CreatedAt: now.Add(-1 * time.Hour)},
				{CreatedAt: now.Add(-2 * time.Hour)},
				{CreatedAt: now.Add(-3 * time.Hour)},
				{CreatedAt: now.Add(-4 * time.Hour)},
				{CreatedAt: now.Add(-5 * time.Hour)},
			},
			expectError: true,
		},
		{
			name: "No trades today",
			trades: []domain.Trade{
				{CreatedAt: now.Add(-25 * time.Hour)},
				{CreatedAt: now.Add(-26 * time.Hour)},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradingRepo := &mockTradingRepo{trades: tt.trades}
			rm := NewRiskManager(cfg, tradingRepo, nil, nil, nil)

			err := rm.checkDailyTradeLimit()
			if (err != nil) != tt.expectError {
				t.Errorf("checkDailyTradeLimit() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckTradeInterval(t *testing.T) {
	cfg := ManagerConfig{
		MinTradeInterval: 5 * time.Minute,
	}

	now := time.Now()

	tests := []struct {
		name        string
		trades      []domain.Trade
		expectError bool
	}{
		{
			name:        "No previous trades",
			trades:      []domain.Trade{},
			expectError: false,
		},
		{
			name: "Sufficient interval",
			trades: []domain.Trade{
				{ExecutedAt: now.Add(-10 * time.Minute)},
			},
			expectError: false,
		},
		{
			name: "Insufficient interval",
			trades: []domain.Trade{
				{ExecutedAt: now.Add(-2 * time.Minute)},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradingRepo := &mockTradingRepo{trades: tt.trades}
			rm := NewRiskManager(cfg, tradingRepo, nil, nil, nil)

			err := rm.checkTradeInterval()
			if (err != nil) != tt.expectError {
				t.Errorf("checkTradeInterval() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckTotalLossLimit(t *testing.T) {
	cfg := ManagerConfig{
		MaxTotalLossPercent: 10.0, // 10% max loss
		InitialBalance:      100000,
	}

	tests := []struct {
		name        string
		metrics     []domain.PerformanceMetric
		balances    []domain.Balance
		expectError bool
	}{
		{
			name:        "No metrics",
			metrics:     []domain.PerformanceMetric{},
			balances:    []domain.Balance{{Currency: "JPY", Available: 100000}},
			expectError: false,
		},
		{
			name: "Within loss limit",
			metrics: []domain.PerformanceMetric{
				{TotalPnL: -5000}, // 5% loss
			},
			balances:    []domain.Balance{{Currency: "JPY", Available: 95000}},
			expectError: false,
		},
		{
			name: "Exceeded loss limit",
			metrics: []domain.PerformanceMetric{
				{TotalPnL: -15000}, // 15% loss
			},
			balances:    []domain.Balance{{Currency: "JPY", Available: 85000}},
			expectError: true,
		},
		{
			name: "Positive PnL",
			metrics: []domain.PerformanceMetric{
				{TotalPnL: 5000}, // Profit
			},
			balances:    []domain.Balance{{Currency: "JPY", Available: 105000}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyticsRepo := &mockAnalyticsRepo{metrics: tt.metrics}
			trader := &mockTrader{balances: tt.balances}
			rm := NewRiskManager(cfg, nil, analyticsRepo, trader, nil)

			err := rm.checkTotalLossLimit(context.Background())
			if (err != nil) != tt.expectError {
				t.Errorf("checkTotalLossLimit() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCheckRiskManagement(t *testing.T) {
	cfg := ManagerConfig{
		MaxTradeAmountPercent: 10.0,
		MaxDailyTrades:        5,
		MinTradeInterval:      5 * time.Minute,
		MaxTotalLossPercent:   10.0,
		FeeRate:               0.001,
		InitialBalance:        100000,
	}

	now := time.Now()

	tests := []struct {
		name        string
		signal      *strategy.Signal
		balances    []domain.Balance
		trades      []domain.Trade
		metrics     []domain.PerformanceMetric
		expectError bool
	}{
		{
			name: "All checks pass",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.0009,
			},
			balances: []domain.Balance{{Currency: "JPY", Available: 100000}},
			trades: []domain.Trade{
				{CreatedAt: now.Add(-10 * time.Minute), ExecutedAt: now.Add(-10 * time.Minute)},
			},
			metrics:     []domain.PerformanceMetric{{TotalPnL: 0}},
			expectError: false,
		},
		{
			name: "Trade amount too large",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.02, // Too large
			},
			balances:    []domain.Balance{{Currency: "JPY", Available: 100000}},
			trades:      []domain.Trade{},
			metrics:     []domain.PerformanceMetric{},
			expectError: true,
		},
		{
			name: "Daily trade limit exceeded",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.0009,
			},
			balances: []domain.Balance{{Currency: "JPY", Available: 100000}},
			trades: []domain.Trade{
				{CreatedAt: now.Add(-1 * time.Hour), ExecutedAt: now.Add(-1 * time.Hour)},
				{CreatedAt: now.Add(-2 * time.Hour), ExecutedAt: now.Add(-2 * time.Hour)},
				{CreatedAt: now.Add(-3 * time.Hour), ExecutedAt: now.Add(-3 * time.Hour)},
				{CreatedAt: now.Add(-4 * time.Hour), ExecutedAt: now.Add(-4 * time.Hour)},
				{CreatedAt: now.Add(-5 * time.Hour), ExecutedAt: now.Add(-5 * time.Hour)},
			},
			metrics:     []domain.PerformanceMetric{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradingRepo := &mockTradingRepo{trades: tt.trades}
			analyticsRepo := &mockAnalyticsRepo{metrics: tt.metrics}
			trader := &mockTrader{balances: tt.balances}
			rm := NewRiskManager(cfg, tradingRepo, analyticsRepo, trader, nil)

			err := rm.CheckRiskManagement(context.Background(), tt.signal)
			if (err != nil) != tt.expectError {
				t.Errorf("CheckRiskManagement() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}
