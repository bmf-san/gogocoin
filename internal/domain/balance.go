package domain

import "time"

// Balance represents balance information
// Central model used by both trading and database packages
type Balance struct {
	ID        int       `json:"id,omitempty"` // Database ID (populated after persistence)
	Currency  string    `json:"currency"`
	Amount    float64   `json:"amount"`
	Available float64   `json:"available"`
	Timestamp time.Time `json:"timestamp"`
}
