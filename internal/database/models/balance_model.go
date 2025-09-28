package models

import (
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// BalanceModel represents the database persistence model for balance
type BalanceModel struct {
	ID        int       `db:"id"`
	Currency  string    `db:"currency"`
	Amount    float64   `db:"amount"`
	Available float64   `db:"available"`
	Reserved  float64   `db:"reserved"`
	Timestamp time.Time `db:"timestamp"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ToDomain converts BalanceModel to domain.Balance
func (m *BalanceModel) ToDomain() domain.Balance {
	return domain.Balance{
		Currency:  m.Currency,
		Amount:    m.Amount,
		Available: m.Available,
		Timestamp: m.Timestamp,
	}
}

// FromDomainBalance creates a BalanceModel from domain.Balance
func FromDomainBalance(b domain.Balance) *BalanceModel {
	now := time.Now()
	return &BalanceModel{
		Currency:  b.Currency,
		Amount:    b.Amount,
		Available: b.Available,
		Reserved:  0, // Can be calculated if needed
		Timestamp: b.Timestamp,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
