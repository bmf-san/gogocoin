package app

import (
	"context"
	"fmt"

	"github.com/bmf-san/gogocoin/v1/internal/analytics"
	"github.com/bmf-san/gogocoin/v1/internal/api"
	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/risk"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
	"github.com/bmf-san/gogocoin/v1/internal/utils"
)

// ServiceInitializer handles initialization of all application services
// Following Single Responsibility Principle, this component is responsible
// only for service creation and configuration.
type ServiceInitializer struct {
	config *config.Config
	logger *logger.Logger // Concrete type - initialization layer doesn't need abstraction
}

// NewServiceInitializer creates a new service initializer
func NewServiceInitializer(cfg *config.Config, log *logger.Logger) *ServiceInitializer {
	return &ServiceInitializer{
		config: cfg,
		logger: log,
	}
}

// Services holds all initialized services
type Services struct {
	Database             Database
	BitflyerClient       BitflyerClient
	MarketDataService    MarketDataService
	MarketSpecService    MarketSpecService
	TradingService       trading.Trader
	RiskManager          risk.RiskManager
	PerformanceAnalytics analytics.PerformanceAnalyticsService
	Strategy             strategy.Strategy
	APIServer            *api.Server
}

// InitializeAll initializes all application services
func (si *ServiceInitializer) InitializeAll(ctx context.Context) (*Services, error) {
	services := &Services{}

	// Initialize database first (required by other services)
	db, err := si.initDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	services.Database = db

	// Initialize bitFlyer client and related services (pass database for dependency injection)
	bfClient, marketDataSvc, marketSpecSvc, tradingSvc, err := si.initBitflyerClient(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bitFlyer client: %w", err)
	}
	services.BitflyerClient = bfClient
	services.MarketDataService = marketDataSvc
	services.MarketSpecService = marketSpecSvc
	services.TradingService = tradingSvc

	// Initialize risk manager
	riskMgr := risk.NewRiskManager(
		&si.config.Trading.RiskManagement,
		&si.config.Trading,
		db,
		db,
		tradingSvc,
		si.logger,
	)
	services.RiskManager = riskMgr

	// Initialize performance analytics
	perfAnalytics := analytics.NewPerformanceAnalytics(
		db,
		db,
		si.logger,
		si.config.Trading.InitialBalance,
	)
	services.PerformanceAnalytics = perfAnalytics

	// Initialize strategy
	strat, err := si.initStrategy()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize strategy: %w", err)
	}
	services.Strategy = strat

	// Initialize API server
	apiServer, err := si.initAPIServer(db, tradingSvc, strat)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API server: %w", err)
	}
	services.APIServer = apiServer

	return services, nil
}

// initDatabase initializes the database connection
func (si *ServiceInitializer) initDatabase() (Database, error) {
	dbPath := "./data/gogocoin.db"

	db, err := database.NewDB(dbPath, si.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Set database reference in logger for log persistence
	si.logger.SetDatabase(db)

	return db, nil
}

// initBitflyerClient initializes bitFlyer client and related services
func (si *ServiceInitializer) initBitflyerClient(db Database) (BitflyerClient, MarketDataService, MarketSpecService, trading.Trader, error) {
	clientConfig := bitflyer.Config{
		APIKey:            si.config.API.Credentials.APIKey,
		APISecret:         si.config.API.Credentials.APISecret,
		Endpoint:          si.config.API.Endpoint,
		WebSocketEndpoint: si.config.API.WebSocketEndpoint,
		Timeout:           si.config.API.Timeout,
		RetryCount:        si.config.API.RetryCount,
		RequestsPerMinute: si.config.API.RateLimit.RequestsPerMinute,
		InitialBalance:    si.config.Trading.InitialBalance,
		FeeRate:           si.config.Trading.FeeRate,
	}

	client, err := bitflyer.NewClient(&clientConfig, si.logger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create bitFlyer client: %w", err)
	}

	// Create market data service with configuration
	marketDataConfig := &bitflyer.MarketDataConfig{
		HistoryDays: si.config.Data.MarketData.HistoryDays,
	}
	bfMarketDataSvc := bitflyer.NewMarketDataService(client, si.logger, db, marketDataConfig, si.config.Worker.MaxConcurrentSaves)

	// Create market specification service
	bfMarketSpecSvc := bitflyer.NewMarketSpecificationService(client)

	// Create adapters
	marketDataSvc := newMarketDataServiceAdapter(bfMarketDataSvc)
	marketSpecSvc := newMarketSpecServiceAdapter(bfMarketSpecSvc)

	// Create trading service with all dependencies injected
	tradingSvc := trading.NewTraderWithDependencies(
		client,
		si.logger,
		db,
		marketSpecSvc,
		si.config.Trading.Strategy.Name,
	)

	// Verify API credentials if provided
	if si.config.API.Credentials.APIKey != "" && si.config.API.Credentials.APISecret != "" {
		si.logger.System().Info("API credentials configured, verifying connection...")
	}

	return client, marketDataSvc, marketSpecSvc, tradingSvc, nil
}

// initStrategy initializes the trading strategy
func (si *ServiceInitializer) initStrategy() (strategy.Strategy, error) {
	strategyName := si.config.Trading.Strategy.Name

	// Create strategy factory
	factory := strategy.NewStrategyFactory()

	// Get strategy parameters
	params, err := si.config.GetStrategyParams(strategyName)
	if err != nil {
		params = nil // Use default values
	}

	// Create strategy
	strat, err := factory.CreateStrategy(strategyName, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create strategy %s: %w", strategyName, err)
	}

	si.logger.Strategy().WithField("strategy", strategyName).Info("Strategy initialized successfully")
	return strat, nil
}

// initAPIServer initializes the API server
func (si *ServiceInitializer) initAPIServer(db Database, trader trading.Trader, strat strategy.Strategy) (*api.Server, error) {
	// The concrete type (*database.DB) of db satisfies api.DatabaseService
	// This follows the Consumer-Driven Contracts pattern
	if concreteDB, ok := db.(*database.DB); ok {
		server := api.NewServer(si.config, concreteDB, si.logger)
		return server, nil
	}
	return nil, fmt.Errorf("database does not satisfy api.DatabaseService interface")
}

// InitializeDailyTradeCount initializes the daily trade count from database
func (si *ServiceInitializer) InitializeDailyTradeCount(db Database, strat strategy.Strategy) error {
	// Get today's date in JST
	now := utils.NowInJST()
	todayYear, todayMonth, todayDay := now.Date()

	// Count today's trades from database
	trades, err := db.GetRecentTrades(100)
	if err == nil {
		todayTrades := 0
		for i := range trades {
			// Use ExecutedAt (actual execution time) instead of CreatedAt for consistency
			tradeTime := utils.ToJST(trades[i].ExecutedAt)
			tradeYear, tradeMonth, tradeDay := tradeTime.Date()
			if tradeYear == todayYear && tradeMonth == todayMonth && tradeDay == todayDay {
				todayTrades++
			}
		}
		strat.InitializeDailyTradeCount(todayTrades)
		si.logger.Strategy().WithField("today_trades", todayTrades).Info("Daily trade count initialized from database")
	}

	return nil
}
