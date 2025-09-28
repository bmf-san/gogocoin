package models

import (
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// PositionModel represents the database persistence model for positions
type PositionModel struct {
	ID           int       `db:"id"`
	Symbol       string    `db:"symbol"`
	Side         string    `db:"side"`
	EntryPrice   float64   `db:"entry_price"`
	CurrentPrice float64   `db:"current_price"`
	Size         float64   `db:"size"`
	UnrealizedPL float64   `db:"unrealized_pl"`
	RealizedPL   float64   `db:"realized_pl"`
	Status       string    `db:"status"`
	Strategy     string    `db:"strategy"`
	OpenedAt     time.Time `db:"opened_at"`
	ClosedAt     *time.Time `db:"closed_at"` // Nullable
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// ToDomain converts PositionModel to domain.Position
func (m *PositionModel) ToDomain() domain.Position {
	return domain.Position{
		ID:           m.ID,
		Symbol:       m.Symbol,
		ProductCode:  m.Symbol,
		Side:         m.Side,
		EntryPrice:   m.EntryPrice,
		CurrentPrice: m.CurrentPrice,
		Price:        m.EntryPrice,
		Size:         m.Size,
		UnrealizedPL: m.UnrealizedPL,
		PnL:          m.RealizedPL,
		Status:       m.Status,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// FromDomainPosition creates a PositionModel from domain.Position
func FromDomainPosition(p domain.Position) *PositionModel {
	now := time.Now()
	model := &PositionModel{
		ID:           p.ID,
		Symbol:       p.Symbol,
		Side:         p.Side,
		EntryPrice:   p.EntryPrice,
		CurrentPrice: p.CurrentPrice,
		Size:         p.Size,
		UnrealizedPL: p.UnrealizedPL,
		RealizedPL:   p.PnL,
		Status:       p.Status,
		Strategy:     "", // Not in domain model
		OpenedAt:     p.CreatedAt,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
	if p.Status == "CLOSED" {
		model.ClosedAt = &now
	}
	return model
}
