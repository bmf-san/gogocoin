package trading

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/logger"
	"github.com/bmf-san/gogocoin/internal/usecase/trading/balance"
	"github.com/bmf-san/gogocoin/internal/usecase/trading/monitor"
	"github.com/bmf-san/gogocoin/internal/usecase/trading/order"
	"github.com/bmf-san/gogocoin/internal/usecase/trading/pnl"
	"github.com/bmf-san/gogocoin/internal/usecase/trading/validator"
)

// Trader is the complete trader interface used by consumers of this package.
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

// BitflyerTrader is a Facade that delegates to specialized services
type BitflyerTrader struct {
	// Delegated services
	orderValidator *validator.OrderValidator
	balanceService *balance.BalanceService
	orderService   *order.OrderService
	orderMonitor   *monitor.OrderMonitor
	pnlCalculator  *pnl.Calculator

	// Collaborating dependencies
	client             *bitflyer.Client
	logger             logger.LoggerInterface
	db                 domain.TradingRepository
	strategyName       string
	onOrderCompletedFn func(*domain.OrderResult)
	mu                 sync.RWMutex

	// Shutdown management
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	wg             sync.WaitGroup
}

// NewTraderWithDependencies creates a new BitflyerTrader with all dependencies injected
func NewTraderWithDependencies(
	client *bitflyer.Client,
	logger logger.LoggerInterface,
	db domain.TradingRepository,
	marketSpecSvc domain.MarketSpecService,
	strategyName string,
	symbol string,
) Trader {
	// Initialize services in dependency order
	balanceSvc := balance.NewBalanceService(client, db, logger)
	orderSvc := order.NewOrderService(client, logger, symbol)
	orderValidator := validator.NewOrderValidator(marketSpecSvc, logger)
	pnlCalc := pnl.NewCalculator(db, logger, strategyName)

	ctx, cancel := context.WithCancel(context.Background())
	t := &BitflyerTrader{
		orderValidator: orderValidator,
		balanceService: balanceSvc,
		orderService:   orderSvc,
		pnlCalculator:  pnlCalc,
		client:         client,
		logger:         logger,
		db:             db,
		strategyName:   strategyName,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}

	// Initialize OrderMonitor (requires all dependencies)
	t.orderMonitor = monitor.NewOrderMonitor(logger, t, t, pnlCalc)

	return t
}

// PlaceOrder executes an order (delegates to services)
func (t *BitflyerTrader) PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error) {
	// 1. Validate order
	if err := t.orderValidator.ValidateOrder(order); err != nil {
		return nil, fmt.Errorf("invalid order: %w", err)
	}

	// 2. Check balance
	balances, err := t.balanceService.GetBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	feeRate := t.client.GetFeeRate()
	if err := t.orderValidator.CheckBalance(order, balances, feeRate); err != nil {
		return nil, fmt.Errorf("insufficient balance: %w", err)
	}

	// 3. Execute order
	result, err := t.orderService.PlaceOrder(ctx, order)
	if err != nil {
		return nil, err
	}

	// 4. Invalidate balance cache after successful order
	t.balanceService.InvalidateCache()

	// 5. Log trade (Facade responsibility)
	t.logger.LogTrade("ORDER_PLACED", order.Symbol, order.Price, order.Size, map[string]interface{}{
		"order_id": result.OrderID,
		"side":     order.Side,
		"type":     order.Type,
	})

	// 6. Start order monitoring (goroutine management is Facade responsibility)
	//
	// Guard against the WaitGroup panic: if Shutdown() has already called
	// shutdownCancel() and its wg.Wait() goroutine is concurrently blocked with
	// a counter of zero, calling wg.Add(1) here would trigger
	// "sync: WaitGroup misuse: Add called concurrently with Wait".
	// Skipping monitoring during shutdown is safe — the process is exiting.
	t.mu.Lock()
	orderMonitor := t.orderMonitor
	shutdownCtx := t.shutdownCtx
	if orderMonitor != nil && shutdownCtx.Err() == nil {
		t.wg.Add(1)
	} else {
		orderMonitor = nil // skip monitoring if shutting down
	}
	t.mu.Unlock()

	if orderMonitor != nil {
		monitorCtx, cancel := context.WithTimeout(shutdownCtx, monitor.OrderMonitoringTimeout)
		go func() {
			defer cancel()
			defer t.wg.Done()
			orderMonitor.MonitorExecution(monitorCtx, result)
		}()
	}

	return result, nil
}

// GetBalance retrieves balance information
func (t *BitflyerTrader) GetBalance(ctx context.Context) ([]domain.Balance, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.balanceService.GetBalance(ctx)
}

// CancelOrder cancels an order
func (t *BitflyerTrader) CancelOrder(ctx context.Context, orderID string) error {
	return t.orderService.CancelOrder(ctx, orderID)
}

// GetOrders retrieves the list of orders for the default symbol.
func (t *BitflyerTrader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return t.orderService.GetOrders(ctx)
}

// GetOrdersBySymbol retrieves orders for an explicit symbol.
// Implements monitor.OrderGetter so that the order monitor can correctly
// track orders for all traded symbols, not just the default one (Symbols[0]).
func (t *BitflyerTrader) GetOrdersBySymbol(ctx context.Context, symbol string) ([]*domain.OrderResult, error) {
	return t.orderService.GetOrdersBySymbol(ctx, symbol)
}

// InvalidateBalanceCache invalidates the balance cache
func (t *BitflyerTrader) InvalidateBalanceCache() {
	t.balanceService.InvalidateCache()
}

// UpdateBalanceToDB updates balance to database (for BalanceUpdater interface)
func (t *BitflyerTrader) UpdateBalanceToDB(ctx context.Context) {
	t.balanceService.UpdateBalanceToDB(ctx)
}

// Shutdown gracefully shuts down the trader
func (t *BitflyerTrader) Shutdown() error {
	t.shutdownCancel() // Signal all monitoring goroutines to stop

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.logger.Trading().Info("Trader shutdown complete")
		return nil
	case <-time.After(30 * time.Second): // must be >= OrderMonitor's per-call timeout (20s)
		t.logger.Trading().Warn("Trader shutdown timeout - some background tasks may still be running")
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// SetOnOrderCompleted sets the callback function for order completion
func (t *BitflyerTrader) SetOnOrderCompleted(fn func(*domain.OrderResult)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.onOrderCompletedFn = fn

	// Also set callback in OrderMonitor if initialized
	if t.orderMonitor != nil {
		t.orderMonitor.SetOnOrderCompleted(fn)
	}
}

// SetStrategyName sets the strategy name (deprecated: use constructor injection)
func (t *BitflyerTrader) SetStrategyName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.strategyName = name

	// Also update PnLCalculator if initialized
	if t.pnlCalculator != nil {
		t.pnlCalculator.SetStrategyName(name)
	}
}
