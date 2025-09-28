package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

func setupTestDB(t *testing.T) (*DB, func()) {
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
	db, err := NewDB(":memory:", testLogger)
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

func TestNewDB(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify database is functional
	if db == nil {
		t.Fatal("Database instance should not be nil")
	}

	// Verify we can query the database
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query database: %v", err)
	}
	if count == 0 {
		t.Error("Expected at least one table to be created")
	}
}

func TestNewDB_InvalidPath(t *testing.T) {
	t.Parallel()

	loggerConfig := &logger.Config{
		Level:  "error",
		Format: "json",
		Output: "console",
	}
	testLogger, err := logger.New(loggerConfig)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer func() { _ = testLogger.Close() }()

	// Invalid path (directory does not exist)
	invalidPath := "/nonexistent/directory/test.db"
	_, err = NewDB(invalidPath, testLogger)
	if err == nil {
		t.Error("Expected error for invalid database path, got nil")
	}
}

func TestSaveTrade(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

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
		OrderID:      "test_order_123",
	}

	err := db.SaveTrade(trade)
	if err != nil {
		t.Errorf("Failed to save trade: %v", err)
	}

	// retrieve and verify saved trades
	trades, err := db.GetRecentTrades(1)
	if err != nil {
		t.Errorf("Failed to get recent trades: %v", err)
	}

	if len(trades) != 1 {
		t.Errorf("Expected 1 trade, got %d", len(trades))
	}

	savedTrade := trades[0]
	if savedTrade.Symbol != trade.Symbol {
		t.Errorf("Expected symbol %s, got %s", trade.Symbol, savedTrade.Symbol)
	}
	if savedTrade.Side != trade.Side {
		t.Errorf("Expected side %s, got %s", trade.Side, savedTrade.Side)
	}
	if savedTrade.Size != trade.Size {
		t.Errorf("Expected size %f, got %f", trade.Size, savedTrade.Size)
	}
}

func TestSavePosition(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	position := &domain.Position{
		Symbol:       "BTC_JPY",
		Side:         "BUY",
		Size:         0.001,
		EntryPrice:   4000000,
		CurrentPrice: 4001500,
		UnrealizedPL: 1500,
		Status:       "OPEN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := db.SavePosition(position)
	if err != nil {
		t.Errorf("Failed to save position: %v", err)
	}

	// retrieve and verify saved positions
	positions, err := db.GetActivePositions()
	if err != nil {
		t.Errorf("Failed to get active positions: %v", err)
	}

	if len(positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(positions))
	}

	savedPosition := positions[0]
	if savedPosition.Symbol != position.Symbol {
		t.Errorf("Expected symbol %s, got %s", position.Symbol, savedPosition.Symbol)
	}
	if savedPosition.Side != position.Side {
		t.Errorf("Expected side %s, got %s", position.Side, savedPosition.Side)
	}
}

func TestSaveBalance(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	balance := domain.Balance{
		Currency:  "JPY",
		Amount:    1000000,
		Available: 950000,
		Timestamp: time.Now(),
	}

	err := db.SaveBalance(balance)
	if err != nil {
		t.Errorf("Failed to save balance: %v", err)
	}

	// retrieve and verify saved balances
	balances, err := db.GetLatestBalances()
	if err != nil {
		t.Errorf("Failed to get latest balances: %v", err)
	}

	if len(balances) != 1 {
		t.Errorf("Expected 1 balance, got %d", len(balances))
	}

	savedBalance := balances[0]
	if savedBalance.Currency != balance.Currency {
		t.Errorf("Expected currency %s, got %s", balance.Currency, savedBalance.Currency)
	}
	if savedBalance.Amount != balance.Amount {
		t.Errorf("Expected amount %f, got %f", balance.Amount, savedBalance.Amount)
	}
}

func TestSaveMarketData(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	marketData := &domain.MarketData{
		Symbol:    "BTC_JPY",
		Close:     4000000,
		Volume:    1.5,
		Timestamp: time.Now(),
	}

	err := db.SaveMarketData(marketData)
	if err != nil {
		t.Errorf("Failed to save market data: %v", err)
	}

	// Verification of saved market data is complex, so only verify that no errors occur
}

func TestSavePerformanceMetric(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

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

	err := db.SavePerformanceMetric(metric)
	if err != nil {
		t.Errorf("Failed to save performance metric: %v", err)
	}

	// Retrieve and verify saved performance metrics
	metrics, err := db.GetPerformanceMetrics(1)
	if err != nil {
		t.Errorf("Failed to get performance metrics: %v", err)
	}

	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(metrics))
	}

	savedMetric := metrics[0]
	if savedMetric.TotalReturn != metric.TotalReturn {
		t.Errorf("Expected total return %f, got %f", metric.TotalReturn, savedMetric.TotalReturn)
	}
	if savedMetric.WinRate != metric.WinRate {
		t.Errorf("Expected win rate %f, got %f", metric.WinRate, savedMetric.WinRate)
	}
}

func TestSaveLog(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logEntry := &domain.LogEntry{
		Level:     "INFO",
		Category:  "trading",
		Message:   "Test log message",
		Timestamp: time.Now(),
	}

	err := db.SaveLog(logEntry)
	if err != nil {
		t.Errorf("Failed to save log: %v", err)
	}

	// Retrieve and verify saved logs
	logs, err := db.GetLogs(1)
	if err != nil {
		t.Errorf("Failed to get logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}

	savedLog := logs[0]
	if savedLog.Level != logEntry.Level {
		t.Errorf("Expected level %s, got %s", logEntry.Level, savedLog.Level)
	}
	if savedLog.Message != logEntry.Message {
		t.Errorf("Expected message %s, got %s", logEntry.Message, savedLog.Message)
	}
}

func TestGetRecentTrades_WithLimit(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save multiple trades
	for i := 0; i < 5; i++ {
		trade := &domain.Trade{
			OrderID:      fmt.Sprintf("test_order_%d", i),
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
			t.Errorf("Failed to save trade %d: %v", i, err)
		}
	}

	// Retrieve with limit
	trades, err := db.GetRecentTrades(3)
	if err != nil {
		t.Errorf("Failed to get recent trades: %v", err)
	}

	if len(trades) != 3 {
		t.Errorf("Expected 3 trades, got %d", len(trades))
	}
}

func TestGetLogs_WithLevelFilter(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save logs with different levels
	levels := []string{"INFO", "ERROR", "WARN", "DEBUG"}
	for _, level := range levels {
		logEntry := &domain.LogEntry{
			Level:     level,
			Category:  "test",
			Message:   "Test message for " + level,
			Timestamp: time.Now(),
		}
		if err := db.SaveLog(logEntry); err != nil {
			t.Errorf("Failed to save log with level %s: %v", level, err)
		}
	}

	// Retrieve all logs (without level filtering)
	logs, err := db.GetLogs(10)
	if err != nil {
		t.Errorf("Failed to get logs: %v", err)
	}

	// Verify that all 4 levels have been saved
	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	// Verify the most recent log (DEBUG was saved last)
	if len(logs) > 0 && logs[0].Level != "DEBUG" {
		t.Errorf("Expected most recent log to be DEBUG level, got %s", logs[0].Level)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.Close()
	if err != nil {
		t.Errorf("Failed to close database: %v", err)
	}

	// Operations after closing should result in an error
	trade := &domain.Trade{
		Symbol:       "BTC_JPY",
		Side:         "BUY",
		Type:         "MARKET",
		Size:         0.001,
		Price:        4000000,
		Fee:          60,
		Status:       "COMPLETED",
		CreatedAt:    time.Now(),
		ExecutedAt:   time.Now(),
		StrategyName: "test",
	}
	err = db.SaveTrade(trade)
	if err == nil {
		t.Error("Expected error when using closed database, got nil")
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save trades concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			trade := &domain.Trade{
				OrderID:      fmt.Sprintf("concurrent_order_%d", id),
				Symbol:       "BTC_JPY",
				Side:         "BUY",
				Type:         "MARKET",
				Size:         0.001,
				Price:        4000000 + float64(id*1000),
				Fee:          60,
				Status:       "COMPLETED",
				ExecutedAt:   time.Now(),
				CreatedAt:    time.Now(),
				StrategyName: "simple_test",
			}
			err := db.SaveTrade(trade)
			if err != nil {
				t.Errorf("Failed to save trade %d: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify that all trades have been saved
	trades, err := db.GetRecentTrades(20)
	if err != nil {
		t.Errorf("Failed to get recent trades: %v", err)
	}

	if len(trades) != 10 {
		t.Errorf("Expected 10 trades, got %d", len(trades))
	}
}
