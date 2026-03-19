package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	adapterhttp "github.com/bmf-san/gogocoin/internal/adapter/http"
	adapterworker "github.com/bmf-san/gogocoin/internal/adapter/worker"
	"github.com/bmf-san/gogocoin/internal/config"
	"github.com/bmf-san/gogocoin/internal/domain"
	"github.com/bmf-san/gogocoin/internal/infra/exchange/bitflyer"
	"github.com/bmf-san/gogocoin/internal/infra/persistence"
	"github.com/bmf-san/gogocoin/internal/logger"
	"github.com/bmf-san/gogocoin/internal/usecase/analytics"
	"github.com/bmf-san/gogocoin/internal/usecase/risk"
	trading "github.com/bmf-san/gogocoin/internal/usecase/trading"
	"github.com/bmf-san/gogocoin/internal/utils"
	pkgstrategy "github.com/bmf-san/gogocoin/pkg/strategy"
)

// Run loads configuration, wires all components, and blocks until ctx is cancelled.
// Callers register their strategy implementation(s) via WithStrategy() options.
func Run(ctx context.Context, opts ...Option) error {
	ec := newEngineConfig()
	for _, o := range opts {
		o(ec)
	}

	// ── Load config ───────────────────────────────────────────────────────────
	cfg, err := config.Load(ec.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config %q: %w", ec.configPath, err)
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	log, err := logger.New(&logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAgeDays: cfg.Logging.MaxAgeDays,
		Categories: cfg.Logging.Categories,
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() { _ = log.Close() }()

	return run(ctx, cfg, log, ec)
}

// RunWithLogger is like Run but uses a pre-created logger (useful when the
// caller wants to initialise logging before Run, e.g. for startup messages).
func RunWithLogger(ctx context.Context, log logger.LoggerInterface, opts ...Option) error {
	ec := newEngineConfig()
	for _, o := range opts {
		o(ec)
	}
	cfg, err := config.Load(ec.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config %q: %w", ec.configPath, err)
	}
	return run(ctx, cfg, log, ec)
}

// run contains the actual wiring logic shared by Run and RunWithLogger.
func run(ctx context.Context, cfg *config.Config, log logger.LoggerInterface, ec *engineConfig) error {
	// usecase/trading still requires *logger.Logger concretely.
	concreteLog, ok := log.(*logger.Logger)
	if !ok {
		return fmt.Errorf("logger must be *logger.Logger (got %T)", log)
	}

	// ── 1. Database ───────────────────────────────────────────────────────────
	db, err := persistence.NewDB("./data/gogocoin.db", log)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	repo := persistence.NewRepository(db)
	log.SetDatabase(repo)
	defer func() {
		log.SetDatabase(nil)
		if err := db.Close(); err != nil {
			log.System().WithError(err).Error("Failed to close database")
		}
	}()

	// ── 2. TradingController ──────────────────────────────────────────────────
	tradingCtrl, err := newTradingController(repo, log)
	if err != nil {
		return fmt.Errorf("failed to create trading controller: %w", err)
	}

	// ── 3. bitFlyer client + services ─────────────────────────────────────────
	bfClient, err := bitflyer.NewClient(&bitflyer.Config{
		APIKey:            cfg.API.Credentials.APIKey,
		APISecret:         cfg.API.Credentials.APISecret,
		Endpoint:          cfg.API.Endpoint,
		WebSocketEndpoint: cfg.API.WebSocketEndpoint,
		Timeout:           cfg.API.Timeout,
		RetryCount:        cfg.API.RetryCount,
		RequestsPerMinute: cfg.API.RateLimit.RequestsPerMinute,
		InitialBalance:    cfg.Trading.InitialBalance,
		FeeRate:           cfg.Trading.FeeRate,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to create bitFlyer client: %w", err)
	}
	defer func() { _ = bfClient.Close(context.Background()) }()

	bfMarketDataSvc := bitflyer.NewMarketDataService(
		bfClient, log, repo,
		&bitflyer.MarketDataConfig{HistoryDays: cfg.Data.MarketData.HistoryDays},
		cfg.Worker.MaxConcurrentSaves,
	)
	bfMarketSpecSvc := bitflyer.NewMarketSpecificationService(bfClient)

	// ── 4. Trader ─────────────────────────────────────────────────────────────
	if len(cfg.Trading.Symbols) == 0 {
		return fmt.Errorf("trading.symbols must not be empty")
	}
	trader := trading.NewTraderWithDependencies(
		bfClient, concreteLog, repo, bfMarketSpecSvc,
		cfg.Trading.Strategy.Name, cfg.Trading.Symbols[0],
	)

	// ── 5. Strategy ───────────────────────────────────────────────────────────
	strat, err := buildStrategy(cfg, ec.registry)
	if err != nil {
		return fmt.Errorf("failed to build strategy: %w", err)
	}
	if err := strat.Start(ctx); err != nil {
		return fmt.Errorf("failed to start strategy: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = strat.Stop(stopCtx)
	}()

	initDailyTradeCount(repo, strat, log)

	// ── 6. Risk manager ───────────────────────────────────────────────────────
	minInterval, _ := time.ParseDuration(cfg.Trading.RiskManagement.MinTradeInterval)
	riskMgr := risk.NewRiskManager(
		risk.ManagerConfig{
			MaxTotalLossPercent:   cfg.Trading.RiskManagement.MaxTotalLossPercent,
			MaxTradeLossPercent:   cfg.Trading.RiskManagement.MaxTradeLossPercent,
			MaxDailyLossPercent:   cfg.Trading.RiskManagement.MaxDailyLossPercent,
			MaxTradeAmountPercent: cfg.Trading.RiskManagement.MaxTradeAmountPercent,
			MaxDailyTrades:        cfg.Trading.RiskManagement.MaxDailyTrades,
			MinTradeInterval:      minInterval,
			FeeRate:               cfg.Trading.FeeRate,
			InitialBalance:        cfg.Trading.InitialBalance,
		},
		repo, repo, trader, log,
	)

	// ── 7. Performance analytics ──────────────────────────────────────────────
	perfAnalytics := analytics.NewPerformanceAnalytics(repo, repo, log, cfg.Trading.InitialBalance)

	// ── 8. HTTP server ────────────────────────────────────────────────────────
	appSvc := &appServiceAdapter{tc: tradingCtrl, trader: trader, strat: strat}
	httpServer := adapterhttp.NewServer(cfg, repo, log)
	httpServer.SetApplication(appSvc)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.System().Error("HTTP server error", "error", err)
		}
	}()

	// ── 9. Channels ───────────────────────────────────────────────────────────
	marketDataCh := make(chan domain.MarketData, cfg.Worker.MarketDataChannelBuffer)
	signalCh := make(chan *pkgstrategy.Signal, cfg.Worker.SignalChannelBuffer)

	// ── 10. Workers ───────────────────────────────────────────────────────────
	clientFactory := &marketDataAdapter{client: bfClient, marketDataSvc: bfMarketDataSvc}

	marketDataWorker := adapterworker.NewMarketDataWorker(
		log, cfg.Trading.Symbols, marketDataCh, clientFactory,
		cfg.Worker.ReconnectIntervalSeconds,
		cfg.Worker.MaxReconnectIntervalSeconds,
		cfg.Worker.ConnectionCheckIntervalSeconds,
		cfg.Worker.StaleDataTimeoutSeconds,
	)
	strategyWorker := adapterworker.NewStrategyWorker(log, strat, marketDataCh, signalCh)
	signalWorker := adapterworker.NewSignalWorker(
		log, signalCh, tradingCtrl, riskMgr, trader, strat, perfAnalytics,
		cfg.Runtime.SellSizePercentage,
	)
	strategyMonitor := adapterworker.NewStrategyMonitorWorker(log, &strategyGetter{strat: strat})
	maintenanceWorker := adapterworker.NewMaintenanceWorker(log, repo, cfg.DataRetention.RetentionDays, repo)

	wm := adapterworker.NewWorkerManager(log)
	_ = wm.Register(marketDataWorker.Name(), marketDataWorker)
	_ = wm.Register(strategyWorker.Name(), strategyWorker)
	_ = wm.Register(signalWorker.Name(), signalWorker)
	_ = wm.Register(strategyMonitor.Name(), strategyMonitor)
	_ = wm.Register(maintenanceWorker.Name(), maintenanceWorker)

	if err := wm.StartAll(ctx); err != nil {
		return fmt.Errorf("failed to start workers: %w", err)
	}

	log.System().Info("gogocoin started",
		"strategy", cfg.Trading.Strategy.Name,
		"port", cfg.UI.Port,
	)

	// ── 11. Wait for shutdown ─────────────────────────────────────────────────
	<-ctx.Done()
	log.System().Info("Shutdown signal received, stopping...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.System().WithError(err).Error("HTTP server shutdown error")
	}
	if err := wm.StopAll(); err != nil {
		log.System().WithError(err).Warn("Worker manager shutdown error")
	}
	close(marketDataCh)
	close(signalCh)

	log.System().Info("gogocoin shut down successfully")
	return nil
}

// buildStrategy creates and initialises the strategy from the registry.
func buildStrategy(cfg *config.Config, registry *pkgstrategy.Registry) (pkgstrategy.Strategy, error) {
	name := cfg.Trading.Strategy.Name

	strat, err := registry.Create(name)
	if err != nil {
		return nil, fmt.Errorf("strategy %q not registered: %w", name, err)
	}

	// Load strategy params from config and pass them to Initialize.
	rawParams, err := cfg.GetStrategyParams(name)
	if err != nil || rawParams == nil {
		// No params block – Start with strategy defaults.
		if resetErr := strat.Reset(); resetErr != nil {
			return nil, fmt.Errorf("failed to reset strategy: %w", resetErr)
		}
		return strat, nil
	}

	// Convert config struct to map[string]interface{} for Initialize.
	initMap, err := strategyParamsToMap(name, rawParams)
	if err != nil {
		return nil, fmt.Errorf("failed to convert strategy params: %w", err)
	}
	if err := strat.Initialize(initMap); err != nil {
		return nil, fmt.Errorf("failed to initialize strategy: %w", err)
	}
	if err := strat.Reset(); err != nil {
		return nil, fmt.Errorf("failed to reset strategy: %w", err)
	}
	return strat, nil
}

// strategyParamsToMap converts a typed config params struct to a
// map[string]interface{} suitable for Strategy.Initialize.
func strategyParamsToMap(name string, params interface{}) (map[string]interface{}, error) {
	switch name {
	case "scalping":
		cp, ok := params.(config.ScalpingParams)
		if !ok {
			return nil, fmt.Errorf("expected config.ScalpingParams, got %T", params)
		}
		// Build symbol_params as map[string]map[string]interface{} so that
		// any strategy implementation can consume it generically.
		symParams := make(map[string]map[string]interface{}, len(cp.SymbolParams))
		for sym, ov := range cp.SymbolParams {
			symParams[sym] = map[string]interface{}{
				"ema_fast_period": ov.EMAFastPeriod,
				"ema_slow_period": ov.EMASlowPeriod,
				"cooldown_sec":    ov.CooldownSec,
				"min_notional":    ov.MinNotional,
			}
		}
		return map[string]interface{}{
			"ema_fast_period":  cp.EMAFastPeriod,
			"ema_slow_period":  cp.EMASlowPeriod,
			"take_profit_pct":  cp.TakeProfitPct,
			"stop_loss_pct":    cp.StopLossPct,
			"cooldown_sec":     cp.CooldownSec,
			"max_daily_trades": cp.MaxDailyTrades,
			"min_notional":     cp.MinNotional,
			"fee_rate":         cp.FeeRate,
			"rsi_period":       cp.RSIPeriod,
			"rsi_overbought":   cp.RSIOverbought,
			"rsi_oversold":     cp.RSIOversold,
			"symbol_params":    symParams,
		}, nil
	default:
		// Unknown strategy: pass the YAML-decoded value as-is.
		// Implementations that register non-scalping strategies must handle
		// their own param format in Initialize().
		return map[string]interface{}{"_raw": params}, nil
	}
}

// initDailyTradeCount restores today's trade count into the strategy.
// Uses InitializeDailyTradeCount (not RecordTrade) so that the cooldown timer
// is not reset to time.Now() on every restart.
func initDailyTradeCount(repo *persistence.Repository, strat pkgstrategy.Strategy, log logger.LoggerInterface) {
	trades, err := repo.GetRecentTrades(200)
	if err != nil {
		log.System().WithError(err).Warn("Failed to load recent trades for daily count init")
		return
	}
	now := utils.NowInJST()
	todayY, todayM, todayD := now.Date()
	count := 0
	for i := range trades {
		t := utils.ToJST(trades[i].ExecutedAt)
		y, m, d := t.Date()
		if y == todayY && m == todayM && d == todayD {
			count++
		}
	}
	strat.InitializeDailyTradeCount(count)
}

// ── appServiceAdapter ────────────────────────────────────────────────────────

type appServiceAdapter struct {
	tc     *tradingController
	trader trading.Trader
	strat  pkgstrategy.Strategy
}

func (a *appServiceAdapter) GetBalances(ctx context.Context) ([]domain.Balance, error) {
	return a.trader.GetBalance(ctx)
}
func (a *appServiceAdapter) GetCurrentStrategy() pkgstrategy.Strategy { return a.strat }
func (a *appServiceAdapter) IsTradingEnabled() bool                   { return a.tc.IsTradingEnabled() }
func (a *appServiceAdapter) SetTradingEnabled(enabled bool) error {
	return a.tc.SetTradingEnabled(enabled)
}

// ── marketDataAdapter ────────────────────────────────────────────────────────

type marketDataAdapter struct {
	client        *bitflyer.Client
	marketDataSvc *bitflyer.MarketDataService
}

func (m *marketDataAdapter) IsConnected() bool { return m.client.IsConnected() }
func (m *marketDataAdapter) ReconnectClient() error {
	ctx := context.Background()
	if err := m.client.Reconnect(ctx); err != nil {
		return fmt.Errorf("websocket reconnect failed: %w", err)
	}
	m.marketDataSvc.ResetCallbacks()
	return nil
}
func (m *marketDataAdapter) SubscribeToTicker(ctx context.Context, symbol string, callback func(domain.MarketData)) error {
	return m.marketDataSvc.SubscribeToTicker(ctx, symbol, func(bfd bitflyer.MarketData) {
		callback(domain.MarketData{
			Symbol: bfd.Symbol, ProductCode: bfd.Symbol, Timestamp: bfd.Timestamp,
			Price: bfd.Price, Volume: bfd.Volume, BestBid: bfd.BestBid,
			BestAsk: bfd.BestAsk, Spread: bfd.Spread,
			Open: bfd.Open, High: bfd.High, Low: bfd.Low, Close: bfd.Close,
		})
	})
}

// ── strategyGetter ────────────────────────────────────────────────────────────

type strategyGetter struct{ strat pkgstrategy.Strategy }

func (sg *strategyGetter) GetCurrentStrategy() pkgstrategy.Strategy { return sg.strat }

// ── tradingController ─────────────────────────────────────────────────────────
// Moved here from cmd/gogocoin so the engine package owns the full wiring.

type tradingController struct {
	mu      sync.RWMutex
	enabled bool
	db      domain.AppStateRepository
	log     logger.LoggerInterface
}

const appStateKeyTradingEnabled = "trading_enabled"

func newTradingController(db domain.AppStateRepository, log logger.LoggerInterface) (*tradingController, error) {
	tc := &tradingController{db: db, log: log}
	val, err := db.GetAppState(appStateKeyTradingEnabled)
	if err != nil {
		log.System().WithError(err).Warn("Failed to read trading state at startup")
	} else if val == "" {
		if saveErr := db.SaveAppState(appStateKeyTradingEnabled, "false"); saveErr != nil {
			log.System().WithError(saveErr).Warn("Failed to initialize trading state at startup")
		}
	} else if val == "true" {
		tc.enabled = true
	}
	return tc, nil
}

func (tc *tradingController) IsTradingEnabled() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.enabled
}

func (tc *tradingController) SetTradingEnabled(enabled bool) error {
	tc.mu.Lock()
	if tc.enabled == enabled {
		tc.mu.Unlock()
		return nil
	}
	tc.enabled = enabled
	tc.mu.Unlock()

	val := "false"
	if enabled {
		val = "true"
	}
	if err := tc.db.SaveAppState(appStateKeyTradingEnabled, val); err != nil {
		tc.log.System().WithError(err).Error("Failed to persist trading state")
		return fmt.Errorf("failed to persist trading state: %w", err)
	}
	if enabled {
		tc.log.System().Info("Trading enabled via API")
	} else {
		tc.log.System().Info("Trading disabled via API")
	}
	return nil
}
