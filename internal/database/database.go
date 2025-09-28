package database

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

// DB is a SQLite database
type DB struct {
	db     *sql.DB
	logger *logger.Logger
	mu     sync.RWMutex
}

// Balance represents balance information
type Balance struct {
	ID        int       `json:"id"`
	Currency  string    `json:"currency"`
	Available float64   `json:"available"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

// Trade represents trading information
type Trade struct {
	ID           int                    `json:"id"`
	Symbol       string                 `json:"symbol"`
	ProductCode  string                 `json:"product_code"`
	Side         string                 `json:"side"`
	Type         string                 `json:"type"`
	Amount       float64                `json:"amount"`
	Size         float64                `json:"size"`
	Price        float64                `json:"price"`
	Fee          float64                `json:"fee"`
	Status       string                 `json:"status"`
	OrderID      string                 `json:"order_id"`
	ExecutedAt   time.Time              `json:"executed_at"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	StrategyName string                 `json:"strategy_name"`
	Strategy     string                 `json:"strategy"`
	PnL          float64                `json:"pnl"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Position represents position information
type Position struct {
	ID           int       `json:"id"`
	Symbol       string    `json:"symbol"`
	ProductCode  string    `json:"product_code"`
	Side         string    `json:"side"`
	Size         float64   `json:"size"`
	EntryPrice   float64   `json:"entry_price"`
	CurrentPrice float64   `json:"current_price"`
	Price        float64   `json:"price"`
	UnrealizedPL float64   `json:"unrealized_pl"`
	PnL          float64   `json:"pnl"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PerformanceMetric isperformancemetrics
type PerformanceMetric struct {
	ID              int       `json:"id"`
	Date            time.Time `json:"date"`
	TotalReturn     float64   `json:"total_return"`
	DailyReturn     float64   `json:"daily_return"`
	WinRate         float64   `json:"win_rate"`
	MaxDrawdown     float64   `json:"max_drawdown"`
	SharpeRatio     float64   `json:"sharpe_ratio"`
	ProfitFactor    float64   `json:"profit_factor"`
	TotalTrades     int       `json:"total_trades"`
	WinningTrades   int       `json:"winning_trades"`
	LosingTrades    int       `json:"losing_trades"`
	AverageWin      float64   `json:"average_win"`
	AverageLoss     float64   `json:"average_loss"`
	LargestWin      float64   `json:"largest_win"`
	LargestLoss     float64   `json:"largest_loss"`
	ConsecutiveWins int       `json:"consecutive_wins"`
	ConsecutiveLoss int       `json:"consecutive_loss"`
	TotalPnL        float64   `json:"total_pnl"`
}

// MarketData ismarket data（OHLCV）
type MarketData struct {
	ID          int       `json:"id"`
	Symbol      string    `json:"symbol"`
	ProductCode string    `json:"product_code"`
	Timestamp   time.Time `json:"timestamp"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	CreatedAt   time.Time `json:"created_at"`
}

// PerformanceMetrics is an alias for PerformanceMetric (for test compatibility)
type PerformanceMetrics = PerformanceMetric

// LogEntry represents a log entry
type LogEntry struct {
	ID        int       `json:"id"`
	Level     string    `json:"level"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
	Fields    string    `json:"fields"` // saved as JSON string
	Timestamp time.Time `json:"timestamp"`
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

	dbInstance := &DB{
		db:     db,
		logger: logger,
	}

	// Create tables
	if err := dbInstance.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return dbInstance, nil
}

// Close isdatabasecloses
func (db *DB) Close() error {
	return db.db.Close()
}

// createTables creates tables
func (db *DB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS balances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL,
			available REAL NOT NULL,
			amount REAL NOT NULL,
			timestamp DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			product_code TEXT NOT NULL,
			side TEXT NOT NULL,
			type TEXT NOT NULL,
			amount REAL NOT NULL,
			size REAL NOT NULL,
			price REAL NOT NULL,
			fee REAL NOT NULL,
			status TEXT NOT NULL,
			order_id TEXT,
			executed_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			strategy_name TEXT,
			strategy TEXT,
			pnl REAL DEFAULT 0,
			metadata TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			product_code TEXT NOT NULL,
			side TEXT NOT NULL,
			size REAL NOT NULL,
			entry_price REAL NOT NULL,
			current_price REAL NOT NULL,
			price REAL NOT NULL,
			unrealized_pl REAL DEFAULT 0,
			pnl REAL DEFAULT 0,
			status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS performance_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATETIME NOT NULL,
			total_return REAL DEFAULT 0,
			daily_return REAL DEFAULT 0,
			win_rate REAL DEFAULT 0,
			max_drawdown REAL DEFAULT 0,
			sharpe_ratio REAL DEFAULT 0,
			profit_factor REAL DEFAULT 0,
			total_trades INTEGER DEFAULT 0,
			winning_trades INTEGER DEFAULT 0,
			losing_trades INTEGER DEFAULT 0,
			average_win REAL DEFAULT 0,
			average_loss REAL DEFAULT 0,
			largest_win REAL DEFAULT 0,
			largest_loss REAL DEFAULT 0,
			consecutive_wins INTEGER DEFAULT 0,
			consecutive_loss INTEGER DEFAULT 0,
			total_pnl REAL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS market_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			product_code TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			open REAL NOT NULL,
			high REAL NOT NULL,
			low REAL NOT NULL,
			close REAL NOT NULL,
			volume REAL NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE(symbol, timestamp)
		)`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
			category TEXT NOT NULL,
			message TEXT NOT NULL,
			fields TEXT,
			timestamp DATETIME NOT NULL
		)`,
	}

	for _, query := range queries {
		if _, err := db.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// SaveBalance saves balance information
func (db *DB) SaveBalance(balance domain.Balance) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO balances (currency, available, amount, timestamp)
			  VALUES (?, ?, ?, ?)`

	_, err := db.db.Exec(query, balance.Currency, balance.Available, balance.Amount, time.Now())
	return err
}

// GetLatestBalances gets the latest balance list
func (db *DB) GetLatestBalances() ([]Balance, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

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

	var balances []Balance
	for rows.Next() {
		var b Balance
		if err := rows.Scan(&b.ID, &b.Currency, &b.Available, &b.Amount, &b.Timestamp); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}

	return balances, rows.Err()
}

// SaveTrade saves a trade
func (db *DB) SaveTrade(trade *domain.Trade) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO trades (symbol, product_code, side, type, amount, size, price, fee,
		  status, order_id, executed_at, created_at, updated_at, strategy_name, pnl)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if trade.ExecutedAt.IsZero() {
		trade.ExecutedAt = now
	}
	if trade.CreatedAt.IsZero() {
		trade.CreatedAt = now
	}

	_, err := db.db.Exec(query, trade.Symbol, trade.Symbol, trade.Side, trade.Type,
		trade.Size, trade.Size, trade.Price, trade.Fee, trade.Status, trade.OrderID,
		trade.ExecutedAt, trade.CreatedAt, now, trade.StrategyName, trade.PnL)

	return err
}

// GetTrades istradinghistoryget
func (db *DB) GetTrades(limit int) ([]domain.Trade, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

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

	var trades []domain.Trade
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
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO positions (symbol, product_code, side, size, entry_price,
		  current_price, price, unrealized_pl, pnl, status, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	position.UpdatedAt = now

	_, err := db.db.Exec(query, position.Symbol, position.Symbol, position.Side,
		position.Size, position.EntryPrice, position.CurrentPrice, position.EntryPrice,
		position.UnrealizedPL, position.UnrealizedPL, position.Status, position.CreatedAt, position.UpdatedAt)

	return err
}

// GetPositions gets the position list
func (db *DB) GetPositions() ([]Position, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, symbol, product_code, side, size, entry_price, current_price,
			  price, unrealized_pl, pnl, status, created_at, updated_at
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

	var positions []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.Symbol, &p.ProductCode, &p.Side, &p.Size,
			&p.EntryPrice, &p.CurrentPrice, &p.Price, &p.UnrealizedPL, &p.PnL,
			&p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

// GetActivePositions gets the active position list
func (db *DB) GetActivePositions() ([]Position, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, symbol, product_code, side, size, entry_price, current_price,
			  price, unrealized_pl, pnl, status, created_at, updated_at
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

	var positions []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.Symbol, &p.ProductCode, &p.Side, &p.Size,
			&p.EntryPrice, &p.CurrentPrice, &p.Price, &p.UnrealizedPL, &p.PnL,
			&p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}

	return positions, rows.Err()
}

// SavePerformanceMetric saves performance metrics
func (db *DB) SavePerformanceMetric(metric *PerformanceMetric) error {
	db.mu.Lock()
	defer db.mu.Unlock()

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

// SavePerformanceMetrics saves performance metrics (alias)
func (db *DB) SavePerformanceMetrics(metric *PerformanceMetric) error {
	return db.SavePerformanceMetric(metric)
}

// GetPerformanceMetrics isperformancemetricsget
func (db *DB) GetPerformanceMetrics(days int) ([]PerformanceMetric, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

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

	var metrics []PerformanceMetric
	for rows.Next() {
		var m PerformanceMetric
		if err := rows.Scan(&m.ID, &m.Date, &m.TotalReturn, &m.DailyReturn, &m.WinRate,
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
func (db *DB) SaveMarketData(data *MarketData) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT OR REPLACE INTO market_data (symbol, product_code, timestamp, open, high, low, close, volume, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}

	_, err := db.db.Exec(query, data.Symbol, data.ProductCode, data.Timestamp,
		data.Open, data.High, data.Low, data.Close, data.Volume, data.CreatedAt)

	return err
}

// GetHistoricalMarketData gets market data for the specified period
func (db *DB) GetHistoricalMarketData(symbol string, startDate, endDate time.Time) ([]MarketData, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, symbol, product_code, timestamp, open, high, low, close, volume, created_at
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

	var data []MarketData
	for rows.Next() {
		var d MarketData
		if err := rows.Scan(&d.ID, &d.Symbol, &d.ProductCode, &d.Timestamp,
			&d.Open, &d.High, &d.Low, &d.Close, &d.Volume, &d.CreatedAt); err != nil {
			return nil, err
		}
		data = append(data, d)
	}

	return data, rows.Err()
}

// GetLatestMarketData gets the latest market data
func (db *DB) GetLatestMarketData(symbol string, limit int) ([]MarketData, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT id, symbol, product_code, timestamp, open, high, low, close, volume, created_at
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

	var data []MarketData
	for rows.Next() {
		var d MarketData
		if err := rows.Scan(&d.ID, &d.Symbol, &d.ProductCode, &d.Timestamp,
			&d.Open, &d.High, &d.Low, &d.Close, &d.Volume, &d.CreatedAt); err != nil {
			return nil, err
		}
		data = append(data, d)
	}

	return data, rows.Err()
}

// SaveLogEntry saves a log entry (internal use)
func (db *DB) saveLogEntry(entry *LogEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO logs (level, category, message, fields, timestamp)
			  VALUES (?, ?, ?, ?, ?)`

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	_, err := db.db.Exec(query, entry.Level, entry.Category, entry.Message, entry.Fields, entry.Timestamp)
	return err
}

// SaveLog receives logger.LogEntry and saves the log entry
func (db *DB) SaveLog(entry interface{}) error {
	// logger.LogEntry" "database.LogEntryに変換
	dbEntry := db.convertToLogEntry(entry)
	return db.saveLogEntry(&dbEntry)
}

// convertToLogEntry isinterface{}" "LogEntryに変換
func (db *DB) convertToLogEntry(entry interface{}) LogEntry {
	// リフレクション" "使用してフィールドget
	v := reflect.ValueOf(entry)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	var level, category, message, fields string
	var timestamp time.Time

	if v.Kind() == reflect.Struct {
		// Level フィールドget
		if levelField := v.FieldByName("Level"); levelField.IsValid() && levelField.Kind() == reflect.String {
			level = levelField.String()
		}

		// Category フィールドget
		if categoryField := v.FieldByName("Category"); categoryField.IsValid() && categoryField.Kind() == reflect.String {
			category = categoryField.String()
		}

		// Message フィールドget
		if messageField := v.FieldByName("Message"); messageField.IsValid() && messageField.Kind() == reflect.String {
			message = messageField.String()
		}

		// Fields フィールドget
		if fieldsField := v.FieldByName("Fields"); fieldsField.IsValid() && fieldsField.Kind() == reflect.String {
			fields = fieldsField.String()
		}

		// Timestamp フィールドget
		if timestampField := v.FieldByName("Timestamp"); timestampField.IsValid() {
			if timestampField.Type() == reflect.TypeOf(time.Time{}) {
				timestamp = timestampField.Interface().(time.Time)
			}
		}
	}

	// default値ofconfiguration
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

	return LogEntry{
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    fields,
		Timestamp: timestamp,
	}
}

// GetLogs islogentryget
func (db *DB) GetLogs(limit int) ([]LogEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

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

	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		if err := rows.Scan(&l.ID, &l.Level, &l.Category, &l.Message, &l.Fields, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}

// GetRecentTrades is最近oftradinghistoryget
func (db *DB) GetRecentTrades(limit int) ([]domain.Trade, error) {
	return db.GetTrades(limit)
}

// GetRecentLogs is最近oflogentryget
func (db *DB) GetRecentLogs(limit int) ([]LogEntry, error) {
	return db.GetLogs(limit)
}

// GetRecentLogsWithLevel isレベルフィルタ付きwith最近oflogentryget
// Deprecated: Use GetRecentLogsWithFilters instead
func (db *DB) GetRecentLogsWithLevel(limit int, level string) ([]LogEntry, error) {
	return db.GetRecentLogsWithFilters(limit, level, "")
}

// GetRecentLogsWithFilters isレベルとカテゴリフィルタ付きwith最近oflogentryget
func (db *DB) GetRecentLogsWithFilters(limit int, level, category string) ([]LogEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

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

	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		if err := rows.Scan(&l.ID, &l.Level, &l.Category, &l.Message, &l.Fields, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}
