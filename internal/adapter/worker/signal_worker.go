package worker

import (
	"context"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

type autoScaleConfig struct {
	enabled     bool
	balancePct  float64
	maxNotional float64
	feeRate     float64
}

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

	cfg := w.getAutoScaleConfig()
	if !cfg.enabled {
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

	signal.Quantity = effectiveNotional / signal.Price
	if signal.Metadata == nil {
		signal.Metadata = make(map[string]interface{})
	}
	signal.Metadata["order_notional_effective"] = effectiveNotional
	signal.Metadata["order_notional_base"] = baseNotional

	return true
}

func computeScaledNotional(baseNotional, availableJPY float64, cfg autoScaleConfig) float64 {
	if baseNotional <= 0 || availableJPY <= 0 {
		return 0
	}

	effective := baseNotional
	if cfg.enabled {
		target := availableJPY * cfg.balancePct / 100.0
		if target > effective {
			effective = target
		}
	}

	if cfg.maxNotional > 0 && effective > cfg.maxNotional {
		effective = cfg.maxNotional
	}

	if cfg.feeRate < 0 {
		cfg.feeRate = 0
	}
	affordable := availableJPY / (1.0 + cfg.feeRate)
	if effective > affordable {
		effective = affordable
	}

	if effective < 0 {
		return 0
	}
	return effective
}

func (w *SignalWorker) getAutoScaleConfig() autoScaleConfig {
	cfg := autoScaleConfig{enabled: false, balancePct: 80.0, maxNotional: 0, feeRate: 0}
	if w.currentStrategy == nil {
		return cfg
	}

	raw := w.currentStrategy.GetConfig()
	if v, ok := asBool(raw["auto_scale_enabled"]); ok {
		cfg.enabled = v
	}
	if v, ok := asFloat(raw["auto_scale_balance_pct"]); ok {
		cfg.balancePct = v
	}
	if cfg.balancePct <= 0 || cfg.balancePct > 100 {
		cfg.balancePct = 80.0
	}
	if v, ok := asFloat(raw["auto_scale_max_notional"]); ok {
		cfg.maxNotional = v
	}
	if v, ok := asFloat(raw["fee_rate"]); ok {
		cfg.feeRate = v
	}

	return cfg
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func asBool(v interface{}) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
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

	// Sell 95% of holdings (to avoid rounding errors with full amount)
	return availableBalance * w.sellSizePercentage
}
