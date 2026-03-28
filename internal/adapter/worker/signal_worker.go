package worker

import (
	"context"
	"math"
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
	lotSizeSvc           LotSizeService
	// sellSizePercentage is the fraction of available balance used when selling
	// (< 1.0 to avoid rounding errors). Loaded from config at startup.
	sellSizePercentage float64
	wg                 sync.WaitGroup // Tracks background goroutines for graceful shutdown
	// positionCloser is optional; when set, ghost PARTIAL positions are closed
	// automatically when a stop-loss SELL cannot execute due to dust balance.
	positionCloser PositionCloser
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

// LotSizeService returns the minimum order size (lot size) for a symbol.
type LotSizeService interface {
	GetMinimumOrderSize(symbol string) (float64, error)
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
	lotSizeSvc LotSizeService,
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
		lotSizeSvc:           lotSizeSvc,
		sellSizePercentage:   sellSizePercentage,
	}
}

// SetPositionCloser injects an optional PositionCloser. When set, ghost
// PARTIAL positions are automatically closed if a stop-loss SELL is skipped
// because the actual exchange balance is below the minimum lot size.
func (w *SignalWorker) SetPositionCloser(c PositionCloser) { w.positionCloser = c }

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

	if signal.Action == strategy.SignalBuy {
		if ok := w.applyAutoScaleToBuySignal(ctx, signal); !ok {
			return
		}
	}

	// Risk management check (use parent context instead of Background)
	if err := w.riskChecker.CheckRiskManagement(ctx, signal); err != nil {
		w.logger.Trading().WithError(err).Warn("Risk management check failed - order rejected")
		return
	}

	// Create order (pass context for balance checking)
	order, skip := w.createOrderFromSignal(ctx, signal)
	if skip {
		// If a SELL was skipped because the exchange balance is dust (below the
		// minimum lot size), close ghost OPEN/PARTIAL positions in the DB.
		// This handles SL/TP triggers as well as EMA crossover signals when the
		// position was manually closed on the exchange side.
		if signal.Action == strategy.SignalSell {
			w.closeGhostPositions(signal.Symbol)
		}
		return
	}

	w.logger.Trading().WithField("order", order).Info("Order created from signal")

	// Execute order
	result, err := w.trader.PlaceOrder(ctx, &order)
	if err != nil {
		w.logger.Trading().WithError(err).Error("Failed to place order")
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

func (w *SignalWorker) applyAutoScaleToBuySignal(ctx context.Context, signal *strategy.Signal) bool {
	if signal.Price <= 0 || signal.Quantity <= 0 {
		w.logger.Trading().WithField("symbol", signal.Symbol).
			Warn("Skipping BUY signal - invalid price or quantity")
		return false
	}

	cfg := w.currentStrategy.GetAutoScaleConfig()
	if !cfg.Enabled {
		return true
	}

	availableJPY, ok := w.getAvailableBalance(ctx, "JPY")
	if !ok {
		w.logger.Trading().WithField("symbol", signal.Symbol).
			Warn("Skipping BUY signal - failed to get JPY balance for auto scale")
		return false
	}

	baseNotional := signal.Price * signal.Quantity
	effectiveNotional := computeScaledNotional(baseNotional, availableJPY, cfg)

	if effectiveNotional < baseNotional {
		w.logger.Trading().
			WithField("symbol", signal.Symbol).
			WithField("base_notional", baseNotional).
			WithField("available_jpy", availableJPY).
			Warn("Skipping BUY signal - insufficient JPY for base order_notional")
		return false
	}

	rawQty := effectiveNotional / signal.Price
	lotSize := w.resolveLotsSize(signal.Symbol)
	signal.Quantity = math.Floor(rawQty/lotSize) * lotSize
	if signal.Quantity <= 0 {
		w.logger.Trading().
			WithField("symbol", signal.Symbol).
			WithField("raw_qty", rawQty).
			WithField("lot_size", lotSize).
			Warn("Skipping BUY signal - scaled quantity below minimum lot size after rounding")
		return false
	}
	if signal.Metadata == nil {
		signal.Metadata = make(map[string]interface{})
	}
	signal.Metadata["order_notional_effective"] = effectiveNotional
	signal.Metadata["order_notional_base"] = baseNotional

	return true
}

func computeScaledNotional(baseNotional, availableJPY float64, cfg strategy.AutoScaleConfig) float64 {
	if baseNotional <= 0 || availableJPY <= 0 {
		return 0
	}

	effective := baseNotional
	if cfg.Enabled {
		target := availableJPY * cfg.BalancePct / 100.0
		if target > effective {
			effective = target
		}
	}

	if cfg.MaxNotional > 0 && effective > cfg.MaxNotional {
		effective = cfg.MaxNotional
	}

	feeRate := cfg.FeeRate
	if feeRate < 0 {
		feeRate = 0
	}
	affordable := availableJPY / (1.0 + feeRate)
	if effective > affordable {
		effective = affordable
	}

	if effective < 0 {
		return 0
	}
	return effective
}

func (w *SignalWorker) getAvailableBalance(ctx context.Context, currency string) (float64, bool) {
	balanceCtx, balanceCancel := context.WithTimeout(ctx, 10*time.Second)
	defer balanceCancel()

	balances, err := w.trader.GetBalance(balanceCtx)
	if err != nil {
		w.logger.Trading().WithError(err).Error("Failed to get balance")
		return 0, false
	}

	for i := range balances {
		if balances[i].Currency == currency {
			return balances[i].Available, true
		}
	}

	return 0, false
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

	// Sell a portion of holdings rounded down to the nearest lot size.
	lotSize := w.resolveLotsSize(symbol)
	result := math.Floor(availableBalance*w.sellSizePercentage/lotSize) * lotSize
	if result <= 0 {
		// The percentage-adjusted balance rounds down to zero lots (e.g. dust just
		// below a lot boundary with sell_size_percentage < 1.0).  Fall back to
		// selling exactly one lot when the raw balance covers it, rather than
		// sending a non-lot-rounded quantity that the exchange would reject.
		if availableBalance >= lotSize {
			return lotSize
		}
		// Balance is below the minimum lot size; cannot place a valid sell order.
		w.logger.Trading().
			WithField("symbol", symbol).
			WithField("available", availableBalance).
			WithField("lot_size", lotSize).
			Warn("Available balance below minimum lot size – cannot place SELL order")
		return 0
	}
	return result
}

// closeGhostPositions marks all open/partial BUY positions for symbol as CLOSED
// when a stop-loss SELL cannot execute because exchange balance is dust.
func (w *SignalWorker) closeGhostPositions(symbol string) {
	if w.positionCloser == nil {
		return
	}
	if err := w.positionCloser.CloseOpenPositions(symbol, "BUY"); err != nil {
		w.logger.Trading().WithError(err).WithField("symbol", symbol).
			Warn("Failed to close ghost positions after dust stop-loss skip")
		return
	}
	w.logger.Trading().WithField("symbol", symbol).
		Info("Closed ghost PARTIAL positions — balance was dust (below minimum lot size)")
}

// resolveLotsSize returns the minimum lot size for a symbol using the injected
// LotSizeService when available, falling back to hardcoded values otherwise.
func (w *SignalWorker) resolveLotsSize(symbol string) float64 {
	if w.lotSizeSvc != nil {
		if size, err := w.lotSizeSvc.GetMinimumOrderSize(symbol); err == nil && size > 0 {
			return size
		}
	}
	return fallbackLotSize(symbol)
}

// fallbackLotSize returns hardcoded lot sizes used when the LotSizeService is
// unavailable. Values match BitFlyer's documented minimums.
func fallbackLotSize(symbol string) float64 {
	switch symbol {
	case "BTC_JPY":
		return 0.001
	case "ETH_JPY":
		return 0.01
	case "XRP_JPY":
		return 1.0
	case "XLM_JPY":
		return 10.0
	case "MONA_JPY":
		return 1.0
	case "BCH_JPY":
		return 0.01
	default:
		return 0.001
	}
}
