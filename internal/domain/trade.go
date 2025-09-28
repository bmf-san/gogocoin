package domain

import "time"

// Trade represents trade information
type Trade struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	Type         string    `json:"type"`
	Size         float64   `json:"size"`
	Price        float64   `json:"price"`
	Fee          float64   `json:"fee"`
	Status       string    `json:"status"`
	OrderID      string    `json:"order_id"`
	ExecutedAt   time.Time `json:"executed_at"`
	CreatedAt    time.Time `json:"created_at"`
	StrategyName string    `json:"strategy_name"`
	PnL          float64   `json:"pnl"`
}
