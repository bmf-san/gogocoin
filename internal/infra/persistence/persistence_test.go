package persistence_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/persistence"
	"github.com/bmf-san/gogocoin/internal/logger"
)

// errForcedRollback is used to force rollback inside WithTransaction tests.
var errForcedRollback = fmt.Errorf("forced rollback")

func newTestDB(t *testing.T) *persistence.DB {
	t.Helper()
	db, err := persistence.NewDB(":memory:", logger.NewNopLogger())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ─── DB lifecycle ─────────────────────────────────────────────────────────

func TestDB_Ping(t *testing.T) {
	db := newTestDB(t)
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// ─── TradeRepository ──────────────────────────────────────────────────────

func TestTradeRepository_SaveAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewTradeRepository(db)

	trade := &domain.Trade{
		Symbol: "BTC_JPY", Side: "BUY", Type: "MARKET",
		Size: 0.01, Price: 4000000, Status: "COMPLETED",
		OrderID: "ORDER-1", StrategyName: "scalping",
	}
	if err := repo.SaveTrade(trade); err != nil {
		t.Fatalf("SaveTrade: %v", err)
	}
	trades, err := repo.GetRecentTrades(10)
	if err != nil {
		t.Fatalf("GetRecentTrades: %v", err)
	}
	if len(trades) != 1 || trades[0].OrderID != "ORDER-1" {
		t.Errorf("unexpected result: %v", trades)
	}
}

func TestTradeRepository_Upsert(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewTradeRepository(db)

	trade := &domain.Trade{
		OrderID: "ORDER-2", Symbol: "XRP_JPY", Side: "BUY",
		Type: "MARKET", Size: 1, Price: 100, Status: "COMPLETED",
	}
	if err := repo.SaveTrade(trade); err != nil {
		t.Fatalf("first SaveTrade: %v", err)
	}
	trade.Status = "CANCELED"
	if err := repo.SaveTrade(trade); err != nil {
		t.Fatalf("second SaveTrade (upsert): %v", err)
	}
	trades, _ := repo.GetRecentTrades(10)
	if len(trades) != 1 {
		t.Errorf("expected 1 row after upsert, got %d", len(trades))
	}
}

// ─── PositionRepository ───────────────────────────────────────────────────

func TestPositionRepository_SaveAndGetOpen(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewPositionRepository(db)

	pos := &domain.Position{
		Symbol: "XRP_JPY", Side: "BUY",
		Size: 10, RemainingSize: 10, EntryPrice: 100, CurrentPrice: 105,
		Status: "OPEN", OrderID: "POS-1",
	}
	if err := repo.SavePosition(pos); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
	positions, err := repo.GetOpenPositions("XRP_JPY", "BUY")
	if err != nil {
		t.Fatalf("GetOpenPositions: %v", err)
	}
	if len(positions) != 1 || positions[0].OrderID != "POS-1" {
		t.Errorf("unexpected result: %v", positions)
	}
}

func TestPositionRepository_GetOpenPositions_IncludesPartial(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewPositionRepository(db)

	// PARTIAL position: partially filled, remaining_size > 0 — stop-loss must see this.
	partial := &domain.Position{
		Symbol: "XRP_JPY", Side: "BUY",
		Size: 10, UsedSize: 3, RemainingSize: 7, EntryPrice: 229.0, CurrentPrice: 226.0,
		Status: "PARTIAL", OrderID: "POS-PARTIAL",
	}
	// CLOSED position: must NOT be returned.
	closed := &domain.Position{
		Symbol: "XRP_JPY", Side: "BUY",
		Size: 5, UsedSize: 5, RemainingSize: 0, EntryPrice: 220.0, CurrentPrice: 226.0,
		Status: "CLOSED", OrderID: "POS-CLOSED",
	}
	for _, p := range []*domain.Position{partial, closed} {
		if err := repo.SavePosition(p); err != nil {
			t.Fatalf("SavePosition %s: %v", p.OrderID, err)
		}
	}

	positions, err := repo.GetOpenPositions("XRP_JPY", "BUY")
	if err != nil {
		t.Fatalf("GetOpenPositions: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position (PARTIAL), got %d: %v", len(positions), positions)
	}
	if positions[0].OrderID != "POS-PARTIAL" {
		t.Errorf("expected POS-PARTIAL, got %v", positions[0].OrderID)
	}
}

func TestPositionRepository_UpdatePosition(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewPositionRepository(db)

	pos := &domain.Position{
		Symbol: "XRP_JPY", Side: "BUY",
		Size: 10, RemainingSize: 10, EntryPrice: 100, CurrentPrice: 100,
		Status: "OPEN", OrderID: "POS-2",
	}
	repo.SavePosition(pos)
	pos.Status = "CLOSED"
	pos.RemainingSize = 0
	if err := repo.UpdatePosition(pos); err != nil {
		t.Fatalf("UpdatePosition: %v", err)
	}
	open, _ := repo.GetOpenPositions("XRP_JPY", "BUY")
	if len(open) != 0 {
		t.Errorf("expected 0 open positions, got %d", len(open))
	}
}

// ─── BalanceRepository ────────────────────────────────────────────────────

func TestBalanceRepository_SaveAndGetLatest(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewBalanceRepository(db)

	repo.SaveBalance(domain.Balance{Currency: "JPY", Available: 100000, Amount: 100000})
	repo.SaveBalance(domain.Balance{Currency: "JPY", Available: 200000, Amount: 200000})
	repo.SaveBalance(domain.Balance{Currency: "BTC", Available: 0.5, Amount: 0.5})

	balances, err := repo.GetLatestBalances()
	if err != nil {
		t.Fatalf("GetLatestBalances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 currencies, got %d", len(balances))
	}
	for _, b := range balances {
		if b.Currency == "JPY" && b.Available != 200000 {
			t.Errorf("expected latest JPY 200000, got %f", b.Available)
		}
	}
}

// ─── AppStateRepository ───────────────────────────────────────────────────

func TestAppStateRepository_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewAppStateRepository(db)

	if err := repo.SaveAppState("trading_enabled", "true"); err != nil {
		t.Fatalf("SaveAppState: %v", err)
	}
	val, err := repo.GetAppState("trading_enabled")
	if err != nil {
		t.Fatalf("GetAppState: %v", err)
	}
	if val != "true" {
		t.Errorf("expected 'true', got %q", val)
	}
}

func TestAppStateRepository_MissingKey(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewAppStateRepository(db)
	val, err := repo.GetAppState("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error for missing key: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

// ─── Transaction ──────────────────────────────────────────────────────────

func TestWithTransaction_Commit(t *testing.T) {
	db := newTestDB(t)
	tradeRepo := persistence.NewTradeRepository(db)

	err := persistence.WithTransaction(db, logger.NewNopLogger(), func(tx domain.Transaction) error {
		return tx.SaveTrade(&domain.Trade{
			Symbol: "BTC_JPY", Side: "BUY", Type: "MARKET",
			Size: 0.01, Price: 4000000, Status: "COMPLETED", OrderID: "TX-1",
		})
	})
	if err != nil {
		t.Fatalf("WithTransaction commit: %v", err)
	}
	trades, _ := tradeRepo.GetRecentTrades(10)
	if len(trades) != 1 {
		t.Errorf("expected 1 trade after commit, got %d", len(trades))
	}
}

func TestWithTransaction_Rollback(t *testing.T) {
	db := newTestDB(t)
	tradeRepo := persistence.NewTradeRepository(db)

	persistence.WithTransaction(db, logger.NewNopLogger(), func(tx domain.Transaction) error { //nolint:errcheck
		tx.SaveTrade(&domain.Trade{ //nolint:errcheck
			Symbol: "BTC_JPY", Side: "BUY", Type: "MARKET",
			Size: 0.01, Price: 4000000, Status: "COMPLETED", OrderID: "TX-ROLLBACK",
		})
		return errForcedRollback
	})
	trades, _ := tradeRepo.GetRecentTrades(10)
	if len(trades) != 0 {
		t.Errorf("expected 0 trades after rollback, got %d", len(trades))
	}
}

// ─── LogRepository ────────────────────────────────────────────────────────

func TestLogRepository_SaveAndQuery(t *testing.T) {
	db := newTestDB(t)
	repo := persistence.NewLogRepository(db)

	entry := &domain.LogEntry{
		Level:     "INFO",
		Category:  "trading",
		Message:   "test message",
		Fields:    map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
	}
	if err := repo.SaveLog(entry); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}
	logs, err := repo.GetRecentLogsWithFilters(10, "INFO", "trading")
	if err != nil {
		t.Fatalf("GetRecentLogsWithFilters: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "test message" {
		t.Errorf("unexpected result: %v", logs)
	}
}
