package persistence

import (
	"testing"
	"time"

	"github.com/bmf-san/gogocoin/internal/logger"
)

func newTestDBInternal(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(":memory:", logger.NewNopLogger())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCleanupOldData_InvalidRetentionDays(t *testing.T) {
	db := newTestDBInternal(t)
	repo := NewMaintenanceRepository(db)
	for _, days := range []int{0, -1, -100} {
		if err := repo.CleanupOldData(days); err == nil {
			t.Errorf("CleanupOldData(%d): expected error, got nil", days)
		}
	}
}

const tradeInsertSQL = `INSERT INTO trades
	(symbol, side, type, size, price, fee, status, order_id, executed_at, created_at, updated_at)
	VALUES (?,?,?,?,?,?,?,?,?,?,?)`

func TestCleanupOldData_DeletesOldTradesKeepsNew(t *testing.T) {
	db := newTestDBInternal(t)
	repo := NewMaintenanceRepository(db)
	old := time.Now().AddDate(0, 0, -30).UTC()
	recent := time.Now().UTC()
	db.db.Exec(tradeInsertSQL, "BTC_JPY", "BUY", "MARKET", 0.001, 5000000, 0, "COMPLETED", "OLD-1", old, old, old)
	db.db.Exec(tradeInsertSQL, "BTC_JPY", "BUY", "MARKET", 0.001, 5100000, 0, "COMPLETED", "RECENT-1", recent, recent, recent)
	if err := repo.CleanupOldData(7); err != nil {
		t.Fatalf("CleanupOldData(7): %v", err)
	}
	var count int
	db.db.QueryRow("SELECT COUNT(*) FROM trades").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining trade, got %d", count)
	}
	var id string
	db.db.QueryRow("SELECT order_id FROM trades").Scan(&id)
	if id != "RECENT-1" {
		t.Errorf("expected RECENT-1 to survive, got %q", id)
	}
}

const posInsertSQL = `INSERT INTO positions
	(symbol,side,size,used_size,remaining_size,entry_price,current_price,unrealized_pl,pnl,status,order_id,created_at,updated_at)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`

func TestCleanupOldData_PositionStatusFilter(t *testing.T) {
	db := newTestDBInternal(t)
	repo := NewMaintenanceRepository(db)
	old := time.Now().AddDate(0, 0, -30).UTC()
	db.db.Exec(posInsertSQL, "BTC_JPY", "BUY", 0.001, 0.001, 0, 5000000, 5000000, 0, 0, "CLOSED", "POS-C", old, old)
	db.db.Exec(posInsertSQL, "BTC_JPY", "BUY", 0.001, 0, 0.001, 5000000, 5000000, 0, 0, "OPEN", "POS-O", old, old)
	if err := repo.CleanupOldData(7); err != nil {
		t.Fatalf("CleanupOldData(7): %v", err)
	}
	var count int
	db.db.QueryRow("SELECT COUNT(*) FROM positions").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 position (OPEN) after cleanup, got %d", count)
	}
	var status string
	db.db.QueryRow("SELECT status FROM positions").Scan(&status)
	if status != "OPEN" {
		t.Errorf("expected surviving position to be OPEN, got %q", status)
	}
}
