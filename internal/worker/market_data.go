package worker

import (
	"context"
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
)

// MarketDataWorker manages WebSocket connections and market data subscriptions
type MarketDataWorker struct {
	logger                  *logger.Logger
	symbols                 []string
	marketDataCh            chan<- domain.MarketData
	clientFactory           ClientFactory
	reconnectInterval       time.Duration
	maxReconnectInterval    time.Duration
	connectionCheckInterval time.Duration
}

// ClientFactory defines the interface for creating and managing clients
type ClientFactory interface {
	IsConnected() bool
	ReconnectClient() error
	SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error
}

// NewMarketDataWorker creates a new market data worker
func NewMarketDataWorker(
	logger *logger.Logger,
	symbols []string,
	marketDataCh chan<- domain.MarketData,
	clientFactory ClientFactory,
	reconnectIntervalSec int,
	maxReconnectIntervalSec int,
	connectionCheckIntervalSec int,
) *MarketDataWorker {
	return &MarketDataWorker{
		logger:                  logger,
		symbols:                 symbols,
		marketDataCh:            marketDataCh,
		clientFactory:           clientFactory,
		reconnectInterval:       time.Duration(reconnectIntervalSec) * time.Second,
		maxReconnectInterval:    time.Duration(maxReconnectIntervalSec) * time.Second,
		connectionCheckInterval: time.Duration(connectionCheckIntervalSec) * time.Second,
	}
}

// Run starts the market data worker
func (w *MarketDataWorker) Run(ctx context.Context) {
	w.logger.Data().Info("Starting market data worker")

	// WebSocket reconnection loop
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			w.logger.Data().Info("Market data worker stopped by context")
			return
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
					return
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
				w.logger.Data().WithField("symbol", data.Symbol).WithField("close", data.Close).Info("Market data received in callback")

				select {
				case w.marketDataCh <- data:
					w.logger.Data().WithField("symbol", data.Symbol).Info("Market data sent to channel successfully")
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
				return
			}
		} else {
			w.logger.Data().WithField("subscribed_symbols", subscribedCount).WithField("total_symbols", len(w.symbols)).Info("Market data subscriptions completed")
		}

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
			case <-ctx.Done():
				ticker.Stop()
				w.logger.Data().Info("Market data worker stopped")
				return
			}
		}
	}
}
