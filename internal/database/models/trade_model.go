package models

import (
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// TradeModel represents the database persistence model for trades
// Separated from domain.Trade to isolate database concerns
type TradeModel struct {
	ID              int       `db:"id"`
	OrderID         string    `db:"order_id"`
	Symbol          string    `db:"symbol"`
	Side            string    `db:"side"`
	Amount          float64   `db:"amount"`
	Size            float64   `db:"size"`
	Price           float64   `db:"price"`
	Fee             float64   `db:"fee"`
	PnL             float64   `db:"pnl"`
	Status          string    `db:"status"`
	Strategy        string    `db:"strategy"`
	ExecutedAt      time.Time `db:"executed_at"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	MetadataJSON    string    `db:"metadata"` // JSON serialized metadata
}

// ToDomain converts TradeModel to domain.Trade
func (m *TradeModel) ToDomain() domain.Trade {
	return domain.Trade{
		ID:         m.ID,
		OrderID:    m.OrderID,
		Symbol:     m.Symbol,
		Side:       m.Side,
		Amount:     m.Amount,
		Size:       m.Size,
		Price:      m.Price,
		Fee:        m.Fee,
		PnL:        m.PnL,
		Status:     m.Status,
		Strategy:   m.Strategy,
		ExecutedAt: m.ExecutedAt,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		// Metadata parsing would happen in a mapper
	}
}

// FromDomain creates a TradeModel from domain.Trade
func FromDomainTrade(t domain.Trade) *TradeModel {
	return &TradeModel{
		ID:         t.ID,
		OrderID:    t.OrderID,
		Symbol:     t.Symbol,
		Side:       t.Side,
		Amount:     t.Amount,
		Size:       t.Size,
		Price:      t.Price,
		Fee:        t.Fee,
		PnL:        t.PnL,
		Status:     t.Status,
		Strategy:   t.Strategy,
		ExecutedAt: t.ExecutedAt,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
		// Metadata serialization would happen in a mapper
	}
}
