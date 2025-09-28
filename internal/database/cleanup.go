package database

import (
	"database/sql"
	"fmt"
	"time"
)

// CleanupOldData removes data older than the retention period
// This should be called daily at 00:00 to keep the database lightweight
// retentionDays: number of days to keep (1 = today only, 7 = last 7 days, etc.)
func (db *DB) CleanupOldData(retentionDays int) error {
	var totalDeleted int
	startTime := time.Now()

	db.logger.System().WithField("retention_days", retentionDays).Info("Starting daily cleanup")

	// Cutoff: Start of (N-1) days ago (00:00:00)
	// Everything before this date will be deleted
	// Example: retentionDays=1 (keep today only) → cutoff=start of today (delete yesterday and before)
	//          retentionDays=7 (keep 7 days) → cutoff=start of 6 days ago (keep today + last 6 days)
	today := time.Now()
	cutoffDate := today.AddDate(0, 0, -(retentionDays - 1))
	startOfCutoffDate := time.Date(cutoffDate.Year(), cutoffDate.Month(), cutoffDate.Day(), 0, 0, 0, 0, time.Local)

	// Use transaction to ensure atomicity of cleanup operations
	tx, err := db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// 1. Delete old logs
	deleted, err := db.cleanupDataBeforeDateTx(tx, "logs", "timestamp", startOfCutoffDate)
	if err != nil {
		db.logger.System().WithError(err).Error("Failed to cleanup logs")
		return fmt.Errorf("failed to cleanup logs: %w", err)
	}
	totalDeleted += deleted
	db.logger.System().WithField("deleted", deleted).Info("Cleaned up old logs")

	// 2. Delete old market data
	deleted, err = db.cleanupDataBeforeDateTx(tx, "market_data", "timestamp", startOfCutoffDate)
	if err != nil {
		db.logger.System().WithError(err).Error("Failed to cleanup market data")
		return fmt.Errorf("failed to cleanup market data: %w", err)
	}
	totalDeleted += deleted
	db.logger.System().WithField("deleted", deleted).Info("Cleaned up old market data")

	// 3. Delete old closed positions
	deleted, err = db.cleanupClosedPositionsBeforeDateTx(tx, startOfCutoffDate)
	if err != nil {
		db.logger.System().WithError(err).Error("Failed to cleanup positions")
		return fmt.Errorf("failed to cleanup positions: %w", err)
	}
	totalDeleted += deleted
	db.logger.System().WithField("deleted", deleted).Info("Cleaned up old positions")

	// 4. Delete old balance snapshots
	deleted, err = db.cleanupDataBeforeDateTx(tx, "balances", "timestamp", startOfCutoffDate)
	if err != nil {
		db.logger.System().WithError(err).Error("Failed to cleanup balance snapshots")
		return fmt.Errorf("failed to cleanup balance snapshots: %w", err)
	}
	totalDeleted += deleted
	db.logger.System().WithField("deleted", deleted).Info("Cleaned up old balance snapshots")

	// 5. Delete old trades
	deleted, err = db.cleanupDataBeforeDateTx(tx, "trades", "executed_at", startOfCutoffDate)
	if err != nil {
		db.logger.System().WithError(err).Error("Failed to cleanup trades")
		return fmt.Errorf("failed to cleanup trades: %w", err)
	}
	totalDeleted += deleted
	db.logger.System().WithField("deleted", deleted).Info("Cleaned up old trades")

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	// 6. Incremental VACUUM to reclaim disk space (non-blocking)
	if err := db.incrementalVacuum(100); err != nil {
		db.logger.System().WithError(err).Warn("Failed to run incremental VACUUM (non-critical)")
	} else {
		db.logger.System().Info("Incremental VACUUM completed (non-blocking)")
	}

	duration := time.Since(startTime)
	db.logger.System().WithField("total_deleted", totalDeleted).
		WithField("duration_ms", duration.Milliseconds()).
		WithField("cutoff_date", startOfCutoffDate.Format("2006-01-02")).
		WithField("retention_days", retentionDays).
		Info("Daily cleanup completed successfully")

	return nil
}

// Whitelist of allowed tables and columns for SQL injection prevention
var (
	allowedTables = map[string]bool{
		"logs":        true,
		"market_data": true,
		"positions":   true,
		"balances":    true,
		"trades":      true,
	}
	allowedColumns = map[string]bool{
		"timestamp":   true,
		"executed_at": true,
		"updated_at":  true,
	}
)

// cleanupDataBeforeDate is a generic cleanup function for tables with timestamp columns
func (db *DB) cleanupDataBeforeDate(tableName, timestampColumn string, cutoffDate time.Time) (int, error) {

	// Validate table and column names against whitelist (SQL injection prevention)
	if !allowedTables[tableName] {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}
	if !allowedColumns[timestampColumn] {
		return 0, fmt.Errorf("invalid column name: %s", timestampColumn)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s < ?", tableName, timestampColumn)
	result, err := db.db.Exec(query, cutoffDate)

	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// cleanupDataBeforeDateTx is transaction version of cleanupDataBeforeDate
func (db *DB) cleanupDataBeforeDateTx(tx *sql.Tx, tableName, timestampColumn string, cutoffDate time.Time) (int, error) {
	// Validate table and column names against whitelist (SQL injection prevention)
	if !allowedTables[tableName] {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}
	if !allowedColumns[timestampColumn] {
		return 0, fmt.Errorf("invalid column name: %s", timestampColumn)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s < ?", tableName, timestampColumn)
	result, err := tx.Exec(query, cutoffDate)

	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// cleanupClosedPositionsBeforeDate deletes all closed positions before the specified date
func (db *DB) cleanupClosedPositionsBeforeDate(cutoffDate time.Time) (int, error) {

	result, err := db.db.Exec(`
		DELETE FROM positions
		WHERE updated_at < ?
		AND status = 'CLOSED'
	`, cutoffDate)

	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// cleanupClosedPositionsBeforeDateTx is transaction version of cleanupClosedPositionsBeforeDate
func (db *DB) cleanupClosedPositionsBeforeDateTx(tx *sql.Tx, cutoffDate time.Time) (int, error) {
	result, err := tx.Exec(`
		DELETE FROM positions
		WHERE updated_at < ?
		AND status = 'CLOSED'
	`, cutoffDate)

	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// incrementalVacuum reclaims disk space incrementally (non-blocking)
// pages: number of pages to free (100 pages ~= 400KB with default 4KB page size)
func (db *DB) incrementalVacuum(pages int) error {

	// PRAGMA incremental_vacuum is non-blocking and faster than full VACUUM
	_, err := db.db.Exec("PRAGMA incremental_vacuum(?)", pages)
	return err
}

// GetDatabaseSize returns the current database file size in bytes
func (db *DB) GetDatabaseSize() (int64, error) {
	var pageCount, pageSize int64

	err := db.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get page count: %w", err)
	}

	err = db.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get page size: %w", err)
	}

	return pageCount * pageSize, nil
}

// GetTableStats returns statistics for all tables
func (db *DB) GetTableStats() (map[string]int, error) {
	stats := make(map[string]int)

	tables := []string{"balances", "trades", "positions", "performance_metrics", "market_data", "logs"}

	for _, table := range tables {
		// Validate table name against whitelist (SQL injection prevention)
		if !allowedTables[table] && table != "performance_metrics" {
			return nil, fmt.Errorf("invalid table name: %s", table)
		}

		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		err := db.db.QueryRow(query).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to get count for table %s: %w", table, err)
		}
		stats[table] = count
	}

	return stats, nil
}
