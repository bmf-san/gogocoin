package persistence

import (
	"strings"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// MarketDataRepository implements domain.MarketDataRepository over *DB.
type MarketDataRepository struct{ db *DB }

// NewMarketDataRepository creates a MarketDataRepository backed by db.
func NewMarketDataRepository(db *DB) *MarketDataRepository {
	return &MarketDataRepository{db: db}
}

// Compile-time check.
var _ domain.MarketDataRepository = (*MarketDataRepository)(nil)

// SaveMarketData upserts a market data row (conflict on symbol+timestamp).
func (r *MarketDataRepository) SaveMarketData(data *domain.MarketData) error {
	query := `INSERT OR REPLACE INTO market_data (symbol, timestamp, open, high, low, close, volume, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.db.Exec(query, data.Symbol, data.Timestamp,
		data.Open, data.High, data.Low, data.Close, data.Volume, time.Now())
	return err
}

// SaveTicker is an alias for SaveMarketData (backward compatibility).
func (r *MarketDataRepository) SaveTicker(ticker *domain.MarketData) error {
	return r.SaveMarketData(ticker)
}

// GetLatestMarketData returns the limit most-recent rows for the given symbol.
func (r *MarketDataRepository) GetLatestMarketData(symbol string, limit int) ([]domain.MarketData, error) {
	query := `SELECT symbol, timestamp, open, high, low, close, volume
			  FROM market_data WHERE symbol = ? ORDER BY timestamp DESC LIMIT ?`
	rows, err := r.db.db.Query(query, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	data := make([]domain.MarketData, 0, limit)
	for rows.Next() {
		var d domain.MarketData
		if err := rows.Scan(&d.Symbol, &d.Timestamp, &d.Open, &d.High, &d.Low, &d.Close, &d.Volume); err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, rows.Err()
}

// GetLatestMarketDataForSymbols returns the latest row per symbol for all given symbols.
func (r *MarketDataRepository) GetLatestMarketDataForSymbols(symbols []string) (map[string]domain.MarketData, error) {
	if len(symbols) == 0 {
		return make(map[string]domain.MarketData), nil
	}
	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols))
	for i, s := range symbols {
		placeholders[i] = "?"
		args[i] = s
	}
	query := `SELECT m.symbol, m.timestamp, m.open, m.high, m.low, m.close, m.volume
			  FROM market_data m
			  INNER JOIN (
			      SELECT symbol, MAX(timestamp) as max_timestamp
			      FROM market_data
			      WHERE symbol IN (` + strings.Join(placeholders, ",") + `)
			      GROUP BY symbol
			  ) latest ON m.symbol = latest.symbol AND m.timestamp = latest.max_timestamp`
	rows, err := r.db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	result := make(map[string]domain.MarketData, len(symbols))
	for rows.Next() {
		var d domain.MarketData
		if err := rows.Scan(&d.Symbol, &d.Timestamp, &d.Open, &d.High, &d.Low, &d.Close, &d.Volume); err != nil {
			return nil, err
		}
		result[d.Symbol] = d
	}
	return result, rows.Err()
}
