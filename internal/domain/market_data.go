package domain

import "time"

// MarketData represents market data
type MarketData struct {
	Symbol      string    `json:"symbol"`
	ProductCode string    `json:"product_code"`
	Timestamp   time.Time `json:"timestamp"`
	Price       float64   `json:"price"`      // Current/last traded price
	Volume      float64   `json:"volume"`
	BestBid     float64   `json:"best_bid"`   // Best bid price
	BestAsk     float64   `json:"best_ask"`   // Best ask price
	Spread      float64   `json:"spread"`     // Bid-ask spread

	// OHLCV data
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
}
