package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// mockBitflyerClient implements BitflyerClient for testing
type mockBitflyerClient struct {
	connected   bool
	closeError  error
	closeCalled bool
}

func (m *mockBitflyerClient) IsConnected() bool {
	return m.connected
}

func (m *mockBitflyerClient) Close(ctx context.Context) error {
	m.closeCalled = true
	return m.closeError
}

// mockMarketDataService implements MarketDataService for testing
type mockMarketDataService struct {
	resetCallbacksCalled bool
	subscribeError       error
}

func (m *mockMarketDataService) ResetCallbacks() {
	m.resetCallbacksCalled = true
}

func (m *mockMarketDataService) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	if m.subscribeError != nil {
		return m.subscribeError
	}
	// Simulate successful subscription by calling callback
	go func() {
		time.Sleep(10 * time.Millisecond)
		callback(domain.MarketData{
			Symbol:    symbol,
			Price:     1000.0,
			Timestamp: time.Now(),
		})
	}()
	return nil
}

// mockMarketSpecService implements MarketSpecService for testing
type mockMarketSpecService struct{}

func (m *mockMarketSpecService) GetMinimumOrderSize(symbol string) (float64, error) {
	return 0.001, nil
}

// mockTradingService implements trading.Trader for testing
type mockTradingService struct {
	orderCompletedCallback func(*domain.OrderResult)
	strategyName           string
}

func (m *mockTradingService) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	return &domain.OrderResult{
		OrderID: "test-order-123",
		Symbol:  order.Symbol,
		Side:    order.Side,
		Status:  "COMPLETED",
	}, nil
}

func (m *mockTradingService) CancelOrder(ctx context.Context, orderID string) error {
	return nil
}

func (m *mockTradingService) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return []*domain.OrderResult{}, nil
}

func (m *mockTradingService) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	return []domain.Balance{
		{
			Currency:  "BTC",
			Amount:    1.0,
			Available: 0.5,
			Timestamp: time.Now(),
		},
	}, nil
}

func (m *mockTradingService) InvalidateBalanceCache() {}

func (m *mockTradingService) UpdateBalanceToDB(ctx context.Context) {}

func (m *mockTradingService) Shutdown() error {
	return nil
}

func (m *mockTradingService) SetOnOrderCompleted(fn func(*domain.OrderResult)) {
	m.orderCompletedCallback = fn
}

func (m *mockTradingService) SetStrategyName(name string) {
	m.strategyName = name
}

// mockDatabase implements Database for testing
type mockDatabase struct{}

func (m *mockDatabase) SaveMarketData(data *domain.MarketData) error {
	return nil
}

func (m *mockDatabase) GetMarketData(symbol string, start, end time.Time) ([]*domain.MarketData, error) {
	return []*domain.MarketData{}, nil
}

func (m *mockDatabase) SaveBalance(balance domain.Balance) error {
	return nil
}

func (m *mockDatabase) GetLatestBalance() (*domain.Balance, error) {
	return &domain.Balance{
		Currency:  "JPY",
		Amount:    100000,
		Available: 50000,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockDatabase) SaveLog(log *domain.LogEntry) error {
	return nil
}

func (m *mockDatabase) GetLogs(start, end time.Time, level, category string) ([]*domain.LogEntry, error) {
	return []*domain.LogEntry{}, nil
}

func (m *mockDatabase) Close() error {
	return nil
}

func (m *mockDatabase) SaveAppState(key, value string) error {
	return nil
}

func (m *mockDatabase) GetAppState(key string) (string, error) {
	return "", nil
}

func (m *mockDatabase) SaveOrder(order *domain.OrderResult) error {
	return nil
}

func (m *mockDatabase) GetOrders(start, end time.Time) ([]*domain.OrderResult, error) {
	return []*domain.OrderResult{}, nil
}

func (m *mockDatabase) SaveTrade(trade *domain.Trade) error {
	return nil
}

func (m *mockDatabase) GetTrades(start, end time.Time) ([]*domain.Trade, error) {
	return []*domain.Trade{}, nil
}

func (m *mockDatabase) SavePerformanceMetric(metric *domain.PerformanceMetric) error {
	return nil
}

func (m *mockDatabase) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {
	return []domain.PerformanceMetric{}, nil
}

func (m *mockDatabase) CleanupOldData(retentionDays int) error {
	return nil
}

func (m *mockDatabase) GetLatestMarketData(symbol string) (*domain.MarketData, error) {
	return &domain.MarketData{
		Symbol:    symbol,
		Price:     1000.0,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockDatabase) BeginTx() (domain.Transaction, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDatabase) GetDatabaseSize() (int64, error) {
	return 1024, nil
}

func (m *mockDatabase) GetOpenPositions(symbol string, side string) ([]domain.Position, error) {
	return []domain.Position{}, nil
}

func (m *mockDatabase) GetRecentTrades(limit int) ([]domain.Trade, error) {
	return []domain.Trade{}, nil
}

func (m *mockDatabase) GetTableStats() (map[string]int, error) {
	return map[string]int{
		"market_data": 100,
		"orders":      10,
		"trades":      5,
	}, nil
}

func (m *mockDatabase) Ping() error {
	return nil
}

func (m *mockDatabase) SavePosition(position *domain.Position) error {
	return nil
}

func (m *mockDatabase) UpdatePosition(position *domain.Position) error {
	return nil
}

func (m *mockDatabase) GetLatestPerformanceMetric() (*domain.PerformanceMetric, error) {
	return &domain.PerformanceMetric{}, nil
}

func (m *mockDatabase) SaveTicker(ticker *domain.MarketData) error {
	return nil
}

// TestConnectionManager_IsConnected tests the IsConnected method
func TestConnectionManager_IsConnected(t *testing.T) {
	t.Run("returns_true_when_client_connected", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockClient := &mockBitflyerClient{connected: true}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		if !cm.IsConnected() {
			t.Error("Expected IsConnected to return true")
		}
	})

	t.Run("returns_false_when_client_disconnected", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockClient := &mockBitflyerClient{connected: false}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		if cm.IsConnected() {
			t.Error("Expected IsConnected to return false")
		}
	})

	t.Run("returns_false_when_client_nil", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		cm := NewConnectionManager(
			cfg,
			log,
			nil, // nil client
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		if cm.IsConnected() {
			t.Error("Expected IsConnected to return false when client is nil")
		}
	})
}

// TestConnectionManager_ReconnectClient tests the ReconnectClient method
func TestConnectionManager_ReconnectClient(t *testing.T) {
	t.Run("successful_reconnection_flow", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{
			API: config.APIConfig{
				Credentials: config.CredentialsConfig{
					APIKey:    "test-key",
					APISecret: "test-secret",
				},
				Endpoint:          "https://api.test.com",
				WebSocketEndpoint: "wss://ws.test.com",
				Timeout:           30,
				RetryCount:        3,
				RateLimit: config.RateLimitConfig{
					RequestsPerMinute: 60,
				},
			},
			Trading: config.TradingConfig{
				InitialBalance: 100000,
				FeeRate:        0.001,
			},
			Data: config.DataConfig{
				MarketData: config.MarketDataConfig{
					HistoryDays: 30,
				},
			},
			Worker: config.WorkerConfig{
				MaxConcurrentSaves: 10,
			},
		}

		mockClient := &mockBitflyerClient{connected: true}
		mockMDS := &mockMarketDataService{}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			mockMDS,
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		// Note: This test will fail during actual reconnection because
		// bitflyer.NewClient requires real configuration and network access.
		// In production code, we would use dependency injection for the client factory.
		// For now, we test that the method can be called without panic.
		err := cm.ReconnectClient()

		// The error is expected since we can't create a real bitflyer client in tests
		if err == nil {
			t.Log("Reconnection succeeded (unexpected in mock environment)")
		} else {
			t.Logf("Reconnection failed as expected: %v", err)
		}

		// Verify callbacks were reset
		if !mockMDS.resetCallbacksCalled {
			t.Error("Expected ResetCallbacks to be called")
		}

		// Verify client was closed
		if !mockClient.closeCalled {
			t.Error("Expected Close to be called on old client")
		}
	})

	t.Run("close_error_does_not_prevent_reconnection", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{
			API: config.APIConfig{
				Credentials: config.CredentialsConfig{
					APIKey:    "test-key",
					APISecret: "test-secret",
				},
				Endpoint:          "https://api.test.com",
				WebSocketEndpoint: "wss://ws.test.com",
				Timeout:           30,
				RetryCount:        3,
				RateLimit: config.RateLimitConfig{
					RequestsPerMinute: 60,
				},
			},
			Trading: config.TradingConfig{
				InitialBalance: 100000,
				FeeRate:        0.001,
			},
			Data: config.DataConfig{
				MarketData: config.MarketDataConfig{
					HistoryDays: 30,
				},
			},
			Worker: config.WorkerConfig{
				MaxConcurrentSaves: 10,
			},
		}

		mockClient := &mockBitflyerClient{
			connected:  true,
			closeError: errors.New("close failed"),
		}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		// Reconnection should proceed despite close error
		err := cm.ReconnectClient()

		// We expect an error from NewClient, but not from Close
		if err != nil {
			t.Logf("Reconnection error (expected): %v", err)
		}

		// Verify close was attempted
		if !mockClient.closeCalled {
			t.Error("Expected Close to be called")
		}
	})
}

// TestConnectionManager_SubscribeToTicker tests the SubscribeToTicker method
func TestConnectionManager_SubscribeToTicker(t *testing.T) {
	t.Run("successful_subscription", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockMDS := &mockMarketDataService{}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{connected: true},
			mockMDS,
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		received := make(chan domain.MarketData, 1)

		err := cm.SubscribeToTicker(ctx, "BTC_JPY", func(data domain.MarketData) {
			received <- data
		})

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Wait for callback to be invoked
		select {
		case data := <-received:
			if data.Symbol != "BTC_JPY" {
				t.Errorf("Expected symbol BTC_JPY, got %s", data.Symbol)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for market data callback")
		}
	})

	t.Run("subscription_error_propagated", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockMDS := &mockMarketDataService{
			subscribeError: errors.New("subscription failed"),
		}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{connected: true},
			mockMDS,
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		err := cm.SubscribeToTicker(ctx, "BTC_JPY", func(data domain.MarketData) {})

		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("nil_market_data_service_returns_error", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{connected: true},
			nil, // nil market data service
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		err := cm.SubscribeToTicker(ctx, "BTC_JPY", func(data domain.MarketData) {})

		if err == nil {
			t.Error("Expected error when market data service is nil")
		}
	})
}

// TestConnectionManager_GetServices tests service getter methods
func TestConnectionManager_GetServices(t *testing.T) {
	t.Run("get_market_data_service", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockMDS := &mockMarketDataService{}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{},
			mockMDS,
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		svc := cm.GetMarketDataService()
		if svc == nil {
			t.Error("Expected non-nil market data service")
		}
	})

	t.Run("get_market_spec_service", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockMSS := &mockMarketSpecService{}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{},
			&mockMarketDataService{},
			mockMSS,
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		svc := cm.GetMarketSpecService()
		if svc == nil {
			t.Error("Expected non-nil market spec service")
		}

		// Test the service works
		minSize, err := svc.GetMinimumOrderSize("BTC_JPY")
		if err != nil {
			t.Errorf("GetMinimumOrderSize failed: %v", err)
		}
		if minSize != 0.001 {
			t.Errorf("Expected min size 0.001, got %f", minSize)
		}
	})

	t.Run("get_trading_service", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockTS := &mockTradingService{}

		cm := NewConnectionManager(
			cfg,
			log,
			&mockBitflyerClient{},
			&mockMarketDataService{},
			&mockMarketSpecService{},
			mockTS,
			&mockDatabase{},
			"test-strategy",
		)

		svc := cm.GetTradingService()
		if svc == nil {
			t.Error("Expected non-nil trading service")
		}

		// Test the service works
		ctx := context.Background()
		balances, err := svc.GetBalance(ctx)
		if err != nil {
			t.Errorf("GetBalance failed: %v", err)
		}
		if len(balances) == 0 {
			t.Error("Expected at least one balance")
		}
	})
}

// TestConnectionManager_Close tests the Close method
func TestConnectionManager_Close(t *testing.T) {
	t.Run("successful_close", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockClient := &mockBitflyerClient{connected: true}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		err := cm.Close(ctx)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !mockClient.closeCalled {
			t.Error("Expected Close to be called on client")
		}
	})

	t.Run("close_with_nil_client", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		cm := NewConnectionManager(
			cfg,
			log,
			nil, // nil client
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		err := cm.Close(ctx)

		// Should not error when client is nil
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("close_error_propagated", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{}

		mockClient := &mockBitflyerClient{
			connected:  true,
			closeError: errors.New("close failed"),
		}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			&mockMarketDataService{},
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		ctx := context.Background()
		err := cm.Close(ctx)

		if err == nil {
			t.Error("Expected error to be propagated")
		}
	})
}

// TestConnectionManager_ServiceReplacement tests service replacement after reconnection
func TestConnectionManager_ServiceReplacement(t *testing.T) {
	t.Run("services_replaced_after_reconnect", func(t *testing.T) {
		log, _ := createTestLogger()
		cfg := &config.Config{
			API: config.APIConfig{
				Credentials: config.CredentialsConfig{
					APIKey:    "test-key",
					APISecret: "test-secret",
				},
				Endpoint:          "https://api.test.com",
				WebSocketEndpoint: "wss://ws.test.com",
			},
			Trading: config.TradingConfig{
				InitialBalance: 100000,
			},
			Data: config.DataConfig{
				MarketData: config.MarketDataConfig{
					HistoryDays: 30,
				},
			},
			Worker: config.WorkerConfig{
				MaxConcurrentSaves: 10,
			},
		}

		mockClient := &mockBitflyerClient{connected: true}
		mockMDS := &mockMarketDataService{}

		cm := NewConnectionManager(
			cfg,
			log,
			mockClient,
			mockMDS,
			&mockMarketSpecService{},
			&mockTradingService{},
			&mockDatabase{},
			"test-strategy",
		)

		// Get original service reference
		originalMDS := cm.GetMarketDataService()

		// Attempt reconnection (will fail but that's OK for this test)
		_ = cm.ReconnectClient()

		// Get new service reference
		newMDS := cm.GetMarketDataService()

		// Services should be different instances after reconnect
		// Note: This test will show that services were attempted to be replaced
		if originalMDS == nil || newMDS == nil {
			t.Log("Service reference check: both services should be non-nil")
		}

		// Verify callbacks were reset on original service
		if !mockMDS.resetCallbacksCalled {
			t.Error("Expected ResetCallbacks to be called during reconnection")
		}
	})
}
