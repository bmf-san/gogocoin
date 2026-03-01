package persistence

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// DBTx wraps a *sql.Tx and implements domain.Transaction.
type DBTx struct {
	tx     *sql.Tx
	logger logger.LoggerInterface
}

// Compile-time check: DBTx satisfies domain.Transaction.
var _ domain.Transaction = (*DBTx)(nil)

// Commit commits the transaction.
func (t *DBTx) Commit() error { return t.tx.Commit() }

// Rollback rolls back the transaction.
func (t *DBTx) Rollback() error { return t.tx.Rollback() }

// SaveTrade saves a trade within the transaction.
func (t *DBTx) SaveTrade(trade *domain.Trade) error {
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
	_, err := t.tx.Exec(query, trade.Symbol, trade.Side, trade.Type,
		trade.Size, trade.Price, trade.Fee, trade.Status, trade.OrderID,
		trade.ExecutedAt, trade.CreatedAt, now, trade.StrategyName, trade.PnL)
	return err
}

// SavePosition saves a position within the transaction.
func (t *DBTx) SavePosition(position *domain.Position) error {
	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	position.UpdatedAt = now
	query := `INSERT INTO positions (symbol, side, size, used_size, remaining_size,
		  entry_price, current_price, unrealized_pl, pnl, status, order_id, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := t.tx.Exec(query, position.Symbol, position.Side,
		position.Size, position.UsedSize, position.RemainingSize,
		position.EntryPrice, position.CurrentPrice, position.UnrealizedPL, position.PnL,
		position.Status, position.OrderID, position.CreatedAt, position.UpdatedAt)
	return err
}

// UpdatePosition updates a position within the transaction.
func (t *DBTx) UpdatePosition(position *domain.Position) error {
	position.UpdatedAt = time.Now()
	query := `UPDATE positions SET used_size = ?, remaining_size = ?, current_price = ?,
		  unrealized_pl = ?, status = ?, updated_at = ? WHERE order_id = ?`
	_, err := t.tx.Exec(query, position.UsedSize, position.RemainingSize,
		position.CurrentPrice, position.UnrealizedPL, position.Status,
		position.UpdatedAt, position.OrderID)
	return err
}

// TxFunc is a function that runs within a transaction.
type TxFunc func(tx domain.Transaction) error

// WithTransaction executes fn inside a transaction.
// Automatically rolls back on error or panic; commits on success.
func WithTransaction(db domain.TransactionManager, log logger.LoggerInterface, fn TxFunc) error {
	tx, err := db.BeginTx()
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if r := recover(); r != nil {
			if !committed {
				if rbErr := tx.Rollback(); rbErr != nil && log != nil {
					log.System().WithError(rbErr).Error("Failed to rollback after panic")
				}
			}
			panic(r)
		}
	}()

	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && log != nil {
			log.System().WithError(rbErr).Error("Failed to rollback after error")
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && log != nil {
			log.System().WithError(rbErr).Error(fmt.Sprintf("Failed to rollback after commit failure: %v", rbErr))
		}
		return err
	}
	committed = true
	return nil
}
