package worker

import (
	"context"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// SignalWorker processes trading signals and executes trades
type SignalWorker struct {
	logger               logger.LoggerInterface
	signalCh             <-chan *strategy.Signal
	tradingEnabledGetter TradingEnabledGetter
	riskChecker          RiskChecker
	trader               Trader
	currentStrategy      strategy.Strategy
	performanceUpdater   PerformanceUpdater
	// sellSizePercentage is the fraction of available balance used when selling
	// (< 1.0 to avoid rounding errors). Loaded from config at startup.
	sellSizePercentage float64
	wg                 sync.WaitGroup // Tracks background goroutines for graceful shutdown
}

// TradingEnabledGetter defines the interface for checking if trading is enabled
type TradingEnabledGetter interface {
	IsTradingEnabled() bool
}

// RiskChecker defines the interface for risk management checks
type RiskChecker interface {
	CheckRiskManagement(ctx context.Context, signal *strategy.Signal) error
}

// Trader defines the interface for trading operations
type Trader interface {
	PlaceOrder(ctx context.Context, order *domain.OrderRequest) (*domain.OrderResult, error)
	GetBalance(ctx context.Context) ([]domain.Balance, error)
}

// PerformanceUpdater defines the interface for updating performance metrics
type PerformanceUpdater interface {
	UpdateMetrics(ctx context.Context) error
}

// NewSignalWorker creates a new signal worker
func NewSignalWorker(
	logger logger.LoggerInterface,
	signalCh <-chan *strategy.Signal,
	tradingEnabledGetter TradingEnabledGetter,
	riskChecker RiskChecker,
	trader Trader,
	currentStrategy strategy.Strategy,
	performanceUpdater PerformanceUpdater,
	sellSizePercentage float64,
) *SignalWorker {
	return &SignalWorker{
		logger:               logger,
		signalCh:             signalCh,
		tradingEnabledGetter: tradingEnabledGetter,
		riskChecker:          riskChecker,
		trader:               trader,
		currentStrategy:      currentStrategy,
		performanceUpdater:   performanceUpdater,
		sellSizePercentage:   sellSizePercentage,
	}
}

// Name returns the worker name.
func (w *SignalWorker) Name() string { return "signal-worker" }

// Run starts the signal worker
func (w *SignalWorker) Run(ctx context.Context) error {
	w.logger.Trading().Info("Starting signal worker")

	defer func() {
		// Wait for all background goroutines to complete before exiting
		w.wg.Wait()
		w.logger.Trading().Info("All background tasks completed")
	}()

	for {
		select {
		case <-ctx.Done():
			w.logger.Trading().Info("Signal worker stopped")
			return nil
		case signal, ok := <-w.signalCh:
			if !ok {
				w.logger.Trading().Info("Signal channel closed, stopping signal worker")
				return nil
			}
			w.processSignal(ctx, signal)
		}
	}
}

// processSignal processes a signal
func (w *SignalWorker) processSignal(ctx context.Context, signal *strategy.Signal) {
	if signal.Action == strategy.SignalHold {
		return
	}

	w.logger.Trading().WithField("action", string(signal.Action)).WithField("symbol", signal.Symbol).WithField("price", signal.Price).Info("Processing trading signal")

	// Pre-trading checks
	if !w.tradingEnabledGetter.IsTradingEnabled() {
		w.logger.Trading().Warn("Trading is disabled, skipping signal")
		w.logger.Trading().Info("To enable trading, use the Web UI or API")
		return
	}

	// Risk management check (use parent context instead of Background)
	if err := w.riskChecker.CheckRiskManagement(ctx, signal); err != nil {
		w.logger.Trading().WithError(err).Warn("Risk management check failed - order rejected")
		return
	}

	// Create order (pass context for balance checking)
	order, skip := w.createOrderFromSignal(ctx, signal)
	if skip {
		return
	}

	w.logger.Trading().WithField("order", order).Info("Order created from signal")

	// Execute order
	result, err := w.trader.PlaceOrder(ctx, &order)
	if err != nil {
		w.logger.Trading().WithError(err).Error("Failed to place order")
		w.logger.Trading().Info("Check API credentials and account permissions")
		return
	}

	w.logger.Trading().WithField("order_id", result.OrderID).Info("Order placed successfully")

	w.logger.LogTrade(
		string(signal.Action),
		signal.Symbol,
		signal.Price,
		signal.Quantity,
		map[string]interface{}{
			"order_id":        result.OrderID,
			"strategy":        w.currentStrategy.Name(),
			"signal_strength": signal.Strength,
		},
	)

	// Note: RecordTrade() will be called in monitorOrderExecution after order is completed
	// to ensure accurate trade counting and cooldown timing

	// Update performance metrics after trade execution (use parent context)
	// Track this goroutine for graceful shutdown
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.performanceUpdater.UpdateMetrics(ctx); err != nil {
			w.logger.System().WithError(err).Error("Failed to update performance metrics")
		}
	}()
}

// createOrderFromSignal creates an order from a signal.
// Returns (order, skip): if skip is true the caller should discard the order;
// createOrderFromSignal already logged the reason.
func (w *SignalWorker) createOrderFromSignal(ctx context.Context, signal *strategy.Signal) (domain.OrderRequest, bool) {
	var side string
	var size float64

	switch signal.Action {
	case strategy.SignalBuy:
		side = "BUY"
		size = signal.Quantity
	case strategy.SignalSell:
		side = "SELL"
		// For SELL, get actual holdings and adjust
		size = w.getAvailableSellSize(ctx, signal.Symbol, signal.Quantity)
		// If no crypto to sell, skip the order.
		if size == 0 {
			w.logger.Trading().WithField("symbol", signal.Symbol).
				Info("Skipping SELL signal - no crypto holdings available")
			return domain.OrderRequest{}, true
		}
	default:
		// Unknown signal action — skip rather than defaulting to BUY
		w.logger.Trading().WithField("action", signal.Action).Warn("Unknown signal action, skipping order")
		return domain.OrderRequest{}, true
	}

	return domain.OrderRequest{
		Symbol:      signal.Symbol,
		Side:        side,
		Type:        "MARKET", // Market order for now
		Size:        size,
		Price:       signal.Price,
		TimeInForce: "IOC", // Immediate or Cancel
	}, false
}

// getAvailableSellSize gets the available size for selling
func (w *SignalWorker) getAvailableSellSize(ctx context.Context, symbol string, requestedSize float64) float64 {
	// Extract currency from symbol (e.g., "BTC_JPY" -> "BTC", "ETH_USD" -> "ETH")
	currency := symbol
	// Split by "_" and get the first part
	if idx := len(symbol) - 1; idx > 0 {
		for i := idx; i >= 0; i-- {
			if symbol[i] == '_' {
				currency = symbol[:i]
				break
			}
		}
	}

	// Get current balance (use context with timeout to prevent indefinite blocking)
	balanceCtx, balanceCancel := context.WithTimeout(ctx, 10*time.Second)
	defer balanceCancel()

	balances, err := w.trader.GetBalance(balanceCtx)
	if err != nil {
		w.logger.Trading().WithError(err).Error("Failed to get balance for SELL order")
		return 0
	}

	// Find balance for the relevant currency
	var availableBalance float64
	for i := range balances {
		if balances[i].Currency == currency {
			availableBalance = balances[i].Available
			break
		}
	}

	// Return 0 if no holdings
	if availableBalance == 0 {
		w.logger.Trading().WithField("symbol", symbol).WithField("currency", currency).
			Warn("No available balance for SELL order")
		return 0
	}

	// Return the smaller of requested size and available balance
	if requestedSize > 0 && requestedSize < availableBalance {
		return requestedSize
	}

	// Sell 95% of holdings (to avoid rounding errors with full amount)
	return availableBalance * w.sellSizePercentage
}
