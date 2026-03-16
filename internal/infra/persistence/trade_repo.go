package persistence

import (
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// TradeRepository implements domain.TradeRepository over *DB.
type TradeRepository struct{ db *DB }

// NewTradeRepository creates a TradeRepository backed by db.
func NewTradeRepository(db *DB) *TradeRepository { return &TradeRepository{db: db} }

// Compile-time check.
var _ domain.TradeRepository = (*TradeRepository)(nil)

// SaveTrade upserts a trade record (conflict on order_id updates key fields).
func (r *TradeRepository) SaveTrade(trade *domain.Trade) error {
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
	  strategy_name = excluded.strategy_name,
		  executed_at = excluded.executed_at,
		  updated_at = excluded.updated_at`
	_, err := r.db.db.Exec(query, trade.Symbol, trade.Side, trade.Type,
		trade.Size, trade.Price, trade.Fee, trade.Status, trade.OrderID,
		trade.ExecutedAt, trade.CreatedAt, now, trade.StrategyName, trade.PnL)
	return err
}

// GetRecentTrades returns the limit most-recent trades ordered by executed_at DESC.
func (r *TradeRepository) GetRecentTrades(limit int) ([]domain.Trade, error) {
	query := `SELECT id, symbol, side, type, size, price, fee, status, order_id,
			  executed_at, created_at, updated_at, strategy_name, pnl
			  FROM trades ORDER BY executed_at DESC LIMIT ?`
	rows, err := r.db.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	trades := make([]domain.Trade, 0, limit)
	for rows.Next() {
		var t domain.Trade
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Side, &t.Type, &t.Size, &t.Price, &t.Fee,
			&t.Status, &t.OrderID, &t.ExecutedAt, &t.CreatedAt,
			&t.UpdatedAt, &t.StrategyName, &t.PnL); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// GetTradesCount returns the total number of trade records.
func (r *TradeRepository) GetTradesCount() (int, error) {
	var count int
	err := r.db.db.QueryRow("SELECT COUNT(*) FROM trades").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
