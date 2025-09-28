package app

import (
	"context"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/analytics"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/risk"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// extractCurrency function moved to config.Validator package
// Test is now in internal/config/validator_test.go

// TestCalculatePerformanceFromTrades tests calculatePerformanceFromTrades method
func TestCalculatePerformanceFromTrades(t *testing.T) {
	tests := []struct {
		name                string
		initialBalance      float64
		trades              []domain.Trade
		expectedTotalReturn float64
		expectedWinRate     float64
		expectedTotalTrades int
	}{
		{
			name:                "No trades",
			initialBalance:      100000.0,
			trades:              []domain.Trade{},
			expectedTotalReturn: 0.0,
			expectedWinRate:     0.0,
			expectedTotalTrades: 0,
		},
		{
			name:           "Single winning trade",
			initialBalance: 100000.0,
			trades: []domain.Trade{
				{Side: "BUY", Price: 1000.0, Size: 1.0, Fee: 10.0, PnL: 100.0},
			},
			expectedTotalReturn: 0.1,
			expectedWinRate:     100.0,
			expectedTotalTrades: 1,
		},
		{
			name:           "Single losing trade",
			initialBalance: 100000.0,
			trades: []domain.Trade{
				{Side: "SELL", Price: 1000.0, Size: 1.0, Fee: 10.0, PnL: -100.0},
			},
			expectedTotalReturn: -0.1,
			expectedWinRate:     0.0,
			expectedTotalTrades: 1,
		},
		{
			name:           "Mixed trades",
			initialBalance: 100000.0,
			trades: []domain.Trade{
				{Side: "BUY", Price: 1000.0, Size: 1.0, Fee: 10.0, PnL: 100.0},
				{Side: "SELL", Price: 1100.0, Size: 1.0, Fee: 11.0, PnL: -50.0},
				{Side: "BUY", Price: 1050.0, Size: 1.0, Fee: 10.5, PnL: 200.0},
			},
			expectedTotalReturn: 0.25,
			expectedWinRate:     66.67,
			expectedTotalTrades: 3,
		},
		{
			name:           "Trade with zero PnL (BUY)",
			initialBalance: 100000.0,
			trades: []domain.Trade{
				{Side: "BUY", Price: 1000.0, Size: 1.0, Fee: 10.0, PnL: 0.0},
			},
			expectedTotalReturn: -0.01,
			expectedWinRate:     0.0,
			expectedTotalTrades: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := analytics.NewPerformanceAnalytics(nil, nil, nil, tt.initialBalance)
			result := pa.CalculateFromTrades(tt.trades)

			if len(tt.trades) == 0 {
				if result.TotalTrades != 0 {
					t.Errorf("TotalTrades = %d, want 0", result.TotalTrades)
				}
				return
			}

			tolerance := 0.02
			if abs(result.TotalReturn-tt.expectedTotalReturn) > tolerance {
				t.Errorf("TotalReturn = %f, want %f", result.TotalReturn, tt.expectedTotalReturn)
			}

			if abs(result.WinRate-tt.expectedWinRate) > tolerance {
				t.Errorf("WinRate = %f, want %f", result.WinRate, tt.expectedWinRate)
			}

			if result.TotalTrades != tt.expectedTotalTrades {
				t.Errorf("TotalTrades = %d, want %d", result.TotalTrades, tt.expectedTotalTrades)
			}
		})
	}
}

// TestCreateOrderFromSignal tests createOrderFromSignal method

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

type mockTrader struct {
	balances     []domain.Balance
	balanceError error
	orderResult  *domain.OrderResult
	orderError   error
}

func (m *mockTrader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	if m.orderError != nil {
		return nil, m.orderError
	}
	return m.orderResult, nil
}

func (m *mockTrader) CancelOrder(ctx context.Context, orderID string) error {
	return nil
}

func (m *mockTrader) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	if m.balanceError != nil {
		return nil, m.balanceError
	}
	return m.balances, nil
}

func (m *mockTrader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return nil, nil
}

func (m *mockTrader) SetStrategyName(name string) {}

func (m *mockTrader) SetOnOrderCompleted(fn func(*domain.OrderResult)) {}

func (m *mockTrader) InvalidateBalanceCache() {}

func (m *mockTrader) UpdateBalanceToDB(ctx context.Context) {}

func (m *mockTrader) Shutdown() error {
	return nil
}

// TestGetAvailableSellSize tests getAvailableSellSize method

// createTestLogger creates a logger for testing that outputs to /dev/null
func createTestLogger() (*logger.Logger, error) {
	cfg := &logger.Config{
		Level:    "error", // Only log errors
		Format:   "json",
		Output:   "file",
		FilePath: "/dev/null",
	}
	return logger.New(cfg)
}

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	// Create test logger
	loggerConfig := &logger.Config{
		Level:    "error",
		Format:   "json",
		Output:   "file",
		FilePath: "/dev/null",
	}
	testLogger, err := logger.New(loggerConfig)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	// Use in-memory SQLite database
	db, err := database.NewDB(":memory:", testLogger)
	if err != nil {
		_ = testLogger.Close()
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		if db != nil {
			_ = db.Close()
		}
		if testLogger != nil {
			_ = testLogger.Close()
		}
	}

	return db, cleanup
}

// TestCheckRiskManagement tests checkRiskManagement method with fee calculations
func TestCheckRiskManagement(t *testing.T) {
	tests := []struct {
		name           string
		signal         *strategy.Signal
		balances       []domain.Balance
		feeRate        float64
		maxTradeAmtPct float64
		recentTrades   []domain.Trade
		expectError    bool
		errorContains  string
	}{
		{
			name: "BUY signal - within limits (fee included)",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Symbol:   "BTC_JPY",
				Price:    1000000.0,
				Quantity: 0.01,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 100000.0, Amount: 100000.0},
			},
			feeRate:        0.0015, // 0.15%
			maxTradeAmtPct: 20.0,   // 20% of balance
			recentTrades:   []domain.Trade{},
			expectError:    false,
		},
		{
			name: "BUY signal - exceeds max trade amount (with fee)",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Symbol:   "BTC_JPY",
				Price:    1000000.0,
				Quantity: 0.05, // 50,000 + fee = 50,075 JPY
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 100000.0, Amount: 100000.0},
			},
			feeRate:        0.0015,
			maxTradeAmtPct: 20.0, // Max 20,000 JPY
			recentTrades:   []domain.Trade{},
			expectError:    true,
			errorContains:  "exceeds maximum",
		},
		{
			name: "SELL signal - within limits (no fee in trade amount)",
			signal: &strategy.Signal{
				Action:   strategy.SignalSell,
				Symbol:   "BTC_JPY",
				Price:    1000000.0,
				Quantity: 0.01,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 100000.0, Amount: 100000.0},
			},
			feeRate:        0.0015,
			maxTradeAmtPct: 20.0,
			recentTrades:   []domain.Trade{},
			expectError:    false,
		},
		{
			name: "Trade interval too short",
			signal: &strategy.Signal{
				Action:   strategy.SignalBuy,
				Symbol:   "BTC_JPY",
				Price:    1000000.0,
				Quantity: 0.001,
			},
			balances: []domain.Balance{
				{Currency: "JPY", Available: 100000.0, Amount: 100000.0},
			},
			feeRate:        0.0015,
			maxTradeAmtPct: 50.0,
			recentTrades: []domain.Trade{
				{
					Symbol:     "BTC_JPY",
					Side:       "BUY",
					Type:       "MARKET",
					Size:       0.001,
					Price:      1000000.0,
					Fee:        1.5,
					Status:     "COMPLETED",
					OrderID:    "test-order-1",
					ExecutedAt: time.Now().Add(-1 * time.Minute),
					CreatedAt:  time.Now().Add(-1 * time.Minute),
				},
			},
			expectError:   true,
			errorContains: "trade interval too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := createTestLogger()
			if err != nil {
				t.Fatalf("Failed to create test logger: %v", err)
			}
			defer func() {
				if err := logger.Close(); err != nil {
					t.Logf("Failed to close logger: %v", err)
				}
			}()

			// Setup test database
			db, cleanup := setupTestDB(t)
			defer cleanup()

			// Populate test data
			for _, trade := range tt.recentTrades {
				_ = db.SaveTrade(&trade)
			}

			cfg := &config.Config{
				Trading: config.TradingConfig{
					FeeRate: tt.feeRate,
					RiskManagement: config.RiskManagementConfig{
						MaxTradeAmountPercent: tt.maxTradeAmtPct,
						MinTradeInterval:      "5m",
						MaxTotalLossPercent:   50.0,
						MaxDailyTrades:        100, // Add daily trade limit
					},
				},
			}

			trader := &mockTrader{
				balances: tt.balances,
			}

			app := &Application{
				config:     cfg,
				logger:     logger,
				serviceRegistry: &ServiceRegistry{TradingService: trader,
				Database: db},
			}

			// Initialize RiskManager
			app.serviceRegistry.RiskManager = risk.NewRiskManager(
				&cfg.Trading.RiskManagement,
				&cfg.Trading,
				db,
				db,
				trader,
				logger,
			)

			err = app.serviceRegistry.RiskManager.CheckRiskManagement(context.Background(), tt.signal)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestCheckTradeInterval tests checkTradeInterval method
func TestCheckTradeInterval(t *testing.T) {
	tests := []struct {
		name         string
		recentTrades []domain.Trade
		minInterval  string
		expectError  bool
	}{
		{
			name:         "No recent trades - first trade allowed",
			recentTrades: []domain.Trade{},
			minInterval:  "5m",
			expectError:  false,
		},
		{
			name: "Sufficient interval passed",
			recentTrades: []domain.Trade{
				{
					Symbol:     "BTC_JPY",
					Side:       "BUY",
					Type:       "MARKET",
					Size:       0.001,
					Price:      1000000.0,
					Fee:        1.5,
					Status:     "COMPLETED",
					OrderID:    "test-order-2",
					ExecutedAt: time.Now().Add(-10 * time.Minute),
					CreatedAt:  time.Now().Add(-10 * time.Minute),
				},
			},
			minInterval: "5m",
			expectError: false,
		},
		{
			name: "Interval too short",
			recentTrades: []domain.Trade{
				{
					Symbol:     "BTC_JPY",
					Side:       "BUY",
					Type:       "MARKET",
					Size:       0.001,
					Price:      1000000.0,
					Fee:        1.5,
					Status:     "COMPLETED",
					OrderID:    "test-order-3",
					ExecutedAt: time.Now().Add(-2 * time.Minute),
					CreatedAt:  time.Now().Add(-2 * time.Minute),
				},
			},
			minInterval: "5m",
			expectError: true,
		},
		{
			name: "Invalid interval format - uses default 5m",
			recentTrades: []domain.Trade{
				{
					Symbol:     "BTC_JPY",
					Side:       "BUY",
					Type:       "MARKET",
					Size:       0.001,
					Price:      1000000.0,
					Fee:        1.5,
					Status:     "COMPLETED",
					OrderID:    "test-order-4",
					ExecutedAt: time.Now().Add(-3 * time.Minute),
					CreatedAt:  time.Now().Add(-3 * time.Minute),
				},
			},
			minInterval: "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := createTestLogger()
			if err != nil {
				t.Fatalf("Failed to create test logger: %v", err)
			}
			defer func() {
				if err := logger.Close(); err != nil {
					t.Logf("Failed to close logger: %v", err)
				}
			}()

			// Setup test database
			db, cleanup := setupTestDB(t)
			defer cleanup()

			// Populate test data
			for _, trade := range tt.recentTrades {
				_ = db.SaveTrade(&trade)
			}

			cfg := &config.Config{
				Trading: config.TradingConfig{
					RiskManagement: config.RiskManagementConfig{
						MinTradeInterval:      tt.minInterval,
						MaxTradeAmountPercent: 100.0, // High to pass trade amount check
						MaxDailyTrades:        1000,  // High to pass daily limit
					},
				},
			}

			// Use CheckRiskManagement which internally calls checkTradeInterval
			// We need to test with a minimal signal since interval check is part of full risk check
			signal := &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.0001, // Very small amount to pass trade amount check
			}

			// For interval-only testing, we use a mock trader with high balance
			mockTrader := &mockTrader{
				balances: []domain.Balance{{Currency: "JPY", Available: 100000000}}, // High balance
			}
			riskMgr := risk.NewRiskManager(
				&cfg.Trading.RiskManagement,
				&cfg.Trading,
				db,
				db,
				mockTrader,
				logger,
			)

			err = riskMgr.CheckRiskManagement(context.Background(), signal)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestCheckTotalLossLimit tests checkTotalLossLimit method
// Note: Full testing of loss limit enforcement requires tradingSvc.GetBalance() (external API)
// which is explicitly excluded from testing per user requirements.
// These tests only cover the basic early-exit scenarios.
func TestCheckTotalLossLimit(t *testing.T) {
	tests := []struct {
		name        string
		performance []domain.PerformanceMetric
		maxLossPct  float64
		expectError bool
	}{
		{
			name:        "No performance metrics - allowed",
			performance: []domain.PerformanceMetric{},
			maxLossPct:  50.0,
			expectError: false,
		},
		{
			name: "Positive PnL - early exit without API call",
			performance: []domain.PerformanceMetric{
				{
					Date:          time.Now(),
					TotalReturn:   20.0,
					DailyReturn:   2.0,
					WinRate:       65.0,
					MaxDrawdown:   -5.0,
					SharpeRatio:   1.5,
					TotalTrades:   100,
					WinningTrades: 65,
					LosingTrades:  35,
					TotalPnL:      20000.0, // Positive PnL - no loss check needed
				},
			},
			maxLossPct:  50.0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := createTestLogger()
			if err != nil {
				t.Fatalf("Failed to create test logger: %v", err)
			}
			defer func() {
				if err := logger.Close(); err != nil {
					t.Logf("Failed to close logger: %v", err)
				}
			}()

			// Setup test database
			db, cleanup := setupTestDB(t)
			defer cleanup()

			// Populate test data
			for _, metric := range tt.performance {
				_ = db.SavePerformanceMetric(&metric)
			}

			cfg := &config.Config{
				Trading: config.TradingConfig{
					InitialBalance: 100000.0,
					RiskManagement: config.RiskManagementConfig{
						MaxTotalLossPercent:   tt.maxLossPct,
						MaxTradeAmountPercent: 100.0, // High to pass trade amount check
						MaxDailyTrades:        1000,  // High to pass daily limit
						MinTradeInterval:      "0s",  // No interval check
					},
				},
			}

			signal := &strategy.Signal{
				Action:   strategy.SignalBuy,
				Price:    1000000,
				Quantity: 0.0001,
			}

			mockTrader := &mockTrader{
				balances: []domain.Balance{{Currency: "JPY", Available: 100000000}}, // High balance
			}

			// Initialize RiskManager
			riskMgr := risk.NewRiskManager(
				&cfg.Trading.RiskManagement,
				&cfg.Trading,
				db,
				db,
				mockTrader,
				logger,
			)

			err = riskMgr.CheckRiskManagement(context.Background(), signal)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
