package domain

import "time"

// Trade represents trade information
type Trade struct {
	ID           int                    `json:"id,omitempty"`      // Database ID (populated after persistence)
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
	UpdatedAt    time.Time              `json:"updated_at,omitempty"`
	StrategyName string                 `json:"strategy_name"`
	Strategy     string                 `json:"strategy"`
	PnL          float64                `json:"pnl"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
