package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/infra/persistence"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	strategy "github.com/bmf-san/gogocoin/v1/pkg/strategy"
)

// mock structure for testing
type mockApplication struct {
	strategy       strategy.Strategy
	tradingEnabled bool
}

func (m *mockApplication) GetCurrentStrategy() strategy.Strategy {
	return m.strategy
}

func (m *mockApplication) GetBalances(ctx context.Context) ([]domain.Balance, error) {
	return []domain.Balance{
		{
			Currency:  "JPY",
			Amount:    1000000,
			Available: 1000000,
			Timestamp: time.Now(),
		},
	}, nil
}

func (m *mockApplication) GetTradingService() interface{} {
	return nil // return nil for testing
}

func (m *mockApplication) IsTradingEnabled() bool {
	return m.tradingEnabled
}

func (m *mockApplication) SetTradingEnabled(enabled bool) error {
	m.tradingEnabled = enabled
	return nil
}

type mockStrategy struct {
	name string
}

func (m *mockStrategy) Name() string                                                 { return m.name }
func (m *mockStrategy) Description() string                                          { return "Mock strategy" }
func (m *mockStrategy) Version() string                                              { return "1.0.0" }
func (m *mockStrategy) Start(ctx context.Context) error                              { return nil }
func (m *mockStrategy) Stop(ctx context.Context) error                               { return nil }
func (m *mockStrategy) Analyze(data []strategy.MarketData) (*strategy.Signal, error) { return nil, nil }
func (m *mockStrategy) GenerateSignal(ctx context.Context, data *strategy.MarketData, history []strategy.MarketData) (*strategy.Signal, error) {
	return nil, nil
}
func (m *mockStrategy) GetStatus() strategy.StrategyStatus               { return strategy.StrategyStatus{} }
func (m *mockStrategy) GetMetrics() strategy.StrategyMetrics             { return strategy.StrategyMetrics{} }
func (m *mockStrategy) GetConfig() map[string]interface{}                { return map[string]interface{}{} }
func (m *mockStrategy) Initialize(config map[string]interface{}) error   { return nil }
func (m *mockStrategy) UpdateConfig(config map[string]interface{}) error { return nil }
func (m *mockStrategy) IsRunning() bool                                  { return true }
func (m *mockStrategy) Reset() error                                     { return nil }
func (m *mockStrategy) RecordTrade()                                     {}
func (m *mockStrategy) InitializeDailyTradeCount(count int)              {}

func setupTestServer(t *testing.T) (*Server, *persistence.Repository, func()) {
	t.Helper()

	// Create test logger (console only, no file I/O)
	loggerConfig := &logger.Config{
		Level:  "error",
		Format: "json",
		Output: "console",
	}
	testLogger, err := logger.New(loggerConfig)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	// Use in-memory SQLite database (no file system I/O)
	rawDB, err := persistence.NewDB(":memory:", testLogger)
	if err != nil {
		_ = testLogger.Close()
		t.Fatalf("Failed to create test database: %v", err)
	}
	db := persistence.NewRepository(rawDB)

	// Create test config
	testConfig := &config.Config{
		App: config.AppConfig{
			Name: "test-gogocoin",
		},
		Trading: config.TradingConfig{
			InitialBalance: 100000,
			Strategy: config.StrategyConfig{
				Name: "simple_test",
			},
		},
	}

	server := NewServer(testConfig, db, testLogger)

	// Set mock application
	mockApp := &mockApplication{
		strategy: &mockStrategy{name: "simple_test"},
	}
	server.SetApplication(mockApp)

	// Return cleanup function
	cleanup := func() {
		if rawDB != nil {
			_ = rawDB.Close()
		}
		if testLogger != nil {
			_ = testLogger.Close()
		}
	}

	return server, db, cleanup
}

// testHandler creates a full HTTP handler via the generated router for use in tests.
func testHandler(s *Server) http.Handler {
	return HandlerWithOptions(NewStrictHandler(s, nil), StdHTTPServerOptions{})
}

func TestNewServer(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	if server == nil {
		t.Error("NewServer should not return nil")
	}
}

func TestHandleStatus(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response.Status == nil || *response.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestHandleStatus_InvalidMethod(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/status", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code 405, got %d", w.Code)
	}
}

func TestHandleBalance(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// save test balance data
	balance := domain.Balance{
		Currency:  "JPY",
		Amount:    1000000,
		Available: 950000,
		Timestamp: time.Now(),
	}
	if err := db.SaveBalance(balance); err != nil {
		t.Fatalf("Failed to save test balance: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/balance", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var balances []Balance
	if err := json.NewDecoder(w.Body).Decode(&balances); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(balances) == 0 {
		t.Error("Expected at least one balance")
	}
}

// TestHandlePositions is removed because positions endpoint was removed
// (spot trading simulation does not use positions)

func TestHandleTrades(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Save trading data for testing
	trade := &domain.Trade{
		Symbol:       "BTC_JPY",
		Side:         "BUY",
		Type:         "MARKET",
		Size:         0.001,
		Price:        4000000,
		Fee:          60,
		Status:       "COMPLETED",
		ExecutedAt:   time.Now(),
		CreatedAt:    time.Now(),
		StrategyName: "simple_test",
	}
	if err := db.SaveTrade(trade); err != nil {
		t.Fatalf("Failed to save test trade: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/trades", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var trades []Trade
	if err := json.NewDecoder(w.Body).Decode(&trades); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(trades) == 0 {
		t.Error("Expected at least one trade")
	}
}

func TestHandleTrades_WithLimit(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Save multiple trading data
	for i := 0; i < 5; i++ {
		trade := &domain.Trade{OrderID: fmt.Sprintf("api_test_order_%d", i), Symbol: "BTC_JPY",
			Side:         "BUY",
			Type:         "MARKET",
			Size:         0.001,
			Price:        4000000 + float64(i*1000),
			Fee:          60,
			Status:       "COMPLETED",
			ExecutedAt:   time.Now().Add(time.Duration(i) * time.Minute),
			CreatedAt:    time.Now(),
			StrategyName: "simple_test",
		}
		if err := db.SaveTrade(trade); err != nil {
			t.Fatalf("Failed to save test trade %d: %v", i, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/trades?limit=3", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var trades []Trade
	if err := json.NewDecoder(w.Body).Decode(&trades); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(trades) != 3 {
		t.Errorf("Expected 3 trades, got %d", len(trades))
	}
}

func TestHandlePerformance(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Save performance data for testing
	metric := &domain.PerformanceMetric{
		Date:          time.Now(),
		TotalReturn:   5.5,
		WinRate:       65.0,
		MaxDrawdown:   2.1,
		SharpeRatio:   1.2,
		ProfitFactor:  1.8,
		TotalTrades:   100,
		WinningTrades: 65,
		LosingTrades:  35,
		TotalPnL:      55000,
	}
	if err := db.SavePerformanceMetric(metric); err != nil {
		t.Fatalf("Failed to save test performance metric: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/performance", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var metrics []PerformanceMetric
	if err := json.NewDecoder(w.Body).Decode(&metrics); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected at least one performance metric")
	}
}

func TestHandleConfig_Get(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var cfg config.Config
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if cfg.App.Name != "test-gogocoin" {
		t.Errorf("Expected app name 'test-gogocoin', got '%s'", cfg.App.Name)
	}
}

func TestHandleConfig_Post(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Config update request using JSON directly (generated type uses pointer fields)
	reqBody := []byte(`{"strategy":{"name":"scalping"},"risk":{"stop_loss":2.5,"take_profit":5.0}}`)

	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", response["status"])
	}
}

func TestHandleStrategyReset(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", response["status"])
	}
}

func TestHandleStrategyReset_InvalidMethod(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code 405, got %d", w.Code)
	}
}

func TestHandleStrategyReset_NoApplication(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Do not set application
	server.SetApplication(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code 503, got %d", w.Code)
	}
}

func TestHandleLogs(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Save log data for testing
	logEntry := &domain.LogEntry{
		Level:     "INFO",
		Category:  "trading",
		Message:   "Test log message",
		Timestamp: time.Now(),
	}
	if err := db.SaveLog(logEntry); err != nil {
		t.Fatalf("Failed to save test log: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var logs []LogEntry
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(logs) == 0 {
		t.Error("Expected at least one log entry")
	}
}

func TestHandleLogs_WithLevelFilter(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Save logs with different levels
	levels := []string{"INFO", "ERROR", "WARN"}
	for _, level := range levels {
		logEntry := &domain.LogEntry{
			Level:     level,
			Category:  "test",
			Message:   "Test message for " + level,
			Timestamp: time.Now(),
		}
		if err := db.SaveLog(logEntry); err != nil {
			t.Fatalf("Failed to save test log: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/logs?level=ERROR", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var logs []LogEntry
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	// Verify that only ERROR level logs are returned
	for _, log := range logs {
		if log.Level == nil || *log.Level != "ERROR" {
			t.Errorf("Expected ERROR level log, got %v", log.Level)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	testData := map[string]interface{}{
		"message": "test",
		"value":   123,
	}

	server.writeJSON(w, testData)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
	}

	if response["message"] != "test" {
		t.Errorf("Expected message 'test', got '%v'", response["message"])
	}
}

func TestSetApplication(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	mockApp := &mockApplication{
		strategy: &mockStrategy{name: "test_strategy"},
	}

	server.SetApplication(mockApp)

	// Verify that application is set by calling a route that requires it
	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	testHandler(server).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200 after setting application, got %d", w.Code)
	}
}

