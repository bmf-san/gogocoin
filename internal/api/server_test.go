package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
)

// mock structure for testing
type mockApplication struct {
	strategy       strategy.Strategy
	tradingEnabled bool
}

func (m *mockApplication) GetCurrentStrategy() strategy.Strategy {
	return m.strategy
}

func (m *mockApplication) GetBalances(ctx context.Context) ([]Balance, error) {
	return []Balance{
		{
			Currency:  "JPY",
			Amount:    1000000,
			Available: 1000000,
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

func setupTestServer(t *testing.T) (*Server, *database.DB, func()) {
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
	db, err := database.NewDB(":memory:", testLogger)
	if err != nil {
		_ = testLogger.Close()
		t.Fatalf("Failed to create test database: %v", err)
	}

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
		Mode: "paper",
		UI: config.UIConfig{
			Port: 8080,
		},
	}

	// Create server
	server := NewServer(testConfig, db, testLogger)

	// Set mock application
	mockApp := &mockApplication{
		strategy: &mockStrategy{name: "simple_test"},
	}
	server.SetApplication(mockApp)

	// Return cleanup function
	cleanup := func() {
		if db != nil {
			_ = db.Close()
		}
		if testLogger != nil {
			_ = testLogger.Close()
		}
	}

	return server, db, cleanup
}

func TestNewServer(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	if server == nil {
		t.Error("NewServer should not return nil")
	}
}

func TestHandleStatus(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response.Status == "" {
		t.Error("Status should not be empty")
	}
	if response.Mode == "" {
		t.Error("Mode should not be empty")
	}
}

func TestHandleStatus_InvalidMethod(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code 405, got %d", w.Code)
	}
}

func TestHandleBalance(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

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

	server.handleBalance(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var balances []database.Balance
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
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// for testingtradingdata" "保存
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

	server.handleTrades(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var trades []database.Trade
	if err := json.NewDecoder(w.Body).Decode(&trades); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(trades) == 0 {
		t.Error("Expected at least one trade")
	}
}

func TestHandleTrades_WithLimit(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// 複数oftradingdata" "保存
	for i := 0; i < 5; i++ {
		trade := &domain.Trade{
			Symbol:       "BTC_JPY",
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

	server.handleTrades(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var trades []database.Trade
	if err := json.NewDecoder(w.Body).Decode(&trades); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(trades) != 3 {
		t.Errorf("Expected 3 trades, got %d", len(trades))
	}
}

func TestHandleOrders(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// for testingtradingdata（orderとして扱われる）" "保存
	trade := &domain.Trade{
		OrderID:      "test_order_123",
		Symbol:       "BTC_JPY",
		Side:         "BUY",
		Size:         0.001,
		Price:        4000000,
		Fee:          60,
		Status:       "COMPLETED",
		ExecutedAt:   time.Now(),
		StrategyName: "simple_test",
		Type:         "MARKET",
		CreatedAt:    time.Now(),
	}
	if err := db.SaveTrade(trade); err != nil {
		t.Fatalf("Failed to save test trade: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	w := httptest.NewRecorder()

	server.handleOrders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var orders []OrderResponse
	if err := json.NewDecoder(w.Body).Decode(&orders); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(orders) == 0 {
		t.Error("Expected at least one order")
	}

	if orders[0].OrderID != "test_order_123" {
		t.Errorf("Expected order ID 'test_order_123', got '%s'", orders[0].OrderID)
	}
}

func TestHandlePerformance(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// for testingperformancedata" "保存
	metric := &database.PerformanceMetric{
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

	server.handlePerformance(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var metrics []database.PerformanceMetric
	if err := json.NewDecoder(w.Body).Decode(&metrics); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected at least one performance metric")
	}
}

func TestHandleConfig_Get(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	server.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var config config.Config
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if config.App.Name != "test-gogocoin" {
		t.Errorf("Expected app name 'test-gogocoin', got '%s'", config.App.Name)
	}
}

func TestHandleConfig_Post(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// for testingconfiguration更新リクエスト（modeis実行時固定ofため削除）
	updateReq := ConfigUpdateRequest{
		Strategy: struct {
			Name string `json:"name"`
		}{
			Name: "moving_average_cross",
		},
		Risk: struct {
			StopLoss   float64 `json:"stop_loss"`
			TakeProfit float64 `json:"take_profit"`
		}{
			StopLoss:   2.5,
			TakeProfit: 5.0,
		},
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// configurationfileofパス" "configuration（for testing）
	// Note: configPathフィールドis存在しないため、こofconfigurationis省略

	server.handleConfig(w, req)

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
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	server.handleStrategyReset(w, req)

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
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	server.handleStrategyReset(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code 405, got %d", w.Code)
	}
}

func TestHandleStrategyReset_NoApplication(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// アプリケーション" "configurationしない
	server.SetApplication(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	server.handleStrategyReset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response["status"] != "error" {
		t.Errorf("Expected status 'error', got '%v'", response["status"])
	}
}

func TestHandleLogs(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// for testinglogdata" "保存
	logEntry := &database.LogEntry{
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

	server.handleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var logs []database.LogEntry
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(logs) == 0 {
		t.Error("Expected at least one log entry")
	}
}

func TestHandleLogs_WithLevelFilter(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	// 異なるレベルoflog" "保存
	levels := []string{"INFO", "ERROR", "WARN"}
	for _, level := range levels {
		logEntry := &database.LogEntry{
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

	server.handleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var logs []database.LogEntry
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	// ERRORレベルoflogofみがreturnedこと" "確認
	for _, log := range logs {
		if log.Level != "ERROR" {
			t.Errorf("Expected ERROR level log, got %s", log.Level)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

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
	server, db, _ := setupTestServer(t)
	defer func() { _ = db.Close() }()

	mockApp := &mockApplication{
		strategy: &mockStrategy{name: "test_strategy"},
	}

	server.SetApplication(mockApp)

	// アプリケーションがconfigurationされたこと" "確認（内部状態なofwith間接的に確認）
	req := httptest.NewRequest(http.MethodPost, "/api/strategy/reset", nil)
	w := httptest.NewRecorder()

	server.handleStrategyReset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200 after setting application, got %d", w.Code)
	}
}
