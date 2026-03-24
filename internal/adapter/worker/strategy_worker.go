package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/logger"
	strategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

const (
	// DefaultHistoryLimit is the default number of market data points to keep in history
	DefaultHistoryLimit = 1000
	// DefaultMaxSymbols is the default maximum number of symbols to track simultaneously
	DefaultMaxSymbols = 100
	// DefaultSymbolExpiryPeriod is the default period after which inactive symbols are removed
	DefaultSymbolExpiryPeriod = 24 * time.Hour
	// DefaultMaxConcurrentProcessing is the default maximum number of concurrent market data processing goroutines
	// This prevents memory exhaustion from unbounded goroutine creation during high-frequency market data
	DefaultMaxConcurrentProcessing = 10

	// ForcedExitCooldown is the minimum interval between successive forced-exit
	// (stop-loss / take-profit) SELL signals for the same symbol. This prevents
	// duplicate orders while still retrying if the first attempt failed.
	ForcedExitCooldown = 30 * time.Second
)

// symbolHistory holds market data history for a single symbol with its own lock
type symbolHistory struct {
	data       []strategy.MarketData
	lastAccess time.Time
	mu         sync.RWMutex
}

// StrategyWorker processes market data and generates trading signals
type StrategyWorker struct {
	logger             logger.LoggerInterface
	strategy           strategy.Strategy
	marketDataCh       <-chan domain.MarketData
	signalCh           chan<- *strategy.Signal
	historyLimit       int
	maxSymbols         int           // Maximum number of symbols to track
	symbolExpiryPeriod time.Duration // Period after which inactive symbols are removed

	// Symbol histories with per-symbol locking for better concurrency
	histories sync.Map // map[string]*symbolHistory

	// Concurrency control
	processingPool chan struct{} // Semaphore to limit concurrent processing goroutines

	// Metrics
	droppedSignals int64 // Atomic counter for dropped signals

	// Deduplication: track last sent signal action per symbol to avoid flooding the channel
	// with identical signals on every tick during a sustained trend
	lastSentSignals sync.Map // map[string]strategy.SignalAction

	// lastForcedExitAttempt tracks when a forced-exit (SL/TP) SELL was last dispatched
	// per symbol so that repeated attempts are gated by ForcedExitCooldown rather than
	// the standard action-level dedup (which would block them indefinitely after any EMA SELL).
	lastForcedExitAttempt sync.Map // map[string]time.Time

	// positionReader is optional; when set, stop-loss is checked on every tick.
	positionReader PositionReader
}

// NewStrategyWorker creates a new strategy worker
func NewStrategyWorker(
	logger logger.LoggerInterface,
	strat strategy.Strategy,
	marketDataCh <-chan domain.MarketData,
	signalCh chan<- *strategy.Signal,
) *StrategyWorker {
	return &StrategyWorker{
		logger:             logger,
		strategy:           strat,
		marketDataCh:       marketDataCh,
		signalCh:           signalCh,
		historyLimit:       DefaultHistoryLimit,
		maxSymbols:         DefaultMaxSymbols,
		symbolExpiryPeriod: DefaultSymbolExpiryPeriod,
		processingPool:     make(chan struct{}, DefaultMaxConcurrentProcessing),
	}
}

// Name returns the worker name.
func (w *StrategyWorker) Name() string { return "strategy-worker" }

// Run starts the strategy worker
func (w *StrategyWorker) Run(ctx context.Context) error {
	w.logger.Strategy().Info("Starting strategy worker")

	// WaitGroup to track all spawned goroutines for graceful shutdown
	var wg sync.WaitGroup
	defer func() {
		// Wait for all goroutines to complete before exiting
		wg.Wait()
		w.logger.Strategy().Info("All strategy processing goroutines completed")
	}()

	// Periodic cleanup of inactive symbols to prevent memory leak
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	// Track cleanup goroutine in WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-cleanupTicker.C:
				w.cleanupInactiveSymbols()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			w.logger.Strategy().Info("Strategy worker stopping, waiting for pending operations...")
			return nil

		case marketData, ok := <-w.marketDataCh:
			if !ok {
				w.logger.Strategy().Info("Market data channel closed")
				return nil
			}

			// Check context before spawning new goroutine
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			// Process market data asynchronously to prevent blocking the channel
			// Make a copy of marketData to avoid race conditions
			dataCopy := marketData

			// Acquire slot from processing pool (blocks if pool is full)
			// This prevents unbounded goroutine creation during high-frequency market data
			select {
			case w.processingPool <- struct{}{}:
				// Slot acquired, proceed with processing
			case <-ctx.Done():
				// Context canceled while waiting for pool slot
				return nil
			}

			wg.Add(1)
			go func(data domain.MarketData) {
				defer wg.Done()
				defer func() {
					// Release processing pool slot
					<-w.processingPool
				}()
				defer func() {
					if r := recover(); r != nil {
						w.logger.Strategy().WithField("panic", r).Error("Strategy processing goroutine panicked")
					}
				}()

				// Check context before processing
				select {
				case <-ctx.Done():
					return
				default:
				}

				w.logger.Strategy().WithField("symbol", data.Symbol).WithField("price", data.Price).Debug("Market data received in strategy worker")

				// Add to history (by symbol)
				strategyData := strategy.MarketData{
					Symbol:    data.Symbol,
					Price:     data.Price,
					Volume:    data.Volume,
					BestBid:   data.BestBid,
					BestAsk:   data.BestAsk,
					Spread:    data.Spread,
					Timestamp: data.Timestamp,
				}

				// Get or create symbol history with per-symbol locking
				hist := w.getOrCreateHistory(data.Symbol)

				// Thread-safe access to this symbol's history
				hist.mu.Lock()
				hist.data = append(hist.data, strategyData)

				// Limit history size (for memory efficiency)
				if len(hist.data) > w.historyLimit {
					hist.data = hist.data[len(hist.data)-w.historyLimit:]
				}

				// Update last access time
				hist.lastAccess = time.Now()

				// Make a copy for strategy execution (avoid holding lock during strategy execution)
				historyCopy := make([]strategy.MarketData, len(hist.data))
				copy(historyCopy, hist.data)
				hist.mu.Unlock()

				w.logger.Strategy().WithField("symbol", data.Symbol).WithField("history_count", len(historyCopy)).Debug("Market data added to history")

				// Execute strategy on market data receipt (pass only history for the relevant symbol)
				w.logger.Strategy().WithField("symbol", data.Symbol).Debug("Executing strategy on market data update")
				w.executeStrategy(ctx, &data, historyCopy)
			}(dataCopy)
		}
	}
}

// getOrCreateHistory gets or creates a symbol history with per-symbol locking
func (w *StrategyWorker) getOrCreateHistory(symbol string) *symbolHistory {
	// Try to load existing history
	if val, ok := w.histories.Load(symbol); ok {
		return val.(*symbolHistory)
	}

	// Create new history
	hist := &symbolHistory{
		data:       make([]strategy.MarketData, 0, w.historyLimit),
		lastAccess: time.Now(),
	}

	// Store and return (LoadOrStore handles race conditions)
	actual, _ := w.histories.LoadOrStore(symbol, hist)
	return actual.(*symbolHistory)
}

// cleanupInactiveSymbols removes symbols that haven't been updated recently
func (w *StrategyWorker) cleanupInactiveSymbols() {
	now := time.Now()
	removedCount := 0
	remainingCount := 0

	// Iterate over all symbols
	w.histories.Range(func(key, value interface{}) bool {
		symbol := key.(string)
		hist := value.(*symbolHistory)

		// Check last access time (with read lock)
		hist.mu.RLock()
		lastAccess := hist.lastAccess
		hist.mu.RUnlock()

		if now.Sub(lastAccess) > w.symbolExpiryPeriod {
			w.histories.Delete(symbol)
			removedCount++
		} else {
			remainingCount++
		}

		return true // Continue iteration
	})

	if removedCount > 0 {
		w.logger.Strategy().
			WithField("removed_symbols", removedCount).
			WithField("remaining_symbols", remainingCount).
			Info("Cleaned up inactive symbols to prevent memory leak")
	}
}

// SetPositionReader injects an optional PositionReader so that stop-loss can be
// checked on every market data tick. Safe to call before Run().
func (w *StrategyWorker) SetPositionReader(r PositionReader) {
	w.positionReader = r
}

// checkStopLoss returns a SELL signal when the current price has fallen below the
// stop-loss threshold of any open BUY position for the symbol. Returns nil when
// stop-loss is not triggered or no position reader is configured.
func (w *StrategyWorker) checkStopLoss(symbol string, currentPrice float64) *strategy.Signal {
	if w.positionReader == nil {
		return nil
	}

	stopLossPct := w.getStopLossPct()
	if stopLossPct <= 0 {
		return nil
	}

	positions, err := w.positionReader.GetOpenPositions(symbol, "BUY")
	if err != nil {
		w.logger.Strategy().WithError(err).Warn("Failed to read open positions for stop-loss check")
		return nil
	}

	for i := range positions {
		pos := &positions[i]
		if pos.EntryPrice <= 0 {
			continue
		}
		stopPrice := pos.EntryPrice * (1.0 - stopLossPct/100.0)
		if currentPrice <= stopPrice {
			w.logger.Trading().
				WithField("symbol", symbol).
				WithField("entry_price", pos.EntryPrice).
				WithField("current_price", currentPrice).
				WithField("stop_price", stopPrice).
				WithField("stop_loss_pct", stopLossPct).
				Warn("Stop loss triggered — injecting SELL signal")
			return &strategy.Signal{
				Symbol:    symbol,
				Action:    strategy.SignalSell,
				Strength:  1.0,
				Price:     currentPrice,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"reason":       "stop_loss",
					"entry_price":  pos.EntryPrice,
					"stop_price":   stopPrice,
					"stop_loss_pct": stopLossPct,
				},
			}
		}
	}
	return nil
}

// getStopLossPct reads stop_loss_pct from the strategy configuration.
// Returns 0 when not configured or invalid.
func (w *StrategyWorker) getStopLossPct() float64 {
	if w.strategy == nil {
		return 0
	}
	if v, ok := asFloat(w.strategy.GetConfig()["stop_loss_pct"]); ok && v > 0 {
		return v
	}
	return 0
}

// checkTakeProfit returns a SELL signal when the current price has risen above the
// take-profit threshold of any open BUY position for the symbol. Returns nil when
// take-profit is not triggered or no position reader is configured.
func (w *StrategyWorker) checkTakeProfit(symbol string, currentPrice float64) *strategy.Signal {
	if w.positionReader == nil {
		return nil
	}

	takeProfitPct := w.getTakeProfitPct()
	if takeProfitPct <= 0 {
		return nil
	}

	positions, err := w.positionReader.GetOpenPositions(symbol, "BUY")
	if err != nil {
		w.logger.Strategy().WithError(err).Warn("Failed to read open positions for take-profit check")
		return nil
	}

	for i := range positions {
		pos := &positions[i]
		if pos.EntryPrice <= 0 {
			continue
		}
		takePrice := pos.EntryPrice * (1.0 + takeProfitPct/100.0)
		if currentPrice >= takePrice {
			w.logger.Trading().
				WithField("symbol", symbol).
				WithField("entry_price", pos.EntryPrice).
				WithField("current_price", currentPrice).
				WithField("take_price", takePrice).
				WithField("take_profit_pct", takeProfitPct).
				Info("Take profit triggered — injecting SELL signal")
			return &strategy.Signal{
				Symbol:    symbol,
				Action:    strategy.SignalSell,
				Strength:  1.0,
				Price:     currentPrice,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"reason":          "take_profit",
					"entry_price":     pos.EntryPrice,
					"take_price":      takePrice,
					"take_profit_pct": takeProfitPct,
				},
			}
		}
	}
	return nil
}

// getTakeProfitPct reads take_profit_pct from the strategy configuration.
// Returns 0 when not configured or invalid.
func (w *StrategyWorker) getTakeProfitPct() float64 {
	if w.strategy == nil {
		return 0
	}
	if v, ok := asFloat(w.strategy.GetConfig()["take_profit_pct"]); ok && v > 0 {
		return v
	}
	return 0
}

// executeStrategy executes the strategy
func (w *StrategyWorker) executeStrategy(ctx context.Context, marketData *domain.MarketData, history []strategy.MarketData) {
	// History already includes the latest data, use as is
	// (already added in Run method)
	signal, err := w.strategy.Analyze(history)
	if err != nil {
		w.logger.Strategy().WithError(err).Error("Failed to analyze market data")
		return
	}

	// Log debug information (output all metadata)
	logEntry := w.logger.Strategy().WithField("symbol", marketData.Symbol).
		WithField("price", marketData.Price).
		WithField("signal", signal.Action)

	// Expand metadata for logging
	for key, value := range signal.Metadata {
		logEntry = logEntry.WithField(key, value)
	}

	// Only log strategy analysis at debug level (not in production)
	// Reduce log volume for high-frequency market data
	logEntry.Debug("Strategy analysis completed")

	// Stop-loss / take-profit override: inject a SELL regardless of the EMA signal
	// when an open BUY position has breached its stop or take-profit price.
	// Stop-loss is checked first (risk priority); take-profit only when no SL fires.
	isForcedExit := false
	if signal.Action != strategy.SignalSell {
		if slSignal := w.checkStopLoss(marketData.Symbol, marketData.Price); slSignal != nil {
			signal = slSignal
			isForcedExit = true
		} else if tpSignal := w.checkTakeProfit(marketData.Symbol, marketData.Price); tpSignal != nil {
			signal = tpSignal
			isForcedExit = true
		}
	}

	if signal.Action != strategy.SignalHold {
		if isForcedExit {
			// Forced exits (SL/TP) bypass the standard action-level dedup because a prior
			// EMA SELL sets lastSentSignals=SELL and would block all subsequent SL/TP signals
			// indefinitely. Instead, use a separate per-symbol cooldown so we retry after
			// ForcedExitCooldown without hammering the exchange on every tick.
			if last, ok := w.lastForcedExitAttempt.Load(signal.Symbol); ok {
				if time.Since(last.(time.Time)) < ForcedExitCooldown {
					return // still within cooldown — wait for prior order to settle
				}
			}
			// Cooldown elapsed (or first attempt): clear standard dedup so the SELL is not
			// blocked by a stale EMA-generated SELL recorded in lastSentSignals.
			w.lastSentSignals.Delete(signal.Symbol)
		} else {
			// Standard EMA dedup: skip if the action is the same as the last sent signal.
			// Once a SELL/BUY trend is established, every tick would re-generate the same
			// signal; we only need to act on the transition (e.g., HOLD→SELL or BUY→SELL).
			if last, ok := w.lastSentSignals.Load(signal.Symbol); ok && last.(strategy.SignalAction) == signal.Action {
				return
			}
		}

		// Only log actual trading signals (not HOLD signals) to reduce log volume
		w.logger.LogStrategySignal(
			w.strategy.Name(),
			signal.Symbol,
			string(signal.Action),
			signal.Strength,
			signal.Metadata,
		)

		// Send signal to processing queue
		select {
		case w.signalCh <- signal:
			// Record last sent action only on successful send
			w.lastSentSignals.Store(signal.Symbol, signal.Action)
			if isForcedExit {
				w.lastForcedExitAttempt.Store(signal.Symbol, time.Now())
			}
		case <-ctx.Done():
			return
		default:
			// Increment dropped signal counter atomically
			droppedCount := atomic.AddInt64(&w.droppedSignals, 1)
			w.logger.Strategy().
				WithField("dropped_count", droppedCount).
				WithField("symbol", signal.Symbol).
				WithField("action", signal.Action).
				Warn("Signal channel is full, dropping signal")
		}
	} else if signal.Action == strategy.SignalHold {
		// Reset last sent signal when strategy returns to HOLD (trend reversed or insufficient data)
		w.lastSentSignals.Delete(signal.Symbol)
	}
}
