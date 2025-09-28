package domain

import "time"

// OrderRequest represents an order request
type OrderRequest struct {
	Symbol         string  `json:"symbol"`           // Trading pair (e.g., BTC_JPY)
	Side           string  `json:"side"`             // BUY or SELL
	Type           string  `json:"type"`             // MARKET or LIMIT
	Size           float64 `json:"size"`             // Order quantity
	Price          float64 `json:"price"`            // Price (for LIMIT orders)
	TimeInForce    string  `json:"time_in_force"`    // GTC, IOC, FOK
	MinuteToExpire int32   `json:"minute_to_expire"` // Expiration time (minutes)
}

// OrderResult represents an order result
type OrderResult struct {
	OrderID         string    `json:"order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Type            string    `json:"type"`
	Size            float64   `json:"size"`
	Price           float64   `json:"price"`
	Status          string    `json:"status"`
	FilledSize      float64   `json:"filled_size"`
	RemainingSize   float64   `json:"remaining_size"`
	AveragePrice    float64   `json:"average_price"`
	TotalCommission float64   `json:"total_commission"`
	Fee             float64   `json:"fee"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
