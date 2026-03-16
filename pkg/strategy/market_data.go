package strategy

import "time"

// MarketData represents a single market data point received from an exchange.
// This is a standalone definition so external consumers do not need to import
// the internal domain package.
type MarketData struct {
	Symbol      string
	ProductCode string
	Timestamp   time.Time
	Price       float64
	Volume      float64
	BestBid     float64
	BestAsk     float64
	Spread      float64
	Open        float64
	High        float64
	Low         float64
	Close       float64
}
