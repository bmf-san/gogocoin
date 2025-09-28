package app

import (
	"context"
	"fmt"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
)

// ConnectionManager handles bitFlyer client connection and reconnection
// Following Single Responsibility Principle, this component is responsible
// only for managing WebSocket connections and reconnection logic.
type ConnectionManager struct {
	config         *config.Config
	logger         *logger.Logger // Concrete type - connection management doesn't need abstraction
	client         BitflyerClient
	marketDataSvc  MarketDataService
	marketSpecSvc  MarketSpecService
	tradingSvc     trading.Trader
	db             Database
	strategyName   string // Strategy name for trading service initialization
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(
	cfg *config.Config,
	log *logger.Logger,
	client BitflyerClient,
	marketDataSvc MarketDataService,
	marketSpecSvc MarketSpecService,
	tradingSvc trading.Trader,
	db Database,
	strategyName string,
) *ConnectionManager {
	return &ConnectionManager{
		config:        cfg,
		logger:        log,
		client:        client,
		strategyName:  strategyName,
		marketDataSvc: marketDataSvc,
		marketSpecSvc: marketSpecSvc,
		tradingSvc:    tradingSvc,
		db:            db,
	}
}

// IsConnected checks if the bitFlyer client is connected
func (cm *ConnectionManager) IsConnected() bool {
	if cm.client == nil {
		return false
	}
	return cm.client.IsConnected()
}

// ReconnectClient reconnects the bitFlyer client
func (cm *ConnectionManager) ReconnectClient() error {
	cm.logger.System().Info("Reconnecting bitFlyer client...")

	// Close existing client connection
	if cm.client != nil {
		ctx := context.Background()
		if err := cm.client.Close(ctx); err != nil {
			cm.logger.System().WithError(err).Warn("Failed to close existing client during reconnection")
		}
	}

	// Reset market data service callbacks to prevent leaks
	if cm.marketDataSvc != nil {
		cm.marketDataSvc.ResetCallbacks()
	}

	// Create new client configuration
	clientConfig := bitflyer.Config{
		APIKey:            cm.config.API.Credentials.APIKey,
		APISecret:         cm.config.API.Credentials.APISecret,
		Endpoint:          cm.config.API.Endpoint,
		WebSocketEndpoint: cm.config.API.WebSocketEndpoint,
		Timeout:           cm.config.API.Timeout,
		RetryCount:        cm.config.API.RetryCount,
		RequestsPerMinute: cm.config.API.RateLimit.RequestsPerMinute,
		InitialBalance:    cm.config.Trading.InitialBalance,
		FeeRate:           cm.config.Trading.FeeRate,
	}

	// Create new client
	client, err := bitflyer.NewClient(&clientConfig, cm.logger)
	if err != nil {
		return fmt.Errorf("failed to create new bitFlyer client: %w", err)
	}

	// Create new services with configuration
	marketDataConfig := &bitflyer.MarketDataConfig{
		HistoryDays: cm.config.Data.MarketData.HistoryDays,
	}
	bfMarketDataSvc := bitflyer.NewMarketDataService(client, cm.logger, cm.db, marketDataConfig, cm.config.Worker.MaxConcurrentSaves)

	bfMarketSpecSvc := bitflyer.NewMarketSpecificationService(client)

	// Update adapters
	cm.client = client
	cm.marketDataSvc = newMarketDataServiceAdapter(bfMarketDataSvc)
	cm.marketSpecSvc = newMarketSpecServiceAdapter(bfMarketSpecSvc)

	// Create new trading service with all dependencies
	tradingSvc := trading.NewTraderWithDependencies(
		client,
		cm.logger,
		cm.db,
		cm.marketSpecSvc,
		cm.strategyName,
	)
	cm.tradingSvc = tradingSvc

	cm.logger.System().Info("bitFlyer client reconnected successfully")
	return nil
}

// SubscribeToTicker subscribes to ticker data for a symbol
func (cm *ConnectionManager) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	if cm.marketDataSvc == nil {
		return fmt.Errorf("market data service not initialized")
	}
	return cm.marketDataSvc.SubscribeToTicker(ctx, symbol, callback)
}

// GetMarketDataService returns the market data service
func (cm *ConnectionManager) GetMarketDataService() MarketDataService {
	return cm.marketDataSvc
}

// GetMarketSpecService returns the market specification service
func (cm *ConnectionManager) GetMarketSpecService() MarketSpecService {
	return cm.marketSpecSvc
}

// GetTradingService returns the trading service
func (cm *ConnectionManager) GetTradingService() trading.Trader {
	return cm.tradingSvc
}

// Close closes the bitFlyer client connection
func (cm *ConnectionManager) Close(ctx context.Context) error {
	if cm.client != nil {
		return cm.client.Close(ctx)
	}
	return nil
}
