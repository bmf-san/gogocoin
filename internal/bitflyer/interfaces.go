package bitflyer

import (
	"context"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// Client is the interface for the bitFlyer API client.
// Following the Consumer-Driven Contracts pattern, only the minimal set
// of methods used by consumers are defined, making it easier
// to implement mocks during testing.
type ClientInterface interface {
	IsConnected() bool
	Close(ctx context.Context) error
}

// MarketDataServiceInterface is the interface for market data service.
// Defines the minimal set of methods needed to retrieve market data via WebSocket.
type MarketDataServiceInterface interface {
	SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error
	ResetCallbacks() // Reset callbacks to prevent leaks on WebSocket reconnection
}

// MarketSpecServiceInterface is the interface for market specification service.
// Provides minimum order sizes and other market specifications.
type MarketSpecServiceInterface interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}
