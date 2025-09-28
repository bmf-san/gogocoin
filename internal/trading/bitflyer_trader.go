package trading

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/trading/balance"
	"github.com/bmf-san/gogocoin/v1/internal/trading/monitor"
	"github.com/bmf-san/gogocoin/v1/internal/trading/order"
	"github.com/bmf-san/gogocoin/v1/internal/trading/pnl"
	"github.com/bmf-san/gogocoin/v1/internal/trading/validator"
)

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
	logger             *logger.Logger
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
	logger *logger.Logger,
	db domain.TradingRepository,
	marketSpecSvc MarketSpecService,
	strategyName string,
) Trader {
	// Initialize services in dependency order
	balanceSvc := balance.NewBalanceService(client, db, logger)
	orderSvc := order.NewOrderService(client, logger)
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
	t.orderMonitor = monitor.NewOrderMonitor(logger, db, t, t, pnlCalc)

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
	t.mu.Lock()
	orderMonitor := t.orderMonitor
	shutdownCtx := t.shutdownCtx
	if orderMonitor != nil {
		t.wg.Add(1)
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

// GetOrders retrieves the list of orders
func (t *BitflyerTrader) GetOrders(ctx context.Context) ([]*domain.OrderResult, error) {
	return t.orderService.GetOrders(ctx)
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
	case <-time.After(10 * time.Second):
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
