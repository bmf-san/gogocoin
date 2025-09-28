package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
	"github.com/bmf-san/gogocoin/v1/internal/worker"
)

// Deprecated: These constants are now configured via config.yaml (worker section)
// They are kept for backward compatibility but will be removed in v2.0
const (
	// MarketDataChannelBuffer is deprecated, use config.Worker.MarketDataChannelBuffer
	MarketDataChannelBuffer = 1000
	// SignalChannelBuffer is deprecated, use config.Worker.SignalChannelBuffer
	SignalChannelBuffer = 100
)

// App state keys for database persistence
const (
	AppStateKeyTradingEnabled = "AppStateKeyTradingEnabled"
)

// Type aliases for better readability and backward compatibility
// These interfaces are now defined in their respective domain packages
type (
	// BitflyerClient is an alias for bitflyer.ClientInterface
	BitflyerClient = bitflyer.ClientInterface

	// MarketDataService is an alias for bitflyer.MarketDataServiceInterface
	MarketDataService = bitflyer.MarketDataServiceInterface

	// MarketSpecService is an alias for bitflyer.MarketSpecServiceInterface
	MarketSpecService = bitflyer.MarketSpecServiceInterface

	// Database is an alias for database.DatabaseInterface
	Database = database.DatabaseInterface
)

// Application is the main application
// Following Single Responsibility Principle, this component now focuses on lifecycle management
// Service initialization is delegated to ServiceInitializer
// Connection management is delegated to ConnectionManager
// Service references are managed by ServiceRegistry
type Application struct {
	config *config.Config
	logger logger.LoggerInterface // Interface type following Dependency Inversion Principle

	// Component delegation (reduces responsibilities from 19 to core lifecycle)
	connManager     *ConnectionManager // Handles bitFlyer connection and reconnection
	serviceRegistry *ServiceRegistry   // Manages all service references

	// State management
	isRunning      bool
	tradingEnabled bool
	mu             sync.RWMutex

	// Data channels
	marketDataCh chan domain.MarketData
	signalCh     chan *strategy.Signal

	// Stop channel
	stopCh    chan struct{}
	closeOnce sync.Once // Ensures stopCh is closed only once
}

// New creates a new application
// Delegates service initialization to ServiceInitializer, reducing Application's responsibilities
// Uses ServiceRegistry to manage service references
func New(cfg *config.Config, log *logger.Logger) (*Application, error) {
	app := &Application{
		config:         cfg,
		logger:         log,
		tradingEnabled: false, // Always starts in stopped state, user must explicitly start

		// Channel ownership:
		// - marketDataCh: owned by Application, closed by MarketDataWorker (producer)
		// - signalCh: owned by Application, closed by StrategyWorker (producer)
		// - stopCh: owned by Application, closed by Application.Shutdown() (via closeOnce)
		marketDataCh: make(chan domain.MarketData, cfg.Worker.MarketDataChannelBuffer),
		signalCh:     make(chan *strategy.Signal, cfg.Worker.SignalChannelBuffer),
		stopCh:       make(chan struct{}),
	}

	// Use ServiceInitializer to initialize all services
	serviceInit := NewServiceInitializer(cfg, log)
	services, err := serviceInit.InitializeAll(context.Background())
	if err != nil {
		return nil, ErrServiceInitializationFailed("services", err)
	}

	// Create and populate service registry
	app.serviceRegistry = NewServiceRegistry()
	app.serviceRegistry.Database = services.Database
	app.serviceRegistry.BitflyerClient = services.BitflyerClient
	app.serviceRegistry.MarketDataService = services.MarketDataService
	app.serviceRegistry.MarketSpecService = services.MarketSpecService
	app.serviceRegistry.TradingService = services.TradingService
	app.serviceRegistry.RiskManager = services.RiskManager
	app.serviceRegistry.PerformanceAnalytics = services.PerformanceAnalytics
	app.serviceRegistry.CurrentStrategy = services.Strategy
	app.serviceRegistry.APIServer = services.APIServer

	// Validate all services are initialized
	if err := app.serviceRegistry.Validate(); err != nil {
		return nil, err
	}

	// Set Application reference in API server
	if app.serviceRegistry.APIServer != nil {
		app.serviceRegistry.APIServer.SetApplication(app)
	}

	// Create ConnectionManager for handling bitFlyer connections
	app.connManager = NewConnectionManager(
		cfg,
		log,
		services.BitflyerClient,
		services.MarketDataService,
		services.MarketSpecService,
		services.TradingService,
		services.Database,
		cfg.Trading.Strategy.Name,
	)

	// Always reset trading state to false on startup (for safety)
	if app.serviceRegistry.Database != nil {
		if err := app.serviceRegistry.Database.SaveAppState("AppStateKeyTradingEnabled", "false"); err != nil {
			app.logger.System().WithError(err).Warn("Failed to reset trading state, continuing anyway")
		}
	}

	// Initialize daily trade count from database
	if err := serviceInit.InitializeDailyTradeCount(app.serviceRegistry.Database, app.serviceRegistry.CurrentStrategy); err != nil {
		log.System().WithError(err).Warn("Failed to initialize daily trade count")
	}

	// Set order completion callback for strategy tracking
	app.serviceRegistry.TradingService.SetOnOrderCompleted(func(result *domain.OrderResult) {
		if app.serviceRegistry.CurrentStrategy != nil {
			app.serviceRegistry.CurrentStrategy.RecordTrade()
			app.logger.Strategy().WithField("order_id", result.OrderID).Info("Trade recorded in strategy")
		}
	})

	return app, nil
}


// Run runs the application
func (app *Application) Run(ctx context.Context) error {
	app.mu.Lock()
	app.isRunning = true
	app.mu.Unlock()

	app.logger.System().Info("Starting gogocoin application")
	app.logger.System().Info("Trading started in stopped state. Please start trading via WebUI or API.")

	// Diagnose trading configuration
	app.diagnoseTradingSetup()

	// Start strategy
	app.logger.Strategy().Info("Starting strategy...")
	if err := app.serviceRegistry.CurrentStrategy.Start(ctx); err != nil {
		return fmt.Errorf("failed to start strategy: %w", err)
	}
	app.logger.Strategy().WithField("is_running", app.serviceRegistry.CurrentStrategy.IsRunning()).Info("Strategy started successfully")

	// Start API server (always enabled)
	if app.serviceRegistry.APIServer != nil {
		go func() {
			if err := app.serviceRegistry.APIServer.Start(); err != nil {
				app.logger.System().Error("API server failed", "error", err)
			}
		}()
		app.logger.System().Info("API server started", "port", app.config.UI.Port)
	}

	// Start workers with panic recovery
	var wg sync.WaitGroup

	// Type assert to concrete logger for workers that haven't migrated to interface yet
	concreteLogger, ok := app.logger.(*logger.Logger)
	if !ok {
		return fmt.Errorf("logger is not *logger.Logger")
	}

	// Create workers
	marketDataWorker := worker.NewMarketDataWorker(
		concreteLogger,
		app.config.Trading.Symbols,
		app.marketDataCh,
		app,
		app.config.Worker.ReconnectIntervalSeconds,
		app.config.Worker.MaxReconnectIntervalSeconds,
		app.config.Worker.ConnectionCheckIntervalSeconds,
	)
	strategyWorker := worker.NewStrategyWorker(concreteLogger, app.serviceRegistry.CurrentStrategy, app.marketDataCh, app.signalCh)
	signalWorker := worker.NewSignalWorker(concreteLogger, app.signalCh, app, app, app.serviceRegistry.TradingService, app.serviceRegistry.CurrentStrategy, app)
	strategyMonitorWorker := worker.NewStrategyMonitorWorker(concreteLogger, app)
	maintenanceWorker := worker.NewMaintenanceWorker(concreteLogger, app.serviceRegistry.Database, app.config.DataRetention.RetentionDays)

	// Start all workers
	app.startWorker(&wg, ctx, "Market data worker", marketDataWorker.Run)
	app.startWorker(&wg, ctx, "Signal worker", signalWorker.Run)
	app.startWorker(&wg, ctx, "Strategy worker", strategyWorker.Run)
	app.startWorker(&wg, ctx, "Strategy monitor worker", strategyMonitorWorker.Run)
	app.startWorker(&wg, ctx, "Maintenance worker", maintenanceWorker.Run)

	// Wait for stop signal or context cancellation
	select {
	case <-ctx.Done():
		app.logger.System().Info("Context canceled, shutting down")
	case <-app.stopCh:
		app.logger.System().Info("Stop signal received, shutting down")
	}

	// Stop strategy with a fresh context (don't reuse potentially cancelled ctx)
	// Use a timeout context to prevent indefinite blocking
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	if err := app.serviceRegistry.CurrentStrategy.Stop(stopCtx); err != nil {
		app.logger.System().WithError(err).Error("Failed to stop strategy")
	}

	// Wait for all workers to finish
	wg.Wait()

	// Close data channels after all workers have stopped (prevents sending on closed channel)
	// These channels are owned by Application and should be closed here
	if app.marketDataCh != nil {
		close(app.marketDataCh)
	}
	if app.signalCh != nil {
		close(app.signalCh)
	}

	app.mu.Lock()
	app.isRunning = false
	app.mu.Unlock()

	app.logger.System().Info("Application stopped")

	// Close logger file handles
	if err := app.logger.Close(); err != nil {
		// Log to stderr as logger might be already closed
		fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
	}

	return nil
}

// startWorker starts a worker goroutine with panic recovery
func (app *Application) startWorker(wg *sync.WaitGroup, ctx context.Context, name string, runFn func(context.Context)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				app.logger.System().Error(name+" panicked", "panic", r)
			}
		}()
		runFn(ctx)
	}()
}


// Adapter methods implementing ClientFactory interface
// Following Single Responsibility Principle, these methods delegate to ConnectionManager

// IsConnected implements ClientFactory.IsConnected
func (app *Application) IsConnected() bool {
	if app.connManager == nil {
		return false
	}
	return app.connManager.IsConnected()
}

// ReconnectClient implements ClientFactory.ReconnectClient
func (app *Application) ReconnectClient() error {
	if app.connManager == nil {
		return fmt.Errorf("connection manager not initialized")
	}

	// Delegate to ConnectionManager
	if err := app.connManager.ReconnectClient(); err != nil {
		return fmt.Errorf("failed to reconnect bitFlyer client: %w", err)
	}

	// Update references to reconnected services
	app.serviceRegistry.TradingService = app.connManager.GetTradingService()

	// Re-register order completion callback
	app.serviceRegistry.TradingService.SetOnOrderCompleted(func(result *domain.OrderResult) {
		if app.serviceRegistry.CurrentStrategy != nil {
			app.serviceRegistry.CurrentStrategy.RecordTrade()
			app.logger.Strategy().WithField("order_id", result.OrderID).Info("Trade recorded in strategy")
		}
	})

	// Set strategy name if strategy is initialized
	if app.serviceRegistry.CurrentStrategy != nil {
		app.serviceRegistry.TradingService.SetStrategyName(app.serviceRegistry.CurrentStrategy.Name())
	}

	return nil
}

// SubscribeToTicker implements ClientFactory.SubscribeToTicker
func (app *Application) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	if app.connManager == nil {
		return fmt.Errorf("connection manager not initialized")
	}
	return app.connManager.SubscribeToTicker(ctx, symbol, callback)
}

// Adapter methods implementing worker.RiskChecker and worker.PerformanceUpdater interfaces
// Following Consumer-Driven Contracts pattern: interfaces are defined in worker package
// and Application implements them as an adapter to delegate to internal components

// CheckRiskManagement implements worker.RiskChecker interface
func (app *Application) CheckRiskManagement(ctx context.Context, signal *strategy.Signal) error {
	return app.serviceRegistry.RiskManager.CheckRiskManagement(ctx, signal)
}

// UpdateMetrics implements worker.PerformanceUpdater interface
func (app *Application) UpdateMetrics(ctx context.Context) error {
	return app.serviceRegistry.PerformanceAnalytics.UpdateMetrics(ctx)
}

// GetCurrentStrategy returns the current strategy
func (app *Application) GetCurrentStrategy() strategy.Strategy {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.serviceRegistry.CurrentStrategy
}


// IsTradingEnabled returns whether trading is enabled
func (app *Application) IsTradingEnabled() bool {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.tradingEnabled
}

// SetTradingEnabled sets trading enabled/disabled
func (app *Application) SetTradingEnabled(enabled bool) error {
	app.mu.Lock()
	if app.tradingEnabled == enabled {
		app.mu.Unlock()
		return nil
	}
	app.tradingEnabled = enabled
	app.mu.Unlock()

	// Persist state to database (outside of mutex lock to avoid holding lock during I/O)
	if app.serviceRegistry.Database != nil {
		value := "false"
		if enabled {
			value = "true"
		}
		if err := app.serviceRegistry.Database.SaveAppState("AppStateKeyTradingEnabled", value); err != nil {
			app.logger.System().WithError(err).Error("Failed to persist trading state")
			// Continue anyway, as the in-memory state is set
		}
	}

	// Log the state change (outside of mutex lock)
	if enabled {
		app.logger.System().Info("Trading started", "strategy", app.config.Trading.Strategy.Name)
		app.logger.Trading().Info("Trading enabled via API")
	} else {
		app.logger.System().Info("Trading stopped", "strategy", app.config.Trading.Strategy.Name)
		app.logger.Trading().Info("Trading disabled via API")
	}

	return nil
}

// Shutdown shuts down the application
func (app *Application) Shutdown(ctx context.Context) error {
	app.logger.System().Info("Shutting down application")

	// Send stop signal (only once to prevent panic)
	// This triggers workers to stop gracefully
	app.closeOnce.Do(func() {
		close(app.stopCh)
	})

	// Note: Workers are waited for in Run() method via wg.Wait()
	// If Shutdown is called independently (e.g., from API), we need to ensure
	// workers have finished before closing database and other resources.
	// However, the main shutdown path is through Run() which properly waits.

	// Shutdown sequence (order is critical):
	// 1. Close bitFlyer client (workers already stopped at this point)
	// 2. Flush logger to ensure all logs are written to database
	// 3. Remove database reference from logger
	// 4. Close database connection
	// 5. Close channels (owned by this application)

	// Step 1: Close bitFlyer client (safe to close after stop signal sent)
	// Delegate to ConnectionManager
	if app.connManager != nil {
		if err := app.connManager.Close(ctx); err != nil {
			app.logger.System().WithError(err).Error("Failed to close bitFlyer client")
		}
	}

	// Step 2: Flush logger to write any pending logs to database
	// This ensures all logs are persisted before database is closed
	app.logger.System().Info("Flushing logger before database shutdown")

	// Step 3: Remove database reference from logger BEFORE closing database
	// This prevents logger from attempting to write to closed database
	app.logger.SetDatabase(nil)

	// Step 4: Close database - this should only happen after all workers have stopped
	// In the main execution path, Run() waits for workers before calling this
	if app.serviceRegistry.Database != nil {
		if err := app.serviceRegistry.Database.Close(); err != nil {
			// Note: Cannot log to database at this point, only to stdout/stderr
			app.logger.System().WithError(err).Error("Failed to close database")
		}
	}

	// Step 5: Close channels (owned by Application)
	// Channels should be closed last to prevent any lingering goroutines from panicking
	// Note: stopCh is already closed via closeOnce.Do() at the beginning
	// marketDataCh and signalCh are closed by workers, not by Application
	// This is intentional - the producer (workers) should close the channels

	app.logger.System().Info("Application shutdown completed")
	return nil
}

// GetStatus gets the application status
func (app *Application) GetStatus() map[string]any {
	app.mu.RLock()
	defer app.mu.RUnlock()

	status := map[string]any{
		"is_running": app.isRunning,
		"symbols":    app.config.Trading.Symbols,
	}

	if app.serviceRegistry.CurrentStrategy != nil {
		status["strategy"] = map[string]any{
			"name":    app.serviceRegistry.CurrentStrategy.Name(),
			"status":  app.serviceRegistry.CurrentStrategy.GetStatus(),
			"metrics": app.serviceRegistry.CurrentStrategy.GetMetrics(),
		}
	}

	if app.connManager != nil {
		status["bitflyer_connected"] = app.connManager.IsConnected()
	}

	return status
}

// GetBalances gets the current balances
func (app *Application) GetBalances(ctx context.Context) ([]domain.Balance, error) {
	if app.serviceRegistry.TradingService == nil {
		return nil, fmt.Errorf("trading service not initialized")
	}

	bitflyerBalances, err := app.serviceRegistry.TradingService.GetBalance(ctx)
	if err != nil {
		return nil, err
	}

	// Convert bitflyer.Balance to domain.Balance struct
	var balances []domain.Balance
	for _, bal := range bitflyerBalances {
		balances = append(balances, domain.Balance{
			Currency:  bal.Currency,
			Amount:    bal.Amount,
			Available: bal.Available,
			Timestamp: time.Now(),
		})
	}

	return balances, nil
}

// Start starts the application
func (app *Application) Start(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.isRunning {
		return fmt.Errorf("application is already running")
	}

	app.logger.System().Info("Starting application...")

	// Start database connection
	if app.serviceRegistry.Database != nil {
		app.logger.System().Info("Database connected")
	}

	// Connect bitFlyer client
	if app.connManager != nil {
		app.logger.System().Info("bitFlyer client ready")
	}

	// API server is started in the Run() method, so don't start it here

	app.isRunning = true
	app.logger.System().Info("Application started successfully")

	return nil
}

// Stop stops the application
func (app *Application) Stop() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if !app.isRunning {
		return nil
	}

	app.logger.System().Info("Stopping application...")

	// Send stop signal (only once to prevent panic)
	app.closeOnce.Do(func() {
		close(app.stopCh)
	})

	// Stop API server gracefully
	if app.serviceRegistry.APIServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := app.serviceRegistry.APIServer.Shutdown(shutdownCtx); err != nil {
			app.logger.System().WithError(err).Error("Failed to shutdown API server gracefully")
		}
	}

	// Disconnect bitFlyer client
	if app.connManager != nil {
		app.logger.System().Info("bitFlyer client stopped")
	}

	app.isRunning = false
	app.logger.System().Info("Application stopped")

	return nil
}

// Close releases application resources
func (app *Application) Close() error {
	if err := app.Stop(); err != nil {
		return err
	}

	// Close bitFlyer client
	// Delegate to ConnectionManager
	if app.connManager != nil {
		ctx := context.Background()
		if err := app.connManager.Close(ctx); err != nil {
			app.logger.System().Error("Failed to close bitFlyer client", "error", err)
		}
	}

	// Close database connection
	if app.serviceRegistry.Database != nil {
		if err := app.serviceRegistry.Database.Close(); err != nil {
			app.logger.System().Error("Failed to close database", "error", err)
		}
	}

	return nil
}

// InitializeDatabase initializes the database
func (app *Application) InitializeDatabase() error {
	if app.serviceRegistry.Database == nil {
		return fmt.Errorf("database not initialized")
	}

	app.logger.System().Info("Initializing database...")

	// Create initial data if needed
	// Current JSON-based DB is automatically initialized, so no action needed here

	app.logger.System().Info("Database initialized successfully")
	return nil
}

