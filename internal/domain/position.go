package domain

import "time"

// Position represents position information with tracking for partial fills
type Position struct {
	ID            int       `json:"id,omitempty"`       // Database ID (populated after persistence)
	Symbol        string    `json:"symbol"`
	ProductCode   string    `json:"product_code"`
	Side          string    `json:"side"`
	Size          float64   `json:"size"`           // Total position size
	UsedSize      float64   `json:"used_size"`      // Size already matched/closed
	RemainingSize float64   `json:"remaining_size"` // Size available for matching
	EntryPrice    float64   `json:"entry_price"`
	CurrentPrice  float64   `json:"current_price"`
	UnrealizedPL  float64   `json:"unrealized_pl"`
	PnL           float64   `json:"pnl"` // Realized PnL
	Status        string    `json:"status"`   // OPEN, PARTIAL, CLOSED
	OrderID       string    `json:"order_id"` // Original order ID
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdateStatus updates the position status based on remaining size
// Business logic: CLOSED if fully matched, PARTIAL if partially matched, OPEN otherwise
func (p *Position) UpdateStatus() {
	if p.RemainingSize <= 0 {
		p.Status = "CLOSED"
	} else if p.UsedSize > 0 {
		p.Status = "PARTIAL"
	} else {
		p.Status = "OPEN"
	}
}
