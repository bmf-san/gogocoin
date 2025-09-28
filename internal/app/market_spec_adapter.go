package app

import "github.com/bmf-san/gogocoin/v1/internal/bitflyer"

// marketSpecServiceAdapter adapts bitflyer.MarketSpecificationService to app.MarketSpecService interface
type marketSpecServiceAdapter struct {
	underlying *bitflyer.MarketSpecificationService
}

// newMarketSpecServiceAdapter creates a new adapter
func newMarketSpecServiceAdapter(svc *bitflyer.MarketSpecificationService) MarketSpecService {
	return &marketSpecServiceAdapter{underlying: svc}
}

// GetMinimumOrderSize implements MarketSpecService.GetMinimumOrderSize
func (a *marketSpecServiceAdapter) GetMinimumOrderSize(symbol string) (float64, error) {
	return a.underlying.GetMinimumOrderSize(symbol)
}

// GetUnderlying returns the underlying bitflyer service (for backwards compatibility)
func (a *marketSpecServiceAdapter) GetUnderlying() *bitflyer.MarketSpecificationService {
	return a.underlying
}
