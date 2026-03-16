package strategy

import "time"

// Signal represents a trading signal emitted by a Strategy.
type Signal struct {
	Symbol    string                 `json:"symbol"`
	Action    SignalAction           `json:"action"`
	Strength  float64                `json:"strength"` // 0.0–1.0
	Price     float64                `json:"price"`
	Quantity  float64                `json:"quantity"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// SignalAction is the trading direction of a Signal.
type SignalAction string

const (
	SignalBuy  SignalAction = "BUY"
	SignalSell SignalAction = "SELL"
	SignalHold SignalAction = "HOLD"
)
