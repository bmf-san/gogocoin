package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

// DB is a SQLite database
type DB struct {
	db     *sql.DB
	logger *logger.Logger
}

// NewDB creates a new database
func NewDB(dbPath string, logger *logger.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection test
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// WAL mode and concurrent connections require a file-based database.
	// In-memory databases (:memory:) do not support WAL, and multiple
	// connections each get their own isolated in-memory database, so we must
	// keep MaxOpenConns=1 for them (used in tests).
	isMemory := dbPath == ":memory:" || strings.Contains(dbPath, "mode=memory")

	if !isMemory {
		// WAL mode allows concurrent readers while a writer is active, which
		// prevents /api/status and other read handlers from blocking on frequent
		// market-data and log writes.
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
		// busy_timeout tells SQLite to retry for up to 5 s before returning
		// "database is locked", instead of failing immediately.
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
		}
		// With WAL mode, multiple readers and one writer can run concurrently.
		// Allow up to 10 open connections so read-heavy handlers (status, balance,
		// trades) are not forced to wait behind write operations.
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
	} else {
		// In-memory SQLite: single connection keeps all goroutines on the same DB.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	db.SetConnMaxLifetime(0) // No expiration

	dbInstance := &DB{
		db:     db,
		logger: logger,
	}

	// Enable auto_vacuum for non-blocking cleanup
	if _, err := db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		logger.System().WithError(err).Warn("Failed to enable auto_vacuum (non-critical)")
	}

	// Run migrations
	if err := dbInstance.runMigrations(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return dbInstance, nil
}

// Close closes the database
func (db *DB) Close() error {
	return db.db.Close()
}

// Ping checks if the database connection is alive
func (db *DB) Ping() error {
	if db.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	return db.db.Ping()
}

// DBTx represents a database transaction
type DBTx struct {
	tx     *sql.Tx
	logger *logger.Logger
}

// BeginTx starts a new transaction
func (db *DB) BeginTx() (domain.Transaction, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &DBTx{
		tx:     tx,
		logger: db.logger,
	}, nil
}

// Commit commits the transaction
func (tx *DBTx) Commit() error {
	return tx.tx.Commit()
}

// Rollback rolls back the transaction
func (tx *DBTx) Rollback() error {
	return tx.tx.Rollback()
}

// SaveTrade saves a trade within a transaction
func (tx *DBTx) SaveTrade(trade *domain.Trade) error {
	now := time.Now()
	if trade.ExecutedAt.IsZero() {
		trade.ExecutedAt = now
	}
	if trade.CreatedAt.IsZero() {
		trade.CreatedAt = now
	}

	query := `INSERT INTO trades (symbol, side, type, size, price, fee,
		  status, order_id, executed_at, created_at, updated_at, strategy_name, pnl)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		  ON CONFLICT(order_id) DO UPDATE SET
		  status = excluded.status,
		  size = excluded.size,
		  price = excluded.price,
		  fee = excluded.fee,
		  pnl = excluded.pnl,
		  executed_at = excluded.executed_at,
		  updated_at = excluded.updated_at`

	_, err := tx.tx.Exec(query, trade.Symbol, trade.Side, trade.Type,
		trade.Size, trade.Price, trade.Fee, trade.Status, trade.OrderID,
		trade.ExecutedAt, trade.CreatedAt, now, trade.StrategyName, trade.PnL)

	return err
}

// SavePosition saves a position within a transaction
func (tx *DBTx) SavePosition(position *domain.Position) error {
	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	position.UpdatedAt = now

	query := `INSERT INTO positions (symbol, side, size, used_size, remaining_size,
		  entry_price, current_price, status, order_id, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.tx.Exec(query, position.Symbol, position.Side,
		position.Size, position.UsedSize, position.RemainingSize,
		position.EntryPrice, position.CurrentPrice, position.Status,
		position.OrderID, position.CreatedAt, position.UpdatedAt)

	return err
}

// UpdatePosition updates a position within a transaction
func (tx *DBTx) UpdatePosition(position *domain.Position) error {
	position.UpdatedAt = time.Now()

	query := `UPDATE positions SET used_size = ?, remaining_size = ?, current_price = ?,
		  status = ?, updated_at = ? WHERE order_id = ?`

	_, err := tx.tx.Exec(query, position.UsedSize, position.RemainingSize,
		position.CurrentPrice, position.Status, position.UpdatedAt, position.OrderID)

	return err
}

// createTables is deprecated - now using runMigrations() with SQL files
// Kept temporarily for reference, will be removed in future release

// SaveBalance saves balance information
func (db *DB) SaveBalance(balance domain.Balance) error {

	query := `INSERT INTO balances (currency, available, amount, timestamp)
			  VALUES (?, ?, ?, ?)`

	_, err := db.db.Exec(query, balance.Currency, balance.Available, balance.Amount, time.Now())
	return err
}

// GetLatestBalances gets the latest balance list
func (db *DB) GetLatestBalances() ([]domain.Balance, error) {

	query := `SELECT id, currency, available, amount, timestamp FROM balances
			  WHERE id IN (
				  SELECT MAX(id) FROM balances GROUP BY currency
			  ) ORDER BY currency`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	balances := make([]domain.Balance, 0, 10) // Pre-allocate for typical currency count
	for rows.Next() {
		var b domain.Balance
		if err := rows.Scan(&b.ID, &b.Currency, &b.Available, &b.Amount, &b.Timestamp); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}

	return balances, rows.Err()
}

// SaveTrade saves a trade using UPSERT (INSERT or UPDATE if order_id exists)
func (db *DB) SaveTrade(trade *domain.Trade) error {

	now := time.Now()
	if trade.ExecutedAt.IsZero() {
		trade.ExecutedAt = now
	}
	if trade.CreatedAt.IsZero() {
		trade.CreatedAt = now
	}

	// Use UPSERT to avoid duplicates: INSERT or UPDATE if order_id exists
	query := `INSERT INTO trades (symbol, side, type, size, price, fee,
		  status, order_id, executed_at, created_at, updated_at, strategy_name, pnl)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		  ON CONFLICT(order_id) DO UPDATE SET
		  status = excluded.status,
		  size = excluded.size,
		  price = excluded.price,
		  fee = excluded.fee,
		  pnl = excluded.pnl,
		  executed_at = excluded.executed_at,
		  updated_at = excluded.updated_at`

	_, err := db.db.Exec(query, trade.Symbol, trade.Side, trade.Type,
		trade.Size, trade.Price, trade.Fee, trade.Status, trade.OrderID,
		trade.ExecutedAt, trade.CreatedAt, now, trade.StrategyName, trade.PnL)

	return err
}

// GetTrades gets trading history (all statuses, latest per order_id)
func (db *DB) GetTrades(limit int) ([]domain.Trade, error) {

	query := `SELECT symbol, side, type, size, price, fee, status, order_id,
			  executed_at, created_at, strategy_name, pnl
			  FROM trades ORDER BY executed_at DESC LIMIT ?`

	rows, err := db.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	trades := make([]domain.Trade, 0, limit) // Pre-allocate for requested limit
	for rows.Next() {
		var t domain.Trade
		if err := rows.Scan(&t.Symbol, &t.Side, &t.Type, &t.Size, &t.Price, &t.Fee,
			&t.Status, &t.OrderID, &t.ExecutedAt, &t.CreatedAt, &t.StrategyName,
			&t.PnL); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	return trades, rows.Err()
}

// SavePosition saves a position
func (db *DB) SavePosition(position *domain.Position) error {

	query := `INSERT INTO positions (symbol, side, size, used_size, remaining_size,
		  entry_price, current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	position.UpdatedAt = now

	_, err := db.db.Exec(query, position.Symbol, position.Side,
		position.Size, position.UsedSize, position.RemainingSize, position.EntryPrice,
		position.CurrentPrice, position.UnrealizedPL, position.PnL,
		position.Status, position.OrderID, position.CreatedAt, position.UpdatedAt)

	return err
}

// GetPositions gets the position list
func (db *DB) GetPositions() ([]domain.Position, error) {

	query := `SELECT id, symbol, side, size, used_size, remaining_size, entry_price,
			  current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at
			  FROM positions ORDER BY created_at DESC`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	positions := make([]domain.Position, 0, 20) // Pre-allocate for typical position count
	for rows.Next() {
		var p domain.Position
		if err := rows.Scan(&p.ID, &p.Symbol, &p.Side, &p.Size, &p.UsedSize, &p.RemainingSize,
			&p.EntryPrice, &p.CurrentPrice, &p.UnrealizedPL, &p.PnL,
			&p.Status, &p.OrderID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

// GetActivePositions gets the active position list
func (db *DB) GetActivePositions() ([]domain.Position, error) {

	query := `SELECT id, symbol, side, size, used_size, remaining_size, entry_price,
			  current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at
			  FROM positions WHERE size != 0 AND status = 'OPEN' ORDER BY created_at DESC`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	positions := make([]domain.Position, 0, 20) // Pre-allocate for typical position count
	for rows.Next() {
		var p domain.Position
		if err := rows.Scan(&p.ID, &p.Symbol, &p.Side, &p.Size, &p.UsedSize, &p.RemainingSize,
			&p.EntryPrice, &p.CurrentPrice, &p.UnrealizedPL, &p.PnL,
			&p.Status, &p.OrderID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

// GetOpenPositions gets open positions for matching opposite trades (domain.Position)
func (db *DB) GetOpenPositions(symbol string, side string) ([]domain.Position, error) {

	query := `SELECT id, symbol, side, size, used_size, remaining_size, entry_price,
			  current_price, unrealized_pl, status, order_id, created_at, updated_at
			  FROM positions
			  WHERE symbol = ? AND side = ? AND status = 'OPEN' AND remaining_size > 0
			  ORDER BY created_at ASC`

	rows, err := db.db.Query(query, symbol, side)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	positions := make([]domain.Position, 0, 20) // Pre-allocate for typical position count
	for rows.Next() {
		var p domain.Position
		var id int
		if err := rows.Scan(&id, &p.Symbol, &p.Side, &p.Size, &p.UsedSize, &p.RemainingSize,
			&p.EntryPrice, &p.CurrentPrice, &p.UnrealizedPL, &p.Status, &p.OrderID,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

// UpdatePosition updates an existing position
// Note: Caller should call position.UpdateStatus() before calling this method
func (db *DB) UpdatePosition(position *domain.Position) error {

	query := `UPDATE positions
			  SET used_size = ?, remaining_size = ?, current_price = ?,
			      unrealized_pl = ?, status = ?, updated_at = ?
			  WHERE order_id = ?`

	position.UpdatedAt = time.Now()

	_, err := db.db.Exec(query, position.UsedSize, position.RemainingSize,
		position.CurrentPrice, position.UnrealizedPL, position.Status,
		position.UpdatedAt, position.OrderID)

	return err
}

// SavePerformanceMetric saves performance metrics
func (db *DB) SavePerformanceMetric(metric *domain.PerformanceMetric) error {

	query := `INSERT INTO performance_metrics (date, total_return, daily_return, win_rate,
			  max_drawdown, sharpe_ratio, profit_factor, total_trades, winning_trades,
			  losing_trades, average_win, average_loss, largest_win, largest_loss,
			  consecutive_wins, consecutive_loss, total_pnl)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if metric.Date.IsZero() {
		metric.Date = time.Now()
	}

	_, err := db.db.Exec(query, metric.Date, metric.TotalReturn, metric.DailyReturn,
		metric.WinRate, metric.MaxDrawdown, metric.SharpeRatio, metric.ProfitFactor,
		metric.TotalTrades, metric.WinningTrades, metric.LosingTrades, metric.AverageWin,
		metric.AverageLoss, metric.LargestWin, metric.LargestLoss, metric.ConsecutiveWins,
		metric.ConsecutiveLoss, metric.TotalPnL)

	return err
}

// GetPerformanceMetrics gets performance metrics
func (db *DB) GetPerformanceMetrics(days int) ([]domain.PerformanceMetric, error) {

	var query string
	var args []interface{}

	if days > 0 {
		query = `SELECT id, date, total_return, daily_return, win_rate, max_drawdown,
				 sharpe_ratio, profit_factor, total_trades, winning_trades, losing_trades,
				 average_win, average_loss, largest_win, largest_loss, consecutive_wins,
				 consecutive_loss, total_pnl FROM performance_metrics
				 WHERE date >= datetime('now', '-' || ? || ' days') ORDER BY date DESC`
		args = append(args, days)
	} else {
		query = `SELECT id, date, total_return, daily_return, win_rate, max_drawdown,
				 sharpe_ratio, profit_factor, total_trades, winning_trades, losing_trades,
				 average_win, average_loss, largest_win, largest_loss, consecutive_wins,
				 consecutive_loss, total_pnl FROM performance_metrics ORDER BY date DESC`
	}

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	metrics := make([]domain.PerformanceMetric, 0, 100) // Pre-allocate for typical metrics count
	for rows.Next() {
		var m domain.PerformanceMetric
		var id int // Database-specific field
		if err := rows.Scan(&id, &m.Date, &m.TotalReturn, &m.DailyReturn, &m.WinRate,
			&m.MaxDrawdown, &m.SharpeRatio, &m.ProfitFactor, &m.TotalTrades,
			&m.WinningTrades, &m.LosingTrades, &m.AverageWin, &m.AverageLoss,
			&m.LargestWin, &m.LargestLoss, &m.ConsecutiveWins, &m.ConsecutiveLoss,
			&m.TotalPnL); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// SaveMarketData saves market data
func (db *DB) SaveMarketData(data *domain.MarketData) error {

	query := `INSERT OR REPLACE INTO market_data (symbol, timestamp, open, high, low, close, volume, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err := db.db.Exec(query, data.Symbol, data.Timestamp,
		data.Open, data.High, data.Low, data.Close, data.Volume, now)

	return err
}

// SaveTicker saves ticker data (implements domain.MarketDataRepository)
func (db *DB) SaveTicker(ticker *domain.MarketData) error {
	return db.SaveMarketData(ticker)
}

// GetHistoricalMarketData gets market data for the specified period
func (db *DB) GetHistoricalMarketData(symbol string, startDate, endDate time.Time) ([]domain.MarketData, error) {

	query := `SELECT id, symbol, timestamp, open, high, low, close, volume, created_at
			  FROM market_data
			  WHERE symbol = ? AND timestamp BETWEEN ? AND ?
			  ORDER BY timestamp ASC`

	rows, err := db.db.Query(query, symbol, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	data := make([]domain.MarketData, 0, 1000) // Pre-allocate for typical market data history
	for rows.Next() {
		var d domain.MarketData
		var id int
		var createdAt time.Time
		if err := rows.Scan(&id, &d.Symbol, &d.Timestamp,
			&d.Open, &d.High, &d.Low, &d.Close, &d.Volume, &createdAt); err != nil {
			return nil, err
		}
		data = append(data, d)
	}

	return data, rows.Err()
}

// GetLatestMarketData gets the latest market data
func (db *DB) GetLatestMarketData(symbol string, limit int) ([]domain.MarketData, error) {

	query := `SELECT id, symbol, timestamp, open, high, low, close, volume, created_at
			  FROM market_data
			  WHERE symbol = ?
			  ORDER BY timestamp DESC
			  LIMIT ?`

	rows, err := db.db.Query(query, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	data := make([]domain.MarketData, 0, 1000) // Pre-allocate for typical market data history
	for rows.Next() {
		var d domain.MarketData
		var id int
		var createdAt time.Time
		if err := rows.Scan(&id, &d.Symbol, &d.Timestamp,
			&d.Open, &d.High, &d.Low, &d.Close, &d.Volume, &createdAt); err != nil {
			return nil, err
		}
		data = append(data, d)
	}

	return data, rows.Err()
}

// GetLatestMarketDataForSymbols retrieves the latest market data for multiple symbols in a single query
func (db *DB) GetLatestMarketDataForSymbols(symbols []string) (map[string]domain.MarketData, error) {

	if len(symbols) == 0 {
		return make(map[string]domain.MarketData), nil
	}

	// Build IN clause placeholders
	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols))
	for i, symbol := range symbols {
		placeholders[i] = "?"
		args[i] = symbol
	}

	// Use a subquery to get the latest timestamp for each symbol
	query := `SELECT m.symbol, m.timestamp, m.open, m.high, m.low, m.close, m.volume
			  FROM market_data m
			  INNER JOIN (
			      SELECT symbol, MAX(timestamp) as max_timestamp
			      FROM market_data
			      WHERE symbol IN (` + strings.Join(placeholders, ",") + `)
			      GROUP BY symbol
			  ) latest ON m.symbol = latest.symbol AND m.timestamp = latest.max_timestamp`

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	result := make(map[string]domain.MarketData, len(symbols)) // Pre-allocate with known size
	for rows.Next() {
		var d domain.MarketData
		if err := rows.Scan(&d.Symbol, &d.Timestamp,
			&d.Open, &d.High, &d.Low, &d.Close, &d.Volume); err != nil {
			return nil, err
		}
		result[d.Symbol] = d
	}

	return result, rows.Err()
}

// saveLogEntry saves a log entry.
func (db *DB) saveLogEntry(entry *domain.LogEntry) error {
	query := `INSERT INTO logs (level, category, message, fields, timestamp)
			  VALUES (?, ?, ?, ?, ?)`

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Convert fields map to JSON string
	fieldsJSON, err := json.Marshal(entry.Fields)
	if err != nil {
		fieldsJSON = []byte("{}")
	}

	_, err = db.db.Exec(query, entry.Level, entry.Category, entry.Message, string(fieldsJSON), entry.Timestamp)
	return err
}

// SaveLog receives domain.LogEntry and saves the log entry
func (db *DB) SaveLog(entry *domain.LogEntry) error {
	return db.saveLogEntry(entry)
}

// convertToLogEntry converts interface{} to domain.LogEntry
func (db *DB) convertToLogEntry(entry interface{}) domain.LogEntry {
	// Get fields using reflection
	v := reflect.ValueOf(entry)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	var level, category, message string
	var timestamp time.Time

	if v.Kind() == reflect.Struct {
		// Get Level field
		if levelField := v.FieldByName("Level"); levelField.IsValid() && levelField.Kind() == reflect.String {
			level = levelField.String()
		}

		// Get Category field
		if categoryField := v.FieldByName("Category"); categoryField.IsValid() && categoryField.Kind() == reflect.String {
			category = categoryField.String()
		}

		// Get Message field
		if messageField := v.FieldByName("Message"); messageField.IsValid() && messageField.Kind() == reflect.String {
			message = messageField.String()
		}

		// Get Timestamp field
		if timestampField := v.FieldByName("Timestamp"); timestampField.IsValid() {
			if timestampField.Type() == reflect.TypeOf(time.Time{}) {
				timestamp = timestampField.Interface().(time.Time)
			}
		}
	}

	// Configure default values
	if level == "" {
		level = "INFO"
	}
	if category == "" {
		category = "system"
	}
	if message == "" {
		message = "Empty log message"
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return domain.LogEntry{
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    nil, // Will be populated from fields string
		Timestamp: timestamp,
	}
}

// GetLogs gets log entries
func (db *DB) GetLogs(limit int) ([]domain.LogEntry, error) {

	query := `SELECT id, level, category, message, fields, timestamp
			  FROM logs ORDER BY timestamp DESC LIMIT ?`

	rows, err := db.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	logs := make([]domain.LogEntry, 0, 100) // Pre-allocate for typical log query result
	for rows.Next() {
		var l domain.LogEntry
		var id int
		var fieldsJSON string
		if err := rows.Scan(&id, &l.Level, &l.Category, &l.Message, &fieldsJSON, &l.Timestamp); err != nil {
			return nil, err
		}

		// Unmarshal fieldsJSON to l.Fields
		if fieldsJSON != "" {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
				// Log unmarshal error but don't fail the entire query
				db.logger.System().WithError(err).WithField("fields_json", fieldsJSON).
					Warn("Failed to unmarshal log fields, using empty map")
				l.Fields = make(map[string]interface{})
			} else {
				l.Fields = fields
			}
		} else {
			l.Fields = make(map[string]interface{})
		}

		logs = append(logs, l)
	}

	return logs, rows.Err()
}

// GetRecentTrades gets recent trading history
func (db *DB) GetRecentTrades(limit int) ([]domain.Trade, error) {
	return db.GetTrades(limit)
}

// GetTradesCount returns total number of trades
func (db *DB) GetTradesCount() (int, error) {

	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM trades").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count trades: %w", err)
	}
	return count, nil
}

// GetActivePositionsCount returns number of active positions
func (db *DB) GetActivePositionsCount() (int, error) {

	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM positions WHERE status = ?", "OPEN").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active positions: %w", err)
	}
	return count, nil
}

// GetRecentLogsWithFilters gets recent log entries with level and category filters
func (db *DB) GetRecentLogsWithFilters(limit int, level, category string) ([]domain.LogEntry, error) {

	var query string
	var args []interface{}
	var conditions []string

	// Build WHERE clause based on filters
	if level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, level)
	}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	// Build query
	query = `SELECT id, level, category, message, fields, timestamp FROM logs`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.logger.System().WithError(err).Error("Failed to close rows")
		}
	}()

	// Initialize with empty slice to ensure JSON returns [] instead of null
	logs := make([]domain.LogEntry, 0, 100) // Pre-allocate for typical log query result
	for rows.Next() {
		var l domain.LogEntry
		var id int
		var fieldsJSON string
		if err := rows.Scan(&id, &l.Level, &l.Category, &l.Message, &fieldsJSON, &l.Timestamp); err != nil {
			return nil, err
		}

		// Unmarshal fieldsJSON to l.Fields
		if fieldsJSON != "" {
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
				// Log unmarshal error but don't fail the entire query
				db.logger.System().WithError(err).WithField("fields_json", fieldsJSON).
					Warn("Failed to unmarshal log fields, using empty map")
				l.Fields = make(map[string]interface{})
			} else {
				l.Fields = fields
			}
		} else {
			l.Fields = make(map[string]interface{})
		}

		logs = append(logs, l)
	}

	return logs, rows.Err()
}

// SaveAppState saves application state (e.g., trading_enabled)
func (db *DB) SaveAppState(key, value string) error {

	_, err := db.db.Exec(`
		INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value)
	return err
}

// GetAppState retrieves application state by key
func (db *DB) GetAppState(key string) (string, error) {

	var value string
	err := db.db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Key not found, return empty string
	}
	return value, err
}
