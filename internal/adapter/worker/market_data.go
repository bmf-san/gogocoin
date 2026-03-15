package worker

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
)

// Deprecated: These constants are now configured via config.yaml (worker section)
// They are kept for backward compatibility but will be removed in v2.0
const (
	defaultReconnectInterval       = 10 * time.Second
	defaultMaxReconnectInterval    = 5 * time.Minute
	defaultConnectionCheckInterval = 30 * time.Second
	defaultStaleDataTimeout        = 3 * time.Minute
)

// MarketDataWorker manages WebSocket connections and market data subscriptions
type MarketDataWorker struct {
	logger                  logger.LoggerInterface
	symbols                 []string
	marketDataCh            chan<- domain.MarketData
	clientFactory           ClientFactory
	reconnectInterval       time.Duration
	maxReconnectInterval    time.Duration
	connectionCheckInterval time.Duration
	staleDataTimeout        time.Duration
	lastDataReceivedNs      atomic.Int64 // unix nanoseconds; updated on every ticker callback
}

// ClientFactory defines the interface for creating and managing clients
type ClientFactory interface {
	IsConnected() bool
	ReconnectClient() error
	SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error
}

// NewMarketDataWorker creates a new market data worker
func NewMarketDataWorker(
	logger logger.LoggerInterface,
	symbols []string,
	marketDataCh chan<- domain.MarketData,
	clientFactory ClientFactory,
	reconnectIntervalSec int,
	maxReconnectIntervalSec int,
	connectionCheckIntervalSec int,
	staleDataTimeoutSec int,
) *MarketDataWorker {
	// Clamp to defaults to prevent zero/negative duration panics in time.NewTicker
	if reconnectIntervalSec <= 0 {
		reconnectIntervalSec = int(defaultReconnectInterval.Seconds())
	}
	if maxReconnectIntervalSec <= 0 {
		maxReconnectIntervalSec = int(defaultMaxReconnectInterval.Seconds())
	}
	if connectionCheckIntervalSec <= 0 {
		connectionCheckIntervalSec = int(defaultConnectionCheckInterval.Seconds())
	}
	if staleDataTimeoutSec <= 0 {
		staleDataTimeoutSec = int(defaultStaleDataTimeout.Seconds())
	}
	return &MarketDataWorker{
		logger:                  logger,
		symbols:                 symbols,
		marketDataCh:            marketDataCh,
		clientFactory:           clientFactory,
		reconnectInterval:       time.Duration(reconnectIntervalSec) * time.Second,
		maxReconnectInterval:    time.Duration(maxReconnectIntervalSec) * time.Second,
		connectionCheckInterval: time.Duration(connectionCheckIntervalSec) * time.Second,
		staleDataTimeout:        time.Duration(staleDataTimeoutSec) * time.Second,
	}
}

// Name returns the worker name.
func (w *MarketDataWorker) Name() string { return "market-data-worker" }

// Run starts the market data worker
func (w *MarketDataWorker) Run(ctx context.Context) error {
	w.logger.Data().Info("Starting market data worker")

	// WebSocket reconnection loop
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			w.logger.Data().Info("Market data worker stopped by context")
			return nil
		default:
		}

		// Check WebSocket connection status
		if !w.clientFactory.IsConnected() {
			w.logger.Data().Warn("WebSocket client is not connected - attempting reconnection")

			// Reconnect bitFlyer client
			if err := w.clientFactory.ReconnectClient(); err != nil {
				reconnectAttempts++
				wait := w.reconnectInterval * time.Duration(reconnectAttempts)
				if wait > w.maxReconnectInterval {
					wait = w.maxReconnectInterval
				}

				w.logger.Data().WithError(err).
					WithField("attempt", reconnectAttempts).
					WithField("retry_in_seconds", wait.Seconds()).
					Error("Failed to reconnect WebSocket client, retrying")

				// Use Timer instead of time.After to prevent memory leak
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
					continue
				case <-ctx.Done():
					timer.Stop()
					return nil
				}
			} else {
				w.logger.Data().Info("WebSocket client reconnected successfully")
				reconnectAttempts = 0 // Reset counter on successful connection
			}
		} else {
			w.logger.Data().Info("WebSocket client is connected - proceeding with subscriptions")
			reconnectAttempts = 0 // Reset counter
		}

		// Subscribe to ticker data for target symbols
		w.logger.Data().WithField("symbols", w.symbols).Info("Attempting to subscribe to ticker data for symbols")
		subscribedCount := 0
		for _, symbol := range w.symbols {
			w.logger.Data().WithField("symbol", symbol).Info("Subscribing to ticker data")

			err := w.clientFactory.SubscribeToTicker(ctx, symbol, func(data domain.MarketData) {
				w.logger.Data().WithField("symbol", data.Symbol).WithField("close", data.Close).Debug("Market data received in callback")
				w.lastDataReceivedNs.Store(time.Now().UnixNano())

				select {
				case w.marketDataCh <- data:
					w.logger.Data().WithField("symbol", data.Symbol).Debug("Market data sent to channel successfully")
				case <-ctx.Done():
					return
				default:
					// Channel is full - drop new data
					w.logger.Data().WithField("symbol", data.Symbol).Warn("Market data channel is full, dropping new data")
				}
			})

			if err != nil {
				w.logger.Data().WithError(err).WithField("symbol", symbol).Error("Failed to subscribe to ticker")
			} else {
				w.logger.Data().WithField("symbol", symbol).Info("Successfully subscribed to ticker data")
				subscribedCount++
			}
		}

		if subscribedCount == 0 {
			w.logger.Data().Error("No market data subscriptions successful - will retry connection")
			// Use Timer instead of time.After to prevent memory leak
			timer := time.NewTimer(w.reconnectInterval)
			select {
			case <-timer.C:
				continue // Retry subscription
			case <-ctx.Done():
				timer.Stop()
				return nil
			}
		} else {
			w.logger.Data().WithField("subscribed_symbols", subscribedCount).WithField("total_symbols", len(w.symbols)).Info("Market data subscriptions completed")
		}

		// Reset last-data timestamp so the stale-data detector doesn't trip immediately
		w.lastDataReceivedNs.Store(time.Now().UnixNano())

		// Monitor connection health
		ticker := time.NewTicker(w.connectionCheckInterval)

	monitorLoop:
		for {
			select {
			case <-ticker.C:
				if !w.clientFactory.IsConnected() {
					w.logger.Data().Warn("WebSocket connection lost, initiating reconnection")
					ticker.Stop()
					break monitorLoop // Exit inner loop to reconnect
				}
				// Stale data check: reconnect if no ticker has arrived for staleDataTimeout
				if elapsed := time.Since(time.Unix(0, w.lastDataReceivedNs.Load())); elapsed > w.staleDataTimeout {
					w.logger.Data().
						WithField("elapsed_seconds", elapsed.Seconds()).
						Warn("No market data received - WebSocket may be silently dead, forcing reconnection")
					ticker.Stop()
					// Mark disconnected so the outer loop will call ReconnectClient
					if rc, ok := w.clientFactory.(interface{ SetDisconnected() }); ok {
						rc.SetDisconnected()
					}
					break monitorLoop
				}
			case <-ctx.Done():
				ticker.Stop()
				w.logger.Data().Info("Market data worker stopped")
				return nil
			}
		}
	}
}
