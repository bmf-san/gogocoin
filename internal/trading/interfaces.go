package trading

import (
	"context"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
)

// BalanceProvider provides balance information
type BalanceProvider interface {
	// GetBalance retrieves current balance
	GetBalance(ctx context.Context) ([]domain.Balance, error)

	// InvalidateBalanceCache invalidates the balance cache
	InvalidateBalanceCache()

	// UpdateBalanceToDB updates balance to database
	UpdateBalanceToDB(ctx context.Context)
}

// OrderExecutor executes trading orders
type OrderExecutor interface {
	// PlaceOrder places a new order
	PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error)

	// CancelOrder cancels an existing order
	CancelOrder(ctx context.Context, orderID string) error

	// GetOrders retrieves order list
	GetOrders(ctx context.Context) ([]*domain.OrderResult, error)
}

// OrderValidator validates orders before execution
type OrderValidator interface {
	// ValidateOrder validates an order request
	ValidateOrder(order *domain.OrderRequest) error

	// CheckBalance checks if balance is sufficient for the order
	CheckBalance(order *domain.OrderRequest, balances []domain.Balance, feeRate float64) error
}

// TradingService combines all trading capabilities
type TradingService interface {
	BalanceProvider
	OrderExecutor
}

// Trader is the complete trader interface
// Combines all trading capabilities for convenience
type Trader interface {
	// Order execution
	PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error)
	CancelOrder(ctx context.Context, orderID string) error
	GetOrders(ctx context.Context) ([]*domain.OrderResult, error)

	// Balance management
	GetBalance(ctx context.Context) ([]domain.Balance, error)
	InvalidateBalanceCache()
	UpdateBalanceToDB(ctx context.Context)

	// Lifecycle management
	Shutdown() error

	// Configuration
	SetOnOrderCompleted(fn func(*domain.OrderResult))
	SetStrategyName(name string)
}

// MarketSpecService provides market specifications
type MarketSpecService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
}
