package persistence

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// MaintenanceRepository implements domain.MaintenanceRepository over *DB.
type MaintenanceRepository struct{ db *DB }

// NewMaintenanceRepository creates a MaintenanceRepository backed by db.
func NewMaintenanceRepository(db *DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// Compile-time check.
var _ domain.MaintenanceRepository = (*MaintenanceRepository)(nil)

// GetDatabaseSize returns the current database size in bytes.
func (r *MaintenanceRepository) GetDatabaseSize() (int64, error) {
	var pageCount, pageSize int64
	if err := r.db.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, fmt.Errorf("failed to get page count: %w", err)
	}
	if err := r.db.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, fmt.Errorf("failed to get page size: %w", err)
	}
	return pageCount * pageSize, nil
}

// CleanupOldData removes data older than retentionDays across all tables.
func (r *MaintenanceRepository) CleanupOldData(retentionDays int) error {
	if retentionDays <= 0 {
		return fmt.Errorf("retentionDays must be greater than 0, got %d", retentionDays)
	}
	today := time.Now()
	cutoff := today.AddDate(0, 0, -(retentionDays - 1))
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.Local)

	tx, err := r.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin cleanup transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	steps := []struct {
		table  string
		column string
	}{
		{"logs", "timestamp"},
		{"market_data", "timestamp"},
		{"balances", "timestamp"},
		{"trades", "executed_at"},
	}
	for _, s := range steps {
		if _, err := deleteBeforeTx(tx, s.table, s.column, cutoff); err != nil {
			return fmt.Errorf("cleanup %s: %w", s.table, err)
		}
	}
	// Closed positions cleanup
	if _, err := tx.Exec(`DELETE FROM positions WHERE updated_at < ? AND status = 'CLOSED'`, cutoff); err != nil {
		return fmt.Errorf("cleanup positions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cleanup: %w", err)
	}

	// Non-blocking incremental vacuum (ignore error — non-critical)
	r.db.db.Exec("PRAGMA incremental_vacuum(100)") //nolint:errcheck
	return nil
}

// GetTableStats returns row counts for core tables.
func (r *MaintenanceRepository) GetTableStats() (map[string]int, error) {
	stats := make(map[string]int)
	tables := []string{"balances", "trades", "positions", "performance_metrics", "market_data", "logs"}
	for _, t := range tables {
		var count int
		if err := r.db.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		stats[t] = count
	}
	return stats, nil
}

// deleteBeforeTx deletes rows with timestampColumn < cutoff inside a transaction.
func deleteBeforeTx(tx *sql.Tx, table, timestampColumn string, cutoff time.Time) (int, error) {
	result, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s < ?", table, timestampColumn), cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}
