package trading

import (
	"context"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// Trader is the trading execution interface
// Abstracts Paper/Live mode implementations
type Trader interface {
	// PlaceOrder executes an order
	PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error)

	// CancelOrder cancels an order
	CancelOrder(ctx context.Context, orderID string) error

	// GetBalance retrieves balance information
	GetBalance(ctx context.Context) ([]domain.Balance, error)

	// GetOrders retrieves the list of orders
	GetOrders(ctx context.Context) ([]*domain.OrderResult, error)

	// SetDatabase sets the database saver interface
	SetDatabase(db DatabaseSaver)

	// SetStrategyName sets the strategy name
	SetStrategyName(name string)
}

// DatabaseSaver is the database save interface
type DatabaseSaver interface {
	SaveTrade(trade *domain.Trade) error
	SavePosition(position *domain.Position) error
	SaveBalance(balance domain.Balance) error
	GetRecentTrades(limit int) ([]domain.Trade, error)
}
