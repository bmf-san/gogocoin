package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bmf-san/gogocoin/v1/internal/api"
	"github.com/bmf-san/gogocoin/v1/internal/bitflyer"
	"github.com/bmf-san/gogocoin/v1/internal/config"
	"github.com/bmf-san/gogocoin/v1/internal/database"
	"github.com/bmf-san/gogocoin/v1/internal/domain"
	"github.com/bmf-san/gogocoin/v1/internal/logger"
	"github.com/bmf-san/gogocoin/v1/internal/strategy"
	"github.com/bmf-san/gogocoin/v1/internal/trading"
	"github.com/bmf-san/gogocoin/v1/internal/trading/live"
	"github.com/bmf-san/gogocoin/v1/internal/trading/paper"
)

// Application is the main application
type Application struct {
	config *config.Config
	logger *logger.Logger

	// Services
	bitflyerClient *bitflyer.Client
	marketDataSvc  *bitflyer.MarketDataService
	tradingSvc     trading.Trader // Trading execution interface
	db             *database.DB
	server         *api.Server

	// Strategy
	currentStrategy strategy.Strategy

	// State management
	isRunning      bool
	tradingEnabled bool
	mu             sync.RWMutex

	// Data channels
	marketDataCh chan bitflyer.MarketData
	signalCh     chan *strategy.Signal

	// Stop channel
	stopCh chan struct{}
}

// New creates a new application
func New(cfg *config.Config, log *logger.Logger) (*Application, error) {
	app := &Application{
		config:         cfg,
		logger:         log,
		tradingEnabled: false, // Always starts in stopped state, user must explicitly start
		marketDataCh:   make(chan bitflyer.MarketData, 100),
		signalCh:       make(chan *strategy.Signal, 100),
		stopCh:         make(chan struct{}),
	}

	// Initialize database
	if err := app.initDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize bitFlyer client
	if err := app.initBitflyerClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize bitFlyer client: %w", err)
	}

	// Initialize strategy
	if err := app.initStrategy(); err != nil {
		return nil, fmt.Errorf("failed to initialize strategy: %w", err)
	}

	// Initialize API server
	app.initAPIServer()

	return app, nil
}

// initDatabase initializes the database
func (app *Application) initDatabase() error {
	dbPath := "./data/gogocoin.db"

	db, err := database.NewDB(dbPath, app.logger)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	app.db = db

	// Set database for logger
	app.logger.SetDatabase(db)

	app.logger.System().Info("Database initialized successfully")
	return nil
}

// initAPIServer initializes the API server
func (app *Application) initAPIServer() {
	app.server = api.NewServer(app.config, app.db, app.logger)
	app.server.SetApplication(app) // Set application instance
	app.logger.System().Info("API server initialized successfully")
}

// initBitflyerClient initializes the bitFlyer client
func (app *Application) initBitflyerClient() error {
	// Client configuration
	clientConfig := bitflyer.Config{
		APIKey:            app.config.API.Credentials.APIKey,
		APISecret:         app.config.API.Credentials.APISecret,
		Endpoint:          app.config.API.Endpoint,
		WebSocketEndpoint: app.config.API.WebSocketEndpoint,
		Timeout:           app.config.API.Timeout,
		RetryCount:        app.config.API.RetryCount,
		RequestsPerMinute: app.config.API.RateLimit.RequestsPerMinute,
		PaperTrading:      app.config.IsPaperTrading(),
		InitialBalance:    app.config.Trading.InitialBalance,
		FeeRate:           app.config.Trading.FeeRate,
	}

	// Create client
	client, err := bitflyer.NewClient(&clientConfig, app.logger)
	if err != nil {
		return err
	}

	app.bitflyerClient = client

	// Create services
	app.marketDataSvc = bitflyer.NewMarketDataService(client, app.logger)

	// Create new trader based on mode
	if app.config.IsPaperTrading() {
		app.tradingSvc = paper.NewTrader(client, app.logger, app.config.Trading.InitialBalance, app.config.Trading.FeeRate)
	} else {
		app.tradingSvc = live.NewTrader(client, app.logger)
	}

	// Set database for each service
	app.tradingSvc.SetDatabase(app.db)
	app.marketDataSvc.SetDatabase(app.db)

	// Apply configuration to MarketDataService
	app.marketDataSvc.SetConfig(
		app.config.Data.MarketData.HistoryDays,
	)

	app.logger.System().Info("bitFlyer client initialized successfully")
	return nil
}

// initStrategy initializes the strategy
func (app *Application) initStrategy() error {
	strategyName := app.config.Trading.Strategy.Name

	// Create strategy factory
	factory := strategy.NewStrategyFactory()

	// risk management configuration" "準備
	riskConfig := strategy.RiskConfig{
		MaxTradeAmountPercent: app.config.Trading.RiskManagement.MaxTradeAmountPercent,
		InitialBalance:        app.config.Trading.InitialBalance,
		StopLossPercent:       app.config.Trading.RiskManagement.StopLossPercent,
		TakeProfitPercent:     app.config.Trading.RiskManagement.TakeProfitPercent,
	}

	// get strategy parameters（use nil for default values on error）
	params, err := app.config.GetStrategyParams(strategyName)
	if err != nil {
		params = nil // default値" "使用
	}

	// factory" "使ってstrategy" "作成
	createdStrategy, err := factory.CreateStrategy(strategyName, params, riskConfig)
	if err != nil {
		return fmt.Errorf("failed to create strategy %s: %w", strategyName, err)
	}

	app.currentStrategy = createdStrategy

	// TradingServiceにstrategy名" "configuration
	app.tradingSvc.SetStrategyName(strategyName)

	app.logger.Strategy().WithField("strategy", strategyName).Info("Strategy initialized successfully")
	return nil
}

// Run runs the application
func (app *Application) Run(ctx context.Context) error {
	app.mu.Lock()
	app.isRunning = true
	app.mu.Unlock()

	app.logger.System().Info("Starting gogocoin application")
	app.logger.System().Info("Trading started in stopped state. Please start trading via WebUI or API.")

	// tradingconfigurationofdiagnosis
	app.diagnoseTradingSetup()

	// Strategy" "開始
	app.logger.Strategy().Info("Starting strategy...")
	if err := app.currentStrategy.Start(ctx); err != nil {
		return fmt.Errorf("failed to start strategy: %w", err)
	}
	app.logger.Strategy().WithField("is_running", app.currentStrategy.IsRunning()).Info("Strategy started successfully")

	// APIserver" "開始（常に有効）
	if app.server != nil {
		go func() {
			if err := app.server.Start(); err != nil {
				app.logger.System().Error("API server failed", "error", err)
			}
		}()
		app.logger.System().Info("API server started", "port", app.config.API.Port)
	}

	// worker" "開始
	var wg sync.WaitGroup

	// market data収集worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.runMarketDataWorker(ctx)
	}()

	// signal処理worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.runSignalWorker(ctx)
	}()

	// Strategy実行worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.runStrategyWorker(ctx)
	}()

	// Strategy監視worker（定期的にstrategy状態" "チェックしてリセット）
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.runStrategyMonitorWorker(ctx)
	}()

	// 停止signalまたisコンテキストキャンセル" "待機
	select {
	case <-ctx.Done():
		app.logger.System().Info("Context canceled, shutting down")
	case <-app.stopCh:
		app.logger.System().Info("Stop signal received, shutting down")
	}

	// Strategy" "停止
	if err := app.currentStrategy.Stop(ctx); err != nil {
		app.logger.System().WithError(err).Error("Failed to stop strategy")
	}

	// 全workerof終了" "待機
	wg.Wait()

	app.mu.Lock()
	app.isRunning = false
	app.mu.Unlock()

	app.logger.System().Info("Application stopped")
	return nil
}

// runMarketDataWorker runs the market data collection worker
func (app *Application) runMarketDataWorker(ctx context.Context) {
	app.logger.Data().Info("Starting market data worker")

	// WebSocket reconnection loop
	reconnectInterval := 10 * time.Second
	maxReconnectInterval := 5 * time.Minute
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			app.logger.Data().Info("Market data worker stopped by context")
			return
		default:
		}

		// Check WebSocket connection status
		if !app.bitflyerClient.IsConnected() {
			app.logger.Data().Warn("WebSocket client is not connected - attempting reconnection")

			// Reconnect bitFlyer client
			if err := app.reconnectBitflyerClient(); err != nil {
				reconnectAttempts++
				wait := reconnectInterval * time.Duration(reconnectAttempts)
				if wait > maxReconnectInterval {
					wait = maxReconnectInterval
				}

				app.logger.Data().WithError(err).
					WithField("attempt", reconnectAttempts).
					WithField("retry_in_seconds", wait.Seconds()).
					Error("Failed to reconnect WebSocket client, retrying")

				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return
				}
			} else {
				app.logger.Data().Info("WebSocket client reconnected successfully")
				reconnectAttempts = 0 // Reset counter on successful connection
			}
		} else {
			app.logger.Data().Info("WebSocket client is connected - proceeding with subscriptions")
			reconnectAttempts = 0 // Reset counter
		}

		// Subscribe to ticker data for target symbols
		app.logger.Data().WithField("symbols", app.config.Trading.Symbols).Info("Attempting to subscribe to ticker data for symbols")
		subscribedCount := 0
		for _, symbol := range app.config.Trading.Symbols {
			app.logger.Data().WithField("symbol", symbol).Info("Subscribing to ticker data")

			err := app.marketDataSvc.SubscribeToTicker(ctx, symbol, func(data bitflyer.MarketData) {
				app.logger.Data().WithField("symbol", data.Symbol).WithField("price", data.Price).Info("Market data received in callback")

				select {
				case app.marketDataCh <- data:
					app.logger.Data().WithField("symbol", data.Symbol).Info("Market data sent to channel successfully")
				case <-ctx.Done():
					return
				default:
					// Drop old data if channel is full
					select {
					case <-app.marketDataCh:
						app.marketDataCh <- data
						app.logger.Data().WithField("symbol", data.Symbol).Info("Market data channel was full, dropped old data and sent new")
					default:
						app.logger.Data().WithField("symbol", data.Symbol).Warn("Market data channel is full, dropping new data")
					}
				}
			})

			if err != nil {
				app.logger.Data().WithError(err).WithField("symbol", symbol).Error("Failed to subscribe to ticker")
			} else {
				app.logger.Data().WithField("symbol", symbol).Info("Successfully subscribed to ticker data")
				subscribedCount++
			}
		}

		if subscribedCount == 0 {
			app.logger.Data().Error("No market data subscriptions successful - will retry connection")
			select {
			case <-time.After(reconnectInterval):
				continue // Retry subscription
			case <-ctx.Done():
				return
			}
		} else {
			app.logger.Data().WithField("subscribed_symbols", subscribedCount).WithField("total_symbols", len(app.config.Trading.Symbols)).Info("Market data subscriptions completed")
		}

		// Monitor connection health
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

	monitorLoop:
		for {
			select {
			case <-ticker.C:
				if !app.bitflyerClient.IsConnected() {
					app.logger.Data().Warn("WebSocket connection lost, initiating reconnection")
					break monitorLoop // Exit inner loop to reconnect
				}
			case <-ctx.Done():
				app.logger.Data().Info("Market data worker stopped")
				return
			}
		}
	}
}

// reconnectBitflyerClient reconnects the bitFlyer client
func (app *Application) reconnectBitflyerClient() error {
	app.logger.Data().Info("Reconnecting bitFlyer client...")

	// Close existing client
	if app.bitflyerClient != nil {
		if err := app.bitflyerClient.Close(context.Background()); err != nil {
			app.logger.Data().WithError(err).Warn("Failed to close existing bitFlyer client during reconnect")
		}
	}

	// Create new client
	clientConfig := bitflyer.Config{
		APIKey:            app.config.API.Credentials.APIKey,
		APISecret:         app.config.API.Credentials.APISecret,
		Endpoint:          app.config.API.Endpoint,
		WebSocketEndpoint: app.config.API.WebSocketEndpoint,
		Timeout:           app.config.API.Timeout,
		RetryCount:        app.config.API.RetryCount,
		RequestsPerMinute: app.config.API.RateLimit.RequestsPerMinute,
		PaperTrading:      app.config.IsPaperTrading(),
		InitialBalance:    app.config.Trading.InitialBalance,
		FeeRate:           app.config.Trading.FeeRate,
	}

	client, err := bitflyer.NewClient(&clientConfig, app.logger)
	if err != nil {
		return fmt.Errorf("failed to create new client: %w", err)
	}

	app.bitflyerClient = client

	// Recreate market data service
	app.marketDataSvc = bitflyer.NewMarketDataService(client, app.logger)
	app.marketDataSvc.SetDatabase(app.db)
	app.marketDataSvc.SetConfig(app.config.Data.MarketData.HistoryDays)

	app.logger.Data().Info("bitFlyer client reconnected successfully")
	return nil
}

// generateMockMarketData isfor testingofモックmarket data" "生成する

// GetCurrentStrategy is現在ofstrategyreturns
func (app *Application) GetCurrentStrategy() strategy.Strategy {
	return app.currentStrategy
}

// GetTradingService returns the trading service
func (app *Application) GetTradingService() interface{} {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.tradingSvc
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
	defer app.mu.Unlock()

	if app.tradingEnabled == enabled {
		// 既に同じ状態of場合is何もしない
		return nil
	}

	app.tradingEnabled = enabled

	if enabled {
		app.logger.System().Info("Trading started", "mode", app.config.Mode, "strategy", app.config.Trading.Strategy.Name)
		app.logger.Trading().Info("Trading enabled via API")
	} else {
		app.logger.System().Info("Trading stopped", "mode", app.config.Mode, "strategy", app.config.Trading.Strategy.Name)
		app.logger.Trading().Info("Trading disabled via API")
	}

	return nil
}

// runStrategyWorker runs the strategy execution worker
func (app *Application) runStrategyWorker(ctx context.Context) {
	app.logger.Strategy().Info("Starting strategy worker")

	var marketDataHistory []strategy.MarketData

	for {
		select {
		case <-ctx.Done():
			app.logger.Strategy().Info("Strategy worker stopped")
			return

		case marketData := <-app.marketDataCh:
			app.logger.Strategy().WithField("symbol", marketData.Symbol).WithField("price", marketData.Price).Info("Market data received in strategy worker")

			// historyに追加
			strategyData := strategy.MarketData{
				Symbol:    marketData.Symbol,
				Price:     marketData.Price,
				Volume:    marketData.Volume,
				BestBid:   marketData.BestBid,
				BestAsk:   marketData.BestAsk,
				Spread:    marketData.Spread,
				Timestamp: marketData.Timestamp,
			}

			marketDataHistory = append(marketDataHistory, strategyData)

			// historyサイズ" "制限（メモリ効率ofため）
			if len(marketDataHistory) > 1000 {
				marketDataHistory = marketDataHistory[len(marketDataHistory)-1000:]
			}

			app.logger.Strategy().WithField("symbol", marketData.Symbol).WithField("history_count", len(marketDataHistory)).Info("Market data added to history")

			// market data受信時にstrategy" "実行
			app.logger.Strategy().WithField("symbol", marketData.Symbol).Info("Executing strategy on market data update")
			app.executeStrategy(ctx, &marketData, marketDataHistory)
		}
	}
}

// executeStrategy isstrategyexecutes
func (app *Application) executeStrategy(ctx context.Context, marketData *bitflyer.MarketData, history []strategy.MarketData) {
	// Strategydataに変換
	strategyData := strategy.MarketData{
		Symbol:    marketData.Symbol,
		Price:     marketData.Price,
		Volume:    marketData.Volume,
		BestBid:   marketData.BestBid,
		BestAsk:   marketData.BestAsk,
		Spread:    marketData.Spread,
		Timestamp: marketData.Timestamp,
	}

	// signal生成（historydata" "含めて分析）
	// 元ofスライス" "変更せずに新しいスライス" "作成
	historyWithCurrent := make([]strategy.MarketData, len(history)+1)
	copy(historyWithCurrent, history)
	historyWithCurrent[len(history)] = strategyData
	signal, err := app.currentStrategy.Analyze(historyWithCurrent)
	if err != nil {
		app.logger.Strategy().WithError(err).Error("Failed to analyze market data")
		return
	}

	// デバッグ情報" "log出力（全メタdata" "出力）
	logEntry := app.logger.Strategy().WithField("symbol", marketData.Symbol).
		WithField("price", marketData.Price).
		WithField("signal", signal.Action)

	// メタdata" "展開してlog出力
	for key, value := range signal.Metadata {
		logEntry = logEntry.WithField(key, value)
	}

	logEntry.Debug("Strategy analysis completed")

	if signal.Action != strategy.SignalHold {
		app.logger.LogStrategySignal(
			app.currentStrategy.Name(),
			signal.Symbol,
			string(signal.Action),
			signal.Strength,
			signal.Metadata,
		)

		// signal" "処理キューに送信
		select {
		case app.signalCh <- signal:
		case <-ctx.Done():
			return
		default:
			app.logger.Strategy().Warn("Signal channel is full, dropping signal")
		}
	}
}

// runSignalWorker issignal処理workerexecutes
func (app *Application) runSignalWorker(ctx context.Context) {
	app.logger.Trading().Info("Starting signal worker")

	for {
		select {
		case <-ctx.Done():
			app.logger.Trading().Info("Signal worker stopped")
			return

		case signal := <-app.signalCh:
			app.processSignal(ctx, signal)
		}
	}
}

// processSignal issignal" "処理する
func (app *Application) processSignal(ctx context.Context, signal *strategy.Signal) {
	if signal.Action == strategy.SignalHold {
		return
	}

	app.logger.Trading().WithField("action", string(signal.Action)).WithField("symbol", signal.Symbol).WithField("price", signal.Price).Info("Processing trading signal")

	// trading前チェック
	if !app.IsTradingEnabled() {
		app.logger.Trading().Warn("Trading is disabled, skipping signal")
		app.logger.Trading().Info("To enable trading, use the Web UI or API")
		return
	}

	// modeチェック
	switch app.config.Mode {
	case "paper":
		app.logger.Trading().Info("Paper trading mode - executing simulated order")
	case "live":
		app.logger.Trading().Info("Live trading mode - executing real order")
	case "dev":
		app.logger.Trading().Info("Development mode - executing simulated order with debug features")
	}

	// Risk management check
	if err := app.checkRiskManagement(signal); err != nil {
		app.logger.Trading().WithError(err).Warn("Risk management check failed - order rejected")
		return
	}

	// order" "作成
	order := app.createOrderFromSignal(signal)
	app.logger.Trading().WithField("order", order).Info("Order created from signal")

	// Execute order
	result, err := app.tradingSvc.PlaceOrder(ctx, &order)
	if err != nil {
		app.logger.Trading().WithError(err).Error("Failed to place order")
		app.logger.Trading().Info("Check API credentials and account permissions")
		return
	}

	app.logger.Trading().WithField("order_id", result.OrderID).Info("Order placed successfully")

	app.logger.LogTrade(
		string(signal.Action),
		signal.Symbol,
		signal.Price,
		signal.Quantity,
		map[string]interface{}{
			"order_id":        result.OrderID,
			"strategy":        app.currentStrategy.Name(),
			"signal_strength": signal.Strength,
		},
	)

	// trading実行後にperformancemetrics" "更新
	go app.updatePerformanceMetrics()
}

// checkRiskManagement isrisk management" "チェックする
func (app *Application) checkRiskManagement(signal *strategy.Signal) error {
	// trading金額チェック
	tradeAmount := signal.Price * signal.Quantity
	maxAmount := app.config.Trading.InitialBalance * app.config.Trading.RiskManagement.MaxTradeAmountPercent / 100

	if tradeAmount > maxAmount {
		return fmt.Errorf("trade amount %f exceeds maximum %f", tradeAmount, maxAmount)
	}

	// 追加ofrisk managementチェック

	// 1. 日次trading回数制限チェック
	if err := app.checkDailyTradeLimit(); err != nil {
		return fmt.Errorf("daily trade limit exceeded: %w", err)
	}

	// 2. 連続trading間隔制限チェック
	if err := app.checkTradeInterval(); err != nil {
		return fmt.Errorf("trade interval too short: %w", err)
	}

	// 3. 総損失制限チェック
	if err := app.checkTotalLossLimit(); err != nil {
		return fmt.Errorf("total loss limit exceeded: %w", err)
	}

	return nil
}

// createOrderFromSignal creates an order from a signal
func (app *Application) createOrderFromSignal(signal *strategy.Signal) domain.OrderRequest {
	var side string
	var size float64

	switch signal.Action {
	case strategy.SignalBuy:
		side = "BUY"
		size = signal.Quantity
	case strategy.SignalSell:
		side = "SELL"
		// SELL時is実際of保有量getして調整
		size = app.getAvailableSellSize(signal.Symbol, signal.Quantity)
	default:
		side = "BUY" // default
		size = signal.Quantity
	}

	return domain.OrderRequest{
		Symbol:      signal.Symbol,
		Side:        side,
		Type:        "MARKET", // とりあえず成行order
		Size:        size,
		Price:       signal.Price,
		TimeInForce: "IOC", // Immediate or Cancel
	}
}

// getAvailableSellSize is売却可能なsizegets
func (app *Application) getAvailableSellSize(symbol string, requestedSize float64) float64 {
	// シンボルfrom通貨" "抽出（例: "BTC_JPY" -> "BTC"）
	currency := symbol
	if len(symbol) > 3 {
		currency = symbol[:len(symbol)-4] // "_JPY" " "除去
	}

	// 現在ofbalanceget
	balances, err := app.tradingSvc.GetBalance(context.Background())
	if err != nil {
		app.logger.Trading().WithError(err).Error("Failed to get balance for SELL order")
		return 0
	}

	// 該当通貨ofbalance" "探す
	var availableBalance float64
	for i := range balances {
		if balances[i].Currency == currency {
			availableBalance = balances[i].Available
			break
		}
	}

	// 保有量がない場合is0returns
	if availableBalance == 0 {
		app.logger.Trading().WithField("symbol", symbol).WithField("currency", currency).
			Warn("No available balance for SELL order")
		return 0
	}

	// 要求サイズと保有量of小さい方returns
	if requestedSize > 0 && requestedSize < availableBalance {
		return requestedSize
	}

	// 保有量of95%" "売却（全量だと誤差witherrorになる可能性があるため）
	return availableBalance * 0.95
}

// Shutdown isアプリケーション" "シャットダウンする
func (app *Application) Shutdown(ctx context.Context) error {
	app.logger.System().Info("Shutting down application")

	// bitFlyerclient" "クローズ
	if app.bitflyerClient != nil {
		if err := app.bitflyerClient.Close(ctx); err != nil {
			app.logger.System().WithError(err).Error("Failed to close bitFlyer client")
		}
	}

	// ロガーfromdatabase参照" "削除
	app.logger.SetDatabase(nil)

	// database" "クローズ
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			app.logger.System().WithError(err).Error("Failed to close database")
		}
	}

	app.logger.System().Info("Application shutdown completed")
	return nil
}

// GetStatus isアプリケーションof状態gets
func (app *Application) GetStatus() map[string]interface{} {
	app.mu.RLock()
	defer app.mu.RUnlock()

	status := map[string]interface{}{
		"is_running": app.isRunning,
		"mode":       app.config.Mode,
		"symbols":    app.config.Trading.Symbols,
	}

	if app.currentStrategy != nil {
		status["strategy"] = map[string]interface{}{
			"name":    app.currentStrategy.Name(),
			"status":  app.currentStrategy.GetStatus(),
			"metrics": app.currentStrategy.GetMetrics(),
		}
	}

	if app.bitflyerClient != nil {
		status["bitflyer_connected"] = app.bitflyerClient.IsConnected()
	}

	return status
}

// GetBalances is現在ofbalancegets（papertradeof場合is実際ofpaperbalancereturns）
func (app *Application) GetBalances(ctx context.Context) ([]api.Balance, error) {
	if app.tradingSvc == nil {
		return nil, fmt.Errorf("trading service not initialized")
	}

	bitflyerBalances, err := app.tradingSvc.GetBalance(ctx)
	if err != nil {
		return nil, err
	}

	// bitflyer.Balance" "Balance構造体に変換
	var balances []api.Balance
	for _, bal := range bitflyerBalances {
		balances = append(balances, api.Balance{
			Currency:  bal.Currency,
			Amount:    bal.Amount,
			Available: bal.Available,
		})
	}

	return balances, nil
}

// Start isアプリケーションstarts
func (app *Application) Start(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.isRunning {
		return fmt.Errorf("application is already running")
	}

	app.logger.System().Info("Starting application...")

	// Database connection開始
	if app.db != nil {
		app.logger.System().Info("Database connected")
	}

	// bitFlyerclientconnection
	if app.bitflyerClient != nil {
		app.logger.System().Info("bitFlyer client ready")
	}

	// APIserverisRun()メソッドwith起動されるため、ここwithis起動しない

	app.isRunning = true
	app.logger.System().Info("Application started successfully")

	return nil
}

// Stop isアプリケーションstops
func (app *Application) Stop() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if !app.isRunning {
		return nil
	}

	app.logger.System().Info("Stopping application...")

	// 停止signal送信
	close(app.stopCh)

	// APIserver停止
	if app.server != nil {
		app.logger.System().Info("API server stopped")
	}

	// bitFlyerclient切断
	if app.bitflyerClient != nil {
		app.logger.System().Info("bitFlyer client stopped")
	}

	app.isRunning = false
	app.logger.System().Info("Application stopped")

	return nil
}

// Close isアプリケーションリソース" "解放する
func (app *Application) Close() error {
	if err := app.Stop(); err != nil {
		return err
	}

	// bitFlyer client" "クローズ
	if app.bitflyerClient != nil {
		ctx := context.Background()
		if err := app.bitflyerClient.Close(ctx); err != nil {
			app.logger.System().Error("Failed to close bitFlyer client", "error", err)
		}
	}

	// Database connectioncloses
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			app.logger.System().Error("Failed to close database", "error", err)
		}
	}

	return nil
}

// InitializeDatabase isdatabaseinitializes
func (app *Application) InitializeDatabase() error {
	if app.db == nil {
		return fmt.Errorf("database not initialized")
	}

	app.logger.System().Info("Initializing database...")

	// 初期dataof作成など、必要に応じて実装
	// 現在ofJSONベースDBis自動的に初期化されるため、特に何もしない

	app.logger.System().Info("Database initialized successfully")
	return nil
}

// diagnoseTradingSetup istradingconfiguration" "diagnosisする
func (app *Application) diagnoseTradingSetup() {
	app.logger.System().Info("=== Trading Setup Diagnosis ===")

	// 1. trading有効性チェック（動的制御）
	if app.IsTradingEnabled() {
		app.logger.System().Info("✓ Trading is ENABLED (dynamic control)")
	} else {
		app.logger.System().Warn("✗ Trading is DISABLED (dynamic control)")
		app.logger.System().Info("  To enable: use Web UI or API /api/trading/start")
	}

	// 2. tradingmodeチェック
	app.logger.System().WithField("mode", app.config.Mode).Info("Trading mode configured")
	switch app.config.Mode {
	case "paper":
		app.logger.System().Info("  Paper trading: Orders will be simulated (no real money)")
	case "live":
		app.logger.System().Info("  Live trading: Orders will use real money - BE CAREFUL!")
	case "dev":
		app.logger.System().Info("  Development mode: Paper trading with debug features")
	}

	// 3. APIcredentialsチェック
	hasAPIKey := app.config.API.Credentials.APIKey != "" && app.config.API.Credentials.APIKey != "${BITFLYER_API_KEY}"
	hasAPISecret := app.config.API.Credentials.APISecret != "" && app.config.API.Credentials.APISecret != "${BITFLYER_API_SECRET}"

	if hasAPIKey && hasAPISecret {
		app.logger.System().Info("✓ API credentials are configured")
		if app.config.Mode == "live" {
			app.logger.System().Info("  Live trading credentials ready")
		}
	} else {
		app.logger.System().Warn("✗ API credentials are missing or not expanded")
		app.logger.System().Info("  Required environment variables:")
		app.logger.System().Info("    export BITFLYER_API_KEY='your_api_key'")
		app.logger.System().Info("    export BITFLYER_API_SECRET='your_api_secret'")
		if app.config.Mode == "live" {
			app.logger.System().Error("  ⚠️  CRITICAL: Live trading requires valid API credentials!")
			app.logger.System().Error("  ⚠️  WITHOUT CREDENTIALS, SYSTEM WILL FALLBACK TO PAPER MODE!")
			app.logger.System().Error("  ⚠️  This means orders will be simulated, not real!")
		}
	}

	// 4. 対象シンボルチェック
	app.logger.System().WithField("symbols", app.config.Trading.Symbols).Info("Target trading symbols")
	if len(app.config.Trading.Symbols) == 0 {
		app.logger.System().Warn("✗ No trading symbols configured")
	}

	// 5. strategyチェック
	app.logger.System().WithField("strategy", app.config.Trading.Strategy.Name).Info("Trading strategy")

	// 6. risk management configuration
	app.logger.System().Info("Risk management settings:")
	app.logger.System().WithField("max_trade_amount_percent", app.config.Trading.RiskManagement.MaxTradeAmountPercent).Info("  Max trade amount per order")
	app.logger.System().WithField("stop_loss_percent", app.config.Trading.RiskManagement.StopLossPercent).Info("  Stop loss percentage")
	app.logger.System().WithField("take_profit_percent", app.config.Trading.RiskManagement.TakeProfitPercent).Info("  Take profit percentage")

	// 7. WebSocketconnectiontest
	if app.bitflyerClient != nil {
		if app.bitflyerClient.IsConnected() {
			app.logger.System().Info("✓ WebSocket connection is active")
		} else {
			app.logger.System().Warn("✗ WebSocket connection is not active")
			app.logger.System().Info("  Market data may not be available")
		}
	}

	// 8. strategy初期化状態チェック
	if app.currentStrategy != nil {
		if app.currentStrategy.IsRunning() {
			app.logger.System().Info("✓ Strategy is initialized and running")
		} else {
			app.logger.System().Warn("✗ Strategy is not running")
		}
	} else {
		app.logger.System().Error("✗ No strategy initialized")
	}

	app.logger.System().Info("=== End Diagnosis ===")
}

// checkDailyTradeLimit is日次trading回数制限" "チェックする
func (app *Application) checkDailyTradeLimit() error {
	// 今日oftrading数get
	today := time.Now().Truncate(24 * time.Hour)
	trades, err := app.db.GetRecentTrades(100) // 最近of100件get
	if err != nil {
		return fmt.Errorf("failed to get recent trades: %w", err)
	}

	// 今日oftrading" "カウント
	todayTrades := 0
	for i := range trades {
		if trades[i].CreatedAt.Truncate(24 * time.Hour).Equal(today) {
			todayTrades++
		}
	}

	maxDailyTrades := app.config.Trading.RiskManagement.MaxDailyTrades
	if todayTrades >= maxDailyTrades {
		return fmt.Errorf("daily trade limit reached: %d/%d", todayTrades, maxDailyTrades)
	}

	return nil
}

// checkTradeInterval is連続trading間隔制限" "チェックする
func (app *Application) checkTradeInterval() error {
	// 最後oftrading時刻get
	trades, err := app.db.GetRecentTrades(1)
	if err != nil {
		return fmt.Errorf("failed to get recent trades: %w", err)
	}

	if len(trades) == 0 {
		return nil // 初回tradingof場合isOK
	}

	lastTradeTime := trades[0].CreatedAt
	minInterval := app.config.Trading.RiskManagement.MinTradeInterval

	// configurationfrom時間間隔" "解析（例: "5m" -> 5分）
	duration, err := time.ParseDuration(minInterval)
	if err != nil || minInterval == "" {
		app.logger.System().WithError(err).Warn("Invalid min_trade_interval format, using default 5m")
		duration = 5 * time.Minute
	}

	if time.Since(lastTradeTime) < duration {
		return fmt.Errorf("trade interval too short: %v < %v", time.Since(lastTradeTime), duration)
	}

	return nil
}

// checkTotalLossLimit is総損失制限" "チェックする
func (app *Application) checkTotalLossLimit() error {
	// 最近ofperformancemetricsget
	metrics, err := app.db.GetPerformanceMetrics(30) // 過去30日
	if err != nil {
		return fmt.Errorf("failed to get performance metrics: %w", err)
	}

	if len(metrics) == 0 {
		return nil // dataがない場合isOK
	}

	// 最新of総PnL" "確認
	latestMetric := metrics[len(metrics)-1]
	totalLoss := -latestMetric.TotalPnL // 負of値が損失

	if totalLoss <= 0 {
		return nil // 損失がない場合isOK
	}

	// 初期balanceに対する損失率" "計算
	lossPercent := (totalLoss / app.config.Trading.InitialBalance) * 100
	maxLossPercent := app.config.Trading.RiskManagement.MaxTotalLossPercent

	if lossPercent > maxLossPercent {
		return fmt.Errorf("total loss limit exceeded: %.2f%% > %.2f%%", lossPercent, maxLossPercent)
	}

	return nil
}

// runStrategyMonitorWorker isstrategyof状態" "監視し、必要に応じてリセットする
func (app *Application) runStrategyMonitorWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // 5分間隔withチェック
	defer ticker.Stop()

	app.logger.System().Info("Strategy monitor worker started")

	for {
		select {
		case <-ctx.Done():
			app.logger.System().Info("Strategy monitor worker stopped")
			return
		case <-ticker.C:
			app.checkAndResetStrategy()
		}
	}
}

// checkAndResetStrategy is no longer needed as we only have scalping strategy
// This function is kept as a placeholder for future strategy-specific logic if needed
func (app *Application) checkAndResetStrategy() {
	if app.currentStrategy == nil {
		return
	}
	// No strategy-specific checks needed for scalping
}

// updatePerformanceMetrics isperformancemetrics" "計算・更新する
func (app *Application) updatePerformanceMetrics() {
	app.logger.System().Debug("Calculating performance metrics")

	// 最近oftradingdataget
	trades, err := app.db.GetRecentTrades(1000) // 最大1000件
	if err != nil {
		app.logger.System().WithError(err).Error("Failed to get recent trades for performance calculation")
		return
	}

	if len(trades) == 0 {
		app.logger.System().Debug("No trades found for performance calculation")
		return
	}

	// performancemetrics" "計算
	metrics := app.calculatePerformanceFromTrades(trades)

	// Save to database
	if err := app.db.SavePerformanceMetric(&metrics); err != nil {
		app.logger.System().WithError(err).Error("Failed to save performance metrics")
	} else {
		app.logger.System().Info("Performance metrics updated",
			"total_return", metrics.TotalReturn,
			"win_rate", metrics.WinRate,
			"total_trades", metrics.TotalTrades)
	}
}

// calculatePerformanceFromTrades istradingdatafromperformancemetricscalculates
func (app *Application) calculatePerformanceFromTrades(trades []domain.Trade) database.PerformanceMetric {
	if len(trades) == 0 {
		return database.PerformanceMetric{Date: time.Now()}
	}

	var totalPnL float64
	var totalReturn float64
	var winningTrades, losingTrades int
	var totalWin, totalLoss float64
	var maxWin, maxLoss float64
	var returns []float64

	initialBalance := app.config.Trading.InitialBalance

	// 各tradingofPnL" "計算
	for i := range trades {
		trade := &trades[i]

		// Save to databaseされたPnL" "使用
		// papermode: SELL時に計算されたPnL
		// livemode: 実際oftradingofPnL
		pnl := trade.PnL

		// PnLが0of場合of処理" "改善
		if pnl == 0 {
			switch trade.Side {
			case "BUY":
				// BUYtradingis手数料分ofみ損失として計上
				pnl = -trade.Fee
			case "SELL":
				// SELLtradingwithPnL=0of場合is手数料分ofみ損失
				pnl = -trade.Fee
			}
		}

		totalPnL += pnl

		// 勝敗判定" "改善（手数料" "考慮した実質的なPnLwith判定）
		if pnl > 0.01 { // 1銭以上of利益
			winningTrades++
			totalWin += pnl
			if pnl > maxWin {
				maxWin = pnl
			}
		} else if pnl < -0.01 { // 1銭以上of損失
			losingTrades++
			totalLoss += -pnl
			if -pnl > maxLoss {
				maxLoss = -pnl
			}
		}
		// -0.01円from0.01円of間is引き分け扱い（勝敗にカウントしない）

		// リターン率" "計算
		returnRate := pnl / initialBalance
		returns = append(returns, returnRate)
	}

	totalReturn = totalPnL / initialBalance * 100

	// 勝率計算
	var winRate float64
	totalTrades := len(trades)
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}

	// 平均勝ち/負け計算
	var avgWin, avgLoss float64
	if winningTrades > 0 {
		avgWin = totalWin / float64(winningTrades)
	}
	if losingTrades > 0 {
		avgLoss = totalLoss / float64(losingTrades)
	}

	// プロフィットファクター計算
	var profitFactor float64
	if totalLoss > 0 {
		profitFactor = totalWin / totalLoss
	}

	// シャープレシオof簡易計算
	var sharpeRatio float64
	if len(returns) > 1 {
		mean := totalReturn / float64(len(returns))
		var variance float64
		for _, r := range returns {
			variance += (r*100 - mean) * (r*100 - mean)
		}
		variance /= float64(len(returns) - 1)
		stdDev := variance
		if stdDev > 0 {
			sharpeRatio = mean / stdDev
		}
	}

	// 最大ドローダウンof計算
	var maxDrawdown float64
	var peak float64
	runningPnL := 0.0
	for i := range trades {
		pnl := trades[i].PnL
		if pnl == 0 && trades[i].Fee > 0 {
			pnl = -trades[i].Fee
		}
		runningPnL += pnl
		if runningPnL > peak {
			peak = runningPnL
		}
		drawdown := (peak - runningPnL) / initialBalance * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return database.PerformanceMetric{
		Date:          time.Now(),
		TotalReturn:   totalReturn,
		TotalPnL:      totalPnL,
		WinRate:       winRate,
		MaxDrawdown:   maxDrawdown,
		SharpeRatio:   sharpeRatio,
		ProfitFactor:  profitFactor,
		TotalTrades:   totalTrades,
		WinningTrades: winningTrades,
		LosingTrades:  losingTrades,
		AverageWin:    avgWin,
		AverageLoss:   avgLoss,
		LargestWin:    maxWin,
		LargestLoss:   maxLoss,
	}
}
