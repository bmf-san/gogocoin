package app

import (
	"context"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// marketDataServiceAdapter adapts bitflyer.MarketDataService to app.MarketDataService interface
type marketDataServiceAdapter struct {
	underlying *bitflyer.MarketDataService
}

// newMarketDataServiceAdapter creates a new adapter
func newMarketDataServiceAdapter(svc *bitflyer.MarketDataService) MarketDataService {
	return &marketDataServiceAdapter{underlying: svc}
}

// SubscribeToTicker implements MarketDataService.SubscribeToTicker
func (a *marketDataServiceAdapter) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	// Adapt callback from domain.MarketData to bitflyer.MarketData
	bitflyerCallback := func(bfData bitflyer.MarketData) {
		domainData := domain.MarketData{
			Symbol:      bfData.Symbol,
			ProductCode: bfData.Symbol,
			Timestamp:   bfData.Timestamp,
			Price:       bfData.Price,
			Volume:      bfData.Volume,
			BestBid:     bfData.BestBid,
			BestAsk:     bfData.BestAsk,
			Spread:      bfData.Spread,
			Open:        bfData.Open,
			High:        bfData.High,
			Low:         bfData.Low,
			Close:       bfData.Close,
		}
		callback(domainData)
	}

	return a.underlying.SubscribeToTicker(ctx, symbol, bitflyerCallback)
}

// ResetCallbacks implements MarketDataService.ResetCallbacks
func (a *marketDataServiceAdapter) ResetCallbacks() {
	a.underlying.ResetCallbacks()
}
