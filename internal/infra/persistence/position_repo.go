package persistence

import (
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// PositionRepository implements domain.PositionRepository over *DB.
type PositionRepository struct{ db *DB }

// NewPositionRepository creates a PositionRepository backed by db.
func NewPositionRepository(db *DB) *PositionRepository { return &PositionRepository{db: db} }

// Compile-time check.
var _ domain.PositionRepository = (*PositionRepository)(nil)

// SavePosition inserts a new position record.
func (r *PositionRepository) SavePosition(position *domain.Position) error {
	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	position.UpdatedAt = now
	query := `INSERT INTO positions (symbol, side, size, used_size, remaining_size,
		  entry_price, current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.db.Exec(query, position.Symbol, position.Side,
		position.Size, position.UsedSize, position.RemainingSize,
		position.EntryPrice, position.CurrentPrice, position.UnrealizedPL, position.PnL,
		position.Status, position.OrderID, position.CreatedAt, position.UpdatedAt)
	return err
}

// GetOpenPositions returns OPEN positions for symbol+side with remaining_size > 0, ordered oldest first.
func (r *PositionRepository) GetOpenPositions(symbol string, side string) ([]domain.Position, error) {
	query := `SELECT id, symbol, side, size, used_size, remaining_size, entry_price,
			  current_price, unrealized_pl, status, order_id, created_at, updated_at
			  FROM positions
			  WHERE symbol = ? AND side = ? AND status = 'OPEN' AND remaining_size > 0
			  ORDER BY created_at ASC`
	rows, err := r.db.db.Query(query, symbol, side)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	positions := make([]domain.Position, 0, 20)
	for rows.Next() {
		var p domain.Position
		if err := rows.Scan(&p.ID, &p.Symbol, &p.Side, &p.Size, &p.UsedSize, &p.RemainingSize,
			&p.EntryPrice, &p.CurrentPrice, &p.UnrealizedPL, &p.Status, &p.OrderID,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	return positions, rows.Err()
}

// UpdatePosition updates a position by order_id.
func (r *PositionRepository) UpdatePosition(position *domain.Position) error {
	position.UpdatedAt = time.Now()
	query := `UPDATE positions
			  SET used_size = ?, remaining_size = ?, current_price = ?,
			      unrealized_pl = ?, status = ?, updated_at = ?
			  WHERE order_id = ?`
	_, err := r.db.db.Exec(query, position.UsedSize, position.RemainingSize,
		position.CurrentPrice, position.UnrealizedPL, position.Status,
		position.UpdatedAt, position.OrderID)
	return err
}

// GetActivePositions returns all positions with status='OPEN' and size != 0.
func (r *PositionRepository) GetActivePositions() ([]domain.Position, error) {
	query := `SELECT id, symbol, side, size, used_size, remaining_size, entry_price,
			  current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at
			  FROM positions WHERE size != 0 AND status = 'OPEN' ORDER BY created_at DESC`
	rows, err := r.db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	positions := make([]domain.Position, 0, 20)
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

// GetActivePositionsCount returns the count of OPEN positions.
func (r *PositionRepository) GetActivePositionsCount() (int, error) {
	var count int
	err := r.db.db.QueryRow("SELECT COUNT(*) FROM positions WHERE status = ?", "OPEN").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
