package domain

// MarketSpecService provides exchange-specific market specifications.
// This interface is defined in domain/ for use across multiple packages
// (usecase/trading, usecase/strategy, etc.) without circular dependencies.
type MarketSpecService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}
