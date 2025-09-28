package domain

import "time"

// Position represents position information
type Position struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	Size         float64   `json:"size"`
	EntryPrice   float64   `json:"entry_price"`
	CurrentPrice float64   `json:"current_price"`
	UnrealizedPL float64   `json:"unrealized_pl"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
