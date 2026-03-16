package persistence

import (
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
)

// BalanceRepository implements domain.BalanceRepository over *DB.
type BalanceRepository struct{ db *DB }

// NewBalanceRepository creates a BalanceRepository backed by db.
func NewBalanceRepository(db *DB) *BalanceRepository { return &BalanceRepository{db: db} }

// Compile-time check.
var _ domain.BalanceRepository = (*BalanceRepository)(nil)

// SaveBalance inserts a balance snapshot.
func (r *BalanceRepository) SaveBalance(balance domain.Balance) error {
	query := `INSERT INTO balances (currency, available, amount, timestamp)
			  VALUES (?, ?, ?, ?)`
	_, err := r.db.db.Exec(query, balance.Currency, balance.Available, balance.Amount, time.Now())
	return err
}

// GetLatestBalances returns the latest balance row per currency (MAX(id) GROUP BY currency).
func (r *BalanceRepository) GetLatestBalances() ([]domain.Balance, error) {
	query := `SELECT id, currency, available, amount, timestamp FROM balances
			  WHERE id IN (
			      SELECT MAX(id) FROM balances GROUP BY currency
			  ) ORDER BY currency`
	rows, err := r.db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	balances := make([]domain.Balance, 0, 10)
	for rows.Next() {
		var b domain.Balance
		if err := rows.Scan(&b.ID, &b.Currency, &b.Available, &b.Amount, &b.Timestamp); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, rows.Err()
}
